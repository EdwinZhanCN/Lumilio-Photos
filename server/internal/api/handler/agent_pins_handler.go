package handler

import (
	"errors"
	"log"
	"strconv"
	"strings"
	"time"

	"server/internal/agent/pins"
	"server/internal/agent/ref"
	"server/internal/api"
	"server/internal/api/dto"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// CreatePin pins a session ref onto the durable widget board.
// @Summary Pin an Agent Ref
// @Description Copy a session ref into a durable board widget. Live mode replays the producing plan on hydration when replayable; otherwise the pin freezes the snapshot.
// @Tags agent
// @Accept json
// @Produce json
// @Param request body dto.CreateAgentPinRequest true "Pin request"
// @Success 200 {object} dto.AgentPinDTO
// @Failure 400 {object} api.ProblemResponse "Invalid request"
// @Failure 401 {object} api.ProblemResponse "Unauthorized"
// @Failure 404 {object} api.ProblemResponse "Ref not found"
// @Router /api/v1/agent/pins [post]
func (h *AgentHandler) CreatePin(c *gin.Context) {
	var req dto.CreateAgentPinRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}
	user, ok := requireCurrentUser(c)
	if !ok {
		return
	}

	params := pins.CreateParams{
		UserID:   int32(user.UserID),
		ThreadID: req.ThreadID,
		RefID:    req.RefID,
		Title:    req.Title,
		Widget:   req.Widget,
		Mode:     req.Mode,
	}
	if req.Layout != nil {
		params.Layout = pins.Layout{X: req.Layout.X, Y: req.Layout.Y, W: req.Layout.W, H: req.Layout.H}
	}

	pin, err := h.pins.CreateFromRef(c.Request.Context(), params)
	if err != nil {
		if errors.Is(err, pins.ErrNotFound) {
			api.WriteProblem(c, api.NotFound(err))
			return
		}
		api.WriteProblem(c, api.Internal(err))
		return
	}
	api.JSONOK(c, toAgentPinDTO(pin))
}

// ListPins lists the user's board widgets.
// @Summary List Agent Pins
// @Description List all pinned widgets for the current user, in creation order.
// @Tags agent
// @Produce json
// @Success 200 {array} dto.AgentPinDTO
// @Failure 401 {object} api.ProblemResponse "Unauthorized"
// @Router /api/v1/agent/pins [get]
func (h *AgentHandler) ListPins(c *gin.Context) {
	user, ok := requireCurrentUser(c)
	if !ok {
		return
	}
	rows, err := h.pins.List(c.Request.Context(), int32(user.UserID))
	if err != nil {
		api.WriteProblem(c, api.Internal(err))
		return
	}
	out := make([]dto.AgentPinDTO, 0, len(rows))
	for _, pin := range rows {
		out = append(out, toAgentPinDTO(pin))
	}
	api.JSONOK(c, out)
}

// GetPin returns pinned widget metadata with facets.
// @Summary Get Agent Pin Metadata
// @Description Get metadata and facet summary for a pinned widget. Frozen pins serve the stored snapshot; live pins replay their plan before facets are computed.
// @Tags agent
// @Produce json
// @Param id path string true "Pin ID"
// @Success 200 {object} dto.AgentPinDTO
// @Failure 401 {object} api.ProblemResponse "Unauthorized"
// @Failure 404 {object} api.ProblemResponse "Pin not found"
// @Router /api/v1/agent/pins/{id} [get]
func (h *AgentHandler) GetPin(c *gin.Context) {
	user, ok := requireCurrentUser(c)
	if !ok {
		return
	}
	pinID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		api.WriteProblem(c, api.NotFound(err))
		return
	}

	pin, ids, hydration, err := h.pins.AssetIDsWithMeta(c.Request.Context(), int32(user.UserID), pinID)
	if err != nil {
		api.WriteProblem(c, api.NotFound(err))
		return
	}

	facetSummary, err := h.libraries.ForUser(int32(user.UserID)).BuildFacets(c.Request.Context(), &ref.Ref{
		ID:       pinID.String(),
		AssetIDs: ids,
		Scope:    ref.Scope{UserID: int32(user.UserID)},
	})
	if err != nil {
		api.WriteProblem(c, api.Internal(err))
		return
	}

	out := toAgentPinDTO(pin)
	out.Count = len(ids)
	out.Facets = dto.ToAgentRefFacetsDTO(facetSummary)
	out.HydrationSource = hydration.Source
	out.FallbackReason = hydration.FallbackReason
	out.LastSuccessfulRefreshAt = hydration.LastSuccessfulAt
	api.JSONOK(c, out)
}

