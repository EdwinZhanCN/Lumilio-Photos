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

	"github.com/edwinzhancn/lumen-sdk/pkg/client"
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
	getAvailableModelsFn func(ctx context.Context) ([]*client.NodeInfo, error)
	isTaskAvailableFn    func(taskName string) bool
}

func (s stubLumenService) GetAvailableModels(ctx context.Context) ([]*client.NodeInfo, error) {
	return s.getAvailableModelsFn(ctx)
}

func (s stubLumenService) IsTaskAvailable(taskName string) bool {
	return s.isTaskAvailableFn(taskName)
}

func TestCapabilitiesHandlerGetCapabilities_IncludesClipTextEmbed(t *testing.T) {
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
						AutoMode:    config.MLAutoModeEnable,
						CLIPEnabled: true,
					},
				}, nil
			},
			getEffectiveMLFn: func(ctx context.Context) (config.MLConfig, error) {
				return config.MLConfig{
					AutoMode:    config.MLAutoModeEnable,
					CLIPEnabled: true,
					OCREnabled:  true,
				}, nil
			},
		},
		stubLumenService{
			getAvailableModelsFn: func(ctx context.Context) ([]*client.NodeInfo, error) {
				return []*client.NodeInfo{}, nil
			},
			isTaskAvailableFn: func(taskName string) bool {
				return taskName == "clip_image_embed" || taskName == "clip_text_embed"
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
	require.True(t, response.Data.ML.Tasks.ClipImageEmbed.Enabled)
	require.True(t, response.Data.ML.Tasks.ClipImageEmbed.Available)
	require.True(t, response.Data.ML.Tasks.ClipTextEmbed.Enabled)
	require.True(t, response.Data.ML.Tasks.ClipTextEmbed.Available)
}
