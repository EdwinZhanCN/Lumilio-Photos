package service

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"server/config"
	"server/internal/db/repo"
	"server/internal/llm"

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
	AutoMode       config.MLAutoMode
	CLIPEnabled    bool
	OCREnabled     bool
	CaptionEnabled bool
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
	AutoMode       *config.MLAutoMode
	CLIPEnabled    *bool
	OCREnabled     *bool
	CaptionEnabled *bool
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
		MlAuto:              normalizeStoredMLAutoMode(row.MlAuto),
		MlClipEnabled:       row.MlClipEnabled,
		MlOcrEnabled:        row.MlOcrEnabled,
		MlCaptionEnabled:    row.MlCaptionEnabled,
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
		if input.ML.AutoMode != nil {
			params.MlAuto = normalizeStoredMLAutoMode(string(*input.ML.AutoMode))
		}
		if input.ML.CLIPEnabled != nil {
			params.MlClipEnabled = *input.ML.CLIPEnabled
		}
		if input.ML.OCREnabled != nil {
			params.MlOcrEnabled = *input.ML.OCREnabled
		}
		if input.ML.CaptionEnabled != nil {
			params.MlCaptionEnabled = *input.ML.CaptionEnabled
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
		AutoMode:       config.MLAutoMode(normalizeStoredMLAutoMode(row.MlAuto)),
		CLIPEnabled:    row.MlClipEnabled,
		OCREnabled:     row.MlOcrEnabled,
		CaptionEnabled: row.MlCaptionEnabled,
		FaceEnabled:    row.MlFaceEnabled,
	}, nil
}

func (s *settingsService) GetEffectiveMLConfig(ctx context.Context) (config.MLConfig, error) {
	cfg, err := s.GetMLConfig(ctx)
	if err != nil {
		return config.MLConfig{}, err
	}

	return cfg.EffectiveRuntimeConfig(), nil
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
		MlAuto:              string(normalizeMLAutoMode(mlCfg.AutoMode)),
		MlClipEnabled:       mlCfg.CLIPEnabled,
		MlOcrEnabled:        mlCfg.OCREnabled,
		MlCaptionEnabled:    mlCfg.CaptionEnabled,
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
			AutoMode:       config.MLAutoMode(normalizeStoredMLAutoMode(row.MlAuto)),
			CLIPEnabled:    row.MlClipEnabled,
			OCREnabled:     row.MlOcrEnabled,
			CaptionEnabled: row.MlCaptionEnabled,
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

func normalizeStoredMLAutoMode(raw string) string {
	return string(normalizeMLAutoMode(config.MLAutoMode(strings.TrimSpace(raw))))
}

func normalizeMLAutoMode(mode config.MLAutoMode) config.MLAutoMode {
	switch strings.ToLower(strings.TrimSpace(string(mode))) {
	case string(config.MLAutoModeEnable):
		return config.MLAutoModeEnable
	default:
		return config.MLAutoModeDisable
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

		secret, err := loadOrCreateLumilioSecretKey(s.secretPath)
		if err != nil {
			s.secretErr = err
			return
		}
		s.encryptionSecret = secret
	})

	return s.secretErr
}

func loadOrCreateLumilioSecretKey(configuredPath string) (string, error) {
	keyFile := strings.TrimSpace(configuredPath)
	if keyFile != "" {
		if !isExplicitFilePath(keyFile) {
			return "", errors.New("LUMILIO_SECRET_KEY must be a key file path (absolute path, ./relative, or ../relative)")
		}
		keyFile = filepath.Clean(keyFile)
	} else {
		keyFile = defaultLumilioSecretKeyPath()
	}

	content, err := os.ReadFile(keyFile)
	switch {
	case err == nil:
		secret := strings.TrimSpace(string(content))
		if secret == "" {
			return "", fmt.Errorf("LUMILIO secret key file is empty: %s", keyFile)
		}
		return secret, nil
	case errors.Is(err, os.ErrNotExist):
		// Continue to generate a new key.
	default:
		return "", fmt.Errorf("read LUMILIO secret key file %s: %w", keyFile, err)
	}

	if err := os.MkdirAll(filepath.Dir(keyFile), 0o700); err != nil {
		return "", fmt.Errorf("create secret key directory: %w", err)
	}

	random := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, random); err != nil {
		return "", fmt.Errorf("generate LUMILIO secret key: %w", err)
	}

	secret := fmt.Sprintf("%x", random)
	if err := os.WriteFile(keyFile, []byte(secret+"\n"), 0o600); err != nil {
		return "", fmt.Errorf("persist LUMILIO secret key: %w", err)
	}

	return secret, nil
}

func isExplicitFilePath(path string) bool {
	if filepath.IsAbs(path) {
		return true
	}
	return strings.HasPrefix(path, "./") || strings.HasPrefix(path, "../")
}

func defaultLumilioSecretKeyPath() string {
	storagePath := strings.TrimSpace(os.Getenv("STORAGE_PATH"))
	if storagePath != "" {
		normalized := filepath.Clean(storagePath)
		if strings.EqualFold(filepath.Base(normalized), "primary") {
			normalized = filepath.Dir(normalized)
		}
		return filepath.Join(normalized, ".secrets", "lumilio_secret_key")
	}

	return filepath.Join("data", "storage", ".secrets", "lumilio_secret_key")
}

func (s *settingsService) encrypt(plaintext string) ([]byte, error) {
	key, err := s.encryptionKey()
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create gcm: %w", err)
	}

	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}

	return aead.Seal(nonce, nonce, []byte(plaintext), nil), nil
}

func (s *settingsService) decrypt(ciphertext []byte) (string, error) {
	key, err := s.encryptionKey()
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("create cipher: %w", err)
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create gcm: %w", err)
	}

	if len(ciphertext) < aead.NonceSize() {
		return "", errors.New("invalid ciphertext")
	}

	nonce := ciphertext[:aead.NonceSize()]
	data := ciphertext[aead.NonceSize():]

	plaintext, err := aead.Open(nil, nonce, data, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt settings secret: %w", err)
	}

	return string(plaintext), nil
}

func cloneBytes(src []byte) []byte {
	if len(src) == 0 {
		return nil
	}

	dst := make([]byte, len(src))
	copy(dst, src)
	return dst
}
