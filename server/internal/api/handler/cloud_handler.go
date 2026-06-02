package handler

import (
	"context"
	"errors"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"server/internal/api"
	"server/internal/api/dto"
	"server/internal/cloud"
	"server/internal/db/repo"
)

// CloudHandler handles cloud credential and import endpoints.
type CloudHandler struct {
	cloudService cloud.CloudSyncService
}

// NewCloudHandler creates a CloudHandler.
func NewCloudHandler(cloudService cloud.CloudSyncService) *CloudHandler {
	return &CloudHandler{cloudService: cloudService}
}

// ListCredentials returns all saved cloud credentials.
// @Summary List cloud credentials
// @Description List configured cloud credentials without exposing secrets.
// @Tags cloud
// @Produce json
// @Security BearerAuth
// @Success 200 {object} api.Result{data=dto.ListCloudCredentialsResponse} "Credential list"
// @Failure 401 {object} api.Result "Unauthorized"
// @Failure 500 {object} api.Result "Internal server error"
// @Router /api/v1/cloud/credentials [get]
func (h *CloudHandler) ListCredentials(c *gin.Context) {
	if _, ok := requireAdminUser(c); !ok {
		return
	}

	credentials, err := h.cloudService.ListCredentials(c.Request.Context())
	if err != nil {
		api.GinInternalError(c, err, "Failed to list cloud credentials")
		return
	}

	items := make([]dto.CloudCredentialDTO, 0, len(credentials))
	for _, credential := range credentials {
		items = append(items, toCloudCredentialDTO(credential))
	}
	api.GinSuccess(c, dto.ListCloudCredentialsResponse{Credentials: items})
}

// CreateICloudCredential initiates iCloud credential creation.
// @Summary Create iCloud credential
// @Description Authenticate with iCloud and save a repo-reusable credential session. If 2FA is required, returns needs_2fa=true.
// @Tags cloud
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.CreateICloudCredentialRequest true "iCloud credential"
// @Success 200 {object} api.Result{data=dto.CreateICloudCredentialResponse} "Credential creation result"
// @Failure 400 {object} api.Result "Invalid request"
// @Failure 401 {object} api.Result "Unauthorized"
// @Failure 500 {object} api.Result "Internal server error"
// @Router /api/v1/cloud/icloud/credentials [post]
func (h *CloudHandler) CreateICloudCredential(c *gin.Context) {
	user, ok := requireAdminUser(c)
	if !ok {
		return
	}

	var req dto.CreateICloudCredentialRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.GinBadRequest(c, err, "Invalid request data")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	createdBy := int32(user.UserID)
	result, err := h.cloudService.CreateICloudCredential(ctx, cloud.CreateICloudCredentialInput{
		Username:        req.Username,
		Password:        req.Password,
		Domain:          req.Domain,
		DisplayName:     req.DisplayName,
		CreatedByUserID: &createdBy,
	})
	if err != nil {
		api.GinInternalError(c, err, "Failed to create iCloud credential")
		return
	}

	api.GinSuccess(c, dto.CreateICloudCredentialResponse{
		Credential: toCloudCredentialDTO(result.Credential),
		Needs2FA:   result.Needs2FA,
	})
}

// VerifyICloudCredential2FA submits a 2FA code for a pending iCloud credential.
// @Summary Verify iCloud credential 2FA
// @Description Submit a two-factor authentication code to complete iCloud credential creation.
// @Tags cloud
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Credential UUID"
// @Param request body dto.VerifyICloud2FARequest true "2FA code"
// @Success 200 {object} api.Result "2FA verified successfully"
// @Failure 400 {object} api.Result "Invalid request"
// @Failure 401 {object} api.Result "Unauthorized"
// @Failure 500 {object} api.Result "Internal server error"
// @Router /api/v1/cloud/icloud/credentials/{id}/verify-2fa [post]
func (h *CloudHandler) VerifyICloudCredential2FA(c *gin.Context) {
	if _, ok := requireAdminUser(c); !ok {
		return
	}

	credentialID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		api.GinBadRequest(c, err, "Invalid credential id")
		return
	}

	var req dto.VerifyICloud2FARequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.GinBadRequest(c, err, "Invalid request data")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	if err := h.cloudService.VerifyICloudCredential2FA(ctx, cloud.VerifyICloudCredential2FAInput{
		CredentialID: credentialID,
		Code:         req.Code,
	}); err != nil {
		api.GinInternalError(c, err, "2FA verification failed")
		return
	}

	api.GinSuccess(c, gin.H{"message": "iCloud credential authenticated successfully"})
}

