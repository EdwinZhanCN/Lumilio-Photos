package handler

import (
	"context"
	"errors"
	"log"
	"strconv"
	"strings"

	"server/internal/api"
	"server/internal/api/dto"
	"server/internal/db/repo"
	"server/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// DuplicateHandler exposes the Utilities Rail "Duplicates" feature: detection,
// review, merge, and dismiss endpoints used by the frontend Duplicates page.
type DuplicateHandler struct {
	duplicateService service.DuplicateService
	queries          *repo.Queries
}

// NewDuplicateHandler builds the handler with its required collaborators.
func NewDuplicateHandler(duplicateService service.DuplicateService, queries *repo.Queries) *DuplicateHandler {
	return &DuplicateHandler{
		duplicateService: duplicateService,
		queries:          queries,
	}
}

// GetDuplicateSummary returns headline metrics for the Utilities Rail card.
// @Summary Get duplicate detection summary
// @Description Returns counts and recoverable space for pending duplicate groups, scoped by optional repository_id.
// @Tags duplicates
// @Accept json
// @Produce json
// @Param repository_id query string false "Repository UUID to scope the summary"
// @Success 200 {object} dto.DuplicateSummaryDTO
// @Failure 400 {object} api.ErrorResponse
// @Failure 500 {object} api.ErrorResponse
// @Router /api/v1/duplicates/summary [get]
func (h *DuplicateHandler) GetDuplicateSummary(c *gin.Context) {
	repoID, err := optionalRepositoryUUIDParam(c.Query("repository_id"))
	if err != nil {
		api.GinBadRequest(c, err, "Invalid repository_id")
		return
	}
	summary, err := h.duplicateService.GetSummary(c.Request.Context(), repoID, ownerScopeID(c))
	if err != nil {
		log.Printf("get duplicate summary failed: %v", err)
		api.GinInternalError(c, err, "Failed to load duplicate summary")
		return
	}
	api.JSONOK(c, toDuplicateSummaryDTO(summary))
}

// ListDuplicateGroups returns paginated duplicate groups with their assets.
// @Summary List duplicate groups
// @Description Paginated list of duplicate groups, scoped by repository and status (default pending).
// @Tags duplicates
// @Accept json
// @Produce json
// @Param repository_id query string false "Repository UUID"
// @Param status query string false "pending | merged | dismissed (defaults to pending)"
// @Param limit query int false "Page size" default(20)
// @Param offset query int false "Page offset" default(0)
// @Success 200 {object} dto.ListDuplicateGroupsResponseDTO
// @Failure 400 {object} api.ErrorResponse
// @Failure 500 {object} api.ErrorResponse
// @Router /api/v1/duplicates/groups [get]
func (h *DuplicateHandler) ListDuplicateGroups(c *gin.Context) {
	repoID, err := optionalRepositoryUUIDParam(c.Query("repository_id"))
	if err != nil {
		api.GinBadRequest(c, err, "Invalid repository_id")
		return
	}

	status := strings.TrimSpace(c.Query("status"))
	if status == "" {
		status = service.DuplicateStatusPending
	}
	if !isValidDuplicateStatus(status) {
		api.GinBadRequest(c, errors.New("invalid status"), "Invalid status")
		return
	}

	limit, offset := parseDuplicatePagination(c)

	result, err := h.duplicateService.ListGroups(c.Request.Context(), service.ListDuplicateGroupsParams{
		RepositoryID: repoID,
		OwnerID:      ownerScopeID(c),
		Status:       status,
		Limit:        limit,
		Offset:       offset,
	})
	if err != nil {
		log.Printf("list duplicate groups failed: %v", err)
		api.GinInternalError(c, err, "Failed to list duplicate groups")
		return
	}

	groups, err := h.materializeGroups(c.Request.Context(), result.Groups, false)
	if err != nil {
		log.Printf("materialize duplicate groups failed: %v", err)
		api.GinInternalError(c, err, "Failed to load duplicate group assets")
		return
	}

	api.JSONOK(c, dto.ListDuplicateGroupsResponseDTO{
		Groups: groups,
		Total:  result.Total,
		Limit:  limit,
		Offset: offset,
	})
}

// GetDuplicateGroup returns one duplicate group, including pair-level edge evidence.
// @Summary Get a duplicate group
// @Description Returns one duplicate group with all assets and evidence edges.
// @Tags duplicates
// @Accept json
// @Produce json
// @Param id path string true "Duplicate group UUID"
// @Success 200 {object} dto.DuplicateGroupDTO
// @Failure 404 {object} api.ErrorResponse
// @Router /api/v1/duplicates/groups/{id} [get]
func (h *DuplicateHandler) GetDuplicateGroup(c *gin.Context) {
	groupID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		api.GinBadRequest(c, err, "Invalid duplicate group id")
		return
	}
	detail, err := h.duplicateService.GetGroup(c.Request.Context(), groupID, ownerScopeID(c))
	if err != nil {
		if errors.Is(err, service.ErrDuplicateGroupNotFound) {
			api.GinNotFound(c, err, "Duplicate group not found")
			return
		}
		log.Printf("get duplicate group failed: %v", err)
		api.GinInternalError(c, err, "Failed to load duplicate group")
		return
	}
	groups, err := h.materializeGroups(c.Request.Context(), []service.DuplicateGroupDetail{detail}, true)
	if err != nil {
		log.Printf("materialize duplicate group failed: %v", err)
		api.GinInternalError(c, err, "Failed to load duplicate group assets")
		return
	}
	if len(groups) == 0 {
		api.GinNotFound(c, errors.New("group disappeared"), "Duplicate group not found")
		return
	}
	api.JSONOK(c, groups[0])
}

