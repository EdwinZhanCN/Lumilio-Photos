package handler

import (
	"errors"
	"strings"

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

func (h *AssetHandler) loadAsset(c *gin.Context, assetID uuid.UUID) (*repo.Asset, bool) {
	asset, err := h.assetService.GetAsset(c.Request.Context(), assetID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			api.GinNotFound(c, err, "Asset not found")
			return nil, false
		}
		api.GinInternalError(c, err, "Failed to access asset")
		return nil, false
	}

	return asset, true
}

func (h *AssetHandler) getAuthorizedAsset(c *gin.Context, assetID uuid.UUID, unauthorizedMessage, forbiddenMessage string) (*repo.Asset, bool) {
	asset, ok := h.loadAsset(c, assetID)
	if !ok {
		return nil, false
	}

	if !ensureOwnerAccess(c, asset.OwnerID, unauthorizedMessage, forbiddenMessage) {
		return nil, false
	}

	return asset, true
}

func (h *AssetHandler) getAuthorizedAssetForMedia(c *gin.Context, assetID uuid.UUID, unauthorizedMessage, forbiddenMessage string) (*repo.Asset, bool) {
	asset, ok := h.loadAsset(c, assetID)
	if !ok {
		return nil, false
	}

	if !h.ensureOwnerAccessForMedia(c, asset.OwnerID, unauthorizedMessage, forbiddenMessage) {
		return nil, false
	}

	return asset, true
}

func (h *AssetHandler) ensureOwnerAccessForMedia(c *gin.Context, ownerID *int32, unauthorizedMessage, forbiddenMessage string) bool {
	if ownerID == nil {
		return true
	}

	user, hasUser := currentUserFromContext(c)
	if hasUser {
		if service.IsAdminRole(user.Role) || int32(user.UserID) == *ownerID {
			return true
		}
		api.GinForbidden(c, errors.New("access denied"), forbiddenMessage)
		return false
	}

	mediaToken := strings.TrimSpace(c.Query("mt"))
	if mediaToken == "" {
		api.GinUnauthorized(c, errors.New("authentication required"), unauthorizedMessage)
		return false
	}
	if h.authService == nil {
		api.GinUnauthorized(c, errors.New("media token authentication unavailable"), unauthorizedMessage)
		return false
	}

	claims, err := h.authService.ValidateMediaToken(mediaToken)
	if err != nil {
		api.GinUnauthorized(c, errors.New("invalid or expired media token"), unauthorizedMessage)
		return false
	}

	if service.IsAdminRole(claims.Role) || int32(claims.UserID) == *ownerID {
		return true
	}

	api.GinForbidden(c, errors.New("access denied"), forbiddenMessage)
	return false
}