// GetPinAssets hydrates one page of a pinned widget.
// @Summary Get Agent Pin Assets
// @Description Get a page of assets for a pinned widget. Frozen pins serve the stored snapshot; live pins replay their plan.
// @Tags agent
// @Produce json
// @Param id path string true "Pin ID"
// @Param limit query int false "Page size (default 50, max 200)"
// @Param offset query int false "Page offset (default 0)"
// @Success 200 {object} dto.AgentRefAssetsDTO
// @Failure 401 {object} api.ProblemResponse "Unauthorized"
// @Failure 404 {object} api.ProblemResponse "Pin not found"
// @Router /api/v1/agent/pins/{id}/assets [get]
func (h *AgentHandler) GetPinAssets(c *gin.Context) {
	user, ok := requireCurrentUser(c)
	if !ok {
		return
	}
	pinID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		api.WriteProblem(c, api.NotFound(err))
		return
	}

	_, ids, err := h.pins.AssetIDs(c.Request.Context(), int32(user.UserID), pinID)
	if err != nil {
		api.WriteProblem(c, api.NotFound(err))
		return
	}

	limit, offset := refAssetsDefaultLimit, 0
	if v, err := strconv.Atoi(c.DefaultQuery("limit", "")); err == nil && v > 0 {
		limit = min(v, refAssetsMaxLimit)
	}
	if v, err := strconv.Atoi(c.DefaultQuery("offset", "")); err == nil && v >= 0 {
		offset = v
	}

	assets := make([]dto.AssetDTO, 0, limit)
	if offset < len(ids) {
		end := min(offset+limit, len(ids))
		page := ids[offset:end]
		rows, err := h.libraries.ForUser(int32(user.UserID)).Assets(c.Request.Context(), page)
		if err != nil {
			api.WriteProblem(c, api.NotFound(err))
			return
		}
		byID := make(map[uuid.UUID]dto.AssetDTO, len(rows))
		for _, row := range rows {
			byID[row.AssetID] = dto.ToAssetDTO(row)
		}
		for _, id := range page {
			if row, found := byID[id]; found {
				assets = append(assets, row)
			}
		}
	}

	api.JSONOK(c, dto.AgentRefAssetsDTO{
		Assets:     assets,
		Total:      len(ids),
		Pagination: dto.PaginationDTO{Limit: limit, Offset: offset},
	})
}

func (h *AgentHandler) resolvePinAssetSource(c *gin.Context) (*service.AssetSetSource, bool) {
	user, ok := requireCurrentUser(c)
	if !ok {
		return nil, false
	}
	pinID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		api.WriteProblem(c, api.NotFound(err))
		return nil, false
	}

	_, ids, err := h.pins.AssetIDs(c.Request.Context(), int32(user.UserID), pinID)
	if err != nil {
		api.WriteProblem(c, api.NotFound(err))
		return nil, false
	}

	return &service.AssetSetSource{
		Kind:                  service.AssetSetSourcePin,
		AssetIDs:              ids,
		PreserveSnapshotOrder: true,
	}, true
}

