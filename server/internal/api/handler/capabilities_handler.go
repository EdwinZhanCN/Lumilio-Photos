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
		"clip_image_embed":      false,
		"clip_text_embed":       false,
		"ocr":                   false,
		"vlm_generate":          false,
		"face_detect_and_embed": false,
	}

	if h.lumenService != nil {
		nodes, err := h.lumenService.GetAvailableModels(c.Request.Context())
		if err == nil {
			discoveredNodeCount = len(nodes)
			for _, node := range nodes {
				if node != nil && node.IsActive() {
					activeNodeCount++
				}
			}
		}

		for taskName := range taskAvailability {
			taskAvailability[taskName] = h.lumenService.IsTaskAvailable(taskName)
		}
	}

	response := dto.CapabilitiesResponseDTO{
		ML: dto.MLCapabilitiesDTO{
			AutoMode:            string(systemSettings.ML.AutoMode),
			DiscoveredNodeCount: discoveredNodeCount,
			ActiveNodeCount:     activeNodeCount,
			Tasks: dto.MLTaskSetDTO{
				ClipImageEmbed: dto.MLTaskCapabilityDTO{
					Enabled:   effectiveMLConfig.CLIPEnabled,
					Available: taskAvailability["clip_image_embed"],
				},
				ClipTextEmbed: dto.MLTaskCapabilityDTO{
					Enabled:   effectiveMLConfig.CLIPEnabled,
					Available: taskAvailability["clip_text_embed"],
				},
				OCR: dto.MLTaskCapabilityDTO{
					Enabled:   effectiveMLConfig.OCREnabled,
					Available: taskAvailability["ocr"],
				},
				VLMGenerate: dto.MLTaskCapabilityDTO{
					Enabled:   effectiveMLConfig.CaptionEnabled,
					Available: taskAvailability["vlm_generate"],
				},
				FaceDetectAndEmbed: dto.MLTaskCapabilityDTO{
					Enabled:   effectiveMLConfig.FaceEnabled,
					Available: taskAvailability["face_detect_and_embed"],
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
