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

	baseline, err := migrations.FS.ReadFile("000001_sqlite_baseline.up.sql")
	require.NoError(t, err)
	_, err = database.ExecContext(ctx, string(baseline))
	require.NoError(t, err)
	insertVectorFixtures(t, ctx, database)
	_, err = database.ExecContext(ctx, `
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
