package commit

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
	"server/internal/db/catalogtx"
)

func TestCoordinatorSnapshotUsesBoundedHistogramsAndReportsPressure(t *testing.T) {
	writer, _ := testWriter(t)
	entered := make(chan struct{})
	release := make(chan struct{})
	handler := OutcomeHandler(func(context.Context, *sql.Tx, []Intent) ([]Outcome, error) {
		select {
		case <-entered:
		default:
			close(entered)
		}
		<-release
		return []Outcome{OutcomeApplied}, nil
	})
	coordinator, err := New(writer, Config{Capacity: 1, MaxBatch: 1, OldestWait: time.Millisecond}, map[string]Handler{"test": handler})
	if err != nil {
		t.Fatal(err)
	}
	coordinator.Start()
	defer coordinator.Stop(context.Background())

	var submissions sync.WaitGroup
	submissions.Add(2)
	go func() {
		defer submissions.Done()
		_, _ = coordinator.Submit(context.Background(), intent("active"))
	}()
	<-entered
	go func() {
		defer submissions.Done()
		_, _ = coordinator.Submit(context.Background(), intent("queued"))
	}()
	deadline := time.Now().Add(time.Second)
	for coordinator.Snapshot().Depth != 1 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if snapshot := coordinator.Snapshot(); snapshot.Depth != 1 {
		close(release)
		t.Fatalf("queue depth = %d, want full capacity", snapshot.Depth)
	}

	const canceled = 32
	for index := range canceled {
		ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
		_, submitErr := coordinator.Submit(ctx, intent(string(rune('a'+index))))
		cancel()
		if !errors.Is(submitErr, context.DeadlineExceeded) {
			close(release)
			t.Fatalf("blocked submission %d error = %v", index, submitErr)
		}
	}
	pressured := coordinator.Snapshot()
	if pressured.Capacity != 1 || pressured.Depth != 1 || pressured.PeakDepth != 1 {
		close(release)
		t.Fatalf("pressured queue snapshot = %+v", pressured)
	}
	if pressured.EnqueueCanceled != canceled || pressured.EnqueueWait.Count != 2 {
		close(release)
		t.Fatalf("pressured enqueue snapshot = %+v", pressured)
	}

	close(release)
	submissions.Wait()
	snapshot := coordinator.Snapshot()
	if snapshot.Depth != 0 || snapshot.BlockedSubmitters != 0 {
		t.Fatalf("drained queue snapshot = %+v", snapshot)
	}
	if snapshot.EnqueueWait.Count != 2 || snapshot.OldestWait.Count != 2 || snapshot.BatchSize.Count != 2 || snapshot.Transaction.Count != 2 || snapshot.Commit.Count != 2 {
		t.Fatalf("drained histogram counts = %+v", snapshot)
	}
	if snapshot.BatchSize.P99 > 1 || snapshot.EnqueueWait.P50 > snapshot.EnqueueWait.P95 || snapshot.EnqueueWait.P95 > snapshot.EnqueueWait.P99 {
		t.Fatalf("invalid bounded percentile snapshot = %+v", snapshot)
	}
}

func testWriter(t *testing.T) (*catalogtx.Writer, *sql.DB) {
	t.Helper()
	database, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "commit.sqlite3")+"?_txlock=immediate")
	if err != nil {
		t.Fatal(err)
	}
	database.SetMaxOpenConns(1)
	t.Cleanup(func() { database.Close() })
	if _, err := database.Exec(`CREATE TABLE applied(value TEXT PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	return catalogtx.NewWriter(database, nil), database
}
func intent(value string) Intent {
	return Intent{Key: Key{Family: "test", Subject: value, Fence: "fence", Stage: "stage", DesiredVersion: 1}, Payload: value}
}

func TestCoordinatorSplitsFailedBatchAndAcknowledgesEveryIntent(t *testing.T) {
	writer, database := testWriter(t)
	handler := func(ctx context.Context, tx *sql.Tx, intents []Intent) ([]Outcome, error) {
		out := make([]Outcome, len(intents))
		for i, item := range intents {
			value := item.Payload.(string)
			if value == "bad" {
				return nil, errors.New("injected")
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO applied(value) VALUES (?)`, value); err != nil {
				return nil, err
			}
			out[i] = OutcomeApplied
		}
		return out, nil
	}
	coordinator, err := New(writer, Config{Capacity: 8, MaxBatch: 8, OldestWait: 20 * time.Millisecond}, map[string]Handler{"test": OutcomeHandler(handler)})
	if err != nil {
		t.Fatal(err)
	}
	coordinator.Start()
	defer coordinator.Stop(context.Background())
	type result struct {
		value   string
		outcome Outcome
		err     error
	}
	results := make(chan result, 3)
	for _, value := range []string{"left", "bad", "right"} {
		go func(value string) {
			acknowledgement, err := coordinator.Submit(context.Background(), intent(value))
			results <- result{value, acknowledgement.Outcome, err}
		}(value)
	}
	got := map[string]result{}
	for range 3 {
		item := <-results
		got[item.value] = item
	}
	if got["left"].err != nil || got["right"].err != nil || got["bad"].err == nil {
		t.Fatalf("results=%+v", got)
	}
	var count int
	if err := database.QueryRow(`SELECT count(*) FROM applied`).Scan(&count); err != nil || count != 2 {
		t.Fatalf("count=%d err=%v", count, err)
	}
}

