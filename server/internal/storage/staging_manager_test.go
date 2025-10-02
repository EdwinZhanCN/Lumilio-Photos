package storage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"server/internal/storage/repocfg"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStagingManager_BasicOperations(t *testing.T) {
	sm := NewStagingManager()
	testDir := t.TempDir()

	// Create repository structure
	dm := NewDirectoryManager()
	err := dm.CreateStructure(testDir)
	require.NoError(t, err)

	t.Run("create staging file", func(t *testing.T) {
		stagingFile, err := sm.CreateStagingFile(testDir, "test-upload.jpg")
		require.NoError(t, err)

		assert.NotEmpty(t, stagingFile.ID)
		assert.Equal(t, testDir, stagingFile.RepoPath)
		assert.Equal(t, "test-upload.jpg", stagingFile.Filename)
		assert.True(t, time.Since(stagingFile.CreatedAt) < time.Minute)

		// Verify file exists in staging
		_, err = os.Stat(stagingFile.Path)
		assert.NoError(t, err)
		assert.Contains(t, stagingFile.Path, DefaultStructure.IncomingDir)
	})

	t.Run("commit staging file to custom path", func(t *testing.T) {
		stagingFile, err := sm.CreateStagingFile(testDir, "custom-path-test.jpg")
		require.NoError(t, err)

		// Write test content
		content := []byte("test image content")
		err = os.WriteFile(stagingFile.Path, content, 0644)
		require.NoError(t, err)

		// Commit to custom path
		finalPath := "user-content/photos/custom.jpg"
		err = sm.CommitStagingFile(stagingFile, finalPath)
		require.NoError(t, err)

		// Verify file moved to final location
		finalFullPath := filepath.Join(testDir, finalPath)
		finalContent, err := os.ReadFile(finalFullPath)
		require.NoError(t, err)
		assert.Equal(t, content, finalContent)

		// Staging file should be gone
		_, err = os.Stat(stagingFile.Path)
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("cleanup staging files", func(t *testing.T) {
		// Create multiple staging files
		staging1, err := sm.CreateStagingFile(testDir, "cleanup1.jpg")
		require.NoError(t, err)
		staging2, err := sm.CreateStagingFile(testDir, "cleanup2.jpg")
		require.NoError(t, err)
		staging3, err := sm.CreateStagingFile(testDir, "cleanup3.jpg")
		require.NoError(t, err)

		// Make first two files old
		oldTime := time.Now().Add(-2 * time.Hour)
		err = os.Chtimes(staging1.Path, oldTime, oldTime)
		require.NoError(t, err)
		err = os.Chtimes(staging2.Path, oldTime, oldTime)
		require.NoError(t, err)

		// Cleanup files older than 1 hour
		err = sm.CleanupStaging(testDir, time.Hour)
		require.NoError(t, err)

		// Old files should be gone
		_, err = os.Stat(staging1.Path)
		assert.True(t, os.IsNotExist(err))
		_, err = os.Stat(staging2.Path)
		assert.True(t, os.IsNotExist(err))

		// New file should remain
		_, err = os.Stat(staging3.Path)
		assert.NoError(t, err)
	})
}

func TestStagingManager_InboxIntegration(t *testing.T) {
	sm := NewStagingManager()
	testDir := t.TempDir()

	// Create repository structure
	dm := NewDirectoryManager()
	err := dm.CreateStructure(testDir)
	require.NoError(t, err)

	t.Run("commit to inbox with date strategy", func(t *testing.T) {
		// Create repository config with date strategy
		config := repocfg.NewRepositoryConfig("Test Repo",
			repocfg.WithStorageStrategy("date"),
			repocfg.WithLocalSettings(true, "uuid", 0, false, false))
		err := config.SaveConfigToFile(testDir)
		require.NoError(t, err)

		// Create and commit staging file
		stagingFile, err := sm.CreateStagingFile(testDir, "date-test.jpg")
		require.NoError(t, err)

		content := []byte("test content for date strategy")
		err = os.WriteFile(stagingFile.Path, content, 0644)
		require.NoError(t, err)

		err = sm.CommitStagingFileToInbox(stagingFile, "")
		require.NoError(t, err)

		// Verify file is in inbox with date structure
		now := time.Now()
		expectedPattern := filepath.Join(testDir, "inbox",
			strings.ToLower(now.Format("2006")),
			strings.ToLower(now.Format("01")))

		// Check that the file exists somewhere in the expected date directory
		entries, err := os.ReadDir(expectedPattern)
		require.NoError(t, err)
		assert.NotEmpty(t, entries, "Should have files in date-based inbox directory")

		// Find our file
		found := false
		for _, entry := range entries {
			if strings.Contains(entry.Name(), "date-test") {
				found = true
				break
			}
		}
		assert.True(t, found, "Should find the uploaded file in date-based directory")
	})

	t.Run("commit to inbox with flat strategy", func(t *testing.T) {
		// Create repository config with flat strategy
		config := repocfg.NewRepositoryConfig("Test Repo Flat",
			repocfg.WithStorageStrategy("flat"),
			repocfg.WithLocalSettings(true, "rename", 0, false, false))
		err := config.SaveConfigToFile(testDir)
		require.NoError(t, err)

		stagingFile, err := sm.CreateStagingFile(testDir, "flat-test.jpg")
		require.NoError(t, err)

		content := []byte("test content for flat strategy")
		err = os.WriteFile(stagingFile.Path, content, 0644)
		require.NoError(t, err)

		err = sm.CommitStagingFileToInbox(stagingFile, "")
		require.NoError(t, err)

		// Verify file is directly in inbox
		expectedPath := filepath.Join(testDir, "inbox", "flat-test.jpg")
		finalContent, err := os.ReadFile(expectedPath)
		require.NoError(t, err)
		assert.Equal(t, content, finalContent)
	})

	t.Run("commit to inbox with CAS strategy", func(t *testing.T) {
		// Create repository config with CAS strategy
		config := repocfg.NewRepositoryConfig("Test Repo CAS",
			repocfg.WithStorageStrategy("cas"),
			repocfg.WithLocalSettings(true, "uuid", 0, false, false))
		err := config.SaveConfigToFile(testDir)
		require.NoError(t, err)

		stagingFile, err := sm.CreateStagingFile(testDir, "cas-test.jpg")
		require.NoError(t, err)

		content := []byte("test content for CAS strategy")
		err = os.WriteFile(stagingFile.Path, content, 0644)
		require.NoError(t, err)

		// Provide a hash for CAS
		hash := "abcdef123456789"
		err = sm.CommitStagingFileToInbox(stagingFile, hash)
		require.NoError(t, err)

		// Verify file is in CAS structure: inbox/ab/cd/ef/abcdef123456789.jpg
		expectedPath := filepath.Join(testDir, "inbox", "ab", "cd", "ef", hash+".jpg")
		finalContent, err := os.ReadFile(expectedPath)
		require.NoError(t, err)
		assert.Equal(t, content, finalContent)
	})

	t.Run("CAS fallback to date strategy", func(t *testing.T) {
		// Create repository config with CAS strategy
		config := repocfg.NewRepositoryConfig("Test Repo CAS Fallback",
			repocfg.WithStorageStrategy("cas"),
			repocfg.WithLocalSettings(true, "uuid", 0, false, false))
		err := config.SaveConfigToFile(testDir)
		require.NoError(t, err)

		stagingFile, err := sm.CreateStagingFile(testDir, "cas-fallback.jpg")
		require.NoError(t, err)

		content := []byte("test content for CAS fallback")
		err = os.WriteFile(stagingFile.Path, content, 0644)
		require.NoError(t, err)

		// Provide insufficient hash (should fallback to date)
		err = sm.CommitStagingFileToInbox(stagingFile, "abc") // Too short
		require.NoError(t, err)

		// Should fall back to date strategy - verify file is in date structure
		now := time.Now()
		dateDir := filepath.Join(testDir, "inbox",
			strings.ToLower(now.Format("2006")),
			strings.ToLower(now.Format("01")))

		entries, err := os.ReadDir(dateDir)
		require.NoError(t, err)

		found := false
		for _, entry := range entries {
			if strings.Contains(entry.Name(), "cas-fallback") {
				found = true
				break
			}
		}
		assert.True(t, found, "Should find file in date directory after CAS fallback")
	})
}

func TestStagingManager_DuplicateHandling(t *testing.T) {
	sm := NewStagingManager()
	testDir := t.TempDir()

	// Create repository structure
	dm := NewDirectoryManager()
	err := dm.CreateStructure(testDir)
	require.NoError(t, err)

	t.Run("duplicate handling with rename strategy", func(t *testing.T) {
		config := repocfg.NewRepositoryConfig("Test Rename",
			repocfg.WithStorageStrategy("flat"),
			repocfg.WithLocalSettings(true, "rename", 0, false, false))
		err := config.SaveConfigToFile(testDir)
		require.NoError(t, err)

		// Create first file
		staging1, err := sm.CreateStagingFile(testDir, "duplicate.jpg")
		require.NoError(t, err)
		err = os.WriteFile(staging1.Path, []byte("first"), 0644)
		require.NoError(t, err)
		err = sm.CommitStagingFileToInbox(staging1, "")
		require.NoError(t, err)

		// Create second file with same name
		staging2, err := sm.CreateStagingFile(testDir, "duplicate.jpg")
		require.NoError(t, err)
		err = os.WriteFile(staging2.Path, []byte("second"), 0644)
		require.NoError(t, err)
		err = sm.CommitStagingFileToInbox(staging2, "")
		require.NoError(t, err)

		// Verify both files exist with different names
		inboxDir := filepath.Join(testDir, "inbox")
		entries, err := os.ReadDir(inboxDir)
		require.NoError(t, err)

		duplicateFiles := 0
		for _, entry := range entries {
			if strings.Contains(entry.Name(), "duplicate") {
				duplicateFiles++
			}
		}
		assert.Equal(t, 2, duplicateFiles, "Should have two duplicate files with different names")
	})

	t.Run("duplicate handling with UUID strategy", func(t *testing.T) {
		config := repocfg.NewRepositoryConfig("Test UUID",
			repocfg.WithStorageStrategy("flat"),
			repocfg.WithLocalSettings(true, "uuid", 0, false, false))
		err := config.SaveConfigToFile(testDir)
		require.NoError(t, err)

		// Create first file
		staging1, err := sm.CreateStagingFile(testDir, "uuid-test.jpg")
		require.NoError(t, err)
		err = os.WriteFile(staging1.Path, []byte("first uuid"), 0644)
		require.NoError(t, err)
		err = sm.CommitStagingFileToInbox(staging1, "")
		require.NoError(t, err)

		// Create second file with same name
		staging2, err := sm.CreateStagingFile(testDir, "uuid-test.jpg")
		require.NoError(t, err)
		err = os.WriteFile(staging2.Path, []byte("second uuid"), 0644)
		require.NoError(t, err)
		err = sm.CommitStagingFileToInbox(staging2, "")
		require.NoError(t, err)

		// Verify both files exist with UUID suffixes
		inboxDir := filepath.Join(testDir, "inbox")
		entries, err := os.ReadDir(inboxDir)
		require.NoError(t, err)

		uuidFiles := 0
		for _, entry := range entries {
			if strings.Contains(entry.Name(), "uuid-test") {
				uuidFiles++
				// Should contain a UUID-like string if it's the second file
				if entry.Name() != "uuid-test.jpg" {
					assert.Contains(t, entry.Name(), "_", "UUID duplicate should contain underscore")
				}
			}
		}
		assert.Equal(t, 2, uuidFiles, "Should have two UUID files")
	})
}

func TestStagingManager_ResolveInboxPath(t *testing.T) {
	sm := NewStagingManager()
	testDir := t.TempDir()

	// Create repository structure
	dm := NewDirectoryManager()
	err := dm.CreateStructure(testDir)
	require.NoError(t, err)

	t.Run("resolve path for date strategy", func(t *testing.T) {
		config := repocfg.NewRepositoryConfig("Test Date Resolve",
			repocfg.WithStorageStrategy("date"))
		err := config.SaveConfigToFile(testDir)
		require.NoError(t, err)

		path, err := sm.ResolveInboxPath(testDir, "test-file.jpg", "")
		require.NoError(t, err)

		now := time.Now()
		expectedPrefix := filepath.Join("inbox",
			strings.ToLower(now.Format("2006")),
			strings.ToLower(now.Format("01")))
		assert.Contains(t, path, expectedPrefix)
		assert.Contains(t, path, "test-file.jpg")
	})

	t.Run("resolve path for CAS strategy", func(t *testing.T) {
		config := repocfg.NewRepositoryConfig("Test CAS Resolve",
			repocfg.WithStorageStrategy("cas"))
		err := config.SaveConfigToFile(testDir)
		require.NoError(t, err)

		hash := "fedcba987654321"
		path, err := sm.ResolveInboxPath(testDir, "test-file.jpg", hash)
		require.NoError(t, err)

		expected := filepath.Join("inbox", "fe", "dc", "ba", hash+".jpg")
		assert.Equal(t, expected, path)
	})

	t.Run("resolve path for flat strategy", func(t *testing.T) {
		config := repocfg.NewRepositoryConfig("Test Flat Resolve",
			repocfg.WithStorageStrategy("flat"))
		err := config.SaveConfigToFile(testDir)
		require.NoError(t, err)

		path, err := sm.ResolveInboxPath(testDir, "test-file.jpg", "")
		require.NoError(t, err)

		expected := filepath.Join("inbox", "test-file.jpg")
		assert.Equal(t, expected, path)
	})
}

func TestStagingManager_ErrorHandling(t *testing.T) {
	sm := NewStagingManager()

	t.Run("commit nil staging file", func(t *testing.T) {
		err := sm.CommitStagingFile(nil, "some/path.jpg")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "staging file is nil")
	})

	t.Run("commit to inbox with nil staging file", func(t *testing.T) {
		err := sm.CommitStagingFileToInbox(nil, "hash")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "staging file is nil")
	})

	t.Run("resolve path with missing config", func(t *testing.T) {
		testDir := t.TempDir()

		_, err := sm.ResolveInboxPath(testDir, "test.jpg", "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to load repository config")
	})

	t.Run("commit to inbox with missing config", func(t *testing.T) {
		testDir := t.TempDir()

		// Create staging file structure but no config
		dm := NewDirectoryManager()
		err := dm.CreateStructure(testDir)
		require.NoError(t, err)

		stagingFile, err := sm.CreateStagingFile(testDir, "no-config.jpg")
		require.NoError(t, err)

		err = sm.CommitStagingFileToInbox(stagingFile, "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to load repository config")
	})
}
