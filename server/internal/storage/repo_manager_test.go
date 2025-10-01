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

func TestRepositoryInspection_CustomPath(t *testing.T) {
	// CUSTOMIZABLE INSPECTION TEST
	// Set LUMILIO_INSPECTION_PATH environment variable to specify where to create the repository
	// Example: LUMILIO_INSPECTION_PATH=/Users/username/test-repo go test -v ./internal/storage -run TestRepositoryInspection_CustomPath

	defaultPath := "/tmp/lumilio-custom-inspection"
	inspectionPath := os.Getenv("LUMILIO_INSPECTION_PATH")
	if inspectionPath == "" {
		inspectionPath = defaultPath
		t.Logf("💡 Using default path. To customize, set LUMILIO_INSPECTION_PATH environment variable")
	}

	t.Logf("🎯 Target inspection path: %s", inspectionPath)

	// Ask for confirmation if directory exists
	if _, err := os.Stat(inspectionPath); err == nil {
		t.Logf("⚠️  Directory already exists at %s", inspectionPath)
		t.Logf("🗑️  Removing existing content...")
		os.RemoveAll(inspectionPath)
	}

	manager := NewRepositoryManager(nil)

	// Create a feature-rich configuration for inspection
	config := NewRepositoryConfig("Custom Inspection Repository",
		WithStorageStrategy("date"),
		WithSyncSettings("2m", "20m", []string{".DS_Store", "Thumbs.db", "*.tmp", "node_modules/"}),
		WithLocalSettings(true, "uuid", 209715200, true, true), // 200MB limit, compression & backups enabled
		WithBackupPath("/mnt/backup-drive"))

	t.Logf("🏗️  Creating repository structure...")

	// Create directory structure
	err := manager.(*DefaultRepositoryManager).createRepositoryStructure(inspectionPath)
	require.NoError(t, err)

	// Save configuration
	err = config.SaveConfigToFile(inspectionPath)
	require.NoError(t, err)

	// Create realistic sample content structure
	sampleStructure := map[string]string{
		// Inbox structure (YYYY/MM format)
		"inbox/2024/01/IMG_001.jpg": "Sample photo from January 2024",
		"inbox/2024/01/VID_001.mp4": "Sample video from January 2024",
		"inbox/2024/03/IMG_045.jpg": "Sample photo from March 2024",

		// User-managed structure
		"Photos/Family/2023 Vacation/beach.jpg":  "Family vacation photos",
		"Photos/Family/2023 Vacation/sunset.jpg": "Beautiful sunset photo",
		"Photos/Work/Projects/presentation.jpg":  "Work-related screenshot",
		"Documents/Receipts/receipt_001.pdf":     "Important receipt",
		"Videos/Personal/birthday_party.mp4":     "Birthday celebration video",

		// System files (should be ignored by scanning)
		".DS_Store":           "macOS system file",
		"Photos/.DS_Store":    "Another system file",
		"Thumbs.db":           "Windows thumbnail cache",
		"temp_processing.tmp": "Temporary processing file",

		// Sample log entries
		".lumilio/logs/sample_operations.log": `{"timestamp":"2024-01-15T10:30:00Z","operation":"file_added","path":"inbox/2024/01/IMG_001.jpg","size":2048576}
{"timestamp":"2024-01-15T10:31:00Z","operation":"thumbnail_generated","path":"inbox/2024/01/IMG_001.jpg","thumbnail_size":"300x200"}
{"timestamp":"2024-01-15T10:32:00Z","operation":"scan_completed","files_found":15,"duration":"1.2s"}`,

		// Sample config backup
		".lumilio/backups/config-2024-01-15-10-30-00.yaml": "# Previous config version backup\nversion: \"1.0\"\n# ... previous settings",
	}

	t.Logf("📂 Creating sample content structure...")
	for filePath, content := range sampleStructure {
		fullPath := filepath.Join(inspectionPath, filePath)
		err := os.MkdirAll(filepath.Dir(fullPath), 0755)
		require.NoError(t, err)
		err = os.WriteFile(fullPath, []byte(content), 0644)
		require.NoError(t, err)
	}

	// Validate the created repository
	result, err := manager.ValidateRepository(inspectionPath)
	require.NoError(t, err)
	require.True(t, result.Valid)

	// Test adding the repository
	t.Logf("➕ Testing repository addition...")
	addedRepo, err := manager.AddRepository(inspectionPath)
	require.NoError(t, err)
	t.Logf("✅ Repository successfully added with ID: %s", addedRepo.RepoID.Bytes)

	// Display comprehensive inspection information
	t.Logf("")
	t.Logf("🎉 CUSTOM INSPECTION REPOSITORY CREATED!")
	t.Logf("==================================================")
	t.Logf("📍 Location: %s", inspectionPath)
	t.Logf("📛 Name: %s", config.Name)
	t.Logf("🆔 ID: %s", config.ID)
	t.Logf("📅 Created: %s", config.CreatedAt.Format("2006-01-02 15:04:05"))
	t.Logf("💾 Storage Strategy: %s (YYYY/MM format)", config.StorageStrategy)
	t.Logf("⚡ Quick Scan: %s", config.SyncSettings.QuickScanInterval)
	t.Logf("🔍 Full Scan: %s", config.SyncSettings.FullScanInterval)
	t.Logf("💾 Max File Size: %d bytes (%.1f MB)", config.LocalSettings.MaxFileSize, float64(config.LocalSettings.MaxFileSize)/1024/1024)
	t.Logf("🗜️  Compression: %v", config.LocalSettings.CompressFiles)
	t.Logf("💾 Backups: %v", config.LocalSettings.CreateBackups)
	t.Logf("📁 Backup Path: %s", config.LocalSettings.BackupPath)
	t.Logf("🚫 Ignore Patterns: %d configured", len(config.SyncSettings.IgnorePatterns))
	t.Logf("")

	t.Logf("🔍 INSPECTION COMMANDS:")
	t.Logf("# View complete structure:")
	t.Logf("tree %s", inspectionPath)
	t.Logf("")
	t.Logf("# View configuration:")
	t.Logf("cat %s/.lumiliorepo", inspectionPath)
	t.Logf("")
	t.Logf("# View system directories:")
	t.Logf("ls -la %s/.lumilio/", inspectionPath)
	t.Logf("")
	t.Logf("# View sample inbox structure (YYYY/MM):")
	t.Logf("find %s/inbox -type f", inspectionPath)
	t.Logf("")
	t.Logf("# View user-managed files:")
	t.Logf("find %s -path '*/.lumilio' -prune -o -path '*/inbox' -prune -o -name '.lumiliorepo' -prune -o -type f -print", inspectionPath)
	t.Logf("")
	t.Logf("# View log files:")
	t.Logf("ls -la %s/.lumilio/logs/", inspectionPath)
	t.Logf("")
	t.Logf("🗑️  CLEANUP:")
	t.Logf("rm -rf %s", inspectionPath)
	t.Logf("")
	t.Logf("💡 To create at custom location next time:")
	t.Logf("LUMILIO_INSPECTION_PATH=/your/custom/path go test -v ./internal/storage -run TestRepositoryInspection_CustomPath")
}

