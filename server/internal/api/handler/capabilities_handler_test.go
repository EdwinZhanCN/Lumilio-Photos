package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"server/internal/api/dto"
	"server/internal/service"
	"server/internal/settings"

	"github.com/edwinzhancn/lumen-sdk/pkg/discovery"
	pb "github.com/edwinzhancn/lumen-sdk/proto"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type stubSettingsService struct {
	service.SettingsService
	getSystemSettingsFn func(ctx context.Context) (service.SystemSettings, error)
	getEffectiveMLFn    func(ctx context.Context) (settings.ML, error)
}

func (s stubSettingsService) GetSystemSettings(ctx context.Context) (service.SystemSettings, error) {
	return s.getSystemSettingsFn(ctx)
}

func (s stubSettingsService) GetEffectiveMLConfig(ctx context.Context) (settings.ML, error) {
	return s.getEffectiveMLFn(ctx)
}

type stubLumenService struct {
	service.LumenService
	poolStats     service.PoolStats
	runtime       service.LumenRuntimeSnapshot
	isTaskAvailFn func(string) bool
}

func (s stubLumenService) RuntimeSnapshot() service.LumenRuntimeSnapshot {
	if s.runtime.CapturedAt.IsZero() {
		s.runtime.CapturedAt = time.Now().UTC()
		s.runtime.DiscoveryState = discovery.BackendHealthy
		s.runtime.Counts.DiscoveredNodes = s.poolStats.TotalNodes
		s.runtime.Counts.ActiveNodes = s.poolStats.RoutableNodes
	}
	return s.runtime
}

func (s stubLumenService) PoolStats() service.PoolStats {
	return s.poolStats
}

func TestCapabilitiesHandlerUsesRuntimeDimensionsForAggregateCounts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	runtime := service.LumenRuntimeSnapshot{
		CapturedAt:     time.Now().UTC(),
		DiscoveryState: discovery.BackendDegraded,
	}
	runtime.Counts.DiscoveredNodes = 4
	runtime.Counts.ActiveNodes = 1
	runtime.Counts.ConnectingNodes = 1
	runtime.Counts.UnavailableNodes = 1
	runtime.Counts.PendingNodes = 2
	runtime.Counts.IncompatibleNodes = 1
	handler := NewCapabilitiesHandler(
		stubSettingsService{
			getSystemSettingsFn: func(context.Context) (service.SystemSettings, error) {
				return service.SystemSettings{}, nil
			},
			getEffectiveMLFn: func(context.Context) (settings.ML, error) { return settings.ML{}, nil },
		},
		stubLumenService{
			poolStats: service.PoolStats{TotalNodes: 99, RoutableNodes: 98},
			runtime:   runtime,
		},
	)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/capabilities", nil)
	handler.GetCapabilities(ctx)

	var response dto.CapabilitiesResponseDTO
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, "degraded", response.ML.DiscoveryState)
	require.Equal(t, 4, response.ML.DiscoveredNodeCount)
	require.Equal(t, 1, response.ML.ActiveNodeCount)
	require.Equal(t, 2, response.ML.PendingNodeCount)
	require.Equal(t, 1, response.ML.IncompatibleNodeCount)
}

