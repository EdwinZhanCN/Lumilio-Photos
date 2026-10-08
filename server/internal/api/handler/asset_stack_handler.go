package handler

import (
	"errors"
	"net/http"
	"server/internal/api"
	"server/internal/api/dto"
	"server/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ============================================================================
// Stack operations
// ============================================================================

// GetAssetMediaItem returns the logical media item containing an asset.
// @Summary Get logical media item
// @Description Returns the logical media item and its RAW/JPEG, Live Photo, or edited components
// @Tags assets
// @Produce json
// @Param id path string true "Asset ID"
// @Success 200 {object} dto.MediaItemByAssetResponseDTO
// @Failure 404 {object} api.ProblemResponse
// @Router /api/v1/assets/{id}/media-item [get]
// @Security BearerAuth
func (h *AssetHandler) GetAssetMediaItem(c *gin.Context) {
	assetID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}
	if _, ok := h.getAuthorizedAssetForRead(c, assetID, "Authentication required to access this asset", "You don't have permission to access this asset"); !ok {
		return
	}
	item, err := h.stackService.GetMediaItemByAsset(c.Request.Context(), assetID, ownerScopeID(c))
	if err != nil {
		api.WriteProblem(c, api.NotFound(err))
		return
	}
	components := make([]dto.MediaItemComponentDTO, 0, len(item.Components))
	for _, component := range item.Components {
		components = append(components, dto.MediaItemComponentDTO{
			AssetID: component.AssetID.String(), Relation: string(component.Relation), Position: component.Position,
		})
	}
	api.JSONOK(c, dto.MediaItemByAssetResponseDTO{
		AssetID: assetID.String(),
		MediaItem: dto.MediaItemDTO{
			MediaItemID: item.MediaItemID.String(), MediaKind: item.Kind,
			PrimaryAssetID: item.PrimaryAssetID.String(), Components: components,
		},
	})
}

// GetAssetStack returns the stack that contains the given asset.
// @Summary Get asset stack
// @Description Returns the stack (group) that contains the specified asset
// @Tags assets
// @Produce json
// @Param id path string true "Asset ID"
// @Success 200 {object} dto.StackByAssetResponseDTO
// @Failure 404 {object} api.ProblemResponse
// @Router /api/v1/assets/{id}/stack [get]
// @Security BearerAuth
func (h *AssetHandler) GetAssetStack(c *gin.Context) {
	assetIDStr := c.Param("id")
	assetID, err := uuid.Parse(assetIDStr)
	if err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	if _, ok := h.getAuthorizedAssetForRead(c, assetID, "Authentication required to access this asset", "You don't have permission to access this asset"); !ok {
		return
	}

	stackInfo, err := h.stackService.GetStackByAssetAny(c.Request.Context(), assetID, ownerScopeID(c))
	if err != nil {
		if errors.Is(err, service.ErrStackNotFound) {
			api.WriteProblem(c, api.NotFound(err))
			return
		}
		api.WriteProblem(c, api.Internal(err))
		return
	}

	// Convert to DTO
	members := make([]dto.StackMemberDTO, len(stackInfo.Members))
	for i, m := range stackInfo.Members {
		members[i] = dto.StackMemberDTO{
			MediaItemID:    m.MediaItemID.String(),
			PrimaryAssetID: m.AssetID.String(),
			Position:       m.Position,
		}
	}

	response := dto.StackByAssetResponseDTO{
		AssetID: assetID.String(),
		Stack: dto.StackDTO{
			StackID:     stackInfo.StackID.String(),
			StackKind:   string(stackInfo.Kind),
			MemberCount: stackInfo.MemberCount,
			Members:     members,
		},
	}

	api.JSONOK(c, response)
}

