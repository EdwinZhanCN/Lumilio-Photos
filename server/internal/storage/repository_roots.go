package storage

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/storage/repocfg"
	"server/internal/storage/rootcfg"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrRepositoryRootOffline      = errors.New("storage location is offline")
	ErrRepositoryRootInvalid      = errors.New("storage location is invalid")
	ErrRepositoryRootOverlap      = errors.New("storage location overlaps another registered location")
	ErrRepositoryRootNotRemovable = errors.New("default storage location cannot be removed")
	ErrRepositoryRootInUse        = errors.New("storage location still contains registered repositories")
)

// RepositoryRootConflictError reports a portable .lumilioroot identity that is
// already registered at another path. The host must not infer whether the
// directory moved or was copied.
type RepositoryRootConflictError struct {
	RootID         string
	RegisteredPath string
	RequestedPath  string
}

func (e *RepositoryRootConflictError) Error() string {
	return fmt.Sprintf("storage location %s is already registered at %s", e.RootID, e.RegisteredPath)
}

// EnsureDefaultRepositoryRoot initializes or reopens the configured default
// Storage Location. A pre-migration database row is associated after the marker
// is created, while an existing marker remains disk-authoritative for identity.
func (rm *DefaultRepositoryManager) EnsureDefaultRepositoryRoot(ctx context.Context, path string) (*repo.RepositoryRoot, error) {
	cleanPath, err := CanonicalizeRepositoryPath(path)
	if err != nil {
		return nil, fmt.Errorf("canonicalize default storage location: %w", err)
	}
	if err := os.MkdirAll(cleanPath, 0o755); err != nil {
		return nil, fmt.Errorf("create default storage location: %w", err)
	}
	if repocfg.IsRepositoryRoot(cleanPath) {
		return nil, fmt.Errorf("%w: a repository cannot also be a storage location", ErrRepositoryRootInvalid)
	}

	existingDefault, defaultErr := rm.queries.GetDefaultRepositoryRoot(ctx)
	if defaultErr != nil && !errors.Is(defaultErr, pgx.ErrNoRows) {
		return nil, fmt.Errorf("load default storage location: %w", defaultErr)
	}
	if defaultErr == nil && existingDefault.Path != cleanPath {
		return nil, fmt.Errorf("%w: default Storage Location is registered at %s, not %s", ErrRepositoryRootInvalid, existingDefault.Path, cleanPath)
	}

	config, createdMarker, err := loadOrCreateRootConfig(cleanPath, "Default storage", func() *rootcfg.RootConfig {
		if defaultErr == nil && existingDefault.RootID.Valid {
			return &rootcfg.RootConfig{
				Version:   rootcfg.CurrentVersion,
				ID:        uuid.UUID(existingDefault.RootID.Bytes).String(),
				Name:      existingDefault.Name,
				CreatedAt: existingDefault.CreatedAt.Time,
			}
		}
		return rootcfg.New("Default storage")
	})
	if err != nil {
		return nil, err
	}
	if defaultErr == nil && existingDefault.RootID.Valid && config.ID != uuid.UUID(existingDefault.RootID.Bytes).String() {
		return nil, fmt.Errorf("%w: configured default path contains a different .lumilioroot identity", ErrRepositoryRootInvalid)
	}

	registered, err := rm.registerRepositoryRoot(ctx, cleanPath, config, dbtypes.RepositoryRootKindDefault, false)
	if err != nil {
		if createdMarker {
			_ = os.Remove(filepath.Join(cleanPath, rootcfg.FileName))
		}
		return nil, err
	}
	if err := rm.associateRepositoriesUnderRoot(ctx, *registered); err != nil {
		return nil, err
	}
	return registered, nil
}

