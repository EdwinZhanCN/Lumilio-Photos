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
	FacesDir      string // .lumilio/assets/faces
	SidecarsDir   string // .lumilio/sidecars
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
	FacesDir:      ".lumilio/assets/faces",
	SidecarsDir:   ".lumilio/sidecars",
	StagingDir:    ".lumilio/staging",
	TempDir:       ".lumilio/temp",
	TrashDir:      ".lumilio/trash",
	IncomingDir:   ".lumilio/staging/incoming",
	FailedDir:     ".lumilio/staging/failed",
}

// dirSpec is one directory in a repository's layout: its repo-relative path and
// the permission enforced on it.
type dirSpec struct {
	path string
	mode os.FileMode
}

// repoDirs is the single source of truth for a repository's on-disk layout: the
// directories created at init, checked by ValidateStructure, and permission-
// enforced by protectSystemDirectories. Staging and temp are application-only
// (0700); everything else is world-readable (0755).
var repoDirs = []dirSpec{
	{".lumilio", 0o755},
	{".lumilio/assets", 0o755},
	{".lumilio/assets/thumbnails", 0o755},
	{".lumilio/assets/thumbnails/small", 0o755},
	{".lumilio/assets/thumbnails/medium", 0o755},
	{".lumilio/assets/thumbnails/large", 0o755},
	{".lumilio/assets/videos", 0o755},
	{".lumilio/assets/videos/web", 0o755},
	{".lumilio/assets/audios", 0o755},
	{".lumilio/assets/audios/web", 0o755},
	{".lumilio/assets/faces", 0o755},
	{".lumilio/sidecars", 0o755}, // Studio non-destructive edit sidecar files
	{".lumilio/staging", 0o700},
	{".lumilio/staging/incoming", 0o700},
	{".lumilio/staging/failed", 0o700},
	{".lumilio/temp", 0o700},  // General temporary processing
	{".lumilio/trash", 0o755}, // Soft-deleted user assets
	{".lumilio/logs", 0o755},  // Application and operation logs
	{"inbox", 0o755},          // Structured uploads
}

// repoLogFiles are empty JSONL targets created at init so loggers can append
// valid lines immediately.
var repoLogFiles = []string{
	".lumilio/logs/app.log",
	".lumilio/logs/error.log",
	".lumilio/logs/operations.log",
}

