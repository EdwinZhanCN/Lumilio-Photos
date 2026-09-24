package handler

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"server/internal/api"
	"server/internal/api/dto"
	"server/internal/api/problem"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/service"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// GetAssetTypes returns available asset types
// @Summary Get supported asset types
// @Description Retrieve a list of all supported asset types in the system.
// @Tags assets
// @Accept json
// @Produce json
// @Success 200 {object} dto.AssetTypesResponseDTO "Asset types retrieved successfully"
// @Router /api/v1/assets/types [get]
func (h *AssetHandler) GetAssetTypes(c *gin.Context) {
	types := []dbtypes.AssetType{
		dbtypes.AssetTypePhoto,
		dbtypes.AssetTypeVideo,
		dbtypes.AssetTypeAudio,
	}

	api.JSONOK(c, dto.AssetTypesResponseDTO{Types: types})
}

func normalizeAssetQueryPagination(pagination *dto.PaginationDTO) {
	if pagination.Limit <= 0 || pagination.Limit > 100 {
		pagination.Limit = 20
	}
	if pagination.Offset < 0 {
		pagination.Offset = 0
	}
}

func validateAssetQuerySearchType(searchType string) error {
	if searchType == "" || searchType == "filename" || searchType == "semantic" {
		return nil
	}
	return errors.New("invalid search type")
}

func validateAssetQuerySortBy(sortBy string) error {
	switch strings.ToLower(strings.TrimSpace(sortBy)) {
	case "", "recently_added", "date_captured":
		return nil
	default:
		return errors.New("invalid sort_by")
	}
}

func validateSearchEnhancementMode(mode string) error {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", "auto", "off", "only":
		return nil
	default:
		return errors.New("invalid enhancement mode")
	}
}

func validateStackMode(mode string) error {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", service.StackModeCollapsed, service.StackModeExpanded:
		return nil
	default:
		return errors.New("invalid stack mode")
	}
}

// rejectSearchStackMode enforces that search requests carry no stack_mode:
// search results are always flat by media item (relevance order must not be
// reordered by stack collapse).
func rejectSearchStackMode(mode string) error {
	if strings.TrimSpace(mode) != "" {
		return errors.New("stack_mode is not supported for search")
	}
	return nil
}

func normalizeAssetQuerySortBy(sortBy string) string {
	switch strings.ToLower(strings.TrimSpace(sortBy)) {
	case "recently_added":
		return "recently_added"
	case "date_captured":
		return "date_captured"
	default:
		return "date_captured"
	}
}

func normalizeFilenameOperator(operator string) string {
	switch strings.ToLower(strings.TrimSpace(operator)) {
	case "matches":
		return "matches"
	case "starts_with", "startswith":
		return "starts_with"
	case "ends_with", "endswith":
		return "ends_with"
	default:
		return "contains"
	}
}

// normalizeFolderPath normalizes a repository-relative folder path filter:
// it converts platform separators to '/', collapses repeated separators via
// path.Clean, and trims leading/trailing slashes so SQL prefix matching
// against storage_path is consistent regardless of client input.
func normalizeFolderPath(folderPath string) string {
	cleaned := strings.ReplaceAll(folderPath, "\\", "/")
	cleaned = path.Clean(cleaned)
	cleaned = strings.Trim(cleaned, "/")
	if cleaned == "." {
		return ""
	}
	return cleaned
}

func assetQueryDateLocation(viewerTimeZone string) *time.Location {
	if strings.TrimSpace(viewerTimeZone) == "" {
		return time.UTC
	}
	location, err := time.LoadLocation(strings.TrimSpace(viewerTimeZone))
	if err != nil {
		return time.UTC
	}
	return location
}

// browseFilterFromDTO validates and normalizes the media-item/stack filter
// blocks: unknown enum values are rejected, empty kinds arrays normalize to
// unset, and unstacked+kinds is contradictory (mirrors the service check so
// the request fails fast with a 400).
func browseFilterFromDTO(filter dto.AssetFilterDTO) (service.MediaComposition, service.StackMembership, []string, error) {
	var composition service.MediaComposition
	if filter.MediaItem != nil && filter.MediaItem.Composition != nil {
		switch service.MediaComposition(*filter.MediaItem.Composition) {
		case service.MediaCompositionContainsRAW, service.MediaCompositionJPEGRAW,
			service.MediaCompositionRAWUnpaired, service.MediaCompositionNoRAW,
			service.MediaCompositionLivePhoto:
			composition = service.MediaComposition(*filter.MediaItem.Composition)
		default:
			return "", "", nil, fmt.Errorf("unknown media_item.composition: %q", string(*filter.MediaItem.Composition))
		}
	}

	var membership service.StackMembership
	var kinds []string
	if filter.Stack != nil {
		if filter.Stack.Membership != nil {
			switch service.StackMembership(*filter.Stack.Membership) {
			case service.StackMembershipStacked, service.StackMembershipUnstacked:
				membership = service.StackMembership(*filter.Stack.Membership)
			default:
				return "", "", nil, fmt.Errorf("unknown stack.membership: %q", string(*filter.Stack.Membership))
			}
		}
		for _, kind := range filter.Stack.Kinds {
			kind = strings.ToLower(strings.TrimSpace(kind))
			if kind == "" {
				continue
			}
			if !dbtypes.StackKind(kind).Valid() {
				return "", "", nil, fmt.Errorf("unknown stack.kinds value: %q", kind)
			}
			kinds = append(kinds, kind)
		}
		if membership == service.StackMembershipUnstacked && len(kinds) > 0 {
			return "", "", nil, fmt.Errorf("stack.membership=unstacked excludes stack.kinds")
		}
	}
	return composition, membership, kinds, nil
}

