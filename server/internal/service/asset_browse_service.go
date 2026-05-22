package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"server/internal/db/repo"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/pgvector/pgvector-go"
)

// BrowseStack is stack metadata attached to a collapsed browse row. MemberAssetIDs is the full
// stack membership order; MatchedMemberIDs lists members that matched the current query (e.g. vector hits).
type BrowseStack struct {
	StackID          uuid.UUID
	CoverAssetID     uuid.UUID
	MemberAssetIDs   []uuid.UUID
	MatchedMemberIDs []uuid.UUID
}

// preferredStackFocusAssetID returns the asset that should be focused first
// for semantic stack interactions. Matched members win; otherwise fall back to
// the canonical cover asset.
func preferredStackFocusAssetID(stack *BrowseStack) uuid.UUID {
	if stack == nil {
		return uuid.Nil
	}

	for _, assetID := range stack.MatchedMemberIDs {
		if assetID != uuid.Nil {
			return assetID
		}
	}

	if stack.CoverAssetID != uuid.Nil {
		return stack.CoverAssetID
	}

	for _, assetID := range stack.MemberAssetIDs {
		if assetID != uuid.Nil {
			return assetID
		}
	}

	return uuid.Nil
}

// BrowseItem is one gallery row: Type is "asset" or "stack", ID is a stable prefixed key ("asset:..." / "stack:...").
// Asset always carries the thumbnail row payload (cover for stacks). Stack is set only when Type is "stack".
type BrowseItem struct {
	Type  string
	ID    string
	Asset repo.Asset
	Stack *BrowseStack
}

// BrowseQueryResult is the unified browse/list response: Items matches pagination, TotalVisible counts rows
// after stack collapse, TotalAssets counts underlying assets, StackMode echoes the resolved mode.
type BrowseQueryResult struct {
	Items        []BrowseItem
	TotalVisible int64
	TotalAssets  int64
	StackMode    string
}

// SearchBrowseResult combines optional CLIP "top results" with the main filename-based browse listing.
// TopResults may overlap Results; callers typically dedupe by BrowseItem.ID (see filterOutBrowseItemsByID).
type SearchBrowseResult struct {
	TopResults          []BrowseItem
	TopResultsMeta      SearchTopResultsMeta
	Results             []BrowseItem
	ResultsTotalVisible int64
	ResultsTotalAssets  int64
	StackMode           string
}

// normalizeStackMode maps client input to StackModeCollapsed or StackModeExpanded; unknown values collapse.
func normalizeStackMode(mode string) string {
	switch strings.TrimSpace(mode) {
	case "", StackModeCollapsed:
		return StackModeCollapsed
	case StackModeExpanded:
		return StackModeExpanded
	default:
		return StackModeCollapsed
	}
}

// QueryBrowseItems returns paginated browse rows for the gallery. Expanded mode forwards to QueryAssets
// (one row per asset). Collapsed mode uses SQL unified collapsed queries, except semantic search with a non-empty
// query, where vector-ranked assets are collapsed in application code via collapseAssetsToBrowseItems.
func (s *assetService) QueryBrowseItems(ctx context.Context, params QueryAssetsParams) (BrowseQueryResult, error) {
	params.StackMode = normalizeStackMode(params.StackMode)

	if params.StackMode == StackModeExpanded {
		assets, total, err := s.QueryAssets(ctx, params)
		if err != nil {
			return BrowseQueryResult{}, err
		}
		return BrowseQueryResult{
			Items:        assetsToBrowseItems(assets),
			TotalVisible: total,
			TotalAssets:  total,
			StackMode:    params.StackMode,
		}, nil
	}

	if params.SearchType == "semantic" && strings.TrimSpace(params.Query) != "" {
		return s.queryCollapsedSemanticBrowseItems(ctx, params)
	}

	totalAssets, err := s.countAssetsUnified(ctx, params)
	if err != nil {
		return BrowseQueryResult{}, err
	}
	totalVisible, err := s.countCollapsedBrowseItemsUnified(ctx, params)
	if err != nil {
		return BrowseQueryResult{}, err
	}
	rows, err := s.getCollapsedBrowseItemsUnified(ctx, params)
	if err != nil {
		return BrowseQueryResult{}, err
	}
	items, err := browseItemsFromCollapsedRows(rows)
	if err != nil {
		return BrowseQueryResult{}, err
	}
	return BrowseQueryResult{
		Items:        items,
		TotalVisible: totalVisible,
		TotalAssets:  totalAssets,
		StackMode:    params.StackMode,
	}, nil
}

