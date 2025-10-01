package storage

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateYAMLConfig_MinimalTest(t *testing.T) {
	// Test with a designated path you can inspect
	testPath := "/tmp/lumilio-test-repo"

	// Clean up any existing test directory
	os.RemoveAll(testPath)
	defer os.RemoveAll(testPath)

	// Create the test directory
	err := os.MkdirAll(testPath, 0755)
	require.NoError(t, err)

	// Create a sample config
	config := &RepositoryConfig{
		Version:         "1.0",
		ID:              uuid.New().String(),
		Name:            "Test Family Photos",
		CreatedAt:       time.Now(),
		StorageStrategy: "date",
		SyncSettings: SyncSettings{
			QuickScanInterval: "2m",
			FullScanInterval:  "30m",
			IgnorePatterns:    []string{".DS_Store", "Thumbs.db", "*.tmp", ".lumilio"},
		},
		LocalSettings: LocalSettings{
			PreserveOriginalFilename: true,
			HandleDuplicateFilenames: "uuid",
			MaxFileSize:              104857600, // 100MB
			CompressFiles:            false,
			CreateBackups:            true,
		},
	}

	// Save the config to the designated path
	err = config.SaveConfigToFile(testPath)
	require.NoError(t, err)

	// Verify the file exists
	configPath := filepath.Join(testPath, ".lumiliorepo")
	_, err = os.Stat(configPath)
	assert.NoError(t, err, "Config file should exist at %s", configPath)

	// Read and verify the content
	content, err := os.ReadFile(configPath)
	require.NoError(t, err)

	t.Logf("YAML config created at: %s", configPath)
	t.Logf("Config content:\n%s", string(content))

	// Load the config back and verify it matches
	loadedConfig, err := LoadConfigFromFile(testPath)
	require.NoError(t, err)

	assert.Equal(t, config.Version, loadedConfig.Version)
	assert.Equal(t, config.ID, loadedConfig.ID)
	assert.Equal(t, config.Name, loadedConfig.Name)
	assert.Equal(t, config.StorageStrategy, loadedConfig.StorageStrategy)
	assert.Equal(t, config.SyncSettings.QuickScanInterval, loadedConfig.SyncSettings.QuickScanInterval)
	assert.Equal(t, config.SyncSettings.FullScanInterval, loadedConfig.SyncSettings.FullScanInterval)
	assert.Equal(t, config.SyncSettings.IgnorePatterns, loadedConfig.SyncSettings.IgnorePatterns)
	assert.Equal(t, config.LocalSettings.PreserveOriginalFilename, loadedConfig.LocalSettings.PreserveOriginalFilename)
	assert.Equal(t, config.LocalSettings.HandleDuplicateFilenames, loadedConfig.LocalSettings.HandleDuplicateFilenames)
	assert.Equal(t, config.LocalSettings.MaxFileSize, loadedConfig.LocalSettings.MaxFileSize)
	assert.Equal(t, config.LocalSettings.CompressFiles, loadedConfig.LocalSettings.CompressFiles)
	assert.Equal(t, config.LocalSettings.CreateBackups, loadedConfig.LocalSettings.CreateBackups)

	t.Logf("✅ Config successfully saved and loaded from %s", testPath)
}

func TestCustomPath_CreateConfig(t *testing.T) {
	// You can change this path to anywhere you want to inspect the file
	customPath := "/tmp/my-lumilio-repo-test"

	// Clean up
	os.RemoveAll(customPath)
	defer os.RemoveAll(customPath)

	// Create directory
	err := os.MkdirAll(customPath, 0755)
	require.NoError(t, err)

	// Create a simple config
	config := DefaultRepositoryConfig()
	config.ID = "my-test-repo-" + uuid.New().String()[:8]
	config.Name = "My Custom Test Repository"
	config.CreatedAt = time.Now()

	// Save it
	err = config.SaveConfigToFile(customPath)
	require.NoError(t, err)

	configPath := filepath.Join(customPath, ".lumiliorepo")
	t.Logf("🎯 Custom config created at: %s", configPath)

	// Print the content so you can see it in test output
	content, err := os.ReadFile(configPath)
	require.NoError(t, err)
	t.Logf("📄 YAML Content:\n%s", string(content))

	// Basic validation
	assert.True(t, IsRepositoryRoot(customPath))

	loadedConfig, err := LoadConfigFromFile(customPath)
	require.NoError(t, err)
	assert.Equal(t, config.Name, loadedConfig.Name)
}