// DetectDuplicates triggers a synchronous detection run for a repository.
// @Summary Detect duplicates for a repository
// @Description Rebuilds the pending duplicate graph for a repository by combining exact-hash and pHash edges.
// @Tags duplicates
// @Accept json
// @Produce json
// @Param request body dto.DetectDuplicatesRequestDTO true "Repository to scan"
// @Success 200 {object} dto.DetectDuplicatesResponseDTO
// @Failure 400 {object} api.ErrorResponse
// @Failure 500 {object} api.ErrorResponse
// @Router /api/v1/duplicates/detect [post]
func (h *DuplicateHandler) DetectDuplicates(c *gin.Context) {
	var req dto.DetectDuplicatesRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		api.GinBadRequest(c, err, "Invalid request body")
		return
	}
	repoID, err := uuid.Parse(req.RepositoryID)
	if err != nil {
		api.GinBadRequest(c, err, "Invalid repository_id")
		return
	}
	result, err := h.duplicateService.DetectForRepository(c.Request.Context(), repoID)
	if err != nil {
		log.Printf("duplicate detection failed: %v", err)
		api.GinInternalError(c, err, "Duplicate detection failed")
		return
	}
	api.JSONOK(c, dto.DetectDuplicatesResponseDTO{
		Groups:         result.Groups,
		ExactGroups:    result.ExactGroups,
		PHashGroups:    result.PHashGroups,
		MixedGroups:    result.MixedGroups,
		AssetsAffected: result.AssetsAffected,
		GeneratedAt:    result.GeneratedAt,
	})
}

// MergeDuplicateGroup performs the keeper/merge/soft-delete cascade.
// @Summary Merge a duplicate group
// @Description Keeps the chosen asset, unions metadata from duplicates, and soft-deletes the remaining members.
// @Tags duplicates
// @Accept json
// @Produce json
// @Param id path string true "Duplicate group UUID"
// @Param request body dto.MergeDuplicateGroupRequestDTO true "Merge configuration"
// @Success 200 {object} dto.MergeDuplicateGroupResponseDTO
// @Failure 400 {object} api.ErrorResponse
// @Failure 404 {object} api.ErrorResponse
// @Failure 409 {object} api.ErrorResponse
// @Failure 500 {object} api.ErrorResponse
// @Router /api/v1/duplicates/groups/{id}/merge [post]
func (h *DuplicateHandler) MergeDuplicateGroup(c *gin.Context) {
	groupID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		api.GinBadRequest(c, err, "Invalid duplicate group id")
		return
	}

	var req dto.MergeDuplicateGroupRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		api.GinBadRequest(c, err, "Invalid request body")
		return
	}
	keeperID, err := uuid.Parse(req.KeeperAssetID)
	if err != nil {
		api.GinBadRequest(c, err, "Invalid keeper_asset_id")
		return
	}
	duplicates := make([]uuid.UUID, 0, len(req.DuplicateAssetIDs))
	for _, raw := range req.DuplicateAssetIDs {
		id, parseErr := uuid.Parse(raw)
		if parseErr != nil {
			api.GinBadRequest(c, parseErr, "Invalid duplicate_asset_ids entry")
			return
		}
		duplicates = append(duplicates, id)
	}

	policy := service.DefaultMergePolicy()
	if req.Policy != nil {
		applyPolicyFlag(&policy.Albums, req.Policy.Albums)
		applyPolicyFlag(&policy.Tags, req.Policy.Tags)
		applyPolicyFlag(&policy.Rating, req.Policy.Rating)
		applyPolicyFlag(&policy.Liked, req.Policy.Liked)
		applyPolicyFlag(&policy.Description, req.Policy.Description)
		applyPolicyFlag(&policy.Faces, req.Policy.Faces)
	}

	result, err := h.duplicateService.MergeGroup(c.Request.Context(), service.MergeGroupParams{
		GroupID:           groupID,
		KeeperAssetID:     keeperID,
		DuplicateAssetIDs: duplicates,
		Policy:            policy,
		RequireOwner:      ownerScopeID(c),
	})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrDuplicateGroupNotFound):
			api.GinNotFound(c, err, "Duplicate group not found")
		case errors.Is(err, service.ErrDuplicateGroupAlreadyResolved):
			api.GinError(c, 409, err, 409, "Duplicate group already resolved")
		case errors.Is(err, service.ErrDuplicateKeeperInvalid):
			api.GinBadRequest(c, err, "Invalid keeper or duplicate selection")
		default:
			log.Printf("merge duplicate group failed: %v", err)
			api.GinInternalError(c, err, "Failed to merge duplicate group")
		}
		return
	}

	mergedIDs := make([]string, len(result.MergedDuplicates))
	for i, id := range result.MergedDuplicates {
		mergedIDs[i] = id.String()
	}
	api.JSONOK(c, dto.MergeDuplicateGroupResponseDTO{
		GroupID:          result.GroupID.String(),
		KeeperAssetID:    result.KeeperAssetID.String(),
		MergedDuplicates: mergedIDs,
		RecoveredBytes:   result.RecoveredBytes,
	})
}