// SearchBrowseItems runs hybrid search: optional CLIP-enhanced TopResults plus filename QueryBrowseItems as Results,
// respecting EnhancementMode (off / auto / only). Duplicate IDs between sections are removed from Results and totals adjusted.
func (s *assetService) SearchBrowseItems(ctx context.Context, params SearchAssetsParams) (SearchBrowseResult, error) {
	params = normalizeSearchAssetsParams(params)
	params.StackMode = normalizeStackMode(params.StackMode)

	result := SearchBrowseResult{
		TopResults: []BrowseItem{},
		TopResultsMeta: SearchTopResultsMeta{
			Enabled:     false,
			SourceTypes: []string{},
		},
		Results:   []BrowseItem{},
		StackMode: params.StackMode,
	}

	query := strings.TrimSpace(params.Query)
	topResultsEnabled := query != "" && params.EnhancementMode != SearchEnhancementModeOff

	if topResultsEnabled {
		topItems, meta := s.searchBrowseItemsClipTopResults(ctx, params)
		result.TopResults = topItems
		result.TopResultsMeta = meta
	}

	if params.EnhancementMode != SearchEnhancementModeOnly {
		filenameParams := params.QueryAssetsParams
		filenameParams.Query = query
		filenameParams.SearchType = "filename"

		browseResult, err := s.QueryBrowseItems(ctx, filenameParams)
		if err != nil {
			return SearchBrowseResult{}, err
		}

		filteredResults, removedVisible, removedAssets := filterOutBrowseItemsByID(
			browseResult.Items,
			result.TopResults,
		)
		if removedVisible > 0 {
			browseResult.TotalVisible = subtractBrowseCount(
				browseResult.TotalVisible,
				int64(removedVisible),
			)
		}
		if removedAssets > 0 {
			browseResult.TotalAssets = subtractBrowseCount(
				browseResult.TotalAssets,
				int64(removedAssets),
			)
		}

		result.Results = filteredResults
		result.ResultsTotalVisible = browseResult.TotalVisible
		result.ResultsTotalAssets = browseResult.TotalAssets
	}

	if !topResultsEnabled {
		switch {
		case params.EnhancementMode == SearchEnhancementModeOff:
			result.TopResultsMeta = SearchTopResultsMeta{
				Enabled:     false,
				Reason:      "disabled",
				SourceTypes: []string{},
			}
		case query == "":
			result.TopResultsMeta = SearchTopResultsMeta{
				Enabled:     false,
				Reason:      "empty_query",
				SourceTypes: []string{},
			}
		}
	}

	return result, nil
}

func (s *assetService) queryCollapsedSemanticBrowseItems(ctx context.Context, params QueryAssetsParams) (BrowseQueryResult, error) {
	embeddingResult, err := s.resolveClipQueryEmbedding(ctx, params.Query, false)
	if err != nil {
		return BrowseQueryResult{}, err
	}

	space, err := s.embeddingService.ResolveDefaultSearchSpace(ctx, EmbeddingTypeCLIP, embeddingResult.ModelID, len(embeddingResult.Vector))
	if err != nil {
		return BrowseQueryResult{}, err
	}

	queryVector := pgvector.NewVector(embeddingResult.Vector)
	totalAssets, err := s.countAssetsBySemanticSpace(ctx, params, space, &queryVector)
	if err != nil {
		return BrowseQueryResult{}, err
	}
	if totalAssets == 0 {
		return browseQueryResultFromItems(
			[]BrowseItem{},
			0,
			params.StackMode,
			params.Limit,
			params.Offset,
		), nil
	}

	assets, err := s.searchAssetsBySemanticSpace(ctx, params, space, &queryVector, int(totalAssets), 0)
	if err != nil {
		return BrowseQueryResult{}, err
	}
	items, err := s.collapseAssetsToBrowseItems(ctx, assets)
	if err != nil {
		return BrowseQueryResult{}, err
	}

	return browseQueryResultFromItems(
		items,
		totalAssets,
		params.StackMode,
		params.Limit,
		params.Offset,
	), nil
}

