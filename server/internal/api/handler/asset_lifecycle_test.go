package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"server/config"
	"server/internal/api/problem"
	"server/internal/db"
	"server/internal/testutil"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"server/internal/db/repo"
	"server/internal/service"
	"server/internal/storage/trash"
)

type lifecycleAuthorizationService struct {
	stubAssetService
	calls int
}

func (s *lifecycleAuthorizationService) DeleteAssets(context.Context, trash.Request) (trash.DeleteResult, error) {
	s.calls++
	return trash.DeleteResult{}, nil
}
func (s *lifecycleAuthorizationService) DeleteAssetsPermanently(context.Context, trash.Request) (trash.PurgeResult, error) {
	s.calls++
	return trash.PurgeResult{}, nil
}
func (s *lifecycleAuthorizationService) RemoveMissingAssets(context.Context, trash.RemoveMissingRequest) (trash.PurgeResult, error) {
	s.calls++
	return trash.PurgeResult{}, nil
}
func (s *lifecycleAuthorizationService) EmptyAssetTrash(context.Context, trash.Request, uuid.NullUUID, *int32) (trash.PurgeResult, error) {
	s.calls++
	return trash.PurgeResult{}, nil
}

func TestLifecycleSelectionAuthorizesEveryAssetBeforeMoving(t *testing.T) {
	gin.SetMode(gin.TestMode)
	own, other := uuid.New(), uuid.New()
	serviceStub := &lifecycleAuthorizationService{stubAssetService: stubAssetService{getAssetFn: func(_ context.Context, id uuid.UUID) (*repo.Asset, error) {
		owner := int32(1)
		if id == other {
			owner = 2
		}
		return &repo.Asset{AssetID: id, OwnerID: &owner}, nil
	}}}
	h := &AssetHandler{assetService: serviceStub}
	c, w := lifecycleTestContext(`{"asset_ids":["` + own.String() + `","` + other.String() + `"]}`)
	h.TrashAssets(c)
	require.Equal(t, http.StatusForbidden, w.Code)
	require.Zero(t, serviceStub.calls, "a later unauthorized Asset must prevent the first Asset from moving")
}

func TestLifecycleIrreversibleActionsRequireConfirmation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	stub := &lifecycleAuthorizationService{}
	h := &AssetHandler{assetService: stub}
	for name, action := range map[string]func(*gin.Context){"permanent": h.DeleteAssetsPermanently, "missing": h.RemoveMissingAssets, "empty": h.EmptyAssetTrash} {
		t.Run(name, func(t *testing.T) {
			c, w := lifecycleTestContext(`{"asset_ids":["` + uuid.NewString() + `"],"confirm":false}`)
			action(c)
			require.Equal(t, http.StatusBadRequest, w.Code)
			require.Zero(t, stub.calls)
		})
	}
}

func TestLifecycleRepositoryWideActionsRequireAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	stub := &lifecycleAuthorizationService{}
	h := &AssetHandler{assetService: stub}
	for _, action := range []func(*gin.Context){h.RemoveMissingAssets, h.EmptyAssetTrash} {
		c, w := lifecycleTestContext(`{"repository_id":"` + uuid.NewString() + `","confirm":true}`)
		action(c)
		require.Equal(t, http.StatusForbidden, w.Code)
		require.Zero(t, stub.calls)
	}
}

func lifecycleTestContext(body string) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("current_user", &service.UserResponse{UserID: 1, Role: "user"})
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/assets/trash", bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")
	return c, w
}

func TestAssetAvailabilityUsesAllCopiesAndAuthorizesBeforeState(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx := context.Background()
	directory := t.TempDir()
	require.NoError(t, os.Chmod(directory, 0o700))
	catalog, err := db.Open(ctx, config.DatabaseConfig{Path: filepath.Join(directory, "catalog.sqlite3")})
	require.NoError(t, err)
	require.NoError(t, catalog.Migrate(ctx))
	t.Cleanup(func() { require.NoError(t, catalog.Close(context.Background())) })
	first, second := uuid.New(), uuid.New()
	location := uuid.New()
	assetID := uuid.New()
	_, err = catalog.SQL.ExecContext(ctx, `INSERT INTO users(user_id,username,password,created_at,updated_at,webauthn_user_handle) VALUES(1,'owner','hash',1,1,x'01');
 INSERT INTO storage_locations(storage_location_id,name,path,kind,created_at,updated_at) VALUES(?,'location','/media','external',1,1);
 INSERT INTO repositories(repo_id,name,path,created_at,updated_at,reachability,storage_location_id) VALUES(?,'first','/first',1,1,'offline',?),(?,'second','/second',1,1,'offline',?);`, location, first, location, second, location)
	require.NoError(t, err)
	seed, err := testutil.InsertAssetOccurrence(ctx, catalog.SQL, testutil.AssetOccurrenceParams{AssetID: assetID, RepositoryID: first, OwnerID: 1})
	require.NoError(t, err)
	_, err = catalog.SQL.ExecContext(ctx, `INSERT INTO repository_entries(entry_id,repository_id,path,path_key,parent_key,kind,size,mtime_ns,stat_checked_ns,state,asset_id,content_id,revision,updated_at) VALUES(?,?,'copy.jpg','copy.jpg','','file',0,1,1,'present',?,?,1,1)`, uuid.New(), second, assetID, seed.ContentID)
	require.NoError(t, err)
	h := &AssetHandler{readerDatabase: catalog.ReaderSQL, assetService: stubAssetService{getAssetFn: func(ctx context.Context, id uuid.UUID) (*repo.Asset, error) {
		asset, err := catalog.ReaderQueries.GetAssetByIDAny(ctx, id)
		return &asset, err
	}}}
	check := func(wantStatus int, wantType string) {
		c, w := lifecycleTestContext("")
		c.Params = gin.Params{{Key: "id", Value: assetID.String()}}
		h.GetAssetAvailability(c)
		require.Equal(t, wantStatus, w.Code, w.Body.String())
		if wantType != "" {
			var details problem.Details
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &details))
			require.Equal(t, wantType, details.Type)
		}
	}
	check(http.StatusConflict, problem.AssetOffline.Type)
	_, err = catalog.SQL.ExecContext(ctx, `UPDATE repositories SET reachability='active' WHERE repo_id=?`, second)
	require.NoError(t, err)
	check(http.StatusOK, "")
	require.NoError(t, testutil.SetAssetEntriesState(ctx, catalog.SQL, assetID, "missing"))
	check(http.StatusConflict, problem.AssetMissing.Type)
	require.NoError(t, testutil.SetAssetEntriesState(ctx, catalog.SQL, assetID, "trashed"))
	check(http.StatusConflict, problem.AssetTrashed.Type)
	c, w := lifecycleTestContext("")
	c.Params = gin.Params{{Key: "id", Value: assetID.String()}}
	c.Set("current_user", &service.UserResponse{UserID: 2, Role: "user"})
	h.GetAssetAvailability(c)
	require.Equal(t, http.StatusForbidden, w.Code)
}