func TestLumenRuntimeProjectionIsTypedAndDropsRawDiagnostics(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Now().UTC()
	runtime := service.LumenRuntimeSnapshot{
		CapturedAt:     now,
		DiscoveryState: discovery.BackendHealthy,
		Backends: []discovery.ResolverStatus{{
			Source: "mdns", State: discovery.BackendHealthy, LastScanCompletedAt: now,
			MatchedCount: 1, RejectedCount: 3,
		}},
		Nodes: []*discovery.NodeInfo{{
			ID: "lab-node-1", Address: "192.168.1.20:5866", Sources: []string{"mdns"},
			LastObserved: now, UpdatedAt: now, Availability: discovery.NodeAvailabilityReady,
			Compatibility:      discovery.CompatibilityIncompatible,
			IncompatibleReason: "raw protocol parser detail that must not cross the API",
			Metadata:           map[string]interface{}{"raw_txt": "secret-ish"},
			Capabilities: []*pb.Capability{{
				ServiceName: "siglip", Tasks: []*pb.IOTask{{Name: "semantic_image_embed"}},
			}},
		}},
	}
	runtime.Counts.DiscoveredNodes = 1
	runtime.Counts.IncompatibleNodes = 1
	handler := NewCapabilitiesHandler(nil, stubLumenService{runtime: runtime})
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/lumen/runtime", nil)
	handler.GetLumenRuntime(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response dto.LumenRuntimeDTO
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Len(t, response.Nodes, 1)
	require.Equal(t, "protocol_incompatible", response.Nodes[0].ErrorCode)
	require.Equal(t, "semantic_image_embed", response.Nodes[0].Tasks[0].Task)
	require.NotContains(t, recorder.Body.String(), "raw protocol parser detail")
	require.NotContains(t, recorder.Body.String(), "raw_txt")
}

func (s stubLumenService) IsTaskAvailable(taskName string) bool {
	if s.isTaskAvailFn != nil {
		return s.isTaskAvailFn(taskName)
	}
	return false
}

func TestCapabilitiesHandlerGetCapabilities_IncludesSemanticCapabilities(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := NewCapabilitiesHandler(
		stubSettingsService{
			getSystemSettingsFn: func(ctx context.Context) (service.SystemSettings, error) {
				return service.SystemSettings{
					LLM: service.LLMSettings{
						AgentEnabled:     true,
						Provider:         "openai",
						ModelName:        "gpt-4.1-mini",
						APIKeyConfigured: true,
					},
					ML: service.MLSettings{
						SemanticEnabled: true,
					},
				}, nil
			},
			getEffectiveMLFn: func(ctx context.Context) (settings.ML, error) {
				return settings.ML{
					SemanticEnabled: true,
					BioCLIPEnabled:  true,
					OCREnabled:      true,
				}, nil
			},
		},
		stubLumenService{
			poolStats: service.PoolStats{
				TotalNodes:    1,
				RoutableNodes: 1,
			},
			isTaskAvailFn: func(taskName string) bool {
				return taskName == "semantic_image_embed" ||
					taskName == "semantic_text_embed" ||
					taskName == "bioclip_classify"
			},
		},
	)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/capabilities", nil)

	handler.GetCapabilities(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)

	var response dto.CapabilitiesResponseDTO
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.ML.Tasks.SemanticImageEmbed.Enabled)
	require.True(t, response.ML.Tasks.SemanticImageEmbed.Available)
	require.True(t, response.ML.Tasks.SemanticTextEmbed.Enabled)
	require.True(t, response.ML.Tasks.SemanticTextEmbed.Available)
	require.True(t, response.ML.Tasks.BioClipClassify.Enabled)
	require.True(t, response.ML.Tasks.BioClipClassify.Available)
	require.Equal(t, "ready", response.LLM.Availability)
}

func TestCapabilitiesHandlerReportsTruthfulAgentAvailability(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name string
		llm  service.LLMSettings
		want string
	}{
		{name: "disabled", llm: service.LLMSettings{}, want: "disabled"},
		{
			name: "enabled but incomplete",
			llm:  service.LLMSettings{AgentEnabled: true, Provider: "openai", ModelName: "gpt-4.1-mini"},
			want: "not_configured",
		},
		{
			name: "ready",
			llm: service.LLMSettings{
				AgentEnabled: true, Provider: "openai", ModelName: "gpt-4.1-mini", APIKeyConfigured: true,
			},
			want: "ready",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewCapabilitiesHandler(
				stubSettingsService{
					getSystemSettingsFn: func(context.Context) (service.SystemSettings, error) {
						return service.SystemSettings{LLM: tt.llm}, nil
					},
					getEffectiveMLFn: func(context.Context) (settings.ML, error) {
						return settings.ML{}, nil
					},
				},
				nil,
			)
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/capabilities", nil)
			handler.GetCapabilities(ctx)

			require.Equal(t, http.StatusOK, recorder.Code)
			var response dto.CapabilitiesResponseDTO
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
			require.Equal(t, tt.want, response.LLM.Availability)
			require.Equal(t, tt.llm.AgentEnabled, response.LLM.AgentEnabled)
		})
	}
}
