package handler

import (
	"errors"
	"strconv"

	"server/internal/agent/facets"
	"server/internal/agent/pins"
	"server/internal/agent/ref"
	"server/internal/api"
	"server/internal/api/dto"
	"server/internal/db/repo"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// CreatePin pins a session ref onto the durable widget board.
// @Summary Pin an Agent Ref
// @Description Copy a session ref into a durable board widget. Live mode replays the producing plan on hydration when replayable; otherwise the pin freezes the snapshot.
// @Tags agent
// @Accept json
// @Produce json
// @Param request body dto.CreateAgentPinRequest true "Pin request"
// @Success 200 {object} dto.AgentPinDTO
// @Failure 400 {object} api.ErrorResponse "Invalid request"
// @Failure 401 {object} api.ErrorResponse "Unauthorized"
// @Failure 404 {object} api.ErrorResponse "Ref not found"
// @Router /api/v1/agent/pins [post]
func (h *AgentHandler) CreatePin(c *gin.Context) {
	var req dto.CreateAgentPinRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.GinBadRequest(c, err, "Invalid request data")
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
			api.GinNotFound(c, err, "Ref not found")
			return
		}
		api.GinInternalError(c, err, "Failed to create pin")
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
// @Failure 401 {object} api.ErrorResponse "Unauthorized"
// @Router /api/v1/agent/pins [get]
func (h *AgentHandler) ListPins(c *gin.Context) {
	user, ok := requireCurrentUser(c)
	if !ok {
		return
	}
	rows, err := h.pins.List(c.Request.Context(), int32(user.UserID))
	if err != nil {
		api.GinInternalError(c, err, "Failed to list pins")
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
// @Failure 401 {object} api.ErrorResponse "Unauthorized"
// @Failure 404 {object} api.ErrorResponse "Pin not found"
// @Router /api/v1/agent/pins/{id} [get]
func (h *AgentHandler) GetPin(c *gin.Context) {
	user, ok := requireCurrentUser(c)
	if !ok {
		return
	}
	pinID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		api.GinNotFound(c, err, "Pin not found")
		return
	}

	pin, ids, err := h.pins.AssetIDs(c.Request.Context(), int32(user.UserID), pinID)
	if err != nil {
		api.GinNotFound(c, err, "Pin not found")
		return
	}

	facetSummary, err := facets.Build(c.Request.Context(), h.queries, &ref.Ref{
		ID:       pinID.String(),
		AssetIDs: ids,
	})
	if err != nil {
		api.GinInternalError(c, err, "Failed to compute pin facets")
		return
	}

	out := toAgentPinDTO(pin)
	out.Count = len(ids)
	out.Facets = dto.ToAgentRefFacetsDTO(facetSummary)
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
// @Failure 401 {object} api.ErrorResponse "Unauthorized"
// @Failure 404 {object} api.ErrorResponse "Pin not found"
// @Router /api/v1/agent/pins/{id}/assets [get]
func (h *AgentHandler) GetPinAssets(c *gin.Context) {
	user, ok := requireCurrentUser(c)
	if !ok {
		return
	}
	pinID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		api.GinNotFound(c, err, "Pin not found")
		return
	}

	_, ids, err := h.pins.AssetIDs(c.Request.Context(), int32(user.UserID), pinID)
	if err != nil {
		api.GinNotFound(c, err, "Pin not found")
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
		pgIDs := make([]pgtype.UUID, len(page))
		for i, id := range page {
			pgIDs[i] = pgtype.UUID{Bytes: id, Valid: true}
		}
		rows, err := h.queries.GetAssetsByIDs(c.Request.Context(), pgIDs)
		if err != nil {
			api.GinInternalError(c, err, "Failed to load pin assets")
			return
		}
		byID := make(map[uuid.UUID]repo.Asset, len(rows))
		for _, row := range rows {
			byID[uuid.UUID(row.AssetID.Bytes)] = row
		}
		for _, id := range page {
			if row, found := byID[id]; found {
				assets = append(assets, dto.ToAssetDTO(row))
			}
		}
	}

	api.JSONOK(c, dto.AgentRefAssetsDTO{
		Assets:     assets,
		Total:      len(ids),
		Pagination: dto.PaginationDTO{Limit: limit, Offset: offset},
	})
}

// UpdatePinLayout persists board layout changes.
// @Summary Update Agent Pin Layout
// @Description Persist the board grid placement for one or more pins.
// @Tags agent
// @Accept json
// @Produce json
// @Param request body dto.UpdateAgentPinLayoutRequest true "Layout updates"
// @Success 200 {object} api.SuccessResponse
// @Failure 400 {object} api.ErrorResponse "Invalid request"
// @Failure 401 {object} api.ErrorResponse "Unauthorized"
// @Router /api/v1/agent/pins/layout [patch]
func (h *AgentHandler) UpdatePinLayout(c *gin.Context) {
	var req dto.UpdateAgentPinLayoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.GinBadRequest(c, err, "Invalid request data")
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
			api.GinInternalError(c, err, "Failed to update layout")
			return
		}
	}
	api.JSONOK(c, api.SuccessResponse{Message: "Pin layout updated"})
}

// DeletePin removes a board widget.
// @Summary Delete Agent Pin
// @Description Remove a pinned widget from the board.
// @Tags agent
// @Produce json
// @Param id path string true "Pin ID"
// @Success 200 {object} api.SuccessResponse
// @Failure 401 {object} api.ErrorResponse "Unauthorized"
// @Failure 404 {object} api.ErrorResponse "Pin not found"
// @Router /api/v1/agent/pins/{id} [delete]
func (h *AgentHandler) DeletePin(c *gin.Context) {
	user, ok := requireCurrentUser(c)
	if !ok {
		return
	}
	pinID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		api.GinNotFound(c, err, "Pin not found")
		return
	}
	if err := h.pins.Delete(c.Request.Context(), int32(user.UserID), pinID); err != nil {
		api.GinInternalError(c, err, "Failed to delete pin")
		return
	}
	api.JSONOK(c, api.SuccessResponse{Message: "Pin deleted"})
}

func toAgentPinDTO(pin repo.AgentPin) dto.AgentPinDTO {
	return dto.AgentPinDTO{
		PinID:     uuid.UUID(pin.PinID.Bytes).String(),
		Title:     pin.Title,
		Widget:    pin.Widget,
		Mode:      pin.Mode,
		Count:     len(pin.AssetIds),
		Summary:   pin.Summary,
		Truncated: pin.Truncated,
		Layout: dto.AgentPinLayoutDTO{
			X: int(pin.LayoutX), Y: int(pin.LayoutY), W: int(pin.LayoutW), H: int(pin.LayoutH),
		},
		CreatedAt: pin.CreatedAt.Time,
	}
}