func buildQueryAssetsParams(query, searchType, sortBy, viewerTimeZone, stackMode string, filter dto.AssetFilterDTO, pagination dto.PaginationDTO) (service.QueryAssetsParams, error) {
	mediaComposition, stackMembership, stackKinds, err := browseFilterFromDTO(filter)
	if err != nil {
		return service.QueryAssetsParams{}, err
	}

	var dateFrom, dateTo *time.Time
	if filter.Date != nil {
		dateFrom = filter.Date.From
		dateTo = filter.Date.To

		// Normalize date-only inputs in the viewer's timezone. Exact timestamps
		// remain exact.
		location := assetQueryDateLocation(viewerTimeZone)
		if dateFrom != nil && filter.Date.FromDateOnly {
			start := time.Date(dateFrom.Year(), dateFrom.Month(), dateFrom.Day(), 0, 0, 0, 0, location)
			dateFrom = &start
		}
		if dateFrom != nil && dateTo == nil && filter.Date.FromDateOnly {
			end := time.Date(dateFrom.Year(), dateFrom.Month(), dateFrom.Day(), 23, 59, 59, int(time.Second-time.Nanosecond), location)
			dateTo = &end
		} else if dateTo != nil && filter.Date.ToDateOnly {
			end := time.Date(dateTo.Year(), dateTo.Month(), dateTo.Day(), 23, 59, 59, int(time.Second-time.Nanosecond), location)
			dateTo = &end
		}
	}

	var albumIDPtr *int32
	if filter.AlbumID != nil {
		id := int32(*filter.AlbumID)
		albumIDPtr = &id
	}

	var filenameValue, filenameOperator *string
	if filter.Filename != nil && strings.TrimSpace(filter.Filename.Value) != "" {
		value := strings.TrimSpace(filter.Filename.Value)
		operator := normalizeFilenameOperator(filter.Filename.Operator)
		filenameValue = &value
		filenameOperator = &operator
	}

	var locationNorth, locationSouth, locationEast, locationWest *float64
	if filter.Location != nil {
		locationNorth = &filter.Location.North
		locationSouth = &filter.Location.South
		locationEast = &filter.Location.East
		locationWest = &filter.Location.West
	}

	var folderPath *string
	if filter.FolderPath != nil {
		normalized := normalizeFolderPath(*filter.FolderPath)
		folderPath = &normalized
	}

	return service.QueryAssetsParams{
		Query:            query,
		SearchType:       searchType,
		ViewerTimeZone:   viewerTimeZone,
		RepositoryID:     filter.RepositoryID,
		EventID:          filter.EventID,
		AssetType:        filter.Type,
		AssetTypes:       filter.Types,
		OwnerID:          filter.OwnerID,
		AlbumID:          albumIDPtr,
		FilenameValue:    filenameValue,
		FilenameOperator: filenameOperator,
		DateFrom:         dateFrom,
		DateTo:           dateTo,
		MediaComposition: mediaComposition,
		StackMembership:  stackMembership,
		StackKinds:       stackKinds,
		IsDeleted:        filter.IsDeleted,
		Rating:           filter.Rating,
		Liked:            filter.Liked,
		CameraModel:      filter.CameraModel,
		LensModel:        filter.Lens,
		TagName:          filter.TagName,
		TagSource:        filter.TagSource,
		TagNames:         filter.TagNames,
		PersonID:         filter.PersonID,
		FolderPath:       folderPath,
		FolderRecursive:  filter.FolderRecursive,
		LocationNorth:    locationNorth,
		LocationSouth:    locationSouth,
		LocationEast:     locationEast,
		LocationWest:     locationWest,
		SortBy:           normalizeAssetQuerySortBy(sortBy),
		StackMode:        strings.ToLower(strings.TrimSpace(stackMode)),
		Limit:            pagination.Limit,
		Offset:           pagination.Offset,
	}, nil
}

func toAssetDTOs(assets []repo.Asset) []dto.AssetDTO {
	items := make([]dto.AssetDTO, len(assets))
	for i, asset := range assets {
		items[i] = dto.ToAssetDTO(asset)
	}
	return items
}

