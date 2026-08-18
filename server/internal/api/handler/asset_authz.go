package handler

import (
	"database/sql"
	"errors"
	"strings"

	"server/internal/api"
	"server/internal/db/repo"
	"server/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func applyAssetOwnershipScope(c *gin.Context, params service.QueryAssetsParams) service.QueryAssetsParams {
	user, ok := currentUserFromContext(c)
	if !ok {
		return params
	}
	// Generic asset browsing is intentionally global for administrators, but
	// Event membership is an owner-scoped topology. Event routes resolve the
	// current user's Event identity, so an Event-filtered asset query must carry
	// that same owner instead of treating nil as an all-owner scope.
	if service.IsAdminRole(user.Role) && (params.EventID == nil || strings.TrimSpace(*params.EventID) == "") {
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

func applyLocationClusterOwnershipScope(c *gin.Context, params service.ListLocationClustersParams) service.ListLocationClustersParams {
	user, ok := currentUserFromContext(c)
	if !ok || service.IsAdminRole(user.Role) {
		return params
	}

	ownerID := int32(user.UserID)
	params.OwnerID = &ownerID
	return params
}

// ownerScopeID returns the current non-admin user's ID for scoping
// owner-filtered read endpoints, or nil for admins/unauthenticated callers
// (no scope applied).
func ownerScopeID(c *gin.Context) *int32 {
	user, ok := currentUserFromContext(c)
	if !ok || service.IsAdminRole(user.Role) {
		return nil
	}
	ownerID := int32(user.UserID)
	return &ownerID
}

func (h *AssetHandler) loadAsset(c *gin.Context, assetID uuid.UUID) (*repo.Asset, bool) {
	asset, err := h.assetService.GetAsset(c.Request.Context(), assetID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			api.WriteProblem(c, api.NotFound(err))
			return nil, false
		}
		api.WriteProblem(c, api.Internal(err))
		return nil, false
	}

	return asset, true
}

func (h *AssetHandler) loadAssetAny(c *gin.Context, assetID uuid.UUID) (*repo.Asset, bool) {
	asset, err := h.assetService.GetAssetAny(c.Request.Context(), assetID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			api.WriteProblem(c, api.NotFound(err))
			return nil, false
		}
		api.WriteProblem(c, api.Internal(err))
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

func (h *AssetHandler) getAuthorizedAssetAny(c *gin.Context, assetID uuid.UUID, unauthorizedMessage, forbiddenMessage string) (*repo.Asset, bool) {
	asset, ok := h.loadAssetAny(c, assetID)
	if !ok {
		return nil, false
	}

	if !ensureOwnerAccess(c, asset.OwnerID, unauthorizedMessage, forbiddenMessage) {
		return nil, false
	}

	return asset, true
}

func (h *AssetHandler) getAuthorizedAssetForMedia(c *gin.Context, assetID uuid.UUID, unauthorizedMessage, forbiddenMessage string) (*repo.Asset, bool) {
	asset, ok := h.loadAssetAny(c, assetID)
	if !ok {
		return nil, false
	}

	if !h.ensureOwnerAccessForMedia(c, asset.OwnerID, unauthorizedMessage, forbiddenMessage) {
		return nil, false
	}

	return asset, true
}

func (h *AssetHandler) getAuthorizedAssetForRead(c *gin.Context, assetID uuid.UUID, unauthorizedMessage, forbiddenMessage string) (*repo.Asset, bool) {
	return h.getAuthorizedAssetAny(c, assetID, unauthorizedMessage, forbiddenMessage)
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
		api.WriteProblem(c, api.Forbidden(errors.New("access denied")))
		return false
	}

	mediaToken := strings.TrimSpace(c.Query("mt"))
	if mediaToken == "" {
		api.WriteProblem(c, api.Unauthorized(errors.New("authentication required")))
		return false
	}
	if h.authService == nil {
		api.WriteProblem(c, api.Unauthorized(errors.New("media token authentication unavailable")))
		return false
	}

	claims, err := h.authService.ValidateMediaToken(c.Request.Context(), mediaToken)
	if err != nil {
		api.WriteProblem(c, api.Unauthorized(errors.New("invalid or expired media token")))
		return false
	}

	if service.IsAdminRole(claims.Role) || int32(claims.UserID) == *ownerID {
		return true
	}
	api.WriteProblem(c, api.Forbidden(errors.New("access denied")))
	return false
}
