package service

import (
	"context"
	"fmt"
	"strings"

	"server/internal/db/dbtypes"
	"server/internal/db/repo"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/pgvector/pgvector-go"
)

// BrowseStack is stack metadata attached to a collapsed browse row. MemberAssetIDs is the full
// stack membership order; MatchedMemberIDs lists members that matched the current query (e.g. vector hits).
type BrowseStack struct {
	StackID          uuid.UUID
	Kind             dbtypes.StackKind
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

// SearchBrowseResult combines optional semantic "top results" with the main filename-based browse listing.
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
		return s.queryCollapsedAggregateBrowseItems(ctx, params)
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
	if err := s.attachBrowseStackKinds(ctx, items); err != nil {
		return BrowseQueryResult{}, err
	}
	return BrowseQueryResult{
		Items:        items,
		TotalVisible: totalVisible,
		TotalAssets:  totalAssets,
		StackMode:    params.StackMode,
	}, nil
}

// SearchBrowseItems runs hybrid search: optional semantic-enhanced TopResults plus filename QueryBrowseItems as Results,
// respecting EnhancementMode (off / auto / only). Duplicate IDs between sections are removed from Results and totals adjusted.
// SearchBrowseItems is the browse-tier face of the unified search pipeline
// (see asset_search_fused.go). One fused set: Results is the whole set under
// the presentation sort, Best Results (TopResults) is its confidence-ordered
// Top-N subset. Both tiers are flat — search results are not stack-collapsed
// (matches Apple Photos, and collapse would reorder the relevance set),
// keeping Best Results a literal subset of Results.
func (s *assetService) SearchBrowseItems(ctx context.Context, params SearchAssetsParams) (SearchBrowseResult, error) {
	params = normalizeSearchAssetsParams(params)
	params.StackMode = normalizeStackMode(params.StackMode)

	result := SearchBrowseResult{
		TopResults:     []BrowseItem{},
		TopResultsMeta: SearchTopResultsMeta{Enabled: false, SourceTypes: []string{}},
		Results:        []BrowseItem{},
		StackMode:      params.StackMode,
	}

	query := strings.TrimSpace(params.Query)
	enhanced := query != "" && params.EnhancementMode != SearchEnhancementModeOff

	if enhanced {
		if fused, ok := s.runSearchAssetsFusedSet(ctx, params); ok {
			result.TopResultsMeta = fused.meta()
			ids := fused.ids()

			// Best Results exists only when the set exceeds the showcase size.
			if len(ids) >= params.TopResultsLimit {
				topAssets, err := s.runHydrateAssetsInOrder(ctx, ids[:params.TopResultsLimit], params.IsDeleted)
				if err != nil {
					return SearchBrowseResult{}, err
				}
				result.TopResults = assetsToBrowseItems(topAssets)
			}

			if params.EnhancementMode != SearchEnhancementModeOnly {
				page, err := s.runPageAssetsBySort(ctx, ids, params.SortBy, params.Limit, params.Offset, params.IsDeleted)
				if err != nil {
					return SearchBrowseResult{}, err
				}
				result.Results = assetsToBrowseItems(page)
				result.ResultsTotalVisible = int64(len(ids))
				result.ResultsTotalAssets = int64(len(ids))
			}
			return result, nil
		}

		if params.EnhancementMode == SearchEnhancementModeOnly {
			return SearchBrowseResult{}, fmt.Errorf("aggregate search failed")
		}
		result.TopResultsMeta = SearchTopResultsMeta{
			Enabled:     true,
			Degraded:    true,
			Reason:      semanticUnavailableReason,
			SourceTypes: []string{},
		}
	}

	if params.EnhancementMode != SearchEnhancementModeOnly {
		filenameParams := params.QueryAssetsParams
		filenameParams.Query = query
		filenameParams.SearchType = "filename"

		browseResult, err := s.QueryBrowseItems(ctx, filenameParams)
		if err != nil {
			return SearchBrowseResult{}, err
		}
		result.Results = browseResult.Items
		result.ResultsTotalVisible = browseResult.TotalVisible
		result.ResultsTotalAssets = browseResult.TotalAssets
	}

	if !enhanced {
		switch {
		case params.EnhancementMode == SearchEnhancementModeOff:
			result.TopResultsMeta = SearchTopResultsMeta{Enabled: false, Reason: "disabled", SourceTypes: []string{}}
		case query == "":
			result.TopResultsMeta = SearchTopResultsMeta{Enabled: false, Reason: "empty_query", SourceTypes: []string{}}
		}
	}

	return result, nil
}

