//go:build sqlite_fts5

package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"server/config"
	"server/internal/db"
	"server/internal/testutil"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestAuthorizedLibraryReadOCRDocumentsIsOwnerScopedAndOrdered(t *testing.T) {
	catalog, ids := openAuthorizedLibraryOCRTestCatalog(t)
	ctx := context.Background()
	library := NewAuthorizedLibraryFactory(catalog.Queries, nil, catalog.SQL).ForUser(1)

	rows, err := library.ReadOCRDocuments(ctx, []uuid.UUID{
		ids.ownerPhoto,
		ids.otherOwnerPhoto,
		ids.deletedPhoto,
		ids.ownerVideo,
	})
	require.NoError(t, err)

	byAsset := make(map[uuid.UUID][]string)
	for _, row := range rows {
		byAsset[row.AssetID] = append(byAsset[row.AssetID], row.TextContent)
	}
	require.Equal(t, []string{"first provider line", "second provider line"}, byAsset[ids.ownerPhoto])
	require.Equal(t, []string{""}, byAsset[ids.ownerVideo])
	require.NotContains(t, byAsset, ids.otherOwnerPhoto)
	require.NotContains(t, byAsset, ids.deletedPhoto)

	for _, row := range rows {
		if row.AssetID == ids.ownerVideo {
			require.Zero(t, row.HasOcrResult)
			require.Zero(t, row.RegionCount)
		}
	}
}

type authorizedLibraryOCRIDs struct {
	ownerPhoto      uuid.UUID
	otherOwnerPhoto uuid.UUID
	deletedPhoto    uuid.UUID
	ownerVideo      uuid.UUID
}

func openAuthorizedLibraryOCRTestCatalog(t *testing.T) (*db.DB, authorizedLibraryOCRIDs) {
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
	ids := authorizedLibraryOCRIDs{
		ownerPhoto:      uuid.New(),
		otherOwnerPhoto: uuid.New(),
		deletedPhoto:    uuid.New(),
		ownerVideo:      uuid.New(),
	}
	_, err = catalog.SQL.ExecContext(ctx, `
INSERT INTO users (
    user_id, username, password, created_at, updated_at, webauthn_user_handle
) VALUES
    (1, 'ocr-owner', 'hash', 1, 1, x'01'),
    (2, 'other-owner', 'hash', 1, 1, x'02');
INSERT INTO repository_roots (
    root_id, name, path, kind, created_at, updated_at
) VALUES (?, 'root', '/media', 'default', 1, 1);
INSERT INTO repositories (
    repo_id, name, path, created_at, updated_at, default_owner_id, root_id
) VALUES (?, 'repo', '/media/repo', 1, 1, 1, ?);
`, rootID, repositoryID, rootID)
	require.NoError(t, err)
	for _, fixture := range []struct {
		id, filename, assetType, mime string
		ownerID                       int32
		deleted                       bool
	}{
		{id: ids.ownerPhoto.String(), filename: "owned.jpg", assetType: "PHOTO", mime: "image/jpeg", ownerID: 1},
		{id: ids.otherOwnerPhoto.String(), filename: "secret.jpg", assetType: "PHOTO", mime: "image/jpeg", ownerID: 2},
		{id: ids.deletedPhoto.String(), filename: "deleted.jpg", assetType: "PHOTO", mime: "image/jpeg", ownerID: 1, deleted: true},
		{id: ids.ownerVideo.String(), filename: "clip.mp4", assetType: "VIDEO", mime: "video/mp4", ownerID: 1},
	} {
		_, err = testutil.InsertAssetOccurrence(ctx, catalog.SQL, testutil.AssetOccurrenceParams{
			AssetID: uuid.MustParse(fixture.id), RepositoryID: repositoryID, OwnerID: fixture.ownerID,
			AssetType: fixture.assetType, Filename: fixture.filename, MIMEType: fixture.mime,
			FileSize: 1, IsDeleted: fixture.deleted,
		})
		require.NoError(t, err)
	}
	_, err = catalog.SQL.ExecContext(ctx, `
INSERT INTO ocr_results (
    asset_id, model_id, total_count, processing_time_ms, created_at, updated_at
) VALUES
    (?, 'fixture', 2, 1, 1, 1),
    (?, 'fixture', 1, 1, 1, 1),
    (?, 'fixture', 1, 1, 1, 1);
INSERT INTO ocr_text_items (
    asset_id, text_content, confidence, bounding_box, text_length, area_pixels, created_at
) VALUES
    (?, 'first provider line', 0.40, '[[0,0],[1,0],[1,1],[0,1]]', 19, 1, 1),
    (?, 'second provider line', 0.99, '[[0,2],[1,2],[1,3],[0,3]]', 20, 1, 2),
    (?, 'other owner secret', 0.99, '[[0,0],[1,0],[1,1],[0,1]]', 18, 1, 1),
    (?, 'deleted secret', 0.99, '[[0,0],[1,0],[1,1],[0,1]]', 14, 1, 1);
`,
		ids.ownerPhoto, ids.otherOwnerPhoto, ids.deletedPhoto,
		ids.ownerPhoto, ids.ownerPhoto, ids.otherOwnerPhoto, ids.deletedPhoto,
	)
	require.NoError(t, err)
	return catalog, ids
}
