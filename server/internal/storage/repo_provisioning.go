package storage

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"unicode"

	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/storage/repocfg"
)

// Provisioning errors. Callers (HTTP handlers) map these to status codes.
var (
	ErrPrimaryRepositoryExists   = errors.New("primary repository already exists")
	ErrPrimaryRepositoryRequired = errors.New("primary repository must be created first")
	ErrRepositoryExistsAtPath    = errors.New("repository already exists at path")
)

// CreateRepositorySpec describes a repository to create. StorageStrategy and
// DuplicateHandling are optional; empty values fall back to the storage-owned
// repository defaults.
type CreateRepositorySpec struct {
	Name string
	Role dbtypes.RepoRole
	Root string
	// Path is the caller-requested absolute location. Only FreePolicy honours
	// it; RootedPolicy rejects a non-empty Path rather than silently placing the
	// repository somewhere else.
	Path              string
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

	repoPath, warnings, err := pathPolicyForRole(rm.pathPolicy(), role, spec).ResolveCreatePath(spec)
	if err != nil {
		return nil, err
	}
	if existing, err := rm.GetRepositoryByPath(repoPath); err == nil && existing != nil {
		return nil, fmt.Errorf("%w: %s", ErrRepositoryExistsAtPath, repoPath)
	}

	if repocfg.IsRepositoryRoot(repoPath) {
		dbRepo, err := rm.AddRepository(repoPath, spec.OwnerID, role)
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
	dbRepo, err := rm.InitializeRepository(repoPath, *cfg, spec.OwnerID, role)
	if err != nil {
		return nil, err
	}
	return &CreateRepositoryResult{Repository: dbRepo, Warnings: warnings}, nil
}

// pathPolicy returns the configured policy, defaulting to RootedPolicy so a
// deployment that has not opted into free placement cannot accidentally accept
// caller-supplied paths.
func (rm *DefaultRepositoryManager) pathPolicy() PathPolicy {
	if rm.policy == nil {
		return RootedPolicy{}
	}
	return rm.policy
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
	count, err := rm.queries.CountActivePrimaryRepositories(ctx)
	if err != nil {
		return false, fmt.Errorf("count primary repositories: %w", err)
	}
	return count > 0, nil
}

// resolveRepositoryCreatePath resolves the on-disk path for a new repository
// under root. Primary repositories always live at <root>/primary; others use a
// slugified folder name. The result is guaranteed to stay inside root.
func resolveRepositoryCreatePath(root, name string, role dbtypes.RepoRole) (string, error) {
	trimmed := strings.TrimSpace(root)
	if trimmed == "" {
		return "", errors.New("storage root is not configured")
	}
	cleanRoot, err := CanonicalizeRepositoryPath(trimmed)
	if err != nil {
		return "", fmt.Errorf("invalid storage root: %w", err)
	}

	folderName := repositoryFolderNameFromName(name)
	if role == dbtypes.RepoRolePrimary {
		folderName = "primary"
	}
	if folderName == "" {
		return "", errors.New("repository name must contain letters or numbers")
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
	return repoPath, nil
}

func repositoryFolderNameFromName(name string) string {
	var builder strings.Builder
	lastDash := false
	for _, r := range strings.TrimSpace(name) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			builder.WriteRune(unicode.ToLower(r))
			lastDash = false
		case r == '-' || r == '_':
			if builder.Len() > 0 {
				builder.WriteRune(r)
				lastDash = r == '-'
			}
		case unicode.IsSpace(r):
			if builder.Len() > 0 && !lastDash {
				builder.WriteRune('-')
				lastDash = true
			}
		default:
			if builder.Len() > 0 && !lastDash {
				builder.WriteRune('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(strings.TrimSpace(builder.String()), "-_")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
