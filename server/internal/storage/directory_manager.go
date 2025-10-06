package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

// DirectoryStructure defines the standard repository directory layout
type DirectoryStructure struct {
	SystemDir  string // .lumilio
	ConfigFile string // .lumiliorepo
	InboxDir   string // inbox

	// System subdirectories
	AssetsDir     string // .lumilio/assets
	ThumbnailsDir string // .lumilio/assets/thumbnails
	VideosDir     string // .lumilio/assets/videos
	AudiosDir     string // .lumilio/assets/audios
	StagingDir    string // .lumilio/staging
	TempDir       string // .lumilio/temp
	TrashDir      string // .lumilio/trash

	// Staging subdirectories
	IncomingDir string // .lumilio/staging/incoming
	FailedDir   string // .lumilio/staging/failed
}

// DefaultStructure provides the default directory structure configuration
var DefaultStructure = DirectoryStructure{
	SystemDir:     ".lumilio",
	ConfigFile:    ".lumiliorepo",
	InboxDir:      "inbox",
	AssetsDir:     ".lumilio/assets",
	ThumbnailsDir: ".lumilio/assets/thumbnails",
	VideosDir:     ".lumilio/assets/videos",
	AudiosDir:     ".lumilio/assets/audios",
	StagingDir:    ".lumilio/staging",
	TempDir:       ".lumilio/temp",
	TrashDir:      ".lumilio/trash",
	IncomingDir:   ".lumilio/staging/incoming",
	FailedDir:     ".lumilio/staging/failed",
}

// Directories lists all directories that should be created in a repository
var Directories = []string{
	".lumilio",
	".lumilio/assets",
	".lumilio/assets/thumbnails",
	".lumilio/assets/thumbnails/small",
	".lumilio/assets/thumbnails/medium",
	".lumilio/assets/thumbnails/large",
	".lumilio/assets/videos",
	".lumilio/assets/videos/web",
	".lumilio/assets/audios",
	".lumilio/assets/audios/web",
	".lumilio/staging",          // Upload staging area
	".lumilio/staging/incoming", // Upload staging area
	".lumilio/staging/failed",   // Upload staging area
	".lumilio/temp",             // General temporary processing
	".lumilio/trash",            // Soft-deleted user assets
	".lumilio/logs",             // Application and operation logs
	".lumilio/backups",          // Config version backups
	"inbox",                     // Structured uploads
}

// DirectoryManager handles the physical directory structure and system management for repositories
type DirectoryManager interface {
	// Structure management
	CreateStructure(repoPath string) error
	ValidateStructure(repoPath string) (*StructureValidation, error)
	RepairStructure(repoPath string) error

	// Protection and permissions
	ProtectSystemDirectories(repoPath string) error
	IsProtectedPath(repoPath, filePath string) bool
	EnforcePermissions(repoPath string) error

	// Staging operations
	CreateStagingFile(repoPath, filename string) (*StagingFile, error)
	CommitStagingFile(stagingFile *StagingFile, finalPath string) error
	CleanupStaging(repoPath string, maxAge time.Duration) error

	// Temporary file management
	CreateTempFile(repoPath, purpose string) (*TempFile, error)
	CleanupTempFiles(repoPath string, maxAge time.Duration) error

	// Trash operations
	MoveToTrash(repoPath, filePath string, metadata *DeleteMetadata) error
	ListTrashFiles(repoPath string) ([]*TrashFile, error)
	RecoverFromTrash(repoPath, trashID string) error
	PurgeTrash(repoPath string, olderThan time.Duration) error
}

// StructureValidation represents the result of directory structure validation
type StructureValidation struct {
	Valid              bool     `json:"valid"`
	MissingDirectories []string `json:"missing_directories,omitempty"`
	InvalidPaths       []string `json:"invalid_paths,omitempty"`
	PermissionIssues   []string `json:"permission_issues,omitempty"`
	Warnings           []string `json:"warnings,omitempty"`
}

