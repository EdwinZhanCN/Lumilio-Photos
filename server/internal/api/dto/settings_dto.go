package dto

import (
	"time"

	"server/config"
	"server/internal/service"
)

type SystemSettingsDTO struct {
	LLM       LLMSettingsDTO       `json:"llm"`
	ML        MLSettingsDTO        `json:"ml"`
	Backup    BackupSettingsDTO    `json:"backup"`
	Geocoding GeocodingSettingsDTO `json:"geocoding"`
	UpdatedAt time.Time            `json:"updated_at"`
	UpdatedBy *int32               `json:"updated_by,omitempty"`
}

type BackupSettingsDTO struct {
	Enabled       bool  `json:"enabled"`
	IntervalHours int32 `json:"interval_hours" example:"24"`
	KeepLast      int32 `json:"keep_last" example:"14"`
}

type GeocodingSettingsDTO struct {
	Provider          string `json:"provider" example:"disabled" enums:"disabled,nominatim"`
	NominatimEndpoint string `json:"nominatim_endpoint" example:"https://nominatim.openstreetmap.org/reverse"`
	Language          string `json:"language" example:"en"`
	UserAgent         string `json:"user_agent" example:"Lumilio-Photos/1.0"`
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
	DefaultRoot       string   `json:"default_root" example:"/data/storage"`
	Strategy          string   `json:"strategy" example:"date"`
	DuplicateHandling string   `json:"duplicate_handling" example:"rename"`
	RiskWarnings      []string `json:"risk_warnings,omitempty"`
}

type UpdateSystemSettingsDTO struct {
	LLM       *UpdateLLMSettingsDTO       `json:"llm,omitempty"`
	ML        *UpdateMLSettingsDTO        `json:"ml,omitempty"`
	Backup    *UpdateBackupSettingsDTO    `json:"backup,omitempty"`
	Geocoding *UpdateGeocodingSettingsDTO `json:"geocoding,omitempty"`
}

type UpdateBackupSettingsDTO struct {
	Enabled       *bool  `json:"enabled,omitempty"`
	IntervalHours *int32 `json:"interval_hours,omitempty" binding:"omitempty,min=1,max=720"`
	KeepLast      *int32 `json:"keep_last,omitempty" binding:"omitempty,min=1,max=365"`
}

type UpdateGeocodingSettingsDTO struct {
	Provider          *string `json:"provider,omitempty" enums:"disabled,nominatim"`
	NominatimEndpoint *string `json:"nominatim_endpoint,omitempty"`
	Language          *string `json:"language,omitempty"`
	UserAgent         *string `json:"user_agent,omitempty"`
}

type UpdateLLMSettingsDTO struct {
	AgentEnabled *bool   `json:"agent_enabled,omitempty"`
	Provider     *string `json:"provider,omitempty" binding:"omitempty,oneof=none ark openai deepseek ollama"`
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

type ValidateLLMSettingsRequestDTO struct {
	Provider        string `json:"provider" binding:"required,oneof=ark openai deepseek ollama" example:"openai"`
	ModelName       string `json:"model_name" binding:"required" example:"gpt-4.1-mini"`
	BaseURL         string `json:"base_url,omitempty" example:"https://api.openai.com/v1"`
	APIKey          string `json:"api_key,omitempty"`
	UseStoredAPIKey bool   `json:"use_stored_api_key,omitempty"`
}

type ValidateLLMSettingsResponseDTO struct {
	Valid bool `json:"valid"`
}

func (dto ValidateLLMSettingsRequestDTO) ToServiceInput() service.ValidateLLMDraftInput {
	return service.ValidateLLMDraftInput{
		Provider:        dto.Provider,
		ModelName:       dto.ModelName,
		BaseURL:         dto.BaseURL,
		APIKey:          dto.APIKey,
		UseStoredAPIKey: dto.UseStoredAPIKey,
	}
}

// RuntimeInfoDTO is a read-only snapshot of the runtime-immutable configuration
// (changed only by editing TOML and restarting). Shown in the Settings → Server
// tab so operators can see effective boot configuration.
type RuntimeInfoDTO struct {
	Environment                  string     `json:"environment" example:"production"`
	ServerListen                 string     `json:"server_listen" example:"0.0.0.0:6680"`
	TLSMode                      string     `json:"tls_mode" example:"off"`
	PasskeyEnabled               bool       `json:"passkey_enabled" example:"true"`
	ACMECertificateHostname      string     `json:"acme_certificate_hostname,omitempty" example:"photos.example.com"`
	ACMECertificateStatus        string     `json:"acme_certificate_status" example:"active"`
	ACMECertificateExpiresAt     *time.Time `json:"acme_certificate_expires_at,omitempty"`
	ACMELastManagedAt            *time.Time `json:"acme_last_managed_at,omitempty"`
	LogLevel                     string     `json:"log_level" example:"info"`
	StorageRoot                  string     `json:"storage_root" example:"/data/storage"`
	HardwareAccel                string     `json:"hardware_accel" example:"none"`
	RepositoryScanEnabled        bool       `json:"repository_scan_enabled" example:"true"`
	RepositoryScanIntervalSecond int        `json:"repository_scan_interval_seconds" example:"300"`
	LumenDiscoveryEnabled        bool       `json:"lumen_discovery_enabled" example:"true"`
}

type CertificateRuntimeInfo struct {
	Hostname      string
	Status        string
	ExpiresAt     *time.Time
	LastManagedAt *time.Time
}

// NewRuntimeInfoDTO builds the read-only runtime info snapshot from the immutable
// application configuration.
func NewRuntimeInfoDTO(cfg config.AppConfig) RuntimeInfoDTO {
	return RuntimeInfoDTO{
		Environment:                  cfg.Environment,
		ServerListen:                 cfg.ServerConfig.Listen,
		TLSMode:                      string(cfg.ServerConfig.TLS.Mode),
		PasskeyEnabled:               cfg.Auth.Passkey.Enabled,
		ACMECertificateStatus:        map[bool]string{true: "initializing", false: "not_applicable"}[cfg.ServerConfig.TLS.Mode == config.TLSModeACME],
		LogLevel:                     cfg.LoggingConfig.Level,
		StorageRoot:                  cfg.StorageConfig.Path,
		HardwareAccel:                cfg.Transcode.HardwareAccel,
		RepositoryScanEnabled:        cfg.RepositoryScan.Enabled,
		RepositoryScanIntervalSecond: cfg.RepositoryScan.IntervalSeconds,
		LumenDiscoveryEnabled:        cfg.Lumen.DiscoveryEnabled,
	}
}

func (info *RuntimeInfoDTO) ApplyCertificateRuntime(certificate CertificateRuntimeInfo) {
	info.ACMECertificateHostname = certificate.Hostname
	info.ACMECertificateStatus = certificate.Status
	info.ACMECertificateExpiresAt = certificate.ExpiresAt
	info.ACMELastManagedAt = certificate.LastManagedAt
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
		Geocoding: GeocodingSettingsDTO{
			Provider:          settings.Geocoding.Provider,
			NominatimEndpoint: settings.Geocoding.NominatimEndpoint,
			Language:          settings.Geocoding.Language,
			UserAgent:         settings.Geocoding.UserAgent,
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

	if dto.Geocoding != nil {
		input.Geocoding = &service.UpdateGeocodingSettingsInput{
			Provider:          dto.Geocoding.Provider,
			NominatimEndpoint: dto.Geocoding.NominatimEndpoint,
			Language:          dto.Geocoding.Language,
			UserAgent:         dto.Geocoding.UserAgent,
		}
	}

	return input, nil
}
