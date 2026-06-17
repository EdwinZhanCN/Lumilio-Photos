package repocfg

import (
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
		WithLocalSettings("rename"),
	)

	require.NoError(t, cfg.SaveConfigToFile(repoPath))

	loaded, err := LoadConfigFromFile(repoPath)
	require.NoError(t, err)

	assert.Equal(t, cfg.Version, loaded.Version)
	assert.Equal(t, cfg.ID, loaded.ID)
	assert.Equal(t, cfg.Name, loaded.Name)
	assert.Equal(t, cfg.StorageStrategy, loaded.StorageStrategy)
	assert.Equal(t, cfg.LocalSettings.HandleDuplicateFilenames, loaded.LocalSettings.HandleDuplicateFilenames)
	assert.Equal(t, cfg.LocalSettings.HandleDuplicateFilenames, loaded.LocalSettings.HandleDuplicateFilenames)
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
		WithLocalSettings("overwrite"),
	)

	assert.Equal(t, "cas", cfg.StorageStrategy)
	assert.Equal(t, "overwrite", cfg.LocalSettings.HandleDuplicateFilenames)
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
}

func TestIsRepositoryRoot(t *testing.T) {
	dir := t.TempDir()
	assert.False(t, IsRepositoryRoot(dir))

	cfg := NewRepositoryConfig("Root Test")
	require.NoError(t, cfg.SaveConfigToFile(dir))
	assert.True(t, IsRepositoryRoot(dir))
}
