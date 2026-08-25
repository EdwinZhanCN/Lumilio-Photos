package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"server/config"
	"server/internal/db"
	"server/internal/db/dbtypes"
	"server/internal/testutil"
	"server/internal/utils/phash"

	"github.com/corona10/goimagehash"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestDuplicateDetectionBulkReplacesCompleteGraph(t *testing.T) {
	ctx := context.Background()
	catalogDir := filepath.Join(t.TempDir(), "private")
	require.NoError(t, os.Mkdir(catalogDir, 0o700))
	catalog, err := db.Open(ctx, config.DatabaseConfig{
		Path: filepath.Join(catalogDir, "catalog.sqlite3"),
	})
	require.NoError(t, err)
	defer catalog.Close(context.Background())
	require.NoError(t, catalog.Migrate(ctx))

	repositoryID := uuid.New()
	rootID := uuid.New()
	_, err = catalog.SQL.ExecContext(ctx, `
		INSERT INTO users (
			user_id, username, password, created_at, updated_at,
			display_name, role, webauthn_user_handle
		) VALUES (1, 'owner', 'unused', 1, 1, 'Owner', 'admin', x'01')
	`)
	require.NoError(t, err)
	_, err = catalog.SQL.ExecContext(ctx, `
		INSERT INTO repository_roots (
			root_id, name, path, kind, created_at, updated_at
		) VALUES (?, 'Root', '/test/root', 'default', 1, 1)
	`, rootID)
	require.NoError(t, err)
	_, err = catalog.SQL.ExecContext(ctx, `
		INSERT INTO repositories (
			repo_id, name, path, created_at, updated_at,
			default_owner_id, role, root_id
		) VALUES (?, 'Repository', '/test/root/repository', 1, 1, 1, 'primary', ?)
	`, repositoryID, rootID)
	require.NoError(t, err)
	_, err = catalog.SQL.ExecContext(ctx, `
		INSERT INTO embedding_spaces (
			id, embedding_type, model_id, dimensions, distance_metric,
			created_at, updated_at
		) VALUES (99, 'phash', 'dct-phash-v1', 64, 'l2', 1, 1)
	`)
	require.NoError(t, err)

	assetIDs := []uuid.UUID{uuid.New(), uuid.New()}
	for index, assetID := range assetIDs {
		_, err := testutil.InsertAssetOccurrence(ctx, catalog.SQL, testutil.AssetOccurrenceParams{
			AssetID: assetID, RepositoryID: repositoryID, OwnerID: 1,
			Filename: assetID.String() + ".jpg", FileSize: int64(100 + index),
		})
		require.NoError(t, err)
		_, err = catalog.SQL.ExecContext(ctx, `
			INSERT INTO embeddings (
				asset_id, embedding_type, embedding_model, embedding_dimensions,
				vector, is_primary, created_at, updated_at, space_id
			) VALUES (?, 'phash', 'dct-phash-v1', 64, ?, 1, 1, 1, 99)
		`, assetID, dbtypes.NewVector(phash.ToVector(
			goimagehash.NewImageHash(0x123456789abcdef0, goimagehash.PHash),
		)))
		require.NoError(t, err)
	}

	service := NewDuplicateService(catalog.Queries, catalog.SQL, catalog.Writer, nil, nil)
	for run := 0; run < 2; run++ {
		result, err := service.DetectForRepository(ctx, repositoryID)
		require.NoError(t, err)
		require.Equal(t, 1, result.Groups)
		require.Equal(t, 1, result.PHashGroups)
		require.Equal(t, 2, result.AssetsAffected)

		var groups, members, edges, keepers int
		require.NoError(t, catalog.ReaderSQL.QueryRowContext(ctx, `
			SELECT
				(SELECT count(*) FROM duplicate_groups WHERE status = 'pending'),
				(SELECT count(*) FROM duplicate_group_assets),
				(SELECT count(*) FROM duplicate_group_edges),
				(SELECT count(*) FROM duplicate_group_assets WHERE role = 'keeper')
		`).Scan(&groups, &members, &edges, &keepers))
		require.Equal(t, 1, groups)
		require.Equal(t, 2, members)
		require.Equal(t, 1, edges)
		require.Equal(t, 1, keepers)
	}
}
