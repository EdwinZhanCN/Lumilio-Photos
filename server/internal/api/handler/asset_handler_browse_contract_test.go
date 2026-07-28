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
	"server/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// The browse contract is media-item-first: rows are `media_item` or `stack`,
// ids are `media:<uuid>` / `stack:<uuid>`, and totals are reported as
// total_visible / total_media_items / total_files. There is no `asset` row
// type and no total_assets.

func postJSON(t *testing.T, path string, payload any, run func(*gin.Context)) *httptest.ResponseRecorder {
	t.Helper()

	body, err := json.Marshal(payload)
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	run(ctx)
	return recorder
}

func queryAssetsRequest(stackMode string, filter dto.AssetFilterDTO) dto.AssetQueryRequestDTO {
	return dto.AssetQueryRequestDTO{
		SortBy:     "date_captured",
		StackMode:  stackMode,
		Filter:     filter,
		Pagination: dto.PaginationDTO{Limit: 20, Offset: 0},
	}
}

func searchAssetsRequest(query string, filter dto.AssetFilterDTO) dto.SearchAssetsRequestDTO {
	return dto.SearchAssetsRequestDTO{
		Query:           query,
		SortBy:          "date_captured",
		EnhancementMode: "auto",
		Filter:          filter,
		Pagination:      dto.PaginationDTO{Limit: 10, Offset: 0},
	}
}

// testBrowseStackItem builds a collapsed stack row whose cover is the first
// member and whose matched subset can be narrower than its membership.
func testBrowseStackItem(
	t *testing.T,
	stackID uuid.UUID,
	kind dbtypes.StackKind,
	cover service.BrowseMediaItem,
	members []service.BrowseStackMember,
	matched []service.BrowseStackMember,
) service.BrowseItem {
	t.Helper()

	cover.StackID = stackID
	cover.StackKind = kind
	return service.BrowseItem{
		Type: service.BrowseItemTypeStack,
		ID:   "stack:" + stackID.String(),
		Stack: &service.BrowseStack{
			StackID:        stackID,
			Kind:           kind,
			Cover:          cover,
			Members:        members,
			MatchedMembers: matched,
		},
	}
}

func testBrowseMediaItem(t *testing.T, rawID, filename string) service.BrowseMediaItem {
	t.Helper()

	asset := testHandlerAsset(t, rawID, filename)
	return service.BrowseMediaItem{
		MediaItemID:    asset.AssetID,
		MediaKind:      "photo",
		PrimaryAsset:   asset,
		ComponentCount: 1,
		HasJPEG:        true,
	}
}

