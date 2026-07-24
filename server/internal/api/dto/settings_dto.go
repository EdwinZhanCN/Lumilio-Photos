package dto

import (
	"time"

	"server/config"
	"server/internal/service"
)

type SystemSettingsDTO struct {
	LLM       LLMSettingsDTO    `json:"llm"`
	ML        MLSettingsDTO     `json:"ml"`
	Backup    BackupSettingsDTO `json:"backup"`
	UpdatedAt time.Time         `json:"updated_at"`
	UpdatedBy *int32            `json:"updated_by,omitempty"`
}

type BackupSettingsDTO struct {
	Enabled       bool  `json:"enabled"`
	IntervalHours int32 `json:"interval_hours" example:"24"`
	KeepLast      int32 `json:"keep_last" example:"14"`
}

type LLMSettingsDTO struct {
	AgentEnabled     bool   `json:"agent_enabled"`
	Provider         string `json:"provider" example:"openai"`
	ModelName        string `json:"model_name" example:"gpt-4.1-mini"`
	BaseURL          string `json:"base_url,omitempty" example:"https://api.openai.com/v1"`
	APIKeyConfigured bool   `json:"api_key_configured"`
}

type MLSettingsDTO struct {
	SemanticEnabled           bool    `json:"semantic_enabled"`
	BioCLIPEnabled            bool    `json:"bioclip_enabled"`
	OCREnabled                bool    `json:"ocr_enabled"`
	FaceEnabled               bool    `json:"face_enabled"`
	VideoSemanticEnabled      bool    `json:"video_semantic_enabled"`
	VideoMaxFrames            int32   `json:"video_max_frames" example:"8"`
	VideoLongThresholdSeconds int32   `json:"video_long_threshold_seconds" example:"300"`
	VideoSceneThreshold       float64 `json:"video_scene_threshold" example:"0.4"`
}

type RepositoryDefaultsDTO struct {
	DefaultRoot       string `json:"default_root" example:"/data/storage"`
	Strategy          string `json:"strategy" example:"date"`
	DuplicateHandling string `json:"duplicate_handling" example:"rename"`
}

type UpdateSystemSettingsDTO struct {
	LLM    *UpdateLLMSettingsDTO    `json:"llm,omitempty"`
	ML     *UpdateMLSettingsDTO     `json:"ml,omitempty"`
	Backup *UpdateBackupSettingsDTO `json:"backup,omitempty"`
}

type UpdateBackupSettingsDTO struct {
	Enabled       *bool  `json:"enabled,omitempty"`
	IntervalHours *int32 `json:"interval_hours,omitempty" binding:"omitempty,min=1,max=720"`
	KeepLast      *int32 `json:"keep_last,omitempty" binding:"omitempty,min=1,max=365"`
}

type UpdateLLMSettingsDTO struct {
	AgentEnabled *bool   `json:"agent_enabled,omitempty"`
	Provider     *string `json:"provider,omitempty" binding:"omitempty,oneof=ark openai deepseek ollama"`
	ModelName    *string `json:"model_name,omitempty"`
	BaseURL      *string `json:"base_url,omitempty"`
	APIKey       *string `json:"api_key,omitempty"`
}

type UpdateMLSettingsDTO struct {
	SemanticEnabled           *bool    `json:"semantic_enabled,omitempty"`
	BioCLIPEnabled            *bool    `json:"bioclip_enabled,omitempty"`
	OCREnabled                *bool    `json:"ocr_enabled,omitempty"`
	FaceEnabled               *bool    `json:"face_enabled,omitempty"`
	VideoSemanticEnabled      *bool    `json:"video_semantic_enabled,omitempty"`
	VideoMaxFrames            *int32   `json:"video_max_frames,omitempty" binding:"omitempty,min=1,max=32"`
	VideoLongThresholdSeconds *int32   `json:"video_long_threshold_seconds,omitempty" binding:"omitempty,min=30,max=3600"`
	VideoSceneThreshold       *float64 `json:"video_scene_threshold,omitempty" binding:"omitempty,min=0.05,max=0.95"`
}

