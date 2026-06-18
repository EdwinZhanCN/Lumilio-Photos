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
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
)

type stubAssetService struct {
	service.AssetService
	queryFn        func(ctx context.Context, params service.QueryAssetsParams) ([]repo.Asset, int64, error)
	searchFn       func(ctx context.Context, params service.SearchAssetsParams) (service.SearchAssetsResult, error)
	queryBrowseFn  func(ctx context.Context, params service.QueryAssetsParams) (service.BrowseQueryResult, error)
	searchBrowseFn func(ctx context.Context, params service.SearchAssetsParams) (service.SearchBrowseResult, error)
	getAssetFn     func(ctx context.Context, id uuid.UUID) (*repo.Asset, error)
}

func (s stubAssetService) GetAsset(ctx context.Context, id uuid.UUID) (*repo.Asset, error) {
	return s.getAssetFn(ctx, id)
}

func (s stubAssetService) QueryAssets(ctx context.Context, params service.QueryAssetsParams) ([]repo.Asset, int64, error) {
	return s.queryFn(ctx, params)
}

func (s stubAssetService) SearchAssets(ctx context.Context, params service.SearchAssetsParams) (service.SearchAssetsResult, error) {
	return s.searchFn(ctx, params)
}

func (s stubAssetService) QueryBrowseItems(ctx context.Context, params service.QueryAssetsParams) (service.BrowseQueryResult, error) {
	if s.queryBrowseFn != nil {
		return s.queryBrowseFn(ctx, params)
	}
	assets, total, err := s.queryFn(ctx, params)
	if err != nil {
		return service.BrowseQueryResult{}, err
	}
	items := make([]service.BrowseItem, 0, len(assets))
	for _, asset := range assets {
		if !asset.AssetID.Valid {
			continue
		}
		items = append(items, service.BrowseItem{
			Type:  "asset",
			ID:    "asset:" + uuid.UUID(asset.AssetID.Bytes).String(),
			Asset: asset,
		})
	}
	return service.BrowseQueryResult{
		Items:        items,
		TotalVisible: total,
		TotalAssets:  total,
		StackMode:    service.StackModeCollapsed,
	}, nil
}

func (s stubAssetService) SearchBrowseItems(ctx context.Context, params service.SearchAssetsParams) (service.SearchBrowseResult, error) {
	if s.searchBrowseFn != nil {
		return s.searchBrowseFn(ctx, params)
	}
	result, err := s.searchFn(ctx, params)
	if err != nil {
		return service.SearchBrowseResult{}, err
	}

	toBrowseItems := func(assets []repo.Asset) []service.BrowseItem {
		items := make([]service.BrowseItem, 0, len(assets))
		for _, asset := range assets {
			if !asset.AssetID.Valid {
				continue
			}
			items = append(items, service.BrowseItem{
				Type:  "asset",
				ID:    "asset:" + uuid.UUID(asset.AssetID.Bytes).String(),
				Asset: asset,
			})
		}
		return items
	}

	return service.SearchBrowseResult{
		TopResults:          toBrowseItems(result.TopResults),
		TopResultsMeta:      result.TopResultsMeta,
		Results:             toBrowseItems(result.Results),
		ResultsTotalVisible: result.ResultsTotal,
		ResultsTotalAssets:  result.ResultsTotal,
		StackMode:           service.StackModeCollapsed,
	}, nil
}

func testHandlerAsset(t *testing.T, rawID string, filename string) repo.Asset {
	t.Helper()

	var assetID pgtype.UUID
	require.NoError(t, assetID.Scan(rawID))

	path := "/tmp/" + filename
	return repo.Asset{
		AssetID:          assetID,
		Type:             "PHOTO",
		OriginalFilename: filename,
		StoragePath:      &path,
		MimeType:         "image/jpeg",
		UploadTime:       pgtype.Timestamptz{Time: time.Unix(1700000000, 0), Valid: true},
	}
}

func TestAssetHandlerSearchAssets_ReturnsDegradedResultsWithout503(t *testing.T) {
	gin.SetMode(gin.TestMode)

	filenameResult := testHandlerAsset(t, "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "filename.jpg")
	handler := &AssetHandler{
		assetService: stubAssetService{
			searchFn: func(ctx context.Context, params service.SearchAssetsParams) (service.SearchAssetsResult, error) {
				require.Equal(t, "sunset", params.Query)
				require.Equal(t, service.SearchEnhancementModeAuto, params.EnhancementMode)
				return service.SearchAssetsResult{
					TopResults: []repo.Asset{},
					TopResultsMeta: service.SearchTopResultsMeta{
						Enabled:     true,
						Degraded:    true,
						Reason:      "runtime_unavailable",
						SourceTypes: []string{"semantic"},
					},
					Results:      []repo.Asset{filenameResult},
					ResultsTotal: 1,
				}, nil
			},
		},
	}

	body, err := json.Marshal(dto.SearchAssetsRequestDTO{
		Query: "sunset",
		Pagination: dto.PaginationDTO{
			Limit:  20,
			Offset: 0,
		},
	})
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/assets/search", bytes.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")

	handler.SearchAssets(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)

	var response dto.SearchAssetsResponseDTO
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Empty(t, response.TopItems)
	require.Len(t, response.ResultItems, 1)
	require.True(t, response.TopResultsMeta.Enabled)
	require.True(t, response.TopResultsMeta.Degraded)
	require.Equal(t, "runtime_unavailable", response.TopResultsMeta.Reason)
}

func TestAssetHandlerSearchAssets_ReturnsTopResultsAndResults(t *testing.T) {
	gin.SetMode(gin.TestMode)

	topResult := testHandlerAsset(t, "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", "top.jpg")
	filenameResult := testHandlerAsset(t, "cccccccc-cccc-cccc-cccc-cccccccccccc", "filename.jpg")
	handler := &AssetHandler{
		assetService: stubAssetService{
			searchFn: func(ctx context.Context, params service.SearchAssetsParams) (service.SearchAssetsResult, error) {
				require.Equal(t, "date_captured", params.SortBy)
				require.Equal(t, "America/New_York", params.ViewerTimeZone)
				require.Equal(t, service.SearchEnhancementModeOnly, params.EnhancementMode)
				return service.SearchAssetsResult{
					TopResults: []repo.Asset{topResult},
					TopResultsMeta: service.SearchTopResultsMeta{
						Enabled:     true,
						SourceTypes: []string{"semantic"},
					},
					Results:      []repo.Asset{filenameResult},
					ResultsTotal: 1,
				}, nil
			},
		},
	}

	body, err := json.Marshal(dto.SearchAssetsRequestDTO{
		Query:           "owl",
		SortBy:          "date_captured",
		ViewerTimezone:  "America/New_York",
		EnhancementMode: "only",
		Pagination: dto.PaginationDTO{
			Limit:  10,
			Offset: 0,
		},
	})
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/assets/search", bytes.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")

	handler.SearchAssets(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)

	var response dto.SearchAssetsResponseDTO
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Len(t, response.TopItems, 1)
	require.Len(t, response.ResultItems, 1)
	require.True(t, response.TopResultsMeta.Enabled)
	require.False(t, response.TopResultsMeta.Degraded)
}

func TestAssetHandlerQueryAssets_InvalidSortByReturnsBadRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := &AssetHandler{
		assetService: stubAssetService{},
	}

	body, err := json.Marshal(dto.AssetQueryRequestDTO{
		SortBy: "album",
		Pagination: dto.PaginationDTO{
			Limit:  20,
			Offset: 0,
		},
	})
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/assets/list", bytes.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")

	handler.QueryAssets(ctx)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
}