// QueryPinAssets queries a pinned widget using the normal assets browse contract.
// @Summary Query Agent Pin Assets
// @Description Query a pinned widget with the same list/filter/sort semantics as the assets gallery. Snapshot-order hydration remains available through GET /agent/pins/{id}/assets.
// @Tags agent
// @Accept json
// @Produce json
// @Param id path string true "Pin ID"
// @Param data body dto.AssetQueryRequestDTO true "Query parameters"
// @Success 200 {object} dto.QueryAssetsResponseDTO "Pin assets queried successfully"
// @Failure 400 {object} api.ProblemResponse "Invalid request parameters"
// @Failure 401 {object} api.ProblemResponse "Unauthorized"
// @Failure 404 {object} api.ProblemResponse "Pin not found"
// @Failure 503 {object} api.ProblemResponse "Image Semantic Analysis unavailable"
// @Failure 500 {object} api.ProblemResponse "Internal server error"
// @Router /api/v1/agent/pins/{id}/assets/list [post]
func (h *AgentHandler) QueryPinAssets(c *gin.Context) {
	if h.assetService == nil {
		api.WriteProblem(c, api.Internal(errors.New("asset service unavailable")))
		return
	}

	var req dto.AssetQueryRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	source, ok := h.resolvePinAssetSource(c)
	if !ok {
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
	if req.SearchType == "" {
		req.SearchType = "filename"
	}

	params, err := buildQueryAssetsParams(req.Query, req.SearchType, req.SortBy, req.ViewerTimezone, req.StackMode, req.Filter, req.Pagination)
	if err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}
	params = applyAssetOwnershipScope(c, params)
	params.Source = source

	result, err := h.assetService.QueryBrowseItems(c.Request.Context(), params)
	if err != nil {
		if errors.Is(err, service.ErrInvalidBrowseFilter) {
			api.WriteProblem(c, api.BadRequest(err))
			return
		}
		if errors.Is(err, service.ErrSemanticSearchUnavailable) {
			api.WriteProblem(c, api.StatusProblem(503, err))
			return
		}
		log.Printf("Failed to query pin assets: %v", err)
		api.WriteProblem(c, api.Internal(err))
		return
	}

	api.JSONOK(c, toQueryBrowseResponseDTO(result, req.Pagination.Limit, req.Pagination.Offset))
}

// SearchPinAssets searches inside a pinned widget using the normal assets search contract.
// @Summary Search Agent Pin Assets
// @Description Search a pinned widget with optional top results enhancement and filename fallback, constrained to the pin's asset set.
// @Tags agent
// @Accept json
// @Produce json
// @Param id path string true "Pin ID"
// @Param data body dto.SearchAssetsRequestDTO true "Search parameters"
// @Success 200 {object} dto.SearchAssetsResponseDTO "Pin assets searched successfully"
// @Failure 400 {object} api.ProblemResponse "Invalid request parameters"
// @Failure 401 {object} api.ProblemResponse "Unauthorized"
// @Failure 404 {object} api.ProblemResponse "Pin not found"
// @Failure 500 {object} api.ProblemResponse "Internal server error"
// @Router /api/v1/agent/pins/{id}/assets/search [post]
func (h *AgentHandler) SearchPinAssets(c *gin.Context) {
	if h.assetService == nil {
		api.WriteProblem(c, api.Internal(errors.New("asset service unavailable")))
		return
	}

	var req dto.SearchAssetsRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	source, ok := h.resolvePinAssetSource(c)
	if !ok {
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
	if req.EnhancementMode == "" {
		req.EnhancementMode = string(service.SearchEnhancementModeAuto)
	}
	if req.SimilarToAssetID != nil && strings.TrimSpace(*req.SimilarToAssetID) != "" {
		api.WriteProblem(c, api.BadRequest(errors.New("similar_to_asset_id is not supported for pin search")))
		return
	}

	params, err := buildQueryAssetsParams(req.Query, "filename", req.SortBy, req.ViewerTimezone, "", req.Filter, req.Pagination)
	if err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}
	params = applyAssetOwnershipScope(c, params)
	params.Source = source

	result, err := h.assetService.SearchBrowseItems(c.Request.Context(), service.SearchAssetsParams{
		QueryAssetsParams: params,
		EnhancementMode:   service.SearchEnhancementMode(req.EnhancementMode),
		TopResultsLimit:   req.TopResultsLimit,
		Debug:             req.Debug,
	})
	if err != nil {
		if errors.Is(err, service.ErrInvalidBrowseFilter) {
			api.WriteProblem(c, api.BadRequest(err))
			return
		}
		log.Printf("Failed to search pin assets: %v", err)
		api.WriteProblem(c, api.Internal(err))
		return
	}

	api.JSONOK(c, toSearchBrowseResponseDTO(result, req.Pagination.Limit, req.Pagination.Offset))
}

