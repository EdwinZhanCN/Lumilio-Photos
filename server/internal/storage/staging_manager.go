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

// StagingManager owns the upload staging area (.lumilio/staging) and the rules
// for committing a staged file into a repository's inbox according to that
// repository's storage strategy. It is the sole owner of staging; the directory
// manager no longer exposes staging primitives.
type StagingManager interface {
	// CreateStagingFile creates an empty placeholder in .lumilio/staging/incoming
	// and returns a handle whose Path/RepoPath the caller writes into.
	CreateStagingFile(repoPath, filename string) (*StagingFile, error)

	// CommitStagingFile moves a staged file to a repository-relative finalPath.
	// finalPath must stay inside the repository and must not already exist.
	CommitStagingFile(stagingFile *StagingFile, finalPath string) error

	// CommitStagingFileToInbox commits a staged file to the inbox location
	// derived from the repository's storage strategy, returning the inbox-
	// relative path it was written to.
	CommitStagingFileToInbox(stagingFile *StagingFile, hash string) (string, error)

	// MoveStagingToFailed moves a staged file into .lumilio/staging/failed.
	MoveStagingToFailed(stagingFile *StagingFile) error

	// CleanupStaging removes staged files (incoming and failed) older than maxAge.
	CleanupStaging(repoPath string, maxAge time.Duration) error
}

// DefaultStagingManager implements the StagingManager interface.
type DefaultStagingManager struct{}

// NewStagingManager creates a new staging manager instance.
func NewStagingManager() *DefaultStagingManager {
	return &DefaultStagingManager{}
}

// Ensure the concrete type satisfies the consumer interface.
var _ StagingManager = (*DefaultStagingManager)(nil)

// CreateStagingFile creates an empty placeholder file in the incoming staging area.
func (sm *DefaultStagingManager) CreateStagingFile(repoPath, filename string) (*StagingFile, error) {
	cleanRepoPath, err := filepath.Abs(filepath.Clean(repoPath))
	if err != nil {
		return nil, fmt.Errorf("invalid repository path: %w", err)
	}

	stagingDir := filepath.Join(cleanRepoPath, DefaultStructure.IncomingDir)
	if err := os.MkdirAll(stagingDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create staging directory: %w", err)
	}

	id := uuid.New().String()
	base := filepath.Base(filename)
	stagingFullPath := filepath.Join(stagingDir, fmt.Sprintf("%s_%s", id, base))

	f, err := os.Create(stagingFullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create staging file: %w", err)
	}
	_ = f.Close()

	return &StagingFile{
		ID:        id,
		RepoPath:  cleanRepoPath,
		Path:      stagingFullPath,
		Filename:  base,
		CreatedAt: time.Now(),
	}, nil
}