// DismissDuplicateGroup marks the group as not-a-duplicate without merging.
// @Summary Dismiss a duplicate group
// @Description Marks a duplicate group as dismissed without merging any assets.
// @Tags duplicates
// @Accept json
// @Produce json
// @Param id path string true "Duplicate group UUID"
// @Success 200 {object} dto.MessageResponseDTO
// @Failure 404 {object} api.ErrorResponse
// @Failure 409 {object} api.ErrorResponse
// @Router /api/v1/duplicates/groups/{id}/dismiss [post]
func (h *DuplicateHandler) DismissDuplicateGroup(c *gin.Context) {
	groupID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		api.GinBadRequest(c, err, "Invalid duplicate group id")
		return
	}
	if err := h.duplicateService.DismissGroup(c.Request.Context(), groupID, ownerScopeID(c)); err != nil {
		switch {
		case errors.Is(err, service.ErrDuplicateGroupNotFound):
			api.GinNotFound(c, err, "Duplicate group not found")
		case errors.Is(err, service.ErrDuplicateGroupAlreadyResolved):
			api.GinError(c, 409, err, 409, "Duplicate group already resolved")
		default:
			log.Printf("dismiss duplicate group failed: %v", err)
			api.GinInternalError(c, err, "Failed to dismiss duplicate group")
		}
		return
	}
	api.JSONOK(c, dto.MessageResponseDTO{Message: "Duplicate group dismissed"})
}

// ----------------------------------------------------------------------------
// Helpers
// ----------------------------------------------------------------------------

// materializeGroups fetches every member asset referenced by the supplied
// group details and converts them to the API DTO shape. Edge data is only
// attached when includeEdges is true (the list endpoint omits it).
func (h *DuplicateHandler) materializeGroups(
	ctx context.Context,
	details []service.DuplicateGroupDetail,
	includeEdges bool,
) ([]dto.DuplicateGroupDTO, error) {
	if len(details) == 0 {
		return nil, nil
	}

	// Collect all unique asset IDs across all groups so we can resolve them in
	// a single batched scan rather than N round-trips.
	idSet := make(map[uuid.UUID]struct{})
	for _, d := range details {
		for _, a := range d.Assets {
			idSet[pgUUID(a.AssetID)] = struct{}{}
		}
	}

	assets := make(map[uuid.UUID]repo.Asset, len(idSet))
	for id := range idSet {
		// Each asset may have been soft-deleted (e.g. after a previous merge),
		// in which case we still want to render it for resolved groups. We use
		// the unfiltered "by id" query when available; if the canonical query
		// filters deleted, the resolved view will just omit them.
		asset, err := h.queries.GetAssetByID(ctx, pgtype.UUID{Bytes: id, Valid: true})
		if err != nil {
			// Asset not present (cascade delete elsewhere); skip.
			continue
		}
		assets[id] = asset
	}

	out := make([]dto.DuplicateGroupDTO, 0, len(details))
	for _, d := range details {
		group := toDuplicateGroupDTO(d, assets, includeEdges)
		out = append(out, group)
	}
	return out, nil
}

