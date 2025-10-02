package storage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDirectoryManager_CreateStructure(t *testing.T) {
	dm := NewDirectoryManager()
	testDir := t.TempDir()

	t.Run("create complete structure", func(t *testing.T) {
		err := dm.CreateStructure(testDir)
		require.NoError(t, err)

		// Verify all directories were created
		for _, dir := range Directories {
			dirPath := filepath.Join(testDir, dir)
			info, err := os.Stat(dirPath)
			assert.NoError(t, err, "Directory %s should exist", dir)
			assert.True(t, info.IsDir(), "Path %s should be a directory", dir)
		}

		// Verify log files were created
		logFiles := []string{
			".lumilio/logs/app.log",
			".lumilio/logs/error.log",
			".lumilio/logs/operations.log",
		}

		for _, logFile := range logFiles {
			logPath := filepath.Join(testDir, logFile)
			_, err := os.Stat(logPath)
			assert.NoError(t, err, "Log file %s should exist", logFile)
		}
	})

	t.Run("create structure with invalid path", func(t *testing.T) {
		err := dm.CreateStructure("/invalid\x00path")
		assert.Error(t, err)
		// The error might come from directory creation rather than path validation
		assert.True(t, err != nil, "Should get an error for invalid path")
	})
}

func TestDirectoryManager_ValidateStructure(t *testing.T) {
	dm := NewDirectoryManager()

	t.Run("validate complete structure", func(t *testing.T) {
		testDir := t.TempDir()
		err := dm.CreateStructure(testDir)
		require.NoError(t, err)

		validation, err := dm.ValidateStructure(testDir)
		require.NoError(t, err)
		assert.True(t, validation.Valid)
		assert.Empty(t, validation.MissingDirectories)
		assert.Empty(t, validation.InvalidPaths)
	})

	t.Run("validate missing directories", func(t *testing.T) {
		testDir := t.TempDir()

		// Create only some directories
		requiredDirs := []string{".lumilio", "inbox"}
		for _, dir := range requiredDirs {
			err := os.MkdirAll(filepath.Join(testDir, dir), 0755)
			require.NoError(t, err)
		}

		validation, err := dm.ValidateStructure(testDir)
		require.NoError(t, err)
		assert.True(t, validation.Valid) // Should be valid but with warnings
		assert.NotEmpty(t, validation.MissingDirectories)
		assert.NotEmpty(t, validation.Warnings)
	})

	t.Run("validate nonexistent repository", func(t *testing.T) {
		validation, err := dm.ValidateStructure("/nonexistent/path")
		require.NoError(t, err)
		assert.False(t, validation.Valid)
		assert.Contains(t, validation.InvalidPaths[0], "Repository root does not exist")
	})

	t.Run("validate file instead of directory", func(t *testing.T) {
		testDir := t.TempDir()

		// Create a file where we expect a directory
		invalidDir := filepath.Join(testDir, ".lumilio")
		err := os.WriteFile(invalidDir, []byte("not a directory"), 0644)
		require.NoError(t, err)

		validation, err := dm.ValidateStructure(testDir)
		require.NoError(t, err)
		assert.False(t, validation.Valid)
		assert.Contains(t, validation.InvalidPaths[0], "Expected directory but found file")
	})
}

func TestDirectoryManager_RepairStructure(t *testing.T) {
	dm := NewDirectoryManager()

	t.Run("repair missing directories", func(t *testing.T) {
		testDir := t.TempDir()

		// Create partial structure
		err := os.MkdirAll(filepath.Join(testDir, ".lumilio"), 0755)
		require.NoError(t, err)

		// Repair should recreate missing directories
		err = dm.RepairStructure(testDir)
		require.NoError(t, err)

		// Validate that structure is now complete
		validation, err := dm.ValidateStructure(testDir)
		require.NoError(t, err)
		assert.True(t, validation.Valid)
		assert.Empty(t, validation.MissingDirectories)
	})

	t.Run("repair missing log files", func(t *testing.T) {
		testDir := t.TempDir()
		err := dm.CreateStructure(testDir)
		require.NoError(t, err)

		// Remove a log file
		logPath := filepath.Join(testDir, ".lumilio/logs/app.log")
		err = os.Remove(logPath)
		require.NoError(t, err)

		// Repair should recreate it
		err = dm.RepairStructure(testDir)
		require.NoError(t, err)

		// Verify log file exists again
		_, err = os.Stat(logPath)
		assert.NoError(t, err)
	})
}