// StagingFile represents a file in the staging area
type StagingFile struct {
	ID        string    `json:"id"`
	RepoPath  string    `json:"repo_path"`
	Path      string    `json:"path"`
	Filename  string    `json:"filename"`
	CreatedAt time.Time `json:"created_at"`
}

// TempFile represents a temporary processing file
type TempFile struct {
	ID        string    `json:"id"`
	RepoPath  string    `json:"repo_path"`
	Path      string    `json:"path"`
	Purpose   string    `json:"purpose"`
	CreatedAt time.Time `json:"created_at"`
}

// DeleteMetadata contains metadata about deleted files
type DeleteMetadata struct {
	DeletedAt    time.Time              `json:"deleted_at"`
	OriginalPath string                 `json:"original_path"`
	Reason       string                 `json:"reason,omitempty"`
	AssetID      *string                `json:"asset_id,omitempty"`
	UserID       *string                `json:"user_id,omitempty"`
	Extra        map[string]interface{} `json:"extra,omitempty"`
}

// TrashFile represents a file in the trash system
type TrashFile struct {
	ID        string          `json:"id"`
	RepoPath  string          `json:"repo_path"`
	TrashPath string          `json:"trash_path"`
	Metadata  *DeleteMetadata `json:"metadata,omitempty"`
}

// DefaultDirectoryManager implements the DirectoryManager interface
type DefaultDirectoryManager struct{}

// NewDirectoryManager creates a new directory manager instance
func NewDirectoryManager() DirectoryManager {
	return &DefaultDirectoryManager{}
}

// CreateStructure creates the complete directory structure for a repository
func (dm *DefaultDirectoryManager) CreateStructure(repoPath string) error {
	cleanPath, err := filepath.Abs(filepath.Clean(repoPath))
	if err != nil {
		return fmt.Errorf("invalid repository path: %w", err)
	}

	// Create all required directories
	for _, dir := range Directories {
		dirPath := filepath.Join(cleanPath, dir)
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Create initial log files
	logFiles := map[string]string{
		".lumilio/logs/app.log":        "# Lumilio application logs\n",
		".lumilio/logs/error.log":      "# Lumilio error logs\n",
		".lumilio/logs/operations.log": "# Lumilio operations logs\n",
	}

	for logFile, content := range logFiles {
		logPath := filepath.Join(cleanPath, logFile)
		if err := os.WriteFile(logPath, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to create log file %s: %w", logFile, err)
		}
	}

	// Set initial permissions
	if err := dm.ProtectSystemDirectories(cleanPath); err != nil {
		return fmt.Errorf("failed to set directory permissions: %w", err)
	}

	return nil
}

// ValidateStructure validates the directory structure of a repository
func (dm *DefaultDirectoryManager) ValidateStructure(repoPath string) (*StructureValidation, error) {
	cleanPath, err := filepath.Abs(filepath.Clean(repoPath))
	if err != nil {
		return nil, fmt.Errorf("invalid repository path: %w", err)
	}

	validation := &StructureValidation{
		Valid:              true,
		MissingDirectories: []string{},
		InvalidPaths:       []string{},
		PermissionIssues:   []string{},
		Warnings:           []string{},
	}

	// Check if repository root exists
	if info, err := os.Stat(cleanPath); os.IsNotExist(err) {
		validation.Valid = false
		validation.InvalidPaths = append(validation.InvalidPaths, "Repository root does not exist")
		return validation, nil
	} else if err == nil && !info.IsDir() {
		validation.Valid = false
		validation.InvalidPaths = append(validation.InvalidPaths, "Repository root is not a directory")
		return validation, nil
	}

	// Validate each required directory
	for _, dir := range Directories {
		dirPath := filepath.Join(cleanPath, dir)
		if info, err := os.Stat(dirPath); os.IsNotExist(err) {
			validation.MissingDirectories = append(validation.MissingDirectories, dir)
			validation.Warnings = append(validation.Warnings, fmt.Sprintf("Missing directory: %s", dir))
		} else if err == nil && !info.IsDir() {
			validation.Valid = false
			validation.InvalidPaths = append(validation.InvalidPaths, fmt.Sprintf("Expected directory but found file: %s", dir))
		} else if err != nil {
			validation.PermissionIssues = append(validation.PermissionIssues, fmt.Sprintf("Cannot access directory %s: %v", dir, err))
		}
	}

	// Check permissions on critical directories
	protectedDirs := []string{
		DefaultStructure.SystemDir,
		DefaultStructure.InboxDir,
	}

	for _, dir := range protectedDirs {
		dirPath := filepath.Join(cleanPath, dir)
		if err := dm.checkDirectoryPermissions(dirPath); err != nil {
			validation.PermissionIssues = append(validation.PermissionIssues, fmt.Sprintf("Permission issue with %s: %v", dir, err))
		}
	}

	// If there are missing directories but no critical errors, still consider valid but with warnings
	if len(validation.InvalidPaths) > 0 || len(validation.PermissionIssues) > 0 {
		validation.Valid = false
	}

	return validation, nil
}

// RepairStructure recreates missing directories and fixes permissions
func (dm *DefaultDirectoryManager) RepairStructure(repoPath string) error {
	cleanPath, err := filepath.Abs(filepath.Clean(repoPath))
	if err != nil {
		return fmt.Errorf("invalid repository path: %w", err)
	}

	// Validate current structure to identify issues
	validation, err := dm.ValidateStructure(cleanPath)
	if err != nil {
		return fmt.Errorf("failed to validate structure: %w", err)
	}

	// Recreate missing directories
	for _, missingDir := range validation.MissingDirectories {
		dirPath := filepath.Join(cleanPath, missingDir)
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			return fmt.Errorf("failed to recreate directory %s: %w", missingDir, err)
		}
	}

	// Recreate missing log files
	logFiles := map[string]string{
		".lumilio/logs/app.log":        "# Lumilio application logs\n",
		".lumilio/logs/error.log":      "# Lumilio error logs\n",
		".lumilio/logs/operations.log": "# Lumilio operations logs\n",
	}

	for logFile, content := range logFiles {
		logPath := filepath.Join(cleanPath, logFile)
		if _, err := os.Stat(logPath); os.IsNotExist(err) {
			if err := os.WriteFile(logPath, []byte(content), 0644); err != nil {
				return fmt.Errorf("failed to recreate log file %s: %w", logFile, err)
			}
		}
	}

	// Fix permissions
	if err := dm.ProtectSystemDirectories(cleanPath); err != nil {
		return fmt.Errorf("failed to fix permissions: %w", err)
	}

	return nil
}

