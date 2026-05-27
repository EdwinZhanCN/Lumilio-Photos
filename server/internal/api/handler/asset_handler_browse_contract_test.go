package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"server/internal/api/dto"
	"server/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestAssetHandlerQueryAssets_StackModeCollapsed_ReturnsBrowseRowsAndLegacyRepresentatives(t *testing.T) {
	gin.SetMode(gin.TestMode)

	stackID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	coverUUID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	memberUUID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")

	coverAsset := testHandlerAsset(t, coverUUID.String(), "cover.jpg")

	handler := &AssetHandler{
		assetService: stubAssetService{
			queryBrowseFn: func(_ context.Context, params service.QueryAssetsParams) (service.BrowseQueryResult, error) {
				require.Equal(t, service.StackModeCollapsed, params.StackMode)
				return service.BrowseQueryResult{
					Items: []service.BrowseItem{
						{
							Type:  "stack",
							ID:    "stack:" + stackID.String(),
							Asset: coverAsset,
							Stack: &service.BrowseStack{
								StackID:          stackID,
								CoverAssetID:     coverUUID,
								MemberAssetIDs:   []uuid.UUID{coverUUID, memberUUID},
								MatchedMemberIDs: []uuid.UUID{memberUUID},
							},
						},
					},
					TotalVisible: 1,
					TotalAssets:  2,
					StackMode:    service.StackModeCollapsed,
				}, nil
			},
		},
	}

	body, err := json.Marshal(dto.AssetQueryRequestDTO{
		SortBy:    "date_captured",
		StackMode: service.StackModeCollapsed,
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

	require.Equal(t, http.StatusOK, recorder.Code)

	var response struct {
		Code int                        `json:"code"`
		Data dto.QueryAssetsResponseDTO `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, 0, response.Code)
	require.Len(t, response.Data.Items, 1)
	require.Equal(t, "stack", response.Data.Items[0].Type)
	require.NotNil(t, response.Data.Items[0].Stack)
	require.Len(t, response.Data.Items[0].Stack.MemberAssetIDs, 2)
	require.Equal(t, stackID.String(), response.Data.Items[0].Stack.StackID)
	require.Equal(t, coverUUID.String(), response.Data.Items[0].Stack.CoverAssetID)

	require.NotNil(t, response.Data.TotalVisible)
	require.Equal(t, 1, *response.Data.TotalVisible)
	require.NotNil(t, response.Data.TotalAssets)
	require.Equal(t, 2, *response.Data.TotalAssets)
	require.Equal(t, service.StackModeCollapsed, response.Data.StackMode)
}

func TestAssetHandlerQueryAssets_StackModeExpanded_AssetBrowseItemsConsistentTotals(t *testing.T) {
	gin.SetMode(gin.TestMode)

	a := testHandlerAsset(t, "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "a.jpg")
	b := testHandlerAsset(t, "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", "b.jpg")

	handler := &AssetHandler{
		assetService: stubAssetService{
			queryBrowseFn: func(_ context.Context, params service.QueryAssetsParams) (service.BrowseQueryResult, error) {
				require.Equal(t, service.StackModeExpanded, params.StackMode)
				return service.BrowseQueryResult{
					Items: []service.BrowseItem{
						{Type: "asset", ID: "asset:" + uuid.UUID(a.AssetID.Bytes).String(), Asset: a},
						{Type: "asset", ID: "asset:" + uuid.UUID(b.AssetID.Bytes).String(), Asset: b},
					},
					TotalVisible: 2,
					TotalAssets:  2,
					StackMode:    service.StackModeExpanded,
				}, nil
			},
		},
	}

	body, err := json.Marshal(dto.AssetQueryRequestDTO{
		SortBy:    "date_captured",
		StackMode: service.StackModeExpanded,
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

	require.Equal(t, http.StatusOK, recorder.Code)

	var response struct {
		Code int                        `json:"code"`
		Data dto.QueryAssetsResponseDTO `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Len(t, response.Data.Items, 2)
	require.NotNil(t, response.Data.TotalVisible)
	require.Equal(t, 2, *response.Data.TotalVisible)
	require.NotNil(t, response.Data.TotalAssets)
	require.Equal(t, 2, *response.Data.TotalAssets)
	require.Equal(t, service.StackModeExpanded, response.Data.StackMode)
}

func TestAssetHandlerQueryAssets_InvalidStackModeReturnsBadRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := &AssetHandler{
		assetService: stubAssetService{},
	}

	body, err := json.Marshal(dto.AssetQueryRequestDTO{
		SortBy:    "date_captured",
		StackMode: "fancy",
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

func TestAssetHandlerSearchAssets_StackModeCollapsed_ReturnsBrowseRowsAndLegacyRepresentatives(t *testing.T) {
	gin.SetMode(gin.TestMode)

	stackID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	coverUUID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	memberUUID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")

	coverAsset := testHandlerAsset(t, coverUUID.String(), "cover.jpg")

	handler := &AssetHandler{
		assetService: stubAssetService{
			searchBrowseFn: func(_ context.Context, params service.SearchAssetsParams) (service.SearchBrowseResult, error) {
				require.Equal(t, service.StackModeCollapsed, params.StackMode)
				stackItem := service.BrowseItem{
					Type:  "stack",
					ID:    "stack:" + stackID.String(),
					Asset: coverAsset,
					Stack: &service.BrowseStack{
						StackID:          stackID,
						CoverAssetID:     coverUUID,
						MemberAssetIDs:   []uuid.UUID{coverUUID, memberUUID},
						MatchedMemberIDs: []uuid.UUID{memberUUID},
					},
				}
				return service.SearchBrowseResult{
					TopResults:          []service.BrowseItem{stackItem},
					Results:             []service.BrowseItem{stackItem},
					ResultsTotalVisible: 1,
					ResultsTotalAssets:  2,
					StackMode:           service.StackModeCollapsed,
				}, nil
			},
		},
	}

	body, err := json.Marshal(dto.SearchAssetsRequestDTO{
		Query:           "sunset",
		SortBy:          "date_captured",
		StackMode:       service.StackModeCollapsed,
		EnhancementMode: "auto",
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

	require.Len(t, response.Data.TopItems, 1)
	require.Equal(t, "stack", response.Data.TopItems[0].Type)
	require.NotNil(t, response.Data.TopItems[0].Stack)
	require.Equal(t, stackID.String(), response.Data.TopItems[0].Stack.StackID)
	require.Equal(t, coverUUID.String(), response.Data.TopItems[0].Stack.CoverAssetID)
	require.Equal(t, 2, response.Data.TopItems[0].Stack.StackSize)

	require.Len(t, response.Data.ResultItems, 1)
	require.Equal(t, "stack", response.Data.ResultItems[0].Type)
	require.NotNil(t, response.Data.ResultItems[0].Stack)
	require.Equal(t, stackID.String(), response.Data.ResultItems[0].Stack.StackID)
	require.Equal(t, coverUUID.String(), response.Data.ResultItems[0].Stack.CoverAssetID)
	require.Equal(t, 2, response.Data.ResultItems[0].Stack.StackSize)

	require.NotNil(t, response.Data.ResultsTotalVisible)
	require.Equal(t, 1, *response.Data.ResultsTotalVisible)
	require.NotNil(t, response.Data.ResultsTotalAssets)
	require.Equal(t, 2, *response.Data.ResultsTotalAssets)
	require.Equal(t, service.StackModeCollapsed, response.Data.StackMode)
}

func TestAssetHandlerSearchAssets_StackModeExpanded_AssetBrowseItemsConsistentTotals(t *testing.T) {
	gin.SetMode(gin.TestMode)

	a := testHandlerAsset(t, "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "a.jpg")
	b := testHandlerAsset(t, "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", "b.jpg")

	handler := &AssetHandler{
		assetService: stubAssetService{
			searchBrowseFn: func(_ context.Context, params service.SearchAssetsParams) (service.SearchBrowseResult, error) {
				require.Equal(t, service.StackModeExpanded, params.StackMode)
				return service.SearchBrowseResult{
					Results: []service.BrowseItem{
						{Type: "asset", ID: "asset:" + uuid.UUID(a.AssetID.Bytes).String(), Asset: a},
						{Type: "asset", ID: "asset:" + uuid.UUID(b.AssetID.Bytes).String(), Asset: b},
					},
					ResultsTotalVisible: 2,
					ResultsTotalAssets:  2,
					StackMode:           service.StackModeExpanded,
				}, nil
			},
		},
	}

	body, err := json.Marshal(dto.SearchAssetsRequestDTO{
		Query:           "lake",
		SortBy:          "date_captured",
		StackMode:       service.StackModeExpanded,
		EnhancementMode: "auto",
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

	require.Len(t, response.Data.ResultItems, 2)
	require.NotNil(t, response.Data.ResultsTotalVisible)
	require.Equal(t, 2, *response.Data.ResultsTotalVisible)
	require.NotNil(t, response.Data.ResultsTotalAssets)
	require.Equal(t, 2, *response.Data.ResultsTotalAssets)
	require.Equal(t, service.StackModeExpanded, response.Data.StackMode)
}

func TestAssetHandlerSearchAssets_InvalidStackModeReturnsBadRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := &AssetHandler{
		assetService: stubAssetService{},
	}

	body, err := json.Marshal(dto.SearchAssetsRequestDTO{
		Query:           "sunset",
		SortBy:          "date_captured",
		StackMode:       "fancy",
		EnhancementMode: "auto",
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

	require.Equal(t, http.StatusBadRequest, recorder.Code)
}