func TestDirectoryManager_ProtectedPaths(t *testing.T) {
	dm := NewDirectoryManager()
	testDir := t.TempDir()

	t.Run("identify protected paths", func(t *testing.T) {
		protectedPaths := []string{
			".lumilio/assets/file.jpg",
			".lumilio/staging/upload.tmp",
			"inbox/2024/01/photo.jpg",
			".lumilio",
			"inbox",
		}

		for _, path := range protectedPaths {
			assert.True(t, dm.IsProtectedPath(testDir, path),
				"Path %s should be protected", path)
		}
	})

	t.Run("identify unprotected paths", func(t *testing.T) {
		unprotectedPaths := []string{
			"user-photos/family/vacation.jpg",
			"personal/documents/file.pdf",
			"custom-folder/image.png",
			"random-file.txt",
		}

		for _, path := range unprotectedPaths {
			assert.False(t, dm.IsProtectedPath(testDir, path),
				"Path %s should not be protected", path)
		}
	})

	t.Run("handle absolute paths", func(t *testing.T) {
		absProtectedPath := filepath.Join(testDir, ".lumilio", "assets", "file.jpg")
		assert.True(t, dm.IsProtectedPath(testDir, absProtectedPath))

		absUnprotectedPath := filepath.Join(testDir, "user-content", "file.jpg")
		assert.False(t, dm.IsProtectedPath(testDir, absUnprotectedPath))
	})
}

