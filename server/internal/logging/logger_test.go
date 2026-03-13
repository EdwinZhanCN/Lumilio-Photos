package logging

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestNewLoggerRoutesApplicationAndErrorLogs(t *testing.T) {
	logDir := t.TempDir()
	runtime, err := NewLogger(Config{
		Level:         "debug",
		LogDir:        logDir,
		ConsoleFormat: "console",
		FileFormat:    "json",
		Development:   true,
	})
	require.NoError(t, err)
	defer runtime.Sync()

	logger := runtime.Named("app")
	logger.Info("info message", zap.String("operation", "test.info"))
	logger.Warn("warn message", zap.String("operation", "test.warn"))

	require.NoError(t, runtime.Sync())

	appBytes, err := os.ReadFile(filepath.Join(logDir, globalApplicationLog))
	require.NoError(t, err)
	errorBytes, err := os.ReadFile(filepath.Join(logDir, globalErrorLog))
	require.NoError(t, err)

	appText := string(appBytes)
	errorText := string(errorBytes)
	assert.Contains(t, appText, "info message")
	assert.Contains(t, appText, "warn message")
	assert.Contains(t, errorText, "warn message")
	assert.NotContains(t, errorText, "info message")
	assert.Contains(t, appText, "\"component\":\"app\"")
}

func TestRiverLoggerBridgesSlogToZap(t *testing.T) {
	logDir := t.TempDir()
	runtime, err := NewLogger(Config{
		Level:         "debug",
		LogDir:        logDir,
		ConsoleFormat: "console",
		FileFormat:    "json",
		Development:   true,
	})
	require.NoError(t, err)
	defer runtime.Sync()

	riverLogger := runtime.RiverLogger()
	riverLogger.Warn("river warning", slog.String("job_kind", "process_clip"), slog.Int64("job_id", 42))

	require.NoError(t, runtime.Sync())

	errorBytes, err := os.ReadFile(filepath.Join(logDir, globalErrorLog))
	require.NoError(t, err)
	errorText := string(errorBytes)
	assert.Contains(t, errorText, "river warning")
	assert.Contains(t, errorText, "\"component\":\"river\"")
	assert.Contains(t, errorText, "\"job_kind\":\"process_clip\"")
	assert.Contains(t, errorText, "\"job_id\":42")
}

func TestRepositoryAuditProviderCachesLoggersAndNoopsOutsideRepo(t *testing.T) {
	provider := NewRepositoryAuditProvider(zap.NewNop()).(*repositoryAuditProvider)

	nonRepoPath := t.TempDir()
	provider.ForPath(nonRepoPath).Operation("should_not_write")
	_, err := os.Stat(filepath.Join(nonRepoPath, repositoryLogsDir))
	assert.Error(t, err)

	repoPath := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(repoPath, repositoryLogsDir), 0755))

	first := provider.ForPath(repoPath)
	second := provider.ForPath(repoPath)
	firstConcrete, ok := first.(*repositoryAuditLogger)
	require.True(t, ok)
	secondConcrete, ok := second.(*repositoryAuditLogger)
	require.True(t, ok)
	assert.Same(t, firstConcrete, secondConcrete)

	first.Operation("asset.ingest", zap.String("asset_id", "asset-1"))
	first.Error("asset.ingest", assert.AnError, zap.String("asset_id", "asset-1"))

	opsBytes, err := os.ReadFile(filepath.Join(repoPath, repositoryLogsDir, repositoryOpsLogName))
	require.NoError(t, err)
	errBytes, err := os.ReadFile(filepath.Join(repoPath, repositoryLogsDir, repositoryErrLogName))
	require.NoError(t, err)

	assert.Contains(t, string(opsBytes), "\"operation\":\"asset.ingest\"")
	assert.Contains(t, string(opsBytes), "\"asset_id\":\"asset-1\"")
	assert.Contains(t, string(errBytes), "\"result\":\"error\"")
	assert.True(t, strings.Contains(string(errBytes), "assert.AnError") || strings.Contains(string(errBytes), "assert.AnError general error for testing"))
}
