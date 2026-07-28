package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"server/internal/db/dbtypes"
	"server/internal/db/repo"

	"github.com/google/uuid"
)

// ErrInvalidBrowseFilter rejects contradictory browse filter combinations
// (e.g. stack.membership=unstacked with non-empty stack.kinds). Handlers map
// it to HTTP 400.
var ErrInvalidBrowseFilter = errors.New("invalid browse filter combination")

// Browse item type discriminators: the gallery's smallest user-facing unit is
// the logical media item; a stack row groups media items for presentation.
const (
	BrowseItemTypeMediaItem = "media_item"
	BrowseItemTypeStack     = "stack"
)

// BrowseMediaItem is one logical media item as browsed: the embedded primary
// asset carries the thumbnail/detail payload, the composition facts describe
// the component files, and the stack fields locate the item's presentation
// stack (zero values when unstacked).
type BrowseMediaItem struct {
	MediaItemID    uuid.UUID
	MediaKind      string
	PrimaryAsset   repo.Asset
	ComponentCount int
	HasRAW         bool
	HasJPEG        bool
	HasEdited      bool
	HasLiveMotion  bool
	StackID        uuid.UUID
	StackKind      dbtypes.StackKind
}

// BrowseStackMember references one stack member at media-item granularity.
type BrowseStackMember struct {
	MediaItemID    uuid.UUID
	PrimaryAssetID uuid.UUID
}

// BrowseStack is a collapsed presentation-stack row. Members is the full
// visible membership in stack order; MatchedMembers lists members that
// matched the current query (equal to Members for plain listing).
type BrowseStack struct {
	StackID        uuid.UUID
	Kind           dbtypes.StackKind
	Cover          BrowseMediaItem
	Members        []BrowseStackMember
	MatchedMembers []BrowseStackMember
}

// BrowseItem is one gallery row: Type is "media_item" or "stack", ID is a
// stable prefixed key ("media:..." / "stack:..."). Exactly one of MediaItem
// and Stack is set. BestTsMs is search-hit metadata (nearest matching video
// frame); nil for photos / non-search browse.
type BrowseItem struct {
	Type      string
	ID        string
	MediaItem *BrowseMediaItem
	Stack     *BrowseStack
	BestTsMs  *int32
}

// BrowseQueryResult is the unified browse/list response: Items matches
// pagination, TotalVisible counts rows after stack collapse, TotalMediaItems
// counts matching logical media items, TotalFiles counts their component
// files, StackMode echoes the resolved mode.
type BrowseQueryResult struct {
	Items           []BrowseItem
	TotalVisible    int64
	TotalMediaItems int64
	TotalFiles      int64
	StackMode       string
}

// SearchBrowseResult combines optional confidence-ordered "top results" with
// the main search listing. Both tiers are flat at media-item granularity;
// search has no stack_mode.
type SearchBrowseResult struct {
	TopResults             []BrowseItem
	TopResultsMeta         SearchTopResultsMeta
	Results                []BrowseItem
	ResultsTotalVisible    int64
	ResultsTotalMediaItems int64
}

