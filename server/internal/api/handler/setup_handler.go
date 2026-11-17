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
	// nativeHost reports a Desktop host, whose Settings choose a fresh
	// Default Storage Location; without it the operator edits the manifest.
	nativeHost bool
}

// NewSetupHandler creates a new setup handler.
func NewSetupHandler(setupService *service.SetupService, nativeHost bool) *SetupHandler {
	return &SetupHandler{setupService: setupService, nativeHost: nativeHost}
}

// GetSetupStatus reports the durable owner and primary-repository gates.
// @Summary Get system setup status
// @Description Report whether the first owner and primary repository have been created. Public and phase-only: it never carries a host path, a marker reading, or a review token; those are the authenticated setup primary target.
// @Tags setup
// @Produce json
// @Success 200 {object} dto.SetupStatusDTO "Setup status retrieved successfully"
// @Failure 500 {object} api.ProblemResponse "Internal server error"
// @Router /api/v1/setup/status [get]
func (h *SetupHandler) GetSetupStatus(c *gin.Context) {
	status, err := h.setupService.Status(c.Request.Context())
	if err != nil {
		api.WriteProblem(c, api.Internal(err))
		return
	}
	api.JSONOK(c, dto.ToSetupStatusDTO(status))
}

// GetSetupPrimaryTarget reviews the Primary Repository target for setup.
// @Summary Review the Primary Repository setup target
// @Description Read-only review for the authenticated administrator while first-run setup has not completed: the configured Default Storage Location and its root marker, and the state of its primary/ folder with the actions setup may take (create, adopt the compatible Repository already there, or start fresh elsewhere), the risks to acknowledge, the token those acknowledgements bind to, and the private state an adoption would preserve. Nothing is written. Once setup is complete only setup_complete is reported.
// @Tags setup
// @Produce json
// @Security BearerAuth
// @Success 200 {object} dto.SetupPrimaryTargetDTO
// @Failure 401 {object} api.ProblemResponse "Unauthorized"
// @Failure 403 {object} api.ProblemResponse "Forbidden"
// @Failure 500 {object} api.ProblemResponse "Internal server error"
// @Router /api/v1/setup/primary-target [get]
func (h *SetupHandler) GetSetupPrimaryTarget(c *gin.Context) {
	target, err := h.setupService.PrimaryTarget(c.Request.Context())
	if err != nil {
		api.WriteProblem(c, api.Internal(err))
		return
	}
	api.JSONOK(c, dto.ToSetupPrimaryTargetDTO(target, h.nativeHost))
}
