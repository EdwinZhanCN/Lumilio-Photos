package storage

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/storage/repocfg"

	"github.com/google/uuid"
)

// Provisioning errors. Callers (HTTP handlers) map these to status codes.
var (
	ErrPrimaryRepositoryExists      = errors.New("primary repository already exists")
	ErrPrimaryRepositoryRequired    = errors.New("primary repository must be created first")
	ErrRepositoryExistsAtPath       = errors.New("repository already exists at path")
	ErrInvalidRepositoryName        = errors.New("invalid repository name")
	ErrRepositoryNameConflict       = errors.New("repository name conflicts with an existing directory")
	ErrRepositoryTargetNotEmpty     = errors.New("repository target directory is not empty")
	ErrRepositoryStorageNotWritable = errors.New("repository storage is not writable")
)

const (
	maxRepositoryNameRunes = 80
	maxRepositoryNameBytes = 240
)

// CreateRepositorySpec describes a repository to create. StorageStrategy and
// DuplicateHandling are optional; empty values fall back to the storage-owned
// repository defaults.
type CreateRepositorySpec struct {
	Name              string
	Role              dbtypes.RepoRole
	Root              string
	RootID            string
	OwnerID           *int32
	StorageStrategy   string
	DuplicateHandling string
}

// CreateRepositoryResult carries the created repository plus any non-fatal
// warnings the path policy raised about its location.
type CreateRepositoryResult struct {
	Repository *repo.Repository
	Warnings   []string
}

// CreateRepository is the single entry point for creating (or registering) a
// repository: it enforces the primary-first / single-primary policy, resolves a
// path inside Root, applies repository defaults, and either registers an
// existing on-disk repository or initializes a new one.
func (rm *DefaultRepositoryManager) CreateRepository(ctx context.Context, spec CreateRepositorySpec) (*CreateRepositoryResult, error) {
	if err := ValidateRepositoryName(spec.Name); err != nil {
		return nil, err
	}

	role := normalizeRepoRole(spec.Role)

	primaryExists, err := rm.primaryRepositoryExists(ctx)
	if err != nil {
		return nil, err
	}
	if role == dbtypes.RepoRolePrimary && primaryExists {
		return nil, ErrPrimaryRepositoryExists
	}
	if role != dbtypes.RepoRolePrimary && !primaryExists {
		return nil, ErrPrimaryRepositoryRequired
	}

	var rootIDs []uuid.UUID
	if strings.TrimSpace(spec.RootID) != "" && strings.TrimSpace(spec.Root) != "" {
		return nil, fmt.Errorf("%w: root_id and root path cannot both be supplied", ErrPathNotAllowed)
	}
	if strings.TrimSpace(spec.Root) == "" {
		selectedRoot, err := rm.resolveRepositoryRootForCreate(ctx, spec.RootID, role)
		if err != nil {
			return nil, err
		}
		spec.Root = selectedRoot.Path
		rootIDs = append(rootIDs, selectedRoot.RootID)
	}

	repoPath, err := resolveRepositoryCreatePath(spec.Root, spec.Name, role)
	if err != nil {
		return nil, err
	}
	warnings := RepositoryRootWarnings(spec.Root)
	if existing, err := rm.GetRepositoryByPath(repoPath); err == nil && existing != nil {
		return nil, fmt.Errorf("%w: %s", ErrRepositoryExistsAtPath, repoPath)
	}

	if repocfg.IsRepositoryRoot(repoPath) {
		dbRepo, err := rm.AddRepository(repoPath, spec.OwnerID, role, rootIDs...)
		if err != nil {
			return nil, err
		}
		return &CreateRepositoryResult{Repository: dbRepo, Warnings: warnings}, nil
	}

	defaults, err := rm.GetRepositoryDefaults(ctx)
	if err != nil {
		return nil, err
	}
	cfg := repocfg.NewRepositoryConfig(
		spec.Name,
		repocfg.WithStorageStrategy(firstNonEmpty(spec.StorageStrategy, defaults.Strategy, "date")),
		repocfg.WithLocalSettings(firstNonEmpty(spec.DuplicateHandling, defaults.DuplicateHandling, "rename")),
	)
	dbRepo, err := rm.InitializeRepository(repoPath, *cfg, spec.OwnerID, role, rootIDs...)
	if err != nil {
		return nil, err
	}
	return &CreateRepositoryResult{Repository: dbRepo, Warnings: warnings}, nil
}