// DirectoryManager owns the structure *inside* a single repository (the
// .lumilio/* system tree and inbox) and the file operations over it. All paths
// are repo-relative and resolved under repoPath; operations never escape the
// repository root. It does not deal with the storage root layout
// (<path>/.secrets, <path>/.cloud) — that is the storage package's provisioning
// concern. Staging is owned by StagingManager, not here.
//
// Implementations are stateless and safe for concurrent use across different
// repositories.
type DirectoryManager interface {
	// CreateStructure creates the full repository directory tree (repoDirs) with
	// their enforced permissions and the empty log files. It is safe to call on
	// an existing repository (directories already present are left intact).
	CreateStructure(repoPath string) error

	// ValidateStructure reports the structural health of a repository. Missing
	// directories are returned as warnings with Valid still true (they are
	// recoverable via RepairStructure); a file where a directory is expected, a
	// missing/non-directory root, or a permission problem set Valid to false.
	ValidateStructure(repoPath string) (*StructureValidation, error)

	// CreateTempFile creates an empty file under .lumilio/temp for transient
	// processing, named by purpose. Callers are responsible for removing it.
	CreateTempFile(repoPath, purpose string) (*TempFile, error)

	// MoveToTrash moves a repo file into .lumilio/trash and writes a sidecar JSON
	// of metadata. filePath must resolve inside the repository.
	MoveToTrash(repoPath, filePath string, metadata *DeleteMetadata) error

	// ReadSidecar returns the raw sidecar bytes for an asset, or (nil, nil) when
	// no sidecar exists. WriteSidecar writes it atomically (temp file + rename).
	ReadSidecar(repoPath, assetID string) ([]byte, error)
	WriteSidecar(repoPath, assetID string, data []byte) error
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

// NewDirectoryManager creates a new directory manager instance.
func NewDirectoryManager() *DefaultDirectoryManager {
	return &DefaultDirectoryManager{}
}

// Ensure the concrete type satisfies the consumer interface. Methods kept off
// the interface (RepairStructure, IsProtectedPath, CleanupTempFiles, the trash
// listing/recovery/purge, protectSystemDirectories) remain available on the
// concrete type for maintenance use and tests.
var _ DirectoryManager = (*DefaultDirectoryManager)(nil)

// CreateStructure creates the complete directory structure for a repository
func (dm *DefaultDirectoryManager) CreateStructure(repoPath string) error {
	cleanPath, err := filepath.Abs(filepath.Clean(repoPath))
	if err != nil {
		return fmt.Errorf("invalid repository path: %w", err)
	}

	// Create all required directories
	for _, d := range repoDirs {
		dirPath := filepath.Join(cleanPath, d.path)
		if err := os.MkdirAll(dirPath, d.mode); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", d.path, err)
		}
	}

	// Create empty log files so JSON loggers can append valid lines immediately.
	for _, logFile := range repoLogFiles {
		logPath := filepath.Join(cleanPath, logFile)
		if err := os.WriteFile(logPath, nil, 0644); err != nil {
			return fmt.Errorf("failed to create log file %s: %w", logFile, err)
		}
	}

	// Set initial permissions
	if err := dm.protectSystemDirectories(cleanPath); err != nil {
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
	for _, d := range repoDirs {
		dirPath := filepath.Join(cleanPath, d.path)
		if info, err := os.Stat(dirPath); os.IsNotExist(err) {
			validation.MissingDirectories = append(validation.MissingDirectories, d.path)
			validation.Warnings = append(validation.Warnings, fmt.Sprintf("Missing directory: %s", d.path))
		} else if err == nil && !info.IsDir() {
			validation.Valid = false
			validation.InvalidPaths = append(validation.InvalidPaths, fmt.Sprintf("Expected directory but found file: %s", d.path))
		} else if err != nil {
			validation.PermissionIssues = append(validation.PermissionIssues, fmt.Sprintf("Cannot access directory %s: %v", d.path, err))
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

	// Recreate missing directories at their enforced permission.
	for _, missingDir := range validation.MissingDirectories {
		mode := os.FileMode(0o755)
		for _, d := range repoDirs {
			if d.path == missingDir {
				mode = d.mode
				break
			}
		}
		dirPath := filepath.Join(cleanPath, missingDir)
		if err := os.MkdirAll(dirPath, mode); err != nil {
			return fmt.Errorf("failed to recreate directory %s: %w", missingDir, err)
		}
	}

	// Recreate missing log files as empty JSONL targets.
	for _, logFile := range repoLogFiles {
		logPath := filepath.Join(cleanPath, logFile)
		if _, err := os.Stat(logPath); os.IsNotExist(err) {
			if err := os.WriteFile(logPath, nil, 0644); err != nil {
				return fmt.Errorf("failed to recreate log file %s: %w", logFile, err)
			}
		}
	}

	// Fix permissions
	if err := dm.protectSystemDirectories(cleanPath); err != nil {
		return fmt.Errorf("failed to fix permissions: %w", err)
	}

	return nil
}

// protectSystemDirectories enforces each directory's permission from repoDirs
// (the single layout source). Directories that do not yet exist are skipped.
func (dm *DefaultDirectoryManager) protectSystemDirectories(repoPath string) error {
	cleanPath, err := filepath.Abs(filepath.Clean(repoPath))
	if err != nil {
		return fmt.Errorf("invalid repository path: %w", err)
	}

	for _, d := range repoDirs {
		dirPath := filepath.Join(cleanPath, d.path)
		if _, err := os.Stat(dirPath); err == nil {
			if err := os.Chmod(dirPath, d.mode); err != nil {
				return fmt.Errorf("failed to set permissions for %s: %w", d.path, err)
			}
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

// CreateTempFile creates a new temporary file
func (dm *DefaultDirectoryManager) CreateTempFile(repoPath, purpose string) (*TempFile, error) {
	cleanRepoPath, err := filepath.Abs(filepath.Clean(repoPath))
	if err != nil {
		return nil, fmt.Errorf("invalid repository path: %w", err)
	}

	tempDir := filepath.Join(cleanRepoPath, DefaultStructure.TempDir)
	if err := os.MkdirAll(tempDir, 0700); err != nil {
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

	originalFull, err := resolveInRepo(cleanRepoPath, filePath)
	if err != nil {
		return err
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

	destFull, err := resolveInRepo(cleanRepoPath, originalRel)
	if err != nil {
		return fmt.Errorf("cannot recover trash item %s: %w", trashID, err)
	}
	if _, err := os.Stat(destFull); err == nil {
		return fmt.Errorf("cannot recover trash item %s: destination %s already exists", trashID, originalRel)
	}
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

// sidecarPath returns the full path for an asset's sidecar file. assetID is
// reduced to its base name so it can never escape the sidecars directory.
func (dm *DefaultDirectoryManager) sidecarPath(repoPath, assetID string) string {
	safeID := filepath.Base(filepath.Clean(assetID))
	return filepath.Join(repoPath, DefaultStructure.SidecarsDir, safeID+".lumilio-sidecar")
}

// ReadSidecar reads the raw content of an asset's sidecar file.
func (dm *DefaultDirectoryManager) ReadSidecar(repoPath, assetID string) ([]byte, error) {
	path := dm.sidecarPath(repoPath, assetID)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // no sidecar is not an error
		}
		return nil, fmt.Errorf("failed to read sidecar: %w", err)
	}
	return data, nil
}

// WriteSidecar atomically writes the sidecar data using a temp-file+rename strategy.
func (dm *DefaultDirectoryManager) WriteSidecar(repoPath, assetID string, data []byte) error {
	dir := filepath.Join(repoPath, DefaultStructure.SidecarsDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to prepare sidecar directory: %w", err)
	}

	targetPath := dm.sidecarPath(repoPath, assetID)
	tempPath := targetPath + ".tmp"

	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write sidecar temp file: %w", err)
	}
	if err := os.Rename(tempPath, targetPath); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("failed to save sidecar file: %w", err)
	}
	return nil
}

// resolveInRepo resolves a repo-relative or absolute path to a cleaned absolute
// path and verifies it stays within repoRoot, rejecting traversal escapes.
func resolveInRepo(repoRoot, p string) (string, error) {
	root, err := filepath.Abs(filepath.Clean(repoRoot))
	if err != nil {
		return "", fmt.Errorf("invalid repository path: %w", err)
	}
	full := filepath.Clean(p)
	if !filepath.IsAbs(full) {
		full = filepath.Join(root, full)
	}
	rel, err := filepath.Rel(root, full)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q escapes repository root", p)
	}
	return full, nil
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
