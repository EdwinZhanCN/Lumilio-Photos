package handler

import (
	"bytes"
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
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
)

type stubAssetIndexingService struct {
	service.AssetIndexingService
	getIndexingStatsFn   func(ctx context.Context, repositoryID *string) (service.AssetIndexingStats, error)
	enqueueReindexAssets func(ctx context.Context, input service.ReindexAssetsInput) (service.ReindexAssetsJobResult, error)
}

func (s stubAssetIndexingService) GetIndexingStats(ctx context.Context, repositoryID *string) (service.AssetIndexingStats, error) {
	return s.getIndexingStatsFn(ctx, repositoryID)
}

func (s stubAssetIndexingService) EnqueueReindexAssets(ctx context.Context, input service.ReindexAssetsInput) (service.ReindexAssetsJobResult, error) {
	return s.enqueueReindexAssets(ctx, input)
}

type stubRepositoryManager struct {
	storage.RepositoryManager
	listRepositoriesFn func() ([]*repo.Repository, error)
}

func (s stubRepositoryManager) ListRepositories() ([]*repo.Repository, error) {
	return s.listRepositoriesFn()
}

func (s stubRepositoryManager) ReconcileAll(context.Context) error { return nil }

func testRepository(t *testing.T, rawID string, name string, path string) *repo.Repository {
	t.Helper()

	var repositoryID pgtype.UUID
	require.NoError(t, repositoryID.Scan(rawID))

	return &repo.Repository{
		RepoID: repositoryID,
		Name:   name,
		Path:   path,
		Role:   dbtypes.RepoRoleRegular,
	}
}