func TestAcknowledgingHandlerPreservesPerIntentTransactionBoundary(t *testing.T) {
	writer, database := testWriter(t)
	if _, err := database.Exec(`CREATE TABLE observed_batches(size INTEGER NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	handler := AcknowledgingHandler(func(ctx context.Context, tx *sql.Tx, intents []Intent) ([]Result, error) {
		if _, err := tx.ExecContext(ctx, `INSERT INTO observed_batches(size) VALUES (?)`, len(intents)); err != nil {
			return nil, err
		}
		results := make([]Result, len(intents))
		for index := range results {
			results[index].Outcome = OutcomeApplied
		}
		return results, nil
	})
	coordinator, err := New(writer, Config{Capacity: 8, MaxBatch: 8, OldestWait: 20 * time.Millisecond}, map[string]Handler{"test": handler})
	if err != nil {
		t.Fatal(err)
	}
	coordinator.Start()
	defer coordinator.Stop(context.Background())

	errors := make(chan error, 2)
	for _, value := range []string{"first", "second"} {
		go func(value string) {
			_, err := coordinator.Submit(context.Background(), intent(value))
			errors <- err
		}(value)
	}
	for range 2 {
		if err := <-errors; err != nil {
			t.Fatal(err)
		}
	}

	var count, minimum, maximum int
	if err := database.QueryRow(`SELECT count(*), min(size), max(size) FROM observed_batches`).Scan(&count, &minimum, &maximum); err != nil {
		t.Fatal(err)
	}
	if count != 2 || minimum != 1 || maximum != 1 {
		t.Fatalf("acknowledging handler transactions = count %d, sizes %d..%d; want two single-intent transactions", count, minimum, maximum)
	}
}

func TestCoordinatorBackpressureIsCancellationAware(t *testing.T) {
	writer, _ := testWriter(t)
	entered := make(chan struct{})
	release := make(chan struct{})
	handler := func(context.Context, *sql.Tx, []Intent) ([]Outcome, error) {
		select {
		case <-entered:
		default:
			close(entered)
		}
		<-release
		return []Outcome{OutcomeApplied}, nil
	}
	coordinator, _ := New(writer, Config{Capacity: 1, MaxBatch: 1, OldestWait: time.Hour}, map[string]Handler{"test": OutcomeHandler(handler)})
	coordinator.Start()
	defer coordinator.Stop(context.Background())
	go coordinator.Submit(context.Background(), intent("first"))
	<-entered
	go coordinator.Submit(context.Background(), intent("second"))
	time.Sleep(10 * time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err := coordinator.Submit(ctx, intent("third"))
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Submit error=%v", err)
	}
	close(release)
}

func TestCoordinatorStopUnblocksSubmissionWhenQueueIsFull(t *testing.T) {
	writer, _ := testWriter(t)
	entered := make(chan struct{})
	release := make(chan struct{})
	handler := func(context.Context, *sql.Tx, []Intent) ([]Outcome, error) {
		select {
		case <-entered:
		default:
			close(entered)
		}
		<-release
		return []Outcome{OutcomeApplied}, nil
	}
	coordinator, err := New(writer, Config{Capacity: 1, MaxBatch: 1, OldestWait: time.Hour}, map[string]Handler{"test": OutcomeHandler(handler)})
	if err != nil {
		t.Fatal(err)
	}
	coordinator.Start()
	firstDone := make(chan struct{})
	go func() {
		_, _ = coordinator.Submit(context.Background(), intent("first"))
		close(firstDone)
	}()
	<-entered
	secondDone := make(chan struct{})
	go func() {
		_, _ = coordinator.Submit(context.Background(), intent("second"))
		close(secondDone)
	}()
	time.Sleep(10 * time.Millisecond)
	thirdDone := make(chan error, 1)
	go func() {
		_, err := coordinator.Submit(context.Background(), intent("third"))
		thirdDone <- err
	}()
	stopDone := make(chan error, 1)
	go func() {
		stopDone <- coordinator.Stop(context.Background())
	}()
	select {
	case err := <-thirdDone:
		if !errors.Is(err, ErrStopped) {
			t.Fatalf("third submission error=%v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Stop did not unblock the full-queue submission")
	}
	close(release)
	select {
	case err := <-stopDone:
		if err != nil {
			t.Fatalf("Stop error=%v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Stop did not complete after releasing the active batch")
	}
	<-firstDone
	<-secondDone
}

func TestCoordinatorStopBeforeStartRejectsSubmit(t *testing.T) {
	writer, _ := testWriter(t)
	coordinator, err := New(writer, Config{Capacity: 1, MaxBatch: 1, OldestWait: time.Millisecond}, map[string]Handler{
		"test": OutcomeHandler(func(context.Context, *sql.Tx, []Intent) ([]Outcome, error) {
			return []Outcome{OutcomeApplied}, nil
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := coordinator.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := coordinator.Submit(ctx, intent("after-stop")); !errors.Is(err, ErrNotRunning) {
		// The coordinator has not started a worker goroutine in this case, so
		// rejection is immediate and deterministic.
		t.Fatalf("submit after stop error=%v", err)
	}
}

func TestCoordinatorRejectsAlreadyCanceledSubmission(t *testing.T) {
	writer, _ := testWriter(t)
	coordinator, err := New(writer, Config{Capacity: 1, MaxBatch: 1, OldestWait: time.Millisecond}, map[string]Handler{
		"test": OutcomeHandler(func(context.Context, *sql.Tx, []Intent) ([]Outcome, error) {
			return []Outcome{OutcomeApplied}, nil
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	coordinator.Start()
	defer coordinator.Stop(context.Background())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := coordinator.Submit(ctx, intent("already-canceled")); !errors.Is(err, context.Canceled) {
		t.Fatalf("Submit error=%v, want context canceled", err)
	}
}

func TestAssetStageCommitCompletesReceiptOnlyAfterEveryBoundStage(t *testing.T) {
	_, database := testWriter(t)
	for _, statement := range []string{
		`CREATE TABLE assets(asset_id TEXT PRIMARY KEY,type TEXT,status TEXT,updated_at INTEGER)`,
		`CREATE TABLE domain_outbox(outbox_id TEXT PRIMARY KEY,envelope_version INTEGER,command_kind TEXT,subject_key TEXT,desired_version INTEGER,envelope TEXT,available_at INTEGER,delivered_at INTEGER,delivery_attempts INTEGER,last_error TEXT,created_at INTEGER,updated_at INTEGER,UNIQUE(command_kind,subject_key,desired_version))`,
		`CREATE TABLE catalog_operation_receipts(receipt_id TEXT PRIMARY KEY,kind TEXT,subject_id TEXT,desired_version INTEGER,applied_version INTEGER DEFAULT 0,state TEXT,terminal_error TEXT,created_at INTEGER,updated_at INTEGER)`,
		`CREATE TABLE asset_pipeline_state(asset_id TEXT,source_content_id TEXT,stage TEXT,pipeline_version TEXT,desired_version INTEGER,applied_version INTEGER,terminal_error TEXT,updated_at INTEGER,PRIMARY KEY(asset_id,stage))`,
		`CREATE TABLE asset_pipeline_receipt_stages(receipt_id TEXT,asset_id TEXT,stage TEXT,desired_version INTEGER,PRIMARY KEY(receipt_id,asset_id,stage))`,
	} {
		if _, err := database.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	assetID, fence, receiptID := uuid.New(), uuid.New(), uuid.New()
	now := time.Now().UTC().UnixMicro()
	if _, err := database.Exec(`INSERT INTO assets VALUES(?, 'PHOTO', '{"state":"processing"}', ?)`, assetID.String(), now); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO catalog_operation_receipts VALUES(?, 'reprocess', ?, 1, 0, 'pending', NULL, ?, ?)`, receiptID.String(), assetID.String(), now, now); err != nil {
		t.Fatal(err)
	}
	for _, stage := range []string{"analyze", "enrich"} {
		if _, err := database.Exec(`INSERT INTO asset_pipeline_state VALUES(?,?,?,'asset-v1',1,0,NULL,?)`, assetID.String(), fence.String(), stage, now); err != nil {
			t.Fatal(err)
		}
		if _, err := database.Exec(`INSERT INTO asset_pipeline_receipt_stages VALUES(?,?,?,1)`, receiptID.String(), assetID.String(), stage); err != nil {
			t.Fatal(err)
		}
	}
	apply := func(stage string) {
		tx, err := database.BeginTx(context.Background(), nil)
		if err != nil {
			t.Fatal(err)
		}
		_, err = applyAssetStages(context.Background(), tx, []Intent{{Key: Key{Family: FamilyAssetStage, Subject: assetID.String(), Fence: fence.String(), Stage: stage, DesiredVersion: 1}, Payload: AssetStageApplied{AssetID: assetID, SourceFence: fence, Stage: stage, PipelineVersion: "asset-v1", DesiredVersion: 1}}})
		if err != nil {
			_ = tx.Rollback()
			t.Fatal(err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
	}
	apply("analyze")
	var state string
	if err := database.QueryRow(`SELECT state FROM catalog_operation_receipts WHERE receipt_id=?`, receiptID.String()).Scan(&state); err != nil || state != "pending" {
		t.Fatalf("after analyze state=%s err=%v", state, err)
	}
	apply("enrich")
	if err := database.QueryRow(`SELECT state FROM catalog_operation_receipts WHERE receipt_id=?`, receiptID.String()).Scan(&state); err != nil || state != "completed" {
		t.Fatalf("after enrich state=%s err=%v", state, err)
	}
}

func TestAssetStageTerminalFailureProjectsProductAndReceiptFailure(t *testing.T) {
	_, database := testWriter(t)
	for _, statement := range []string{
		`CREATE TABLE assets(asset_id TEXT PRIMARY KEY,type TEXT,status TEXT,updated_at INTEGER)`,
		`CREATE TABLE catalog_operation_receipts(receipt_id TEXT PRIMARY KEY,kind TEXT,subject_id TEXT,desired_version INTEGER,applied_version INTEGER DEFAULT 0,state TEXT,terminal_error TEXT,created_at INTEGER,updated_at INTEGER)`,
		`CREATE TABLE asset_pipeline_state(asset_id TEXT,source_content_id TEXT,stage TEXT,pipeline_version TEXT,desired_version INTEGER,applied_version INTEGER,terminal_error TEXT,updated_at INTEGER,PRIMARY KEY(asset_id,stage))`,
		`CREATE TABLE asset_pipeline_receipt_stages(receipt_id TEXT,asset_id TEXT,stage TEXT,desired_version INTEGER,PRIMARY KEY(receipt_id,asset_id,stage))`,
		`CREATE TABLE domain_outbox(outbox_id TEXT PRIMARY KEY,envelope_version INTEGER,command_kind TEXT,subject_key TEXT,desired_version INTEGER,envelope TEXT,available_at INTEGER,delivered_at INTEGER,delivery_attempts INTEGER,last_error TEXT,created_at INTEGER,updated_at INTEGER,UNIQUE(command_kind,subject_key,desired_version))`,
	} {
		if _, err := database.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	assetID, fence, receiptID := uuid.New(), uuid.New(), uuid.New()
	now := time.Now().UTC().UnixMicro()
	for _, statement := range []struct {
		query string
		args  []any
	}{
		{`INSERT INTO assets VALUES(?, 'PHOTO', '{"state":"processing"}', ?)`, []any{assetID.String(), now}},
		{`INSERT INTO catalog_operation_receipts VALUES(?, 'reprocess', ?, 1, 0, 'pending', NULL, ?, ?)`, []any{receiptID.String(), assetID.String(), now, now}},
		{`INSERT INTO asset_pipeline_state VALUES(?,?, 'derivatives', 'asset-v1', 1, 0, NULL, ?)`, []any{assetID.String(), fence.String(), now}},
		{`INSERT INTO asset_pipeline_receipt_stages VALUES(?,?, 'derivatives',1)`, []any{receiptID.String(), assetID.String()}},
	} {
		if _, err := database.Exec(statement.query, statement.args...); err != nil {
			t.Fatal(err)
		}
	}
	tx, err := database.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = applyAssetStages(context.Background(), tx, []Intent{{Key: Key{Family: FamilyAssetStage, Subject: assetID.String(), Fence: fence.String(), Stage: "derivatives", DesiredVersion: 1}, Payload: AssetStageApplied{AssetID: assetID, SourceFence: fence, Stage: "derivatives", PipelineVersion: "asset-v1", DesiredVersion: 1, TerminalError: "attempts_exhausted"}}})
	if err != nil {
		_ = tx.Rollback()
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	var stageError, assetStatus, receiptState, receiptError string
	if err := database.QueryRow(`SELECT terminal_error FROM asset_pipeline_state WHERE asset_id=?`, assetID.String()).Scan(&stageError); err != nil || stageError != "attempts_exhausted" {
		t.Fatalf("stage terminal error = %q, err=%v", stageError, err)
	}
	if err := database.QueryRow(`SELECT status FROM assets WHERE asset_id=?`, assetID.String()).Scan(&assetStatus); err != nil || assetStatus != `{"state":"failed"}` {
		t.Fatalf("asset status = %q, err=%v", assetStatus, err)
	}
	if err := database.QueryRow(`SELECT state,terminal_error FROM catalog_operation_receipts WHERE receipt_id=?`, receiptID.String()).Scan(&receiptState, &receiptError); err != nil || receiptState != "failed" || receiptError != "attempts_exhausted" {
		t.Fatalf("receipt = %q/%q, err=%v", receiptState, receiptError, err)
	}
}
