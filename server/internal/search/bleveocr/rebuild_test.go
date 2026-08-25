//go:build sqlite_fts5

package bleveocr

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"server/config"
	"server/internal/db"
	"server/internal/db/repo"
	"server/internal/testutil"

	"github.com/blevesearch/bleve/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestMissingCorruptAndMismatchedIndexesRebuildFromSQLite(t *testing.T) {
	ctx := context.Background()
	database, assetID := openRebuildTestDatabase(t, "北京 rebuild source")

	index, err := Open(ctx, database.Path, database.Queries, false, nil)
	require.NoError(t, err)
	require.Equal(t, []string{assetID.String()}, searchIDs(t, index, "北京"))
	path := index.Path()
	require.NoError(t, index.Close())

	require.NoError(t, os.RemoveAll(path))
	require.NoError(t, os.WriteFile(path, []byte("corrupt"), 0o600))
	index, err = Open(ctx, database.Path, database.Queries, false, nil)
	require.NoError(t, err)
	require.Equal(t, []string{assetID.String()}, searchIDs(t, index, "北京"))
	require.NoError(t, index.Close())

	raw, err := bleve.Open(path)
	require.NoError(t, err)
	require.NoError(t, raw.SetInternal(mappingVersionKey, []byte("obsolete")))
	require.NoError(t, raw.Close())
	index, err = Open(ctx, database.Path, database.Queries, false, nil)
	require.NoError(t, err)
	require.Equal(t, []string{assetID.String()}, searchIDs(t, index, "北京"))
	require.NoError(t, index.Close())
}

func TestForcedRebuildReplacesHealthyIndexFromCurrentSQLite(t *testing.T) {
	ctx := context.Background()
	database, assetID := openRebuildTestDatabase(t, "old wording")
	index, err := Open(ctx, database.Path, database.Queries, false, nil)
	require.NoError(t, err)
	require.NoError(t, index.Close())

	replaceOCRText(t, database, assetID, "restored database wording")
	index, err = Open(ctx, database.Path, database.Queries, true, nil)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, index.Close()) })
	require.Equal(t, []string{assetID.String()}, searchIDs(t, index, "restored"))
	require.Empty(t, searchIDs(t, index, "old"))
}

func TestOutboxCrashAfterBleveBatchRetriesIdempotently(t *testing.T) {
	ctx := context.Background()
	database, assetID := openRebuildTestDatabase(t, "initial text")
	index, err := Open(ctx, database.Path, database.Queries, false, nil)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, index.Close()) })

	replaceOCRText(t, database, assetID, "updated crash recovery")
	writer := NewWriter(database.SQL, database.Writer, database.Queries, index)
	crash := errors.New("simulated crash before SQLite acknowledgement")
	writer.afterApply = func() error { return crash }
	_, err = writer.ProcessBatch(ctx, 16)
	require.ErrorIs(t, err, crash)
	require.Equal(t, []string{assetID.String()}, searchIDs(t, index, "recovery"))
	require.Equal(t, 1, outboxCount(t, database))

	writer.afterApply = nil
	processed, err := writer.ProcessBatch(ctx, 16)
	require.NoError(t, err)
	require.Equal(t, 1, processed)
	require.Equal(t, 0, outboxCount(t, database))
	require.Equal(t, []string{assetID.String()}, searchIDs(t, index, "recovery"))
}

func openRebuildTestDatabase(t *testing.T, text string) (*db.DB, uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	dir := t.TempDir()
	require.NoError(t, os.Chmod(dir, 0o700))
	database, err := db.Open(ctx, config.DatabaseConfig{Path: filepath.Join(dir, "library.sqlite3")})
	require.NoError(t, err)
	require.NoError(t, database.Migrate(ctx))
	t.Cleanup(func() { require.NoError(t, database.Close(context.Background())) })

	rootID := uuid.New()
	repositoryID := uuid.New()
	assetID := uuid.New()
	_, err = database.SQL.ExecContext(ctx, `
INSERT INTO users (
    user_id, username, password, created_at, updated_at, webauthn_user_handle
) VALUES (1, 'ocr-owner', 'hash', 1, 1, x'01');
INSERT INTO repository_roots (
    root_id, name, path, kind, created_at, updated_at
) VALUES (?, 'root', '/media', 'default', 1, 1);
INSERT INTO repositories (
    repo_id, name, path, created_at, updated_at, default_owner_id, root_id
) VALUES (?, 'repo', '/media/repo', 1, 1, 1, ?);
`, rootID, repositoryID, rootID)
	require.NoError(t, err)
	_, err = testutil.InsertAssetOccurrence(ctx, database.SQL, testutil.AssetOccurrenceParams{
		AssetID: assetID, RepositoryID: repositoryID, OwnerID: 1,
		AssetType: "PHOTO", Filename: "ocr.jpg", MIMEType: "image/jpeg", FileSize: 1,
	})
	require.NoError(t, err)
	_, err = database.SQL.ExecContext(ctx, `
INSERT INTO ocr_results (
    asset_id, model_id, total_count, processing_time_ms, created_at, updated_at
) VALUES (?, 'fixture', 1, 1, 1, 1);
INSERT INTO ocr_text_items (
    asset_id, text_content, confidence, bounding_box, text_length, created_at
) VALUES (?, ?, 0.9, '[[0,0],[1,0],[1,1],[0,1]]', length(?), 1);
INSERT INTO ocr_index_metadata (asset_id, revision, updated_at)
VALUES (?, 1, 1);
INSERT INTO ocr_index_outbox (asset_id, revision, updated_at)
VALUES (?, 1, 1);
`, assetID, assetID, text, text, assetID, assetID)
	require.NoError(t, err)
	return database, assetID
}

func replaceOCRText(t *testing.T, database *db.DB, assetID uuid.UUID, text string) {
	t.Helper()
	ctx := context.Background()
	tx, err := database.SQL.BeginTx(ctx, nil)
	require.NoError(t, err)
	queries := database.Queries.WithTx(tx)
	require.NoError(t, queries.DeleteOCRResultByAsset(ctx, assetID))
	_, err = queries.CreateOCRResult(ctx, repo.CreateOCRResultParams{
		AssetID:    assetID,
		ModelID:    "fixture",
		TotalCount: 1,
	})
	require.NoError(t, err)
	_, err = queries.CreateOCRTextItem(ctx, repo.CreateOCRTextItemParams{
		AssetID:     assetID,
		TextContent: text,
		Confidence:  0.9,
		BoundingBox: []byte(`[[0,0],[1,0],[1,1],[0,1]]`),
		TextLength:  int64(len(text)),
	})
	require.NoError(t, err)
	revision, err := queries.BumpOCRIndexRevision(ctx, assetID)
	require.NoError(t, err)
	require.NoError(t, queries.UpsertOCRIndexOutbox(ctx, repo.UpsertOCRIndexOutboxParams{
		AssetID:  assetID,
		Revision: revision,
	}))
	require.NoError(t, tx.Commit())
}

func searchIDs(t *testing.T, index *Index, text string) []string {
	t.Helper()
	page, err := index.SearchPage(context.Background(), text, BasicFilters{}, QueryStrict, 0, 10)
	require.NoError(t, err)
	return hitIDs(page.Hits)
}

func outboxCount(t *testing.T, database *db.DB) int {
	t.Helper()
	var count int
	require.NoError(t, database.SQL.QueryRow("SELECT count(*) FROM ocr_index_outbox").Scan(&count))
	return count
}
