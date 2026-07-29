//go:build sqlite_fts5

package migrations_test

import (
	"context"
	"testing"

	"server/internal/db/sqlitespike"
	"server/migrations"

	"github.com/stretchr/testify/require"
)

func TestOCRBleveMigrationDropsOnlyOCRFTSAndSeedsOutbox(t *testing.T) {
	ctx := context.Background()
	database, err := sqlitespike.Open(ctx, t.TempDir()+"/library.sqlite3")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, database.Close()) })

	_, err = database.ExecContext(ctx, `
CREATE TABLE assets (
    asset_id TEXT PRIMARY KEY
) STRICT;
CREATE TABLE ocr_results (
    asset_id TEXT PRIMARY KEY REFERENCES assets(asset_id) ON DELETE CASCADE,
    model_id TEXT NOT NULL,
    total_count INTEGER NOT NULL DEFAULT 0 CHECK (total_count >= 0),
    processing_time_ms INTEGER,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    full_text TEXT NOT NULL DEFAULT ''
) STRICT;
CREATE TABLE ocr_text_items (
    id INTEGER PRIMARY KEY,
    asset_id TEXT NOT NULL REFERENCES ocr_results(asset_id) ON DELETE CASCADE,
    text_content TEXT NOT NULL,
    confidence REAL NOT NULL CHECK (confidence BETWEEN 0 AND 1),
    bounding_box TEXT NOT NULL CHECK (json_valid(bounding_box)),
    text_length INTEGER NOT NULL CHECK (text_length >= 0),
    area_pixels REAL,
    created_at INTEGER NOT NULL
) STRICT;
CREATE VIRTUAL TABLE ocr_search_fts USING fts5(
    full_text,
    content = 'ocr_results',
    content_rowid = 'rowid',
    tokenize = 'trigram'
);
CREATE VIRTUAL TABLE location_search_fts USING fts5(label);
CREATE TRIGGER ocr_search_fts_insert AFTER INSERT ON ocr_results BEGIN
    INSERT INTO ocr_search_fts (rowid, full_text) VALUES (new.rowid, new.full_text);
END;
CREATE TRIGGER ocr_search_fts_delete AFTER DELETE ON ocr_results BEGIN
    INSERT INTO ocr_search_fts (ocr_search_fts, rowid, full_text)
    VALUES ('delete', old.rowid, old.full_text);
END;
CREATE TRIGGER ocr_search_fts_update AFTER UPDATE OF full_text ON ocr_results BEGIN
    INSERT INTO ocr_search_fts (ocr_search_fts, rowid, full_text)
    VALUES ('delete', old.rowid, old.full_text);
    INSERT INTO ocr_search_fts (rowid, full_text) VALUES (new.rowid, new.full_text);
END;
INSERT INTO assets (asset_id)
VALUES ('00000000-0000-0000-0000-000000000003');
INSERT INTO ocr_results (
    asset_id, model_id, total_count, processing_time_ms, created_at, updated_at, full_text
) VALUES (
    '00000000-0000-0000-0000-000000000003', 'fixture', 1, 1, 1, 1, 'legacy text'
);
INSERT INTO ocr_text_items (
    asset_id, text_content, confidence, bounding_box, text_length, created_at
) VALUES (
    '00000000-0000-0000-0000-000000000003',
    'authoritative text', 0.9, '[[0,0],[1,0],[1,1],[0,1]]', 18, 1
);
`)
	require.NoError(t, err)

	migration, err := migrations.FS.ReadFile("000002_ocr_bleve.up.sql")
	require.NoError(t, err)
	_, err = database.ExecContext(ctx, string(migration))
	require.NoError(t, err)

	var count int
	require.NoError(t, database.QueryRowContext(ctx, `
SELECT count(*) FROM sqlite_schema
WHERE name = 'ocr_search_fts'
   OR name IN (
       'ocr_search_fts_insert',
       'ocr_search_fts_delete',
       'ocr_search_fts_update'
   )
`).Scan(&count))
	require.Zero(t, count)

	rows, err := database.QueryContext(ctx, "PRAGMA table_info(ocr_results)")
	require.NoError(t, err)
	defer rows.Close()
	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, columnType string
		var defaultValue any
		require.NoError(t, rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey))
		require.NotEqual(t, "full_text", name)
	}
	require.NoError(t, rows.Err())

	var revision int64
	require.NoError(t, database.QueryRowContext(ctx, `
SELECT m.revision
FROM ocr_index_metadata m
JOIN ocr_index_outbox o
  ON o.asset_id = m.asset_id AND o.revision = m.revision
WHERE m.asset_id = '00000000-0000-0000-0000-000000000003'
`).Scan(&revision))
	require.Equal(t, int64(1), revision)

	require.NoError(t, database.QueryRowContext(ctx, `
SELECT count(*) FROM sqlite_schema
WHERE type = 'table' AND name = 'location_search_fts'
`).Scan(&count))
	require.Equal(t, 1, count)
}
