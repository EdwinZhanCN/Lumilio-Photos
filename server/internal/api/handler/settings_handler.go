package handler

import (
	"errors"
	"fmt"

	"server/internal/api"
	"server/internal/api/dto"
	"server/internal/service"

	"github.com/gin-gonic/gin"
)

type SettingsHandler struct {
	settingsService service.SettingsService
}

func NewSettingsHandler(settingsService service.SettingsService) *SettingsHandler {
	return &SettingsHandler{settingsService: settingsService}
}

// GetSystemSettings returns the persisted system settings.
// @Summary Get system settings
// @Description Return persisted system settings without exposing secret values.
// @Tags settings
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} api.Result{data=dto.SystemSettingsDTO} "System settings retrieved successfully"
// @Failure 401 {object} api.Result "Unauthorized"
// @Failure 500 {object} api.Result "Internal server error"
// @Router /api/v1/settings/system [get]
func (h *SettingsHandler) GetSystemSettings(c *gin.Context) {
	settings, err := h.settingsService.GetSystemSettings(c.Request.Context())
	if err != nil {
		api.GinInternalError(c, err, "Failed to load system settings")
		return
	}

	api.GinSuccess(c, dto.ToSystemSettingsDTO(settings))
}

// UpdateSystemSettings updates persisted system settings.
// @Summary Update system settings
// @Description Update persisted system settings. API keys are write-only and never returned.
// @Tags settings
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.UpdateSystemSettingsDTO true "System settings patch"
// @Success 200 {object} api.Result{data=dto.SystemSettingsDTO} "System settings updated successfully"
// @Failure 400 {object} api.Result "Invalid request data"
// @Failure 401 {object} api.Result "Unauthorized"
// @Failure 500 {object} api.Result "Internal server error"
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

	api.GinSuccess(c, dto.ToSystemSettingsDTO(settings))
}

// ValidateLLMSettings validates the persisted LLM configuration against the current provider.
// @Summary Validate LLM settings
// @Description Validate the persisted LLM configuration by issuing a lightweight test request.
// @Tags settings
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} api.Result{data=dto.ValidateLLMSettingsResponseDTO} "LLM settings validated successfully"
// @Failure 400 {object} api.Result "LLM validation failed"
// @Failure 401 {object} api.Result "Unauthorized"
// @Failure 500 {object} api.Result "Internal server error"
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

	api.GinSuccess(c, dto.ValidateLLMSettingsResponseDTO{Valid: true})
}

func currentUserIDFromContext(c *gin.Context) (*int32, error) {
	value, exists := c.Get("user_id")
	if !exists {
		return nil, errors.New("user id not found in context")
	}

	switch userID := value.(type) {
	case int:
		converted := int32(userID)
		return &converted, nil
	case int32:
		converted := userID
		return &converted, nil
	case int64:
		converted := int32(userID)
		return &converted, nil
	default:
		return nil, fmt.Errorf("unexpected user id type %T", value)
	}
}
