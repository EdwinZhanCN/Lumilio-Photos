package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"server/internal/api/dto"
	"server/internal/db/repo"
	"server/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
)

type stubPeopleFaceService struct {
	service.FaceService
	listFn   func(context.Context, pgtype.UUID, *int32, int, int) ([]service.Person, int64, error)
	getFn    func(context.Context, int32, pgtype.UUID, *int32) (*service.Person, error)
	renameFn func(context.Context, int32, string) (*repo.FaceCluster, error)
}

func (s stubPeopleFaceService) ListPeople(ctx context.Context, repositoryID pgtype.UUID, ownerID *int32, limit, offset int) ([]service.Person, int64, error) {
	return s.listFn(ctx, repositoryID, ownerID, limit, offset)
}

func (s stubPeopleFaceService) GetPerson(ctx context.Context, clusterID int32, repositoryID pgtype.UUID, ownerID *int32) (*service.Person, error) {
	return s.getFn(ctx, clusterID, repositoryID, ownerID)
}

func (s stubPeopleFaceService) RenamePerson(ctx context.Context, clusterID int32, name string) (*repo.FaceCluster, error) {
	return s.renameFn(ctx, clusterID, name)
}

func TestPeopleHandlerListPeople(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var capturedRepo pgtype.UUID
	handler := NewPeopleHandler(
		stubAssetService{},
		stubPeopleFaceService{
			listFn: func(_ context.Context, repositoryID pgtype.UUID, ownerID *int32, limit, offset int) ([]service.Person, int64, error) {
				capturedRepo = repositoryID
				require.Nil(t, ownerID)
				require.Equal(t, 24, limit)
				require.Equal(t, 0, offset)
				return []service.Person{
					{
						PersonID:    7,
						Name:        strPtr("Alice"),
						IsConfirmed: true,
						MemberCount: 4,
						AssetCount:  3,
						CreatedAt:   time.Unix(1700000000, 0),
						UpdatedAt:   time.Unix(1700003600, 0),
					},
				}, 1, nil
			},
		},
		nil,
		nil,
	)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/people?repository_id=550e8400-e29b-41d4-a716-446655440000", nil)

	handler.ListPeople(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.True(t, capturedRepo.Valid)

	var response struct {
		Code int                       `json:"code"`
		Data dto.ListPeopleResponseDTO `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, 1, response.Data.Total)
	require.Len(t, response.Data.People, 1)
	require.Equal(t, int32(7), response.Data.People[0].PersonID)
	require.Equal(t, "Alice", *response.Data.People[0].Name)
}

func TestPeopleHandlerUpdatePerson(t *testing.T) {
	gin.SetMode(gin.TestMode)

	current := service.Person{
		PersonID:    9,
		Name:        nil,
		IsConfirmed: false,
		MemberCount: 2,
		AssetCount:  2,
		CreatedAt:   time.Unix(1700000000, 0),
		UpdatedAt:   time.Unix(1700003600, 0),
	}

	handler := NewPeopleHandler(
		stubAssetService{},
		stubPeopleFaceService{
			getFn: func(_ context.Context, clusterID int32, _ pgtype.UUID, ownerID *int32) (*service.Person, error) {
				require.Equal(t, int32(9), clusterID)
				require.Nil(t, ownerID)
				copy := current
				return &copy, nil
			},
			renameFn: func(_ context.Context, clusterID int32, name string) (*repo.FaceCluster, error) {
				require.Equal(t, int32(9), clusterID)
				require.Equal(t, "Grace", name)
				current.Name = strPtr(name)
				current.IsConfirmed = true
				return &repo.FaceCluster{ClusterID: clusterID, ClusterName: &name}, nil
			},
		},
		nil,
		nil,
	)

	body, err := json.Marshal(dto.UpdatePersonRequestDTO{Name: "  Grace  "})
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/people/9", bytes.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Params = gin.Params{{Key: "id", Value: "9"}}

	handler.UpdatePerson(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)

	var response struct {
		Code int                 `json:"code"`
		Data dto.PersonDetailDTO `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, "Grace", *response.Data.Name)
	require.True(t, response.Data.IsConfirmed)
}

func TestPeopleHandlerListPersonAssetsInjectsPersonScope(t *testing.T) {
	gin.SetMode(gin.TestMode)

	asset := testHandlerAsset(t, "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "person-photo.jpg")
	handler := NewPeopleHandler(
		stubAssetService{
			queryFn: func(_ context.Context, params service.QueryAssetsParams) ([]repo.Asset, int64, error) {
				require.NotNil(t, params.PersonID)
				require.Equal(t, int32(12), *params.PersonID)
				require.NotNil(t, params.RepositoryID)
				require.Equal(t, "550e8400-e29b-41d4-a716-446655440000", *params.RepositoryID)
				return []repo.Asset{asset}, 1, nil
			},
		},
		stubPeopleFaceService{
			getFn: func(_ context.Context, clusterID int32, repositoryID pgtype.UUID, _ *int32) (*service.Person, error) {
				require.Equal(t, int32(12), clusterID)
				require.True(t, repositoryID.Valid)
				return &service.Person{
					PersonID:    12,
					Name:        strPtr("Kai"),
					IsConfirmed: true,
					MemberCount: 1,
					AssetCount:  1,
					CreatedAt:   time.Unix(1700000000, 0),
					UpdatedAt:   time.Unix(1700003600, 0),
				}, nil
			},
		},
		nil,
		nil,
	)

	body, err := json.Marshal(dto.AssetQueryRequestDTO{
		Filter: dto.AssetFilterDTO{
			RepositoryID: strPtr("550e8400-e29b-41d4-a716-446655440000"),
		},
		Pagination: dto.PaginationDTO{Limit: 20, Offset: 0},
		GroupBy:    "date",
	})
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/people/12/assets/list", bytes.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Params = gin.Params{{Key: "id", Value: "12"}}

	handler.ListPersonAssets(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
}

func strPtr(value string) *string {
	return &value
}
