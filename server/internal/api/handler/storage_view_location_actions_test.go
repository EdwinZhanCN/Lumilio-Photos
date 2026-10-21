package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"server/internal/api/dto"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/service"
	"server/internal/storage"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type storageViewLocationActionsManager struct {
	storage.RepositoryManager
	locations []repo.StorageLocation
	original  map[uuid.UUID]storage.OriginalIdentityState
	children  map[uuid.UUID]int64
}

func (stub storageViewLocationActionsManager) ListStorageLocations(context.Context) ([]repo.StorageLocation, error) {
	return stub.locations, nil
}

func (stub storageViewLocationActionsManager) ListRepositories() ([]*repo.Repository, error) {
	return nil, nil
}

func (stub storageViewLocationActionsManager) PreviewStorageLocationRemoval(_ context.Context, id string) (storage.StorageLocationRemovalImpact, error) {
	parsed := uuid.MustParse(id)
	impact := storage.StorageLocationRemovalImpact{StorageLocationID: id, RepositoryCount: stub.children[parsed], FilesPreserved: true}
	impact.CanRemove = impact.RepositoryCount == 0
	return impact, nil
}

func (stub storageViewLocationActionsManager) StorageLocationActionPolicyFor(_ context.Context, storageLocation repo.StorageLocation) (storage.StorageLocationActionPolicy, error) {
	return storage.DeriveStorageLocationActions(storage.StorageLocationActionFacts{
		Kind: storageLocation.Kind, Status: storageLocation.Status,
		RepositoryCount: stub.children[storageLocation.StorageLocationID],
		Original:        stub.original[storageLocation.StorageLocationID],
	}), nil
}

// The storage view carries the kind-aware Location commands next to the row
// commands: the Default Storage Location is protected on both, a positively
// unavailable external Location offers Reconnect and withholds Detach while it
// still registers Repositories, and an online external one offers neither
// reason to reconnect.
func TestStorageViewProjectsStorageLocationActions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	defaultID, movedID, onlineID := uuid.New(), uuid.New(), uuid.New()
	handler := NewStorageHandler(storageViewLocationActionsManager{
		locations: []repo.StorageLocation{
			{StorageLocationID: defaultID, Name: "Default", Kind: dbtypes.StorageLocationKindDefault, Status: dbtypes.StorageLocationStatusActive},
			{StorageLocationID: movedID, Name: "Moved", Kind: dbtypes.StorageLocationKindExternal, Status: dbtypes.StorageLocationStatusActive},
			{StorageLocationID: onlineID, Name: "Online", Kind: dbtypes.StorageLocationKindExternal, Status: dbtypes.StorageLocationStatusActive},
		},
		original: map[uuid.UUID]storage.OriginalIdentityState{
			defaultID: storage.OriginalOnline, movedID: storage.OriginalAbsent, onlineID: storage.OriginalOnline,
		},
		children: map[uuid.UUID]int64{defaultID: 1, movedID: 2},
	}, nil, nil)

	recorder := httptest.NewRecorder()
	requestContext, _ := gin.CreateTestContext(recorder)
	requestContext.Request = httptest.NewRequest(http.MethodGet, "/api/v1/storage/view", nil)
	requestContext.Set("current_user", &service.UserResponse{UserID: 1, Role: "admin"})
	handler.GetStorageView(requestContext)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())

	var response dto.StorageViewResponseDTO
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Len(t, response.StorageLocations, 3)
	byID := map[string]dto.StorageLocationViewDTO{}
	for _, location := range response.StorageLocations {
		byID[location.ID] = location
	}

	defaultView := byID[defaultID.String()]
	require.Equal(t, []string{}, defaultView.Actions)
	require.Len(t, defaultView.UnavailableActions, 2)
	for _, blocked := range defaultView.UnavailableActions {
		require.Contains(t, blocked.Blockers, "default_protected", blocked.Action)
		require.Contains(t, blocked.Alternatives, "default_configuration", blocked.Action)
	}

	movedView := byID[movedID.String()]
	require.Equal(t, []string{"reconnect"}, movedView.Actions)
	require.Len(t, movedView.UnavailableActions, 1)
	require.Equal(t, "detach", movedView.UnavailableActions[0].Action)
	require.Equal(t, []string{"registered_repositories"}, movedView.UnavailableActions[0].Blockers)
	require.False(t, movedView.CanRemove)

	onlineView := byID[onlineID.String()]
	require.Equal(t, []string{"detach"}, onlineView.Actions)
	require.Len(t, onlineView.UnavailableActions, 1)
	require.Equal(t, "reconnect", onlineView.UnavailableActions[0].Action)
	require.Equal(t, []string{"original_available"}, onlineView.UnavailableActions[0].Blockers)
	require.True(t, onlineView.CanRemove)
}