// ProtectSystemDirectories sets appropriate permissions for system directories
func (dm *DefaultDirectoryManager) ProtectSystemDirectories(repoPath string) error {
	cleanPath, err := filepath.Abs(filepath.Clean(repoPath))
	if err != nil {
		return fmt.Errorf("invalid repository path: %w", err)
	}

	// System directories should be readable by users but only writable by the application
	systemDirs := map[string]os.FileMode{
		DefaultStructure.SystemDir:     0755, // rwxr-xr-x
		DefaultStructure.AssetsDir:     0755, // rwxr-xr-x
		DefaultStructure.ThumbnailsDir: 0755, // rwxr-xr-x
		DefaultStructure.VideosDir:     0755, // rwxr-xr-x
		DefaultStructure.AudiosDir:     0755, // rwxr-xr-x
		DefaultStructure.StagingDir:    0700, // rwx------ (app-only)
		DefaultStructure.TempDir:       0700, // rwx------
		DefaultStructure.TrashDir:      0755, // rwxr-xr-x
	}

	for dir, mode := range systemDirs {
		dirPath := filepath.Join(cleanPath, dir)
		if _, err := os.Stat(dirPath); err == nil {
			if err := os.Chmod(dirPath, mode); err != nil {
				return fmt.Errorf("failed to set permissions for %s: %w", dir, err)
			}
		}
	}

	// Inbox should be read-only for users (application manages content)
	inboxPath := filepath.Join(cleanPath, DefaultStructure.InboxDir)
	if _, err := os.Stat(inboxPath); err == nil {
		if err := os.Chmod(inboxPath, 0755); err != nil {
			return fmt.Errorf("failed to set inbox permissions: %w", err)
		}
	}

	return nil
}