// DisableCredential disables a saved cloud credential.
// @Summary Disable cloud credential
// @Description Disable a saved cloud credential so it cannot start new imports.
// @Tags cloud
// @Produce json
// @Security BearerAuth
// @Param id path string true "Credential UUID"
// @Success 200 {object} api.Result "Credential disabled"
// @Failure 400 {object} api.Result "Invalid request"
// @Failure 401 {object} api.Result "Unauthorized"
// @Failure 500 {object} api.Result "Internal server error"
// @Router /api/v1/cloud/credentials/{id} [delete]
func (h *CloudHandler) DisableCredential(c *gin.Context) {
	if _, ok := requireAdminUser(c); !ok {
		return
	}

	credentialID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		api.GinBadRequest(c, err, "Invalid credential id")
		return
	}

	if err := h.cloudService.DisableCredential(c.Request.Context(), credentialID); err != nil {
		api.GinInternalError(c, err, "Failed to disable credential")
		return
	}

	api.GinSuccess(c, gin.H{"message": "credential disabled"})
}

// StartRepositoryImport starts an iCloud import for a repository's binding.
// @Summary Start repository cloud import
// @Description Start an import run for the repository's configured cloud credential.
// @Tags cloud
// @Produce json
// @Security BearerAuth
// @Param id path string true "Repository UUID"
// @Success 200 {object} api.Result{data=dto.StartCloudImportResponse} "Import started"
// @Failure 400 {object} api.Result "Invalid request"
// @Failure 401 {object} api.Result "Unauthorized"
// @Failure 500 {object} api.Result "Internal server error"
// @Router /api/v1/repositories/{id}/cloud/import [post]
func (h *CloudHandler) StartRepositoryImport(c *gin.Context) {
	user, ok := requireAdminUser(c)
	if !ok {
		return
	}

	repositoryID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		api.GinBadRequest(c, err, "Invalid repository id")
		return
	}

	ownerID := int32(user.UserID)
	runID, err := h.cloudService.StartRepositoryImport(c.Request.Context(), cloud.StartRepositoryImportInput{
		RepositoryID: repositoryID,
		OwnerID:      &ownerID,
	})
	if err != nil {
		api.GinInternalError(c, err, "Failed to start cloud import")
		return
	}

	run, err := h.cloudService.GetImportRun(c.Request.Context(), runID)
	if err != nil {
		api.GinInternalError(c, err, "Failed to load cloud import run")
		return
	}
	api.GinSuccess(c, dto.StartCloudImportResponse{Run: toCloudImportRunDTO(run)})
}

// GetRepositoryCloudStatus returns cloud binding and latest import status for a repository.
// @Summary Get repository cloud status
// @Description Return cloud credential binding and latest import run for a repository.
// @Tags cloud
// @Produce json
// @Security BearerAuth
// @Param id path string true "Repository UUID"
// @Success 200 {object} api.Result{data=dto.RepositoryCloudStatusDTO} "Repository cloud status"
// @Failure 400 {object} api.Result "Invalid request"
// @Failure 401 {object} api.Result "Unauthorized"
// @Failure 500 {object} api.Result "Internal server error"
// @Router /api/v1/repositories/{id}/cloud [get]
func (h *CloudHandler) GetRepositoryCloudStatus(c *gin.Context) {
	if _, ok := requireAdminUser(c); !ok {
		return
	}

	repositoryID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		api.GinBadRequest(c, err, "Invalid repository id")
		return
	}

	status, err := h.cloudService.GetRepositoryCloudStatus(c.Request.Context(), repositoryID)
	if err != nil {
		api.GinInternalError(c, err, "Failed to load repository cloud status")
		return
	}
	api.GinSuccess(c, toRepositoryCloudStatusDTO(status))
}