// CommitStagingFile moves a staging file to its final repository-relative
// destination. The destination filename is decided upstream (uniqueInboxFilename
// for rename/uuid, the original for overwrite), so this performs an atomic rename
// that intentionally replaces an existing file under the "overwrite" strategy.
func (sm *DefaultStagingManager) CommitStagingFile(stagingFile *StagingFile, finalPath string) error {
	if stagingFile == nil {
		return fmt.Errorf("staging file is nil")
	}
	if strings.TrimSpace(finalPath) == "" {
		return fmt.Errorf("final path cannot be empty")
	}
	if filepath.IsAbs(finalPath) {
		return fmt.Errorf("final path must be repository-relative")
	}

	destFullPath, err := resolveInRepo(stagingFile.RepoPath, finalPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(destFullPath), 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	if err := os.Rename(stagingFile.Path, destFullPath); err != nil {
		return fmt.Errorf("failed to move staged file: %w", err)
	}
	return nil
}

// CommitStagingFileToInbox commits a staging file to the inbox using repository configuration
func (sm *DefaultStagingManager) CommitStagingFileToInbox(stagingFile *StagingFile, hash string) (string, error) {
	if stagingFile == nil {
		return "", fmt.Errorf("staging file is nil")
	}

	// Load repository configuration
	cfg, err := repocfg.LoadConfigFromFile(stagingFile.RepoPath)
	if err != nil {
		return "", fmt.Errorf("failed to load repository config: %w", err)
	}

	// Resolve inbox path based on repository configuration
	inboxPath, err := sm.resolveInboxRelativePath(stagingFile.RepoPath, cfg, stagingFile.Filename, hash)
	if err != nil {
		return "", fmt.Errorf("failed to resolve inbox path: %w", err)
	}

	// Commit to the resolved inbox path
	if err := sm.CommitStagingFile(stagingFile, inboxPath); err != nil {
		return "", err
	}
	return inboxPath, nil
}

// MoveStagingToFailed moves a staging file to the failed directory
func (sm *DefaultStagingManager) MoveStagingToFailed(stagingFile *StagingFile) error {
	if stagingFile == nil {
		return fmt.Errorf("staging file is nil")
	}

	// Resolve failed path
	failedPath, err := sm.resolveFailedPath(stagingFile.RepoPath, stagingFile.Filename)
	if err != nil {
		return fmt.Errorf("failed to resolve failed path: %w", err)
	}

	// Move to failed directory
	if err := sm.CommitStagingFile(stagingFile, failedPath); err != nil {
		return fmt.Errorf("failed to move staging file to failed directory: %w", err)
	}
	return nil
}

// resolveFailedPath resolves a timestamped target path under the failed area.
func (sm *DefaultStagingManager) resolveFailedPath(repoPath string, originalFilename string) (string, error) {
	failedDir := filepath.Join(repoPath, DefaultStructure.FailedDir)
	if err := os.MkdirAll(failedDir, 0700); err != nil {
		return "", fmt.Errorf("failed to create failed directory: %w", err)
	}

	// Use timestamp to avoid filename conflicts
	timestamp := time.Now().Format("20060102_150405")
	base := filepath.Base(originalFilename)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	failedFilename := fmt.Sprintf("%s_%s%s", name, timestamp, ext)

	return filepath.Join(DefaultStructure.FailedDir, failedFilename), nil
}

// ResolveInboxPath computes (without moving) the inbox-relative target path for a
// file under the repository's storage strategy. Kept off the interface; used for
// inspection and tests. Note: cas/date strategies create the target directory as
// a side effect.
func (sm *DefaultStagingManager) ResolveInboxPath(repoPath string, originalFilename, hash string) (string, error) {
	cfg, err := repocfg.LoadConfigFromFile(repoPath)
	if err != nil {
		return "", fmt.Errorf("failed to load repository config: %w", err)
	}
	return sm.resolveInboxRelativePath(repoPath, cfg, originalFilename, hash)
}

// CleanupStaging removes staged files (incoming and failed) older than maxAge.
func (sm *DefaultStagingManager) CleanupStaging(repoPath string, maxAge time.Duration) error {
	cleanRepoPath, err := filepath.Abs(filepath.Clean(repoPath))
	if err != nil {
		return fmt.Errorf("invalid repository path: %w", err)
	}
	cutoff := time.Now().Add(-maxAge)

	dirs := []string{
		filepath.Join(cleanRepoPath, DefaultStructure.IncomingDir),
		filepath.Join(cleanRepoPath, DefaultStructure.FailedDir),
	}

	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("failed to read staging directory %s: %w", dir, err)
		}
		for _, e := range entries {
			info, err := e.Info()
			if err != nil {
				continue
			}
			if info.ModTime().Before(cutoff) {
				_ = os.Remove(filepath.Join(dir, e.Name()))
			}
		}
	}

	return nil
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

	switch strategy {
	case "flat":
		// inbox/<filename>
		filename := sm.uniqueInboxFilename(inboxRoot, originalFilename, duplicateMode)
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
