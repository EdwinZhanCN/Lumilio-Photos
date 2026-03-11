package handler

import (
	"errors"

	"server/internal/api"
	"server/internal/db/repo"
	"server/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func applyAssetOwnershipScope(c *gin.Context, params service.QueryAssetsParams) service.QueryAssetsParams {
	user, ok := currentUserFromContext(c)
	if !ok || service.IsAdminRole(user.Role) {
		return params
	}

	ownerID := int32(user.UserID)
	params.OwnerID = &ownerID
	return params
}

func applyMapPointOwnershipScope(c *gin.Context, params service.QueryPhotoMapPointsParams) service.QueryPhotoMapPointsParams {
	user, ok := currentUserFromContext(c)
	if !ok || service.IsAdminRole(user.Role) {
		return params
	}

	ownerID := int32(user.UserID)
	params.OwnerID = &ownerID
	return params
}

func (h *AssetHandler) getAuthorizedAsset(c *gin.Context, assetID uuid.UUID, unauthorizedMessage, forbiddenMessage string) (*repo.Asset, bool) {
	asset, err := h.assetService.GetAsset(c.Request.Context(), assetID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			api.GinNotFound(c, err, "Asset not found")
			return nil, false
		}
		api.GinInternalError(c, err, "Failed to access asset")
		return nil, false
	}

	if !ensureOwnerAccess(c, asset.OwnerID, unauthorizedMessage, forbiddenMessage) {
		return nil, false
	}

	return asset, true
}