func TestDefaultIgnorePatterns(t *testing.T) {
	// Test that default config includes comprehensive ignore patterns
	config := DefaultRepositoryConfig()

	t.Logf("📝 Default ignore patterns (%d total):", len(config.SyncSettings.IgnorePatterns))
	for i, pattern := range config.SyncSettings.IgnorePatterns {
		t.Logf("  %2d. %s", i+1, pattern)
	}

	// Verify key patterns are included
	patterns := config.SyncSettings.IgnorePatterns
	expectedPatterns := []string{
		".DS_Store",
		"Thumbs.db",
		"*.tmp",
		"*.temp",
		".lumilio",
		"desktop.ini",
		"npm-debug.log",
		"yarn-error.log",
	}

	for _, expected := range expectedPatterns {
		assert.Contains(t, patterns, expected, "Expected pattern '%s' should be in default ignore list", expected)
	}

	// Test that the patterns are actually used when creating config files
	testDir := t.TempDir()
	// Use NewRepositoryConfig instead of manually setting fields
	config = NewRepositoryConfig("Test Default Patterns")

	err := config.SaveConfigToFile(testDir)
	require.NoError(t, err)

	// Read back and verify
	loadedConfig, err := LoadConfigFromFile(testDir)
	require.NoError(t, err)
	assert.Equal(t, len(DefaultIgnorePatterns), len(loadedConfig.SyncSettings.IgnorePatterns))
	assert.Contains(t, loadedConfig.SyncSettings.IgnorePatterns, ".DS_Store")
	assert.Contains(t, loadedConfig.SyncSettings.IgnorePatterns, "yarn-error.log")
}

func TestNewRepositoryConfig(t *testing.T) {
	// Test that NewRepositoryConfig creates a complete, valid configuration
	config := NewRepositoryConfig("My New Repository")

	// Should have all required fields set
	assert.NotEmpty(t, config.ID)
	assert.Equal(t, "My New Repository", config.Name)
	assert.False(t, config.CreatedAt.IsZero())

	// Should include default values
	assert.Equal(t, "1.0", config.Version)
	assert.Equal(t, "date", config.StorageStrategy)
	assert.Equal(t, "5m", config.SyncSettings.QuickScanInterval)

	// Should be valid
	err := config.Validate()
	assert.NoError(t, err)

	// ID should be a valid UUID
	_, err = uuid.Parse(config.ID)
	assert.NoError(t, err)

	t.Logf("✅ Created valid config: ID=%s, Name=%s", config.ID[:8], config.Name)
}

func TestDefaultRepositoryConfig_Template(t *testing.T) {
	// Test that DefaultRepositoryConfig is a template without unique fields
	config := DefaultRepositoryConfig()

	// Should NOT have unique fields set (these are set by NewRepositoryConfig)
	assert.Empty(t, config.ID)
	assert.Empty(t, config.Name)
	assert.True(t, config.CreatedAt.IsZero())

	// Should have default values
	assert.Equal(t, "1.0", config.Version)
	assert.Equal(t, "date", config.StorageStrategy)
	assert.Equal(t, len(DefaultIgnorePatterns), len(config.SyncSettings.IgnorePatterns))

	// Should NOT be valid (missing required fields)
	err := config.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "repository ID is required")

	t.Logf("✅ Default config is properly a template (invalid without ID/Name)")
}

