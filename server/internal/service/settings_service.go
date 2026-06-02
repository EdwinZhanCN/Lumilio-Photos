package service

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"server/config"
	"server/internal/db/repo"
	"server/internal/llm"
	"server/internal/secretbox"

	"github.com/jackc/pgx/v5"
)

type SystemSettings struct {
	LLM       LLMSettings
	ML        MLSettings
	UpdatedAt time.Time
	UpdatedBy *int32
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
	CLIPEnabled    bool
	BioCLIPEnabled bool
	OCREnabled     bool
	FaceEnabled    bool
}

type UpdateSystemSettingsInput struct {
	LLM       *UpdateLLMSettingsInput
	ML        *UpdateMLSettingsInput
	UpdatedBy *int32
}

type UpdateLLMSettingsInput struct {
	AgentEnabled *bool
	Provider     *string
	ModelName    *string
	BaseURL      *string
	APIKey       *string
}

type UpdateMLSettingsInput struct {
	CLIPEnabled    *bool
	BioCLIPEnabled *bool
	OCREnabled     *bool
	FaceEnabled    *bool
}

type SettingsService interface {
	EnsureInitialized(ctx context.Context) error
	GetSystemSettings(ctx context.Context) (SystemSettings, error)
	UpdateSystemSettings(ctx context.Context, input UpdateSystemSettingsInput) (SystemSettings, error)
	GetLLMConfig(ctx context.Context) (config.LLMConfig, error)
	GetMLConfig(ctx context.Context) (config.MLConfig, error)
	GetEffectiveMLConfig(ctx context.Context) (config.MLConfig, error)
	ValidateLLMSettings(ctx context.Context) error
}

type settingsService struct {
	queries          *repo.Queries
	secretPath       string
	encryptionSecret string
	secretOnce       sync.Once
	secretErr        error
}

func NewSettingsService(queries *repo.Queries) SettingsService {
	return &settingsService{
		queries:    queries,
		secretPath: strings.TrimSpace(os.Getenv("LUMILIO_SECRET_KEY")),
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

	return s.seedFromEnv(ctx)
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
		LlmAgentEnabled:     row.LlmAgentEnabled,
		LlmProvider:         normalizeStoredLLMProvider(row.LlmProvider),
		LlmModelName:        strings.TrimSpace(row.LlmModelName),
		LlmBaseUrl:          strings.TrimSpace(row.LlmBaseUrl),
		LlmApiKeyCiphertext: cloneBytes(row.LlmApiKeyCiphertext),
		LlmApiKeyConfigured: row.LlmApiKeyConfigured,
		MlAuto:              row.MlAuto,
		MlClipEnabled:       row.MlClipEnabled,
		MlBioclipEnabled:    row.MlBioclipEnabled,
		MlOcrEnabled:        row.MlOcrEnabled,
		MlFaceEnabled:       row.MlFaceEnabled,
		UpdatedBy:           input.UpdatedBy,
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
		if input.ML.CLIPEnabled != nil {
			params.MlClipEnabled = *input.ML.CLIPEnabled
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
	}

	updated, err := s.queries.UpsertSettings(ctx, params)
	if err != nil {
		return SystemSettings{}, fmt.Errorf("update settings: %w", err)
	}

	return mapSystemSettings(updated), nil
}

func (s *settingsService) GetLLMConfig(ctx context.Context) (config.LLMConfig, error) {
	row, err := s.getSettingsRow(ctx)
	if err != nil {
		return config.LLMConfig{}, err
	}

	apiKey := ""
	if len(row.LlmApiKeyCiphertext) > 0 {
		apiKey, err = s.decrypt(row.LlmApiKeyCiphertext)
		if err != nil {
			return config.LLMConfig{}, err
		}
	}

	return config.LLMConfig{
		AgentEnabled: row.LlmAgentEnabled,
		Provider:     normalizeStoredLLMProvider(row.LlmProvider),
		APIKey:       apiKey,
		ModelName:    strings.TrimSpace(row.LlmModelName),
		BaseURL:      strings.TrimSpace(row.LlmBaseUrl),
	}, nil
}

func (s *settingsService) GetMLConfig(ctx context.Context) (config.MLConfig, error) {
	row, err := s.getSettingsRow(ctx)
	if err != nil {
		return config.MLConfig{}, err
	}

	return config.MLConfig{
		CLIPEnabled:    row.MlClipEnabled,
		BioCLIPEnabled: row.MlBioclipEnabled,
		OCREnabled:     row.MlOcrEnabled,
		FaceEnabled:    row.MlFaceEnabled,
	}, nil
}

func (s *settingsService) GetEffectiveMLConfig(ctx context.Context) (config.MLConfig, error) {
	return s.GetMLConfig(ctx)
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

func (s *settingsService) seedFromEnv(ctx context.Context) error {
	llmCfg := config.LoadLLMConfig()
	mlCfg := config.LoadMLConfig()

	params := repo.UpsertSettingsParams{
		LlmAgentEnabled:     llmCfg.AgentEnabled,
		LlmProvider:         normalizeStoredLLMProvider(llmCfg.Provider),
		LlmModelName:        strings.TrimSpace(llmCfg.ModelName),
		LlmBaseUrl:          strings.TrimSpace(llmCfg.BaseURL),
		LlmApiKeyConfigured: strings.TrimSpace(llmCfg.APIKey) != "",
		MlAuto:              "disable",
		MlClipEnabled:       mlCfg.CLIPEnabled,
		MlBioclipEnabled:    mlCfg.BioCLIPEnabled,
		MlOcrEnabled:        mlCfg.OCREnabled,
		MlFaceEnabled:       mlCfg.FaceEnabled,
	}

	if params.LlmApiKeyConfigured {
		ciphertext, err := s.encrypt(strings.TrimSpace(llmCfg.APIKey))
		if err != nil {
			return fmt.Errorf("seed settings from env: %w", err)
		}
		params.LlmApiKeyCiphertext = ciphertext
	}

	if _, err := s.queries.UpsertSettings(ctx, params); err != nil {
		return fmt.Errorf("seed settings from env: %w", err)
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
			CLIPEnabled:    row.MlClipEnabled,
			BioCLIPEnabled: row.MlBioclipEnabled,
			OCREnabled:     row.MlOcrEnabled,
			FaceEnabled:    row.MlFaceEnabled,
		},
		UpdatedAt: updatedAt,
		UpdatedBy: row.UpdatedBy,
	}
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