func TestDirectoryManager_StagingOperations(t *testing.T) {
	dm := NewDirectoryManager()
	testDir := t.TempDir()

	err := dm.CreateStructure(testDir)
	require.NoError(t, err)

	t.Run("create staging file", func(t *testing.T) {
		stagingFile, err := dm.CreateStagingFile(testDir, "test-upload.jpg")
		require.NoError(t, err)

		assert.NotEmpty(t, stagingFile.ID)
		assert.Equal(t, testDir, stagingFile.RepoPath)
		assert.Equal(t, "test-upload.jpg", stagingFile.Filename)
		assert.True(t, time.Since(stagingFile.CreatedAt) < time.Minute)

		// Verify file exists
		_, err = os.Stat(stagingFile.Path)
		assert.NoError(t, err)

		// Verify it's in the correct staging directory
		expectedDir := filepath.Join(testDir, DefaultStructure.IncomingDir)
		assert.Contains(t, stagingFile.Path, expectedDir)
	})

	t.Run("commit staging file", func(t *testing.T) {
		stagingFile, err := dm.CreateStagingFile(testDir, "commit-test.jpg")
		require.NoError(t, err)

		// Write some content to the staging file
		content := []byte("test image content")
		err = os.WriteFile(stagingFile.Path, content, 0644)
		require.NoError(t, err)

		finalPath := "user-content/photos/final.jpg"
		err = dm.CommitStagingFile(stagingFile, finalPath)
		require.NoError(t, err)

		// Verify file moved to final location
		finalFullPath := filepath.Join(testDir, finalPath)
		finalContent, err := os.ReadFile(finalFullPath)
		require.NoError(t, err)
		assert.Equal(t, content, finalContent)

		// Verify staging file no longer exists
		_, err = os.Stat(stagingFile.Path)
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("commit staging file validation", func(t *testing.T) {
		stagingFile, err := dm.CreateStagingFile(testDir, "validation-test.jpg")
		require.NoError(t, err)

		// Test with nil staging file
		err = dm.CommitStagingFile(nil, "some/path.jpg")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "staging file is nil")

		// Test with empty final path
		err = dm.CommitStagingFile(stagingFile, "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "final path cannot be empty")

		// Test with absolute final path
		err = dm.CommitStagingFile(stagingFile, "/absolute/path.jpg")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "final path must be repository-relative")
	})

	t.Run("cleanup staging files", func(t *testing.T) {
		// Create some staging files
		staging1, err := dm.CreateStagingFile(testDir, "old-file1.jpg")
		require.NoError(t, err)
		staging2, err := dm.CreateStagingFile(testDir, "old-file2.jpg")
		require.NoError(t, err)
		staging3, err := dm.CreateStagingFile(testDir, "new-file.jpg")
		require.NoError(t, err)

		// Make first two files old
		oldTime := time.Now().Add(-2 * time.Hour)
		err = os.Chtimes(staging1.Path, oldTime, oldTime)
		require.NoError(t, err)
		err = os.Chtimes(staging2.Path, oldTime, oldTime)
		require.NoError(t, err)

		// Cleanup files older than 1 hour
		err = dm.CleanupStaging(testDir, time.Hour)
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

func TestDirectoryManager_TempFileOperations(t *testing.T) {
	dm := NewDirectoryManager()
	testDir := t.TempDir()

	err := dm.CreateStructure(testDir)
	require.NoError(t, err)

	t.Run("create temp file", func(t *testing.T) {
		tempFile, err := dm.CreateTempFile(testDir, "image-processing")
		require.NoError(t, err)

		assert.NotEmpty(t, tempFile.ID)
		assert.Equal(t, testDir, tempFile.RepoPath)
		assert.Equal(t, "image-processing", tempFile.Purpose)
		assert.True(t, time.Since(tempFile.CreatedAt) < time.Minute)

		// Verify file exists
		_, err = os.Stat(tempFile.Path)
		assert.NoError(t, err)

		// Verify it's in the temp directory
		expectedDir := filepath.Join(testDir, DefaultStructure.TempDir)
		assert.Contains(t, tempFile.Path, expectedDir)
		assert.Contains(t, tempFile.Path, "image-processing")
		assert.Contains(t, tempFile.Path, ".tmp")
	})

	t.Run("cleanup temp files", func(t *testing.T) {
		// Create temp files
		temp1, err := dm.CreateTempFile(testDir, "old-processing")
		require.NoError(t, err)
		temp2, err := dm.CreateTempFile(testDir, "new-processing")
		require.NoError(t, err)

		// Make first file old
		oldTime := time.Now().Add(-2 * time.Hour)
		err = os.Chtimes(temp1.Path, oldTime, oldTime)
		require.NoError(t, err)

		// Cleanup files older than 1 hour
		err = dm.CleanupTempFiles(testDir, time.Hour)
		require.NoError(t, err)

		// Old file should be gone
		_, err = os.Stat(temp1.Path)
		assert.True(t, os.IsNotExist(err))

		// New file should remain
		_, err = os.Stat(temp2.Path)
		assert.NoError(t, err)
	})
}

func TestDirectoryManager_TrashOperations(t *testing.T) {
	dm := NewDirectoryManager()
	testDir := t.TempDir()

	err := dm.CreateStructure(testDir)
	require.NoError(t, err)

	t.Run("move file to trash", func(t *testing.T) {
		// Create a test file
		testFile := filepath.Join(testDir, "user-content", "delete-me.txt")
		err := os.MkdirAll(filepath.Dir(testFile), 0755)
		require.NoError(t, err)
		err = os.WriteFile(testFile, []byte("content to delete"), 0644)
		require.NoError(t, err)

		// Move to trash
		metadata := &DeleteMetadata{
			DeletedAt:    time.Now(),
			OriginalPath: "user-content/delete-me.txt",
			Reason:       "user_delete",
			UserID:       stringPtr("user123"),
		}
		err = dm.MoveToTrash(testDir, "user-content/delete-me.txt", metadata)
		require.NoError(t, err)

		// Original file should be gone
		_, err = os.Stat(testFile)
		assert.True(t, os.IsNotExist(err))

		// File should be in trash
		trashFiles, err := dm.ListTrashFiles(testDir)
		require.NoError(t, err)
		assert.Len(t, trashFiles, 1)

		trashFile := trashFiles[0]
		assert.NotEmpty(t, trashFile.ID)
		assert.Equal(t, testDir, trashFile.RepoPath)
		assert.Contains(t, trashFile.TrashPath, "delete-me.txt")

		// Verify metadata
		require.NotNil(t, trashFile.Metadata)
		assert.Equal(t, "user-content/delete-me.txt", trashFile.Metadata.OriginalPath)
		assert.Equal(t, "user_delete", trashFile.Metadata.Reason)
		assert.Equal(t, "user123", *trashFile.Metadata.UserID)
	})

	t.Run("recover from trash", func(t *testing.T) {
		// Create and trash a file
		testFile := filepath.Join(testDir, "user-content", "recover-me.txt")
		content := []byte("content to recover")
		err := os.MkdirAll(filepath.Dir(testFile), 0755)
		require.NoError(t, err)
		err = os.WriteFile(testFile, content, 0644)
		require.NoError(t, err)

		err = dm.MoveToTrash(testDir, "user-content/recover-me.txt", nil)
		require.NoError(t, err)

		// Get trash file ID
		trashFiles, err := dm.ListTrashFiles(testDir)
		require.NoError(t, err)

		var targetTrashFile *TrashFile
		for _, tf := range trashFiles {
			if strings.Contains(tf.TrashPath, "recover-me.txt") {
				targetTrashFile = tf
				break
			}
		}
		require.NotNil(t, targetTrashFile)

		// Recover the file
		err = dm.RecoverFromTrash(testDir, targetTrashFile.ID)
		require.NoError(t, err)

		// File should be back in original location
		recoveredContent, err := os.ReadFile(testFile)
		require.NoError(t, err)
		assert.Equal(t, content, recoveredContent)

		// Should no longer be in trash
		trashFiles, err = dm.ListTrashFiles(testDir)
		require.NoError(t, err)
		for _, tf := range trashFiles {
			assert.NotContains(t, tf.TrashPath, "recover-me.txt")
		}
	})

	t.Run("purge old trash files", func(t *testing.T) {
		// Use a separate test directory to avoid interference from previous tests
		purgeTestDir := t.TempDir()
		err := dm.CreateStructure(purgeTestDir)
		require.NoError(t, err)

		// Create and trash files
		oldFile := filepath.Join(purgeTestDir, "user-content", "old-file.txt")
		newFile := filepath.Join(purgeTestDir, "user-content", "new-file.txt")

		for _, file := range []string{oldFile, newFile} {
			err := os.MkdirAll(filepath.Dir(file), 0755)
			require.NoError(t, err)
			err = os.WriteFile(file, []byte("content"), 0644)
			require.NoError(t, err)
		}

		// Trash both files
		err = dm.MoveToTrash(purgeTestDir, "user-content/old-file.txt", &DeleteMetadata{
			DeletedAt: time.Now().Add(-48 * time.Hour), // 2 days ago
		})
		require.NoError(t, err)

		err = dm.MoveToTrash(purgeTestDir, "user-content/new-file.txt", &DeleteMetadata{
			DeletedAt: time.Now(), // Just now
		})
		require.NoError(t, err)

		// Purge files older than 24 hours
		err = dm.PurgeTrash(purgeTestDir, 24*time.Hour)
		require.NoError(t, err)

		// Check remaining trash files
		trashFiles, err := dm.ListTrashFiles(purgeTestDir)
		require.NoError(t, err)

		// Should only have the new file
		assert.Len(t, trashFiles, 1)
		assert.Contains(t, trashFiles[0].TrashPath, "new-file.txt")
	})

	t.Run("handle missing trash item", func(t *testing.T) {
		err := dm.RecoverFromTrash(testDir, "nonexistent-id")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "trash item nonexistent-id not found")
	})
}

func TestDirectoryManager_EdgeCases(t *testing.T) {
	dm := NewDirectoryManager()

	t.Run("invalid repository paths", func(t *testing.T) {
		invalidPaths := []string{
			"/dev/null/invalid",
		}

		for _, path := range invalidPaths {
			err := dm.CreateStructure(path)
			assert.Error(t, err, "Expected error for path %s", path)
		}
	})

	t.Run("permission denied scenarios", func(t *testing.T) {
		if os.Getuid() == 0 {
			t.Skip("Skipping permission tests when running as root")
		}

		// Try to create structure in read-only directory
		readOnlyDir := t.TempDir()
		err := os.Chmod(readOnlyDir, 0444) // Read-only
		require.NoError(t, err)
		defer os.Chmod(readOnlyDir, 0755) // Restore for cleanup

		subDir := filepath.Join(readOnlyDir, "repo")
		err = dm.CreateStructure(subDir)
		assert.Error(t, err)
	})

	t.Run("cleanup nonexistent directories", func(t *testing.T) {
		// Should not error for valid repo path with missing staging dir
		testDir := t.TempDir()
		err := dm.CleanupStaging(testDir, time.Hour)
		assert.NoError(t, err) // Should succeed even if staging dir doesn't exist

		err = dm.CleanupTempFiles(testDir, time.Hour)
		assert.NoError(t, err) // Should succeed even if temp dir doesn't exist
	})
}
