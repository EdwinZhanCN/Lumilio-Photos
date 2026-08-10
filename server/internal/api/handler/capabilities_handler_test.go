package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"server/internal/api/dto"
	"server/internal/service"
	"server/internal/settings"

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
	isTaskAvailFn func(string) bool
}

func (s stubLumenService) PoolStats() service.PoolStats {
	return s.poolStats
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