// CreateManualStack manually groups assets into a stack.
// @Summary Create manual stack
// @Description Manually groups the specified assets into a new stack
// @Tags assets
// @Produce json
// @Param data body dto.CreateManualStackRequestDTO true "Asset IDs to stack"
// @Success 201 {object} dto.StackDTO
// @Failure 400 {object} api.ProblemResponse
// @Failure 409 {object} api.ProblemResponse
// @Router /api/v1/assets/stacks [post]
// @Security BearerAuth
func (h *AssetHandler) CreateManualStack(c *gin.Context) {
	var req dto.CreateManualStackRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	if len(req.AssetIDs) < 2 {
		api.WriteProblem(c, api.BadRequest(errors.New("at least 2 asset IDs are required")))
		return
	}

	assetIDs := make([]uuid.UUID, len(req.AssetIDs))
	for i, idStr := range req.AssetIDs {
		id, err := uuid.Parse(idStr)
		if err != nil {
			api.WriteProblem(c, api.BadRequest(err))
			return
		}
		assetIDs[i] = id
	}

	// Every asset in the stack must belong to the caller (or caller is admin).
	for _, id := range assetIDs {
		if _, ok := h.getAuthorizedAsset(c, id, "Authentication required to stack these assets", "You don't have permission to stack one or more of these assets"); !ok {
			return
		}
	}

	stackInfo, err := h.stackService.CreateManualStack(c.Request.Context(), assetIDs)
	if err != nil {
		if errors.Is(err, service.ErrAssetAlreadyStacked) {
			api.WriteProblem(c, api.StatusProblem(http.StatusConflict, err))
			return
		}
		api.WriteProblem(c, api.Internal(err))
		return
	}

	members := make([]dto.StackMemberDTO, len(stackInfo.Members))
	for i, m := range stackInfo.Members {
		members[i] = dto.StackMemberDTO{
			MediaItemID:    m.MediaItemID.String(),
			PrimaryAssetID: m.AssetID.String(),
			Position:       m.Position,
		}
	}

	response := dto.StackDTO{
		StackID:     stackInfo.StackID.String(),
		StackKind:   string(stackInfo.Kind),
		MemberCount: stackInfo.MemberCount,
		Members:     members,
	}

	c.JSON(http.StatusCreated, response)
}

// UnstackAsset removes an asset from its stack.
// @Summary Remove asset from stack
// @Description Removes an asset from its stack, making it standalone
// @Tags assets
// @Produce json
// @Param id path string true "Asset ID"
// @Success 200 {object} api.SuccessResponse
// @Router /api/v1/assets/{id}/stack [delete]
// @Security BearerAuth
func (h *AssetHandler) UnstackAsset(c *gin.Context) {
	assetIDStr := c.Param("id")
	assetID, err := uuid.Parse(assetIDStr)
	if err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	if _, ok := h.getAuthorizedAsset(c, assetID, "Authentication required to modify this asset", "You don't have permission to modify this asset"); !ok {
		return
	}

	if err := h.stackService.RemoveFromStack(c.Request.Context(), assetID); err != nil {
		api.WriteProblem(c, api.Internal(err))
		return
	}

	api.JSONOK(c, api.SuccessResponse{Message: "Asset removed from stack"})
}

// AutoDetectStacks merges structural media components and detects burst stacks for a repository.
// @Summary Auto-detect stacks
// @Description Merges RAW/JPEG and Live Photo components into logical media items, then detects burst presentation stacks
// @Tags repositories
// @Produce json
// @Param id path string true "Repository ID"
// @Success 200 {object} dto.AutoDetectStacksResponseDTO
// @Router /api/v1/repositories/{id}/stacks/detect [post]
// @Security BearerAuth
func (h *AssetHandler) AutoDetectStacks(c *gin.Context) {
	repoIDStr := c.Param("id")
	repoID, err := uuid.Parse(repoIDStr)
	if err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	count, err := h.stackService.AutoDetectStacks(c.Request.Context(), repoID)
	if err != nil {
		api.WriteProblem(c, api.Internal(err))
		return
	}

	api.JSONOK(c, dto.AutoDetectStacksResponseDTO{
		RepositoryID:  repoID.String(),
		StacksCreated: count,
	})
}
