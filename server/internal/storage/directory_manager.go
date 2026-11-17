package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"

	"server/internal/storage/layout"
)

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
	// CreateStructure creates the repository tree [layout.Init] declares. It
	// is safe to call on an existing repository: present directories keep
	// their contents and no file is truncated.
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

// CreateStructure creates the repository tree the layout declares.
func (dm *defaultDirectoryManager) CreateStructure(repoPath string) error {
	return layout.Init(repoPath)
}

// ValidateStructure validates the directory structure of a repository.
func (dm *defaultDirectoryManager) ValidateStructure(repoPath string) (*StructureValidation, error) {
	cleanPath, err := filepath.Abs(filepath.Clean(repoPath))
	if err != nil {
		return nil, fmt.Errorf("invalid repository path: %w", err)
	}
	validation := &StructureValidation{Valid: true, MissingDirectories: []string{}, InvalidPaths: []string{}, PermissionIssues: []string{}, Warnings: []string{}}
	if info, err := os.Stat(cleanPath); os.IsNotExist(err) {
		validation.Valid = false
		validation.InvalidPaths = append(validation.InvalidPaths, "Repository root does not exist")
		return validation, nil
	} else if err == nil && !info.IsDir() {
		validation.Valid = false
		validation.InvalidPaths = append(validation.InvalidPaths, "Repository root is not a directory")
		return validation, nil
	}
	report, err := layout.Validate(cleanPath)
	if err != nil {
		return nil, err
	}
	for _, missing := range report.Missing {
		validation.MissingDirectories = append(validation.MissingDirectories, missing)
		validation.Warnings = append(validation.Warnings, fmt.Sprintf("Missing directory: %s", missing))
	}
	for _, wrong := range report.WrongKind {
		validation.InvalidPaths = append(validation.InvalidPaths, fmt.Sprintf("Unexpected entry kind: %s", wrong))
	}
	for _, inaccessible := range report.Inaccessible {
		validation.PermissionIssues = append(validation.PermissionIssues, fmt.Sprintf("Cannot access %s", inaccessible))
	}
	// The root directories must exist and be readable and writable.
	for _, dir := range []string{layout.PrivateRoot, layout.Inbox} {
		dirPath := filepath.Join(cleanPath, dir)
		if err := dm.checkDirectoryPermissions(dirPath); err != nil {
			validation.PermissionIssues = append(validation.PermissionIssues, fmt.Sprintf("Permission issue with %s: %v", dir, err))
		}
	}
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