func (s *assetService) queryCollapsedSemanticBrowseItems(ctx context.Context, params QueryAssetsParams) (BrowseQueryResult, error) {
	embeddingResult, err := s.resolveSemanticQueryEmbedding(ctx, params.Query, false)
	if err != nil {
		return BrowseQueryResult{}, err
	}

	space, err := s.embeddingService.ResolveDefaultSearchSpace(ctx, EmbeddingTypeSemantic, embeddingResult.ModelID, len(embeddingResult.Vector))
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

func (s *assetService) queryCollapsedAggregateBrowseItems(ctx context.Context, params QueryAssetsParams) (BrowseQueryResult, error) {
	assets, totalAssets, err := s.queryAssetsAggregate(ctx, QueryAssetsParams{
		Query:            params.Query,
		SearchType:       params.SearchType,
		ViewerTimeZone:   params.ViewerTimeZone,
		RepositoryID:     params.RepositoryID,
		PersonID:         params.PersonID,
		AssetType:        params.AssetType,
		AssetTypes:       cloneStringSlice(params.AssetTypes),
		OwnerID:          params.OwnerID,
		AlbumID:          params.AlbumID,
		FilenameValue:    params.FilenameValue,
		FilenameOperator: params.FilenameOperator,
		DateFrom:         params.DateFrom,
		DateTo:           params.DateTo,
		IsRaw:            params.IsRaw,
		Rating:           params.Rating,
		Liked:            params.Liked,
		CameraModel:      params.CameraModel,
		LensModel:        params.LensModel,
		TagName:          params.TagName,
		TagSource:        params.TagSource,
		LocationNorth:    params.LocationNorth,
		LocationSouth:    params.LocationSouth,
		LocationEast:     params.LocationEast,
		LocationWest:     params.LocationWest,
		SortBy:           params.SortBy,
		StackMode:        params.StackMode,
		Limit:            aggregateCandidatePoolSize(params.Limit, params.Offset),
		Offset:           0,
	})
	if err != nil {
		return BrowseQueryResult{}, err
	}
	if len(assets) == 0 {
		return browseQueryResultFromItems(
			[]BrowseItem{},
			totalAssets,
			params.StackMode,
			params.Limit,
			params.Offset,
		), nil
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

	if err := s.attachBrowseStackKinds(ctx, items); err != nil {
		return nil, err
	}

	return items, nil
}

func (s *assetService) attachBrowseStackKinds(ctx context.Context, items []BrowseItem) error {
	if s == nil || s.pool == nil || len(items) == 0 {
		return nil
	}

	stackIDs := make([]pgtype.UUID, 0)
	seen := make(map[uuid.UUID]struct{})
	for _, item := range items {
		if item.Stack == nil || item.Type != "stack" {
			continue
		}
		stackID := item.Stack.StackID
		if stackID == uuid.Nil {
			continue
		}
		if _, exists := seen[stackID]; exists {
			continue
		}
		seen[stackID] = struct{}{}
		stackIDs = append(stackIDs, pgtype.UUID{Bytes: stackID, Valid: true})
	}

	if len(stackIDs) == 0 {
		return nil
	}

	rows, err := s.pool.Query(ctx, `SELECT stack_id, stack_kind FROM asset_stacks WHERE stack_id = ANY($1::uuid[])`, stackIDs)
	if err != nil {
		return fmt.Errorf("get browse stack kinds: %w", err)
	}
	defer rows.Close()

	kindByStackID := make(map[uuid.UUID]dbtypes.StackKind, len(stackIDs))
	for rows.Next() {
		var stackID pgtype.UUID
		var stackKind dbtypes.StackKind
		if err := rows.Scan(&stackID, &stackKind); err != nil {
			return fmt.Errorf("scan browse stack kinds: %w", err)
		}
		if parsed, ok := uuidFromPgUUID(stackID); ok {
			kindByStackID[parsed] = stackKind
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate browse stack kinds: %w", err)
	}

	for i := range items {
		if items[i].Stack == nil {
			continue
		}
		if kind, ok := kindByStackID[items[i].Stack.StackID]; ok {
			items[i].Stack.Kind = kind
		}
	}

	return nil
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
		TagName:          params.TagName,
		TagSource:        params.TagSource,
		LocationNorth:    params.LocationNorth,
		LocationSouth:    params.LocationSouth,
		LocationEast:     params.LocationEast,
		LocationWest:     params.LocationWest,
		DateFrom:         fromTime,
		DateTo:           toTime,
		IsDeleted:        params.IsDeleted,
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
		TagName:          params.TagName,
		TagSource:        params.TagSource,
		LocationNorth:    params.LocationNorth,
		LocationSouth:    params.LocationSouth,
		LocationEast:     params.LocationEast,
		LocationWest:     params.LocationWest,
		IsDeleted:        params.IsDeleted,
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
		TagName:          params.TagName,
		TagSource:        params.TagSource,
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
		IsDeleted:        params.IsDeleted,
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