// mediaRef pairs a logical media item with its primary asset for hydration.
type mediaRef struct {
	MediaItemID    uuid.UUID
	PrimaryAssetID uuid.UUID
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

// validateBrowseFilter rejects contradictory stack filter combinations.
// Non-empty StackKinds implicitly requires stacked items (the SQL kind
// predicate never matches NULL stack kinds), so unstacked+kinds can only
// produce an empty, misleading result and is rejected instead.
func validateBrowseFilter(params QueryAssetsParams) error {
	if params.StackMembership == StackMembershipUnstacked && len(params.StackKinds) > 0 {
		return fmt.Errorf("%w: stack.membership=unstacked excludes stack.kinds", ErrInvalidBrowseFilter)
	}
	return nil
}

func browseMediaItemRowID(mediaItemID uuid.UUID) string {
	return "media:" + mediaItemID.String()
}

func browseStackRowID(stackID uuid.UUID) string {
	return "stack:" + stackID.String()
}

// QueryBrowseItems returns paginated browse rows for the gallery at
// media-item granularity. Expanded mode lists one row per logical media item;
// collapsed mode groups stacked items into presentation-stack rows. Semantic
// search with a non-empty query resolves the vector-ranked asset set to media
// items in application code; every other combination is served by the unified
// SQL queries.
func (s *assetService) QueryBrowseItems(ctx context.Context, params QueryAssetsParams) (BrowseQueryResult, error) {
	params.StackMode = normalizeStackMode(params.StackMode)
	if err := validateBrowseFilter(params); err != nil {
		return BrowseQueryResult{}, err
	}

	if params.SearchType == "semantic" && strings.TrimSpace(params.Query) != "" {
		return s.queryAggregateBrowseItems(ctx, params)
	}

	in, err := newUnifiedQueryInputs(params)
	if err != nil {
		return BrowseQueryResult{}, err
	}

	totalMediaItems, err := s.queries.CountMediaItemsUnified(ctx, countMediaItemsUnifiedParams(params, in))
	if err != nil {
		return BrowseQueryResult{}, fmt.Errorf("failed to count media items: %w", err)
	}
	totalFiles, err := s.queries.CountMediaItemFilesUnified(ctx, countMediaItemFilesUnifiedParams(params, in))
	if err != nil {
		return BrowseQueryResult{}, fmt.Errorf("failed to count media item files: %w", err)
	}

	if params.StackMode == StackModeExpanded {
		rows, err := s.queries.GetMediaItemsUnified(ctx, getMediaItemsUnifiedParams(params, in))
		if err != nil {
			return BrowseQueryResult{}, err
		}
		return BrowseQueryResult{
			Items:           browseItemsFromMediaItemRows(rows),
			TotalVisible:    totalMediaItems,
			TotalMediaItems: totalMediaItems,
			TotalFiles:      totalFiles,
			StackMode:       params.StackMode,
		}, nil
	}

	totalVisible, err := s.queries.CountCollapsedBrowseItemsUnified(ctx, countCollapsedBrowseItemsUnifiedParams(params, in))
	if err != nil {
		return BrowseQueryResult{}, fmt.Errorf("failed to count collapsed browse items: %w", err)
	}
	rows, err := s.queries.GetCollapsedBrowseItemsUnified(ctx, getCollapsedBrowseItemsUnifiedParams(params, in))
	if err != nil {
		return BrowseQueryResult{}, err
	}
	items, err := browseItemsFromCollapsedRows(rows)
	if err != nil {
		return BrowseQueryResult{}, err
	}
	return BrowseQueryResult{
		Items:           items,
		TotalVisible:    totalVisible,
		TotalMediaItems: totalMediaItems,
		TotalFiles:      totalFiles,
		StackMode:       params.StackMode,
	}, nil
}

// QueryMediaItems lists matching logical media items with composition facts
// (one row per media item, no stack collapse).
func (s *assetService) QueryMediaItems(ctx context.Context, params QueryAssetsParams) ([]BrowseMediaItem, int64, error) {
	if err := validateBrowseFilter(params); err != nil {
		return nil, 0, err
	}
	in, err := newUnifiedQueryInputs(params)
	if err != nil {
		return nil, 0, err
	}
	total, err := s.queries.CountMediaItemsUnified(ctx, countMediaItemsUnifiedParams(params, in))
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count media items: %w", err)
	}
	rows, err := s.queries.GetMediaItemsUnified(ctx, getMediaItemsUnifiedParams(params, in))
	if err != nil {
		return nil, 0, err
	}
	items := make([]BrowseMediaItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, browseMediaItemFromUnifiedRow(row))
	}
	return items, total, nil
}

// CountMediaItems counts matching logical media items.
func (s *assetService) CountMediaItems(ctx context.Context, params QueryAssetsParams) (int64, error) {
	if err := validateBrowseFilter(params); err != nil {
		return 0, err
	}
	in, err := newUnifiedQueryInputs(params)
	if err != nil {
		return 0, err
	}
	return s.queries.CountMediaItemsUnified(ctx, countMediaItemsUnifiedParams(params, in))
}

// CountMediaItemFiles counts the component files of matching media items.
func (s *assetService) CountMediaItemFiles(ctx context.Context, params QueryAssetsParams) (int64, error) {
	if err := validateBrowseFilter(params); err != nil {
		return 0, err
	}
	in, err := newUnifiedQueryInputs(params)
	if err != nil {
		return 0, err
	}
	return s.queries.CountMediaItemFilesUnified(ctx, countMediaItemFilesUnifiedParams(params, in))
}

