package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"server/internal/api/dto"
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

func testRepository(t *testing.T, rawID string, name string, path string) *repo.Repository {
	t.Helper()

	var repositoryID pgtype.UUID
	require.NoError(t, repositoryID.Scan(rawID))

	return &repo.Repository{
		RepoID: repositoryID,
		Name:   name,
		Path:   path,
	}
}

func TestAssetHandlerListIndexingRepositories_ReturnsOptions(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := &AssetHandler{
		repoManager: stubRepositoryManager{
			listRepositoriesFn: func() ([]*repo.Repository, error) {
				return []*repo.Repository{
					testRepository(
						t,
						"550e8400-e29b-41d4-a716-446655440000",
						"primary",
						"/Volumes/Media/primary",
					),
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

	var response struct {
		Code int                                   `json:"code"`
		Data dto.IndexingRepositoryListResponseDTO `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, 0, response.Code)
	require.Len(t, response.Data.Repositories, 2)
	require.Equal(t, "primary", response.Data.Repositories[0].Name)
	require.Equal(t, "/Volumes/Media/primary", response.Data.Repositories[0].Path)
	require.True(t, response.Data.Repositories[0].IsPrimary)
	require.False(t, response.Data.Repositories[1].IsPrimary)
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
				stats.Tasks.Clip = service.AssetIndexingTaskStats{IndexedCount: 120, QueuedJobs: 4}
				stats.Tasks.OCR = service.AssetIndexingTaskStats{IndexedCount: 110, QueuedJobs: 3}
				stats.Tasks.Caption = service.AssetIndexingTaskStats{IndexedCount: 90, QueuedJobs: 2}
				stats.Tasks.Face = service.AssetIndexingTaskStats{IndexedCount: 60, QueuedJobs: 1}
				return stats, nil
			},
		},
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/assets/indexing/stats?repository_id="+repositoryID, nil)

	handler.GetIndexingStats(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)

	var response struct {
		Code int                               `json:"code"`
		Data dto.AssetIndexingStatsResponseDTO `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, 0, response.Code)
	require.Equal(t, 240, response.Data.PhotoTotal)
	require.Equal(t, 2, response.Data.ReindexJobs)
	require.Equal(t, 120, response.Data.Tasks.Clip.IndexedCount)
	require.Equal(t, 4, response.Data.Tasks.Clip.QueuedJobs)
	require.Equal(t, 110, response.Data.Tasks.OCR.IndexedCount)
	require.Equal(t, 3, response.Data.Tasks.OCR.QueuedJobs)
	require.Equal(t, 90, response.Data.Tasks.Caption.IndexedCount)
	require.Equal(t, 2, response.Data.Tasks.Caption.QueuedJobs)
	require.Equal(t, 60, response.Data.Tasks.Face.IndexedCount)
	require.Equal(t, 1, response.Data.Tasks.Face.QueuedJobs)
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
					Requested:   []service.AssetIndexingTask{service.AssetIndexingTaskClip, service.AssetIndexingTaskOCR},
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

	var response struct {
		Code int                                `json:"code"`
		Data dto.RebuildAssetIndexesResponseDTO `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, 0, response.Code)
	require.Equal(t, "queued", response.Data.Status)
	require.Equal(t, int64(42), response.Data.JobID)
	require.Equal(t, 200, response.Data.Limit)
	require.True(t, response.Data.MissingOnly)
	require.Equal(t, []string{"clip", "ocr"}, response.Data.RequestedTasks)
}

func TestAssetHandlerRebuildAssetIndexes_NormalizesTasksAndLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repositoryID := "550e8400-e29b-41d4-a716-446655440000"
	requestBody, err := json.Marshal(dto.RebuildAssetIndexesRequestDTO{
		RepositoryID: repositoryID,
		Tasks:        []string{" clip ", "OCR"},
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
					service.AssetIndexingTaskClip,
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

	var response struct {
		Code int                                `json:"code"`
		Data dto.RebuildAssetIndexesResponseDTO `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, 0, response.Code)
	require.Equal(t, int64(99), response.Data.JobID)
	require.Equal(t, 500, response.Data.Limit)
	require.False(t, response.Data.MissingOnly)
	require.Equal(t, repositoryID, *response.Data.RepositoryID)
	require.Equal(t, []string{"clip", "ocr"}, response.Data.RequestedTasks)
}

func TestAssetHandlerRebuildAssetIndexes_RejectsInvalidTask(t *testing.T) {
	gin.SetMode(gin.TestMode)

	requestBody, err := json.Marshal(dto.RebuildAssetIndexesRequestDTO{
		Tasks: []string{"clip", "bogus"},
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

func boolPtr(value bool) *bool {
	return &value
}
