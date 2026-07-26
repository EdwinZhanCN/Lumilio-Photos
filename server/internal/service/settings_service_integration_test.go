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
