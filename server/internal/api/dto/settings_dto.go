package dto

import (
	"time"

	"server/config"
	"server/internal/service"
)

type SystemSettingsDTO struct {
	LLM       LLMSettingsDTO `json:"llm"`
	ML        MLSettingsDTO  `json:"ml"`
	UpdatedAt time.Time      `json:"updated_at"`
	UpdatedBy *int32         `json:"updated_by,omitempty"`
}

type LLMSettingsDTO struct {
	AgentEnabled     bool   `json:"agent_enabled"`
	Provider         string `json:"provider" example:"openai"`
	ModelName        string `json:"model_name" example:"gpt-4.1-mini"`
	BaseURL          string `json:"base_url,omitempty" example:"https://api.openai.com/v1"`
	APIKeyConfigured bool   `json:"api_key_configured"`
}

type MLSettingsDTO struct {
	AutoMode       string `json:"auto_mode" example:"disable"`
	CLIPEnabled    bool   `json:"clip_enabled"`
	OCREnabled     bool   `json:"ocr_enabled"`
	CaptionEnabled bool   `json:"caption_enabled"`
	FaceEnabled    bool   `json:"face_enabled"`
}

type UpdateSystemSettingsDTO struct {
	LLM *UpdateLLMSettingsDTO `json:"llm,omitempty"`
	ML  *UpdateMLSettingsDTO  `json:"ml,omitempty"`
}

type UpdateLLMSettingsDTO struct {
	AgentEnabled *bool   `json:"agent_enabled,omitempty"`
	Provider     *string `json:"provider,omitempty" binding:"omitempty,oneof=ark openai deepseek ollama"`
	ModelName    *string `json:"model_name,omitempty"`
	BaseURL      *string `json:"base_url,omitempty"`
	APIKey       *string `json:"api_key,omitempty"`
}

type UpdateMLSettingsDTO struct {
	AutoMode       *string `json:"auto_mode,omitempty" binding:"omitempty,oneof=enable disable"`
	CLIPEnabled    *bool   `json:"clip_enabled,omitempty"`
	OCREnabled     *bool   `json:"ocr_enabled,omitempty"`
	CaptionEnabled *bool   `json:"caption_enabled,omitempty"`
	FaceEnabled    *bool   `json:"face_enabled,omitempty"`
}

type ValidateLLMSettingsResponseDTO struct {
	Valid bool `json:"valid"`
}

func ToSystemSettingsDTO(settings service.SystemSettings) SystemSettingsDTO {
	return SystemSettingsDTO{
		LLM: LLMSettingsDTO{
			AgentEnabled:     settings.LLM.AgentEnabled,
			Provider:         settings.LLM.Provider,
			ModelName:        settings.LLM.ModelName,
			BaseURL:          settings.LLM.BaseURL,
			APIKeyConfigured: settings.LLM.APIKeyConfigured,
		},
		ML: MLSettingsDTO{
			AutoMode:       string(settings.ML.AutoMode),
			CLIPEnabled:    settings.ML.CLIPEnabled,
			OCREnabled:     settings.ML.OCREnabled,
			CaptionEnabled: settings.ML.CaptionEnabled,
			FaceEnabled:    settings.ML.FaceEnabled,
		},
		UpdatedAt: settings.UpdatedAt,
		UpdatedBy: settings.UpdatedBy,
	}
}

func (dto UpdateSystemSettingsDTO) ToServiceInput(updatedBy *int32) (service.UpdateSystemSettingsInput, error) {
	input := service.UpdateSystemSettingsInput{
		UpdatedBy: updatedBy,
	}

	if dto.LLM != nil {
		input.LLM = &service.UpdateLLMSettingsInput{
			AgentEnabled: dto.LLM.AgentEnabled,
			Provider:     dto.LLM.Provider,
			ModelName:    dto.LLM.ModelName,
			BaseURL:      dto.LLM.BaseURL,
			APIKey:       dto.LLM.APIKey,
		}
	}

	if dto.ML != nil {
		var autoMode *config.MLAutoMode
		if dto.ML.AutoMode != nil {
			mode := config.MLAutoMode(*dto.ML.AutoMode)
			autoMode = &mode
		}

		input.ML = &service.UpdateMLSettingsInput{
			AutoMode:       autoMode,
			CLIPEnabled:    dto.ML.CLIPEnabled,
			OCREnabled:     dto.ML.OCREnabled,
			CaptionEnabled: dto.ML.CaptionEnabled,
			FaceEnabled:    dto.ML.FaceEnabled,
		}
	}

	return input, nil
}
