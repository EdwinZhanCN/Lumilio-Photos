package sqlitespike

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riversqlite"
	"github.com/riverqueue/river/rivermigrate"
)

type retryOnceArgs struct {
	Value string `json:"value"`
}

func (retryOnceArgs) Kind() string { return "sqlite_spike_retry_once" }

type retryOnceWorker struct {
	river.WorkerDefaults[retryOnceArgs]

	attempts  atomic.Int32
	completed chan struct{}
	closeOnce sync.Once
}

func (w *retryOnceWorker) Work(_ context.Context, _ *river.Job[retryOnceArgs]) error {
	if w.attempts.Add(1) == 1 {
		return errors.New("intentional first-attempt failure")
	}
	w.closeOnce.Do(func() { close(w.completed) })
	return nil
}

func (w *retryOnceWorker) NextRetry(*river.Job[retryOnceArgs]) time.Time {
	return time.Now()
}

func TestRiverSQLiteMigrationTransactionAndRetry(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	database := openTestDatabase(t, ctx)
	if _, err := database.ExecContext(ctx, `
		CREATE TABLE app_records (
			id INTEGER PRIMARY KEY,
			value TEXT NOT NULL
		) STRICT
	`); err != nil {
		t.Fatalf("run application migration: %v", err)
	}

	driver := riversqlite.New(database)
	migrator, err := rivermigrate.New(driver, nil)
	if err != nil {
		t.Fatalf("create River migrator: %v", err)
	}
	firstMigration, err := migrator.Migrate(ctx, rivermigrate.DirectionUp, nil)
	if err != nil {
		t.Fatalf("run River migrations: %v", err)
	}
	if len(firstMigration.Versions) == 0 {
		t.Fatal("first River migration applied no versions")
	}
	secondMigration, err := migrator.Migrate(ctx, rivermigrate.DirectionUp, nil)
	if err != nil {
		t.Fatalf("rerun River migrations: %v", err)
	}
	if len(secondMigration.Versions) != 0 {
		t.Fatalf("second River migration applied %d versions, want 0", len(secondMigration.Versions))
	}

	worker := &retryOnceWorker{completed: make(chan struct{})}
	workers := river.NewWorkers()
	river.AddWorker(workers, worker)
	client, err := river.NewClient(driver, &river.Config{
		FetchCooldown:     5 * time.Millisecond,
		FetchPollInterval: 10 * time.Millisecond,
		Queues: map[string]river.QueueConfig{
			river.QueueDefault: {MaxWorkers: 1},
		},
		TestOnly: true,
		Workers:  workers,
	})
	if err != nil {
		t.Fatalf("create River client: %v", err)
	}

	rollbackTx, err := database.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin rollback transaction: %v", err)
	}
	if _, err := rollbackTx.ExecContext(ctx, "INSERT INTO app_records (id, value) VALUES (1, 'rollback')"); err != nil {
		t.Fatalf("insert rollback app record: %v", err)
	}
	if _, err := client.InsertTx(ctx, rollbackTx, retryOnceArgs{Value: "rollback"}, nil); err != nil {
		t.Fatalf("insert rollback River job: %v", err)
	}
	if err := rollbackTx.Rollback(); err != nil {
		t.Fatalf("rollback app + River transaction: %v", err)
	}
	assertCount(t, database, "SELECT count(*) FROM app_records", 0)
	assertCount(t, database, "SELECT count(*) FROM river_job WHERE kind = 'sqlite_spike_retry_once'", 0)

	commitTx, err := database.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin commit transaction: %v", err)
	}
	if _, err := commitTx.ExecContext(ctx, "INSERT INTO app_records (id, value) VALUES (2, 'commit')"); err != nil {
		t.Fatalf("insert commit app record: %v", err)
	}
	if _, err := client.InsertTx(ctx, commitTx, retryOnceArgs{Value: "commit"}, nil); err != nil {
		t.Fatalf("insert commit River job: %v", err)
	}
	if err := commitTx.Commit(); err != nil {
		t.Fatalf("commit app + River transaction: %v", err)
	}
	assertCount(t, database, "SELECT count(*) FROM app_records", 1)
	assertCount(t, database, "SELECT count(*) FROM river_job WHERE kind = 'sqlite_spike_retry_once'", 1)

	if err := client.Start(ctx); err != nil {
		t.Fatalf("start River client: %v", err)
	}
	select {
	case <-worker.completed:
	case <-ctx.Done():
		t.Fatalf("wait for retried River job: %v", ctx.Err())
	}
	if err := client.Stop(ctx); err != nil {
		t.Fatalf("stop River client: %v", err)
	}

	if got := worker.attempts.Load(); got != 2 {
		t.Fatalf("worker attempts = %d, want 2", got)
	}
	var state string
	var attempt int
	if err := database.QueryRowContext(ctx, `
		SELECT state, attempt
		FROM river_job
		WHERE kind = 'sqlite_spike_retry_once'
	`).Scan(&state, &attempt); err != nil {
		t.Fatalf("query completed River job: %v", err)
	}
	if state != "completed" || attempt != 2 {
		t.Fatalf("River job state/attempt = %s/%d, want completed/2", state, attempt)
	}
}

func assertCount(t *testing.T, database interface {
	QueryRow(query string, args ...any) *sql.Row
}, query string, want int) {
	t.Helper()

	var got int
	if err := database.QueryRow(query).Scan(&got); err != nil {
		t.Fatalf("query count: %v", err)
	}
	if got != want {
		t.Fatalf("count for %q = %d, want %d", query, got, want)
	}
}
