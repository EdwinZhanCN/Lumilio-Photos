//go:build sqlite_fts5

package sqlitespike

import (
	"context"
	"testing"
)

func TestSQLiteFTS5(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := openTestDatabase(t, ctx)

	if _, err := database.ExecContext(ctx, "CREATE VIRTUAL TABLE documents USING fts5(body, tokenize='trigram')"); err != nil {
		t.Fatalf("create FTS5 table: %v", err)
	}
	if _, err := database.ExecContext(ctx, `
		INSERT INTO documents (rowid, body)
		VALUES (1, 'quiet lake at sunrise'), (2, 'city lights after rain')
	`); err != nil {
		t.Fatalf("insert FTS5 fixtures: %v", err)
	}

	var rowID int64
	if err := database.QueryRowContext(ctx, `
		SELECT rowid
		FROM documents
		WHERE documents MATCH 'sunrise'
		ORDER BY bm25(documents)
		LIMIT 1
	`).Scan(&rowID); err != nil {
		t.Fatalf("query FTS5: %v", err)
	}
	if rowID != 1 {
		t.Fatalf("FTS5 rowid = %d, want 1", rowID)
	}
}
