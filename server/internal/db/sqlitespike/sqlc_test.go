package sqlitespike

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"server/internal/db/dbtypes"
	sqlcfixture "server/internal/db/sqlitespike/sqlcfixture/gen"
)

func TestSQLiteSQLCTypeMappingRoundTrip(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := openTestDatabase(t, ctx)
	if _, err := database.ExecContext(ctx, `
		CREATE TABLE spike_records (
			id TEXT PRIMARY KEY,
			optional_parent_id TEXT,
			created_at INTEGER NOT NULL,
			payload TEXT NOT NULL CHECK (json_valid(payload)),
			embedding BLOB NOT NULL
		) STRICT
	`); err != nil {
		t.Fatalf("create sqlc fixture table: %v", err)
	}

	queries := sqlcfixture.New(database)
	id := uuid.New()
	createdAt := dbtypes.NewTimestamp(time.Date(2026, time.July, 25, 18, 15, 0, 123456789, time.UTC))
	payload := dbtypes.JSON(`{"source":"sqlc-spike"}`)
	embedding := []byte{0, 1, 2, 3, 4}

	inserted, err := queries.InsertSpikeRecord(ctx, sqlcfixture.InsertSpikeRecordParams{
		ID:               id,
		OptionalParentID: uuid.NullUUID{},
		CreatedAt:        createdAt,
		Payload:          payload,
		Embedding:        embedding,
	})
	if err != nil {
		t.Fatalf("InsertSpikeRecord() error = %v", err)
	}
	if inserted.ID != id {
		t.Fatalf("inserted ID = %s, want %s", inserted.ID, id)
	}
	if inserted.OptionalParentID.Valid {
		t.Fatalf("inserted OptionalParentID = %+v, want NULL", inserted.OptionalParentID)
	}
	if !inserted.CreatedAt.Time.Equal(createdAt.Time) {
		t.Fatalf("inserted CreatedAt = %s, want %s", inserted.CreatedAt.Time, createdAt.Time)
	}
	if !bytes.Equal(inserted.Payload, payload) {
		t.Fatalf("inserted Payload = %s, want %s", inserted.Payload, payload)
	}
	if !bytes.Equal(inserted.Embedding, embedding) {
		t.Fatalf("inserted Embedding = %v, want %v", inserted.Embedding, embedding)
	}

	loaded, err := queries.GetSpikeRecord(ctx, id)
	if err != nil {
		t.Fatalf("GetSpikeRecord() error = %v", err)
	}
	if loaded.ID != id || !loaded.CreatedAt.Time.Equal(createdAt.Time) {
		t.Fatalf("loaded record = %+v, want ID %s at %s", loaded, id, createdAt.Time)
	}
}
