package queue

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/riverqueue/river"
	"server/config"
	"server/internal/db"
	"server/internal/queue/jobs"
	"server/internal/workqos"
)

func TestLowLoadShortSnoozeWakesAfterCommitAndDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	root := t.TempDir()
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	database, err := db.OpenQueue(ctx, config.DatabaseConfig{Path: filepath.Join(root, "catalog.sqlite3"), QueuePath: filepath.Join(root, "river.sqlite3")})
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close(context.Background())
	if err := database.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	workers := river.NewWorkers()
	var turns atomic.Int32
	reached := make(chan time.Time, 3)
	river.AddWorker(workers, &ScanRepositoryBatchWorker{Execute: func(context.Context, workqos.Class, jobs.ScanRepositoryBatchArgs) (bool, error) {
		reached <- time.Now()
		return turns.Add(1) < 3, nil
	}})
	client, err := New(database.SQL, database.ReaderSQL, workers, slog.New(slog.NewTextHandler(io.Discard, nil)), 2)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer func() {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer stopCancel()
		if err := client.Stop(stopCtx); err != nil {
			t.Error(err)
		}
	}()
	_, err = client.Insert(ctx, jobs.ScanRepositoryBatchArgs{RepositoryID: uuid.New(), RequestedEpoch: 1, DesiredVersion: 1}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var previous time.Time
	for i := 0; i < 3; i++ {
		select {
		case at := <-reached:
			if !previous.IsZero() && at.Sub(previous) < 25*time.Millisecond {
				t.Fatalf("continuation ran before deadline: %s", at.Sub(previous))
			}
			previous = at
		case <-ctx.Done():
			t.Fatalf("only %d bounded turns ran before deadline: %v", turns.Load(), ctx.Err())
		}
	}
}
