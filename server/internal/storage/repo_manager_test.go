package storage

import (
	"os"
	"path/filepath"
	"server/internal/storage/repocfg"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateRepository(t *testing.T) {
	manager := NewRepositoryManager(nil) // Using nil for tests since we're not testing DB operations

	t.Run("valid repository", func(t *testing.T) {
		testDir := t.TempDir()

		// Create a valid repository using directory manager
		dirManager := NewDirectoryManager()
		err := dirManager.CreateStructure(testDir)
		require.NoError(t, err)

		// Create config file
		config := repocfg.NewRepositoryConfig("Valid Test Repo")
		err = config.SaveConfigToFile(testDir)
		require.NoError(t, err)

		result, err := manager.ValidateRepository(testDir)
		require.NoError(t, err)
		assert.True(t, result.Valid)
		assert.Empty(t, result.Errors)
	})

	t.Run("missing config file", func(t *testing.T) {
		testDir := t.TempDir()

		result, err := manager.ValidateRepository(testDir)
		require.NoError(t, err)
		assert.False(t, result.Valid)
		assert.Contains(t, result.Errors[0], "Missing .lumiliorepo configuration file")
	})

	t.Run("invalid config", func(t *testing.T) {
		testDir := t.TempDir()

		// Create invalid config file
		configPath := filepath.Join(testDir, ".lumiliorepo")
		err := os.WriteFile(configPath, []byte("invalid yaml: ["), 0644)
		require.NoError(t, err)

		result, err := manager.ValidateRepository(testDir)
		require.NoError(t, err)
		assert.False(t, result.Valid)
		assert.Contains(t, result.Errors[0], "Invalid configuration")
	})

	t.Run("nonexistent directory", func(t *testing.T) {
		result, err := manager.ValidateRepository("/nonexistent/path")
		require.NoError(t, err)
		assert.False(t, result.Valid)
		assert.Contains(t, result.Errors[0], "Repository directory does not exist")
	})
}

func TestIsNestedRepository(t *testing.T) {
	manager := NewRepositoryManager(nil) // Using nil for tests since we're not testing DB operations
	testDir := t.TempDir()

	// Create parent repository
	parentRepo := filepath.Join(testDir, "parent")
	err := os.MkdirAll(parentRepo, 0755)
	require.NoError(t, err)

	config := repocfg.NewRepositoryConfig("Parent Repo")
	err = config.SaveConfigToFile(parentRepo)
	require.NoError(t, err)

	// Test nested path
	nestedPath := filepath.Join(parentRepo, "subdir", "nested")
	err = os.MkdirAll(nestedPath, 0755)
	require.NoError(t, err)

	isNested, parentPath, err := manager.IsNestedRepository(nestedPath)
	require.NoError(t, err)
	assert.True(t, isNested)
	assert.Equal(t, parentRepo, parentPath)

	// Test non-nested path
	nonNestedPath := filepath.Join(testDir, "separate")
	err = os.MkdirAll(nonNestedPath, 0755)
	require.NoError(t, err)

	isNested, _, err = manager.IsNestedRepository(nonNestedPath)
	require.NoError(t, err)
	assert.False(t, isNested)
}

func TestRepositoryWorkflow_Integration(t *testing.T) {
	manager := NewRepositoryManager(nil) // Using nil for tests since we're not testing DB operations
	dirManager := NewDirectoryManager()
	testRoot := t.TempDir()

	// Create multiple test repositories
	repos := []struct {
		name   string
		path   string
		config *repocfg.RepositoryConfig
	}{
		{
			name: "photos-2024",
			path: filepath.Join(testRoot, "photos-2024"),
			config: func() *repocfg.RepositoryConfig {
				config := repocfg.NewRepositoryConfig("Family Photos 2024")
				config.SyncSettings.QuickScanInterval = "2m"
				config.SyncSettings.IgnorePatterns = []string{".DS_Store", "*.tmp"}
				config.LocalSettings.MaxFileSize = 104857600 // 100MB
				return config
			}(),
		},
		{
			name:   "vacation-pics",
			path:   filepath.Join(testRoot, "vacation", "pics"),
			config: repocfg.NewRepositoryConfig("Vacation Pictures"),
		},
	}

	// Set up repositories
	for _, repo := range repos {
		err := os.MkdirAll(repo.path, 0755)
		require.NoError(t, err)

		// Create directory structure using directory manager
		err = dirManager.CreateStructure(repo.path)
		require.NoError(t, err)

		// Save config file
		err = repo.config.SaveConfigToFile(repo.path)
		require.NoError(t, err)

		t.Logf("Created repository: %s at %s", repo.config.Name, repo.path)
	}

	// Test adding repositories individually - skip with nil queries
	t.Run("add repositories", func(t *testing.T) {
		t.Skip("Skipping database operations test with nil queries")
	})

	// Test validation of each repository
	t.Run("validate repositories", func(t *testing.T) {
		for _, repo := range repos {
			result, err := manager.ValidateRepository(repo.path)
			require.NoError(t, err)

			t.Logf("Validation for %s: Valid=%v, Errors=%d, Warnings=%d",
				repo.config.Name, result.Valid, len(result.Errors), len(result.Warnings))

			if len(result.Errors) > 0 {
				t.Logf("  Errors: %v", result.Errors)
			}
			if len(result.Warnings) > 0 {
				t.Logf("  Warnings: %v", result.Warnings)
			}

			// Should be valid despite missing some directories (they'll be warnings)
			assert.True(t, result.Valid)
		}
	})

	// Test nested repository detection
	t.Run("detect nested repository", func(t *testing.T) {
		nestedPath := filepath.Join(repos[0].path, "nested-attempt")
		err := os.MkdirAll(nestedPath, 0755)
		require.NoError(t, err)

		isNested, parentPath, err := manager.IsNestedRepository(nestedPath)
		require.NoError(t, err)
		assert.True(t, isNested)
		assert.Equal(t, repos[0].path, parentPath)

		t.Logf("Nested check: %s is nested inside %s", nestedPath, parentPath)
	})
}
