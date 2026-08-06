package service

import (
	"context"
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
