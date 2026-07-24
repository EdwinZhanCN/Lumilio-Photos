package service

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"server/internal/db/repo"
	"server/internal/llm"
	"server/internal/secretbox"
	"server/internal/settings"

	"github.com/jackc/pgx/v5"
)

type SystemSettings struct {
	LLM       LLMSettings
	ML        MLSettings
	Backup    BackupSettings
	UpdatedAt time.Time
	UpdatedBy *int32
}

type BackupSettings struct {
	Enabled       bool
	IntervalHours int32
	KeepLast      int32
}

type LLMSettings struct {
	AgentEnabled     bool
	Provider         string
	ModelName        string
	BaseURL          string
	APIKeyConfigured bool
}

func (s LLMSettings) EffectiveProvider() string {
	return normalizeStoredLLMProvider(s.Provider)
}

func (s LLMSettings) IsConfigured() bool {
	modelName := strings.TrimSpace(s.ModelName)
	if modelName == "" {
		return false
	}

	switch s.EffectiveProvider() {
	case "ollama":
		return strings.TrimSpace(s.BaseURL) != ""
	default:
		return s.APIKeyConfigured
	}
}

type MLSettings struct {
	SemanticEnabled           bool
	BioCLIPEnabled            bool
	OCREnabled                bool
	FaceEnabled               bool
	VideoSemanticEnabled      bool
	VideoMaxFrames            int32
	VideoLongThresholdSeconds int32
	VideoSceneThreshold       float64
}

type UpdateSystemSettingsInput struct {
	LLM       *UpdateLLMSettingsInput
	ML        *UpdateMLSettingsInput
	Backup    *UpdateBackupSettingsInput
	UpdatedBy *int32
}

type UpdateBackupSettingsInput struct {
	Enabled       *bool
	IntervalHours *int32
	KeepLast      *int32
}

type UpdateLLMSettingsInput struct {
	AgentEnabled *bool
	Provider     *string
	ModelName    *string
	BaseURL      *string
	APIKey       *string
}

type UpdateMLSettingsInput struct {
	SemanticEnabled           *bool
	BioCLIPEnabled            *bool
	OCREnabled                *bool
	FaceEnabled               *bool
	VideoSemanticEnabled      *bool
	VideoMaxFrames            *int32
	VideoLongThresholdSeconds *int32
	VideoSceneThreshold       *float64
}

type SettingsService interface {
	EnsureInitialized(ctx context.Context) error
	GetSystemSettings(ctx context.Context) (SystemSettings, error)
	UpdateSystemSettings(ctx context.Context, input UpdateSystemSettingsInput) (SystemSettings, error)
	GetLLMConfig(ctx context.Context) (settings.LLM, error)
	GetMLConfig(ctx context.Context) (settings.ML, error)
	GetBackupConfig(ctx context.Context) (settings.Backup, error)
	GetEffectiveMLConfig(ctx context.Context) (settings.ML, error)
	ValidateLLMSettings(ctx context.Context) error
}

type settingsService struct {
	queries          *repo.Queries
	secretPath       string
	defaults         settings.Settings
	encryptionSecret string
	secretOnce       sync.Once
	secretErr        error
}

// NewSettingsService wires the settings service. defaults supplies the
// program-fixed seed values for the runtime-mutable settings; secretKeyPath is
// the encryption key file. Repository defaults are owned by the storage package.
func NewSettingsService(queries *repo.Queries, defaults settings.Settings, secretKeyPath string) SettingsService {
	return &settingsService{
		queries:    queries,
		secretPath: strings.TrimSpace(secretKeyPath),
		defaults:   defaults,
	}
}

func (s *settingsService) EnsureInitialized(ctx context.Context) error {
	_, err := s.queries.GetSettings(ctx)
	if err == nil {
		return nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("get settings: %w", err)
	}

	return s.seedFromDefaults(ctx)
}

func (s *settingsService) GetSystemSettings(ctx context.Context) (SystemSettings, error) {
	row, err := s.getSettingsRow(ctx)
	if err != nil {
		return SystemSettings{}, err
	}

	return mapSystemSettings(row), nil
}