func toDuplicateSummaryDTO(s service.DuplicateSummary) dto.DuplicateSummaryDTO {
	return dto.DuplicateSummaryDTO{
		PendingGroups:     s.PendingGroups,
		MergedGroups:      s.MergedGroups,
		DismissedGroups:   s.DismissedGroups,
		PendingAssets:     s.PendingAssets,
		RecoverableAssets: s.RecoverableAssets,
		RecoverableBytes:  s.RecoverableBytes,
		LastDetectedAt:    s.LastDetectedAt,
	}
}

func toDuplicateGroupDTO(
	d service.DuplicateGroupDetail,
	assets map[uuid.UUID]repo.Asset,
	includeEdges bool,
) dto.DuplicateGroupDTO {
	g := d.Group
	groupDTO := dto.DuplicateGroupDTO{
		GroupID:          uuid.UUID(g.GroupID.Bytes).String(),
		RepositoryID:     uuid.UUID(g.RepositoryID.Bytes).String(),
		Method:           g.Method,
		Status:           g.Status,
		AssetCount:       g.AssetCount,
		TotalSize:        g.TotalSize,
		DetectionVersion: g.DetectionVersion,
	}
	if g.DetectedAt.Valid {
		groupDTO.DetectedAt = g.DetectedAt.Time
	}
	if g.ResolvedAt.Valid {
		t := g.ResolvedAt.Time
		groupDTO.ResolvedAt = &t
	}
	if g.RecommendedKeeperAssetID.Valid {
		id := uuid.UUID(g.RecommendedKeeperAssetID.Bytes).String()
		groupDTO.RecommendedKeeperAssetID = &id
	}
	if g.KeeperAssetID.Valid {
		id := uuid.UUID(g.KeeperAssetID.Bytes).String()
		groupDTO.KeeperAssetID = &id
	}

	// Compute recoverable bytes as total_size minus the largest member, matching
	// the SQL definition used for the summary (Apple Photos style "keep one").
	largest := int64(0)
	for _, a := range d.Assets {
		if a.FileSize > largest {
			largest = a.FileSize
		}
	}
	if g.AssetCount >= 2 && g.TotalSize > largest {
		groupDTO.RecoverableBytes = g.TotalSize - largest
	}

	groupDTO.Assets = make([]dto.DuplicateAssetDTO, 0, len(d.Assets))
	for _, a := range d.Assets {
		id := pgUUID(a.AssetID)
		asset, ok := assets[id]
		if !ok {
			continue
		}
		groupDTO.Assets = append(groupDTO.Assets, dto.DuplicateAssetDTO{
			Asset:    dto.ToAssetDTO(asset),
			Role:     a.Role,
			FileSize: a.FileSize,
		})
	}

	if includeEdges {
		groupDTO.Edges = make([]dto.DuplicateEdgeDTO, 0, len(d.Edges))
		for _, e := range d.Edges {
			groupDTO.Edges = append(groupDTO.Edges, dto.DuplicateEdgeDTO{
				AssetIDA:   uuid.UUID(e.AssetIDA.Bytes).String(),
				AssetIDB:   uuid.UUID(e.AssetIDB.Bytes).String(),
				Method:     e.Method,
				Distance:   e.Distance,
				Confidence: e.Confidence,
			})
		}
	}

	return groupDTO
}

func optionalRepositoryUUIDParam(value string) (*uuid.UUID, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, nil
	}
	id, err := uuid.Parse(trimmed)
	if err != nil {
		return nil, err
	}
	return &id, nil
}

func parseDuplicatePagination(c *gin.Context) (int, int) {
	limit := 20
	offset := 0
	if rawLimit := strings.TrimSpace(c.Query("limit")); rawLimit != "" {
		if v, err := strconv.Atoi(rawLimit); err == nil && v > 0 && v <= 100 {
			limit = v
		}
	}
	if rawOffset := strings.TrimSpace(c.Query("offset")); rawOffset != "" {
		if v, err := strconv.Atoi(rawOffset); err == nil && v >= 0 {
			offset = v
		}
	}
	return limit, offset
}

func isValidDuplicateStatus(status string) bool {
	switch status {
	case service.DuplicateStatusPending,
		service.DuplicateStatusMerged,
		service.DuplicateStatusDismissed:
		return true
	default:
		return false
	}
}

func applyPolicyFlag(dst *bool, src *bool) {
	if src == nil {
		return
	}
	*dst = *src
}

func pgUUID(id pgtype.UUID) uuid.UUID {
	if !id.Valid {
		return uuid.Nil
	}
	return uuid.UUID(id.Bytes)
}
