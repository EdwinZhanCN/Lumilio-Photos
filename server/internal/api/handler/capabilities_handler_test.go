package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"server/config"
	"server/internal/api/dto"
	"server/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type stubSettingsService struct {
	service.SettingsService
	getSystemSettingsFn func(ctx context.Context) (service.SystemSettings, error)
	getEffectiveMLFn    func(ctx context.Context) (config.MLConfig, error)
}

func (s stubSettingsService) GetSystemSettings(ctx context.Context) (service.SystemSettings, error) {
	return s.getSystemSettingsFn(ctx)
}

func (s stubSettingsService) GetEffectiveMLConfig(ctx context.Context) (config.MLConfig, error) {
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

func TestCapabilitiesHandlerGetCapabilities_IncludesClipCapabilities(t *testing.T) {
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
						CLIPEnabled: true,
					},
				}, nil
			},
			getEffectiveMLFn: func(ctx context.Context) (config.MLConfig, error) {
				return config.MLConfig{
					CLIPEnabled:    true,
					BioCLIPEnabled: true,
					OCREnabled:     true,
				}, nil
			},
		},
		stubLumenService{
			poolStats: service.PoolStats{
				TotalConnections:   1,
				HealthyConnections: 1,
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

	var response struct {
		Code int                         `json:"code"`
		Data dto.CapabilitiesResponseDTO `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, 0, response.Code)
	require.True(t, response.Data.ML.Tasks.SemanticImageEmbed.Enabled)
	require.True(t, response.Data.ML.Tasks.SemanticImageEmbed.Available)
	require.True(t, response.Data.ML.Tasks.SemanticTextEmbed.Enabled)
	require.True(t, response.Data.ML.Tasks.SemanticTextEmbed.Available)
	require.True(t, response.Data.ML.Tasks.BioClipClassify.Enabled)
	require.True(t, response.Data.ML.Tasks.BioClipClassify.Available)
}
