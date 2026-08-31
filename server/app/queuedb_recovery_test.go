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
	"server/internal/domainoutbox"
	"server/internal/pipeline"
	"server/internal/queue"
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
		return pipeline.RequestIngestTx(ctx, tx, commitID, receiptID, uuid.New())
	}); err != nil {
		t.Fatal(err)
	}
	assertReceiptState(t, catalog.ReaderSQL, receiptID, "pending", 0)

	queueDatabase := openRecoveryQueue(t, ctx, configuration)
	client := newRecoveryQueueClient(t, queueDatabase)
	dispatcher, err := domainoutbox.NewDispatcher(
		catalog.Reader,
		catalog.Writer,
		queue.NewDomainAdapter(client),
		8,
		time.Millisecond,
	)
	if err != nil {
		t.Fatal(err)
	}
	if delivered, err := dispatcher.DeliverOnce(ctx); err != nil || delivered != 1 {
		t.Fatalf("initial QueueDB delivery = %d, %v", delivered, err)
	}
	assertMacroJobs(t, queueDatabase.ReaderSQL, 1, "ingest_asset")
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
	replacementDispatcher, err := domainoutbox.NewDispatcher(
		catalog.Reader,
		catalog.Writer,
		queue.NewDomainAdapter(replacementClient),
		8,
		time.Millisecond,
	)
	if err != nil {
		t.Fatal(err)
	}
	if inserted, err := domainoutbox.NewReconciler(catalog.Writer, time.Second).ReconcileOnce(ctx); err != nil || inserted != 1 {
		t.Fatalf("catalog recovery reconciliation = %d, %v", inserted, err)
	}
	if delivered, err := replacementDispatcher.DeliverOnce(ctx); err != nil || delivered != 1 {
		t.Fatalf("replacement QueueDB delivery = %d, %v", delivered, err)
	}
	assertMacroJobs(t, replacement.ReaderSQL, 1, "ingest_asset")
	assertReceiptState(t, catalog.ReaderSQL, receiptID, "pending", 0)

	handlers := commit.CatalogHandlersWithAllServices(nil, nil, nil)
	coordinator, err := commit.New(
		catalog.Writer,
		commit.Config{Capacity: 4, MaxBatch: 1, OldestWait: time.Millisecond},
		handlers,
	)
	if err != nil {
		t.Fatal(err)
	}
	coordinator.Start()
	t.Cleanup(func() { _ = coordinator.Stop(context.Background()) })
	if result, err := coordinator.Submit(ctx, commit.Intent{
		Key: commit.Key{
			Family: commit.FamilyIngestReceipt, Subject: receiptID.String(),
			Fence: commitID.String(), Stage: "ingest", DesiredVersion: 1,
		},
		Payload: commit.IngestReceiptApplied{ReceiptID: receiptID},
	}); err != nil || result.Outcome != commit.OutcomeApplied {
		t.Fatalf("redelivered macro acknowledgement = %+v, %v", result, err)
	}
	assertReceiptState(t, catalog.ReaderSQL, receiptID, "completed", 1)
	if inserted, err := domainoutbox.NewReconciler(catalog.Writer, time.Second).ReconcileOnce(ctx); err != nil || inserted != 0 {
		t.Fatalf("completed receipt reconciliation = %d, %v", inserted, err)
	}
	if delivered, err := replacementDispatcher.DeliverOnce(ctx); err != nil || delivered != 0 {
		t.Fatalf("completed receipt redelivery = %d, %v", delivered, err)
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
		queue.NewMacroErrorHandler(),
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

func assertMacroJobs(t *testing.T, reader *sql.DB, count int, kind string) {
	t.Helper()
	var gotCount int
	var gotKind string
	if err := reader.QueryRow(`SELECT count(*),coalesce(max(kind),'') FROM river_job`).Scan(&gotCount, &gotKind); err != nil {
		t.Fatal(err)
	}
	if gotCount != count || gotKind != kind {
		t.Fatalf("QueueDB jobs = %d/%q, want %d/%q", gotCount, gotKind, count, kind)
	}
}