func TestAssetHandlerListIndexingRepositories_ReturnsOptions(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := &AssetHandler{
		repoManager: stubRepositoryManager{
			listRepositoriesFn: func() ([]*repo.Repository, error) {
				return []*repo.Repository{
					func() *repo.Repository {
						repository := testRepository(
							t,
							"550e8400-e29b-41d4-a716-446655440000",
							"primary",
							"/Volumes/Media/primary",
						)
						repository.Role = dbtypes.RepoRolePrimary
						return repository
					}(),
					testRepository(
						t,
						"660e8400-e29b-41d4-a716-446655440000",
						"Archive",
						"/Volumes/Media/Archive",
					),
				}, nil
			},
		},
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/assets/indexing/repositories", nil)

	handler.ListIndexingRepositories(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)

	var response dto.IndexingRepositoryListResponseDTO
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Len(t, response.Repositories, 2)
	require.Equal(t, "primary", response.Repositories[0].Name)
	require.Equal(t, "/Volumes/Media/primary", response.Repositories[0].Path)
	require.True(t, response.Repositories[0].IsPrimary)
	require.False(t, response.Repositories[1].IsPrimary)
}

func TestAssetHandlerGetIndexingStats_ReturnsStats(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repositoryID := "550e8400-e29b-41d4-a716-446655440000"
	handler := &AssetHandler{
		indexingService: stubAssetIndexingService{
			getIndexingStatsFn: func(ctx context.Context, requestRepositoryID *string) (service.AssetIndexingStats, error) {
				require.NotNil(t, requestRepositoryID)
				require.Equal(t, repositoryID, *requestRepositoryID)

				stats := service.AssetIndexingStats{
					PhotoTotal:  240,
					ReindexJobs: 2,
				}
				stats.Tasks.Semantic = service.AssetIndexingTaskStats{IndexedCount: 120, QueuedJobs: 4, TotalCount: 240}
				stats.Tasks.BioCLIP = service.AssetIndexingTaskStats{IndexedCount: 80, QueuedJobs: 5, TotalCount: 90}
				stats.Tasks.OCR = service.AssetIndexingTaskStats{IndexedCount: 110, QueuedJobs: 3, TotalCount: 240}
				stats.Tasks.Face = service.AssetIndexingTaskStats{IndexedCount: 60, QueuedJobs: 1, TotalCount: 240}
				return stats, nil
			},
		},
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/assets/indexing/stats?repository_id="+repositoryID, nil)

	handler.GetIndexingStats(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)

	var response dto.AssetIndexingStatsResponseDTO
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, 240, response.PhotoTotal)
	require.Equal(t, 2, response.ReindexJobs)
	require.Equal(t, 120, response.Tasks.Semantic.IndexedCount)
	require.Equal(t, 4, response.Tasks.Semantic.QueuedJobs)
	require.Equal(t, 240, response.Tasks.Semantic.TotalCount)
	require.Equal(t, 80, response.Tasks.BioCLIP.IndexedCount)
	require.Equal(t, 5, response.Tasks.BioCLIP.QueuedJobs)
	require.Equal(t, 90, response.Tasks.BioCLIP.TotalCount)
	require.Equal(t, 110, response.Tasks.OCR.IndexedCount)
	require.Equal(t, 3, response.Tasks.OCR.QueuedJobs)
	require.Equal(t, 240, response.Tasks.OCR.TotalCount)
	require.Equal(t, 60, response.Tasks.Face.IndexedCount)
	require.Equal(t, 1, response.Tasks.Face.QueuedJobs)
	require.Equal(t, 240, response.Tasks.Face.TotalCount)
}

func TestAssetHandlerGetIndexingStats_RejectsInvalidRepositoryID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := &AssetHandler{
		indexingService: stubAssetIndexingService{
			getIndexingStatsFn: func(ctx context.Context, repositoryID *string) (service.AssetIndexingStats, error) {
				t.Fatal("service should not be called for invalid repository IDs")
				return service.AssetIndexingStats{}, nil
			},
		},
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/assets/indexing/stats?repository_id=not-a-uuid", nil)

	handler.GetIndexingStats(ctx)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestAssetHandlerRebuildAssetIndexes_QueuesDefaultBatch(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := &AssetHandler{
		indexingService: stubAssetIndexingService{
			enqueueReindexAssets: func(ctx context.Context, input service.ReindexAssetsInput) (service.ReindexAssetsJobResult, error) {
				require.Nil(t, input.RepositoryID)
				require.Nil(t, input.Tasks)
				require.Equal(t, 200, input.Limit)
				require.True(t, input.MissingOnly)

				return service.ReindexAssetsJobResult{
					JobID:       42,
					Requested:   []service.AssetIndexingTask{service.AssetIndexingTaskSemanticImage, service.AssetIndexingTaskOCR},
					Limit:       input.Limit,
					MissingOnly: input.MissingOnly,
				}, nil
			},
		},
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/assets/indexing/rebuild", bytes.NewReader([]byte("{}")))
	ctx.Request.Header.Set("Content-Type", "application/json")

	handler.RebuildAssetIndexes(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)

	var response dto.RebuildAssetIndexesResponseDTO
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, "queued", response.Status)
	require.Equal(t, int64(42), response.JobID)
	require.Equal(t, 200, response.Limit)
	require.True(t, response.MissingOnly)
	require.Equal(t, []string{"semantic", "ocr"}, response.RequestedTasks)
}

func TestAssetHandlerRebuildAssetIndexes_NormalizesTasksAndLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repositoryID := "550e8400-e29b-41d4-a716-446655440000"
	requestBody, err := json.Marshal(dto.RebuildAssetIndexesRequestDTO{
		RepositoryID: repositoryID,
		Tasks:        []string{" semantic ", "OCR"},
		Limit:        999,
		MissingOnly:  boolPtr(false),
	})
	require.NoError(t, err)

	handler := &AssetHandler{
		indexingService: stubAssetIndexingService{
			enqueueReindexAssets: func(ctx context.Context, input service.ReindexAssetsInput) (service.ReindexAssetsJobResult, error) {
				require.NotNil(t, input.RepositoryID)
				require.Equal(t, repositoryID, *input.RepositoryID)
				require.Equal(t, []service.AssetIndexingTask{
					service.AssetIndexingTaskSemanticImage,
					service.AssetIndexingTaskOCR,
				}, input.Tasks)
				require.Equal(t, 500, input.Limit)
				require.False(t, input.MissingOnly)

				return service.ReindexAssetsJobResult{
					JobID:        99,
					Requested:    input.Tasks,
					Limit:        input.Limit,
					MissingOnly:  input.MissingOnly,
					RepositoryID: input.RepositoryID,
				}, nil
			},
		},
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/assets/indexing/rebuild", bytes.NewReader(requestBody))
	ctx.Request.Header.Set("Content-Type", "application/json")

	handler.RebuildAssetIndexes(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)

	var response dto.RebuildAssetIndexesResponseDTO
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, int64(99), response.JobID)
	require.Equal(t, 500, response.Limit)
	require.False(t, response.MissingOnly)
	require.Equal(t, repositoryID, *response.RepositoryID)
	require.Equal(t, []string{"semantic", "ocr"}, response.RequestedTasks)
}

func TestAssetHandlerRebuildAssetIndexes_RejectsInvalidTask(t *testing.T) {
	gin.SetMode(gin.TestMode)

	requestBody, err := json.Marshal(dto.RebuildAssetIndexesRequestDTO{
		Tasks: []string{"semantic", "bogus"},
	})
	require.NoError(t, err)

	handler := &AssetHandler{
		indexingService: stubAssetIndexingService{
			enqueueReindexAssets: func(ctx context.Context, input service.ReindexAssetsInput) (service.ReindexAssetsJobResult, error) {
				t.Fatal("service should not be called for invalid tasks")
				return service.ReindexAssetsJobResult{}, nil
			},
		},
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/assets/indexing/rebuild", bytes.NewReader(requestBody))
	ctx.Request.Header.Set("Content-Type", "application/json")

	handler.RebuildAssetIndexes(ctx)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestAssetHandlerRebuildAssetIndexes_RejectsBioClipTask(t *testing.T) {
	gin.SetMode(gin.TestMode)

	requestBody, err := json.Marshal(dto.RebuildAssetIndexesRequestDTO{
		Tasks: []string{"bioclip"},
	})
	require.NoError(t, err)

	handler := &AssetHandler{
		indexingService: stubAssetIndexingService{
			enqueueReindexAssets: func(ctx context.Context, input service.ReindexAssetsInput) (service.ReindexAssetsJobResult, error) {
				t.Fatal("service should not be called for album-scoped BioCLIP tasks")
				return service.ReindexAssetsJobResult{}, nil
			},
		},
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/assets/indexing/rebuild", bytes.NewReader(requestBody))
	ctx.Request.Header.Set("Content-Type", "application/json")

	handler.RebuildAssetIndexes(ctx)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func boolPtr(value bool) *bool {
	return &value
}
