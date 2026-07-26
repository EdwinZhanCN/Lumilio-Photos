package handler

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"server/config"
	"server/internal/db"

	"github.com/stretchr/testify/require"
)

func TestQueueHandlerLoadQueueSummariesRunsAgainstMigratedSQLiteSchema(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	directory := t.TempDir()
	require.NoError(t, os.Chmod(directory, 0o700))
	database, err := db.Open(ctx, config.DatabaseConfig{
		Path: filepath.Join(directory, "queue-handler.sqlite3"),
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, database.Close(context.Background()))
	})
	require.NoError(t, database.Migrate(ctx))

	summaries, err := NewQueueHandler(database.SQL).loadQueueSummaries(ctx)
	require.NoError(t, err)
	require.Empty(t, summaries)
}
