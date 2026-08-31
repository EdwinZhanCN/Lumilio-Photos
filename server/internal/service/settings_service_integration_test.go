package service

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"server/config"
	"server/internal/db"
	runtimesettings "server/internal/settings"

	"github.com/stretchr/testify/require"
)

func TestSettingsServiceUpdatesFreshSQLiteCatalog(t *testing.T) {
	ctx := context.Background()
	catalogDir := filepath.Join(t.TempDir(), "private")
	require.NoError(t, os.Mkdir(catalogDir, 0o700))

	database, err := db.Open(ctx, config.DatabaseConfig{
		Path: filepath.Join(catalogDir, "settings.sqlite3"),
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, database.Close(context.Background()))
	})
	require.NoError(t, database.Migrate(ctx))

	service := NewSettingsService(
		database.Queries,
		runtimesettings.Settings{},
		filepath.Join(catalogDir, "lumilio_secret_key"),
	)
	semanticEnabled := true
	videoSemanticEnabled := true
	updated, err := service.UpdateSystemSettings(ctx, UpdateSystemSettingsInput{
		ML: &UpdateMLSettingsInput{
			SemanticEnabled:      &semanticEnabled,
			VideoSemanticEnabled: &videoSemanticEnabled,
		},
	})
	require.NoError(t, err)
	require.True(t, updated.ML.SemanticEnabled)
	require.True(t, updated.ML.VideoSemanticEnabled)

	row, err := database.Queries.GetSettings(ctx)
	require.NoError(t, err)
	require.True(t, row.MlSemanticEnabled)
	require.True(t, row.MlVideoSemanticEnabled)
}

func TestSettingsServiceDoesNotReuseCredentialAcrossProviders(t *testing.T) {
	ctx := context.Background()
	catalogDir := filepath.Join(t.TempDir(), "private")
	require.NoError(t, os.Mkdir(catalogDir, 0o700))

	database, err := db.Open(ctx, config.DatabaseConfig{Path: filepath.Join(catalogDir, "settings.sqlite3")})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, database.Close(context.Background())) })
	require.NoError(t, database.Migrate(ctx))

	settingsService := NewSettingsService(
		database.Queries,
		runtimesettings.Settings{},
		filepath.Join(catalogDir, "lumilio_secret_key"),
	)
	provider := "openai"
	model := "gpt-4.1-mini"
	key := "openai-secret"
	_, err = settingsService.UpdateSystemSettings(ctx, UpdateSystemSettingsInput{LLM: &UpdateLLMSettingsInput{
		Provider:  &provider,
		ModelName: &model,
		APIKey:    &key,
	}})
	require.NoError(t, err)

	provider = "deepseek"
	updated, err := settingsService.UpdateSystemSettings(ctx, UpdateSystemSettingsInput{LLM: &UpdateLLMSettingsInput{
		Provider: &provider,
	}})
	require.NoError(t, err)
	require.Equal(t, "deepseek", updated.LLM.Provider)
	require.False(t, updated.LLM.APIKeyConfigured)

	config, err := settingsService.GetLLMConfig(ctx)
	require.NoError(t, err)
	require.Empty(t, config.APIKey)

	provider = "ollama"
	baseURL := "http://127.0.0.1:11434"
	updated, err = settingsService.UpdateSystemSettings(ctx, UpdateSystemSettingsInput{LLM: &UpdateLLMSettingsInput{
		Provider: &provider,
		BaseURL:  &baseURL,
	}})
	require.NoError(t, err)
	require.Equal(t, "ollama", updated.LLM.Provider)
	require.False(t, updated.LLM.APIKeyConfigured)
	require.True(t, updated.LLM.IsConfigured())

	config, err = settingsService.GetLLMConfig(ctx)
	require.NoError(t, err)
	require.Empty(t, config.APIKey)

	unknownProvider := "future-provider"
	_, err = settingsService.UpdateSystemSettings(ctx, UpdateSystemSettingsInput{LLM: &UpdateLLMSettingsInput{
		Provider: &unknownProvider,
	}})
	require.ErrorContains(t, err, "unsupported llm provider")
	persisted, err := settingsService.GetSystemSettings(ctx)
	require.NoError(t, err)
	require.Equal(t, "ollama", persisted.LLM.Provider)
	require.ErrorContains(t, settingsService.ValidateLLMDraft(ctx, ValidateLLMDraftInput{
		Provider:  unknownProvider,
		ModelName: "fixture-model",
		APIKey:    "fixture-secret",
	}), "unsupported llm provider")
}

