package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
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
	"server/internal/workqos"
)

type failureFixture struct {
	database  *db.DB
	runtime   *pipelineRuntime
	args      jobs.AnalyzeAssetArgs
	receiptID uuid.UUID
}

func newFailureFixture(t *testing.T) failureFixture {
	t.Helper()
	ctx := context.Background()
	root := t.TempDir()
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	database, err := db.Open(ctx, config.DatabaseConfig{Path: filepath.Join(root, "catalog.sqlite3")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close(context.Background()) })
	if err := database.MigrateCatalog(ctx, nil); err != nil {
		t.Fatal(err)
	}
	assetID, fence, receiptID := uuid.New(), uuid.New(), uuid.New()
	if err := database.Writer.Transact(ctx, catalogtx.OperationAssetStagingCommit, nil, func(tx *sql.Tx) error {
		for _, statement := range []struct {
			sql  string
			args []any
		}{
			{`INSERT INTO users(user_id,username,password,created_at,updated_at,webauthn_user_handle) VALUES(1,'test','unused',1,1,X'01')`, nil},
			{`INSERT INTO content_objects(content_id,hash_algorithm,full_hash,file_size,created_at) VALUES(?,'blake3-v1',?,1,1)`, []any{fence.String(), strings.Repeat("a", 64)}},
			{`INSERT INTO assets(asset_id,owner_id,content_id,type,original_filename,mime_type,upload_time,updated_at) VALUES(?,1,?,'PHOTO','fixture.jxl','image/jxl',1,1)`, []any{assetID.String(), fence.String()}},
			{`INSERT INTO catalog_operation_receipts(receipt_id,kind,subject_id,desired_version,state,created_at,updated_at) VALUES(?,'retry',?,1,'pending',1,1)`, []any{receiptID.String(), assetID.String()}},
		} {
			if _, err := tx.ExecContext(ctx, statement.sql, statement.args...); err != nil {
				return err
			}
		}
		return pipeline.RequestAssetStagesTx(ctx, tx, assetID, fence, []pipeline.Stage{pipeline.StageAnalyze}, pipeline.AssetPipelineVersion, workqos.Background, receiptID)
	}); err != nil {
		t.Fatal(err)
	}
	coordinator, err := commit.New(database.Writer, commit.Config{Capacity: 8, MaxBatch: 1, OldestWait: time.Millisecond}, commit.CatalogDependencies{})
	if err != nil {
		t.Fatal(err)
	}
	coordinator.Start()
	t.Cleanup(func() { _ = coordinator.Stop(context.Background()) })
	return failureFixture{database: database, runtime: &pipelineRuntime{pipelineReader: database.ReaderSQL, commits: coordinator}, args: jobs.AnalyzeAssetArgs{AssetID: assetID, SourceFence: fence, PipelineVersion: pipeline.AssetPipelineVersion, DesiredVersion: 1}, receiptID: receiptID}
}

func (f failureFixture) expireRetry(t *testing.T) {
	t.Helper()
	if err := f.database.Writer.Transact(context.Background(), catalogtx.OperationAssetStagingCommit, nil, func(tx *sql.Tx) error {
		_, err := tx.Exec(`UPDATE asset_pipeline_failures SET retry_after=0 WHERE asset_id=?`, f.args.AssetID.String())
		return err
	}); err != nil {
		t.Fatal(err)
	}
}

