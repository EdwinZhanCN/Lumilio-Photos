package service

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"server/internal/db/catalogtx"
	"server/internal/db/repo"
	"server/internal/llm"
	"server/internal/queue/jobs"
	"server/internal/secretbox"
	"server/internal/settings"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
)

var ErrInvalidSystemSettings = errors.New("invalid system settings")

// RiverJobInserter is the small transaction boundary the settings service
// needs. Keeping this interface here avoids making service depend on the
// queue package while still allowing River InsertTx to share the settings
// transaction.
type RiverJobInserter interface {
	InsertTx(context.Context, *sql.Tx, river.JobArgs, *river.InsertOpts) (*rivertype.JobInsertResult, error)
}

type SettingsRuntime struct {
	DB     *sql.DB
	Writer *catalogtx.Writer
	Queue  RiverJobInserter
}

type SystemSettings struct {
	LLM       LLMSettings
	ML        MLSettings
	Backup    BackupSettings
	Geocoding GeocodingSettings
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

type GeocodingSettings struct {
	Provider          string
	NominatimEndpoint string
	Language          string
	UserAgent         string
}

func (s LLMSettings) EffectiveProvider() string {
	return normalizeStoredLLMProvider(s.Provider)
}

func (s LLMSettings) IsConfigured() bool {
	provider := s.EffectiveProvider()
	descriptor, ok := settings.LookupLLMProvider(provider)
	if !ok || strings.TrimSpace(s.ModelName) == "" {
		return false
	}
	if descriptor.BaseURLRequired && strings.TrimSpace(s.BaseURL) == "" {
		return false
	}
	if descriptor.APIKeyRequired && !s.APIKeyConfigured {
		return false
	}
	return true
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
	Geocoding *UpdateGeocodingSettingsInput
	UpdatedBy *int32
}

type UpdateGeocodingSettingsInput struct {
	Provider          *string
	NominatimEndpoint *string
	Language          *string
	UserAgent         *string
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

type ValidateLLMDraftInput struct {
	Provider        string
	ModelName       string
	BaseURL         string
	APIKey          string
	UseStoredAPIKey bool
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
	GetGeocodingConfig(ctx context.Context) (settings.Geocoding, error)
	GetEffectiveMLConfig(ctx context.Context) (settings.ML, error)
	ValidateLLMDraft(ctx context.Context, input ValidateLLMDraftInput) error
}

type settingsService struct {
	queries          *repo.Queries
	db               *sql.DB
	writer           *catalogtx.Writer
	queue            RiverJobInserter
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
	return NewSettingsServiceWithRuntime(queries, defaults, secretKeyPath, SettingsRuntime{})
}

func NewSettingsServiceWithRuntime(queries *repo.Queries, defaults settings.Settings, secretKeyPath string, runtime SettingsRuntime) SettingsService {
	return &settingsService{
		queries:    queries,
		db:         runtime.DB,
		writer:     runtime.Writer,
		queue:      runtime.Queue,
		secretPath: strings.TrimSpace(secretKeyPath),
		defaults:   defaults,
	}
}

func (s *settingsService) EnsureInitialized(ctx context.Context) error {
	_, err := s.queries.GetSettings(ctx)
	if err == nil {
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
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
		GeocodingProvider:           strings.TrimSpace(row.GeocodingProvider),
		GeocodingNominatimEndpoint:  strings.TrimSpace(row.GeocodingNominatimEndpoint),
		GeocodingLanguage:           strings.TrimSpace(row.GeocodingLanguage),
		GeocodingUserAgent:          strings.TrimSpace(row.GeocodingUserAgent),
		GeocodingRevision:           row.GeocodingRevision,
		UpdatedBy:                   input.UpdatedBy,
	}

	currentGeocoding, err := normalizeStoredGeocoding(row)
	if err != nil {
		return SystemSettings{}, fmt.Errorf("%w: stored geocoding settings: %v", ErrInvalidSystemSettings, err)
	}
	candidateGeocoding := currentGeocoding
	geocodingChanged := false

	if input.LLM != nil {
		if input.LLM.AgentEnabled != nil {
			params.LlmAgentEnabled = *input.LLM.AgentEnabled
		}
		if input.LLM.Provider != nil {
			rawProvider := strings.ToLower(strings.TrimSpace(*input.LLM.Provider))
			if rawProvider != "none" && !settings.IsSupportedLLMProvider(rawProvider) {
				return SystemSettings{}, fmt.Errorf("unsupported llm provider %q", rawProvider)
			}
			previousProvider := params.LlmProvider
			params.LlmProvider = normalizeStoredLLMProvider(rawProvider)
			// Credentials are bound to the explicitly selected provider. Reusing a
			// secret after the provider changes is an invisible and usually invalid
			// state transition, so require an explicit replacement instead.
			if params.LlmProvider != previousProvider && input.LLM.APIKey == nil {
				params.LlmApiKeyCiphertext = nil
				params.LlmApiKeyConfigured = false
			}
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
		if params.LlmProvider == "" {
			params.LlmAgentEnabled = false
			params.LlmModelName = ""
			params.LlmBaseUrl = ""
			params.LlmApiKeyCiphertext = nil
			params.LlmApiKeyConfigured = false
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
			params.MlVideoMaxFrames = int64(clampInt32(*input.ML.VideoMaxFrames, 1, 32))
		}
		if input.ML.VideoLongThresholdSeconds != nil {
			params.MlVideoLongThresholdSeconds = int64(clampInt32(*input.ML.VideoLongThresholdSeconds, 30, 3600))
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
			params.BackupIntervalHours = int64(clampInt32(*input.Backup.IntervalHours, 1, 24*30))
		}
		if input.Backup.KeepLast != nil {
			params.BackupKeepLast = int64(clampInt32(*input.Backup.KeepLast, 1, 365))
		}
	}

	if input.Geocoding != nil {
		candidate := currentGeocoding
		if input.Geocoding.Provider != nil {
			candidate.Provider = *input.Geocoding.Provider
		}
		if input.Geocoding.NominatimEndpoint != nil {
			candidate.NominatimEndpoint = *input.Geocoding.NominatimEndpoint
		}
		if input.Geocoding.Language != nil {
			candidate.Language = *input.Geocoding.Language
		}
		if input.Geocoding.UserAgent != nil {
			candidate.UserAgent = *input.Geocoding.UserAgent
		}
		normalized, normalizeErr := candidate.Normalize()
		if normalizeErr != nil {
			return SystemSettings{}, fmt.Errorf("%w: %v", ErrInvalidSystemSettings, normalizeErr)
		}
		candidateGeocoding = normalized
		geocodingChanged = candidateGeocoding != currentGeocoding
		params.GeocodingProvider = candidateGeocoding.Provider
		params.GeocodingNominatimEndpoint = candidateGeocoding.NominatimEndpoint
		params.GeocodingLanguage = candidateGeocoding.Language
		params.GeocodingUserAgent = candidateGeocoding.UserAgent
		if geocodingChanged {
			params.GeocodingRevision = row.GeocodingRevision + 1
		}
	}

	candidateLLM := LLMSettings{
		AgentEnabled:     params.LlmAgentEnabled,
		Provider:         params.LlmProvider,
		ModelName:        params.LlmModelName,
		BaseURL:          params.LlmBaseUrl,
		APIKeyConfigured: params.LlmApiKeyConfigured,
	}
	if candidateLLM.AgentEnabled && !candidateLLM.IsConfigured() {
		return SystemSettings{}, errors.New("cannot enable Lumilio Agent until an explicit provider, model, and credential have been configured")
	}

	if geocodingChanged {
		return s.updateGeocodingSettings(ctx, params, currentGeocoding, candidateGeocoding)
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

func (s *settingsService) GetGeocodingConfig(ctx context.Context) (settings.Geocoding, error) {
	row, err := s.getSettingsRow(ctx)
	if err != nil {
		return settings.Geocoding{}, err
	}
	return normalizeStoredGeocoding(row)
}

func (s *settingsService) updateGeocodingSettings(
	ctx context.Context,
	params repo.UpsertSettingsParams,
	previous settings.Geocoding,
	candidate settings.Geocoding,
) (SystemSettings, error) {
	if s.db == nil || s.queue == nil {
		return SystemSettings{}, errors.New("geocoding settings runtime dependencies are not configured")
	}

	tx, err := s.writer.BeginTx(ctx, catalogtx.OperationSettingsGeocodingUpdate, nil)
	if err != nil {
		return SystemSettings{}, fmt.Errorf("begin geocoding settings update: %w", err)
	}
	defer tx.Rollback()
	qtx := s.queries.WithTx(tx.Raw())

	updated, err := qtx.UpsertSettings(ctx, params)
	if err != nil {
		return SystemSettings{}, fmt.Errorf("update settings: %w", err)
	}

	previousEnabled := previous.IsEnabled()
	candidateEnabled := candidate.IsEnabled()
	sourceChanged := previous.SourceKey() != candidate.SourceKey()
	userAgentChanged := previous.UserAgent != candidate.UserAgent
	switch {
	case !candidateEnabled && previousEnabled:
		if err := qtx.DisableUnresolvedLocationClusters(ctx); err != nil {
			return SystemSettings{}, fmt.Errorf("disable unresolved location clusters: %w", err)
		}
	case candidateEnabled && sourceChanged:
		if err := qtx.ResetLocationClustersForGeocodingSource(ctx); err != nil {
			return SystemSettings{}, fmt.Errorf("reset location clusters for geocoding source: %w", err)
		}
	case candidateEnabled && userAgentChanged:
		if err := qtx.ResetLocationClustersForGeocodingUserAgent(ctx); err != nil {
			return SystemSettings{}, fmt.Errorf("reset location clusters for geocoding user agent: %w", err)
		}
	}

	if candidateEnabled {
		args := jobs.ResolveLocationClustersArgs{GeocodingRevision: params.GeocodingRevision}
		opts := args.InsertOpts()
		if _, err := s.queue.InsertTx(ctx, tx.Raw(), args, &opts); err != nil {
			return SystemSettings{}, fmt.Errorf("enqueue location cluster resolution: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return SystemSettings{}, fmt.Errorf("commit geocoding settings update: %w", err)
	}
	return mapSystemSettings(updated), nil
}

func (s *settingsService) ValidateLLMDraft(ctx context.Context, input ValidateLLMDraftInput) error {
	cfg := settings.LLM{
		Provider:  settings.NormalizeLLMProvider(input.Provider),
		ModelName: strings.TrimSpace(input.ModelName),
		BaseURL:   strings.TrimSpace(input.BaseURL),
		APIKey:    strings.TrimSpace(input.APIKey),
	}
	if input.UseStoredAPIKey {
		stored, err := s.GetLLMConfig(ctx)
		if err != nil {
			return err
		}
		if stored.EffectiveProvider() != cfg.EffectiveProvider() || strings.TrimSpace(stored.APIKey) == "" {
			return errors.New("the stored API key does not belong to the selected provider")
		}
		cfg.APIKey = stored.APIKey
	}

	validateCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	if err := llm.ValidateChatModel(validateCtx, cfg); err != nil {
		return fmt.Errorf("validate llm draft: %w", err)
	}
	return nil
}

func (s *settingsService) seedFromDefaults(ctx context.Context) error {
	llmCfg := s.defaults.LLM
	mlCfg := s.defaults.ML
	geocodingCfg, err := s.defaults.Geocoding.Normalize()
	if err != nil {
		return fmt.Errorf("seed settings from defaults: %w", err)
	}

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
		MlVideoMaxFrames:            int64(mlCfg.EffectiveVideoMaxFrames()),
		MlVideoLongThresholdSeconds: int64(mlCfg.EffectiveVideoLongThresholdSeconds()),
		MlVideoSceneThreshold:       mlCfg.EffectiveVideoSceneThreshold(),
		// Mirror the migration's column defaults: this INSERT names the backup
		// columns explicitly, so zero values here would override them.
		BackupEnabled:              true,
		BackupIntervalHours:        24,
		BackupKeepLast:             14,
		GeocodingProvider:          geocodingCfg.Provider,
		GeocodingNominatimEndpoint: geocodingCfg.NominatimEndpoint,
		GeocodingLanguage:          geocodingCfg.Language,
		GeocodingUserAgent:         geocodingCfg.UserAgent,
		GeocodingRevision:          1,
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
			VideoMaxFrames:            int32(row.MlVideoMaxFrames),
			VideoLongThresholdSeconds: int32(row.MlVideoLongThresholdSeconds),
			VideoSceneThreshold:       row.MlVideoSceneThreshold,
		},
		Backup: BackupSettings{
			Enabled:       row.BackupEnabled,
			IntervalHours: int32(row.BackupIntervalHours),
			KeepLast:      int32(row.BackupKeepLast),
		},
		Geocoding: GeocodingSettings{
			Provider:          strings.TrimSpace(row.GeocodingProvider),
			NominatimEndpoint: strings.TrimSpace(row.GeocodingNominatimEndpoint),
			Language:          strings.TrimSpace(row.GeocodingLanguage),
			UserAgent:         strings.TrimSpace(row.GeocodingUserAgent),
		},
		UpdatedAt: updatedAt,
		UpdatedBy: row.UpdatedBy,
	}
}

func normalizeStoredGeocoding(row repo.Setting) (settings.Geocoding, error) {
	return (settings.Geocoding{
		Provider:          row.GeocodingProvider,
		NominatimEndpoint: row.GeocodingNominatimEndpoint,
		Language:          row.GeocodingLanguage,
		UserAgent:         row.GeocodingUserAgent,
	}).Normalize()
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
	provider := settings.NormalizeLLMProvider(raw)
	if !settings.IsSupportedLLMProvider(provider) {
		return ""
	}
	return provider
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