// AddRepositoryRoot registers a native-host-authorized directory as an
// external Storage Location. The directory must already exist; the server never
// turns a missing mount path into a new directory.
func (rm *DefaultRepositoryManager) AddRepositoryRoot(ctx context.Context, path, name string) (*repo.RepositoryRoot, error) {
	cleanPath, err := rm.validateRepositoryRootPath(path)
	if err != nil {
		return nil, err
	}

	if existing, err := rm.queries.GetRepositoryRootByPath(ctx, cleanPath); err == nil {
		config, loadErr := rootcfg.Load(cleanPath)
		if loadErr != nil {
			return nil, loadErr
		}
		if existing.RootID.Valid && config.ID != uuid.UUID(existing.RootID.Bytes).String() {
			return nil, fmt.Errorf("%w: database and .lumilioroot identities differ", ErrRepositoryRootInvalid)
		}
		if err := rm.associateRepositoriesUnderRoot(ctx, existing); err != nil {
			return nil, err
		}
		return &existing, nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("find storage location by path: %w", err)
	}

	if err := rm.rejectOverlappingRepositoryRoot(ctx, cleanPath); err != nil {
		return nil, err
	}
	rootName := strings.TrimSpace(name)
	if rootName == "" {
		rootName = filepath.Base(cleanPath)
	}
	config, createdMarker, err := loadOrCreateRootConfig(cleanPath, rootName, func() *rootcfg.RootConfig {
		return rootcfg.New(rootName)
	})
	if err != nil {
		return nil, err
	}
	registered, err := rm.registerRepositoryRoot(ctx, cleanPath, config, dbtypes.RepositoryRootKindExternal, false)
	if err != nil {
		if createdMarker {
			_ = os.Remove(filepath.Join(cleanPath, rootcfg.FileName))
		}
		return nil, err
	}
	if err := rm.associateRepositoriesUnderRoot(ctx, *registered); err != nil {
		return nil, err
	}
	return registered, nil
}

// RelocateRepositoryRoot reconnects an existing external Storage Location at
// a new native-host-authorized path. The marker at the requested path must
// carry the registered identity; this never rewrites or guesses identity.
func (rm *DefaultRepositoryManager) RelocateRepositoryRoot(ctx context.Context, id, path string) (*repo.RepositoryRoot, error) {
	rootID, err := uuid.Parse(strings.TrimSpace(id))
	if err != nil {
		return nil, fmt.Errorf("invalid storage location id: %w", err)
	}
	registered, err := rm.queries.GetRepositoryRoot(ctx, pgtype.UUID{Bytes: rootID, Valid: true})
	if err != nil {
		return nil, err
	}
	if registered.Kind != dbtypes.RepositoryRootKindExternal {
		return nil, ErrRepositoryRootNotRemovable
	}

	cleanPath, err := rm.validateRepositoryRootPath(path)
	if err != nil {
		return nil, err
	}
	config, err := rootcfg.Load(cleanPath)
	if err != nil {
		return nil, err
	}
	if config.ID != rootID.String() {
		return nil, fmt.Errorf("%w: selected directory has a different .lumilioroot identity", ErrRepositoryRootInvalid)
	}
	if err := rm.rejectOverlappingRepositoryRootExcept(ctx, cleanPath, rootID); err != nil {
		return nil, err
	}

	type repositoryMove struct {
		id   string
		path string
	}
	repositories, err := rm.queries.ListRepositories(ctx)
	if err != nil {
		return nil, fmt.Errorf("list repositories for storage location relocate: %w", err)
	}
	moves := make([]repositoryMove, 0)
	for _, repository := range repositories {
		if !repository.RootID.Valid || uuid.UUID(repository.RootID.Bytes) != rootID {
			continue
		}
		requestedRepositoryPath, moveErr := relocatedRepositoryPath(registered.Path, cleanPath, repository.Path)
		if moveErr != nil {
			return nil, moveErr
		}
		repositoryConfig, loadErr := repocfg.LoadConfigFromFile(requestedRepositoryPath)
		if loadErr != nil {
			return nil, fmt.Errorf("validate repository after Storage Location move: %w", loadErr)
		}
		if !repository.RepoID.Valid || repositoryConfig.ID != uuid.UUID(repository.RepoID.Bytes).String() {
			return nil, fmt.Errorf("%w: repository identity differs at %s", ErrRepositoryRootInvalid, requestedRepositoryPath)
		}
		if occupying, findErr := rm.queries.GetRepositoryByPath(ctx, requestedRepositoryPath); findErr == nil && occupying.RepoID != repository.RepoID {
			return nil, fmt.Errorf("%w: %s", ErrRepositoryExistsAtPath, requestedRepositoryPath)
		} else if findErr != nil && !errors.Is(findErr, pgx.ErrNoRows) {
			return nil, fmt.Errorf("check repository destination: %w", findErr)
		}
		moves = append(moves, repositoryMove{id: repositoryConfig.ID, path: requestedRepositoryPath})
	}

	// Move child repository rows first. If one update fails, the root still
	// points at its previous path and a retry can resume; successfully moved rows
	// are temporarily detached and re-associated below. Moving the root row first
	// would lose the old prefix needed to recover from a partial failure.
	for _, move := range moves {
		if _, err := rm.RelocateRepository(ctx, move.id, move.path); err != nil {
			return nil, fmt.Errorf("relocate repository with Storage Location: %w", err)
		}
	}
	root, err := rm.registerRepositoryRoot(ctx, cleanPath, config, dbtypes.RepositoryRootKindExternal, true)
	if err != nil {
		return nil, err
	}
	if err := rm.associateRepositoriesUnderRoot(ctx, *root); err != nil {
		return nil, err
	}
	return root, nil
}

