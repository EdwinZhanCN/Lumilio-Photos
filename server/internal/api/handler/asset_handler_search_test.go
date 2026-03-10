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

type stubAssetService struct {
	service.AssetService
	queryFn  func(ctx context.Context, params service.QueryAssetsParams) ([]repo.Asset, int64, error)
	searchFn func(ctx context.Context, params service.SearchAssetsParams) (service.SearchAssetsResult, error)
}

func (s stubAssetService) QueryAssets(ctx context.Context, params service.QueryAssetsParams) ([]repo.Asset, int64, error) {
	return s.queryFn(ctx, params)
}

func (s stubAssetService) SearchAssets(ctx context.Context, params service.SearchAssetsParams) (service.SearchAssetsResult, error) {
	return s.searchFn(ctx, params)
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
						SourceTypes: []string{"clip"},
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

	var response struct {
		Code int                         `json:"code"`
		Data dto.SearchAssetsResponseDTO `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, 0, response.Code)
	require.Empty(t, response.Data.TopResults)
	require.Len(t, response.Data.ResultGroups, 1)
	require.Equal(t, "flat:all", response.Data.ResultGroups[0].Key)
	require.Len(t, response.Data.ResultGroups[0].Assets, 1)
	require.True(t, response.Data.TopResultsMeta.Enabled)
	require.True(t, response.Data.TopResultsMeta.Degraded)
	require.Equal(t, "runtime_unavailable", response.Data.TopResultsMeta.Reason)
}

func TestAssetHandlerSearchAssets_ReturnsTopResultsAndResults(t *testing.T) {
	gin.SetMode(gin.TestMode)

	topResult := testHandlerAsset(t, "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", "top.jpg")
	filenameResult := testHandlerAsset(t, "cccccccc-cccc-cccc-cccc-cccccccccccc", "filename.jpg")
	handler := &AssetHandler{
		assetService: stubAssetService{
			searchFn: func(ctx context.Context, params service.SearchAssetsParams) (service.SearchAssetsResult, error) {
				require.Equal(t, "date", params.GroupBy)
				require.Equal(t, "America/New_York", params.ViewerTimeZone)
				require.Equal(t, service.SearchEnhancementModeOnly, params.EnhancementMode)
				return service.SearchAssetsResult{
					TopResults: []repo.Asset{topResult},
					TopResultsMeta: service.SearchTopResultsMeta{
						Enabled:     true,
						SourceTypes: []string{"clip"},
					},
					Results:      []repo.Asset{filenameResult},
					ResultsTotal: 1,
				}, nil
			},
		},
	}

	body, err := json.Marshal(dto.SearchAssetsRequestDTO{
		Query:           "owl",
		GroupBy:         "date",
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

	var response struct {
		Code int                         `json:"code"`
		Data dto.SearchAssetsResponseDTO `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, 0, response.Code)
	require.Len(t, response.Data.TopResults, 1)
	require.Len(t, response.Data.ResultGroups, 1)
	require.Equal(t, "date:year:2023", response.Data.ResultGroups[0].Key)
	require.Len(t, response.Data.ResultGroups[0].Assets, 1)
	require.True(t, response.Data.TopResultsMeta.Enabled)
	require.False(t, response.Data.TopResultsMeta.Degraded)
}

func TestAssetHandlerQueryAssets_InvalidAlbumGroupByReturnsBadRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := &AssetHandler{
		assetService: stubAssetService{},
	}

	body, err := json.Marshal(dto.AssetQueryRequestDTO{
		GroupBy: "album",
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