func (f failureFixture) freshQueueSchedules(t *testing.T, want int) {
	t.Helper()
	root := t.TempDir()
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	q := openRecoveryQueue(t, context.Background(), config.DatabaseConfig{Path: filepath.Join(root, "unused.sqlite3"), QueuePath: filepath.Join(root, "queue.sqlite3")})
	defer q.Close(context.Background())
	workers := river.NewWorkers()
	river.AddWorker(workers, queue.NewAnalyzeAssetWorker(nil))
	client, err := queue.New(q.SQL, q.ReaderSQL, workers, slog.New(slog.NewTextHandler(io.Discard, nil)), 2)
	if err != nil {
		t.Fatal(err)
	}
	scheduler, err := queue.NewScheduler(f.database.Reader, f.database.Writer, client, make(chan struct{}), 8, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if count, err := scheduler.ScheduleOnce(context.Background()); err != nil || count != want {
		t.Fatalf("replacement QueueDB scheduled %d, want %d: %v", count, want, err)
	}
}

func TestCatalogFailureBudgetSurvivesFreshQueueAndExplicitRetryRecovers(t *testing.T) {
	f := newFailureFixture(t)
	ctx := context.Background()
	executions := 0
	guarded := guardAssetExecution(f.runtime, func(context.Context, workqos.Class, jobs.AnalyzeAssetArgs) error {
		executions++
		return errors.New("temporary processor failure")
	})
	for attempt := 1; attempt <= pipeline.AssetFailureLimit; attempt++ {
		f.expireRetry(t)
		if err := guarded(ctx, workqos.Background, f.args); err != nil {
			t.Fatal(err)
		}
		var failures int
		if err := f.database.ReaderSQL.QueryRow(`SELECT failure_count FROM asset_pipeline_failures WHERE asset_id=?`, f.args.AssetID.String()).Scan(&failures); err != nil || failures != attempt {
			t.Fatalf("failures=%d, want %d: %v", failures, attempt, err)
		}
		f.freshQueueSchedules(t, 0)
		if attempt < pipeline.AssetFailureLimit {
			if err := guarded(ctx, workqos.Background, f.args); err == nil {
				t.Fatal("cooldown did not defer old delivery")
			}
			if executions != attempt {
				t.Fatal("cooldown ran compute")
			}
		}
	}
	var terminal, status, receiptState string
	if err := f.database.ReaderSQL.QueryRow(`SELECT terminal_error FROM asset_pipeline_state WHERE asset_id=?`, f.args.AssetID.String()).Scan(&terminal); err != nil || terminal != "processing_retry_exhausted" {
		t.Fatalf("terminal=%q: %v", terminal, err)
	}
	if err := f.database.ReaderSQL.QueryRow(`SELECT status FROM assets WHERE asset_id=?`, f.args.AssetID.String()).Scan(&status); err != nil || status != `{"state":"failed"}` {
		t.Fatalf("status=%q: %v", status, err)
	}
	if err := f.database.ReaderSQL.QueryRow(`SELECT state FROM catalog_operation_receipts WHERE receipt_id=?`, f.receiptID.String()).Scan(&receiptState); err != nil || receiptState != "failed" {
		t.Fatalf("receipt=%q: %v", receiptState, err)
	}
	if err := guarded(ctx, workqos.Background, f.args); err != nil || executions != pipeline.AssetFailureLimit {
		t.Fatalf("terminal replay reran compute: executions=%d error=%v", executions, err)
	}
	if err := f.database.Writer.Transact(ctx, catalogtx.OperationAssetStagingCommit, nil, func(tx *sql.Tx) error {
		return pipeline.RequestAssetStagesTx(ctx, tx, f.args.AssetID, f.args.SourceFence, []pipeline.Stage{pipeline.StageAnalyze}, pipeline.AssetPipelineVersion, workqos.Interactive, uuid.New())
	}); err != nil {
		t.Fatal(err)
	}
	f.freshQueueSchedules(t, 1)
	if err := guarded(ctx, workqos.Background, f.args); err != nil || executions != pipeline.AssetFailureLimit {
		t.Fatal("superseded delivery ran compute")
	}
	f.args.DesiredVersion++
	recovered := guardAssetExecution(f.runtime, func(ctx context.Context, _ workqos.Class, args jobs.AnalyzeAssetArgs) error {
		return f.runtime.submitAssetStage(ctx, args.AssetID, args.SourceFence, "analyze", args.PipelineVersion, args.DesiredVersion)
	})
	if err := recovered(ctx, workqos.Background, f.args); err != nil {
		t.Fatal(err)
	}
	var applied uint64
	if err := f.database.ReaderSQL.QueryRow(`SELECT applied_version FROM asset_pipeline_state WHERE asset_id=?`, f.args.AssetID.String()).Scan(&applied); err != nil || applied != 2 {
		t.Fatalf("recovery applied=%d: %v", applied, err)
	}
	f.freshQueueSchedules(t, 0)
}

func TestAssetFailureAcknowledgementIsFencedAndIdempotent(t *testing.T) {
	f := newFailureFixture(t)
	identity, err := assetExecutionIdentity(f.args)
	if err != nil {
		t.Fatal(err)
	}
	payload := commit.AssetStageFailure{Identity: identity}
	for i := 0; i < 2; i++ {
		result, err := f.runtime.commits.ApplyAssetFailure(context.Background(), payload)
		if err != nil {
			t.Fatal(err)
		}
		if i == 1 && result.Outcome != commit.OutcomeDuplicate {
			t.Fatalf("repeat outcome=%v", result.Outcome)
		}
	}
	payload.Identity.SourceFence = uuid.New()
	if result, err := f.runtime.commits.ApplyAssetFailure(context.Background(), payload); err != nil || result.Outcome != commit.OutcomeStale {
		t.Fatalf("stale acknowledgement=%v: %v", result, err)
	}
	var failures int
	if err := f.database.ReaderSQL.QueryRow(`SELECT failure_count FROM asset_pipeline_failures WHERE asset_id=?`, f.args.AssetID.String()).Scan(&failures); err != nil || failures != 1 {
		t.Fatalf("duplicate consumed failures=%d: %v", failures, err)
	}
}

func TestAssetFailureClassificationAndCancellation(t *testing.T) {
	for _, kind := range []string{"unsupported", "cancelled"} {
		t.Run(kind, func(t *testing.T) {
			f := newFailureFixture(t)
			computeErr := fmt.Errorf("processor: %w", pipeline.ErrUnsupportedMedia)
			if kind == "cancelled" {
				computeErr = context.Canceled
			}
			guarded := guardAssetExecution(f.runtime, func(context.Context, workqos.Class, jobs.AnalyzeAssetArgs) error { return computeErr })
			err := guarded(context.Background(), workqos.Background, f.args)
			if kind == "cancelled" {
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("cancel error=%v", err)
				}
				var count int
				if err := f.database.ReaderSQL.QueryRow(`SELECT count(*) FROM asset_pipeline_failures`).Scan(&count); err != nil || count != 0 {
					t.Fatalf("cancellation consumed failure budget: %d %v", count, err)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				var terminal string
				if err := f.database.ReaderSQL.QueryRow(`SELECT terminal_error FROM asset_pipeline_state WHERE asset_id=?`, f.args.AssetID.String()).Scan(&terminal); err != nil || terminal != "unsupported_media" {
					t.Fatalf("terminal=%q: %v", terminal, err)
				}
			}
		})
	}
}

func TestAssetGuardDoesNotAcknowledgeFailedCatalogCommit(t *testing.T) {
	f := newFailureFixture(t)
	ctx := context.Background()
	if err := f.database.Writer.Transact(ctx, catalogtx.OperationAssetStagingCommit, nil, func(tx *sql.Tx) error {
		_, err := tx.Exec(`CREATE TRIGGER reject_stage_commit BEFORE UPDATE OF applied_version ON asset_pipeline_state BEGIN SELECT RAISE(ABORT,'injected stage commit failure'); END`)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	guarded := guardAssetExecution(f.runtime, func(ctx context.Context, _ workqos.Class, args jobs.AnalyzeAssetArgs) error {
		return f.runtime.submitAssetStage(ctx, args.AssetID, args.SourceFence, "analyze", args.PipelineVersion, args.DesiredVersion)
	})
	if err := guarded(ctx, workqos.Background, f.args); err == nil {
		t.Fatal("failed Catalog commit was acknowledged as a completed delivery")
	}
	var count int
	if err := f.database.ReaderSQL.QueryRow(`SELECT count(*) FROM asset_pipeline_failures`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("commit error consumed domain budget: %d %v", count, err)
	}
	if err := f.database.Writer.Transact(ctx, catalogtx.OperationAssetStagingCommit, nil, func(tx *sql.Tx) error {
		_, err := tx.Exec(`DROP TRIGGER reject_stage_commit`)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if err := guarded(ctx, workqos.Background, f.args); err != nil {
		t.Fatal(err)
	}
	var applied int
	if err := f.database.ReaderSQL.QueryRow(`SELECT applied_version FROM asset_pipeline_state WHERE asset_id=?`, f.args.AssetID.String()).Scan(&applied); err != nil || applied != 1 {
		t.Fatalf("redelivery did not apply: %d %v", applied, err)
	}
}