func TestNewRepositoryConfig_WithOptions(t *testing.T) {
	// Test basic usage (same as before)
	config := NewRepositoryConfig("Basic Repository")
	assert.Equal(t, "Basic Repository", config.Name)
	assert.Equal(t, "date", config.StorageStrategy) // default
	assert.NoError(t, config.Validate())

	// Test with storage strategy option
	configCAS := NewRepositoryConfig("CAS Repository",
		WithStorageStrategy("cas"))
	assert.Equal(t, "CAS Repository", configCAS.Name)
	assert.Equal(t, "cas", configCAS.StorageStrategy)
	assert.NoError(t, configCAS.Validate())

	// Test with sync settings
	configSync := NewRepositoryConfig("Fast Sync Repository",
		WithSyncSettings("1m", "15m", []string{".DS_Store", "*.log"}))
	assert.Equal(t, "1m", configSync.SyncSettings.QuickScanInterval)
	assert.Equal(t, "15m", configSync.SyncSettings.FullScanInterval)
	assert.Equal(t, []string{".DS_Store", "*.log"}, configSync.SyncSettings.IgnorePatterns)
	assert.NoError(t, configSync.Validate())

	// Test with local settings
	configLocal := NewRepositoryConfig("Custom Local Repository",
		WithLocalSettings(false, "rename", 52428800, true, true))
	assert.False(t, configLocal.LocalSettings.PreserveOriginalFilename)
	assert.Equal(t, "rename", configLocal.LocalSettings.HandleDuplicateFilenames)
	assert.Equal(t, int64(52428800), configLocal.LocalSettings.MaxFileSize) // 50MB
	assert.True(t, configLocal.LocalSettings.CompressFiles)
	assert.True(t, configLocal.LocalSettings.CreateBackups)
	assert.NoError(t, configLocal.Validate())

	// Test with multiple options
	configMultiple := NewRepositoryConfig("Fully Customized Repository",
		WithStorageStrategy("flat"),
		WithSyncSettings("2m", "1h", []string{"*.tmp"}),
		WithLocalSettings(true, "uuid", 0, false, true))

	assert.Equal(t, "Fully Customized Repository", configMultiple.Name)
	assert.Equal(t, "flat", configMultiple.StorageStrategy)
	assert.Equal(t, "2m", configMultiple.SyncSettings.QuickScanInterval)
	assert.Equal(t, "1h", configMultiple.SyncSettings.FullScanInterval)
	assert.Equal(t, []string{"*.tmp"}, configMultiple.SyncSettings.IgnorePatterns)
	assert.True(t, configMultiple.LocalSettings.PreserveOriginalFilename)
	assert.Equal(t, "uuid", configMultiple.LocalSettings.HandleDuplicateFilenames)
	assert.Equal(t, int64(0), configMultiple.LocalSettings.MaxFileSize)
	assert.False(t, configMultiple.LocalSettings.CompressFiles)
	assert.True(t, configMultiple.LocalSettings.CreateBackups)
	assert.NoError(t, configMultiple.Validate())

	t.Logf("✅ Flexible config creation with options works correctly")
}

func TestNewDefaultRepositoryConfig(t *testing.T) {
	// Test convenience function
	config := NewDefaultRepositoryConfig("Default Test Repository")

	assert.Equal(t, "Default Test Repository", config.Name)
	assert.NotEmpty(t, config.ID)
	assert.False(t, config.CreatedAt.IsZero())
	assert.Equal(t, "date", config.StorageStrategy) // should use defaults
	assert.Equal(t, "5m", config.SyncSettings.QuickScanInterval)
	assert.NoError(t, config.Validate())

	t.Logf("✅ Convenience function NewDefaultRepositoryConfig works")
}

func TestNewRepositoryConfig_PracticalUsageExamples(t *testing.T) {
	// Example 1: Photo storage for family with fast scanning
	familyConfig := NewRepositoryConfig("Family Photos 2024",
		WithStorageStrategy("date"),
		WithSyncSettings("2m", "20m", []string{".DS_Store", "Thumbs.db", "*.tmp"}),
		WithLocalSettings(true, "uuid", 104857600, false, true)) // 100MB max, with backups

	assert.Equal(t, "Family Photos 2024", familyConfig.Name)
	assert.Equal(t, "date", familyConfig.StorageStrategy)
	assert.Equal(t, "2m", familyConfig.SyncSettings.QuickScanInterval)
	assert.Equal(t, int64(104857600), familyConfig.LocalSettings.MaxFileSize)
	assert.True(t, familyConfig.LocalSettings.CreateBackups)

	// Example 2: Professional archive with content-addressed storage
	archiveConfig := NewRepositoryConfig("Professional Archive",
		WithStorageStrategy("cas"),
		WithSyncSettings("10m", "2h", DefaultIgnorePatterns),
		WithLocalSettings(true, "rename", 0, true, true)) // No size limit, compression enabled

	assert.Equal(t, "Professional Archive", archiveConfig.Name)
	assert.Equal(t, "cas", archiveConfig.StorageStrategy)
	assert.True(t, archiveConfig.LocalSettings.CompressFiles)
	assert.Equal(t, len(DefaultIgnorePatterns), len(archiveConfig.SyncSettings.IgnorePatterns))

	// Example 3: Temporary workspace with aggressive cleanup
	workspaceConfig := NewRepositoryConfig("Temp Workspace",
		WithStorageStrategy("flat"),
		WithSyncSettings("30s", "5m", []string{"*.tmp", "*.temp", "*.part", "*.processing"}),
		WithLocalSettings(false, "overwrite", 52428800, false, false)) // 50MB max, no backups

	assert.Equal(t, "Temp Workspace", workspaceConfig.Name)
	assert.Equal(t, "flat", workspaceConfig.StorageStrategy)
	assert.Equal(t, "30s", workspaceConfig.SyncSettings.QuickScanInterval)
	assert.Equal(t, "overwrite", workspaceConfig.LocalSettings.HandleDuplicateFilenames)
	assert.False(t, workspaceConfig.LocalSettings.CreateBackups)

	// All configs should be valid
	for i, config := range []*RepositoryConfig{familyConfig, archiveConfig, workspaceConfig} {
		assert.NoError(t, config.Validate(), "Config %d should be valid", i+1)
	}

	t.Logf("✅ Created 3 practical configuration examples:")
	t.Logf("  1. Family Photos: date strategy, 2m scan, 100MB limit, backups")
	t.Logf("  2. Professional: CAS strategy, compression, full ignore list")
	t.Logf("  3. Workspace: flat strategy, 30s scan, 50MB limit, no backups")
}