func toBrowseMediaItemDTO(media service.BrowseMediaItem) dto.BrowseMediaItemDTO {
	item := dto.BrowseMediaItemDTO{
		MediaItemID:  media.MediaItemID.String(),
		MediaKind:    media.MediaKind,
		PrimaryAsset: dto.ToAssetDTO(media.PrimaryAsset),
		Composition: dto.MediaCompositionDTO{
			ComponentCount: media.ComponentCount,
			HasRAW:         media.HasRAW,
			HasJPEG:        media.HasJPEG,
			HasEdited:      media.HasEdited,
			HasLiveMotion:  media.HasLiveMotion,
		},
	}
	if media.StackID != uuid.Nil {
		item.Stack = &dto.StackPreviewDTO{
			StackID:   media.StackID.String(),
			StackKind: string(media.StackKind),
		}
	}
	return item
}

func toBrowseStackMemberDTOs(members []service.BrowseStackMember) []dto.BrowseStackMemberDTO {
	items := make([]dto.BrowseStackMemberDTO, 0, len(members))
	for _, member := range members {
		items = append(items, dto.BrowseStackMemberDTO{
			MediaItemID:    member.MediaItemID.String(),
			PrimaryAssetID: member.PrimaryAssetID.String(),
		})
	}
	return items
}

func toBrowseItemDTOs(items []service.BrowseItem) []dto.BrowseItemDTO {
	dtos := make([]dto.BrowseItemDTO, 0, len(items))
	for _, item := range items {
		if item.Type == service.BrowseItemTypeStack && item.Stack != nil {
			cover := toBrowseMediaItemDTO(item.Stack.Cover)
			stackSize := len(item.Stack.Members)
			cover.Stack = &dto.StackPreviewDTO{
				StackID:    item.Stack.StackID.String(),
				StackKind:  string(item.Stack.Kind),
				StackCover: true,
				StackSize:  &stackSize,
			}
			dtos = append(dtos, dto.BrowseItemDTO{
				Type:     service.BrowseItemTypeStack,
				ID:       item.ID,
				BestTsMs: item.BestTsMs,
				Stack: &dto.BrowseStackDTO{
					StackID:        item.Stack.StackID.String(),
					StackKind:      string(item.Stack.Kind),
					Cover:          cover,
					Members:        toBrowseStackMemberDTOs(item.Stack.Members),
					MatchedMembers: toBrowseStackMemberDTOs(item.Stack.MatchedMembers),
				},
			})
			continue
		}

		if item.MediaItem == nil {
			continue
		}
		media := toBrowseMediaItemDTO(*item.MediaItem)
		dtos = append(dtos, dto.BrowseItemDTO{
			Type:      service.BrowseItemTypeMediaItem,
			ID:        item.ID,
			MediaItem: &media,
			BestTsMs:  item.BestTsMs,
		})
	}
	return dtos
}

func toQueryBrowseResponseDTO(result service.BrowseQueryResult, limit, offset int) dto.QueryAssetsResponseDTO {
	totalVisible := int(result.TotalVisible)
	totalMediaItems := int(result.TotalMediaItems)
	totalFiles := int(result.TotalFiles)
	itemDTOs := toBrowseItemDTOs(result.Items)
	return dto.QueryAssetsResponseDTO{
		Items:           itemDTOs,
		TotalVisible:    &totalVisible,
		TotalMediaItems: &totalMediaItems,
		TotalFiles:      &totalFiles,
		StackMode:       result.StackMode,
		Limit:           limit,
		Offset:          offset,
	}
}

func toSearchBrowseResponseDTO(result service.SearchBrowseResult, limit, offset int) dto.SearchAssetsResponseDTO {
	resultsTotalVisible := int(result.ResultsTotalVisible)
	resultsTotalMediaItems := int(result.ResultsTotalMediaItems)
	topItemDTOs := toBrowseItemDTOs(result.TopResults)
	resultItemDTOs := toBrowseItemDTOs(result.Results)
	return dto.SearchAssetsResponseDTO{
		TopItems: topItemDTOs,
		TopResultsMeta: dto.SearchTopResultsMetaDTO{
			Enabled:           result.TopResultsMeta.Enabled,
			Degraded:          result.TopResultsMeta.Degraded,
			Reason:            result.TopResultsMeta.Reason,
			SourceTypes:       append([]string{}, result.TopResultsMeta.SourceTypes...),
			CandidateCount:    result.TopResultsMeta.CandidateCount,
			CandidatePoolSize: result.TopResultsMeta.CandidatePoolSize,
			Sources:           toSearchSourceMetaDTOs(result.TopResultsMeta.Sources),
			Debug:             toSearchDebugItemDTOs(result.TopResultsMeta.Debug),
		},
		ResultItems:            resultItemDTOs,
		ResultsTotalVisible:    &resultsTotalVisible,
		ResultsTotalMediaItems: &resultsTotalMediaItems,
		Limit:                  limit,
		Offset:                 offset,
	}
}

func toSearchSourceMetaDTOs(sources []service.SearchSourceMeta) []dto.SearchSourceMetaDTO {
	items := make([]dto.SearchSourceMetaDTO, 0, len(sources))
	for _, source := range sources {
		items = append(items, dto.SearchSourceMetaDTO{
			Type:           source.Type,
			Weight:         source.Weight,
			CandidateCount: source.CandidateCount,
			DurationMs:     source.DurationMs,
			Error:          source.Error,
		})
	}
	return items
}