// searchBrowseItemsClipTopResults fetches extra vector-ranked assets (fetchLimit ~= 4× cap) under a short timeout,
// then applies stack collapse when not expanded. Marks meta degraded on timeout/vector errors or collapse failure.
func (s *assetService) searchBrowseItemsClipTopResults(ctx context.Context, params SearchAssetsParams) ([]BrowseItem, SearchTopResultsMeta) {
	meta := SearchTopResultsMeta{
		Enabled:     true,
		SourceTypes: []string{"clip"},
	}

	searchCtx, cancel := context.WithTimeout(ctx, 750*time.Millisecond)
	defer cancel()

	requestedLimit := params.TopResultsLimit
	if requestedLimit <= 0 {
		requestedLimit = TOP_RESULTS_FALLBACK_LIMIT
	}

	fetchLimit := requestedLimit * 4
	if fetchLimit < requestedLimit {
		fetchLimit = requestedLimit
	}

	assets, err := s.queryAssetsVectorTopResults(searchCtx, params.QueryAssetsParams, fetchLimit)
	if err != nil {
		meta.Degraded = true
		meta.Reason = classifySearchEnhancementError(err, searchCtx)
		return []BrowseItem{}, meta
	}

	if params.StackMode == StackModeExpanded {
		items := assetsToBrowseItems(assets)
		if len(items) > requestedLimit {
			items = items[:requestedLimit]
		}
		return items, meta
	}

	items, err := s.collapseAssetsToBrowseItems(searchCtx, assets)
	if err != nil {
		meta.Degraded = true
		meta.Reason = "collapse_failed"
		return []BrowseItem{}, meta
	}
	if len(items) > requestedLimit {
		items = items[:requestedLimit]
	}
	return items, meta
}

// TOP_RESULTS_FALLBACK_LIMIT is the default CLIP top-result cap when the client does not specify one.
const TOP_RESULTS_FALLBACK_LIMIT = 12

// assetsToBrowseItems maps plain asset rows to browse items without stack merging (expanded / pre-collapsed paths).
func assetsToBrowseItems(assets []repo.Asset) []BrowseItem {
	items := make([]BrowseItem, 0, len(assets))
	for _, asset := range assets {
		assetID, ok := uuidFromPgUUID(asset.AssetID)
		if !ok {
			continue
		}
		items = append(items, BrowseItem{
			Type:  "asset",
			ID:    "asset:" + assetID.String(),
			Asset: asset,
		})
	}
	return items
}

// browseItemsFromCollapsedRows converts SQL collapsed-browse rows into BrowseItem values (asset vs stack rows).
func browseItemsFromCollapsedRows(rows []repo.GetCollapsedBrowseItemsUnifiedRow) ([]BrowseItem, error) {
	items := make([]BrowseItem, 0, len(rows))
	for _, row := range rows {
		coverAssetID, ok := uuidFromPgUUID(row.Asset.AssetID)
		if !ok {
			continue
		}

		if row.ItemType == "asset" {
			items = append(items, BrowseItem{
				Type:  "asset",
				ID:    "asset:" + coverAssetID.String(),
				Asset: row.Asset,
			})
			continue
		}

		stackID, ok := uuidFromPgUUID(row.StackID)
		if !ok {
			return nil, fmt.Errorf("collapsed browse row missing stack id for cover %s", coverAssetID.String())
		}

		items = append(items, BrowseItem{
			Type:  "stack",
			ID:    "stack:" + stackID.String(),
			Asset: row.Asset,
			Stack: &BrowseStack{
				StackID:          stackID,
				CoverAssetID:     coverAssetID,
				MemberAssetIDs:   uuidSliceFromPgUUIDs(row.MemberAssetIds),
				MatchedMemberIDs: uuidSliceFromPgUUIDs(row.MatchedAssetIds),
			},
		})
	}
	return items, nil
}