func (rm *DefaultRepositoryManager) validateRepositoryRootPath(path string) (string, error) {
	cleanPath, err := CanonicalizeRepositoryPath(path)
	if err != nil {
		return "", fmt.Errorf("canonicalize storage location: %w", err)
	}
	info, err := os.Stat(cleanPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("%w: %s", ErrRepositoryRootOffline, cleanPath)
		}
		return "", fmt.Errorf("stat storage location: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%w: path is not a directory", ErrRepositoryRootInvalid)
	}
	if repocfg.IsRepositoryRoot(cleanPath) {
		return "", fmt.Errorf("%w: choose attach repository for a .lumiliorepo directory", ErrRepositoryRootInvalid)
	}
	if isInsidePhotosLibrary(cleanPath) {
		return "", fmt.Errorf("%w: a Photos library bundle cannot be a storage location", ErrRepositoryRootInvalid)
	}
	if nested, parent, err := rm.isNestedRepository(cleanPath); err != nil {
		return "", err
	} else if nested {
		return "", fmt.Errorf("%w: path is inside repository %s", ErrRepositoryRootInvalid, parent)
	}
	return cleanPath, nil
}

func loadOrCreateRootConfig(path, name string, create func() *rootcfg.RootConfig) (*rootcfg.RootConfig, bool, error) {
	if rootcfg.Exists(path) {
		config, err := rootcfg.Load(path)
		return config, false, err
	}
	config := create()
	if strings.TrimSpace(config.Name) == "" {
		config.Name = strings.TrimSpace(name)
	}
	if err := config.Save(path); err != nil {
		return nil, false, err
	}
	return config, true, nil
}