func toSearchDebugItemDTOs(debug []service.SearchDebugItem) []dto.SearchDebugItemDTO {
	items := make([]dto.SearchDebugItemDTO, 0, len(debug))
	for _, item := range debug {
		contributions := make(map[string]dto.SearchDebugContributionDTO, len(item.Contributions))
		for source, contribution := range item.Contributions {
			contributions[source] = dto.SearchDebugContributionDTO{
				Rank:     contribution.Rank,
				Weight:   contribution.Weight,
				RRFScore: contribution.RRFScore,
				RawScore: contribution.RawScore,
			}
		}
		items = append(items, dto.SearchDebugItemDTO{
			AssetID:       item.AssetID,
			Score:         item.Score,
			Contributions: contributions,
		})
	}
	return items
}

// QueryAssets handles unified asset listing, filtering, and searching
// @Summary Query assets (unified endpoint)
// @Description Unified endpoint for listing, filtering, and searching assets. Replaces separate /filter and /search endpoints.
// @Tags assets
// @Produce json
// @Param data body dto.AssetQueryRequestDTO true "Query parameters"
// @Success 200 {object} dto.QueryAssetsResponseDTO "Assets queried successfully"
// @Failure 400 {object} api.ProblemResponse "Invalid request parameters"
// @Failure 503 {object} api.ProblemResponse "Image Semantic Analysis unavailable"
// @Failure 500 {object} api.ProblemResponse "Internal server error"
// @Router /api/v1/assets/list [post]
func (h *AssetHandler) QueryAssets(c *gin.Context) {
	var req dto.AssetQueryRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	normalizeAssetQueryPagination(&req.Pagination)

	if err := validateAssetQuerySearchType(req.SearchType); err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}
	if err := validateAssetQuerySortBy(req.SortBy); err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}
	if err := validateStackMode(req.StackMode); err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	// Default to filename search if not specified
	if req.SearchType == "" {
		req.SearchType = "filename"
	}

	params, err := buildQueryAssetsParams(req.Query, req.SearchType, req.SortBy, req.ViewerTimezone, req.StackMode, req.Filter, req.Pagination)
	if err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}
	params = applyAssetOwnershipScope(c, params)

	browseResult, err := h.assetService.QueryBrowseItems(c.Request.Context(), params)
	if err != nil {
		if errors.Is(err, service.ErrInvalidBrowseFilter) {
			api.WriteProblem(c, api.BadRequest(err))
			return
		}
		// Check for semantic search unavailable error
		if errors.Is(err, service.ErrSemanticSearchUnavailable) {
			api.WriteProblem(c, api.StatusProblem(503, err))
			return
		}
		log.Printf("Failed to query assets: %v", err)
		api.WriteProblem(c, api.Internal(err))
		return
	}

	response := toQueryBrowseResponseDTO(
		browseResult,
		req.Pagination.Limit,
		req.Pagination.Offset,
	)
	api.JSONOK(c, response)
}

// SearchAssets handles sectioned asset search with best-effort top results.
// @Summary Search assets
// @Description Search assets with optional top results enhancement, filename fallback, or visual similarity to a catalog asset.
// @Tags assets
// @Produce json
// @Param data body dto.SearchAssetsRequestDTO true "Search parameters"
// @Success 200 {object} dto.SearchAssetsResponseDTO "Assets searched successfully"
// @Failure 400 {object} api.ProblemResponse "Invalid request parameters"
// @Failure 404 {object} api.ProblemResponse "Query asset not found"
// @Failure 409 {object} api.ProblemResponse "Query asset has no Image Semantic Analysis embedding"
// @Failure 503 {object} api.ProblemResponse "Image Semantic Analysis unavailable"
// @Failure 500 {object} api.ProblemResponse "Internal server error"
// @Router /api/v1/assets/search [post]
func (h *AssetHandler) SearchAssets(c *gin.Context) {
	var req dto.SearchAssetsRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	normalizeAssetQueryPagination(&req.Pagination)
	if err := validateAssetQuerySortBy(req.SortBy); err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}
	if err := rejectSearchStackMode(req.StackMode); err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	if err := validateSearchEnhancementMode(req.EnhancementMode); err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}
	if strings.TrimSpace(req.EnhancementMode) == "" {
		req.EnhancementMode = string(service.SearchEnhancementModeAuto)
	}

	similarID, err := parseSimilarToAssetID(req.SimilarToAssetID)
	if err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}
	query := strings.TrimSpace(req.Query)
	if similarID != nil && query != "" {
		api.WriteProblem(c, api.BadRequest(errors.New("query and similar_to_asset_id are mutually exclusive")))
		return
	}
	if similarID != nil {
		if _, ok := h.loadVisibleSearchQueryAsset(c, *similarID); !ok {
			return
		}
	}

	params, err := buildQueryAssetsParams(query, "filename", req.SortBy, req.ViewerTimezone, "", req.Filter, req.Pagination)
	if err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}
	params = applyAssetOwnershipScope(c, params)

	result, err := h.assetService.SearchBrowseItems(c.Request.Context(), service.SearchAssetsParams{
		QueryAssetsParams: params,
		EnhancementMode:   service.SearchEnhancementMode(req.EnhancementMode),
		TopResultsLimit:   req.TopResultsLimit,
		Debug:             req.Debug,
		SimilarToAssetID:  similarID,
	})
	if err != nil {
		if !h.respondVisualSearchError(c, err) {
			return
		}
		if errors.Is(err, service.ErrInvalidBrowseFilter) {
			api.WriteProblem(c, api.BadRequest(err))
			return
		}
		log.Printf("Failed to search assets: %v", err)
		api.WriteProblem(c, api.Internal(err))
		return
	}

	searchResponse := toSearchBrowseResponseDTO(result, req.Pagination.Limit, req.Pagination.Offset)

	api.JSONOK(c, searchResponse)
}