// IsProtectedPath checks if a file path is in a protected area
func (dm *DefaultDirectoryManager) IsProtectedPath(repoPath, filePath string) bool {
	cleanRepoPath, err := filepath.Abs(filepath.Clean(repoPath))
	if err != nil {
		return false
	}

	var targetPath string
	if filepath.IsAbs(filePath) {
		targetPath = filePath
	} else {
		targetPath = filepath.Join(cleanRepoPath, filePath)
	}

	// Make path relative to repository root for comparison
	relPath, err := filepath.Rel(cleanRepoPath, targetPath)
	if err != nil {
		return false
	}

	// Normalize path separators for comparison
	relPath = filepath.ToSlash(relPath)

	// Protected areas
	protectedPrefixes := []string{
		".lumilio/",
		"inbox/",
	}

	for _, prefix := range protectedPrefixes {
		if strings.HasPrefix(relPath, prefix) || relPath == strings.TrimSuffix(prefix, "/") {
			return true
		}
	}

	return false
}

// EnforcePermissions ensures proper permissions are maintained
func (dm *DefaultDirectoryManager) EnforcePermissions(repoPath string) error {
	return dm.ProtectSystemDirectories(repoPath)
}

// CreateStagingFile creates a new file in the staging area
func (dm *DefaultDirectoryManager) CreateStagingFile(repoPath, filename string) (*StagingFile, error) {
	cleanRepoPath, err := filepath.Abs(filepath.Clean(repoPath))
	if err != nil {
		return nil, fmt.Errorf("invalid repository path: %w", err)
	}

	stagingDir := filepath.Join(cleanRepoPath, DefaultStructure.IncomingDir)
	if err := os.MkdirAll(stagingDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create staging directory: %w", err)
	}

	id := uuid.New().String()
	base := filepath.Base(filename)
	stagingName := fmt.Sprintf("%s_%s", id, base)
	stagingFullPath := filepath.Join(stagingDir, stagingName)

	// Create empty file placeholder
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

// CommitStagingFile moves a staging file to its final destination
func (dm *DefaultDirectoryManager) CommitStagingFile(stagingFile *StagingFile, finalPath string) error {
	if stagingFile == nil {
		return fmt.Errorf("staging file is nil")
	}

	// Validate that finalPath is provided
	if strings.TrimSpace(finalPath) == "" {
		return fmt.Errorf("final path cannot be empty")
	}

	// finalPath must be repo-relative
	if filepath.IsAbs(finalPath) {
		return fmt.Errorf("final path must be repository-relative")
	}

	destFullPath := filepath.Join(stagingFile.RepoPath, finalPath)
	destDir := filepath.Dir(destFullPath)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	if err := os.Rename(stagingFile.Path, destFullPath); err != nil {
		return fmt.Errorf("failed to move staged file: %w", err)
	}
	return nil
}

// CleanupStaging removes old staging files
func (dm *DefaultDirectoryManager) CleanupStaging(repoPath string, maxAge time.Duration) error {
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

// CreateTempFile creates a new temporary file
func (dm *DefaultDirectoryManager) CreateTempFile(repoPath, purpose string) (*TempFile, error) {
	cleanRepoPath, err := filepath.Abs(filepath.Clean(repoPath))
	if err != nil {
		return nil, fmt.Errorf("invalid repository path: %w", err)
	}

	tempDir := filepath.Join(cleanRepoPath, DefaultStructure.TempDir)
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	id := uuid.New().String()
	filename := fmt.Sprintf("%s_%s.tmp", purpose, id)
	fullPath := filepath.Join(tempDir, filename)

	f, err := os.Create(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	_ = f.Close()

	return &TempFile{
		ID:        id,
		RepoPath:  cleanRepoPath,
		Path:      fullPath,
		Purpose:   purpose,
		CreatedAt: time.Now(),
	}, nil
}

// CleanupTempFiles removes old temporary files
func (dm *DefaultDirectoryManager) CleanupTempFiles(repoPath string, maxAge time.Duration) error {
	cleanRepoPath, err := filepath.Abs(filepath.Clean(repoPath))
	if err != nil {
		return fmt.Errorf("invalid repository path: %w", err)
	}
	tempDir := filepath.Join(cleanRepoPath, DefaultStructure.TempDir)
	cutoff := time.Now().Add(-maxAge)

	entries, err := os.ReadDir(tempDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read temp directory: %w", err)
	}

	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			_ = os.Remove(filepath.Join(tempDir, e.Name()))
		}
	}

	return nil
}

// MoveToTrash moves a file to the trash with metadata
func (dm *DefaultDirectoryManager) MoveToTrash(repoPath, filePath string, metadata *DeleteMetadata) error {
	cleanRepoPath, err := filepath.Abs(filepath.Clean(repoPath))
	if err != nil {
		return fmt.Errorf("invalid repository path: %w", err)
	}

	var originalFull string
	if filepath.IsAbs(filePath) {
		originalFull = filePath
	} else {
		originalFull = filepath.Join(cleanRepoPath, filePath)
	}

	if _, err := os.Stat(originalFull); err != nil {
		return fmt.Errorf("original file not found: %w", err)
	}

	trashDir := filepath.Join(cleanRepoPath, DefaultStructure.TrashDir)
	if err := os.MkdirAll(trashDir, 0755); err != nil {
		return fmt.Errorf("failed to create trash directory: %w", err)
	}

	id := uuid.New().String()
	trashName := fmt.Sprintf("%s_%s", id, filepath.Base(originalFull))
	trashFullPath := filepath.Join(trashDir, trashName)

	// Move the file into trash
	if err := os.Rename(originalFull, trashFullPath); err != nil {
		return fmt.Errorf("failed to move file to trash: %w", err)
	}

	// Write metadata
	if metadata == nil {
		rel, _ := filepath.Rel(cleanRepoPath, originalFull)
		metadata = &DeleteMetadata{
			DeletedAt:    time.Now(),
			OriginalPath: rel,
		}
	} else {
		// Ensure OriginalPath is set if not provided
		if metadata.OriginalPath == "" {
			rel, _ := filepath.Rel(cleanRepoPath, originalFull)
			metadata.OriginalPath = rel
		}
		// Ensure DeletedAt is set if not provided
		if metadata.DeletedAt.IsZero() {
			metadata.DeletedAt = time.Now()
		}
	}
	metaBytes, err := json.MarshalIndent(metadata, "", "  ")
	if err == nil {
		_ = os.WriteFile(trashFullPath+".json", metaBytes, 0644)
	}

	return nil
}

// ListTrashFiles returns all files currently in trash
func (dm *DefaultDirectoryManager) ListTrashFiles(repoPath string) ([]*TrashFile, error) {
	cleanRepoPath, err := filepath.Abs(filepath.Clean(repoPath))
	if err != nil {
		return nil, fmt.Errorf("invalid repository path: %w", err)
	}
	trashDir := filepath.Join(cleanRepoPath, DefaultStructure.TrashDir)

	entries, err := os.ReadDir(trashDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*TrashFile{}, nil
		}
		return nil, fmt.Errorf("failed to read trash directory: %w", err)
	}

	var results []*TrashFile
	for _, e := range entries {
		// Skip metadata files
		if e.IsDir() || filepath.Ext(e.Name()) == ".json" {
			continue
		}
		id := e.Name()
		if underscore := len(e.Name()); underscore > 0 {
			// Derive ID from prefix before first underscore
			for i := 0; i < len(e.Name()); i++ {
				if e.Name()[i] == '_' {
					id = e.Name()[:i]
					break
				}
			}
		}
		tf := &TrashFile{
			ID:        id,
			RepoPath:  cleanRepoPath,
			TrashPath: filepath.Join(trashDir, e.Name()),
		}
		// Load metadata if present
		metaPath := filepath.Join(trashDir, e.Name()) + ".json"
		if b, err := os.ReadFile(metaPath); err == nil {
			var dm DeleteMetadata
			if err := json.Unmarshal(b, &dm); err == nil {
				tf.Metadata = &dm
			}
		}
		results = append(results, tf)
	}

	return results, nil
}

