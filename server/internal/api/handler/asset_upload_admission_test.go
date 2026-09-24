package handler

import (
	"context"
	"database/sql"
	"net/http"
	"testing"

	"server/internal/api/dto"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/storage"
	"server/internal/utils/hash"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestResolveUploadRepositoryAdmission(t *testing.T) {
	database, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	defer database.Close()
	database.SetMaxOpenConns(1)
	for _, stmt := range []string{
		`CREATE TABLE repositories(repo_id TEXT PRIMARY KEY, name TEXT, path TEXT, config TEXT, reachability TEXT, activity TEXT, pause_reason TEXT, last_sync INTEGER, created_at INTEGER, updated_at INTEGER, default_owner_id INTEGER, role TEXT, storage_location_id TEXT)`,
		`CREATE TABLE assets(asset_id TEXT,original_filename TEXT,content_id TEXT)`,
		`CREATE TABLE content_objects(content_id TEXT,full_hash TEXT,file_size INTEGER)`,
		`CREATE TABLE active_asset_occurrences(asset_id TEXT,repository_id TEXT,quick_fingerprint TEXT,file_size INTEGER)`,
	} {
		_, err = database.Exec(stmt)
		require.NoError(t, err)
	}

	insertRepo := func(reachability, activity, pauseReason string) string {
		id := uuid.NewString()
		_, err := database.Exec(
			`INSERT INTO repositories VALUES (?, ?, '/tmp/repo', '{}', ?, ?, ?, 0, 0, 0, 1, 'regular', ?)`,
			id, "test-"+reachability+"-"+activity, reachability, activity, pauseReason, uuid.NewString(),
		)
		require.NoError(t, err)
		return id
	}

	handler := &AssetHandler{queries: repo.New(database)}
	ctx := context.Background()
	version := hash.QuickFingerprintVersion

	t.Run("manual pause refuses upload", func(t *testing.T) {
		repositoryID := insertRepo(string(dbtypes.RepositoryReachabilityActive), string(dbtypes.RepositoryActivityPaused), "manual")
		_, err := handler.resolveUploadRepository(ctx, repositoryID)
		require.ErrorIs(t, err, storage.ErrRepositoryUploadPaused)
	})

	t.Run("scanning allows upload precheck", func(t *testing.T) {
		repositoryID := insertRepo(string(dbtypes.RepositoryReachabilityActive), string(dbtypes.RepositoryActivityScanning), "")
		resolved, err := handler.resolveUploadRepository(ctx, repositoryID)
		require.NoError(t, err)
		require.Equal(t, repositoryID, resolved.RepoID.String())

		h := &AssetHandler{queries: repo.New(database)}
		response := postJSON(t, "/api/v1/assets/precheck", dto.UploadPrecheckRequestDTO{
			RepositoryID: repositoryID,
			Files:        []dto.UploadPrecheckFileDTO{{Hash: "any", Size: 1, IsQuick: true, FingerprintVersion: &version}},
		}, h.PrecheckUpload)
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	})

	t.Run("offline refuses upload", func(t *testing.T) {
		repositoryID := insertRepo(string(dbtypes.RepositoryReachabilityOffline), string(dbtypes.RepositoryActivityIdle), "")
		_, err := handler.resolveUploadRepository(ctx, repositoryID)
		require.ErrorIs(t, err, storage.ErrRepositoryOffline)
	})

	t.Run("manual pause still allows open original admission", func(t *testing.T) {
		decision := storage.OpenOriginalAdmission(repo.Repository{
			Reachability: dbtypes.RepositoryReachabilityActive,
			Activity:     dbtypes.RepositoryActivityPaused,
			PauseReason:  "manual",
		})
		require.True(t, decision.Allowed)
	})
}