func TestSettingsServiceRejectsEnablingIncompleteAgentConfiguration(t *testing.T) {
	ctx := context.Background()
	catalogDir := filepath.Join(t.TempDir(), "private")
	require.NoError(t, os.Mkdir(catalogDir, 0o700))

	database, err := db.Open(ctx, config.DatabaseConfig{Path: filepath.Join(catalogDir, "settings.sqlite3")})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, database.Close(context.Background())) })
	require.NoError(t, database.Migrate(ctx))

	settingsService := NewSettingsService(
		database.Queries,
		runtimesettings.Settings{},
		filepath.Join(catalogDir, "lumilio_secret_key"),
	)
	enabled := true
	provider := "openai"
	model := "gpt-4.1-mini"
	_, err = settingsService.UpdateSystemSettings(ctx, UpdateSystemSettingsInput{LLM: &UpdateLLMSettingsInput{
		AgentEnabled: &enabled,
		Provider:     &provider,
		ModelName:    &model,
	}})
	require.ErrorContains(t, err, "cannot enable Lumilio Agent")

	persisted, err := settingsService.GetSystemSettings(ctx)
	require.NoError(t, err)
	require.False(t, persisted.LLM.AgentEnabled)
}

func TestSettingsServiceClearsAgentFieldsWhenProviderIsUnset(t *testing.T) {
	ctx := context.Background()
	catalogDir := filepath.Join(t.TempDir(), "private")
	require.NoError(t, os.Mkdir(catalogDir, 0o700))

	database, err := db.Open(ctx, config.DatabaseConfig{Path: filepath.Join(catalogDir, "settings.sqlite3")})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, database.Close(context.Background())) })
	require.NoError(t, database.Migrate(ctx))

	settingsService := NewSettingsService(
		database.Queries,
		runtimesettings.Settings{},
		filepath.Join(catalogDir, "lumilio_secret_key"),
	)
	provider := "openai"
	model := "gpt-4.1-mini"
	baseURL := "https://api.openai.com/v1"
	key := "secret"
	_, err = settingsService.UpdateSystemSettings(ctx, UpdateSystemSettingsInput{LLM: &UpdateLLMSettingsInput{
		Provider:  &provider,
		ModelName: &model,
		BaseURL:   &baseURL,
		APIKey:    &key,
	}})
	require.NoError(t, err)

	provider = "none"
	enabled := true
	updated, err := settingsService.UpdateSystemSettings(ctx, UpdateSystemSettingsInput{LLM: &UpdateLLMSettingsInput{
		Provider:     &provider,
		AgentEnabled: &enabled,
		ModelName:    &model,
		BaseURL:      &baseURL,
		APIKey:       &key,
	}})
	require.NoError(t, err)
	require.False(t, updated.LLM.AgentEnabled)
	require.Empty(t, updated.LLM.Provider)
	require.Empty(t, updated.LLM.ModelName)
	require.Empty(t, updated.LLM.BaseURL)
	require.False(t, updated.LLM.APIKeyConfigured)
}

