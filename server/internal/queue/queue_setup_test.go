package queue

import (
	"context"
	"database/sql"
	"sync"
	"testing"
	"time"

	sqlite3 "github.com/mattn/go-sqlite3"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver"

	"server/internal/db/catalogtx"
	"server/internal/queue/jobs"
)

func TestClampWorkers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value int
		min   int
		max   int
		want  int
	}{
		{name: "below minimum", value: 1, min: 2, max: 8, want: 2},
		{name: "within range", value: 6, min: 2, max: 8, want: 6},
		{name: "above maximum", value: 10, min: 2, max: 8, want: 8},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := clampWorkers(tt.value, tt.min, tt.max); got != tt.want {
				t.Fatalf("clampWorkers(%d, %d, %d) = %d, want %d", tt.value, tt.min, tt.max, got, tt.want)
			}
		})
	}
}

func TestQueueWorkerCountsForCPU(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		cpuCount      int
		wantIngest    int
		wantThumbnail int
		wantPHash     int
	}{
		{name: "single cpu", cpuCount: 1, wantIngest: 2, wantThumbnail: 4, wantPHash: 1},
		{name: "four cpu", cpuCount: 4, wantIngest: 2, wantThumbnail: 4, wantPHash: 1},
		{name: "eight cpu", cpuCount: 8, wantIngest: 4, wantThumbnail: 8, wantPHash: 2},
		{name: "many cpu", cpuCount: 32, wantIngest: 8, wantThumbnail: 12, wantPHash: 4},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ingestWorkers, thumbnailWorkers, phashWorkers := queueWorkerCountsForCPU(tt.cpuCount)
			if ingestWorkers != tt.wantIngest {
				t.Fatalf("ingestWorkers = %d, want %d", ingestWorkers, tt.wantIngest)
			}
			if thumbnailWorkers != tt.wantThumbnail {
				t.Fatalf("thumbnailWorkers = %d, want %d", thumbnailWorkers, tt.wantThumbnail)
			}
			if phashWorkers != tt.wantPHash {
				t.Fatalf("phashWorkers = %d, want %d", phashWorkers, tt.wantPHash)
			}
			if thumbnailWorkers < ingestWorkers {
				t.Fatalf("thumbnailWorkers = %d, want >= ingestWorkers = %d", thumbnailWorkers, ingestWorkers)
			}
		})
	}
}

func TestRuntimeQueueConfigsExactlyMatchClosedJobCatalog(t *testing.T) {
	t.Parallel()

	queues := runtimeQueueConfigsForCPU(8)
	if err := validateRuntimeQueueConfigs(queues); err != nil {
		t.Fatal(err)
	}
	if len(queues) != len(jobs.RuntimeQueueNames()) {
		t.Fatalf("configured queues=%d catalog queues=%d", len(queues), len(jobs.RuntimeQueueNames()))
	}

	delete(queues, jobs.QueueMetadataAsset)
	queues["orphan_queue"] = river.QueueConfig{MaxWorkers: 1}
	if err := validateRuntimeQueueConfigs(queues); err == nil {
		t.Fatal("queue catalog drift must fail closed")
	}
}

func TestRiverNotificationListenerUsesReaderWhileWriterIsHeld(t *testing.T) {
	t.Parallel()

	dsn := "file:" + t.Name() + "?mode=memory&cache=shared&_busy_timeout=5000"
	writerCapture := &queueStatementCapture{}
	readerCapture := &queueStatementCapture{}
	writerPool := sql.OpenDB(catalogtx.NewConnector(&sqlite3.SQLiteDriver{}, dsn, catalogtx.RoleWriter, writerCapture))
	writerPool.SetMaxOpenConns(1)
	writerPool.SetMaxIdleConns(1)
	readerPool := sql.OpenDB(catalogtx.NewConnector(&sqlite3.SQLiteDriver{}, dsn+"&_query_only=on", catalogtx.RoleReader, readerCapture))
	readerPool.SetMaxOpenConns(1)
	readerPool.SetMaxIdleConns(1)
	t.Cleanup(func() {
		_ = readerPool.Close()
		_ = writerPool.Close()
	})

	if _, err := writerPool.ExecContext(context.Background(), `
		CREATE TABLE river_notification (
			id INTEGER PRIMARY KEY,
			created_at TEXT NOT NULL,
			payload TEXT NOT NULL,
			topic TEXT NOT NULL
		)
	`); err != nil {
		t.Fatalf("create notification table: %v", err)
	}
	heldWriter, err := writerPool.Conn(context.Background())
	if err != nil {
		t.Fatalf("hold writer connection: %v", err)
	}
	defer heldWriter.Close()

	listener := newRiverSQLiteSplitDriver(writerPool, readerPool).GetListener(&riverdriver.GetListenenerParams{})
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	if err := listener.Connect(ctx); err != nil {
		t.Fatalf("listener connect waited on held writer: %v", err)
	}
	if err := listener.Close(context.Background()); err != nil {
		t.Fatalf("close listener: %v", err)
	}

	readerSample, ok := readerCapture.findQuery("NotificationGetLastID")
	if !ok {
		t.Fatal("notification listener did not issue its bootstrap query on the reader pool")
	}
	if readerSample.Role != catalogtx.RoleReader || readerSample.Operation != catalogtx.OperationCatalogUnknownReaderStatement {
		t.Fatalf("listener query role/operation = %s/%s, want reader/unknown_read", readerSample.Role, readerSample.Operation)
	}
	if _, ok := writerCapture.findQuery("NotificationGetLastID"); ok {
		t.Fatal("notification listener bootstrap query reached the sole writer")
	}
}

type queueStatementCapture struct {
	mu         sync.Mutex
	statements []catalogtx.StatementSample
}

func (*queueStatementCapture) ObserveTransaction(catalogtx.TransactionSample) {}

func (c *queueStatementCapture) ObserveStatement(sample catalogtx.StatementSample) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.statements = append(c.statements, sample)
}

func (c *queueStatementCapture) findQuery(name string) (catalogtx.StatementSample, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, sample := range c.statements {
		if sample.QueryName == name {
			return sample, true
		}
	}
	return catalogtx.StatementSample{}, false
}