func (s *settingsService) UpdateSystemSettings(ctx context.Context, input UpdateSystemSettingsInput) (SystemSettings, error) {
	row, err := s.getSettingsRow(ctx)
	if err != nil {
		return SystemSettings{}, err
	}

	params := repo.UpsertSettingsParams{
		LlmAgentEnabled:             row.LlmAgentEnabled,
		LlmProvider:                 normalizeStoredLLMProvider(row.LlmProvider),
		LlmModelName:                strings.TrimSpace(row.LlmModelName),
		LlmBaseUrl:                  strings.TrimSpace(row.LlmBaseUrl),
		LlmApiKeyCiphertext:         cloneBytes(row.LlmApiKeyCiphertext),
		LlmApiKeyConfigured:         row.LlmApiKeyConfigured,
		MlAuto:                      row.MlAuto,
		MlSemanticEnabled:           row.MlSemanticEnabled,
		MlBioclipEnabled:            row.MlBioclipEnabled,
		MlOcrEnabled:                row.MlOcrEnabled,
		MlFaceEnabled:               row.MlFaceEnabled,
		MlVideoSemanticEnabled:      row.MlVideoSemanticEnabled,
		MlVideoMaxFrames:            row.MlVideoMaxFrames,
		MlVideoLongThresholdSeconds: row.MlVideoLongThresholdSeconds,
		MlVideoSceneThreshold:       row.MlVideoSceneThreshold,
		BackupEnabled:               row.BackupEnabled,
		BackupIntervalHours:         row.BackupIntervalHours,
		BackupKeepLast:              row.BackupKeepLast,
		UpdatedBy:                   input.UpdatedBy,
	}

	if input.LLM != nil {
		if input.LLM.AgentEnabled != nil {
			params.LlmAgentEnabled = *input.LLM.AgentEnabled
		}
		if input.LLM.Provider != nil {
			params.LlmProvider = normalizeStoredLLMProvider(*input.LLM.Provider)
		}
		if input.LLM.ModelName != nil {
			params.LlmModelName = strings.TrimSpace(*input.LLM.ModelName)
		}
		if input.LLM.BaseURL != nil {
			params.LlmBaseUrl = strings.TrimSpace(*input.LLM.BaseURL)
		}
		if input.LLM.APIKey != nil {
			apiKey := strings.TrimSpace(*input.LLM.APIKey)
			if apiKey == "" {
				params.LlmApiKeyCiphertext = nil
				params.LlmApiKeyConfigured = false
			} else {
				ciphertext, err := s.encrypt(apiKey)
				if err != nil {
					return SystemSettings{}, err
				}
				params.LlmApiKeyCiphertext = ciphertext
				params.LlmApiKeyConfigured = true
			}
		}
	}

	if input.ML != nil {
		if input.ML.SemanticEnabled != nil {
			params.MlSemanticEnabled = *input.ML.SemanticEnabled
		}
		if input.ML.BioCLIPEnabled != nil {
			params.MlBioclipEnabled = *input.ML.BioCLIPEnabled
		}
		if input.ML.OCREnabled != nil {
			params.MlOcrEnabled = *input.ML.OCREnabled
		}
		if input.ML.FaceEnabled != nil {
			params.MlFaceEnabled = *input.ML.FaceEnabled
		}
		if input.ML.VideoSemanticEnabled != nil {
			params.MlVideoSemanticEnabled = *input.ML.VideoSemanticEnabled
		}
		if input.ML.VideoMaxFrames != nil {
			params.MlVideoMaxFrames = clampInt32(*input.ML.VideoMaxFrames, 1, 32)
		}
		if input.ML.VideoLongThresholdSeconds != nil {
			params.MlVideoLongThresholdSeconds = clampInt32(*input.ML.VideoLongThresholdSeconds, 30, 3600)
		}
		if input.ML.VideoSceneThreshold != nil {
			params.MlVideoSceneThreshold = clampFloat64(*input.ML.VideoSceneThreshold, 0.05, 0.95)
		}
	}

	if input.Backup != nil {
		if input.Backup.Enabled != nil {
			params.BackupEnabled = *input.Backup.Enabled
		}
		if input.Backup.IntervalHours != nil {
			params.BackupIntervalHours = clampInt32(*input.Backup.IntervalHours, 1, 24*30)
		}
		if input.Backup.KeepLast != nil {
			params.BackupKeepLast = clampInt32(*input.Backup.KeepLast, 1, 365)
		}
	}

	updated, err := s.queries.UpsertSettings(ctx, params)
	if err != nil {
		return SystemSettings{}, fmt.Errorf("update settings: %w", err)
	}

	return mapSystemSettings(updated), nil
}