const (
	maxImageSearchUploadBytes         = 256 << 20
	imageSearchMultipartMemoryBytes   = 32 << 20
	imageSearchMultipartOverheadBytes = 8 << 20
)

// SearchAssetsByImage searches the catalog by a live-embedded local image file.
// @Summary Search assets by image
// @Description Embed an uploaded image with Image Semantic Analysis and return visually similar catalog media. The original is reduced to an in-memory medium thumbnail, then discarded; it is not stored. RAW uses the same OpenPhoto path as ingest. Maximum upload size is 256 MiB.
// @Tags assets
// @Accept multipart/form-data
// @Produce json
// @Param file formData file true "Query image"
// @Param filter formData string false "JSON AssetFilterDTO"
// @Param limit formData int false "Page size"
// @Param offset formData int false "Page offset"
// @Param top_results_limit formData int false "KNN cap, maximum 200"
// @Param viewer_timezone formData string false "Viewer timezone"
// @Success 200 {object} dto.SearchAssetsResponseDTO "Assets searched successfully"
// @Failure 400 {object} api.ProblemResponse "Invalid request parameters"
// @Failure 503 {object} api.ProblemResponse "Image Semantic Analysis unavailable"
// @Failure 500 {object} api.ProblemResponse "Internal server error"
// @Router /api/v1/assets/search/by-image [post]
func (h *AssetHandler) SearchAssetsByImage(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxImageSearchUploadBytes+imageSearchMultipartOverheadBytes)
	if err := c.Request.ParseMultipartForm(imageSearchMultipartMemoryBytes); err != nil {
		if isImageSearchTooLarge(err) {
			api.WriteProblem(c, api.BadRequest(service.ErrAssetFileTooLarge))
			return
		}
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}
	defer file.Close()
	if header.Size > maxImageSearchUploadBytes {
		api.WriteProblem(c, api.BadRequest(service.ErrAssetFileTooLarge))
		return
	}

	filename := filepath.Base(filepath.ToSlash(header.Filename))
	if filename == "." || filename == string(filepath.Separator) {
		filename = ""
	}
	queryPath, queryBytes, err := readImageSearchUpload(file, header)
	if err != nil {
		if isImageSearchTooLarge(err) {
			api.WriteProblem(c, api.BadRequest(service.ErrAssetFileTooLarge))
			return
		}
		api.WriteProblem(c, api.BadRequest(err))
		return
	}
	if queryPath == "" && len(queryBytes) == 0 {
		api.WriteProblem(c, api.BadRequest(errors.New("empty image file")))
		return
	}

	filter, err := parseImageSearchFilter(c.PostForm("filter"))
	if err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}
	pagination := dto.PaginationDTO{
		Limit:  formIntDefault(c.PostForm("limit"), 20),
		Offset: formIntDefault(c.PostForm("offset"), 0),
	}
	normalizeAssetQueryPagination(&pagination)
	topResultsLimit := formIntDefault(c.PostForm("top_results_limit"), 0)

	params, err := buildQueryAssetsParams("", "filename", "", c.PostForm("viewer_timezone"), "", filter, pagination)
	if err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}
	params = applyAssetOwnershipScope(c, params)

	result, err := h.assetService.SearchBrowseItems(c.Request.Context(), service.SearchAssetsParams{
		QueryAssetsParams:  params,
		EnhancementMode:    service.SearchEnhancementModeOnly,
		TopResultsLimit:    topResultsLimit,
		QueryImage:         queryBytes,
		QueryImagePath:     queryPath,
		QueryImageFilename: filename,
	})
	if err != nil {
		if !h.respondVisualSearchError(c, err) {
			return
		}
		if errors.Is(err, service.ErrInvalidBrowseFilter) {
			api.WriteProblem(c, api.BadRequest(err))
			return
		}
		log.Printf("Failed to search assets by image: %v", err)
		api.WriteProblem(c, api.Internal(err))
		return
	}

	api.JSONOK(c, toSearchBrowseResponseDTO(result, pagination.Limit, pagination.Offset))
}