// SearchBrowseItems is the browse-tier face of the unified search pipeline
// (see asset_search_fused.go). One fused set, resolved to media-item
// granularity: Results is the whole set under the presentation sort, Best
// Results (TopResults) is its confidence-ordered Top-N subset. Both tiers are
// flat — search results are never stack-collapsed (collapse would reorder the
// relevance set), keeping Best Results a literal subset of Results.
func (s *assetService) SearchBrowseItems(ctx context.Context, params SearchAssetsParams) (SearchBrowseResult, error) {
	params = normalizeSearchAssetsParams(params)
	if err := validateBrowseFilter(params.QueryAssetsParams); err != nil {
		return SearchBrowseResult{}, err
	}

	result := SearchBrowseResult{
		TopResults:     []BrowseItem{},
		TopResultsMeta: SearchTopResultsMeta{Enabled: false, SourceTypes: []string{}},
		Results:        []BrowseItem{},
	}

	query := strings.TrimSpace(params.Query)
	enhanced := query != "" && params.EnhancementMode != SearchEnhancementModeOff

	if enhanced {
		if fused, ok := s.runSearchAssetsFusedSet(ctx, params); ok {
			result.TopResultsMeta = fused.meta()
			refs, bestTsByItem, err := s.resolveSearchMediaRefs(ctx, fused)
			if err != nil {
				return SearchBrowseResult{}, err
			}

			// Best Results exists only when the set exceeds the showcase size.
			if len(refs) >= params.TopResultsLimit {
				top, err := s.browseItemsForMediaRefs(ctx, refs[:params.TopResultsLimit], bestTsByItem, params.IsDeleted)
				if err != nil {
					return SearchBrowseResult{}, err
				}
				result.TopResults = top
			}

			if params.EnhancementMode != SearchEnhancementModeOnly {
				page, err := s.pageMediaRefsBySort(ctx, refs, params.SortBy, params.Limit, params.Offset, bestTsByItem, params.IsDeleted)
				if err != nil {
					return SearchBrowseResult{}, err
				}
				result.Results = page
				result.ResultsTotalVisible = int64(len(refs))
				result.ResultsTotalMediaItems = int64(len(refs))
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
		// Search results stay flat by media item.
		filenameParams.StackMode = StackModeExpanded

		browseResult, err := s.QueryBrowseItems(ctx, filenameParams)
		if err != nil {
			return SearchBrowseResult{}, err
		}
		result.Results = browseResult.Items
		result.ResultsTotalVisible = browseResult.TotalVisible
		result.ResultsTotalMediaItems = browseResult.TotalMediaItems
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

// queryAggregateBrowseItems serves semantic browse: the vector-ranked asset
// pool is resolved to media items (dedupe by media item, keep the highest
// ranked contribution), then presented flat or stack-collapsed with
// application-level pagination.
func (s *assetService) queryAggregateBrowseItems(ctx context.Context, params QueryAssetsParams) (BrowseQueryResult, error) {
	poolParams := params
	poolParams.AssetTypes = cloneStringSlice(params.AssetTypes)
	poolParams.Limit = aggregateCandidatePoolSize(params.Limit, params.Offset)
	poolParams.Offset = 0

	assets, _, err := s.queryAssetsAggregate(ctx, poolParams)
	if err != nil {
		return BrowseQueryResult{}, err
	}
	assetIDs := make([]uuid.UUID, 0, len(assets))
	for _, asset := range assets {
		if asset.AssetID != uuid.Nil {
			assetIDs = append(assetIDs, asset.AssetID)
		}
	}
	refs, _, err := s.resolveMediaRefsInOrder(ctx, assetIDs)
	if err != nil {
		return BrowseQueryResult{}, err
	}
	if len(refs) == 0 {
		return BrowseQueryResult{Items: []BrowseItem{}, StackMode: params.StackMode}, nil
	}

	itemIDs := make([]uuid.UUID, 0, len(refs))
	for _, ref := range refs {
		itemIDs = append(itemIDs, ref.MediaItemID)
	}
	facts, err := s.mediaItemFactsByIDs(ctx, itemIDs)
	if err != nil {
		return BrowseQueryResult{}, err
	}
	var totalFiles int64
	for _, ref := range refs {
		if fact, ok := facts[ref.MediaItemID]; ok {
			totalFiles += fact.ComponentCount
		} else {
			totalFiles++
		}
	}

	var items []BrowseItem
	if params.StackMode == StackModeExpanded {
		items, err = s.browseItemsForMediaRefs(ctx, refs, nil, params.IsDeleted)
	} else {
		items, err = s.collapseRefsToBrowseItems(ctx, refs, facts, params.OwnerID, params.IsDeleted)
	}
	if err != nil {
		return BrowseQueryResult{}, err
	}

	return BrowseQueryResult{
		Items:           pageBrowseItems(items, params.Limit, params.Offset),
		TotalVisible:    int64(len(items)),
		TotalMediaItems: int64(len(refs)),
		TotalFiles:      totalFiles,
		StackMode:       params.StackMode,
	}, nil
}

// resolveMediaRefsInOrder maps ranked asset IDs to their logical media items,
// deduplicating by media item while preserving the input (rank) order of each
// item's first contribution. The second return maps every resolvable asset ID
// to its media item.
func (s *assetService) resolveMediaRefsInOrder(ctx context.Context, assetIDs []uuid.UUID) ([]mediaRef, map[uuid.UUID]uuid.UUID, error) {
	if len(assetIDs) == 0 {
		return []mediaRef{}, map[uuid.UUID]uuid.UUID{}, nil
	}
	rows, err := s.queries.GetMediaItemsByAssetIDs(ctx, assetIDs)
	if err != nil {
		return nil, nil, fmt.Errorf("resolve media items: %w", err)
	}
	refByAsset := make(map[uuid.UUID]mediaRef, len(rows))
	for _, row := range rows {
		if row.AssetID == uuid.Nil || row.MediaItemID == uuid.Nil || !row.PrimaryAssetID.Valid {
			continue
		}
		refByAsset[row.AssetID] = mediaRef{
			MediaItemID:    row.MediaItemID,
			PrimaryAssetID: row.PrimaryAssetID.UUID,
		}
	}

	refs := make([]mediaRef, 0, len(assetIDs))
	itemByAsset := make(map[uuid.UUID]uuid.UUID, len(assetIDs))
	seen := make(map[uuid.UUID]struct{}, len(assetIDs))
	for _, assetID := range assetIDs {
		ref, ok := refByAsset[assetID]
		if !ok {
			continue
		}
		itemByAsset[assetID] = ref.MediaItemID
		if _, exists := seen[ref.MediaItemID]; exists {
			continue
		}
		seen[ref.MediaItemID] = struct{}{}
		refs = append(refs, ref)
	}
	return refs, itemByAsset, nil
}

// resolveSearchMediaRefs resolves the fused search set to media-item refs in
// confidence order and carries each media item's first non-nil BestTs
// contribution over from the asset-level candidates.
func (s *assetService) resolveSearchMediaRefs(ctx context.Context, fused fusedSearchSet) ([]mediaRef, map[uuid.UUID]*int32, error) {
	ids := fused.ids()
	refs, itemByAsset, err := s.resolveMediaRefsInOrder(ctx, ids)
	if err != nil {
		return nil, nil, err
	}
	bestTsByAsset := fused.bestTsByID()
	bestTsByItem := make(map[uuid.UUID]*int32, len(refs))
	for _, assetID := range ids {
		itemID, ok := itemByAsset[assetID]
		if !ok {
			continue
		}
		if _, exists := bestTsByItem[itemID]; exists {
			continue
		}
		if ts, found := bestTsByAsset[assetID]; found {
			bestTsByItem[itemID] = ts
		}
	}
	return refs, bestTsByItem, nil
}

// browseItemsForMediaRefs hydrates refs into flat media-item browse rows,
// preserving the given ref order and dropping refs whose primary asset is no
// longer visible.
func (s *assetService) browseItemsForMediaRefs(ctx context.Context, refs []mediaRef, bestTsByItem map[uuid.UUID]*int32, isDeleted *bool) ([]BrowseItem, error) {
	if len(refs) == 0 {
		return []BrowseItem{}, nil
	}
	primaryIDs := make([]uuid.UUID, 0, len(refs))
	itemIDs := make([]uuid.UUID, 0, len(refs))
	refByPrimary := make(map[uuid.UUID]mediaRef, len(refs))
	for _, ref := range refs {
		primaryIDs = append(primaryIDs, ref.PrimaryAssetID)
		itemIDs = append(itemIDs, ref.MediaItemID)
		refByPrimary[ref.PrimaryAssetID] = ref
	}

	assets, err := s.runHydrateAssetsInOrder(ctx, primaryIDs, isDeleted)
	if err != nil {
		return nil, err
	}
	facts, err := s.mediaItemFactsByIDs(ctx, itemIDs)
	if err != nil {
		return nil, err
	}

	items := make([]BrowseItem, 0, len(assets))
	for _, asset := range assets {
		ref, ok := refByPrimary[asset.AssetID]
		if !ok {
			continue
		}
		media := browseMediaItemFromFacts(ref, facts, asset)
		var bestTs *int32
		if bestTsByItem != nil {
			bestTs = bestTsByItem[ref.MediaItemID]
		}
		items = append(items, browseItemForMediaItem(media, bestTs))
	}
	return items, nil
}

// pageMediaRefsBySort orders a media-item membership set by the requested
// presentation sort (newest first, keyed on primary assets) and returns the
// requested page as browse rows.
func (s *assetService) pageMediaRefsBySort(ctx context.Context, refs []mediaRef, sortBy string, limit, offset int, bestTsByItem map[uuid.UUID]*int32, isDeleted *bool) ([]BrowseItem, error) {
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 || offset >= len(refs) {
		return []BrowseItem{}, nil
	}

	primaryIDs := make([]uuid.UUID, 0, len(refs))
	refByPrimary := make(map[uuid.UUID]mediaRef, len(refs))
	for _, ref := range refs {
		primaryIDs = append(primaryIDs, ref.PrimaryAssetID)
		refByPrimary[ref.PrimaryAssetID] = ref
	}

	var orderedAsc []uuid.UUID
	var err error
	if sortBy == "recently_added" {
		orderedAsc, err = s.queries.RankAssetIDsByUploadTime(ctx, primaryIDs)
	} else {
		orderedAsc, err = s.queries.RankAssetIDsByTime(ctx, primaryIDs)
	}
	if err != nil {
		return nil, err
	}

	ordered := make([]mediaRef, 0, len(orderedAsc))
	for i := len(orderedAsc) - 1; i >= 0; i-- {
		if ref, ok := refByPrimary[orderedAsc[i]]; ok {
			ordered = append(ordered, ref)
		}
	}

	end := offset + limit
	if end > len(ordered) {
		end = len(ordered)
	}
	if offset >= end {
		return []BrowseItem{}, nil
	}
	return s.browseItemsForMediaRefs(ctx, ordered[offset:end], bestTsByItem, isDeleted)
}

// collapseRefsToBrowseItems groups ranked media-item refs into presentation
// rows: unstacked refs emit media-item rows, stacked refs emit one stack row
// at the stack's first appearance (input order). Stack covers prefer the
// designated cover media item, then the lowest-position visible member.
// ownerID restricts member lists to items the caller may see (nil = admin).
func (s *assetService) collapseRefsToBrowseItems(ctx context.Context, refs []mediaRef, facts map[uuid.UUID]repo.GetMediaItemBrowseFactsByIDsRow, ownerID *int32, isDeleted *bool) ([]BrowseItem, error) {
	if len(refs) == 0 {
		return []BrowseItem{}, nil
	}

	type orderedEntry struct {
		ref     mediaRef
		stackID uuid.UUID // uuid.Nil for unstacked media-item rows
	}
	entries := make([]orderedEntry, 0, len(refs))
	stackOrder := make([]uuid.UUID, 0)
	matchedByStack := make(map[uuid.UUID][]BrowseStackMember)
	seenStacks := make(map[uuid.UUID]struct{})
	for _, ref := range refs {
		stackID := uuid.Nil
		if fact, ok := facts[ref.MediaItemID]; ok {
			stackID = fact.StackID
		}
		if stackID == uuid.Nil {
			entries = append(entries, orderedEntry{ref: ref})
			continue
		}
		matchedByStack[stackID] = append(matchedByStack[stackID], BrowseStackMember{
			MediaItemID:    ref.MediaItemID,
			PrimaryAssetID: ref.PrimaryAssetID,
		})
		if _, seen := seenStacks[stackID]; seen {
			continue
		}
		seenStacks[stackID] = struct{}{}
		stackOrder = append(stackOrder, stackID)
		entries = append(entries, orderedEntry{ref: ref, stackID: stackID})
	}

	kindByStack := make(map[uuid.UUID]dbtypes.StackKind, len(stackOrder))
	coverItemByStack := make(map[uuid.UUID]uuid.UUID, len(stackOrder))
	if len(stackOrder) > 0 {
		rows, err := s.queries.GetStackKindsByIDs(ctx, stackOrder)
		if err != nil {
			return nil, fmt.Errorf("get browse stack kinds: %w", err)
		}
		for _, row := range rows {
			kindByStack[row.StackID] = dbtypes.StackKind(row.StackKind)
			if row.CoverMediaItemID.Valid {
				coverItemByStack[row.StackID] = row.CoverMediaItemID.UUID
			}
		}
	}

	membersByStack := make(map[uuid.UUID][]BrowseStackMember, len(stackOrder))
	for _, stackID := range stackOrder {
		memberRows, err := s.queries.GetStackMembers(ctx, repo.GetStackMembersParams{
			StackID: stackID,
			OwnerID: ownerID,
		})
		if err != nil {
			return nil, fmt.Errorf("get stack members for %s: %w", stackID.String(), err)
		}
		members := make([]BrowseStackMember, 0, len(memberRows))
		for _, member := range memberRows {
			if !member.AssetID.Valid {
				continue
			}
			members = append(members, BrowseStackMember{
				MediaItemID:    member.MediaItemID,
				PrimaryAssetID: member.AssetID.UUID,
			})
		}
		membersByStack[stackID] = members
	}

	coverRefByStack := make(map[uuid.UUID]mediaRef, len(stackOrder))
	for _, stackID := range stackOrder {
		members := membersByStack[stackID]
		var cover *BrowseStackMember
		if designated, ok := coverItemByStack[stackID]; ok {
			for i := range members {
				if members[i].MediaItemID == designated {
					cover = &members[i]
					break
				}
			}
		}
		if cover == nil && len(members) > 0 {
			cover = &members[0]
		}
		if cover == nil {
			// Stack has no visible members for this caller: fall back to
			// the first matched ref so the row can still render.
			if matched := matchedByStack[stackID]; len(matched) > 0 {
				cover = &matched[0]
			}
		}
		if cover != nil {
			coverRefByStack[stackID] = mediaRef{
				MediaItemID:    cover.MediaItemID,
				PrimaryAssetID: cover.PrimaryAssetID,
			}
		}
	}

	missingFactIDs := make([]uuid.UUID, 0)
	for _, cover := range coverRefByStack {
		if _, ok := facts[cover.MediaItemID]; !ok {
			missingFactIDs = append(missingFactIDs, cover.MediaItemID)
		}
	}
	if len(missingFactIDs) > 0 {
		extra, err := s.mediaItemFactsByIDs(ctx, missingFactIDs)
		if err != nil {
			return nil, err
		}
		for id, fact := range extra {
			facts[id] = fact
		}
	}

	assetIDs := make([]uuid.UUID, 0, len(entries))
	for _, entry := range entries {
		if entry.stackID == uuid.Nil {
			assetIDs = append(assetIDs, entry.ref.PrimaryAssetID)
			continue
		}
		if cover, ok := coverRefByStack[entry.stackID]; ok {
			assetIDs = append(assetIDs, cover.PrimaryAssetID)
		}
	}
	assetByID, err := s.assetsByIDsMap(ctx, assetIDs, isDeleted)
	if err != nil {
		return nil, err
	}

	items := make([]BrowseItem, 0, len(entries))
	for _, entry := range entries {
		if entry.stackID == uuid.Nil {
			asset, ok := assetByID[entry.ref.PrimaryAssetID]
			if !ok {
				continue
			}
			media := browseMediaItemFromFacts(entry.ref, facts, asset)
			items = append(items, browseItemForMediaItem(media, nil))
			continue
		}

		coverRef, ok := coverRefByStack[entry.stackID]
		if !ok {
			continue
		}
		coverAsset, ok := assetByID[coverRef.PrimaryAssetID]
		if !ok {
			continue
		}
		kind := kindByStack[entry.stackID]
		cover := browseMediaItemFromFacts(coverRef, facts, coverAsset)
		cover.StackID = entry.stackID
		cover.StackKind = kind

		items = append(items, BrowseItem{
			Type: BrowseItemTypeStack,
			ID:   browseStackRowID(entry.stackID),
			Stack: &BrowseStack{
				StackID:        entry.stackID,
				Kind:           kind,
				Cover:          cover,
				Members:        membersByStack[entry.stackID],
				MatchedMembers: matchedByStack[entry.stackID],
			},
		})
	}
	return items, nil
}

// mediaItemFactsByIDs loads composition/stack facts for the given media items.
func (s *assetService) mediaItemFactsByIDs(ctx context.Context, mediaItemIDs []uuid.UUID) (map[uuid.UUID]repo.GetMediaItemBrowseFactsByIDsRow, error) {
	facts := make(map[uuid.UUID]repo.GetMediaItemBrowseFactsByIDsRow, len(mediaItemIDs))
	if len(mediaItemIDs) == 0 {
		return facts, nil
	}
	rows, err := s.queries.GetMediaItemBrowseFactsByIDs(ctx, mediaItemIDs)
	if err != nil {
		return nil, fmt.Errorf("get media item browse facts: %w", err)
	}
	for _, row := range rows {
		facts[row.MediaItemID] = row
	}
	return facts, nil
}

// assetsByIDsMap fetches asset rows keyed by ID (visibility follows isDeleted).
func (s *assetService) assetsByIDsMap(ctx context.Context, ids []uuid.UUID, isDeleted *bool) (map[uuid.UUID]repo.Asset, error) {
	out := make(map[uuid.UUID]repo.Asset, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	var rows []repo.Asset
	var err error
	if queryIncludesDeletedAssets(isDeleted) {
		rows, err = s.queries.GetAssetsByIDsAny(ctx, ids)
	} else {
		rows, err = s.queries.GetAssetsByIDs(ctx, ids)
	}
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.AssetID] = row
	}
	return out, nil
}

// browseItemForMediaItem wraps a media item as a flat browse row.
func browseItemForMediaItem(media BrowseMediaItem, bestTs *int32) BrowseItem {
	item := BrowseItem{
		Type:      BrowseItemTypeMediaItem,
		ID:        browseMediaItemRowID(media.MediaItemID),
		MediaItem: &media,
		BestTsMs:  bestTs,
	}
	return item
}

// browseMediaItemFromUnifiedRow converts an expanded unified row.
func browseMediaItemFromUnifiedRow(row repo.GetMediaItemsUnifiedRow) BrowseMediaItem {
	media := BrowseMediaItem{
		MediaItemID:    row.MediaItemID,
		MediaKind:      row.MediaKind,
		PrimaryAsset:   row.Asset,
		ComponentCount: int(row.ComponentCount),
		HasRAW:         row.HasRaw != 0,
		HasJPEG:        row.HasJpeg != 0,
		HasEdited:      row.HasEdited != 0,
		HasLiveMotion:  row.HasLiveMotion != 0,
		StackID:        row.StackID,
	}
	if row.StackKind != nil {
		media.StackKind = dbtypes.StackKind(*row.StackKind)
	}
	return media
}

// browseMediaItemFromFacts builds a media item from a ref, its facts row (may
// be absent for degenerate data) and the hydrated primary asset.
func browseMediaItemFromFacts(ref mediaRef, facts map[uuid.UUID]repo.GetMediaItemBrowseFactsByIDsRow, primary repo.Asset) BrowseMediaItem {
	media := BrowseMediaItem{
		MediaItemID:    ref.MediaItemID,
		PrimaryAsset:   primary,
		ComponentCount: 1,
	}
	if fact, ok := facts[ref.MediaItemID]; ok {
		media.MediaKind = fact.MediaKind
		media.ComponentCount = int(fact.ComponentCount)
		media.HasRAW = fact.HasRaw != 0
		media.HasJPEG = fact.HasJpeg != 0
		media.HasEdited = fact.HasEdited != 0
		media.HasLiveMotion = fact.HasLiveMotion != 0
		media.StackID = fact.StackID
		if fact.StackKind != nil {
			media.StackKind = dbtypes.StackKind(*fact.StackKind)
		}
	}
	return media
}

// browseItemsFromMediaItemRows maps expanded unified rows to flat browse rows.
func browseItemsFromMediaItemRows(rows []repo.GetMediaItemsUnifiedRow) []BrowseItem {
	items := make([]BrowseItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, browseItemForMediaItem(browseMediaItemFromUnifiedRow(row), nil))
	}
	return items
}

// browseItemsFromCollapsedRows converts SQL collapsed-browse rows into
// BrowseItem values (media-item vs stack rows).
func browseItemsFromCollapsedRows(rows []repo.GetCollapsedBrowseItemsUnifiedRow) ([]BrowseItem, error) {
	items := make([]BrowseItem, 0, len(rows))
	for _, row := range rows {
		cover := BrowseMediaItem{
			MediaItemID:    row.CoverMediaItemID,
			MediaKind:      row.CoverMediaKind,
			PrimaryAsset:   row.Asset,
			ComponentCount: int(row.CoverComponentCount),
			HasRAW:         row.CoverHasRaw != 0,
			HasJPEG:        row.CoverHasJpeg != 0,
			HasEdited:      row.CoverHasEdited != 0,
			HasLiveMotion:  row.CoverHasLiveMotion != 0,
		}

		if row.ItemType != BrowseItemTypeStack {
			items = append(items, browseItemForMediaItem(cover, nil))
			continue
		}

		if row.StackID == uuid.Nil {
			return nil, fmt.Errorf("collapsed browse row missing stack id for media item %s", row.CoverMediaItemID.String())
		}
		kind := stackKindFromAny(row.StackKind)
		cover.StackID = row.StackID
		cover.StackKind = kind

		members, err := stackMembersFromSQLiteJSON(row.MemberItems)
		if err != nil {
			return nil, fmt.Errorf("decode stack members for %s: %w", row.StackID.String(), err)
		}
		matched, err := stackMembersFromSQLiteJSON(row.MatchedItems)
		if err != nil {
			return nil, fmt.Errorf("decode matched stack members for %s: %w", row.StackID.String(), err)
		}

		items = append(items, BrowseItem{
			Type: BrowseItemTypeStack,
			ID:   browseStackRowID(row.StackID),
			Stack: &BrowseStack{
				StackID:        row.StackID,
				Kind:           kind,
				Cover:          cover,
				Members:        members,
				MatchedMembers: matched,
			},
		})
	}
	return items, nil
}

func stackKindFromAny(value any) dbtypes.StackKind {
	switch v := value.(type) {
	case string:
		return dbtypes.StackKind(v)
	case []byte:
		return dbtypes.StackKind(string(v))
	default:
		return ""
	}
}

// stackMembersFromSQLiteJSON decodes json_group_array(json_object(
// 'media_item_id', ..., 'primary_asset_id', ...)) payloads.
func stackMembersFromSQLiteJSON(value any) ([]BrowseStackMember, error) {
	var raw []byte
	switch v := value.(type) {
	case nil:
		return []BrowseStackMember{}, nil
	case string:
		raw = []byte(v)
	case []byte:
		raw = v
	default:
		return nil, fmt.Errorf("unexpected stack member payload type %T", value)
	}
	if len(raw) == 0 {
		return []BrowseStackMember{}, nil
	}

	var decoded []struct {
		MediaItemID    string `json:"media_item_id"`
		PrimaryAssetID string `json:"primary_asset_id"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil, err
	}

	members := make([]BrowseStackMember, 0, len(decoded))
	for _, entry := range decoded {
		if entry.MediaItemID == "" || entry.PrimaryAssetID == "" {
			continue
		}
		itemID, err := uuid.Parse(entry.MediaItemID)
		if err != nil {
			return nil, fmt.Errorf("invalid stack member media item id %q: %w", entry.MediaItemID, err)
		}
		primaryID, err := uuid.Parse(entry.PrimaryAssetID)
		if err != nil {
			return nil, fmt.Errorf("invalid stack member primary asset id %q: %w", entry.PrimaryAssetID, err)
		}
		members = append(members, BrowseStackMember{
			MediaItemID:    itemID,
			PrimaryAssetID: primaryID,
		})
	}
	return members, nil
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

func countCollapsedBrowseItemsUnifiedParams(params QueryAssetsParams, in unifiedQueryInputs) repo.CountCollapsedBrowseItemsUnifiedParams {
	return repo.CountCollapsedBrowseItemsUnifiedParams{
		AssetIds:         in.assetIDs,
		AssetTypes:       in.assetTypes,
		TagNames:         in.tagNames,
		StackKinds:       in.stackKinds,
		IsDeleted:        in.isDeleted,
		Query:            in.query,
		AssetType:        params.AssetType,
		OwnerID:          params.OwnerID,
		RepositoryID:     in.repoUUID,
		FolderPath:       params.FolderPath,
		FolderRecursive:  params.FolderRecursive,
		PersonID:         params.PersonID,
		AlbumID:          params.AlbumID,
		TagName:          params.TagName,
		TagSource:        params.TagSource,
		FilenameVal:      params.FilenameValue,
		FilenameOperator: params.FilenameOperator,
		DateFrom:         in.dateFrom,
		DateTo:           in.dateTo,
		Composition:      in.composition,
		StackMembership:  in.stackMembership,
		Rating:           in.rating,
		Liked:            params.Liked,
		CameraModel:      params.CameraModel,
		LensModel:        params.LensModel,
		LocationNorth:    params.LocationNorth,
		LocationSouth:    params.LocationSouth,
		LocationEast:     params.LocationEast,
		LocationWest:     params.LocationWest,
	}
}

func getCollapsedBrowseItemsUnifiedParams(params QueryAssetsParams, in unifiedQueryInputs) repo.GetCollapsedBrowseItemsUnifiedParams {
	return repo.GetCollapsedBrowseItemsUnifiedParams{
		AssetIds:         in.assetIDs,
		AssetTypes:       in.assetTypes,
		TagNames:         in.tagNames,
		StackKinds:       in.stackKinds,
		IsDeleted:        in.isDeleted,
		Query:            in.query,
		AssetType:        params.AssetType,
		OwnerID:          params.OwnerID,
		RepositoryID:     in.repoUUID,
		FolderPath:       params.FolderPath,
		FolderRecursive:  params.FolderRecursive,
		PersonID:         params.PersonID,
		AlbumID:          params.AlbumID,
		TagName:          params.TagName,
		TagSource:        params.TagSource,
		FilenameVal:      params.FilenameValue,
		FilenameOperator: params.FilenameOperator,
		DateFrom:         in.dateFrom,
		DateTo:           in.dateTo,
		Composition:      in.composition,
		StackMembership:  in.stackMembership,
		Rating:           in.rating,
		Liked:            params.Liked,
		CameraModel:      params.CameraModel,
		LensModel:        params.LensModel,
		LocationNorth:    params.LocationNorth,
		LocationSouth:    params.LocationSouth,
		LocationEast:     params.LocationEast,
		LocationWest:     params.LocationWest,
		SortBy:           in.sortBy,
		Offset:           int64(params.Offset),
		Limit:            int64(params.Limit),
	}
}
