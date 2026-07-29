package queue

import (
	"context"
	"database/sql"
	"reflect"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func TestPendingEventOwnerIDsReleasesSingleConnection(t *testing.T) {
	t.Parallel()

	db, err := sql.Open("sqlite3", "file:event-scheduler-test?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`
CREATE TABLE event_dirty_ranges (
 owner_id INTEGER NOT NULL,
 claim_token TEXT
);
INSERT INTO event_dirty_ranges(owner_id,claim_token)
VALUES (2,NULL),(1,NULL),(2,NULL),(3,'claimed')`); err != nil {
		t.Fatal(err)
	}

	ownerIDs, err := pendingEventOwnerIDs(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	if want := []int32{1, 2}; !reflect.DeepEqual(ownerIDs, want) {
		t.Fatalf("pending owners = %v, want %v", ownerIDs, want)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := db.ExecContext(ctx, `INSERT INTO event_dirty_ranges(owner_id) VALUES (4)`); err != nil {
		t.Fatalf("single connection remained occupied after owner scan: %v", err)
	}
}
