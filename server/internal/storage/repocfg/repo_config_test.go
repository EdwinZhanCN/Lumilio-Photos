package repocfg

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepositoryConfig_SaveAndLoad(t *testing.T) {
	repoPath := t.TempDir()

	cfg := NewRepositoryConfig("Family Photos",
		WithStorageStrategy("date"),
		WithLocalSettings(true, "rename", 128*1024*1024, false, true),
		WithBackupPath("/mnt/backup"),
	)

	require.NoError(t, cfg.SaveConfigToFile(repoPath))

	loaded, err := LoadConfigFromFile(repoPath)
	require.NoError(t, err)

	assert.Equal(t, cfg.Version, loaded.Version)
	assert.Equal(t, cfg.ID, loaded.ID)
	assert.Equal(t, cfg.Name, loaded.Name)
	assert.Equal(t, cfg.StorageStrategy, loaded.StorageStrategy)
	assert.Equal(t, cfg.LocalSettings.PreserveOriginalFilename, loaded.LocalSettings.PreserveOriginalFilename)
	assert.Equal(t, cfg.LocalSettings.HandleDuplicateFilenames, loaded.LocalSettings.HandleDuplicateFilenames)
	assert.Equal(t, cfg.LocalSettings.MaxFileSize, loaded.LocalSettings.MaxFileSize)
	assert.Equal(t, cfg.LocalSettings.CompressFiles, loaded.LocalSettings.CompressFiles)
	assert.Equal(t, cfg.LocalSettings.CreateBackups, loaded.LocalSettings.CreateBackups)
	assert.Equal(t, cfg.LocalSettings.BackupPath, loaded.LocalSettings.BackupPath)
}

func TestDefaultRepositoryConfig_Template(t *testing.T) {
	cfg := DefaultRepositoryConfig()

	assert.Empty(t, cfg.ID)
	assert.Empty(t, cfg.Name)
	assert.True(t, cfg.CreatedAt.IsZero())

	assert.Equal(t, "1.0", cfg.Version)
	assert.Equal(t, "date", cfg.StorageStrategy)
	assert.Equal(t, "uuid", cfg.LocalSettings.HandleDuplicateFilenames)

	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "repository ID is required")
}

func TestNewRepositoryConfig_SystemGeneratedFields(t *testing.T) {
	cfg1 := NewRepositoryConfig("Repo A")
	cfg2 := NewRepositoryConfig("Repo B")

	assert.NotEqual(t, cfg1.ID, cfg2.ID)
	assert.NotEmpty(t, cfg1.ID)
	assert.NotEmpty(t, cfg2.ID)

	_, err1 := uuid.Parse(cfg1.ID)
	_, err2 := uuid.Parse(cfg2.ID)
	assert.NoError(t, err1)
	assert.NoError(t, err2)

	now := time.Now()
	assert.True(t, cfg1.CreatedAt.After(now.Add(-time.Minute)))
	assert.True(t, cfg2.CreatedAt.After(now.Add(-time.Minute)))
	assert.NoError(t, cfg1.Validate())
	assert.NoError(t, cfg2.Validate())
}

func TestNewRepositoryConfig_WithOptions(t *testing.T) {
	cfg := NewRepositoryConfig("Archive",
		WithStorageStrategy("cas"),
		WithLocalSettings(false, "overwrite", 50*1024*1024, true, false),
	)

	assert.Equal(t, "cas", cfg.StorageStrategy)
	assert.False(t, cfg.LocalSettings.PreserveOriginalFilename)
	assert.Equal(t, "overwrite", cfg.LocalSettings.HandleDuplicateFilenames)
	assert.Equal(t, int64(50*1024*1024), cfg.LocalSettings.MaxFileSize)
	assert.True(t, cfg.LocalSettings.CompressFiles)
	assert.False(t, cfg.LocalSettings.CreateBackups)
	assert.NoError(t, cfg.Validate())
}

func TestRepositoryConfig_ValidateFailures(t *testing.T) {
	t.Run("invalid storage strategy", func(t *testing.T) {
		cfg := NewRepositoryConfig("Invalid", WithStorageStrategy("unknown"))
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid storage strategy")
	})

	t.Run("invalid duplicate handling", func(t *testing.T) {
		cfg := NewRepositoryConfig("Invalid")
		cfg.LocalSettings.HandleDuplicateFilenames = "bad-mode"
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid handle_duplicate_filenames")
	})

	t.Run("negative max file size", func(t *testing.T) {
		cfg := NewRepositoryConfig("Invalid")
		cfg.LocalSettings.MaxFileSize = -1
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "max_file_size cannot be negative")
	})
}

func TestRepositoryConfig_WithBackupPathInYAML(t *testing.T) {
	cfg := NewRepositoryConfig("Repository with Backup",
		WithLocalSettings(true, "uuid", 0, false, true),
		WithBackupPath("/external/backup/drive"),
	)

	dir := t.TempDir()
	require.NoError(t, cfg.SaveConfigToFile(dir))

	configPath := filepath.Join(dir, ".lumiliorepo")
	content, err := os.ReadFile(configPath)
	require.NoError(t, err)

	assert.Contains(t, string(content), "backup_path: /external/backup/drive")
	assert.Contains(t, string(content), "create_backups: true")
}

func TestIsRepositoryRoot(t *testing.T) {
	dir := t.TempDir()
	assert.False(t, IsRepositoryRoot(dir))

	cfg := NewRepositoryConfig("Root Test")
	require.NoError(t, cfg.SaveConfigToFile(dir))
	assert.True(t, IsRepositoryRoot(dir))
}