// RecoverFromTrash restores a file from trash to its original location
func (dm *DefaultDirectoryManager) RecoverFromTrash(repoPath, trashID string) error {
	cleanRepoPath, err := filepath.Abs(filepath.Clean(repoPath))
	if err != nil {
		return fmt.Errorf("invalid repository path: %w", err)
	}
	trashDir := filepath.Join(cleanRepoPath, DefaultStructure.TrashDir)

	entries, err := os.ReadDir(trashDir)
	if err != nil {
		return fmt.Errorf("failed to read trash directory: %w", err)
	}

	var trashFileName string
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) == ".json" {
			continue
		}
		// Match by ID prefix before underscore
		name := e.Name()
		for i := 0; i < len(name); i++ {
			if name[i] == '_' {
				if name[:i] == trashID {
					trashFileName = name
				}
				break
			}
		}
	}
	if trashFileName == "" {
		return fmt.Errorf("trash item %s not found", trashID)
	}

	trashFull := filepath.Join(trashDir, trashFileName)
	metaPath := trashFull + ".json"

	// Load metadata to determine original path
	var originalRel string
	if b, err := os.ReadFile(metaPath); err == nil {
		var dm DeleteMetadata
		if err := json.Unmarshal(b, &dm); err == nil {
			originalRel = dm.OriginalPath
		}
	}
	if originalRel == "" {
		return fmt.Errorf("cannot recover trash item %s: missing original path metadata", trashID)
	}

	destFull := filepath.Join(cleanRepoPath, originalRel)
	if err := os.MkdirAll(filepath.Dir(destFull), 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	if err := os.Rename(trashFull, destFull); err != nil {
		return fmt.Errorf("failed to restore file from trash: %w", err)
	}
	_ = os.Remove(metaPath)

	return nil
}