func (rm *DefaultRepositoryManager) registerRepositoryRoot(
	ctx context.Context,
	path string,
	config *rootcfg.RootConfig,
	kind dbtypes.RepositoryRootKind,
	allowMove bool,
) (*repo.RepositoryRoot, error) {
	rootID, err := uuid.Parse(config.ID)
	if err != nil {
		return nil, fmt.Errorf("parse storage location id: %w", err)
	}
	if registered, err := rm.queries.GetRepositoryRoot(ctx, pgtype.UUID{Bytes: rootID, Valid: true}); err == nil {
		if registered.Path != path && !allowMove {
			return nil, &RepositoryRootConflictError{
				RootID:         config.ID,
				RegisteredPath: registered.Path,
				RequestedPath:  path,
			}
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("find storage location by id: %w", err)
	}

	now := time.Now()
	createdAt := config.CreatedAt
	if createdAt.IsZero() {
		createdAt = now
	}
	registered, err := rm.queries.UpsertRepositoryRoot(ctx, repo.UpsertRepositoryRootParams{
		RootID:    pgtype.UUID{Bytes: rootID, Valid: true},
		Name:      config.Name,
		Path:      path,
		Kind:      kind,
		Status:    dbtypes.RepositoryRootStatusActive,
		CreatedAt: pgtype.Timestamptz{Time: createdAt, Valid: true},
		UpdatedAt: pgtype.Timestamptz{Time: now, Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("register storage location: %w", err)
	}
	return &registered, nil
}

// ListRepositoryRoots refreshes reachability before returning server facts to
// the Web UI, so a drive unplugged after boot is not advertised as writable.
func (rm *DefaultRepositoryManager) ListRepositoryRoots(ctx context.Context) ([]repo.RepositoryRoot, error) {
	if err := rm.ReconcileRepositoryRoots(ctx); err != nil {
		return nil, err
	}
	roots, err := rm.queries.ListRepositoryRoots(ctx)
	if err != nil {
		return nil, fmt.Errorf("list storage locations: %w", err)
	}
	return roots, nil
}

func (rm *DefaultRepositoryManager) ReconcileRepositoryRoots(ctx context.Context) error {
	roots, err := rm.queries.ListRepositoryRoots(ctx)
	if err != nil {
		return fmt.Errorf("list storage locations for reconcile: %w", err)
	}
	for _, root := range roots {
		status := dbtypes.RepositoryRootStatusActive
		name := root.Name
		if info, statErr := os.Stat(root.Path); statErr != nil || !info.IsDir() {
			status = dbtypes.RepositoryRootStatusOffline
		} else if config, loadErr := rootcfg.Load(root.Path); loadErr != nil {
			status = dbtypes.RepositoryRootStatusError
		} else if !root.RootID.Valid || config.ID != uuid.UUID(root.RootID.Bytes).String() {
			status = dbtypes.RepositoryRootStatusError
		} else {
			name = config.Name
		}
		_, updateErr := rm.queries.UpdateRepositoryRootFromDisk(ctx, repo.UpdateRepositoryRootFromDiskParams{
			RootID:    root.RootID,
			Name:      name,
			Status:    status,
			UpdatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
		})
		if updateErr != nil {
			return fmt.Errorf("update storage location status: %w", updateErr)
		}
	}
	return nil
}

func (rm *DefaultRepositoryManager) GetRepositoryRoot(ctx context.Context, id string) (*repo.RepositoryRoot, error) {
	rootID, err := uuid.Parse(strings.TrimSpace(id))
	if err != nil {
		return nil, fmt.Errorf("invalid storage location id: %w", err)
	}
	root, err := rm.queries.GetRepositoryRoot(ctx, pgtype.UUID{Bytes: rootID, Valid: true})
	if err != nil {
		return nil, err
	}
	return &root, nil
}

func (rm *DefaultRepositoryManager) DeleteRepositoryRoot(ctx context.Context, id string) error {
	rootID, err := uuid.Parse(strings.TrimSpace(id))
	if err != nil {
		return fmt.Errorf("invalid storage location id: %w", err)
	}
	rootUUID := pgtype.UUID{Bytes: rootID, Valid: true}
	root, err := rm.queries.GetRepositoryRoot(ctx, rootUUID)
	if err != nil {
		return err
	}
	if root.Kind != dbtypes.RepositoryRootKindExternal {
		return ErrRepositoryRootNotRemovable
	}
	deleted, err := rm.queries.DeleteExternalRepositoryRoot(ctx, rootUUID)
	if err != nil {
		return fmt.Errorf("remove storage location: %w", err)
	}
	if deleted == 0 {
		return ErrRepositoryRootInUse
	}
	return nil
}

func (rm *DefaultRepositoryManager) resolveRepositoryRootForCreate(ctx context.Context, id string, role dbtypes.RepoRole) (*repo.RepositoryRoot, error) {
	var root repo.RepositoryRoot
	var err error
	if strings.TrimSpace(id) == "" {
		root, err = rm.queries.GetDefaultRepositoryRoot(ctx)
	} else {
		rootID, parseErr := uuid.Parse(strings.TrimSpace(id))
		if parseErr != nil {
			return nil, fmt.Errorf("invalid storage location id: %w", parseErr)
		}
		root, err = rm.queries.GetRepositoryRoot(ctx, pgtype.UUID{Bytes: rootID, Valid: true})
	}
	if err != nil {
		return nil, fmt.Errorf("load storage location: %w", err)
	}
	if normalizeRepoRole(role) == dbtypes.RepoRolePrimary && root.Kind != dbtypes.RepositoryRootKindDefault {
		return nil, fmt.Errorf("%w: primary repository must use the default storage location", ErrPathNotAllowed)
	}
	if root.Status != dbtypes.RepositoryRootStatusActive {
		return nil, fmt.Errorf("%w: %s", ErrRepositoryRootOffline, root.Path)
	}
	config, err := rootcfg.Load(root.Path)
	if err != nil || !root.RootID.Valid || config.ID != uuid.UUID(root.RootID.Bytes).String() {
		return nil, fmt.Errorf("%w: %s", ErrRepositoryRootInvalid, root.Path)
	}
	return &root, nil
}

func (rm *DefaultRepositoryManager) associateRepositoriesUnderRoot(ctx context.Context, root repo.RepositoryRoot) error {
	repositories, err := rm.queries.ListRepositories(ctx)
	if err != nil {
		return fmt.Errorf("list repositories for storage location association: %w", err)
	}
	for _, repository := range repositories {
		if repository.RootID.Valid || !pathIsStrictlyInside(root.Path, repository.Path) {
			continue
		}
		if _, err := rm.queries.SetRepositoryRoot(ctx, repo.SetRepositoryRootParams{
			RepoID:    repository.RepoID,
			RootID:    root.RootID,
			UpdatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
		}); err != nil {
			return fmt.Errorf("associate repository %s with storage location: %w", repository.Path, err)
		}
	}
	return nil
}

func (rm *DefaultRepositoryManager) repositoryRootIDForPath(ctx context.Context, path string) (pgtype.UUID, error) {
	roots, err := rm.queries.ListRepositoryRoots(ctx)
	if err != nil {
		return pgtype.UUID{}, fmt.Errorf("list storage locations for repository association: %w", err)
	}
	for _, root := range roots {
		if root.Status == dbtypes.RepositoryRootStatusActive && root.RootID.Valid && pathIsStrictlyInside(root.Path, path) {
			return root.RootID, nil
		}
	}
	return pgtype.UUID{}, nil
}

func (rm *DefaultRepositoryManager) rejectOverlappingRepositoryRoot(ctx context.Context, requested string) error {
	return rm.rejectOverlappingRepositoryRootExcept(ctx, requested, uuid.Nil)
}

func (rm *DefaultRepositoryManager) rejectOverlappingRepositoryRootExcept(ctx context.Context, requested string, except uuid.UUID) error {
	roots, err := rm.queries.ListRepositoryRoots(ctx)
	if err != nil {
		return fmt.Errorf("list storage locations for overlap check: %w", err)
	}
	for _, root := range roots {
		if except != uuid.Nil && root.RootID.Valid && root.RootID.Bytes == except {
			continue
		}
		if root.Path == requested || pathIsStrictlyInside(root.Path, requested) || pathIsStrictlyInside(requested, root.Path) {
			return fmt.Errorf("%w: %s and %s", ErrRepositoryRootOverlap, requested, root.Path)
		}
	}
	return nil
}

func pathIsStrictlyInside(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == "." || filepath.IsAbs(rel) {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func relocatedRepositoryPath(oldRoot, newRoot, repositoryPath string) (string, error) {
	relative, err := filepath.Rel(oldRoot, repositoryPath)
	if err != nil || relative == "." || filepath.IsAbs(relative) || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%w: repository %s is outside its registered Storage Location", ErrRepositoryRootInvalid, repositoryPath)
	}
	return filepath.Join(newRoot, relative), nil
}

// RepositoryRootWarnings returns non-fatal placement risks for the Desktop
// Control Panel to surface immediately after a native directory grant.
func RepositoryRootWarnings(path string) []string {
	provider := cloudSyncProvider(path)
	if provider == "" {
		return nil
	}
	return []string{fmt.Sprintf(
		"%s is inside %s. Sync clients may evict originals or duplicate files.", path, provider,
	)}
}