// EnsurePrimaryRepository idempotently ensures a primary repository exists at
// <root>/primary, returning the existing one if already present.
func (rm *DefaultRepositoryManager) EnsurePrimaryRepository(ctx context.Context, root string, ownerID *int32) (*repo.Repository, error) {
	if existing, err := rm.queries.GetPrimaryRepository(ctx); err == nil {
		return &existing, nil
	}
	result, err := rm.CreateRepository(ctx, CreateRepositorySpec{
		Name:    "Primary Storage",
		Role:    dbtypes.RepoRolePrimary,
		Root:    root,
		OwnerID: ownerID,
	})
	if err != nil {
		return nil, err
	}
	return result.Repository, nil
}

func (rm *DefaultRepositoryManager) primaryRepositoryExists(ctx context.Context) (bool, error) {
	count, err := rm.queries.CountPrimaryRepositories(ctx)
	if err != nil {
		return false, fmt.Errorf("count primary repositories: %w", err)
	}
	return count > 0, nil
}

// resolveRepositoryCreatePath resolves the on-disk path for a new repository
// under root. Primary repositories always live at <root>/primary; a regular
// repository uses its validated name verbatim so a Server bind-mount target and
// the name submitted in Web always refer to the same directory. The result is
// guaranteed to stay inside root.
func resolveRepositoryCreatePath(root, name string, role dbtypes.RepoRole) (string, error) {
	trimmed := strings.TrimSpace(root)
	if trimmed == "" {
		return "", errors.New("storage root is not configured")
	}
	cleanRoot, err := CanonicalizeRepositoryPath(trimmed)
	if err != nil {
		return "", fmt.Errorf("invalid storage root: %w", err)
	}

	if err := ValidateRepositoryName(name); err != nil {
		return "", err
	}

	folderName := name
	if role == dbtypes.RepoRolePrimary {
		folderName = "primary"
	}

	repoPath, err := filepath.Abs(filepath.Join(cleanRoot, folderName))
	if err != nil {
		return "", fmt.Errorf("invalid repository path: %w", err)
	}
	rel, err := filepath.Rel(cleanRoot, repoPath)
	if err != nil {
		return "", fmt.Errorf("invalid repository path: %w", err)
	}
	if rel == "." || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return "", errors.New("repository path must be inside storage root")
	}
	if err := rejectCaseInsensitiveRepositoryNameConflict(cleanRoot, folderName); err != nil {
		return "", err
	}
	return repoPath, nil
}

// ValidateRepositoryName applies the portable directory-name contract used by
// every deployment. It is intentionally a whitelist: names are never
// lowercased, trimmed, slugified, or repaired behind the user's back.
func ValidateRepositoryName(name string) error {
	if !utf8.ValidString(name) {
		return fmt.Errorf("%w: name must be valid UTF-8", ErrInvalidRepositoryName)
	}
	if name == "" {
		return fmt.Errorf("%w: name is required", ErrInvalidRepositoryName)
	}
	if len(name) > maxRepositoryNameBytes {
		return fmt.Errorf("%w: name must not exceed %d UTF-8 bytes", ErrInvalidRepositoryName, maxRepositoryNameBytes)
	}
	runeCount := utf8.RuneCountInString(name)
	if runeCount > maxRepositoryNameRunes {
		return fmt.Errorf("%w: name must not exceed %d characters", ErrInvalidRepositoryName, maxRepositoryNameRunes)
	}
	if strings.HasPrefix(name, " ") || strings.HasSuffix(name, " ") {
		return fmt.Errorf("%w: name cannot start or end with a space", ErrInvalidRepositoryName)
	}
	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == ' ' || r == '-' || r == '_' {
			continue
		}
		return fmt.Errorf("%w: character %q is not allowed", ErrInvalidRepositoryName, r)
	}
	return nil
}

func rejectCaseInsensitiveRepositoryNameConflict(root, requestedName string) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		return fmt.Errorf("list storage root: %w", err)
	}
	for _, entry := range entries {
		existingName := entry.Name()
		if existingName != requestedName && strings.EqualFold(existingName, requestedName) {
			return fmt.Errorf("%w: %q already exists as %q", ErrRepositoryNameConflict, requestedName, existingName)
		}
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