// PurgeTrash permanently deletes old files from trash
func (dm *DefaultDirectoryManager) PurgeTrash(repoPath string, olderThan time.Duration) error {
	cleanRepoPath, err := filepath.Abs(filepath.Clean(repoPath))
	if err != nil {
		return fmt.Errorf("invalid repository path: %w", err)
	}
	trashDir := filepath.Join(cleanRepoPath, DefaultStructure.TrashDir)
	cutoff := time.Now().Add(-olderThan)

	entries, err := os.ReadDir(trashDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read trash directory: %w", err)
	}

	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) == ".json" {
			continue
		}
		full := filepath.Join(trashDir, e.Name())
		info, err := os.Stat(full)
		if err != nil {
			continue
		}

		// Prefer metadata DeletedAt if present
		shouldDelete := false
		if b, err := os.ReadFile(full + ".json"); err == nil {
			var dm DeleteMetadata
			if err := json.Unmarshal(b, &dm); err == nil {
				if dm.DeletedAt.Before(cutoff) {
					shouldDelete = true
				}
			}
		}
		if !shouldDelete && info.ModTime().Before(cutoff) {
			shouldDelete = true
		}

		if shouldDelete {
			_ = os.Remove(full)
			_ = os.Remove(full + ".json")
		}
	}

	return nil
}

// checkDirectoryPermissions checks if we have proper read/write permissions
func (dm *DefaultDirectoryManager) checkDirectoryPermissions(path string) error {
	// Test read permission
	if _, err := os.Open(path); err != nil {
		return fmt.Errorf("cannot read directory: %w", err)
	}

	// Test write permission by trying to create a temporary file
	tempFile := filepath.Join(path, ".lumilio_permission_test")
	file, err := os.Create(tempFile)
	if err != nil {
		return fmt.Errorf("cannot write to directory: %w", err)
	}
	file.Close()
	os.Remove(tempFile) // Clean up

	return nil
}