func TestNewRepositoryConfig_SystemGeneratedFields(t *testing.T) {
	// Test that ID and CreatedTime are always system-generated
	config1 := NewRepositoryConfig("Test Repo 1")
	config2 := NewRepositoryConfig("Test Repo 2")

	// IDs should be unique UUIDs
	assert.NotEqual(t, config1.ID, config2.ID)
	assert.NotEmpty(t, config1.ID)
	assert.NotEmpty(t, config2.ID)

	// Should be valid UUIDs
	_, err1 := uuid.Parse(config1.ID)
	_, err2 := uuid.Parse(config2.ID)
	assert.NoError(t, err1)
	assert.NoError(t, err2)

	// CreatedTime should be recent and unique (within reasonable bounds)
	now := time.Now()
	assert.True(t, config1.CreatedAt.After(now.Add(-time.Minute)))
	assert.True(t, config1.CreatedAt.Before(now.Add(time.Minute)))
	assert.True(t, config2.CreatedAt.After(now.Add(-time.Minute)))
	assert.True(t, config2.CreatedAt.Before(now.Add(time.Minute)))

	// Even with options, ID and CreatedTime should still be system-generated
	config3 := NewRepositoryConfig("Test Repo 3",
		WithStorageStrategy("cas"),
		WithSyncSettings("1m", "10m", []string{"*.tmp"}))

	assert.NotEqual(t, config1.ID, config3.ID)
	assert.NotEqual(t, config2.ID, config3.ID)
	assert.True(t, config3.CreatedAt.After(now.Add(-time.Minute)))

	t.Logf("✅ ID and CreatedTime are always system-generated:")
	t.Logf("  Config1 ID: %s", config1.ID[:8])
	t.Logf("  Config2 ID: %s", config2.ID[:8])
	t.Logf("  Config3 ID: %s", config3.ID[:8])
	t.Logf("  All have unique IDs and recent timestamps")
}

func TestNewRepositoryConfig_WithBackupPath(t *testing.T) {
	// Test backup path configuration
	config := NewRepositoryConfig("Repository with Backup",
		WithStorageStrategy("date"),
		WithLocalSettings(true, "uuid", 0, false, true),
		WithBackupPath("/external/backup/drive"))

	assert.Equal(t, "Repository with Backup", config.Name)
	assert.Equal(t, "date", config.StorageStrategy)
	assert.True(t, config.LocalSettings.CreateBackups)
	assert.Equal(t, "/external/backup/drive", config.LocalSettings.BackupPath)
	assert.NoError(t, config.Validate())

	// Test that backup path is saved to YAML
	testDir := t.TempDir()
	err := config.SaveConfigToFile(testDir)
	require.NoError(t, err)

	// Load back and verify backup path is preserved
	loadedConfig, err := LoadConfigFromFile(testDir)
	require.NoError(t, err)
	assert.Equal(t, "/external/backup/drive", loadedConfig.LocalSettings.BackupPath)
	assert.True(t, loadedConfig.LocalSettings.CreateBackups)

	t.Logf("✅ Backup path configuration works: %s", loadedConfig.LocalSettings.BackupPath)
}

func TestRepositoryConfig_WithBackupPath_YAML(t *testing.T) {
	// Create config with backup path to show in YAML
	config := NewRepositoryConfig("Repository with External Backup",
		WithStorageStrategy("cas"),
		WithSyncSettings("3m", "45m", []string{".DS_Store", "*.tmp"}),
		WithLocalSettings(true, "rename", 209715200, true, true),
		WithBackupPath("/mnt/external-backup"))

	testDir := t.TempDir()
	err := config.SaveConfigToFile(testDir)
	require.NoError(t, err)

	// Read and display the YAML content
	configPath := filepath.Join(testDir, ".lumiliorepo")
	content, err := os.ReadFile(configPath)
	require.NoError(t, err)

	t.Logf("📄 Repository config with backup path:\n%s", string(content))

	// Verify backup path is in YAML
	assert.Contains(t, string(content), "backup_path: /mnt/external-backup")
	assert.Contains(t, string(content), "create_backups: true")

	t.Logf("✅ Backup path correctly saved in YAML configuration")
}