// GetImportRun returns a cloud import run.
// @Summary Get cloud import run
// @Description Return a cloud import run by ID.
// @Tags cloud
// @Produce json
// @Security BearerAuth
// @Param id path string true "Import run UUID"
// @Success 200 {object} api.Result{data=dto.CloudImportRunDTO} "Import run"
// @Failure 400 {object} api.Result "Invalid request"
// @Failure 401 {object} api.Result "Unauthorized"
// @Failure 500 {object} api.Result "Internal server error"
// @Router /api/v1/cloud/import-runs/{id} [get]
func (h *CloudHandler) GetImportRun(c *gin.Context) {
	if _, ok := requireAdminUser(c); !ok {
		return
	}

	runID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		api.GinBadRequest(c, err, "Invalid import run id")
		return
	}

	run, err := h.cloudService.GetImportRun(c.Request.Context(), runID)
	if err != nil {
		api.GinInternalError(c, err, "Failed to load cloud import run")
		return
	}
	api.GinSuccess(c, toCloudImportRunDTO(run))
}

// TriggerSync is kept temporarily as an explicit deprecation response.
// @Summary Deprecated cloud sync endpoint
// @Description Deprecated. Use repo-scoped cloud import endpoints instead.
// @Tags cloud
// @Produce json
// @Security BearerAuth
// @Failure 400 {object} api.Result "Deprecated endpoint"
// @Router /api/v1/cloud/sync [post]
func (h *CloudHandler) TriggerSync(c *gin.Context) {
	api.GinBadRequest(c, errors.New("deprecated endpoint"), "Use repo-scoped cloud import endpoints")
}

func toCloudCredentialDTO(credential repo.CloudCredential) dto.CloudCredentialDTO {
	return dto.CloudCredentialDTO{
		ID:            uuid.UUID(credential.CredentialID.Bytes).String(),
		Provider:      credential.Provider,
		DisplayName:   credential.DisplayName,
		MaskedAccount: credential.MaskedAccount,
		Domain:        credential.Domain,
		Status:        credential.Status,
		CreatedAt:     pgTimeOrZero(credential.CreatedAt),
		UpdatedAt:     pgTimeOrZero(credential.UpdatedAt),
	}
}

func toCloudImportRunDTO(run repo.CloudImportRun) dto.CloudImportRunDTO {
	return dto.CloudImportRunDTO{
		ID:              uuid.UUID(run.RunID.Bytes).String(),
		RepositoryID:    uuid.UUID(run.RepositoryID.Bytes).String(),
		CredentialID:    uuid.UUID(run.CredentialID.Bytes).String(),
		Provider:        run.Provider,
		Status:          run.Status,
		TotalSeen:       run.TotalSeen,
		DownloadedCount: run.DownloadedCount,
		ImportedCount:   run.ImportedCount,
		SkippedCount:    run.SkippedCount,
		FailedCount:     run.FailedCount,
		Error:           run.Error,
		StartedAt:       pgTimePtr(run.StartedAt),
		FinishedAt:      pgTimePtr(run.FinishedAt),
		CreatedAt:       pgTimeOrZero(run.CreatedAt),
		UpdatedAt:       pgTimeOrZero(run.UpdatedAt),
	}
}

func toRepositoryCloudStatusDTO(status cloud.RepositoryCloudStatus) dto.RepositoryCloudStatusDTO {
	if status.Binding == nil {
		return dto.RepositoryCloudStatusDTO{}
	}
	result := dto.RepositoryCloudStatusDTO{
		Provider: status.Binding.Provider,
		Enabled:  status.Binding.Enabled,
	}
	if status.Binding.LastImportRunID.Valid {
		result.LastImportRun = uuid.UUID(status.Binding.LastImportRunID.Bytes).String()
	}
	if status.Credential != nil {
		credential := toCloudCredentialDTO(*status.Credential)
		result.Credential = &credential
	}
	if status.LatestRun != nil {
		run := toCloudImportRunDTO(*status.LatestRun)
		result.LatestRun = &run
	}
	return result
}

func pgTimeOrZero(value pgtype.Timestamptz) time.Time {
	if !value.Valid {
		return time.Time{}
	}
	return value.Time
}

func pgTimePtr(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	t := value.Time
	return &t
}
