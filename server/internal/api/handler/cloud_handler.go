package handler

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"server/internal/api"
	"server/internal/api/dto"
	"server/internal/cloud"
)

// CloudHandler handles cloud sync API endpoints.
type CloudHandler struct {
	cloudService cloud.CloudSyncService
}

// NewCloudHandler creates a CloudHandler.
func NewCloudHandler(cloudService cloud.CloudSyncService) *CloudHandler {
	return &CloudHandler{cloudService: cloudService}
}

// ConnectICloud initiates an iCloud connection.
// @Summary Connect to iCloud
// @Description Start iCloud authentication. If 2FA is required, returns needs_2fa=true.
// @Tags cloud
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.ConnectICloudRequest true "iCloud credentials"
// @Success 200 {object} api.Result{data=dto.ConnectICloudResponse} "Connection result"
// @Failure 400 {object} api.Result "Invalid request"
// @Failure 401 {object} api.Result "Unauthorized"
// @Failure 500 {object} api.Result "Internal server error"
// @Router /api/v1/cloud/icloud/connect [post]
func (h *CloudHandler) ConnectICloud(c *gin.Context) {
	if _, ok := requireAdminUser(c); !ok {
		return
	}

	var req dto.ConnectICloudRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.GinBadRequest(c, err, "Invalid request data")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	needs2FA, err := h.cloudService.ConnectICloud(ctx, cloud.ConnectICloudInput{
		Username: req.Username,
		Password: req.Password,
		Domain:   req.Domain,
		SyncMode: req.SyncMode,
	})
	if err != nil {
		api.GinInternalError(c, err, "Failed to connect to iCloud")
		return
	}

	api.GinSuccess(c, dto.ConnectICloudResponse{Needs2FA: needs2FA})
}

// VerifyICloud2FA submits a 2FA code to complete iCloud authentication.
// @Summary Verify iCloud 2FA
// @Description Submit a two-factor authentication code to complete iCloud login.
// @Tags cloud
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.VerifyICloud2FARequest true "2FA code"
// @Success 200 {object} api.Result "2FA verified successfully"
// @Failure 400 {object} api.Result "Invalid request"
// @Failure 401 {object} api.Result "Unauthorized"
// @Failure 500 {object} api.Result "Internal server error"
// @Router /api/v1/cloud/icloud/verify-2fa [post]
func (h *CloudHandler) VerifyICloud2FA(c *gin.Context) {
	if _, ok := requireAdminUser(c); !ok {
		return
	}

	var req dto.VerifyICloud2FARequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.GinBadRequest(c, err, "Invalid request data")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	if err := h.cloudService.VerifyICloud2FA(ctx, cloud.VerifyICloud2FAInput{Code: req.Code}); err != nil {
		api.GinInternalError(c, err, "2FA verification failed")
		return
	}

	api.GinSuccess(c, gin.H{"message": "iCloud authenticated successfully"})
}

// ListProviders returns the status of all configured cloud providers.
// @Summary List cloud providers
// @Description Get the status of all configured cloud storage providers.
// @Tags cloud
// @Produce json
// @Security BearerAuth
// @Success 200 {object} api.Result{data=dto.ListProvidersResponse} "Provider list"
// @Failure 401 {object} api.Result "Unauthorized"
// @Failure 500 {object} api.Result "Internal server error"
// @Router /api/v1/cloud/providers [get]
func (h *CloudHandler) ListProviders(c *gin.Context) {
	if _, ok := requireAdminUser(c); !ok {
		return
	}

	ctx := c.Request.Context()

	statuses, err := h.cloudService.ListProviders(ctx)
	if err != nil {
		api.GinInternalError(c, err, "Failed to list cloud providers")
		return
	}

	var dtos []dto.CloudProviderStatusDTO
	for _, s := range statuses {
		dtos = append(dtos, dto.CloudProviderStatusDTO{
			Provider:        string(s.Provider),
			SyncMode:        string(s.SyncMode),
			Connected:       s.Connected,
			LastCursor:      s.LastCursor,
			SyncedFileCount: s.SyncedFileCount,
		})
	}

	api.GinSuccess(c, dto.ListProvidersResponse{Providers: dtos})
}

// TriggerSync starts a cloud sync operation.
// @Summary Trigger cloud sync
// @Description Start a sync operation for the specified cloud provider.
// @Tags cloud
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.TriggerSyncRequest true "Sync configuration"
// @Success 200 {object} api.Result "Sync started"
// @Failure 400 {object} api.Result "Invalid request"
// @Failure 401 {object} api.Result "Unauthorized"
// @Failure 500 {object} api.Result "Internal server error"
// @Router /api/v1/cloud/sync [post]
func (h *CloudHandler) TriggerSync(c *gin.Context) {
	if _, ok := requireAdminUser(c); !ok {
		return
	}

	var req dto.TriggerSyncRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.GinBadRequest(c, err, "Invalid request data")
		return
	}

	repoID, err := uuid.Parse(req.RepositoryID)
	if err != nil {
		api.GinBadRequest(c, err, "Invalid repository_id")
		return
	}

	user, ok := requireCurrentUser(c)
	if !ok {
		return
	}
	ownerID := int32(user.UserID)

	ctx := c.Request.Context()

	if err := h.cloudService.TriggerSync(ctx, cloud.TriggerSyncInput{
		Provider:     req.Provider,
		RepositoryID: repoID,
		OwnerID:      &ownerID,
	}); err != nil {
		api.GinInternalError(c, err, "Failed to start sync")
		return
	}

	api.GinSuccess(c, gin.H{"message": "sync started"})
}

// Disconnect removes a cloud provider's configuration.
// @Summary Disconnect cloud provider
// @Description Remove a cloud provider's configuration and stop any active sync.
// @Tags cloud
// @Produce json
// @Security BearerAuth
// @Param provider path string true "Provider name" Enums(icloud, s3)
// @Success 200 {object} api.Result "Disconnected successfully"
// @Failure 400 {object} api.Result "Invalid provider"
// @Failure 401 {object} api.Result "Unauthorized"
// @Failure 500 {object} api.Result "Internal server error"
// @Router /api/v1/cloud/{provider} [delete]
func (h *CloudHandler) Disconnect(c *gin.Context) {
	if _, ok := requireAdminUser(c); !ok {
		return
	}

	providerStr := c.Param("provider")
	provider := cloud.ProviderKind(providerStr)

	ctx := c.Request.Context()

	if err := h.cloudService.Disconnect(ctx, provider, uuid.Nil); err != nil {
		api.GinInternalError(c, err, "Failed to disconnect provider")
		return
	}

	api.GinSuccess(c, gin.H{"message": "provider disconnected"})
}