func readImageSearchUpload(file multipart.File, header *multipart.FileHeader) (string, []byte, error) {
	if osFile, ok := file.(*os.File); ok {
		if header.Size == 0 {
			info, err := osFile.Stat()
			if err != nil {
				return "", nil, err
			}
			if info.Size() == 0 {
				return "", nil, nil
			}
			if info.Size() > maxImageSearchUploadBytes {
				return "", nil, &http.MaxBytesError{Limit: maxImageSearchUploadBytes}
			}
		}
		return osFile.Name(), nil, nil
	}

	imageBytes, err := io.ReadAll(io.LimitReader(file, maxImageSearchUploadBytes+1))
	if err != nil {
		return "", nil, err
	}
	if int64(len(imageBytes)) > maxImageSearchUploadBytes {
		return "", nil, &http.MaxBytesError{Limit: maxImageSearchUploadBytes}
	}
	return "", imageBytes, nil
}

func isImageSearchTooLarge(err error) bool {
	var maxBytes *http.MaxBytesError
	return errors.As(err, &maxBytes)
}

func parseSimilarToAssetID(raw *string) (*uuid.UUID, error) {
	if raw == nil {
		return nil, nil
	}
	value := strings.TrimSpace(*raw)
	if value == "" {
		return nil, nil
	}
	id, err := uuid.Parse(value)
	if err != nil {
		return nil, err
	}
	return &id, nil
}

func (h *AssetHandler) loadVisibleSearchQueryAsset(c *gin.Context, assetID uuid.UUID) (*repo.Asset, bool) {
	asset, err := h.assetService.GetAssetAny(c.Request.Context(), assetID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			api.WriteProblem(c, api.NotFound(err))
			return nil, false
		}
		api.WriteProblem(c, api.Internal(err))
		return nil, false
	}

	user, ok := currentUserFromContext(c)
	if !ok {
		if asset.OwnerID != nil {
			api.WriteProblem(c, api.NotFound(errors.New("asset not found")))
			return nil, false
		}
		return asset, true
	}
	if service.IsAdminRole(user.Role) || asset.OwnerID == nil || int32(user.UserID) == *asset.OwnerID {
		return asset, true
	}
	api.WriteProblem(c, api.NotFound(errors.New("asset not found")))
	return nil, false
}

func (h *AssetHandler) respondVisualSearchError(c *gin.Context, err error) bool {
	switch {
	case errors.Is(err, service.ErrEmbeddingMissing):
		api.WriteProblem(c, api.KnownProblem(problem.ImageEmbeddingMissing, err))
		return false
	case errors.Is(err, service.ErrSemanticSearchUnavailable):
		api.WriteProblem(c, api.KnownProblem(problem.SemanticAnalysisUnavailable, err))
		return false
	case errors.Is(err, service.ErrInvalidImageQuery):
		api.WriteProblem(c, api.BadRequest(err))
		return false
	default:
		return true
	}
}

func parseImageSearchFilter(raw string) (dto.AssetFilterDTO, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return dto.AssetFilterDTO{}, nil
	}
	var filter dto.AssetFilterDTO
	if err := json.Unmarshal([]byte(raw), &filter); err != nil {
		return dto.AssetFilterDTO{}, err
	}
	return filter, nil
}

func formIntDefault(raw string, fallback int) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}

// GetFeaturedAssets returns deterministic curated featured photos.
// @Summary Get featured photos
// @Description Select a small set of featured photos using deterministic weighted sampling (A-ES) with diversity constraints.
// @Tags assets
// @Accept json
// @Produce json
// @Param count query int false "Number of featured photos to return" default(8)
// @Param candidate_limit query int false "Max candidate photos considered before selection" default(240)
// @Param days query int false "Only consider photos from the last N days (0 disables date cutoff)" default(3650)
// @Param seed query string false "Deterministic seed (default: current UTC date YYYY-MM-DD)"
// @Param repository_id query string false "Optional repository UUID filter"
// @Success 200 {object} dto.FeaturedAssetsResponseDTO "Featured photos selected successfully"
// @Failure 400 {object} api.ProblemResponse "Invalid request parameters"
// @Failure 500 {object} api.ProblemResponse "Internal server error"
// @Router /api/v1/assets/featured [get]
func (h *AssetHandler) GetFeaturedAssets(c *gin.Context) {
	count, err := parseIntQueryWithRange(c, "count", 8, 1, 24)
	if err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	candidateLimit, err := parseIntQueryWithRange(c, "candidate_limit", 240, 16, 1000)
	if err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	days, err := parseIntQueryWithRange(c, "days", 3650, 0, 36500)
	if err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	seed := strings.TrimSpace(c.Query("seed"))
	now := time.Now().UTC()
	if seed == "" {
		seed = now.Format("2006-01-02")
	}

	var repositoryID *string
	if rawRepoID := strings.TrimSpace(c.Query("repository_id")); rawRepoID != "" {
		if _, err := uuid.Parse(rawRepoID); err != nil {
			api.WriteProblem(c, api.BadRequest(err))
			return
		}
		repositoryID = &rawRepoID
	}

	var dateFrom *time.Time
	if days > 0 {
		from := now.AddDate(0, 0, -days)
		dateFrom = &from
	}

	photoType := service.AssetTypePhoto
	params := service.QueryAssetsParams{
		SearchType:   "filename",
		RepositoryID: repositoryID,
		AssetType:    &photoType,
		DateFrom:     dateFrom,
		Limit:        candidateLimit,
		Offset:       0,
	}
	params = applyAssetOwnershipScope(c, params)

	assets, _, err := h.assetService.QueryAssets(c.Request.Context(), params)
	if err != nil {
		log.Printf("Failed to query featured candidate assets: %v", err)
		api.WriteProblem(c, api.Internal(err))
		return
	}

	selected := service.SelectFeaturedPhotos(assets, service.FeaturedSelectionOptions{
		Count: count,
		Seed:  seed,
		Now:   now,
	})

	uniqueCandidates := countUniqueAssets(assets)

	dtos := make([]dto.AssetDTO, len(selected))
	for i, a := range selected {
		dtos[i] = dto.ToAssetDTO(a)
	}

	response := dto.FeaturedAssetsResponseDTO{
		Assets:          dtos,
		Count:           len(dtos),
		CandidateCount:  uniqueCandidates,
		Seed:            seed,
		Strategy:        "weighted_aes_v1",
		GeneratedAtTime: now,
	}
	api.JSONOK(c, response)
}