type ValidateLLMSettingsResponseDTO struct {
	Valid bool `json:"valid"`
}

// RuntimeInfoDTO is a read-only snapshot of the runtime-immutable configuration
// (changed only by editing TOML and restarting). Shown in the Settings → Server
// tab so operators can see effective boot configuration.
type RuntimeInfoDTO struct {
	Environment                  string `json:"environment" example:"production"`
	ServerPort                   string `json:"server_port" example:"8080"`
	LogLevel                     string `json:"log_level" example:"info"`
	StorageRoot                  string `json:"storage_root" example:"/data/storage"`
	HardwareAccel                string `json:"hardware_accel" example:"none"`
	GeocodingProvider            string `json:"geocoding_provider" example:"disabled"`
	RepositoryScanEnabled        bool   `json:"repository_scan_enabled" example:"true"`
	RepositoryScanIntervalSecond int    `json:"repository_scan_interval_seconds" example:"300"`
	LumenDiscoveryEnabled        bool   `json:"lumen_discovery_enabled" example:"true"`
}

// NewRuntimeInfoDTO builds the read-only runtime info snapshot from the immutable
// application configuration.
func NewRuntimeInfoDTO(cfg config.AppConfig) RuntimeInfoDTO {
	return RuntimeInfoDTO{
		Environment:                  cfg.Environment,
		ServerPort:                   cfg.ServerConfig.Port,
		LogLevel:                     cfg.LoggingConfig.Level,
		StorageRoot:                  cfg.StorageConfig.Path,
		HardwareAccel:                cfg.Transcode.HardwareAccel,
		GeocodingProvider:            cfg.Geocoding.Provider,
		RepositoryScanEnabled:        cfg.RepositoryScan.Enabled,
		RepositoryScanIntervalSecond: cfg.RepositoryScan.IntervalSeconds,
		LumenDiscoveryEnabled:        cfg.Lumen.DiscoveryEnabled,
	}
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
			SemanticEnabled:           settings.ML.SemanticEnabled,
			BioCLIPEnabled:            settings.ML.BioCLIPEnabled,
			OCREnabled:                settings.ML.OCREnabled,
			FaceEnabled:               settings.ML.FaceEnabled,
			VideoSemanticEnabled:      settings.ML.VideoSemanticEnabled,
			VideoMaxFrames:            settings.ML.VideoMaxFrames,
			VideoLongThresholdSeconds: settings.ML.VideoLongThresholdSeconds,
			VideoSceneThreshold:       settings.ML.VideoSceneThreshold,
		},
		Backup: BackupSettingsDTO{
			Enabled:       settings.Backup.Enabled,
			IntervalHours: settings.Backup.IntervalHours,
			KeepLast:      settings.Backup.KeepLast,
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
		input.ML = &service.UpdateMLSettingsInput{
			SemanticEnabled:           dto.ML.SemanticEnabled,
			BioCLIPEnabled:            dto.ML.BioCLIPEnabled,
			OCREnabled:                dto.ML.OCREnabled,
			FaceEnabled:               dto.ML.FaceEnabled,
			VideoSemanticEnabled:      dto.ML.VideoSemanticEnabled,
			VideoMaxFrames:            dto.ML.VideoMaxFrames,
			VideoLongThresholdSeconds: dto.ML.VideoLongThresholdSeconds,
			VideoSceneThreshold:       dto.ML.VideoSceneThreshold,
		}
	}

	if dto.Backup != nil {
		input.Backup = &service.UpdateBackupSettingsInput{
			Enabled:       dto.Backup.Enabled,
			IntervalHours: dto.Backup.IntervalHours,
			KeepLast:      dto.Backup.KeepLast,
		}
	}

	return input, nil
}