func (s *settingsService) GetLLMConfig(ctx context.Context) (settings.LLM, error) {
	row, err := s.getSettingsRow(ctx)
	if err != nil {
		return settings.LLM{}, err
	}

	apiKey := ""
	if len(row.LlmApiKeyCiphertext) > 0 {
		apiKey, err = s.decrypt(row.LlmApiKeyCiphertext)
		if err != nil {
			return settings.LLM{}, err
		}
	}

	return settings.LLM{
		AgentEnabled: row.LlmAgentEnabled,
		Provider:     normalizeStoredLLMProvider(row.LlmProvider),
		APIKey:       apiKey,
		ModelName:    strings.TrimSpace(row.LlmModelName),
		BaseURL:      strings.TrimSpace(row.LlmBaseUrl),
	}, nil
}

func (s *settingsService) GetMLConfig(ctx context.Context) (settings.ML, error) {
	row, err := s.getSettingsRow(ctx)
	if err != nil {
		return settings.ML{}, err
	}

	return settings.ML{
		SemanticEnabled:           row.MlSemanticEnabled,
		BioCLIPEnabled:            row.MlBioclipEnabled,
		OCREnabled:                row.MlOcrEnabled,
		FaceEnabled:               row.MlFaceEnabled,
		VideoSemanticEnabled:      row.MlVideoSemanticEnabled,
		VideoMaxFrames:            int(row.MlVideoMaxFrames),
		VideoLongThresholdSeconds: int(row.MlVideoLongThresholdSeconds),
		VideoSceneThreshold:       row.MlVideoSceneThreshold,
	}, nil
}

func (s *settingsService) GetEffectiveMLConfig(ctx context.Context) (settings.ML, error) {
	return s.GetMLConfig(ctx)
}

func (s *settingsService) GetBackupConfig(ctx context.Context) (settings.Backup, error) {
	row, err := s.getSettingsRow(ctx)
	if err != nil {
		return settings.Backup{}, err
	}

	return settings.Backup{
		Enabled:       row.BackupEnabled,
		IntervalHours: int(row.BackupIntervalHours),
		KeepLast:      int(row.BackupKeepLast),
	}, nil
}

func (s *settingsService) ValidateLLMSettings(ctx context.Context) error {
	cfg, err := s.GetLLMConfig(ctx)
	if err != nil {
		return err
	}

	validateCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	if err := llm.ValidateChatModel(validateCtx, cfg); err != nil {
		return fmt.Errorf("validate llm settings: %w", err)
	}

	return nil
}

func (s *settingsService) seedFromDefaults(ctx context.Context) error {
	llmCfg := s.defaults.LLM
	mlCfg := s.defaults.ML

	params := repo.UpsertSettingsParams{
		LlmAgentEnabled:             llmCfg.AgentEnabled,
		LlmProvider:                 normalizeStoredLLMProvider(llmCfg.Provider),
		LlmModelName:                strings.TrimSpace(llmCfg.ModelName),
		LlmBaseUrl:                  strings.TrimSpace(llmCfg.BaseURL),
		LlmApiKeyConfigured:         strings.TrimSpace(llmCfg.APIKey) != "",
		MlAuto:                      "disable",
		MlSemanticEnabled:           mlCfg.SemanticEnabled,
		MlBioclipEnabled:            mlCfg.BioCLIPEnabled,
		MlOcrEnabled:                mlCfg.OCREnabled,
		MlFaceEnabled:               mlCfg.FaceEnabled,
		MlVideoSemanticEnabled:      mlCfg.VideoSemanticEnabled,
		MlVideoMaxFrames:            int32(mlCfg.EffectiveVideoMaxFrames()),
		MlVideoLongThresholdSeconds: int32(mlCfg.EffectiveVideoLongThresholdSeconds()),
		MlVideoSceneThreshold:       mlCfg.EffectiveVideoSceneThreshold(),
		// Mirror the migration's column defaults: this INSERT names the backup
		// columns explicitly, so zero values here would override them.
		BackupEnabled:       true,
		BackupIntervalHours: 24,
		BackupKeepLast:      14,
	}

	if params.LlmApiKeyConfigured {
		ciphertext, err := s.encrypt(strings.TrimSpace(llmCfg.APIKey))
		if err != nil {
			return fmt.Errorf("seed settings from defaults: %w", err)
		}
		params.LlmApiKeyCiphertext = ciphertext
	}

	if _, err := s.queries.UpsertSettings(ctx, params); err != nil {
		return fmt.Errorf("seed settings from defaults: %w", err)
	}

	return nil
}

