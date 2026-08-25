//go:build sqlite_fts5

package search

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"server/config"
	"server/internal/db"
	"server/internal/search/bleveocr"
	"server/internal/testutil"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestBleveOCRRetrieverStrictRelaxedAuthorizationAndSQLitePostFilter(t *testing.T) {
	ctx := context.Background()
	database := openRetrieverTestDatabase(t)
	repositoryA := uuid.New()
	repositoryB := uuid.New()
	insertRetrieverRepositories(t, database, repositoryA, repositoryB)

	strict := insertRetrieverOCRAsset(t, database, 1, repositoryA, "red bicycle", false)
	relaxedRed := insertRetrieverOCRAsset(t, database, 1, repositoryA, "red car", false)
	relaxedBicycle := insertRetrieverOCRAsset(t, database, 1, repositoryA, "blue bicycle", false)
	unauthorizedOwner := insertRetrieverOCRAsset(t, database, 2, repositoryA, "red bicycle", false)
	wrongRepository := insertRetrieverOCRAsset(t, database, 1, repositoryB, "red bicycle", false)
	deleted := insertRetrieverOCRAsset(t, database, 1, repositoryA, "red bicycle", true)

	_, err := database.SQL.ExecContext(ctx, `
INSERT INTO albums (album_id, user_id, album_name, created_at, updated_at)
VALUES (1, 1, 'post filter', 1, 1);
INSERT INTO album_assets (album_id, asset_id, position, added_time)
VALUES (1, ?, 0, 1);
`, relaxedBicycle)
	require.NoError(t, err)

	index, err := bleveocr.Open(ctx, database.Path, database.Queries, false, nil)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, index.Close()) })
	retriever := NewBleveOCRRetriever(database.SQL, index, 0.7)

	ownerID := int32(1)
	candidates, err := retriever.Retrieve(ctx, Request{
		Query: "red bicycle",
		TopK:  3,
		Filter: Filter{
			OwnerID:      &ownerID,
			RepositoryID: &repositoryA,
		},
	})
	require.NoError(t, err)
	require.Len(t, candidates, 3)
	require.Equal(t, strict, candidates[0].AssetID)
	require.ElementsMatch(t, []uuid.UUID{relaxedRed, relaxedBicycle}, candidateUUIDs(candidates[1:]))
	require.NotContains(t, candidateUUIDs(candidates), unauthorizedOwner)
	require.NotContains(t, candidateUUIDs(candidates), wrongRepository)

	albumID := int32(1)
	candidates, err = retriever.Retrieve(ctx, Request{
		Query: "bicycle",
		TopK:  10,
		Filter: Filter{
			OwnerID:      &ownerID,
			RepositoryID: &repositoryA,
			AlbumID:      &albumID,
		},
	})
	require.NoError(t, err)
	require.Equal(t, []uuid.UUID{relaxedBicycle}, candidateUUIDs(candidates))

	isDeleted := true
	candidates, err = retriever.Retrieve(ctx, Request{
		Query: "red bicycle",
		TopK:  10,
		Filter: Filter{
			OwnerID:      &ownerID,
			RepositoryID: &repositoryA,
			IsDeleted:    &isDeleted,
		},
	})
	require.NoError(t, err)
	require.Equal(t, []uuid.UUID{deleted}, candidateUUIDs(candidates))
}

func openRetrieverTestDatabase(t *testing.T) *db.DB {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.Chmod(dir, 0o700))
	database, err := db.Open(context.Background(), config.DatabaseConfig{
		Path: filepath.Join(dir, "library.sqlite3"),
	})
	require.NoError(t, err)
	require.NoError(t, database.Migrate(context.Background()))
	t.Cleanup(func() { require.NoError(t, database.Close(context.Background())) })
	_, err = database.SQL.Exec(`
INSERT INTO users (
    user_id, username, password, created_at, updated_at, webauthn_user_handle
) VALUES
    (1, 'owner-one', 'hash', 1, 1, x'01'),
    (2, 'owner-two', 'hash', 1, 1, x'02');
`)
	require.NoError(t, err)
	return database
}

func insertRetrieverRepositories(t *testing.T, database *db.DB, repositoryA, repositoryB uuid.UUID) {
	t.Helper()
	rootID := uuid.New()
	_, err := database.SQL.Exec(`
INSERT INTO repository_roots (
    root_id, name, path, kind, created_at, updated_at
) VALUES (?, 'root', '/media', 'default', 1, 1);
INSERT INTO repositories (
    repo_id, name, path, created_at, updated_at, default_owner_id, root_id
) VALUES
    (?, 'repo-a', '/media/a', 1, 1, 1, ?),
    (?, 'repo-b', '/media/b', 1, 1, 1, ?);
`, rootID, repositoryA, rootID, repositoryB, rootID)
	require.NoError(t, err)
}

func insertRetrieverOCRAsset(
	t *testing.T,
	database *db.DB,
	ownerID int32,
	repositoryID uuid.UUID,
	text string,
	isDeleted bool,
) uuid.UUID {
	t.Helper()
	assetID := uuid.New()
	_, err := testutil.InsertAssetOccurrence(context.Background(), database.SQL, testutil.AssetOccurrenceParams{
		AssetID: assetID, RepositoryID: repositoryID, OwnerID: ownerID,
		AssetType: "PHOTO", Filename: assetID.String() + ".jpg", MIMEType: "image/jpeg",
		FileSize: 1, IsDeleted: isDeleted,
	})
	require.NoError(t, err)
	_, err = database.SQL.Exec(`
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
	return assetID
}

func candidateUUIDs(candidates []Candidate) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(candidates))
	for _, candidate := range candidates {
		ids = append(ids, candidate.AssetID)
	}
	return ids
}