// GetPhotoMapPoints returns lightweight photo map points with valid GPS coordinates.
// @Summary Get photo map points
// @Description Return lightweight paginated photo records containing only map-related fields (asset ID, filename, times, GPS lat/lon).
// @Tags assets
// @Accept json
// @Produce json
// @Param limit query int false "Page size (1-5000)" default(1000)
// @Param offset query int false "Page offset" default(0)
// @Param repository_id query string false "Optional repository UUID filter"
// @Param south query number false "Viewport south latitude (-90 to 90)"
// @Param north query number false "Viewport north latitude (-90 to 90)"
// @Param west query number false "Viewport west longitude (-180 to 180)"
// @Param east query number false "Viewport east longitude (-180 to 180)"
// @Success 200 {object} dto.AssetMapPointListResponseDTO "Map points retrieved successfully"
// @Failure 400 {object} api.ProblemResponse "Invalid request parameters"
// @Failure 500 {object} api.ProblemResponse "Internal server error"
// @Router /api/v1/assets/map-points [get]
func (h *AssetHandler) GetPhotoMapPoints(c *gin.Context) {
	limit, err := parseIntQueryWithRange(c, "limit", 1000, 1, 5000)
	if err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	offset, err := parseIntQueryWithRange(c, "offset", 0, 0, 10000000)
	if err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	var repositoryID *string
	if rawRepoID := strings.TrimSpace(c.Query("repository_id")); rawRepoID != "" {
		if _, err := uuid.Parse(rawRepoID); err != nil {
			api.WriteProblem(c, api.BadRequest(err))
			return
		}
		repositoryID = &rawRepoID
	}

	south, north, west, east, err := parseOptionalMapViewport(c)
	if err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	points, total, err := h.assetService.QueryPhotoMapPoints(c.Request.Context(), applyMapPointOwnershipScope(c, service.QueryPhotoMapPointsParams{
		RepositoryID: repositoryID,
		South:        south,
		North:        north,
		West:         west,
		East:         east,
		Limit:        limit,
		Offset:       offset,
	}))
	if err != nil {
		log.Printf("Failed to query photo map points: %v", err)
		api.WriteProblem(c, api.Internal(err))
		return
	}

	pointDTOs := make([]dto.AssetMapPointDTO, len(points))
	for i, point := range points {
		pointDTOs[i] = dto.AssetMapPointDTO{
			AssetID:          point.AssetID,
			OriginalFilename: point.OriginalFilename,
			UploadTime:       point.UploadTime,
			TakenTime:        point.TakenTime,
			GPSLatitude:      point.GPSLatitude,
			GPSLongitude:     point.GPSLongitude,
		}
	}

	totalInt := int(total)
	response := dto.AssetMapPointListResponseDTO{
		Points: pointDTOs,
		Total:  &totalInt,
		Limit:  limit,
		Offset: offset,
	}
	api.JSONOK(c, response)
}

func parseOptionalMapViewport(c *gin.Context) (*float64, *float64, *float64, *float64, error) {
	names := []string{"south", "north", "west", "east"}
	values := make([]*float64, len(names))
	present := 0
	for index, name := range names {
		raw, exists := c.GetQuery(name)
		if !exists || strings.TrimSpace(raw) == "" {
			continue
		}
		value, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return nil, nil, nil, nil, fmt.Errorf("parse %s: %w", name, err)
		}
		values[index] = &value
		present++
	}
	if present == 0 {
		return nil, nil, nil, nil, nil
	}
	if present != len(names) {
		return nil, nil, nil, nil, errors.New("south, north, west, and east must be provided together")
	}
	south, north, west, east := *values[0], *values[1], *values[2], *values[3]
	if south < -90 || south > 90 || north < -90 || north > 90 || south > north {
		return nil, nil, nil, nil, errors.New("latitude bounds must satisfy -90 <= south <= north <= 90")
	}
	if west < -180 || west > 180 || east < -180 || east > 180 {
		return nil, nil, nil, nil, errors.New("longitude bounds must be between -180 and 180")
	}
	return values[0], values[1], values[2], values[3], nil
}

