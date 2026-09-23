package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"server/config"
	"server/internal/api/dto"
	"server/internal/db"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/service"
	"server/internal/storage"
	"server/internal/testutil"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type storageViewCountRepositoryManager struct {
	storage.RepositoryManager
	repositories []*repo.Repository
}

func (stub storageViewCountRepositoryManager) ListStorageLocations(context.Context) ([]repo.StorageLocation, error) {
	return nil, nil
}

func (stub storageViewCountRepositoryManager) ListRepositories() ([]*repo.Repository, error) {
	return stub.repositories, nil
}

// The storage view counts a Repository's completed, non-deleted Assets against
// the real catalog schema, so the state parameter binding and the state name
// the commit coordinator writes are both exercised.
func TestStorageViewCountsCompletedRepositoryAssets(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx := context.Background()
	dir := t.TempDir()
	require.NoError(t, os.Chmod(dir, 0o700))
	catalog, err := db.Open(ctx, config.DatabaseConfig{Path: filepath.Join(dir, "library.sqlite3")})
	require.NoError(t, err)
	require.NoError(t, catalog.Migrate(ctx))
	t.Cleanup(func() { require.NoError(t, catalog.Close(context.Background())) })

	storageLocationID := uuid.New()
	repositoryID := uuid.New()
	otherRepositoryID := uuid.New()
	_, err = catalog.SQL.ExecContext(ctx, `
INSERT INTO users (
    user_id, username, password, created_at, updated_at, webauthn_user_handle
) VALUES (1, 'storage-owner', 'hash', 1, 1, x'01');
INSERT INTO storage_locations (
    storage_location_id, name, path, kind, created_at, updated_at
) VALUES (?, 'root', '/media', 'default', 1, 1);
INSERT INTO repositories (
    repo_id, name, path, created_at, updated_at, default_owner_id, storage_location_id
) VALUES (?, 'counted', '/media/counted', 1, 1, 1, ?), (?, 'other', '/media/other', 1, 1, 1, ?);
`, storageLocationID, repositoryID, storageLocationID, otherRepositoryID, storageLocationID)
	require.NoError(t, err)

	for _, seed := range []struct {
		repositoryID uuid.UUID
		status       string
		deleted      bool
	}{
		{repositoryID, `{"state":"completed"}`, false},
		{repositoryID, `{"state":"completed"}`, false},
		{repositoryID, `{"state":"processing","message":"Pending processing"}`, false},
		{repositoryID, `{"state":"failed"}`, false},
		{repositoryID, `{"state":"completed"}`, true},
		{otherRepositoryID, `{"state":"completed"}`, false},
	} {
		_, err := testutil.InsertAssetOccurrence(ctx, catalog.SQL, testutil.AssetOccurrenceParams{
			AssetID: uuid.New(), RepositoryID: seed.repositoryID, OwnerID: 1,
			MIMEType: "image/jpeg", FileSize: 1, Status: seed.status, IsDeleted: seed.deleted,
		})
		require.NoError(t, err)
	}

	handler := NewStorageHandler(storageViewCountRepositoryManager{
		repositories: []*repo.Repository{{
			RepoID:            repositoryID,
			Name:              "counted",
			Path:              "/media/counted",
			Role:              dbtypes.RepoRoleRegular,
			StorageLocationID: storageLocationID,
			Reachability:      dbtypes.RepositoryReachabilityActive,
			Activity:          dbtypes.RepositoryActivityIdle,
		}},
	}, catalog.ReaderQueries, nil)

	recorder := httptest.NewRecorder()
	requestContext, _ := gin.CreateTestContext(recorder)
	requestContext.Request = httptest.NewRequest(http.MethodGet, "/api/v1/storage/view", nil)
	requestContext.Set("current_user", &service.UserResponse{UserID: 1, Role: "admin"})

	handler.GetStorageView(requestContext)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())

	var response dto.StorageViewResponseDTO
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Len(t, response.Repositories, 1)
	require.NotNil(t, response.Repositories[0].AssetCount, "asset_count must be present")
	require.Equal(t, int64(2), *response.Repositories[0].AssetCount)
}
