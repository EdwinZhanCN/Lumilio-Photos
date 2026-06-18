package handler

import (
	"errors"
	"io"
	"net/http"

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

// GetSetupStatus reports whether the rotated database password exists on disk.
// @Summary Get system setup status
// @Description Report whether Lumilio has rotated the temporary database credential. The web frontend runs setup as a preflight while uninitialized.
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

// Setup performs first-run initialization: it mints a high-entropy database
// password, rotates the database credential away from the temporary bootstrap
// password, and persists the secret with locked-down permissions.
// @Summary Initialize the system
// @Description Run first-run bootstrapping: generate and rotate the database credential, then persist the secret. Refused once the system is already initialized.
// @Tags setup
// @Accept json
// @Produce json
// @Param request body dto.SetupRequestDTO false "Optional empty setup payload"
// @Success 200 {object} dto.SetupResultDTO "System initialized successfully"
// @Failure 400 {object} api.ErrorResponse "Invalid request data"
// @Failure 409 {object} api.ErrorResponse "System already initialized"
// @Failure 500 {object} api.ErrorResponse "Internal server error"
// @Router /api/v1/setup [post]
func (h *SetupHandler) Setup(c *gin.Context) {
	var req dto.SetupRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		api.GinBadRequest(c, err, "Invalid request data")
		return
	}

	result, err := h.setupService.Initialize(c.Request.Context(), service.SetupRequest{})
	if err != nil {
		if errors.Is(err, service.ErrSystemAlreadyInitialized) {
			api.GinError(c, http.StatusConflict, err, http.StatusConflict, "System already initialized")
			return
		}
		api.GinInternalError(c, err, "Failed to initialize system")
		return
	}

	api.JSONOK(c, dto.ToSetupResultDTO(result))
}
