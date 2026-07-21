package handler

import (
	"context"
	"encoding/json"
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

// ListProviders returns available cloud providers.
// @Summary List cloud providers
// @Description List cloud provider descriptors for credential creation.
// @Tags cloud
// @Produce json
// @Security BearerAuth
// @Success 200 {object} dto.ListCloudProvidersResponse "Provider list"
// @Failure 401 {object} api.ErrorResponse "Unauthorized"
// @Failure 500 {object} api.ErrorResponse "Internal server error"
// @Router /api/v1/cloud/providers [get]
func (h *CloudHandler) ListProviders(c *gin.Context) {
	if _, ok := requireAdminUser(c); !ok {
		return
	}

	providers, err := h.cloudService.ListProviders(c.Request.Context())
	if err != nil {
		api.GinInternalError(c, err, "Failed to list cloud providers")
		return
	}

	items := make([]dto.CloudProviderDTO, 0, len(providers))
	for _, provider := range providers {
		items = append(items, toCloudProviderDTO(provider))
	}
	api.JSONOK(c, dto.ListCloudProvidersResponse{Providers: items})
}

// ListCredentials returns all saved cloud credentials.
// @Summary List cloud credentials
// @Description List configured cloud credentials without exposing secrets.
// @Tags cloud
// @Produce json
// @Security BearerAuth
// @Success 200 {object} dto.ListCloudCredentialsResponse "Credential list"
// @Failure 401 {object} api.ErrorResponse "Unauthorized"
// @Failure 500 {object} api.ErrorResponse "Internal server error"
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
		items = append(items, toCloudCredentialDTO(credential, h.cloudService.ProviderTitle(cloud.ProviderKind(credential.Provider))))
	}
	api.JSONOK(c, dto.ListCloudCredentialsResponse{Credentials: items})
}

// CreateCredential initiates cloud credential creation.
// @Summary Create cloud credential
// @Description Authenticate with a cloud provider and save a repo-reusable credential. Provider-specific challenges return auth_status=challenge_required.
// @Tags cloud
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.CreateCloudCredentialRequest true "Cloud credential"
// @Success 200 {object} dto.CreateCloudCredentialResponse "Credential creation result"
// @Failure 400 {object} api.ErrorResponse "Invalid request"
// @Failure 401 {object} api.ErrorResponse "Unauthorized"
// @Failure 500 {object} api.ErrorResponse "Internal server error"
// @Router /api/v1/cloud/credentials [post]
func (h *CloudHandler) CreateCredential(c *gin.Context) {
	user, ok := requireAdminUser(c)
	if !ok {
		return
	}

	var req dto.CreateCloudCredentialRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.GinBadRequest(c, err, "Invalid request data")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	createdBy := int32(user.UserID)
	result, err := h.cloudService.CreateCredential(ctx, cloud.CreateCloudCredentialInput{
		Provider:        cloud.ProviderKind(req.Provider),
		DisplayName:     req.DisplayName,
		Inputs:          req.Inputs,
		CreatedByUserID: &createdBy,
	})
	if err != nil {
		api.GinInternalError(c, err, "Failed to create cloud credential")
		return
	}

	api.JSONOK(c, dto.CreateCloudCredentialResponse{
		Credential: toCloudCredentialDTO(result.Credential, h.cloudService.ProviderTitle(cloud.ProviderKind(result.Credential.Provider))),
		AuthStatus: result.AuthStatus,
		Challenge:  toCloudAuthChallengeDTO(result.Challenge),
	})
}

// VerifyCredentialAuthChallenge submits provider-specific challenge inputs.
// @Summary Verify cloud credential challenge
// @Description Submit challenge inputs to complete cloud credential creation.
// @Tags cloud
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Credential UUID"
// @Param request body dto.VerifyCloudAuthChallengeRequest true "Challenge inputs"
// @Success 200 {object} dto.VerifyCloudAuthChallengeResponse "Challenge verified successfully"
// @Failure 400 {object} api.ErrorResponse "Invalid request"
// @Failure 401 {object} api.ErrorResponse "Unauthorized"
// @Failure 500 {object} api.ErrorResponse "Internal server error"
// @Router /api/v1/cloud/credentials/{id}/auth-challenge [post]
func (h *CloudHandler) VerifyCredentialAuthChallenge(c *gin.Context) {
	if _, ok := requireAdminUser(c); !ok {
		return
	}

	credentialID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		api.GinBadRequest(c, err, "Invalid credential id")
		return
	}

	var req dto.VerifyCloudAuthChallengeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.GinBadRequest(c, err, "Invalid request data")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	result, err := h.cloudService.VerifyCredentialChallenge(ctx, cloud.VerifyCredentialChallengeInput{
		CredentialID: credentialID,
		Inputs:       req.Inputs,
	})
	if err != nil {
		api.GinInternalError(c, err, "Cloud credential challenge verification failed")
		return
	}

	api.JSONOK(c, dto.VerifyCloudAuthChallengeResponse{
		Credential: toCloudCredentialDTO(result.Credential, h.cloudService.ProviderTitle(cloud.ProviderKind(result.Credential.Provider))),
		AuthStatus: result.AuthStatus,
	})
}

