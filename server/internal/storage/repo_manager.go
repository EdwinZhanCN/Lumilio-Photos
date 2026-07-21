package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/logging"
	"server/internal/storage/repocfg"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"
)

// ValidationResult represents the result of repository validation
type ValidationResult struct {
	Valid    bool     `json:"valid"`
	Errors   []string `json:"errors,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

// RepositoryManager is the consumer-facing contract for the repository
// lifecycle. Implementations keep the database record and the on-disk
// repository (its directory structure and .lumiliorepo config) in sync: a
// successful mutating call has applied to both, and a failed one rolls back the
// filesystem side. Calls are not safe for concurrent use on the same repository;
// the caller serializes mutations.
type RepositoryManager interface {
	// InitializeRepository creates a brand-new repository: it builds the
	// directory structure, writes the .lumiliorepo config, and inserts the
	// database record. It fails if a repository already exists at path or path
	// is nested inside one, and removes any partially created files on failure.
	InitializeRepository(path string, config repocfg.RepositoryConfig, defaultOwnerID *int32, role dbtypes.RepoRole) (*repo.Repository, error)

	// AddRepository registers an already-initialized on-disk repository (one
	// that has a valid .lumiliorepo). It fails if the path is not a valid
	// repository or is already registered. If the repository's ID is registered
	// at a different path it returns a *RepositoryConflictError, which the caller
	// resolves with RelocateRepository or RegisterRepositoryCopy.
	AddRepository(path string, defaultOwnerID *int32, role dbtypes.RepoRole) (*repo.Repository, error)

	// RelocateRepository points an existing repository at a new on-disk
	// location. Assets are untouched because assets.storage_path is
	// repository-relative.
	RelocateRepository(ctx context.Context, id string, newPath string) (*repo.Repository, error)

	// RegisterRepositoryCopy registers a duplicated repository directory as an
	// independent repository by minting a fresh UUID into its .lumiliorepo.
	RegisterRepositoryCopy(ctx context.Context, path string, defaultOwnerID *int32, role dbtypes.RepoRole) (*repo.Repository, error)

	// ReconcileAll re-checks every repository's recorded path against the
	// .lumiliorepo actually on disk, updating reachability status and refreshing
	// the cached config from disk.
	ReconcileAll(ctx context.Context) error

	// GetRepository returns the repository with the given UUID, or an error if
	// the id is malformed or no such repository exists.
	GetRepository(id string) (*repo.Repository, error)

	// GetRepositoryByPath returns the repository registered at the given path
	// (matched after cleaning to an absolute path), or an error if none is.
	GetRepositoryByPath(path string) (*repo.Repository, error)

	// ListRepositories returns all registered repositories.
	ListRepositories() ([]*repo.Repository, error)

	// UpdateRepository validates and persists config to both the database record
	// and the on-disk .lumiliorepo file.
	UpdateRepository(id string, config repocfg.RepositoryConfig, defaultOwnerID *int32) (*repo.Repository, error)

	// RemoveRepository deletes only the database record; the on-disk repository
	// and its media are left untouched so the data can be re-registered later.
	RemoveRepository(id string) error

	// GetRepositoryPath returns the absolute on-disk path of a repository.
	// face/people depend on this via their own narrow interfaces.
	GetRepositoryPath(repoID string) (string, error)

	// GetRepositoryDefaults / UpdateRepositoryDefaults are the storage-owned,
	// runtime-mutable defaults applied to newly created repositories.
	GetRepositoryDefaults(ctx context.Context) (RepoDefaults, error)
	UpdateRepositoryDefaults(ctx context.Context, defaults RepoDefaults) (RepoDefaults, error)

	// Provisioning: CreateRepository enforces the primary-first / single-primary
	// policy and path resolution; EnsurePrimaryRepository is its idempotent
	// bootstrap helper for the mandatory primary repository.
	CreateRepository(ctx context.Context, spec CreateRepositorySpec) (*CreateRepositoryResult, error)
	EnsurePrimaryRepository(ctx context.Context, root string, ownerID *int32) (*repo.Repository, error)

	// GetStagingManager and GetDirectoryManager expose the sub-managers.
	// Transitional: consumers should eventually receive these by direct
	// injection instead of reaching through the repository manager.
	GetStagingManager() StagingManager
	GetDirectoryManager() DirectoryManager
}

// DefaultRepositoryManager implements the RepositoryManager interface
type DefaultRepositoryManager struct {
	queries        *repo.Queries
	dirManager     DirectoryManager
	stagingManager StagingManager
	logger         *zap.Logger
	auditProvider  logging.RepositoryAuditProvider
	policy         PathPolicy
}

// WithPathPolicy selects where new repositories may live. The zero value is
// RootedPolicy; desktop builds pass FreePolicy.
func WithPathPolicy(policy PathPolicy) func(*DefaultRepositoryManager) {
	return func(rm *DefaultRepositoryManager) {
		rm.policy = policy
	}
}

// NewRepositoryManager creates a new repository manager instance
func NewRepositoryManager(
	queries *repo.Queries,
	logger *zap.Logger,
	auditProvider logging.RepositoryAuditProvider,
	options ...func(*DefaultRepositoryManager),
) (*DefaultRepositoryManager, error) {
	if logger == nil {
		logger = zap.NewNop()
	}
	if auditProvider == nil {
		auditProvider = logging.NewRepositoryAuditProvider(logger, false)
	}

	rm := &DefaultRepositoryManager{
		queries:        queries,
		dirManager:     NewDirectoryManager(),
		stagingManager: NewStagingManager(),
		logger:         logger.With(zap.String("component", "repository")),
		auditProvider:  auditProvider,
	}
	for _, option := range options {
		option(rm)
	}

	return rm, nil
}

// Ensure the concrete type satisfies the consumer interface.
var _ RepositoryManager = (*DefaultRepositoryManager)(nil)

// AddRepository registers an existing repository with the system
func (rm *DefaultRepositoryManager) AddRepository(path string, defaultOwnerID *int32, role dbtypes.RepoRole) (*repo.Repository, error) {
	// Clean and validate path
	cleanPath, err := CanonicalizeRepositoryPath(path)
	if err != nil {
		rm.logger.Warn("repository add failed: invalid path", zap.String("operation", "repository.add"), zap.String("path", path), zap.Error(err))
		return nil, fmt.Errorf("invalid path: %w", err)
	}

	// Validate that this is a valid repository
	result, err := rm.validateRepository(cleanPath)
	if err != nil {
		rm.repoAudit(cleanPath).Error("repository.add", err, zap.String("repository_path", cleanPath))
		return nil, fmt.Errorf("failed to validate repository: %w", err)
	}
	if !result.Valid {
		validationErr := fmt.Errorf("invalid repository")
		rm.repoAudit(cleanPath).Error("repository.add", validationErr, zap.Strings("errors", result.Errors))
		return nil, fmt.Errorf("invalid repository at %s: %v", cleanPath, result.Errors)
	}

	// Check if repository is already registered
	_, err = rm.GetRepositoryByPath(cleanPath)
	if err == nil {
		return nil, fmt.Errorf("repository at %s is already registered", cleanPath)
	}

	// Load configuration
	config, err := repocfg.LoadConfigFromFile(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load repository configuration: %w", err)
	}

	// The ID on disk is already registered somewhere else. Only the user can say
	// whether this is the same library that moved or an independent copy, so
	// surface both paths and let the caller offer the choice.
	if registered, err := rm.GetRepository(config.ID); err == nil {
		return nil, &RepositoryConflictError{
			RepositoryID:   config.ID,
			RegisteredPath: registered.Path,
			RequestedPath:  cleanPath,
		}
	}

	repoUUID, err := uuid.Parse(config.ID)
	if err != nil {
		return nil, fmt.Errorf("invalid repository ID: %w", err)
	}

	now := time.Now()
	dbRepo, err := rm.queries.CreateRepository(context.Background(), repo.CreateRepositoryParams{
		RepoID:         pgtype.UUID{Bytes: repoUUID, Valid: true},
		Name:           config.Name,
		Path:           cleanPath,
		Config:         *config,
		Role:           normalizeRepoRole(role),
		Status:         dbtypes.RepoStatusActive,
		DefaultOwnerID: defaultOwnerID,
		CreatedAt:      pgtype.Timestamptz{Time: config.CreatedAt, Valid: true},
		UpdatedAt:      pgtype.Timestamptz{Time: now, Valid: true},
	})
	if err != nil {
		rm.repoAudit(cleanPath).Error("repository.add", err, zap.String("repository_id", config.ID))
		return nil, fmt.Errorf("failed to create database record: %w", err)
	}

	rm.repoAudit(cleanPath).Operation("repository.add",
		zap.String("repository_id", config.ID),
		zap.String("repository_name", config.Name),
	)
	rm.logger.Info("repository registered",
		zap.String("operation", "repository.add"),
		zap.String("repository_id", config.ID),
		zap.String("repository_path", cleanPath),
	)

	return &dbRepo, nil
}

// validateRepository validates a repository at the given path
func (rm *DefaultRepositoryManager) validateRepository(path string) (*ValidationResult, error) {
	result := &ValidationResult{
		Valid:    true,
		Errors:   []string{},
		Warnings: []string{},
	}

	cleanPath, err := CanonicalizeRepositoryPath(path)
	if err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Sprintf("Invalid path: %v", err))
		rm.logger.Warn("repository validation failed: invalid path", zap.String("operation", "repository.validate"), zap.String("path", path), zap.Error(err))
		return result, nil
	}

	// Check if directory exists
	info, err := os.Stat(cleanPath)
	if os.IsNotExist(err) {
		result.Valid = false
		result.Errors = append(result.Errors, "Repository directory does not exist")
		return result, nil
	}

	if !info.IsDir() {
		result.Valid = false
		result.Errors = append(result.Errors, "Path is not a directory")
		return result, nil
	}

	// Check for .lumiliorepo file
	configPath := filepath.Join(cleanPath, ".lumiliorepo")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		result.Valid = false
		result.Errors = append(result.Errors, "Missing .lumiliorepo configuration file")
		return result, nil
	}

	// Validate configuration file
	config, err := repocfg.LoadConfigFromFile(cleanPath)
	if err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Sprintf("Invalid configuration: %v", err))
		return result, nil
	}

	// Use directory manager for structure validation
	structureValidation, err := rm.dirManager.ValidateStructure(cleanPath)
	if err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Sprintf("Directory structure validation failed: %v", err))
	} else {
		if !structureValidation.Valid {
			result.Valid = false
		}
		result.Errors = append(result.Errors, structureValidation.InvalidPaths...)
		result.Warnings = append(result.Warnings, structureValidation.Warnings...)
	}

	// Check for nested repositories
	isNested, parentRepo, err := rm.isNestedRepository(cleanPath)
	if err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Could not check for nested repositories: %v", err))
	} else if isNested {
		result.Errors = append(result.Errors, fmt.Sprintf("Repository is nested inside another repository at: %s", parentRepo))
		result.Valid = false
	}

	// Validate configuration values
	if config.Version == "" {
		result.Errors = append(result.Errors, "Configuration missing version")
		result.Valid = false
	}

	if config.ID == "" {
		result.Errors = append(result.Errors, "Configuration missing repository ID")
		result.Valid = false
	} else {
		// Validate UUID format
		if _, err := uuid.Parse(config.ID); err != nil {
			result.Errors = append(result.Errors, "Repository ID is not a valid UUID")
			result.Valid = false
		}
	}

	if config.Name == "" {
		result.Errors = append(result.Errors, "Configuration missing repository name")
		result.Valid = false
	}

	// Check permissions
	if err := rm.checkDirectoryPermissions(cleanPath); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Permission issues: %v", err))
	}

	fields := []zap.Field{
		zap.String("repository_path", cleanPath),
		zap.Bool("valid", result.Valid),
		zap.Strings("warnings", result.Warnings),
	}
	if len(result.Errors) > 0 {
		rm.repoAudit(cleanPath).Error("repository.validate", fmt.Errorf("repository validation failed"), append(fields, zap.Strings("errors", result.Errors))...)
	} else {
		rm.repoAudit(cleanPath).Operation("repository.validate", fields...)
	}

	return result, nil
}

// isNestedRepository checks if a repository path is nested inside another repository
func (rm *DefaultRepositoryManager) isNestedRepository(path string) (bool, string, error) {
	cleanPath, err := CanonicalizeRepositoryPath(path)
	if err != nil {
		return false, "", err
	}

	// Start from parent directory and walk up
	currentPath := filepath.Dir(cleanPath)
	for currentPath != "/" && currentPath != "." && len(currentPath) > 1 {
		// Check if current directory has a .lumiliorepo file
		configPath := filepath.Join(currentPath, ".lumiliorepo")
		if _, err := os.Stat(configPath); err == nil {
			return true, currentPath, nil
		}

		// Move up one directory
		parentPath := filepath.Dir(currentPath)
		if parentPath == currentPath {
			break // Reached root
		}
		currentPath = parentPath
	}

	return false, "", nil
}

// checkDirectoryPermissions checks if we have proper read/write permissions
func (rm *DefaultRepositoryManager) checkDirectoryPermissions(path string) error {
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

// InitializeRepository creates a new repository with full directory structure
func (rm *DefaultRepositoryManager) InitializeRepository(path string, config repocfg.RepositoryConfig, defaultOwnerID *int32, role dbtypes.RepoRole) (*repo.Repository, error) {
	// Clean and validate path
	cleanPath, err := CanonicalizeRepositoryPath(path)
	if err != nil {
		rm.logger.Warn("repository init failed: invalid path", zap.String("operation", "repository.initialize"), zap.String("path", path), zap.Error(err))
		return nil, fmt.Errorf("invalid path: %w", err)
	}

	// Check if repository already exists
	if repocfg.IsRepositoryRoot(cleanPath) {
		return nil, fmt.Errorf("repository already exists at %s", cleanPath)
	}

	// Check for nested repositories
	isNested, parentRepo, err := rm.isNestedRepository(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("failed to check for nested repositories: %w", err)
	}
	if isNested {
		return nil, fmt.Errorf("cannot create repository inside existing repository at %s", parentRepo)
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	// Create directory structure using directory manager
	if err := rm.dirManager.CreateStructure(cleanPath); err != nil {
		rm.repoAudit(cleanPath).Error("repository.initialize", err, zap.String("repository_name", config.Name))
		return nil, fmt.Errorf("failed to create repository structure: %w", err)
	}

	// Save configuration file
	if err := config.SaveConfigToFile(cleanPath); err != nil {
		// Clean up on failure
		os.RemoveAll(cleanPath)
		rm.repoAudit(cleanPath).Error("repository.initialize", err, zap.String("repository_name", config.Name))
		return nil, fmt.Errorf("failed to save configuration: %w", err)
	}

	repoUUID, err := uuid.Parse(config.ID)
	if err != nil {
		// Clean up on failure
		os.RemoveAll(cleanPath)
		rm.repoAudit(cleanPath).Error("repository.initialize", err, zap.String("repository_name", config.Name))
		return nil, fmt.Errorf("invalid repository ID: %w", err)
	}

	now := time.Now()
	dbRepo, err := rm.queries.CreateRepository(context.Background(), repo.CreateRepositoryParams{
		RepoID:         pgtype.UUID{Bytes: repoUUID, Valid: true},
		Name:           config.Name,
		Path:           cleanPath,
		Config:         config,
		Role:           normalizeRepoRole(role),
		Status:         dbtypes.RepoStatusActive,
		DefaultOwnerID: defaultOwnerID,
		CreatedAt:      pgtype.Timestamptz{Time: config.CreatedAt, Valid: true},
		UpdatedAt:      pgtype.Timestamptz{Time: now, Valid: true},
	})
	if err != nil {
		// Clean up on failure
		os.RemoveAll(cleanPath)
		rm.repoAudit(cleanPath).Error("repository.initialize", err, zap.String("repository_id", config.ID), zap.String("repository_name", config.Name))
		return nil, fmt.Errorf("failed to create database record: %w", err)
	}

	rm.repoAudit(cleanPath).Operation("repository.initialize",
		zap.String("repository_id", config.ID),
		zap.String("repository_name", config.Name),
	)
	rm.logger.Info("repository initialized",
		zap.String("operation", "repository.initialize"),
		zap.String("repository_id", config.ID),
		zap.String("repository_path", cleanPath),
	)

	return &dbRepo, nil
}

func (rm *DefaultRepositoryManager) GetRepository(id string) (*repo.Repository, error) {
	repoUUID, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("invalid repository ID: %w", err)
	}

	dbRepo, err := rm.queries.GetRepository(context.Background(), pgtype.UUID{Bytes: repoUUID, Valid: true})
	if err != nil {
		return nil, fmt.Errorf("repository not found: %w", err)
	}

	return &dbRepo, nil
}

func (rm *DefaultRepositoryManager) GetRepositoryByPath(path string) (*repo.Repository, error) {
	cleanPath, err := CanonicalizeRepositoryPath(path)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}

	dbRepo, err := rm.queries.GetRepositoryByPath(context.Background(), cleanPath)
	if err != nil {
		return nil, fmt.Errorf("repository not found at path %s: %w", cleanPath, err)
	}

	return &dbRepo, nil
}

func (rm *DefaultRepositoryManager) ListRepositories() ([]*repo.Repository, error) {
	repos, err := rm.queries.ListRepositories(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to list repositories: %w", err)
	}

	result := make([]*repo.Repository, len(repos))
	for i := range repos {
		result[i] = &repos[i]
	}

	return result, nil
}

func (rm *DefaultRepositoryManager) RemoveRepository(id string) error {
	repoUUID, err := uuid.Parse(id)
	if err != nil {
		rm.logger.Warn("repository remove failed: invalid id", zap.String("operation", "repository.remove"), zap.String("repository_id", id), zap.Error(err))
		return fmt.Errorf("invalid repository ID: %w", err)
	}

	var repoPath string
	if rm.queries != nil {
		existing, getErr := rm.queries.GetRepository(context.Background(), pgtype.UUID{Bytes: repoUUID, Valid: true})
		if getErr == nil {
			repoPath = existing.Path
		}
	}

	err = rm.queries.DeleteRepository(context.Background(), pgtype.UUID{Bytes: repoUUID, Valid: true})
	if err != nil {
		rm.repoAudit(repoPath).Error("repository.remove", err, zap.String("repository_id", id))
		return fmt.Errorf("failed to remove repository: %w", err)
	}

	rm.repoAudit(repoPath).Operation("repository.remove", zap.String("repository_id", id))

	return nil
}

// GetStagingManager returns the staging manager instance
func (rm *DefaultRepositoryManager) GetStagingManager() StagingManager {
	return rm.stagingManager
}

// GetDirectoryManager returns the underlying DirectoryManager for direct file operations.
func (rm *DefaultRepositoryManager) GetDirectoryManager() DirectoryManager {
	return rm.dirManager
}

func (rm *DefaultRepositoryManager) UpdateRepository(id string, config repocfg.RepositoryConfig, defaultOwnerID *int32) (*repo.Repository, error) {
	repoUUID, err := uuid.Parse(id)
	if err != nil {
		rm.logger.Warn("repository update failed: invalid id", zap.String("operation", "repository.update"), zap.String("repository_id", id), zap.Error(err))
		return nil, fmt.Errorf("invalid repository ID: %w", err)
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	// The on-disk .lumiliorepo is authoritative and the DB config column is a
	// cache of it. An offline repository cannot take the disk write, so accepting
	// the DB half would leave the two forked until reconcile silently reverted
	// the edit on remount. Refuse the whole edit instead.
	existing, err := rm.GetRepository(id)
	if err != nil {
		return nil, err
	}
	if existing.Status == dbtypes.RepoStatusOffline {
		return nil, fmt.Errorf("%w: %s", ErrRepositoryOffline, existing.Path)
	}

	// Update database record
	now := time.Now()
	dbRepo, err := rm.queries.UpdateRepository(context.Background(), repo.UpdateRepositoryParams{
		RepoID:         pgtype.UUID{Bytes: repoUUID, Valid: true},
		Name:           config.Name,
		Config:         config,
		DefaultOwnerID: defaultOwnerID,
		UpdatedAt:      pgtype.Timestamptz{Time: now, Valid: true},
	})
	if err != nil {
		rm.repoAudit("").Error("repository.update", err, zap.String("repository_id", id))
		return nil, fmt.Errorf("failed to update repository: %w", err)
	}

	// Update configuration file
	if err := config.SaveConfigToFile(dbRepo.Path); err != nil {
		rm.repoAudit(dbRepo.Path).Error("repository.update", err, zap.String("repository_id", id))
		return nil, fmt.Errorf("failed to update configuration file: %w", err)
	}

	rm.repoAudit(dbRepo.Path).Operation("repository.update",
		zap.String("repository_id", id),
		zap.String("repository_name", config.Name),
	)

	return &dbRepo, nil
}

func (rm *DefaultRepositoryManager) repoAudit(repoPath string) logging.RepositoryAuditLogger {
	if rm.auditProvider == nil {
		return logging.NoopRepositoryAuditLogger()
	}
	return rm.auditProvider.ForPath(repoPath)
}

func normalizeRepoRole(role dbtypes.RepoRole) dbtypes.RepoRole {
	switch role {
	case dbtypes.RepoRolePrimary:
		return dbtypes.RepoRolePrimary
	default:
		return dbtypes.RepoRoleRegular
	}
}

// GetRepositoryPath returns the repository path for use in asset filtering
func (rm *DefaultRepositoryManager) GetRepositoryPath(repoID string) (string, error) {
	repository, err := rm.GetRepository(repoID)
	if err != nil {
		return "", fmt.Errorf("failed to get repository: %w", err)
	}
	// Same reason as the media read path: a caller handed an unreachable path
	// can only produce a bare I/O error, which cannot be told apart from missing
	// data.
	if repository.Status == dbtypes.RepoStatusOffline {
		return "", fmt.Errorf("%w: %s", ErrRepositoryOffline, repository.Name)
	}
	return repository.Path, nil
}
