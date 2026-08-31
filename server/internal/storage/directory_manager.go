package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

// DirectoryStructure defines the standard repository directory layout
type DirectoryStructure struct {
	SystemDir  string // .lumilio
	ConfigFile string // .lumiliorepo
	InboxDir   string // inbox

	// System subdirectories
	FacesDir    string // .lumilio/assets/faces
	SidecarsDir string // .lumilio/sidecars
	StagingDir  string // .lumilio/staging

	// Staging subdirectories
	IncomingDir string // .lumilio/staging/incoming
	FailedDir   string // .lumilio/staging/failed
}

// DefaultStructure provides the default directory structure configuration
var DefaultStructure = DirectoryStructure{
	SystemDir:   ".lumilio",
	ConfigFile:  ".lumiliorepo",
	InboxDir:    "inbox",
	FacesDir:    ".lumilio/assets/faces",
	SidecarsDir: ".lumilio/sidecars",
	StagingDir:  ".lumilio/staging",
	IncomingDir: ".lumilio/staging/incoming",
	FailedDir:   ".lumilio/staging/failed",
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
	{".lumilio/assets/faces", 0o755},
	{".lumilio/sidecars", 0o755}, // Studio non-destructive edit sidecar files
	{".lumilio/staging", 0o700},
	{".lumilio/staging/incoming", 0o700},
	{".lumilio/staging/failed", 0o700},
	{".lumilio/logs", 0o755}, // Repository operation and error audit logs
	{"inbox", 0o755},         // Structured uploads
}

// repoLogFiles are empty JSONL targets created at init so loggers can append
// valid lines immediately.
var repoLogFiles = []string{
	".lumilio/logs/error.log",
	".lumilio/logs/operations.log",
}

// DirectoryManager owns the structure *inside* a single repository (the
// .lumilio/* system tree and inbox) and the file operations over it. All paths
// are repo-relative and resolved under repoPath; operations never escape the
// repository root. It does not deal with the default Storage Location or the
// separately configured app-private directories; provisioning owns those.
// Staging is owned by StagingManager, not here.
//
// Implementations are stateless and safe for concurrent use across different
// repositories.
type DirectoryManager interface {
	// CreateStructure creates the full repository directory tree (repoDirs) with
	// their enforced permissions and the empty log files. It is safe to call on
	// an existing repository (directories already present are left intact).
	CreateStructure(repoPath string) error

	// ValidateStructure reports the structural health of a repository. Missing
	// directories are returned as warnings; a file where a directory is expected, a
	// missing/non-directory root, or a permission problem set Valid to false.
	ValidateStructure(repoPath string) (*StructureValidation, error)
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
	ID           string    `json:"id"`
	RepositoryID uuid.UUID `json:"repository_id"`
	PrivatePath  string    `json:"private_path"`
	Filename     string    `json:"filename"`
	CreatedAt    time.Time `json:"created_at"`
}

type defaultDirectoryManager struct{}

// NewDirectoryManager creates a new directory manager instance.
func NewDirectoryManager() DirectoryManager {
	return &defaultDirectoryManager{}
}

// Repository temp, trash, and sidecar I/O is deliberately absent here; those
// mutations require RepositoryManager/RepositoryFS lifecycle gates.
var _ DirectoryManager = (*defaultDirectoryManager)(nil)

// CreateStructure creates the complete directory structure for a repository
func (dm *defaultDirectoryManager) CreateStructure(repoPath string) error {
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

	for _, directory := range repoDirs {
		if err := applyDirectoryMode(filepath.Join(cleanPath, directory.path), directory.mode); err != nil {
			return fmt.Errorf("failed to set permissions for %s: %w", directory.path, err)
		}
	}

	return nil
}

// ValidateStructure validates the directory structure of a repository
func (dm *defaultDirectoryManager) ValidateStructure(repoPath string) (*StructureValidation, error) {
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

// checkDirectoryPermissions checks if we have proper read/write permissions
func (dm *defaultDirectoryManager) checkDirectoryPermissions(path string) error {
	// Test read permission. The handle must be closed: on Windows an open handle
	// to a directory blocks its deletion, which surfaced as repositories and
	// test temp directories becoming undeletable ("The process cannot access the
	// file because it is being used by another process").
	dir, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("cannot read directory: %w", err)
	}
	if err := dir.Close(); err != nil {
		return fmt.Errorf("cannot close directory: %w", err)
	}

	return requireDirectoryWritable(path)
}
