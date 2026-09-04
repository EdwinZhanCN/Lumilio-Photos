//go:build sqlite_fts5

package dto

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"server/config"
	"server/internal/db"
	"server/internal/testutil"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestAssetDetailOCRPreservesProviderInsertionOrder(t *testing.T) {
	catalog, assetID := openAssetDetailOCRTestCatalog(t)
	ctx := context.Background()

	_, err := catalog.SQL.ExecContext(ctx, `
INSERT INTO ocr_results (
    asset_id, model_id, total_count, processing_time_ms, created_at, updated_at
) VALUES (?, 'fixture', 3, 9, 1, 1);
INSERT INTO ocr_text_items (
    asset_id, text_content, confidence, bounding_box, text_length, area_pixels, created_at
) VALUES
    (?, 'first provider line', 0.51, '[[0,0],[1,0],[1,1],[0,1]]', 19, 1, 1),
    (?, 'second provider line', 0.99, '[[0,2],[1,2],[1,3],[0,3]]', 20, 1, 2),
    (?, 'third provider line', 0.75, '[[0,4],[1,4],[1,5],[0,5]]', 19, 1, 3);
`, assetID, assetID, assetID, assetID)
	require.NoError(t, err)

	row, err := catalog.Queries.GetAssetWithRelations(ctx, assetID)
	require.NoError(t, err)
	detail := ToAssetDetailDTO(row, AssetDetailIncludes{OCR: true})
	require.NotNil(t, detail.OcrResult)
	require.Equal(t, time.UnixMicro(1).UTC(), *detail.OcrResult.CreatedAt)
	require.Equal(t, time.UnixMicro(1).UTC(), *detail.OcrResult.UpdatedAt)
	require.Equal(t, []string{
		"first provider line",
		"second provider line",
		"third provider line",
	}, ocrTextContents(detail.OcrResult.TextItems))
}

func TestAssetDetailOCRDistinguishesMissingAndZeroItemResults(t *testing.T) {
	catalog, assetID := openAssetDetailOCRTestCatalog(t)
	ctx := context.Background()

	row, err := catalog.Queries.GetAssetWithRelations(ctx, assetID)
	require.NoError(t, err)
	detail := ToAssetDetailDTO(row, AssetDetailIncludes{OCR: true})
	require.Nil(t, detail.OcrResult)

	_, err = catalog.SQL.ExecContext(ctx, `
INSERT INTO ocr_results (
    asset_id, model_id, total_count, processing_time_ms, created_at, updated_at
) VALUES (?, 'fixture', 0, 3, 1, 1)
`, assetID)
	require.NoError(t, err)

	row, err = catalog.Queries.GetAssetWithRelations(ctx, assetID)
	require.NoError(t, err)
	detail = ToAssetDetailDTO(row, AssetDetailIncludes{OCR: true})
	require.NotNil(t, detail.OcrResult)
	require.Empty(t, detail.OcrResult.TextItems)
}

func ocrTextContents(items []AssetOCRTextItemDTO) []string {
	contents := make([]string, len(items))
	for i, item := range items {
		contents[i] = item.TextContent
	}
	return contents
}

func openAssetDetailOCRTestCatalog(t *testing.T) (*db.DB, uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	dir := t.TempDir()
	require.NoError(t, os.Chmod(dir, 0o700))
	catalog, err := db.Open(ctx, config.DatabaseConfig{
		Path: filepath.Join(dir, "library.sqlite3"),
	})
	require.NoError(t, err)
	require.NoError(t, catalog.Migrate(ctx))
	t.Cleanup(func() { require.NoError(t, catalog.Close(context.Background())) })

	rootID := uuid.New()
	repositoryID := uuid.New()
	assetID := uuid.New()
	_, err = catalog.SQL.ExecContext(ctx, `
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
	_, err = testutil.InsertAssetOccurrence(ctx, catalog.SQL, testutil.AssetOccurrenceParams{
		AssetID: assetID, RepositoryID: repositoryID, OwnerID: 1,
		AssetType: "PHOTO", Filename: "ocr.jpg", MIMEType: "image/jpeg", FileSize: 1,
	})
	require.NoError(t, err)
	return catalog, assetID
}