// DisconnectCredential disconnects a cloud credential without deleting it.
// @Summary Disconnect cloud credential
// @Description Pause a cloud credential so it cannot start new imports. Can be reconnected later.
// @Tags cloud
// @Produce json
// @Security BearerAuth
// @Param id path string true "Credential UUID"
// @Success 200 {object} api.SuccessResponse "Credential disconnected"
// @Failure 400 {object} api.ErrorResponse "Invalid request"
// @Failure 401 {object} api.ErrorResponse "Unauthorized"
// @Failure 500 {object} api.ErrorResponse "Internal server error"
// @Router /api/v1/cloud/credentials/{id}/disconnect [post]
func (h *CloudHandler) DisconnectCredential(c *gin.Context) {
	if _, ok := requireAdminUser(c); !ok {
		return
	}

	credentialID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		api.GinBadRequest(c, err, "Invalid credential id")
		return
	}

	if err := h.cloudService.DisconnectCredential(c.Request.Context(), credentialID); err != nil {
		api.GinInternalError(c, err, "Failed to disconnect credential")
		return
	}

	api.JSONOK(c, api.SuccessResponse{Message: "credential disconnected"})
}

// ReconnectCredential re-authenticates a disconnected or errored credential.
// @Summary Reconnect cloud credential
// @Description Re-authenticate a disconnected or errored credential. If no password is provided, attempts to reuse the existing session.
// @Tags cloud
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Credential UUID"
// @Param request body dto.ReconnectCloudCredentialRequest true "Reconnect inputs"
// @Success 200 {object} dto.CreateCloudCredentialResponse "Reconnect result"
// @Failure 400 {object} api.ErrorResponse "Invalid request"
// @Failure 401 {object} api.ErrorResponse "Unauthorized"
// @Failure 500 {object} api.ErrorResponse "Internal server error"
// @Router /api/v1/cloud/credentials/{id}/reconnect [post]
func (h *CloudHandler) ReconnectCredential(c *gin.Context) {
	if _, ok := requireAdminUser(c); !ok {
		return
	}

	credentialID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		api.GinBadRequest(c, err, "Invalid credential id")
		return
	}

	var req dto.ReconnectCloudCredentialRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.GinBadRequest(c, err, "Invalid request data")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	result, err := h.cloudService.ReconnectCredential(ctx, cloud.ReconnectCredentialInput{
		CredentialID: credentialID,
		Inputs:       req.Inputs,
	})
	if err != nil {
		api.GinInternalError(c, err, "Failed to reconnect credential")
		return
	}

	api.JSONOK(c, dto.CreateCloudCredentialResponse{
		Credential: toCloudCredentialDTO(result.Credential, h.cloudService.ProviderTitle(cloud.ProviderKind(result.Credential.Provider))),
		AuthStatus: result.AuthStatus,
		Challenge:  toCloudAuthChallengeDTO(result.Challenge),
	})
}

// RemoveCredential permanently deletes a cloud credential.
// @Summary Remove cloud credential
// @Description Permanently delete a cloud credential, its session data, and unbind associated repositories.
// @Tags cloud
// @Produce json
// @Security BearerAuth
// @Param id path string true "Credential UUID"
// @Success 200 {object} api.SuccessResponse "Credential removed"
// @Failure 400 {object} api.ErrorResponse "Invalid request"
// @Failure 401 {object} api.ErrorResponse "Unauthorized"
// @Failure 500 {object} api.ErrorResponse "Internal server error"
// @Router /api/v1/cloud/credentials/{id} [delete]
func (h *CloudHandler) RemoveCredential(c *gin.Context) {
	if _, ok := requireAdminUser(c); !ok {
		return
	}

	credentialID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		api.GinBadRequest(c, err, "Invalid credential id")
		return
	}

	if err := h.cloudService.RemoveCredential(c.Request.Context(), credentialID); err != nil {
		api.GinInternalError(c, err, "Failed to remove credential")
		return
	}

	api.JSONOK(c, api.SuccessResponse{Message: "credential removed"})
}

// StartRepositoryImport starts a cloud import for a repository's binding.
// @Summary Start repository cloud import
// @Description Start an import run for the repository's configured cloud credential.
// @Tags cloud
// @Produce json
// @Security BearerAuth
// @Param id path string true "Repository UUID"
// @Success 200 {object} dto.StartCloudImportResponse "Import started"
// @Failure 400 {object} api.ErrorResponse "Invalid request"
// @Failure 401 {object} api.ErrorResponse "Unauthorized"
// @Failure 500 {object} api.ErrorResponse "Internal server error"
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
	api.JSONOK(c, dto.StartCloudImportResponse{Run: toCloudImportRunDTO(run)})
}

