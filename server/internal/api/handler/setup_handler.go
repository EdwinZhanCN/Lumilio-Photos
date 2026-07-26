package handler

import (
	"server/internal/api"
	"server/internal/api/dto"
	"server/internal/service"

	"github.com/gin-gonic/gin"
)

// SetupHandler exposes the zero-config first-run bootstrapping endpoints.
type SetupHandler struct {
	setupService *service.SetupService
}

// NewSetupHandler creates a new setup handler.
func NewSetupHandler(setupService *service.SetupService) *SetupHandler {
	return &SetupHandler{setupService: setupService}
}

// GetSetupStatus reports the durable owner and primary-repository gates.
// @Summary Get system setup status
// @Description Report whether the first owner and primary repository have been created.
// @Tags setup
// @Produce json
// @Success 200 {object} dto.SetupStatusDTO "Setup status retrieved successfully"
// @Failure 500 {object} api.ErrorResponse "Internal server error"
// @Router /api/v1/setup/status [get]
func (h *SetupHandler) GetSetupStatus(c *gin.Context) {
	status, err := h.setupService.Status(c.Request.Context())
	if err != nil {
		api.GinInternalError(c, err, "Failed to load setup status")
		return
	}
	api.JSONOK(c, dto.ToSetupStatusDTO(status))
}
