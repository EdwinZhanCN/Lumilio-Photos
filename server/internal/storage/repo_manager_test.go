package storage

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateRepository(t *testing.T) {
	manager := NewRepositoryManager(nil) // Using nil for tests since we're not testing DB operations

	t.Run("valid repository", func(t *testing.T) {
		testDir := t.TempDir()

		// Create a valid repository
		config := NewRepositoryConfig("Valid Test Repo")
		err := config.SaveConfigToFile(testDir)
		require.NoError(t, err)

		// Create some required directories
		requiredDirs := []string{
			".lumilio",
			".lumilio/assets",
			"inbox",
		}
		for _, dir := range requiredDirs {
			err := os.MkdirAll(filepath.Join(testDir, dir), 0755)
			require.NoError(t, err)
		}

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

	config := NewRepositoryConfig("Parent Repo")
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
	testRoot := t.TempDir()

	// Create multiple test repositories
	repos := []struct {
		name   string
		path   string
		config *RepositoryConfig
	}{
		{
			name: "photos-2024",
			path: filepath.Join(testRoot, "photos-2024"),
			config: func() *RepositoryConfig {
				config := NewRepositoryConfig("Family Photos 2024")
				config.SyncSettings.QuickScanInterval = "2m"
				config.SyncSettings.IgnorePatterns = []string{".DS_Store", "*.tmp"}
				config.LocalSettings.MaxFileSize = 104857600 // 100MB
				return config
			}(),
		},
		{
			name:   "vacation-pics",
			path:   filepath.Join(testRoot, "vacation", "pics"),
			config: NewRepositoryConfig("Vacation Pictures"),
		},
	}

	// Set up repositories
	for _, repo := range repos {
		err := os.MkdirAll(repo.path, 0755)
		require.NoError(t, err)

		// Config is already set up properly by NewRepositoryConfig

		err = repo.config.SaveConfigToFile(repo.path)
		require.NoError(t, err)

		t.Logf("Created repository: %s at %s", repo.config.Name, repo.path)
	}

	// Test adding repositories individually
	t.Run("add repositories", func(t *testing.T) {
		for _, repo := range repos {
			addedRepo, err := manager.AddRepository(repo.path)
			require.NoError(t, err)
			assert.Equal(t, repo.config.Name, addedRepo.Name)
			assert.Equal(t, repo.path, addedRepo.Path)
			assert.Equal(t, "active", *addedRepo.Status)
			t.Logf("Added: %s at %s (ID: %s)", addedRepo.Name, addedRepo.Path, addedRepo.RepoID.Bytes)
		}
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