func parseIntQueryWithRange(
	c *gin.Context,
	name string,
	defaultValue int,
	minValue int,
	maxValue int,
) (int, error) {
	raw := strings.TrimSpace(c.Query(name))
	if raw == "" {
		return defaultValue, nil
	}

	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, err
	}
	if value < minValue || value > maxValue {
		return 0, fmt.Errorf("%s must be between %d and %d", name, minValue, maxValue)
	}
	return value, nil
}

func countUniqueAssets(assets []repo.Asset) int {
	seen := make(map[string]struct{}, len(assets))
	for _, asset := range assets {
		if asset.AssetID == uuid.Nil {
			continue
		}
		seen[asset.AssetID.String()] = struct{}{}
	}
	return len(seen)
}

// GetFilterOptions returns available options for filters
// @Summary Get filter options
// @Description Get available camera models and lenses for filter dropdowns
// @Tags assets
// @Accept json
// @Produce json
// @Success 200 {object} dto.OptionsResponseDTO "Filter options retrieved successfully"
// @Failure 500 {object} api.ProblemResponse "Internal server error"
// @Router /api/v1/assets/filter-options [get]
func (h *AssetHandler) GetFilterOptions(c *gin.Context) {
	ctx := c.Request.Context()

	cameraModels, err := h.assetService.GetDistinctCameraModels(ctx)
	if err != nil {
		log.Printf("Failed to get camera models: %v", err)
		api.WriteProblem(c, api.Internal(err))
		return
	}

	lenses, err := h.assetService.GetDistinctLenses(ctx)
	if err != nil {
		log.Printf("Failed to get lenses: %v", err)
		api.WriteProblem(c, api.Internal(err))
		return
	}

	response := dto.OptionsResponseDTO{
		CameraModels: cameraModels,
		Lenses:       lenses,
	}
	api.JSONOK(c, response)
}

// GetAssetsByRating gets assets filtered by rating
// @Summary Get assets by rating
// @Description Get assets with a specific rating (0-5)
// @Tags assets
// @Accept json
// @Produce json
// @Param rating path int true "Rating (0-5)"
// @Param limit query int false "Number of assets to return" default(20)
// @Param offset query int false "Number of assets to skip" default(0)
// @Success 200 {object} dto.AssetListResponseDTO "Assets retrieved successfully"
// @Failure 400 {object} api.ProblemResponse "Bad request"
// @Failure 500 {object} api.ProblemResponse "Internal server error"
// @Router /api/v1/assets/rating/{rating} [get]
func (h *AssetHandler) GetAssetsByRating(c *gin.Context) {
	ratingStr := c.Param("rating")
	rating, err := strconv.Atoi(ratingStr)
	if err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	if rating < 0 || rating > 5 {
		api.WriteProblem(c, api.BadRequest(nil))
		return
	}

	limit := 20
	offset := 0

	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	if offsetStr := c.Query("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	user, ok := requireCurrentUser(c)
	if !ok {
		return
	}
	var ownerID *int32
	if !service.IsAdminRole(user.Role) {
		id := int32(user.UserID)
		ownerID = &id
	}

	assets, err := h.assetService.GetAssetsByRating(c.Request.Context(), rating, ownerID, limit, offset)
	if err != nil {
		log.Printf("Failed to get assets by rating: %v", err)
		api.WriteProblem(c, api.Internal(err))
		return
	}

	assetDTOs := make([]dto.AssetDTO, len(assets))
	for i, asset := range assets {
		assetDTOs[i] = dto.ToAssetDTO(asset)
	}

	response := dto.AssetListResponseDTO{
		Assets: assetDTOs,
		Limit:  limit,
		Offset: offset,
	}

	api.JSONOK(c, response)
}

// GetLikedAssets gets all liked/favorited assets
// @Summary Get liked assets
// @Description Get all assets that have been liked/favorited
// @Tags assets
// @Accept json
// @Produce json
// @Param limit query int false "Number of assets to return" default(20)
// @Param offset query int false "Number of assets to skip" default(0)
// @Success 200 {object} dto.AssetListResponseDTO "Liked assets retrieved successfully"
// @Failure 500 {object} api.ProblemResponse "Internal server error"
// @Router /api/v1/assets/liked [get]
func (h *AssetHandler) GetLikedAssets(c *gin.Context) {
	ctx := c.Request.Context()
	limit := 20
	offset := 0

	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	if offsetStr := c.Query("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	user, ok := requireCurrentUser(c)
	if !ok {
		return
	}
	var ownerID *int32
	if !service.IsAdminRole(user.Role) {
		id := int32(user.UserID)
		ownerID = &id
	}

	assets, err := h.assetService.GetLikedAssets(ctx, ownerID, limit, offset)
	if err != nil {
		log.Printf("Failed to get liked assets: %v", err)
		api.WriteProblem(c, api.Internal(err))
		return
	}

	assetDTOs := make([]dto.AssetDTO, len(assets))
	for i, asset := range assets {
		assetDTOs[i] = dto.ToAssetDTO(asset)
	}

	response := dto.AssetListResponseDTO{
		Assets: assetDTOs,
		Limit:  limit,
		Offset: offset,
	}

	api.JSONOK(c, response)
}
