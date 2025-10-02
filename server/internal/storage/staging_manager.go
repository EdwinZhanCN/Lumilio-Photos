package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"server/internal/storage/repocfg"

	"github.com/google/uuid"
)

// StagingManager handles staging operations with repository configuration integration
type StagingManager interface {
	// Staging operations with repository configuration support
	CreateStagingFile(repoPath, filename string) (*StagingFile, error)
	CommitStagingFile(stagingFile *StagingFile, finalPath string) error
	CommitStagingFileToInbox(stagingFile *StagingFile, hash string) error
	CleanupStaging(repoPath string, maxAge time.Duration) error

	// Path resolution
	ResolveInboxPath(repoPath string, originalFilename, hash string) (string, error)
}

// DefaultStagingManager implements the StagingManager interface
type DefaultStagingManager struct {
	dirManager DirectoryManager
}

// NewStagingManager creates a new staging manager instance
func NewStagingManager() StagingManager {
	return &DefaultStagingManager{
		dirManager: NewDirectoryManager(),
	}
}

// CreateStagingFile creates a new file in the staging area
func (sm *DefaultStagingManager) CreateStagingFile(repoPath, filename string) (*StagingFile, error) {
	return sm.dirManager.CreateStagingFile(repoPath, filename)
}

// CommitStagingFile moves a staging file to its final destination
func (sm *DefaultStagingManager) CommitStagingFile(stagingFile *StagingFile, finalPath string) error {
	return sm.dirManager.CommitStagingFile(stagingFile, finalPath)
}

// CommitStagingFileToInbox commits a staging file to the inbox using repository configuration
func (sm *DefaultStagingManager) CommitStagingFileToInbox(stagingFile *StagingFile, hash string) error {
	if stagingFile == nil {
		return fmt.Errorf("staging file is nil")
	}

	// Load repository configuration
	cfg, err := repocfg.LoadConfigFromFile(stagingFile.RepoPath)
	if err != nil {
		return fmt.Errorf("failed to load repository config: %w", err)
	}

	// Resolve inbox path based on repository configuration
	inboxPath, err := sm.resolveInboxRelativePath(stagingFile.RepoPath, cfg, stagingFile.Filename, hash)
	if err != nil {
		return fmt.Errorf("failed to resolve inbox path: %w", err)
	}

	// Commit to the resolved inbox path
	return sm.CommitStagingFile(stagingFile, inboxPath)
}

// ResolveInboxPath resolves the final inbox path for a file based on repository configuration
func (sm *DefaultStagingManager) ResolveInboxPath(repoPath string, originalFilename, hash string) (string, error) {
	cfg, err := repocfg.LoadConfigFromFile(repoPath)
	if err != nil {
		return "", fmt.Errorf("failed to load repository config: %w", err)
	}

	return sm.resolveInboxRelativePath(repoPath, cfg, originalFilename, hash)
}

// CleanupStaging removes old staging files
func (sm *DefaultStagingManager) CleanupStaging(repoPath string, maxAge time.Duration) error {
	return sm.dirManager.CleanupStaging(repoPath, maxAge)
}

// resolveInboxRelativePath decides the inbox-relative final path based on repository storage strategy.
// Strategies:
//   - date: inbox/YYYY/MM/<filename-with-duplicate-handling>
//   - flat: inbox/<filename-with-duplicate-handling>
//   - cas:  inbox/aa/bb/cc/<hash><ext> (falls back to date if hash is empty)
func (sm *DefaultStagingManager) resolveInboxRelativePath(repoPath string, cfg *repocfg.RepositoryConfig, originalFilename string, hash string) (string, error) {
	inboxRoot := filepath.Join(repoPath, DefaultStructure.InboxDir)
	strategy := strings.ToLower(cfg.StorageStrategy)
	duplicateMode := cfg.LocalSettings.HandleDuplicateFilenames
	preserve := cfg.LocalSettings.PreserveOriginalFilename

	switch strategy {
	case "flat":
		// inbox/<filename>
		filename := originalFilename
		if preserve {
			filename = sm.uniqueInboxFilename(inboxRoot, originalFilename, duplicateMode)
		} else {
			filename = sm.uniqueInboxFilename(inboxRoot, filename, duplicateMode)
		}
		return filepath.Join(DefaultStructure.InboxDir, filename), nil

	case "cas":
		// inbox/aa/bb/cc/<hash><ext>
		// If hash is missing, gracefully fall back to date strategy
		if len(hash) < 6 {
			return sm.resolveInboxRelativePath(repoPath, &repocfg.RepositoryConfig{
				StorageStrategy: "date",
				LocalSettings:   cfg.LocalSettings,
			}, originalFilename, hash)
		}

		ext := filepath.Ext(originalFilename)
		seg1 := hash[0:2]
		seg2 := hash[2:4]
		seg3 := hash[4:6]
		dirRel := filepath.Join(DefaultStructure.InboxDir, seg1, seg2, seg3)
		fullDir := filepath.Join(repoPath, dirRel)
		if err := os.MkdirAll(fullDir, 0755); err != nil {
			return "", fmt.Errorf("failed to create CAS inbox directories: %w", err)
		}
		filename := hash + ext
		return filepath.Join(dirRel, filename), nil

	case "date":
		fallthrough
	default:
		// inbox/YYYY/MM/<filename>
		now := time.Now()
		dirRel := filepath.Join(DefaultStructure.InboxDir, fmt.Sprintf("%d", now.Year()), fmt.Sprintf("%02d", now.Month()))
		fullDir := filepath.Join(repoPath, dirRel)
		if err := os.MkdirAll(fullDir, 0755); err != nil {
			return "", fmt.Errorf("failed to create date-based inbox directories: %w", err)
		}
		// Apply duplicate handling in the target directory
		filename := sm.uniqueInboxFilename(fullDir, originalFilename, duplicateMode)
		return filepath.Join(dirRel, filename), nil
	}
}

// uniqueInboxFilename applies duplicate handling within a specific directory.
// duplicateMode can be: "overwrite", "uuid", "rename" (default)
func (sm *DefaultStagingManager) uniqueInboxFilename(dirFullPath string, filename string, duplicateMode string) string {
	originalPath := filepath.Join(dirFullPath, filename)

	// If file doesn't exist, use the provided name
	if _, err := os.Stat(originalPath); os.IsNotExist(err) {
		return filename
	}

	switch strings.ToLower(duplicateMode) {
	case "overwrite":
		// Keep original name; caller will overwrite
		return filename
	case "uuid":
		ext := filepath.Ext(filename)
		base := strings.TrimSuffix(filename, ext)
		return fmt.Sprintf("%s_%s%s", base, uuid.New().String()[:8], ext)
	case "rename":
		fallthrough
	default:
		// Append (1), (2), etc.
		ext := filepath.Ext(filename)
		base := strings.TrimSuffix(filename, ext)
		for i := 1; i <= 999; i++ {
			newFilename := fmt.Sprintf("%s (%d)%s", base, i, ext)
			if _, err := os.Stat(filepath.Join(dirFullPath, newFilename)); os.IsNotExist(err) {
				return newFilename
			}
		}
		// Fallback to timestamp to guarantee uniqueness
		timestamp := time.Now().Format("20060102_150405")
		return fmt.Sprintf("%s_%s%s", base, timestamp, ext)
	}
}