// GetRepositoryCloudStatus returns cloud binding and latest import status for a repository.
// @Summary Get repository cloud status
// @Description Return cloud credential binding and latest import run for a repository.
// @Tags cloud
// @Produce json
// @Security BearerAuth
// @Param id path string true "Repository UUID"
// @Success 200 {object} dto.RepositoryCloudStatusDTO "Repository cloud status"
// @Failure 400 {object} api.ErrorResponse "Invalid request"
// @Failure 401 {object} api.ErrorResponse "Unauthorized"
// @Failure 500 {object} api.ErrorResponse "Internal server error"
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
	api.JSONOK(c, toRepositoryCloudStatusDTO(status, h.cloudService))
}

// GetImportRun returns a cloud import run.
// @Summary Get cloud import run
// @Description Return a cloud import run by ID.
// @Tags cloud
// @Produce json
// @Security BearerAuth
// @Param id path string true "Import run UUID"
// @Success 200 {object} dto.CloudImportRunDTO "Import run"
// @Failure 400 {object} api.ErrorResponse "Invalid request"
// @Failure 401 {object} api.ErrorResponse "Unauthorized"
// @Failure 500 {object} api.ErrorResponse "Internal server error"
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
	api.JSONOK(c, toCloudImportRunDTO(run))
}

// TriggerSync is kept temporarily as an explicit deprecation response.
// @Summary Deprecated cloud sync endpoint
// @Description Deprecated. Use repo-scoped cloud import endpoints instead.
// @Tags cloud
// @Produce json
// @Security BearerAuth
// @Failure 400 {object} api.ErrorResponse "Deprecated endpoint"
// @Router /api/v1/cloud/sync [post]
func (h *CloudHandler) TriggerSync(c *gin.Context) {
	api.GinBadRequest(c, errors.New("deprecated endpoint"), "Use repo-scoped cloud import endpoints")
}

func toCloudProviderDTO(provider cloud.ProviderDescriptor) dto.CloudProviderDTO {
	return dto.CloudProviderDTO{
		ID:              string(provider.ID),
		Title:           provider.Title,
		Description:     provider.Description,
		Status:          provider.Status,
		FormFields:      toCloudProviderFieldDTOs(provider.FormFields),
		ChallengeFields: toCloudProviderFieldDTOs(provider.ChallengeFields),
		SecurityNote:    provider.SecurityNote,
	}
}

func toCloudProviderFieldDTOs(fields []cloud.ProviderField) []dto.CloudProviderFieldDTO {
	items := make([]dto.CloudProviderFieldDTO, 0, len(fields))
	for _, field := range fields {
		options := make([]dto.Option, 0, len(field.Options))
		for _, option := range field.Options {
			options = append(options, dto.Option{Value: option.Value, Label: option.Label})
		}
		items = append(items, dto.CloudProviderFieldDTO{
			Name:         field.Name,
			Label:        field.Label,
			Type:         field.Type,
			Required:     field.Required,
			Placeholder:  field.Placeholder,
			HelpText:     field.HelpText,
			Options:      options,
			Autocomplete: field.Autocomplete,
		})
	}
	return items
}

func toCloudAuthChallengeDTO(challenge *cloud.AuthChallenge) *dto.CloudAuthChallengeDTO {
	if challenge == nil {
		return nil
	}
	return &dto.CloudAuthChallengeDTO{
		Type:        challenge.Type,
		Title:       challenge.Title,
		Description: challenge.Description,
		Params:      challenge.Params,
		Fields:      toCloudProviderFieldDTOs(challenge.Fields),
	}
}

func toCloudCredentialDTO(credential repo.CloudCredential, providerTitle string) dto.CloudCredentialDTO {
	return dto.CloudCredentialDTO{
		ID:             uuid.UUID(credential.CredentialID.Bytes).String(),
		Provider:       credential.Provider,
		ProviderTitle:  providerTitle,
		DisplayName:    credential.DisplayName,
		MaskedIdentity: credential.MaskedIdentity,
		Status:         credential.Status,
		PublicConfig:   publicConfigMap(credential.PublicConfig),
		CreatedAt:      pgTimeOrZero(credential.CreatedAt),
		UpdatedAt:      pgTimeOrZero(credential.UpdatedAt),
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

func toRepositoryCloudStatusDTO(status cloud.RepositoryCloudStatus, cloudService cloud.CloudSyncService) dto.RepositoryCloudStatusDTO {
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
		credential := toCloudCredentialDTO(*status.Credential, cloudService.ProviderTitle(cloud.ProviderKind(status.Credential.Provider)))
		result.Credential = &credential
	}
	if status.LatestRun != nil {
		run := toCloudImportRunDTO(*status.LatestRun)
		result.LatestRun = &run
	}
	return result
}

func publicConfigMap(data []byte) map[string]string {
	if len(data) == 0 {
		return nil
	}
	var config map[string]string
	if err := json.Unmarshal(data, &config); err != nil || len(config) == 0 {
		return nil
	}
	return config
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