// UpdatePinLayout persists board layout changes.
// @Summary Update Agent Pin Layout
// @Description Persist the board grid placement for one or more pins.
// @Tags agent
// @Accept json
// @Produce json
// @Param request body dto.UpdateAgentPinLayoutRequest true "Layout updates"
// @Success 200 {object} api.SuccessResponse
// @Failure 400 {object} api.ProblemResponse "Invalid request"
// @Failure 401 {object} api.ProblemResponse "Unauthorized"
// @Router /api/v1/agent/pins/layout [patch]
func (h *AgentHandler) UpdatePinLayout(c *gin.Context) {
	var req dto.UpdateAgentPinLayoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}
	user, ok := requireCurrentUser(c)
	if !ok {
		return
	}
	for _, item := range req.Layouts {
		pinID, err := uuid.Parse(item.PinID)
		if err != nil {
			continue
		}
		if err := h.pins.UpdateLayout(c.Request.Context(), int32(user.UserID), pinID,
			pins.Layout{X: item.X, Y: item.Y, W: item.W, H: item.H}); err != nil {
			api.WriteProblem(c, api.Internal(err))
			return
		}
	}
	api.JSONOK(c, api.SuccessResponse{Message: "Pin layout updated"})
}

// UpdatePin patches a single board widget: rename it and/or switch which view
// the pinned ref renders through.
// @Summary Update Agent Pin
// @Description Patch one pinned widget. Send title to rename it, widget to switch which view it renders through; both are optional.
// @Tags agent
// @Accept json
// @Produce json
// @Param id path string true "Pin ID"
// @Param request body dto.UpdateAgentPinRequest true "Pin update"
// @Success 200 {object} api.SuccessResponse
// @Failure 400 {object} api.ProblemResponse "Invalid request"
// @Failure 401 {object} api.ProblemResponse "Unauthorized"
// @Failure 404 {object} api.ProblemResponse "Pin not found"
// @Router /api/v1/agent/pins/{id} [patch]
func (h *AgentHandler) UpdatePin(c *gin.Context) {
	var req dto.UpdateAgentPinRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}
	user, ok := requireCurrentUser(c)
	if !ok {
		return
	}
	pinID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		api.WriteProblem(c, api.NotFound(err))
		return
	}
	if req.Title != nil {
		if err := h.pins.UpdateTitle(c.Request.Context(), int32(user.UserID), pinID, *req.Title); err != nil {
			api.WriteProblem(c, api.Internal(err))
			return
		}
	}
	if req.Widget != nil {
		if err := h.pins.UpdateWidget(c.Request.Context(), int32(user.UserID), pinID, *req.Widget); err != nil {
			if errors.Is(err, pins.ErrUnknownWidget) {
				api.WriteProblem(c, api.BadRequest(err))
				return
			}
			api.WriteProblem(c, api.Internal(err))
			return
		}
	}
	api.JSONOK(c, api.SuccessResponse{Message: "Pin updated"})
}

// DeletePin removes a board widget.
// @Summary Delete Agent Pin
// @Description Remove a pinned widget from the board.
// @Tags agent
// @Produce json
// @Param id path string true "Pin ID"
// @Success 200 {object} api.SuccessResponse
// @Failure 401 {object} api.ProblemResponse "Unauthorized"
// @Failure 404 {object} api.ProblemResponse "Pin not found"
// @Router /api/v1/agent/pins/{id} [delete]
func (h *AgentHandler) DeletePin(c *gin.Context) {
	user, ok := requireCurrentUser(c)
	if !ok {
		return
	}
	pinID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		api.WriteProblem(c, api.NotFound(err))
		return
	}
	if err := h.pins.Delete(c.Request.Context(), int32(user.UserID), pinID); err != nil {
		api.WriteProblem(c, api.Internal(err))
		return
	}
	api.JSONOK(c, api.SuccessResponse{Message: "Pin deleted"})
}

func toAgentPinDTO(pin repo.AgentPin) dto.AgentPinDTO {
	return dto.AgentPinDTO{
		PinID:     pin.PinID.String(),
		Title:     pin.Title,
		Widget:    pin.Widget,
		Mode:      pin.Mode,
		Count:     len(pin.AssetIds),
		Summary:   pin.Summary,
		Truncated: pin.Truncated,
		Layout: dto.AgentPinLayoutDTO{
			X: int(pin.LayoutX), Y: int(pin.LayoutY), W: int(pin.LayoutW), H: int(pin.LayoutH),
		},
		CreatedAt:               pin.CreatedAt.Time,
		LastSuccessfulRefreshAt: dbTimePointer(pin.LastSuccessfulRefreshAt),
	}
}

func dbTimePointer(value dbtypes.Timestamp) *time.Time {
	if !value.Valid {
		return nil
	}
	t := value.Time
	return &t
}