func (s *settingsService) getSettingsRow(ctx context.Context) (repo.Setting, error) {
	if err := s.EnsureInitialized(ctx); err != nil {
		return repo.Setting{}, err
	}

	row, err := s.queries.GetSettings(ctx)
	if err != nil {
		return repo.Setting{}, fmt.Errorf("get settings: %w", err)
	}

	return row, nil
}

func mapSystemSettings(row repo.Setting) SystemSettings {
	var updatedAt time.Time
	if row.UpdatedAt.Valid {
		updatedAt = row.UpdatedAt.Time
	}

	return SystemSettings{
		LLM: LLMSettings{
			AgentEnabled:     row.LlmAgentEnabled,
			Provider:         normalizeStoredLLMProvider(row.LlmProvider),
			ModelName:        strings.TrimSpace(row.LlmModelName),
			BaseURL:          strings.TrimSpace(row.LlmBaseUrl),
			APIKeyConfigured: row.LlmApiKeyConfigured,
		},
		ML: MLSettings{
			SemanticEnabled:           row.MlSemanticEnabled,
			BioCLIPEnabled:            row.MlBioclipEnabled,
			OCREnabled:                row.MlOcrEnabled,
			FaceEnabled:               row.MlFaceEnabled,
			VideoSemanticEnabled:      row.MlVideoSemanticEnabled,
			VideoMaxFrames:            row.MlVideoMaxFrames,
			VideoLongThresholdSeconds: row.MlVideoLongThresholdSeconds,
			VideoSceneThreshold:       row.MlVideoSceneThreshold,
		},
		Backup: BackupSettings{
			Enabled:       row.BackupEnabled,
			IntervalHours: row.BackupIntervalHours,
			KeepLast:      row.BackupKeepLast,
		},
		UpdatedAt: updatedAt,
		UpdatedBy: row.UpdatedBy,
	}
}

func clampInt32(v, lo, hi int32) int32 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func clampFloat64(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func normalizeStoredLLMProvider(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "openai":
		return "openai"
	case "deepseek":
		return "deepseek"
	case "ollama":
		return "ollama"
	case "ark":
		return "ark"
	default:
		return "ark"
	}
}

func (s *settingsService) encryptionKey() ([]byte, error) {
	if err := s.ensureEncryptionSecret(); err != nil {
		return nil, err
	}

	sum := sha256.Sum256([]byte(s.encryptionSecret))
	key := make([]byte, len(sum))
	copy(key, sum[:])
	return key, nil
}

func (s *settingsService) ensureEncryptionSecret() error {
	s.secretOnce.Do(func() {
		if s.encryptionSecret != "" {
			return
		}

		secret, err := secretbox.LoadOrCreateLumilioSecretKey(s.secretPath)
		if err != nil {
			s.secretErr = err
			return
		}
		s.encryptionSecret = secret
	})

	return s.secretErr
}

func (s *settingsService) encrypt(plaintext string) ([]byte, error) {
	key, err := s.encryptionKey()
	if err != nil {
		return nil, err
	}
	return secretbox.NewWithKey(key).Encrypt(plaintext)
}

func (s *settingsService) decrypt(ciphertext []byte) (string, error) {
	key, err := s.encryptionKey()
	if err != nil {
		return "", err
	}
	return secretbox.NewWithKey(key).Decrypt(ciphertext)
}

func cloneBytes(src []byte) []byte {
	if len(src) == 0 {
		return nil
	}

	dst := make([]byte, len(src))
	copy(dst, src)
	return dst
}
