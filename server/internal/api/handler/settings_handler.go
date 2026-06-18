package handler

import (
	"server/internal/api"
	"server/internal/api/dto"
	"server/internal/service"

	"github.com/gin-gonic/gin"
)

type SettingsHandler struct {
	settingsService service.SettingsService
	runtimeInfo     dto.RuntimeInfoDTO
}

func NewSettingsHandler(settingsService service.SettingsService, runtimeInfo dto.RuntimeInfoDTO) *SettingsHandler {
	return &SettingsHandler{settingsService: settingsService, runtimeInfo: runtimeInfo}
}

// GetRuntimeInfo returns a read-only snapshot of the runtime-immutable
// configuration (port, storage root, hardware accel, scan schedule, …) for the
// Settings → Server tab.
// @Summary Get runtime info
// @Description Read-only effective runtime-immutable configuration (changed only via TOML + restart).
// @Tags settings
// @Produce json
// @Security BearerAuth
// @Success 200 {object} dto.RuntimeInfoDTO "Runtime info retrieved successfully"
// @Failure 401 {object} api.ErrorResponse "Unauthorized"
// @Router /api/v1/settings/runtime-info [get]
func (h *SettingsHandler) GetRuntimeInfo(c *gin.Context) {
	api.JSONOK(c, h.runtimeInfo)
}

// GetSystemSettings returns the persisted system settings.
// @Summary Get system settings
// @Description Return persisted system settings without exposing secret values.
// @Tags settings
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} dto.SystemSettingsDTO "System settings retrieved successfully"
// @Failure 401 {object} api.ErrorResponse "Unauthorized"
// @Failure 500 {object} api.ErrorResponse "Internal server error"
// @Router /api/v1/settings/system [get]
func (h *SettingsHandler) GetSystemSettings(c *gin.Context) {
	settings, err := h.settingsService.GetSystemSettings(c.Request.Context())
	if err != nil {
		api.GinInternalError(c, err, "Failed to load system settings")
		return
	}

	api.JSONOK(c, dto.ToSystemSettingsDTO(settings))
}

// UpdateSystemSettings updates persisted system settings.
// @Summary Update system settings
// @Description Update persisted system settings. API keys are write-only and never returned.
// @Tags settings
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.UpdateSystemSettingsDTO true "System settings patch"
// @Success 200 {object} dto.SystemSettingsDTO "System settings updated successfully"
// @Failure 400 {object} api.ErrorResponse "Invalid request data"
// @Failure 401 {object} api.ErrorResponse "Unauthorized"
// @Failure 500 {object} api.ErrorResponse "Internal server error"
// @Router /api/v1/settings/system [patch]
func (h *SettingsHandler) UpdateSystemSettings(c *gin.Context) {
	var req dto.UpdateSystemSettingsDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		api.GinBadRequest(c, err, "Invalid request data")
		return
	}

	userID, err := currentUserIDFromContext(c)
	if err != nil {
		api.GinUnauthorized(c, err, "Unauthorized")
		return
	}

	input, err := req.ToServiceInput(userID)
	if err != nil {
		api.GinBadRequest(c, err, "Invalid request data")
		return
	}

	settings, err := h.settingsService.UpdateSystemSettings(c.Request.Context(), input)
	if err != nil {
		api.GinInternalError(c, err, "Failed to update system settings")
		return
	}

	api.JSONOK(c, dto.ToSystemSettingsDTO(settings))
}

// ValidateLLMSettings validates the persisted LLM configuration against the current provider.
// @Summary Validate LLM settings
// @Description Validate the persisted LLM configuration by issuing a lightweight test request.
// @Tags settings
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} dto.ValidateLLMSettingsResponseDTO "LLM settings validated successfully"
// @Failure 400 {object} api.ErrorResponse "LLM validation failed"
// @Failure 401 {object} api.ErrorResponse "Unauthorized"
// @Failure 500 {object} api.ErrorResponse "Internal server error"
// @Router /api/v1/settings/system/validate-llm [post]
func (h *SettingsHandler) ValidateLLMSettings(c *gin.Context) {
	if _, err := currentUserIDFromContext(c); err != nil {
		api.GinUnauthorized(c, err, "Unauthorized")
		return
	}

	if err := h.settingsService.ValidateLLMSettings(c.Request.Context()); err != nil {
		api.GinBadRequest(c, err, "LLM validation failed")
		return
	}

	api.JSONOK(c, dto.ValidateLLMSettingsResponseDTO{Valid: true})
}
