//go:build sqlite_fts5

package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"server/config"
	"server/internal/db"
	"server/internal/search/bleveocr"

	"github.com/edwinzhancn/lumen-sdk/pkg/types"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestOCRSaveUpdateDeleteTrashRestoreAndAtomicRollback(t *testing.T) {
	ctx := context.Background()
	database, assetID := openOCRServiceTestDatabase(t)
	index, err := bleveocr.Open(ctx, database.Path, database.Queries, false, nil)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, index.Close()) })
	writer := bleveocr.NewWriter(database.SQL, database.Queries, index)
	ocrService := NewOCRService(database.Queries, database.SQL)
	assetService, err := NewAssetService(database.Queries, database.SQL, nil, nil, index)
	require.NoError(t, err)

	require.NoError(t, ocrService.SaveOCRResults(ctx, assetID, ocrFixture("Running invoice 2025", 0.95), 12))
	require.Equal(t, int64(1), ocrRevision(t, database, assetID))
	drainOCRWriter(t, writer)
	require.Equal(t, []string{assetID.String()}, serviceSearchIDs(t, index, "invoice", false))

	require.NoError(t, ocrService.SaveOCRResults(ctx, assetID, ocrFixture("Updated bicycle X-T5", 0.90), 8))
	require.Equal(t, int64(2), ocrRevision(t, database, assetID))
	drainOCRWriter(t, writer)
	require.Empty(t, serviceSearchIDs(t, index, "invoice", false))
	require.Equal(t, []string{assetID.String()}, serviceSearchIDs(t, index, "bicycle", false))

	err = ocrService.SaveOCRResults(ctx, assetID, ocrFixture("must roll back", 2), 1)
	require.Error(t, err)
	require.Equal(t, int64(2), ocrRevision(t, database, assetID))
	require.Equal(t, 0, serviceOutboxCount(t, database))
	items, err := database.Queries.GetOCRTextItemsByAsset(ctx, assetID)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, "Updated bicycle X-T5", items[0].TextContent)

	require.NoError(t, assetService.DeleteAsset(ctx, assetID))
	require.Equal(t, int64(3), ocrRevision(t, database, assetID))
	drainOCRWriter(t, writer)
	require.Empty(t, serviceSearchIDs(t, index, "bicycle", false))
	require.Equal(t, []string{assetID.String()}, serviceSearchIDs(t, index, "bicycle", true))

	require.NoError(t, assetService.RestoreAsset(ctx, assetID))
	require.Equal(t, int64(4), ocrRevision(t, database, assetID))
	drainOCRWriter(t, writer)
	require.Equal(t, []string{assetID.String()}, serviceSearchIDs(t, index, "bicycle", false))

	require.NoError(t, ocrService.DeleteOCRResults(ctx, assetID))
	require.Equal(t, int64(5), ocrRevision(t, database, assetID))
	drainOCRWriter(t, writer)
	require.Empty(t, serviceSearchIDs(t, index, "bicycle", false))
	_, err = database.Queries.GetOCRResultByAsset(ctx, assetID)
	require.Error(t, err)
}

func openOCRServiceTestDatabase(t *testing.T) (*db.DB, uuid.UUID) {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.Chmod(dir, 0o700))
	database, err := db.Open(context.Background(), config.DatabaseConfig{
		Path: filepath.Join(dir, "library.sqlite3"),
	})
	require.NoError(t, err)
	require.NoError(t, database.Migrate(context.Background()))
	t.Cleanup(func() { require.NoError(t, database.Close(context.Background())) })

	rootID := uuid.New()
	repositoryID := uuid.New()
	assetID := uuid.New()
	_, err = database.SQL.Exec(`
INSERT INTO users (
    user_id, username, password, created_at, updated_at, webauthn_user_handle
) VALUES (1, 'ocr-owner', 'hash', 1, 1, x'01');
INSERT INTO repository_roots (
    root_id, name, path, kind, created_at, updated_at
) VALUES (?, 'root', '/media', 'default', 1, 1);
INSERT INTO repositories (
    repo_id, name, path, created_at, updated_at, default_owner_id, root_id
) VALUES (?, 'repo', '/media/repo', 1, 1, 1, ?);
INSERT INTO assets (
    asset_id, owner_id, type, original_filename, mime_type, file_size,
    content_hash, upload_time, repository_id, updated_at
) VALUES (?, 1, 'PHOTO', 'ocr.jpg', 'image/jpeg', 1, 'hash', 1, ?, 1);
`, rootID, repositoryID, rootID, assetID, repositoryID)
	require.NoError(t, err)
	return database, assetID
}

func ocrFixture(text string, confidence float32) *types.OCRV1 {
	return &types.OCRV1{
		ModelID: "fixture",
		Count:   1,
		Items: []types.OCRItem{{
			Box:        [][]int{{0, 0}, {10, 0}, {10, 10}, {0, 10}},
			Text:       text,
			Confidence: confidence,
		}},
	}
}

func drainOCRWriter(t *testing.T, writer *bleveocr.Writer) {
	t.Helper()
	processed, err := writer.ProcessBatch(context.Background(), 16)
	require.NoError(t, err)
	require.Equal(t, 1, processed)
}

func ocrRevision(t *testing.T, database *db.DB, assetID uuid.UUID) int64 {
	t.Helper()
	var revision int64
	require.NoError(t, database.SQL.QueryRow(
		"SELECT revision FROM ocr_index_metadata WHERE asset_id = ?",
		assetID,
	).Scan(&revision))
	return revision
}

func serviceOutboxCount(t *testing.T, database *db.DB) int {
	t.Helper()
	var count int
	require.NoError(t, database.SQL.QueryRow("SELECT count(*) FROM ocr_index_outbox").Scan(&count))
	return count
}

func serviceSearchIDs(t *testing.T, index *bleveocr.Index, text string, deleted bool) []string {
	t.Helper()
	page, err := index.SearchPage(context.Background(), text, bleveocr.BasicFilters{
		IsDeleted: deleted,
	}, bleveocr.QueryStrict, 0, 10)
	require.NoError(t, err)
	ids := make([]string, 0, len(page.Hits))
	for _, hit := range page.Hits {
		ids = append(ids, hit.AssetID)
	}
	return ids
}