func TestRepositoryDirectoryStructure(t *testing.T) {
	manager := NewRepositoryManager(nil) // Using nil for tests since we're not testing DB operations
	testDir := t.TempDir()

	// Create a repository config
	config := NewRepositoryConfig("Test Structure Repository")

	// Test creating directory structure (without DB operations)
	err := manager.(*DefaultRepositoryManager).createRepositoryStructure(testDir)
	require.NoError(t, err)

	// Verify all expected directories exist
	expectedDirs := []string{
		".lumilio",
		".lumilio/assets",
		".lumilio/assets/thumbnails",
		".lumilio/assets/videos",
		".lumilio/assets/audios",
		".lumilio/staging",
		".lumilio/temp",
		".lumilio/trash",
		".lumilio/logs",
		".lumilio/backups",
		"inbox",
	}

	for _, dir := range expectedDirs {
		dirPath := filepath.Join(testDir, dir)
		info, err := os.Stat(dirPath)
		assert.NoError(t, err, "Directory %s should exist", dir)
		assert.True(t, info.IsDir(), "%s should be a directory", dir)
	}

	// Verify log files are created
	expectedLogFiles := []string{
		".lumilio/logs/app.log",
		".lumilio/logs/error.log",
		".lumilio/logs/operations.log",
	}

	for _, logFile := range expectedLogFiles {
		logPath := filepath.Join(testDir, logFile)
		_, err := os.Stat(logPath)
		assert.NoError(t, err, "Log file %s should exist", logFile)
	}

	// Save config and validate the complete repository
	err = config.SaveConfigToFile(testDir)
	require.NoError(t, err)

	// Now validation should pass with no warnings
	result, err := manager.ValidateRepository(testDir)
	require.NoError(t, err)
	assert.True(t, result.Valid)
	assert.Empty(t, result.Errors)
	assert.Empty(t, result.Warnings, "No warnings expected with complete structure")

	t.Logf("✅ Repository structure created successfully with %d directories and %d log files",
		len(expectedDirs), len(expectedLogFiles))
}