func TestSettingsServiceGeocodingTransitionsAndAtomicEnqueue(t *testing.T) {
	ctx := context.Background()
	catalogDir := filepath.Join(t.TempDir(), "private")
	require.NoError(t, os.Mkdir(catalogDir, 0o700))

	database, err := db.Open(ctx, config.DatabaseConfig{Path: filepath.Join(catalogDir, "settings.sqlite3")})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, database.Close(context.Background())) })
	require.NoError(t, database.Migrate(ctx))

	settingsService := NewSettingsServiceWithRuntime(
		database.Queries,
		runtimesettings.Default("production"),
		filepath.Join(catalogDir, "lumilio_secret_key"),
		SettingsRuntime{DB: database.SQL, Writer: database.Writer},
	)

	row, err := database.Queries.GetSettings(ctx)
	require.NoError(t, err)
	require.Equal(t, runtimesettings.GeocodingProviderDisabled, row.GeocodingProvider)
	require.Equal(t, runtimesettings.DefaultGeocodingEndpoint, row.GeocodingNominatimEndpoint)
	require.Equal(t, int64(1), row.GeocodingRevision)

	provider := " NOMINATIM "
	endpoint := "HTTP://127.0.0.1:8080/reverse"
	language := " ZH "
	userAgent := " Test-Agent/1.0 "
	updated, err := settingsService.UpdateSystemSettings(ctx, UpdateSystemSettingsInput{
		Geocoding: &UpdateGeocodingSettingsInput{
			Provider:          &provider,
			NominatimEndpoint: &endpoint,
			Language:          &language,
			UserAgent:         &userAgent,
		},
	})
	require.NoError(t, err)
	require.Equal(t, "nominatim", updated.Geocoding.Provider)
	require.Equal(t, "http://127.0.0.1:8080/reverse", updated.Geocoding.NominatimEndpoint)
	require.Equal(t, "zh", updated.Geocoding.Language)
	require.Equal(t, "Test-Agent/1.0", updated.Geocoding.UserAgent)
	assertSettingsProjectionRequestCount(t, database.SQL, 1)

	require.NoError(t, err)
	row, err = database.Queries.GetSettings(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(2), row.GeocodingRevision)

	// Formatting-only changes normalize to the same aggregate and do not
	// enqueue another resolver revision.
	provider = "nominatim"
	endpoint = "http://127.0.0.1:8080/reverse"
	language = "zh"
	userAgent = "Test-Agent/1.0"
	_, err = settingsService.UpdateSystemSettings(ctx, UpdateSystemSettingsInput{
		Geocoding: &UpdateGeocodingSettingsInput{
			Provider:          &provider,
			NominatimEndpoint: &endpoint,
			Language:          &language,
			UserAgent:         &userAgent,
		},
	})
	require.NoError(t, err)
	row, err = database.Queries.GetSettings(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(2), row.GeocodingRevision)

	userAgent = "Test-Agent/2.0"
	_, err = settingsService.UpdateSystemSettings(ctx, UpdateSystemSettingsInput{
		Geocoding: &UpdateGeocodingSettingsInput{UserAgent: &userAgent},
	})
	require.NoError(t, err)
	assertSettingsProjectionRequestCount(t, database.SQL, 2)

	provider = "disabled"
	_, err = settingsService.UpdateSystemSettings(ctx, UpdateSystemSettingsInput{
		Geocoding: &UpdateGeocodingSettingsInput{Provider: &provider},
	})
	require.NoError(t, err)
	row, err = database.Queries.GetSettings(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(4), row.GeocodingRevision)
	require.Equal(t, "disabled", row.GeocodingProvider)

	invalidEndpoint := "not-a-url"
	_, err = settingsService.UpdateSystemSettings(ctx, UpdateSystemSettingsInput{
		Geocoding: &UpdateGeocodingSettingsInput{NominatimEndpoint: &invalidEndpoint},
	})
	require.ErrorIs(t, err, ErrInvalidSystemSettings)
	rowAfterInvalid, err := database.Queries.GetSettings(ctx)
	require.NoError(t, err)
	require.Equal(t, row.GeocodingRevision, rowAfterInvalid.GeocodingRevision)
	require.Equal(t, row.GeocodingNominatimEndpoint, rowAfterInvalid.GeocodingNominatimEndpoint)

	provider = "nominatim"
	_, err = settingsService.UpdateSystemSettings(ctx, UpdateSystemSettingsInput{
		Geocoding: &UpdateGeocodingSettingsInput{Provider: &provider},
	})
	require.NoError(t, err)
	rowAfterQueueFailure, err := database.Queries.GetSettings(ctx)
	require.NoError(t, err)
	require.Equal(t, "nominatim", rowAfterQueueFailure.GeocodingProvider)
	require.Equal(t, int64(5), rowAfterQueueFailure.GeocodingRevision)
	assertSettingsProjectionRequestCount(t, database.SQL, 3)
}

func assertSettingsProjectionRequestCount(t *testing.T, database *sql.DB, want int) {
	t.Helper()
	var got int
	require.NoError(t, database.QueryRow(`SELECT count(*) FROM domain_outbox WHERE command_kind = 'projection.location' AND subject_key = 'all'`).Scan(&got))
	require.Equal(t, want, got)
}