// filterOutBrowseItemsByID drops items whose BrowseItem.ID appears in excluded.
// It returns the filtered slice plus removed visible-row and matched-asset counts.
func filterOutBrowseItemsByID(items []BrowseItem, excluded []BrowseItem) ([]BrowseItem, int, int) {
	if len(items) == 0 || len(excluded) == 0 {
		return items, 0, 0
	}

	excludedIDs := make(map[string]struct{}, len(excluded))
	for _, item := range excluded {
		excludedIDs[item.ID] = struct{}{}
	}

	filtered := make([]BrowseItem, 0, len(items))
	removedVisible := 0
	removedAssets := 0
	for _, item := range items {
		if _, found := excludedIDs[item.ID]; found {
			removedVisible++
			removedAssets += browseItemMatchedAssetCount(item)
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered, removedVisible, removedAssets
}

func browseItemMatchedAssetCount(item BrowseItem) int {
	if item.Type == "stack" && item.Stack != nil && len(item.Stack.MatchedMemberIDs) > 0 {
		return len(item.Stack.MatchedMemberIDs)
	}
	return 1
}

func browseQueryResultFromItems(items []BrowseItem, totalAssets int64, stackMode string, limit, offset int) BrowseQueryResult {
	return BrowseQueryResult{
		Items:        pageBrowseItems(items, limit, offset),
		TotalVisible: int64(len(items)),
		TotalAssets:  totalAssets,
		StackMode:    stackMode,
	}
}

func pageBrowseItems(items []BrowseItem, limit, offset int) []BrowseItem {
	if len(items) == 0 || limit <= 0 {
		return []BrowseItem{}
	}
	if offset < 0 {
		offset = 0
	}
	if offset >= len(items) {
		return []BrowseItem{}
	}

	end := offset + limit
	if end < offset || end > len(items) {
		end = len(items)
	}

	page := make([]BrowseItem, end-offset)
	copy(page, items[offset:end])
	return page
}

func subtractBrowseCount(total int64, removed int64) int64 {
	if removed <= 0 {
		return total
	}
	if total <= removed {
		return 0
	}
	return total - removed
}

// collapseAssetsToBrowseItems groups vector- or list-ranked assets into stack rows: each stack emits once in input order,
// using the stack cover asset for thumbnails and MemberAssetIDs from DB; MatchedMemberIDs lists input assets belonging to that stack.
func (s *assetService) collapseAssetsToBrowseItems(ctx context.Context, assets []repo.Asset) ([]BrowseItem, error) {
	if len(assets) == 0 {
		return []BrowseItem{}, nil
	}

	assetIDs := make([]pgtype.UUID, 0, len(assets))
	for _, asset := range assets {
		if asset.AssetID.Valid {
			assetIDs = append(assetIDs, asset.AssetID)
		}
	}

	stackRows, err := s.queries.GetStacksByAssetIDs(ctx, assetIDs)
	if err != nil {
		return nil, fmt.Errorf("get stack memberships: %w", err)
	}

	type membership struct {
		stackID  uuid.UUID
		position int32
	}

	membershipByAssetID := make(map[uuid.UUID]membership, len(stackRows))
	stackSeen := make(map[uuid.UUID]struct{})
	stackOrder := make([]uuid.UUID, 0, len(stackRows))
	for _, row := range stackRows {
		assetID, okAsset := uuidFromPgUUID(row.AssetID)
		stackID, okStack := uuidFromPgUUID(row.StackID)
		if !okAsset || !okStack {
			continue
		}
		position := int32(0)
		if row.Position != nil {
			position = *row.Position
		}
		membershipByAssetID[assetID] = membership{stackID: stackID, position: position}
		if _, exists := stackSeen[stackID]; !exists {
			stackSeen[stackID] = struct{}{}
			stackOrder = append(stackOrder, stackID)
		}
	}

	memberIDsByStack := make(map[uuid.UUID][]uuid.UUID, len(stackOrder))
	matchedIDsByStack := make(map[uuid.UUID][]uuid.UUID, len(stackOrder))
	coverIDsByStack := make(map[uuid.UUID]uuid.UUID, len(stackOrder))

	for _, stackID := range stackOrder {
		members, err := s.queries.GetStackMembers(ctx, pgtype.UUID{Bytes: stackID, Valid: true})
		if err != nil {
			return nil, fmt.Errorf("get stack members for %s: %w", stackID.String(), err)
		}
		memberIDs := make([]uuid.UUID, 0, len(members))
		coverID := uuid.Nil
		coverPosition := int32(0)
		for index, member := range members {
			memberID, ok := uuidFromPgUUID(member.AssetID)
			if !ok {
				continue
			}
			memberIDs = append(memberIDs, memberID)
			position := int32(0)
			if member.Position != nil {
				position = *member.Position
			}
			if index == 0 || position < coverPosition {
				coverID = memberID
				coverPosition = position
			}
		}
		memberIDsByStack[stackID] = memberIDs
		coverIDsByStack[stackID] = coverID
	}

	for _, asset := range assets {
		assetID, ok := uuidFromPgUUID(asset.AssetID)
		if !ok {
			continue
		}
		membershipInfo, isStacked := membershipByAssetID[assetID]
		if !isStacked {
			continue
		}
		matchedIDsByStack[membershipInfo.stackID] = append(
			matchedIDsByStack[membershipInfo.stackID],
			assetID,
		)
	}

	assetByID := make(map[uuid.UUID]repo.Asset, len(assets))
	missingCoverIDs := make([]pgtype.UUID, 0)
	for _, asset := range assets {
		assetID, ok := uuidFromPgUUID(asset.AssetID)
		if !ok {
			continue
		}
		assetByID[assetID] = asset
	}
	for _, coverID := range coverIDsByStack {
		if coverID == uuid.Nil {
			continue
		}
		if _, ok := assetByID[coverID]; ok {
			continue
		}
		missingCoverIDs = append(missingCoverIDs, pgtype.UUID{Bytes: coverID, Valid: true})
	}
	if len(missingCoverIDs) > 0 {
		coverAssets, err := s.queries.GetAssetsByIDs(ctx, missingCoverIDs)
		if err != nil {
			return nil, fmt.Errorf("get cover assets: %w", err)
		}
		for _, asset := range coverAssets {
			assetID, ok := uuidFromPgUUID(asset.AssetID)
			if !ok {
				continue
			}
			assetByID[assetID] = asset
		}
	}

	items := make([]BrowseItem, 0, len(assets))
	seenStacks := make(map[uuid.UUID]struct{}, len(stackOrder))
	for _, asset := range assets {
		assetID, ok := uuidFromPgUUID(asset.AssetID)
		if !ok {
			continue
		}

		membershipInfo, isStacked := membershipByAssetID[assetID]
		if !isStacked {
			items = append(items, BrowseItem{
				Type:  "asset",
				ID:    "asset:" + assetID.String(),
				Asset: asset,
			})
			continue
		}

		if _, exists := seenStacks[membershipInfo.stackID]; exists {
			continue
		}
		seenStacks[membershipInfo.stackID] = struct{}{}

		coverID := coverIDsByStack[membershipInfo.stackID]
		representative, ok := assetByID[coverID]
		if !ok {
			representative = asset
			coverID = assetID
		}

		items = append(items, BrowseItem{
			Type:  "stack",
			ID:    "stack:" + membershipInfo.stackID.String(),
			Asset: representative,
			Stack: &BrowseStack{
				StackID:          membershipInfo.stackID,
				CoverAssetID:     coverID,
				MemberAssetIDs:   memberIDsByStack[membershipInfo.stackID],
				MatchedMemberIDs: matchedIDsByStack[membershipInfo.stackID],
			},
		})
	}

	return items, nil
}

// countAssetsUnified counts assets matching filters (ignores stack collapse).
func (s *assetService) countAssetsUnified(ctx context.Context, params QueryAssetsParams) (int64, error) {
	repoUUID, ratingPtr, fromTime, toTime, queryPtr, err := normalizeBrowseRepoParams(params)
	if err != nil {
		return 0, err
	}
	return s.queries.CountAssetsUnified(ctx, repo.CountAssetsUnifiedParams{
		AssetType:        params.AssetType,
		AssetTypes:       params.AssetTypes,
		RepositoryID:     repoUUID,
		PersonID:         params.PersonID,
		OwnerID:          params.OwnerID,
		AlbumID:          params.AlbumID,
		Query:            queryPtr,
		FilenameVal:      params.FilenameValue,
		FilenameOperator: params.FilenameOperator,
		IsRaw:            params.IsRaw,
		Rating:           ratingPtr,
		Liked:            params.Liked,
		CameraModel:      params.CameraModel,
		LensModel:        params.LensModel,
		LocationNorth:    params.LocationNorth,
		LocationSouth:    params.LocationSouth,
		LocationEast:     params.LocationEast,
		LocationWest:     params.LocationWest,
		DateFrom:         fromTime,
		DateTo:           toTime,
	})
}

// countCollapsedBrowseItemsUnified counts visible browse rows after stack collapse under the same filters.
func (s *assetService) countCollapsedBrowseItemsUnified(ctx context.Context, params QueryAssetsParams) (int64, error) {
	repoUUID, ratingPtr, fromTime, toTime, queryPtr, err := normalizeBrowseRepoParams(params)
	if err != nil {
		return 0, err
	}
	return s.queries.CountCollapsedBrowseItemsUnified(ctx, repo.CountCollapsedBrowseItemsUnifiedParams{
		Query:            queryPtr,
		AssetType:        params.AssetType,
		AssetTypes:       params.AssetTypes,
		OwnerID:          params.OwnerID,
		RepositoryID:     repoUUID,
		PersonID:         params.PersonID,
		AlbumID:          params.AlbumID,
		FilenameVal:      params.FilenameValue,
		FilenameOperator: params.FilenameOperator,
		DateFrom:         fromTime,
		DateTo:           toTime,
		IsRaw:            params.IsRaw,
		Rating:           ratingPtr,
		Liked:            params.Liked,
		CameraModel:      params.CameraModel,
		LensModel:        params.LensModel,
		LocationNorth:    params.LocationNorth,
		LocationSouth:    params.LocationSouth,
		LocationEast:     params.LocationEast,
		LocationWest:     params.LocationWest,
	})
}

// getCollapsedBrowseItemsUnified loads one page of collapsed browse rows (assets and stacks) with sort/offset/limit.
func (s *assetService) getCollapsedBrowseItemsUnified(ctx context.Context, params QueryAssetsParams) ([]repo.GetCollapsedBrowseItemsUnifiedRow, error) {
	repoUUID, ratingPtr, fromTime, toTime, queryPtr, err := normalizeBrowseRepoParams(params)
	if err != nil {
		return nil, err
	}
	sortByPtr := normalizeSortByPointer(params.SortBy)
	return s.queries.GetCollapsedBrowseItemsUnified(ctx, repo.GetCollapsedBrowseItemsUnifiedParams{
		Query:            queryPtr,
		AssetType:        params.AssetType,
		AssetTypes:       params.AssetTypes,
		OwnerID:          params.OwnerID,
		RepositoryID:     repoUUID,
		PersonID:         params.PersonID,
		AlbumID:          params.AlbumID,
		FilenameVal:      params.FilenameValue,
		FilenameOperator: params.FilenameOperator,
		DateFrom:         fromTime,
		DateTo:           toTime,
		IsRaw:            params.IsRaw,
		Rating:           ratingPtr,
		Liked:            params.Liked,
		CameraModel:      params.CameraModel,
		LensModel:        params.LensModel,
		LocationNorth:    params.LocationNorth,
		LocationSouth:    params.LocationSouth,
		LocationEast:     params.LocationEast,
		LocationWest:     params.LocationWest,
		SortBy:           sortByPtr,
		Offset:           int32(params.Offset),
		Limit:            int32(params.Limit),
	})
}

// normalizeBrowseRepoParams translates API QueryAssetsParams into pgx/repo types shared by unified browse queries.
func normalizeBrowseRepoParams(params QueryAssetsParams) (pgtype.UUID, *int32, pgtype.Timestamptz, pgtype.Timestamptz, *string, error) {
	var repoUUID pgtype.UUID
	if params.RepositoryID != nil && *params.RepositoryID != "" {
		parsedUUID, err := uuid.Parse(*params.RepositoryID)
		if err != nil {
			return pgtype.UUID{}, nil, pgtype.Timestamptz{}, pgtype.Timestamptz{}, nil, fmt.Errorf("invalid repository ID: %w", err)
		}
		repoUUID = pgtype.UUID{Bytes: parsedUUID, Valid: true}
	}

	var ratingPtr *int32
	if params.Rating != nil {
		rating := int32(*params.Rating)
		ratingPtr = &rating
	}

	var fromTime, toTime pgtype.Timestamptz
	if params.DateFrom != nil {
		fromTime = pgtype.Timestamptz{Time: *params.DateFrom, Valid: true}
	}
	if params.DateTo != nil {
		toTime = pgtype.Timestamptz{Time: *params.DateTo, Valid: true}
	}

	var queryPtr *string
	if strings.TrimSpace(params.Query) != "" {
		query := strings.TrimSpace(params.Query)
		queryPtr = &query
	}

	return repoUUID, ratingPtr, fromTime, toTime, queryPtr, nil
}

// normalizeSortByPointer returns a repo sort key for recognized SortBy values, or nil for default ordering.
func normalizeSortByPointer(sortBy string) *string {
	switch sortBy {
	case "recently_added":
		value := "recently_added"
		return &value
	case "date_captured":
		value := "date_captured"
		return &value
	default:
		return nil
	}
}

// uuidFromPgUUID converts pgtype.UUID to google/uuid when Valid.
func uuidFromPgUUID(value pgtype.UUID) (uuid.UUID, bool) {
	if !value.Valid {
		return uuid.Nil, false
	}
	return value.Bytes, true
}

// uuidSliceFromPgUUIDs filters invalid pgtype.UUID entries and returns a dense []uuid.UUID.
func uuidSliceFromPgUUIDs(values []pgtype.UUID) []uuid.UUID {
	if len(values) == 0 {
		return nil
	}
	result := make([]uuid.UUID, 0, len(values))
	for _, value := range values {
		converted, ok := uuidFromPgUUID(value)
		if !ok {
			continue
		}
		result = append(result, converted)
	}
	return result
}
