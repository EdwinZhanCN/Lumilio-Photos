package handler

import (
	"sort"
	"strings"
	"time"

	"server/internal/api"
	"server/internal/api/dto"
	"server/internal/service"

	"github.com/edwinzhancn/lumen-sdk/pkg/discovery"
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
// @Success 200 {object} dto.CapabilitiesResponseDTO "Capabilities retrieved successfully"
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

	runtimeSnapshot := service.NewDisabledLumenService().RuntimeSnapshot()
	taskAvailability := map[string]bool{
		"semantic_image_embed": false,
		"semantic_text_embed":  false,
		"bioclip_classify":     false,
		"ocr":                  false,
		"face_recognition":     false,
	}

	if h.lumenService != nil {
		runtimeSnapshot = h.lumenService.RuntimeSnapshot()

		for taskName := range taskAvailability {
			taskAvailability[taskName] = h.lumenService.IsTaskAvailable(taskName)
		}
	}

	llmConfigured := systemSettings.LLM.IsConfigured()
	llmAvailability := "ready"
	if !systemSettings.LLM.AgentEnabled {
		llmAvailability = "disabled"
	} else if !llmConfigured {
		llmAvailability = "not_configured"
	}

	response := dto.CapabilitiesResponseDTO{
		ML: dto.MLCapabilitiesDTO{
			DiscoveryState:        string(runtimeSnapshot.DiscoveryState),
			DiscoveredNodeCount:   runtimeSnapshot.Counts.DiscoveredNodes,
			ActiveNodeCount:       runtimeSnapshot.Counts.ActiveNodes,
			ConnectingNodeCount:   runtimeSnapshot.Counts.ConnectingNodes,
			UnavailableNodeCount:  runtimeSnapshot.Counts.UnavailableNodes,
			PendingNodeCount:      runtimeSnapshot.Counts.PendingNodes,
			IncompatibleNodeCount: runtimeSnapshot.Counts.IncompatibleNodes,
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
			Availability: llmAvailability,
			AgentEnabled: systemSettings.LLM.AgentEnabled,
			Configured:   llmConfigured,
			Provider:     systemSettings.LLM.Provider,
			ModelName:    systemSettings.LLM.ModelName,
		},
	}

	api.JSONOK(c, response)
}

// GetLumenRuntime returns the authenticated administrator projection of Lumen
// discovery, transport, compatibility, and task state.
// @Summary Get Lumen runtime diagnostics
// @Description Return bounded per-backend and per-node Lumen runtime diagnostics for administrators.
// @Tags capabilities
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} dto.LumenRuntimeDTO "Lumen runtime diagnostics retrieved successfully"
// @Router /api/v1/admin/lumen/runtime [get]
func (h *capabilitiesHandler) GetLumenRuntime(c *gin.Context) {
	snapshot := service.NewDisabledLumenService().RuntimeSnapshot()
	if h.lumenService != nil {
		snapshot = h.lumenService.RuntimeSnapshot()
	}
	api.JSONOK(c, lumenRuntimeDTO(snapshot))
}

func lumenRuntimeDTO(snapshot service.LumenRuntimeSnapshot) dto.LumenRuntimeDTO {
	result := dto.LumenRuntimeDTO{
		CapturedAt:     snapshot.CapturedAt,
		DiscoveryState: string(snapshot.DiscoveryState),
		Counts: dto.LumenRuntimeCountsDTO{
			Discovered:   snapshot.Counts.DiscoveredNodes,
			Active:       snapshot.Counts.ActiveNodes,
			Connecting:   snapshot.Counts.ConnectingNodes,
			Unavailable:  snapshot.Counts.UnavailableNodes,
			Pending:      snapshot.Counts.PendingNodes,
			Incompatible: snapshot.Counts.IncompatibleNodes,
		},
		Backends: make([]dto.LumenBackendStatusDTO, 0, len(snapshot.Backends)),
		Nodes:    make([]dto.LumenNodeRuntimeDTO, 0, len(snapshot.Nodes)),
	}
	for _, backend := range snapshot.Backends {
		result.Backends = append(result.Backends, dto.LumenBackendStatusDTO{
			Source:              backend.Source,
			State:               string(backend.State),
			LastScanStartedAt:   optionalTime(backend.LastScanStartedAt),
			LastScanCompletedAt: optionalTime(backend.LastScanCompletedAt),
			LastScanSucceededAt: optionalTime(backend.LastScanSucceededAt),
			NextScanAt:          optionalTime(backend.NextScanAt),
			ConsecutiveFailures: backend.ConsecutiveFailures,
			LastErrorCode:       backend.LastErrorCode,
			MatchedCount:        backend.MatchedCount,
			RejectedCount:       backend.RejectedCount,
			LastOutcome:         string(backend.LastOutcome),
		})
	}
	for _, node := range snapshot.Nodes {
		if node == nil {
			continue
		}
		tasks := make([]dto.LumenNodeTaskDTO, 0)
		seen := make(map[string]struct{})
		for _, capability := range node.Capabilities {
			if capability == nil {
				continue
			}
			for _, task := range capability.GetTasks() {
				if task == nil || strings.TrimSpace(task.GetName()) == "" {
					continue
				}
				key := capability.GetServiceName() + "\x00" + task.GetName()
				if _, ok := seen[key]; ok {
					continue
				}
				seen[key] = struct{}{}
				tasks = append(tasks, dto.LumenNodeTaskDTO{Service: capability.GetServiceName(), Task: task.GetName()})
			}
		}
		sort.Slice(tasks, func(i, j int) bool {
			if tasks[i].Service == tasks[j].Service {
				return tasks[i].Task < tasks[j].Task
			}
			return tasks[i].Service < tasks[j].Service
		})
		result.Nodes = append(result.Nodes, dto.LumenNodeRuntimeDTO{
			ID:             node.ID,
			Endpoint:       node.Address,
			Sources:        append([]string(nil), node.Sources...),
			LastObservedAt: optionalTime(node.LastObserved),
			UpdatedAt:      optionalTime(node.UpdatedAt),
			Transport:      publicTransportState(node.Availability),
			Compatibility:  string(node.Compatibility),
			Version:        node.Version,
			Runtime:        node.Runtime,
			ErrorCode:      publicNodeErrorCode(node),
			Tasks:          tasks,
		})
	}
	sort.Slice(result.Backends, func(i, j int) bool { return result.Backends[i].Source < result.Backends[j].Source })
	sort.Slice(result.Nodes, func(i, j int) bool { return result.Nodes[i].ID < result.Nodes[j].ID })
	return result
}

func publicTransportState(availability discovery.NodeAvailability) string {
	switch availability {
	case discovery.NodeAvailabilityReady:
		return "ready"
	case discovery.NodeAvailabilityUnavailable:
		return "unavailable"
	default:
		return "connecting"
	}
}

func publicNodeErrorCode(node *discovery.NodeInfo) string {
	if node == nil {
		return ""
	}
	if node.Availability == discovery.NodeAvailabilityUnavailable {
		return "transport_unavailable"
	}
	if node.Compatibility != discovery.CompatibilityIncompatible {
		return ""
	}
	reason := strings.ToLower(node.IncompatibleReason)
	switch {
	case strings.Contains(reason, "not implemented"):
		return "capability_rpc_unimplemented"
	case strings.Contains(reason, "protocol"):
		return "protocol_incompatible"
	default:
		return "incompatible"
	}
}

func optionalTime(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	copyValue := value.UTC()
	return &copyValue
}
