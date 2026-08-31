package app

import (
	"context"
	"database/sql"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"

	"server/internal/commit"
	"server/internal/db/catalogtx"
	"server/internal/db/repo"
	"server/internal/execution"
	"server/internal/queue/jobs"
)

func TestRepositoryScanCommandKeepsActiveRunRunnableAfterDesiredEpochCoalesces(t *testing.T) {
	runID := uuid.New()
	state := repo.RepositoryObservationState{
		DesiredEpoch: 5,
		AppliedEpoch: 0,
		ActiveRunID:  uuid.NullUUID{UUID: runID, Valid: true},
	}
	run := repo.RepositoryScanRun{RunID: runID, RequestedEpoch: 1}
	args := jobs.ScanRepositoryBatchArgs{RepositoryID: uuid.New(), RequestedEpoch: 1, DesiredVersion: 1}
	if !repositoryScanCommandCurrent(state, run, args) {
		t.Fatal("coalesced desired epoch made the active repository run stale")
	}
	args.RequestedEpoch = 5
	args.DesiredVersion = 5
	if repositoryScanCommandCurrent(state, run, args) {
		t.Fatal("latest desired epoch was accepted in place of the active run epoch")
	}
}

func TestCommitQueuePressureStopsExecutionAdmission(t *testing.T) {
	database, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "backpressure.sqlite3")+"?_txlock=immediate")
	if err != nil {
		t.Fatal(err)
	}
	database.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = database.Close() })

	entered := make(chan struct{})
	releaseCommit := make(chan struct{})
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(releaseCommit) }) }
	defer release()
	handler := commit.OutcomeHandler(func(context.Context, *sql.Tx, []commit.Intent) ([]commit.Outcome, error) {
		select {
		case <-entered:
		default:
			close(entered)
		}
		<-releaseCommit
		return []commit.Outcome{commit.OutcomeApplied}, nil
	})
	coordinator, err := commit.New(
		catalogtx.NewWriter(database, nil),
		commit.Config{Capacity: 1, MaxBatch: 1, OldestWait: time.Millisecond},
		map[string]commit.Handler{commit.FamilyAssetStage: handler},
	)
	if err != nil {
		t.Fatal(err)
	}
	coordinator.Start()
	t.Cleanup(func() { _ = coordinator.Stop(context.Background()) })
	governor, err := execution.NewGovernor(execution.Resources{CPU: 2, MemoryBytes: 2}, 1)
	if err != nil {
		t.Fatal(err)
	}
	runtime := &pipelineRuntime{engine: execution.NewEngine(governor), commits: coordinator}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	fence := uuid.New()
	stage := func(assetID uuid.UUID) error {
		return runtime.engine.Run(ctx, execution.ClassBackground, execution.Resources{CPU: 1, MemoryBytes: 1}, func(stepCtx context.Context) error {
			return runtime.submitAssetStage(stepCtx, assetID, fence, "analyze", "asset-v1", 1)
		})
	}

	var group sync.WaitGroup
	errorsByStage := make(chan error, 3)
	group.Add(1)
	go func() {
		defer group.Done()
		errorsByStage <- stage(uuid.New())
	}()
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal("first commit did not enter the coordinator")
	}

	group.Add(1)
	go func() {
		defer group.Done()
		errorsByStage <- stage(uuid.New())
	}()
	waitForRuntimePressure(t, ctx, func() bool {
		return coordinator.Snapshot().Depth == 1 && governor.Snapshot().InUse.CPU == 2
	})

	group.Add(1)
	go func() {
		defer group.Done()
		errorsByStage <- stage(uuid.New())
	}()
	waitForRuntimePressure(t, ctx, func() bool { return governor.Snapshot().Waiting == 1 })
	if snapshot := governor.Snapshot(); snapshot.InUse.CPU != 2 || snapshot.Waiting != 1 {
		release()
		t.Fatalf("execution did not stop at commit pressure: %+v", snapshot)
	}
	if snapshot := coordinator.Snapshot(); snapshot.Depth != snapshot.Capacity {
		release()
		t.Fatalf("coordinator was not full while execution waited: %+v", snapshot)
	}

	release()
	group.Wait()
	close(errorsByStage)
	for err := range errorsByStage {
		if err != nil {
			t.Fatal(err)
		}
	}
	if snapshot := governor.Snapshot(); snapshot.InUse != (execution.Resources{}) || snapshot.Waiting != 0 {
		t.Fatalf("execution resources did not drain: %+v", snapshot)
	}
}

func waitForRuntimePressure(t *testing.T, ctx context.Context, ready func() bool) {
	t.Helper()
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		if ready() {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatal("timed out waiting for runtime pressure")
		case <-ticker.C:
		}
	}
}
