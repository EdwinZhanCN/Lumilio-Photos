package app

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/riverqueue/river"

	"server/config"
	"server/internal/commit"
	"server/internal/db"
	"server/internal/db/catalogtx"
	"server/internal/pipeline"
	"server/internal/queue"
	"server/internal/queue/jobs"
)

func TestCatalogReceiptSurvivesQueueDBReplacementAndCompletesAfterRedelivery(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	configuration := config.DatabaseConfig{
		Path:      filepath.Join(root, "catalog.sqlite3"),
		QueuePath: filepath.Join(root, "river.sqlite3"),
	}
	catalog, err := db.Open(ctx, configuration)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = catalog.Close(context.Background()) })
	if err := catalog.MigrateCatalog(ctx); err != nil {
		t.Fatal(err)
	}

	commitID, receiptID := uuid.New(), uuid.New()
	if err := catalog.Writer.Transact(ctx, catalogtx.OperationAssetStagingCommit, nil, func(tx *sql.Tx) error {
		return pipeline.RequestIngestTx(ctx, tx, commitID, receiptID)
	}); err != nil {
		t.Fatal(err)
	}
	assertReceiptState(t, catalog.ReaderSQL, receiptID, "pending", 0)

	queueDatabase := openRecoveryQueue(t, ctx, configuration)
	client := newRecoveryQueueClient(t, queueDatabase)
	scheduler, err := queue.NewScheduler(catalog.Reader, catalog.Writer, client, make(chan struct{}), 8, time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if scheduled, err := scheduler.ScheduleOnce(ctx); err != nil || scheduled != 1 {
		t.Fatalf("initial QueueDB scheduling = %d, %v", scheduled, err)
	}
	assertMacroJobs(t, queueDatabase.ReaderSQL, 1, "ingest_asset", 1)
	if err := queueDatabase.Close(ctx); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(configuration.QueuePath, configuration.QueuePath+".replaced"); err != nil {
		t.Fatal(err)
	}
	for _, suffix := range []string{"-wal", "-shm"} {
		if err := os.Rename(configuration.QueuePath+suffix, configuration.QueuePath+".replaced"+suffix); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}

	replacement := openRecoveryQueue(t, ctx, configuration)
	t.Cleanup(func() { _ = replacement.Close(context.Background()) })
	replacementClient := newRecoveryQueueClient(t, replacement)
	replacementScheduler, err := queue.NewScheduler(catalog.Reader, catalog.Writer, replacementClient, make(chan struct{}), 8, time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if scheduled, err := replacementScheduler.ScheduleOnce(ctx); err != nil || scheduled != 1 {
		t.Fatalf("catalog recovery scheduling = %d, %v", scheduled, err)
	}
	assertMacroJobs(t, replacement.ReaderSQL, 1, "ingest_asset", 1)
	assertReceiptState(t, catalog.ReaderSQL, receiptID, "pending", 0)

	coordinator, err := commit.New(
		catalog.Writer,
		commit.Config{Capacity: 4, MaxBatch: 1, OldestWait: time.Millisecond},
		commit.CatalogDependencies{},
	)
	if err != nil {
		t.Fatal(err)
	}
	coordinator.Start()
	t.Cleanup(func() { _ = coordinator.Stop(context.Background()) })
	if result, err := coordinator.ApplyIngestReceipt(ctx, commit.IngestReceiptApplied{ReceiptID: receiptID}, commitID); err != nil || result.Outcome != commit.OutcomeApplied {
		t.Fatalf("redelivered macro acknowledgement = %+v, %v", result, err)
	}
	assertReceiptState(t, catalog.ReaderSQL, receiptID, "completed", 1)
	if scheduled, err := replacementScheduler.ScheduleOnce(ctx); err != nil || scheduled != 0 {
		t.Fatalf("completed receipt scheduling = %d, %v", scheduled, err)
	}
}

func openRecoveryQueue(t *testing.T, ctx context.Context, configuration config.DatabaseConfig) *db.QueueDB {
	t.Helper()
	queueDatabase, err := db.OpenQueue(ctx, configuration)
	if err != nil {
		t.Fatal(err)
	}
	if err := queueDatabase.Migrate(ctx); err != nil {
		_ = queueDatabase.Close(context.Background())
		t.Fatal(err)
	}
	return queueDatabase
}

func newRecoveryQueueClient(t *testing.T, queueDatabase *db.QueueDB) *river.Client[*sql.Tx] {
	t.Helper()
	workers := river.NewWorkers()
	river.AddWorker(workers, &queue.IngestMacroWorker{})
	client, err := queue.New(
		queueDatabase.SQL,
		queueDatabase.ReaderSQL,
		workers,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		8,
	)
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func assertReceiptState(t *testing.T, reader *sql.DB, receiptID uuid.UUID, state string, applied uint64) {
	t.Helper()
	var gotState string
	var gotApplied uint64
	if err := reader.QueryRow(`SELECT state,applied_version FROM catalog_operation_receipts WHERE receipt_id=?`, receiptID.String()).Scan(&gotState, &gotApplied); err != nil {
		t.Fatal(err)
	}
	if gotState != state || gotApplied != applied {
		t.Fatalf("receipt state = %s/%d, want %s/%d", gotState, gotApplied, state, applied)
	}
}

func assertMacroJobs(t *testing.T, reader *sql.DB, count int, kind string, priority int) {
	t.Helper()
	var gotCount int
	var gotKind string
	var gotPriority, gotAttempts int
	var gotQueue string
	if err := reader.QueryRow(`SELECT count(*),coalesce(max(kind),''),coalesce(max(priority),0),coalesce(max(max_attempts),0),coalesce(max(queue),'') FROM river_job`).Scan(&gotCount, &gotKind, &gotPriority, &gotAttempts, &gotQueue); err != nil {
		t.Fatal(err)
	}
	if gotCount != count || gotKind != kind || gotPriority != priority || gotAttempts != 8 || gotQueue != jobs.QueueMacro {
		t.Fatalf("QueueDB jobs = %d/%q priority=%d attempts=%d queue=%q, want %d/%q priority=%d attempts=8 queue=%q", gotCount, gotKind, gotPriority, gotAttempts, gotQueue, count, kind, priority, jobs.QueueMacro)
	}
}
