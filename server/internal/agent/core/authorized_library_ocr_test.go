//go:build sqlite_fts5

package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"server/config"
	"server/internal/db"

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
INSERT INTO assets (
    asset_id, owner_id, type, original_filename, mime_type, file_size,
    content_hash, upload_time, repository_id, is_deleted, updated_at
) VALUES
    (?, 1, 'PHOTO', 'owned.jpg', 'image/jpeg', 1, 'owned', 1, ?, false, 1),
    (?, 2, 'PHOTO', 'secret.jpg', 'image/jpeg', 1, 'secret', 1, ?, false, 1),
    (?, 1, 'PHOTO', 'deleted.jpg', 'image/jpeg', 1, 'deleted', 1, ?, true, 1),
    (?, 1, 'VIDEO', 'clip.mp4', 'video/mp4', 1, 'video', 1, ?, false, 1);
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
		rootID, repositoryID, rootID,
		ids.ownerPhoto, repositoryID,
		ids.otherOwnerPhoto, repositoryID,
		ids.deletedPhoto, repositoryID,
		ids.ownerVideo, repositoryID,
		ids.ownerPhoto, ids.otherOwnerPhoto, ids.deletedPhoto,
		ids.ownerPhoto, ids.ownerPhoto, ids.otherOwnerPhoto, ids.deletedPhoto,
	)
	require.NoError(t, err)
	return catalog, ids
}