func TestAssetHandlerQueryAssets_StackModeCollapsed_ReturnsStackRows(t *testing.T) {
	gin.SetMode(gin.TestMode)

	stackID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	cover := testBrowseMediaItem(t, "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "cover.jpg")
	member := testBrowseMediaItem(t, "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", "member.jpg")

	members := []service.BrowseStackMember{
		{MediaItemID: cover.MediaItemID, PrimaryAssetID: cover.PrimaryAsset.AssetID},
		{MediaItemID: member.MediaItemID, PrimaryAssetID: member.PrimaryAsset.AssetID},
	}
	matched := members[1:]

	handler := &AssetHandler{
		assetService: stubAssetService{
			queryBrowseFn: func(_ context.Context, params service.QueryAssetsParams) (service.BrowseQueryResult, error) {
				require.Equal(t, service.StackModeCollapsed, params.StackMode)
				return service.BrowseQueryResult{
					Items:           []service.BrowseItem{testBrowseStackItem(t, stackID, dbtypes.StackKindBurst, cover, members, matched)},
					TotalVisible:    1,
					TotalMediaItems: 2,
					TotalFiles:      3,
					StackMode:       service.StackModeCollapsed,
				}, nil
			},
		},
	}

	recorder := postJSON(t, "/api/v1/assets/list",
		queryAssetsRequest(service.StackModeCollapsed, dto.AssetFilterDTO{}), handler.QueryAssets)
	require.Equal(t, http.StatusOK, recorder.Code)

	var response dto.QueryAssetsResponseDTO
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Len(t, response.Items, 1)

	row := response.Items[0]
	require.Equal(t, "stack", row.Type)
	require.Equal(t, "stack:"+stackID.String(), row.ID)
	require.Nil(t, row.MediaItem)
	require.NotNil(t, row.Stack)
	require.Equal(t, stackID.String(), row.Stack.StackID)
	require.Equal(t, "burst", row.Stack.StackKind)
	require.Len(t, row.Stack.Members, 2)
	require.Len(t, row.Stack.MatchedMembers, 1)
	require.Equal(t, member.MediaItemID.String(), row.Stack.MatchedMembers[0].MediaItemID)

	// The cover carries the badge payload: cover flag plus member count.
	require.NotNil(t, row.Stack.Cover.Stack)
	require.True(t, row.Stack.Cover.Stack.StackCover)
	require.NotNil(t, row.Stack.Cover.Stack.StackSize)
	require.Equal(t, 2, *row.Stack.Cover.Stack.StackSize)

	require.NotNil(t, response.TotalVisible)
	require.Equal(t, 1, *response.TotalVisible)
	require.NotNil(t, response.TotalMediaItems)
	require.Equal(t, 2, *response.TotalMediaItems)
	require.NotNil(t, response.TotalFiles)
	require.Equal(t, 3, *response.TotalFiles)
	require.Equal(t, service.StackModeCollapsed, response.StackMode)
}

func TestAssetHandlerQueryAssets_StackModeExpanded_ReturnsMediaItemRows(t *testing.T) {
	gin.SetMode(gin.TestMode)

	stackID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	first := testBrowseMediaItem(t, "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "a.jpg")
	second := testBrowseMediaItem(t, "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", "b.jpg")
	// An expanded row keeps its stack pointer but carries no member count, so
	// the gallery renders no stack overlay badge for it.
	second.StackID = stackID
	second.StackKind = dbtypes.StackKindBurst

	handler := &AssetHandler{
		assetService: stubAssetService{
			queryBrowseFn: func(_ context.Context, params service.QueryAssetsParams) (service.BrowseQueryResult, error) {
				require.Equal(t, service.StackModeExpanded, params.StackMode)
				return service.BrowseQueryResult{
					Items: []service.BrowseItem{
						{Type: service.BrowseItemTypeMediaItem, ID: "media:" + first.MediaItemID.String(), MediaItem: &first},
						{Type: service.BrowseItemTypeMediaItem, ID: "media:" + second.MediaItemID.String(), MediaItem: &second},
					},
					TotalVisible:    2,
					TotalMediaItems: 2,
					TotalFiles:      2,
					StackMode:       service.StackModeExpanded,
				}, nil
			},
		},
	}

	recorder := postJSON(t, "/api/v1/assets/list",
		queryAssetsRequest(service.StackModeExpanded, dto.AssetFilterDTO{}), handler.QueryAssets)
	require.Equal(t, http.StatusOK, recorder.Code)

	var response dto.QueryAssetsResponseDTO
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Len(t, response.Items, 2)

	for _, row := range response.Items {
		require.Equal(t, "media_item", row.Type)
		require.Nil(t, row.Stack)
		require.NotNil(t, row.MediaItem)
		require.Equal(t, "media:"+row.MediaItem.MediaItemID, row.ID)
	}
	require.Nil(t, response.Items[0].MediaItem.Stack)
	require.NotNil(t, response.Items[1].MediaItem.Stack)
	require.Equal(t, stackID.String(), response.Items[1].MediaItem.Stack.StackID)
	require.Nil(t, response.Items[1].MediaItem.Stack.StackSize)

	require.NotNil(t, response.TotalVisible)
	require.Equal(t, 2, *response.TotalVisible)
	require.NotNil(t, response.TotalMediaItems)
	require.Equal(t, 2, *response.TotalMediaItems)
	require.Equal(t, service.StackModeExpanded, response.StackMode)
}

func TestAssetHandlerQueryAssets_ReturnsCompositionFactsForBadges(t *testing.T) {
	gin.SetMode(gin.TestMode)

	pair := testBrowseMediaItem(t, "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "pair.jpg")
	pair.ComponentCount = 2
	pair.HasRAW = true
	pair.HasEdited = true

	handler := &AssetHandler{
		assetService: stubAssetService{
			queryBrowseFn: func(context.Context, service.QueryAssetsParams) (service.BrowseQueryResult, error) {
				return service.BrowseQueryResult{
					Items:           []service.BrowseItem{{Type: service.BrowseItemTypeMediaItem, ID: "media:" + pair.MediaItemID.String(), MediaItem: &pair}},
					TotalVisible:    1,
					TotalMediaItems: 1,
					TotalFiles:      2,
					StackMode:       service.StackModeCollapsed,
				}, nil
			},
		},
	}

	recorder := postJSON(t, "/api/v1/assets/list",
		queryAssetsRequest(service.StackModeCollapsed, dto.AssetFilterDTO{}), handler.QueryAssets)
	require.Equal(t, http.StatusOK, recorder.Code)

	var response dto.QueryAssetsResponseDTO
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Len(t, response.Items, 1)
	composition := response.Items[0].MediaItem.Composition
	require.Equal(t, 2, composition.ComponentCount)
	require.True(t, composition.HasRAW)
	require.True(t, composition.HasJPEG)
	require.True(t, composition.HasEdited)
	require.False(t, composition.HasLiveMotion)
}

func TestAssetHandlerQueryAssets_AcceptsCompositionAndStackFilters(t *testing.T) {
	gin.SetMode(gin.TestMode)

	composition := dto.MediaCompositionJPEGRAW
	membership := dto.StackMembershipStacked

	var captured service.QueryAssetsParams
	handler := &AssetHandler{
		assetService: stubAssetService{
			queryBrowseFn: func(_ context.Context, params service.QueryAssetsParams) (service.BrowseQueryResult, error) {
				captured = params
				return service.BrowseQueryResult{Items: []service.BrowseItem{}, StackMode: params.StackMode}, nil
			},
		},
	}

	recorder := postJSON(t, "/api/v1/assets/list", queryAssetsRequest(service.StackModeCollapsed, dto.AssetFilterDTO{
		MediaItem: &dto.MediaItemFilterDTO{Composition: &composition},
		Stack:     &dto.StackFilterDTO{Membership: &membership, Kinds: []string{"burst", "manual"}},
	}), handler.QueryAssets)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, service.MediaCompositionJPEGRAW, captured.MediaComposition)
	require.Equal(t, service.StackMembershipStacked, captured.StackMembership)
	require.Equal(t, []string{"burst", "manual"}, captured.StackKinds)
}

func TestAssetHandlerQueryAssets_RejectsInvalidBrowseFilters(t *testing.T) {
	gin.SetMode(gin.TestMode)

	unknownComposition := dto.MediaComposition("sidecar")
	unknownMembership := dto.StackMembership("grouped")
	unstacked := dto.StackMembershipUnstacked

	cases := map[string]dto.AssetFilterDTO{
		"unknown composition": {MediaItem: &dto.MediaItemFilterDTO{Composition: &unknownComposition}},
		"unknown membership":  {Stack: &dto.StackFilterDTO{Membership: &unknownMembership}},
		"unknown stack kind":  {Stack: &dto.StackFilterDTO{Kinds: []string{"panorama"}}},
		// Kinds imply "stacked", so pairing them with unstacked can only ever
		// return an empty, misleading page.
		"unstacked with kinds": {Stack: &dto.StackFilterDTO{Membership: &unstacked, Kinds: []string{"burst"}}},
	}

	for name, filter := range cases {
		t.Run(name, func(t *testing.T) {
			handler := &AssetHandler{assetService: stubAssetService{
				queryBrowseFn: func(context.Context, service.QueryAssetsParams) (service.BrowseQueryResult, error) {
					t.Fatal("service must not be called for an invalid filter")
					return service.BrowseQueryResult{}, nil
				},
			}}

			recorder := postJSON(t, "/api/v1/assets/list",
				queryAssetsRequest(service.StackModeCollapsed, filter), handler.QueryAssets)
			require.Equal(t, http.StatusBadRequest, recorder.Code)
		})
	}
}

func TestAssetHandlerQueryAssets_InvalidStackModeReturnsBadRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := &AssetHandler{assetService: stubAssetService{}}

	recorder := postJSON(t, "/api/v1/assets/list",
		queryAssetsRequest("fancy", dto.AssetFilterDTO{}), handler.QueryAssets)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestAssetHandlerSearchAssets_ReturnsFlatMediaItemRows(t *testing.T) {
	gin.SetMode(gin.TestMode)

	first := testBrowseMediaItem(t, "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "a.jpg")
	second := testBrowseMediaItem(t, "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", "b.jpg")

	items := []service.BrowseItem{
		{Type: service.BrowseItemTypeMediaItem, ID: "media:" + first.MediaItemID.String(), MediaItem: &first},
		{Type: service.BrowseItemTypeMediaItem, ID: "media:" + second.MediaItemID.String(), MediaItem: &second},
	}

	handler := &AssetHandler{
		assetService: stubAssetService{
			searchBrowseFn: func(context.Context, service.SearchAssetsParams) (service.SearchBrowseResult, error) {
				return service.SearchBrowseResult{
					TopResults:             items[:1],
					Results:                items,
					ResultsTotalVisible:    2,
					ResultsTotalMediaItems: 2,
				}, nil
			},
		},
	}

	recorder := postJSON(t, "/api/v1/assets/search",
		searchAssetsRequest("sunset", dto.AssetFilterDTO{}), handler.SearchAssets)
	require.Equal(t, http.StatusOK, recorder.Code)

	var response dto.SearchAssetsResponseDTO
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))

	require.Len(t, response.TopItems, 1)
	require.Equal(t, "media_item", response.TopItems[0].Type)
	require.Len(t, response.ResultItems, 2)
	for _, row := range response.ResultItems {
		require.Equal(t, "media_item", row.Type)
		require.Nil(t, row.Stack)
	}

	require.NotNil(t, response.ResultsTotalVisible)
	require.Equal(t, 2, *response.ResultsTotalVisible)
	require.NotNil(t, response.ResultsTotalMediaItems)
	require.Equal(t, 2, *response.ResultsTotalMediaItems)
}

// Search results are always flat: collapsing them would reorder the relevance
// set. stack_mode left the search contract entirely, and a request that still
// sends it is rejected rather than silently served under a different shape.
func TestAssetHandlerSearchAssets_RejectsStackModeInRequestBody(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := &AssetHandler{assetService: stubAssetService{
		searchBrowseFn: func(context.Context, service.SearchAssetsParams) (service.SearchBrowseResult, error) {
			t.Fatal("service must not be called for a request carrying stack_mode")
			return service.SearchBrowseResult{}, nil
		},
	}}

	for _, mode := range []string{"collapsed", "expanded"} {
		recorder := postJSON(t, "/api/v1/assets/search", map[string]any{
			"query":            "sunset",
			"sort_by":          "date_captured",
			"enhancement_mode": "auto",
			"stack_mode":       mode,
			"pagination":       map[string]int{"limit": 10, "offset": 0},
		}, handler.SearchAssets)
		require.Equal(t, http.StatusBadRequest, recorder.Code, "stack_mode=%s", mode)
	}
}

// The search response schema has no stack_mode field to echo back.
func TestAssetHandlerSearchAssets_ResponseOmitsStackMode(t *testing.T) {
	gin.SetMode(gin.TestMode)

	item := testBrowseMediaItem(t, "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "a.jpg")

	handler := &AssetHandler{
		assetService: stubAssetService{
			searchBrowseFn: func(context.Context, service.SearchAssetsParams) (service.SearchBrowseResult, error) {
				return service.SearchBrowseResult{
					Results:                []service.BrowseItem{{Type: service.BrowseItemTypeMediaItem, ID: "media:" + item.MediaItemID.String(), MediaItem: &item}},
					ResultsTotalVisible:    1,
					ResultsTotalMediaItems: 1,
				}, nil
			},
		},
	}

	recorder := postJSON(t, "/api/v1/assets/search",
		searchAssetsRequest("sunset", dto.AssetFilterDTO{}), handler.SearchAssets)
	require.Equal(t, http.StatusOK, recorder.Code)

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &raw))
	require.NotContains(t, raw, "stack_mode")
	require.NotContains(t, raw, "results_total_assets")
}

func TestAssetHandlerSearchAssets_RejectsInvalidBrowseFilters(t *testing.T) {
	gin.SetMode(gin.TestMode)

	unstacked := dto.StackMembershipUnstacked
	handler := &AssetHandler{assetService: stubAssetService{
		searchBrowseFn: func(context.Context, service.SearchAssetsParams) (service.SearchBrowseResult, error) {
			t.Fatal("service must not be called for an invalid filter")
			return service.SearchBrowseResult{}, nil
		},
	}}

	recorder := postJSON(t, "/api/v1/assets/search", searchAssetsRequest("sunset", dto.AssetFilterDTO{
		Stack: &dto.StackFilterDTO{Membership: &unstacked, Kinds: []string{"burst"}},
	}), handler.SearchAssets)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
}
