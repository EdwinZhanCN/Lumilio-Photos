package handler

import (
	"server/internal/api"
	"server/internal/api/dto"
	"server/internal/service"

	"github.com/gin-gonic/gin"
)

type capabilitiesHandler struct {
	settingsService service.SettingsService
	lumenService    service.LumenService
}

// NewCapabilitiesHandler creates a new read-only public capabilities handler.
func NewCapabilitiesHandler(
	settingsService service.SettingsService,
	lumenService service.LumenService,
) *capabilitiesHandler {
	return &capabilitiesHandler{
		settingsService: settingsService,
		lumenService:    lumenService,
	}
}

// GetCapabilities returns the current public runtime capabilities.
// @Summary Get public runtime capabilities
// @Description Return a de-sensitized view of backend ML and LLM runtime capabilities without exposing secrets.
// @Tags capabilities
// @Accept json
// @Produce json
// @Success 200 {object} api.Result{data=dto.CapabilitiesResponseDTO} "Capabilities retrieved successfully"
// @Router /api/v1/capabilities [get]
func (h *capabilitiesHandler) GetCapabilities(c *gin.Context) {
	systemSettings, err := h.settingsService.GetSystemSettings(c.Request.Context())
	if err != nil {
		api.GinInternalError(c, err, "Failed to load system settings")
		return
	}

	effectiveMLConfig, err := h.settingsService.GetEffectiveMLConfig(c.Request.Context())
	if err != nil {
		api.GinInternalError(c, err, "Failed to load ML settings")
		return
	}

	discoveredNodeCount := 0
	activeNodeCount := 0
	taskAvailability := map[string]bool{
		"semantic_image_embed": false,
		"semantic_text_embed":  false,
		"bioclip_classify":     false,
		"ocr":                  false,
		"face_recognition":     false,
	}

	if h.lumenService != nil {
		stats := h.lumenService.PoolStats()
		discoveredNodeCount = stats.TotalConnections
		activeNodeCount = stats.HealthyConnections

		for taskName := range taskAvailability {
			taskAvailability[taskName] = h.lumenService.IsTaskAvailable(taskName)
		}
	}

	response := dto.CapabilitiesResponseDTO{
		ML: dto.MLCapabilitiesDTO{
			DiscoveredNodeCount: discoveredNodeCount,
			ActiveNodeCount:     activeNodeCount,
			Tasks: dto.MLTaskSetDTO{
				SemanticImageEmbed: dto.MLTaskCapabilityDTO{
					Enabled:   effectiveMLConfig.SemanticEnabled,
					Available: taskAvailability["semantic_image_embed"],
				},
				SemanticTextEmbed: dto.MLTaskCapabilityDTO{
					Enabled:   effectiveMLConfig.SemanticEnabled,
					Available: taskAvailability["semantic_text_embed"],
				},
				BioClipClassify: dto.MLTaskCapabilityDTO{
					Enabled:   effectiveMLConfig.BioCLIPEnabled,
					Available: taskAvailability["bioclip_classify"],
				},
				OCR: dto.MLTaskCapabilityDTO{
					Enabled:   effectiveMLConfig.OCREnabled,
					Available: taskAvailability["ocr"],
				},
				FaceRecognition: dto.MLTaskCapabilityDTO{
					Enabled:   effectiveMLConfig.FaceEnabled,
					Available: taskAvailability["face_recognition"],
				},
			},
		},
		LLM: dto.LLMCapabilitiesDTO{
			AgentEnabled: systemSettings.LLM.AgentEnabled,
			Configured:   systemSettings.LLM.IsConfigured(),
			Provider:     systemSettings.LLM.Provider,
			ModelName:    systemSettings.LLM.ModelName,
		},
	}

	api.GinSuccess(c, response)
}
