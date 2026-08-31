package storage

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"server/internal/db/catalogtx"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/logging"
	"server/internal/storage/repocfg"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ValidationResult represents the result of repository validation
type ValidationResult struct {
	Valid    bool     `json:"valid"`
	Errors   []string `json:"errors,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

var ErrPrimaryRepositoryNotRemovable = errors.New("primary repository cannot be removed")

type RepositoryRemovalImpact struct {
	RepositoryID      string
	RepositoryName    string
	AssetCount        int64
	CatalogMediaBytes int64
	AlbumCount        int64
	ActiveTaskCount   int64
	CloudImportCount  int64
	PrivateStateBytes int64
	PrivateStateFound bool
}

type RepositoryRootRemovalImpact struct {
	RootID               string
	RootName             string
	Kind                 dbtypes.RepositoryRootKind
	RepositoryCount      int64
	ActiveOperationCount int64
	CanRemove            bool
	BlockingReason       string
	FilesPreserved       bool
}

type RepositoryCandidate struct {
	DirectoryName  string
	Classification string
	RepositoryID   string
	Name           string
	Writable       bool
	MountPoint     bool
	CanCreate      bool
	CanOpen        bool
	Actions        []string
	CapacityKnown  bool
	TotalBytes     uint64
	AvailableBytes uint64
	Filesystem     string
	RiskWarnings   []string
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
	InitializeRepository(path string, config repocfg.RepositoryConfig, defaultOwnerID *int32, role dbtypes.RepoRole, rootID ...uuid.UUID) (*repo.Repository, error)

	// AddRepository registers an already-initialized on-disk repository (one
	// that has a valid .lumiliorepo). It fails if the path is not a valid
	// repository or is already registered. If the repository's ID is registered
	// at a different path it returns a *RepositoryConflictError, which the caller
	// resolves with RelocateRepository or RegisterRepositoryCopy.
	AddRepository(path string, defaultOwnerID *int32, role dbtypes.RepoRole, rootID ...uuid.UUID) (*repo.Repository, error)

	// OpenRepository is the durable user-facing attach workflow. It isolates
	// repository-private state left by a previous registration before inserting
	// the catalog row and scheduling an authoritative initial scan.
	OpenRepository(ctx context.Context, path string, defaultOwnerID *int32, role dbtypes.RepoRole, request LifecycleRequest) (*repo.Repository, error)
	ListDefaultRepositoryCandidates(ctx context.Context) ([]RepositoryCandidate, error)
	OpenDefaultRepositoryCandidate(ctx context.Context, directoryName string, defaultOwnerID *int32, request LifecycleRequest) (*repo.Repository, error)
	ResolveDefaultRepositoryCandidate(ctx context.Context, directoryName, resolution string, defaultOwnerID *int32, request LifecycleRequest) (*repo.Repository, error)

	// Storage Locations are host-authorized repository containers. The default
	// location comes from immutable config; external locations come only from a
	// native Desktop grant, never an arbitrary shared-API path.
	EnsureDefaultRepositoryRoot(ctx context.Context, path string, request ...LifecycleRequest) (*repo.RepositoryRoot, error)
	AddRepositoryRoot(ctx context.Context, path, name string, request ...LifecycleRequest) (*repo.RepositoryRoot, error)
	RelocateRepositoryRoot(ctx context.Context, id, path string, request ...LifecycleRequest) (*repo.RepositoryRoot, error)
	ListRepositoryRoots(ctx context.Context) ([]repo.RepositoryRoot, error)
	GetRepositoryRoot(ctx context.Context, id string) (*repo.RepositoryRoot, error)
	PreviewRepositoryRootRemoval(ctx context.Context, id string) (RepositoryRootRemovalImpact, error)
	DeleteRepositoryRoot(ctx context.Context, id string, request ...LifecycleRequest) error
	ReconcileRepositoryRoots(ctx context.Context) error

	// RelocateRepository points an existing repository at a new on-disk
	// location. Assets and Locations are untouched because repository node paths
	// are relative graph projections.
	RelocateRepository(ctx context.Context, id string, newPath string, request ...LifecycleRequest) (*repo.Repository, error)

	// RegisterRepositoryCopy registers a duplicated repository directory as an
	// independent repository by minting a fresh UUID into its .lumiliorepo.
	RegisterRepositoryCopy(ctx context.Context, path string, defaultOwnerID *int32, role dbtypes.RepoRole, request ...LifecycleRequest) (*repo.Repository, error)

	// ReconcileAll re-checks every repository's recorded path against the
	// .lumiliorepo actually on disk, updating reachability status and refreshing
	// the cached config from disk.
	ReconcileAll(ctx context.Context) error

	// StorageRuntimeStatus derives current availability independently of the
	// durable first-run bootstrap phase. Only the default Storage Location and
	// primary repository can put the whole instance into degraded recovery.
	StorageRuntimeStatus(ctx context.Context) (StorageRuntimeStatus, error)
	RecoverLifecycleOperations(ctx context.Context) error
	RecordLifecycleAudit(ctx context.Context, input LifecycleAuditInput) (LifecycleAuditEvent, error)
	ListLifecycleAudit(ctx context.Context, filter LifecycleAuditFilter) ([]LifecycleAuditEvent, error)
	CheckRepositoryWriteCapacity(ctx context.Context, repositoryID string, expectedBytes uint64) (CapacityDecision, error)
	ReconcileRepositoryCapacity(ctx context.Context) error
	ScheduleInitialRepositoryScan(ctx context.Context, repositoryID string) error
	RetryPendingInitialRepositoryScans(ctx context.Context) error
	CreateHostAction(ctx context.Context, input CreateHostActionInput) (HostAction, error)
	GetHostAction(ctx context.Context, id string) (HostAction, error)
	ListPendingHostActions(ctx context.Context) ([]HostAction, error)
	SetHostActionExpectedVersion(ctx context.Context, actionID, nonce string, version uint64) (HostAction, error)
	ListHostActionsForActor(ctx context.Context, actorUserID int32) ([]HostAction, error)
	RecoverHostActions(ctx context.Context) error
	AcquireRuntimeStorageOwnership(ctx context.Context) (func(), error)
	ExecuteHostAction(ctx context.Context, actionID, nonce, hostInstanceID, selectedPath string, riskConfirmation ...bool) (HostAction, error)
	ResolveHostAction(ctx context.Context, actionID, resolution string, riskConfirmation ...bool) (HostAction, error)
	CancelHostAction(ctx context.Context, actionID string) (HostAction, error)

	// GetRepository returns the repository with the given UUID, or an error if
	// the id is malformed or no such repository exists.
	GetRepository(id string) (*repo.Repository, error)

	// GetRepositoryByPath returns the repository registered at the given path
	// (matched after cleaning to an absolute path), or an error if none is.
	GetRepositoryByPath(path string) (*repo.Repository, error)

	// ListRepositories returns all registered repositories.
	ListRepositories() ([]*repo.Repository, error)

	// HostOwnerID returns the initial administrator that represents ownership
	// of this host. The primary repository pins the identity after bootstrap;
	// before then, the first account is used. Nil means setup has no user yet.
	HostOwnerID(ctx context.Context) (*int32, error)

	RenameRepository(ctx context.Context, id, name string, request ...LifecycleRequest) (*repo.Repository, error)
	BeginRepositoryWork(ctx context.Context, id string, activity dbtypes.RepositoryActivity) (*repo.Repository, func() error, error)
	ReadRepositorySidecar(ctx context.Context, repositoryID, assetID string) ([]byte, error)
	WriteRepositorySidecar(ctx context.Context, repositoryID, assetID string, data []byte) error

	// Removal deletes repository-scoped catalog state while preserving every
	// on-disk file and marker. Preview supplies the required confirmation facts.
	PreviewRepositoryRemoval(ctx context.Context, id string) (RepositoryRemovalImpact, error)
	RemoveRepository(ctx context.Context, id string, request ...LifecycleRequest) error

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
}

// DefaultRepositoryManager implements the RepositoryManager interface
type DefaultRepositoryManager struct {
	database       *sql.DB
	writer         *catalogtx.Writer
	readerDatabase *sql.DB
	queries        *repo.Queries
	readerQueries  *repo.Queries
	dirManager     DirectoryManager
	files          *RepositoryFSFactory
	logger         *zap.Logger
	auditProvider  logging.RepositoryAuditProvider
	initialScan    func(context.Context, string) error
	ownershipMu    sync.Mutex
	ownershipOn    bool
	ownership      map[string]func()
}

// SetInitialScanEnqueuer connects the storage lifecycle to the queue only
// after the queue has been constructed. Storage tests and maintenance tools
// can leave it nil; the running application always installs it before serving.
func (rm *DefaultRepositoryManager) SetInitialScanEnqueuer(enqueue func(context.Context, string) error) {
	rm.initialScan = enqueue
}

func (rm *DefaultRepositoryManager) ScheduleInitialRepositoryScan(ctx context.Context, repositoryID string) error {
	if rm.initialScan == nil {
		return nil
	}
	return rm.initialScan(ctx, repositoryID)
}

// NewRepositoryManager creates a new repository manager instance
func NewRepositoryManager(
	database *sql.DB,
	queries *repo.Queries,
	logger *zap.Logger,
	auditProvider logging.RepositoryAuditProvider,
	files *RepositoryFSFactory,
) (*DefaultRepositoryManager, error) {
	return NewRepositoryManagerWithReader(database, database, queries, logger, auditProvider, files)
}

func NewRepositoryManagerWithReader(
	database *sql.DB,
	readerDatabase *sql.DB,
	queries *repo.Queries,
	logger *zap.Logger,
	auditProvider logging.RepositoryAuditProvider,
	files *RepositoryFSFactory,
) (*DefaultRepositoryManager, error) {
	return NewRepositoryManagerWithCatalog(database, catalogtx.NewWriter(database, nil), readerDatabase, queries, logger, auditProvider, files)
}

func NewRepositoryManagerWithCatalog(
	database *sql.DB,
	writer *catalogtx.Writer,
	readerDatabase *sql.DB,
	queries *repo.Queries,
	logger *zap.Logger,
	auditProvider logging.RepositoryAuditProvider,
	files *RepositoryFSFactory,
) (*DefaultRepositoryManager, error) {
	if logger == nil {
		logger = zap.NewNop()
	}
	if files == nil {
		files = NewRepositoryFSFactory(nil, queries)
	}

	readerQueries := queries
	if readerDatabase != nil {
		readerQueries = repo.New(readerDatabase)
	}
	rm := &DefaultRepositoryManager{
		database:       database,
		writer:         writer,
		readerDatabase: readerDatabase,
		queries:        queries,
		readerQueries:  readerQueries,
		dirManager:     NewDirectoryManager(),
		files:          files,
		logger:         logger.With(zap.String("component", "repository")),
		auditProvider:  auditProvider,
	}
	return rm, nil
}

// Ensure the concrete type satisfies the consumer interface.
var _ RepositoryManager = (*DefaultRepositoryManager)(nil)

// HostOwnerID resolves the one host-level fallback owner used by every
// repository. Repository defaults are ingestion policy, not per-repository
// authorization; explicit upload and cloud owners still take precedence.
func (rm *DefaultRepositoryManager) HostOwnerID(ctx context.Context) (*int32, error) {
	ownerID, err := rm.queries.GetHostOwnerID(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("resolve host owner: %w", err)
	}
	return &ownerID, nil
}

// AddRepository registers an existing repository with the system
func (rm *DefaultRepositoryManager) AddRepository(path string, defaultOwnerID *int32, role dbtypes.RepoRole, rootID ...uuid.UUID) (*repo.Repository, error) {
	return rm.addRepository(context.Background(), path, defaultOwnerID, role, false, rootID...)
}

func (rm *DefaultRepositoryManager) addRepository(ctx context.Context, path string, defaultOwnerID *int32, role dbtypes.RepoRole, locksHeld bool, rootID ...uuid.UUID) (*repo.Repository, error) {
	// Clean and validate path
	cleanPath, err := CanonicalizeRepositoryPath(path)
	if err != nil {
		rm.logger.Warn("repository add failed: invalid path", zap.String("operation", "repository.add"), zap.String("path", path), zap.Error(err))
		return nil, fmt.Errorf("invalid path: %w", err)
	}
	if err := rm.claimRuntimeStoragePath(ctx, "repository", cleanPath); err != nil {
		return nil, err
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
			Actions:        repositoryConflictActions(registered.Path, config.ID),
		}
	}

	repoUUID, err := uuid.Parse(config.ID)
	if err != nil {
		return nil, fmt.Errorf("invalid repository ID: %w", err)
	}
	associatedRootID, err := rm.resolveRepositoryAssociation(ctx, cleanPath, rootID)
	if err != nil {
		return nil, err
	}
	associatedRoot, err := rm.queries.GetRepositoryRoot(ctx, associatedRootID)
	if err != nil {
		return nil, fmt.Errorf("load repository Storage Location: %w", err)
	}
	if err := rm.claimRuntimeStoragePath(ctx, "root", associatedRoot.Path); err != nil {
		return nil, err
	}
	releaseRoot := func() {}
	releaseMutation := func() {}
	if !locksHeld {
		releaseRoot = rm.acquireRepositoryRootRead(associatedRootID)
		releaseMutation = rm.acquireRepositoryMutation(repoUUID)
	}
	defer releaseRoot()
	defer releaseMutation()

	now := time.Now()
	dbRepo, err := rm.queries.CreateRepository(ctx, repo.CreateRepositoryParams{
		RepoID:         repoUUID,
		Name:           config.Name,
		Path:           cleanPath,
		Config:         *config,
		Role:           normalizeRepoRole(role),
		Reachability:   dbtypes.RepositoryReachabilityActive,
		Activity:       dbtypes.RepositoryActivityIdle,
		DefaultOwnerID: defaultOwnerID,
		CreatedAt:      dbtypes.NewTimestamp(config.CreatedAt),
		UpdatedAt:      dbtypes.NewTimestamp(now),
		RootID:         associatedRootID,
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

	return requireDirectoryWritable(path)
}

// InitializeRepository creates a new repository with full directory structure
func (rm *DefaultRepositoryManager) InitializeRepository(path string, config repocfg.RepositoryConfig, defaultOwnerID *int32, role dbtypes.RepoRole, rootID ...uuid.UUID) (*repo.Repository, error) {
	return rm.initializeRepository(context.Background(), path, config, defaultOwnerID, role, nil, rootID...)
}

func (rm *DefaultRepositoryManager) initializeRepository(
	ctx context.Context,
	path string,
	config repocfg.RepositoryConfig,
	defaultOwnerID *int32,
	role dbtypes.RepoRole,
	operation *repo.LifecycleOperation,
	rootID ...uuid.UUID,
) (*repo.Repository, error) {
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
	repoUUID, err := uuid.Parse(config.ID)
	if err != nil {
		return nil, fmt.Errorf("invalid repository ID: %w", err)
	}
	associatedRootID, err := rm.resolveRepositoryAssociation(ctx, cleanPath, rootID)
	if err != nil {
		return nil, err
	}
	associatedRoot, err := rm.queries.GetRepositoryRoot(ctx, associatedRootID)
	if err != nil {
		return nil, fmt.Errorf("load repository Storage Location: %w", err)
	}
	if err := rm.claimRuntimeStoragePath(ctx, "root", associatedRoot.Path); err != nil {
		return nil, err
	}
	releaseRoot := rm.acquireRepositoryRootRead(associatedRootID)
	defer releaseRoot()
	releaseMutation := rm.acquireRepositoryMutation(repoUUID)
	defer releaseMutation()

	// A Server operator may bind-mount an empty host directory directly at this
	// path before creating the repository. Prove the exact target is empty and
	// writable, while remembering whether Lumilio owns the directory itself so
	// rollback never recursively removes a pre-existing mount.
	targetCreated, err := prepareRepositoryInitializationTarget(cleanPath)
	if err != nil {
		rm.repoAudit(cleanPath).Error("repository.initialize", err, zap.String("repository_name", config.Name))
		return nil, err
	}
	if err := rm.claimRuntimeStoragePath(ctx, "repository", cleanPath); err != nil {
		_ = cleanupRepositoryInitializationTarget(cleanPath, targetCreated)
		return nil, err
	}
	rollbackData := createRepositoryRollbackData{Path: cleanPath, TargetCreated: targetCreated}
	if operation != nil {
		if err := rm.updateLifecycleOperationPhase(ctx, operation.OperationID, lifecyclePhasePrepared, rollbackData); err != nil {
			_ = cleanupRepositoryInitializationTarget(cleanPath, targetCreated)
			return nil, fmt.Errorf("persist repository create preparation: %w", err)
		}
	}
	rollback := func(cause error) error {
		cleanupErr := cleanupRepositoryInitializationTarget(cleanPath, targetCreated)
		if operation != nil {
			_ = rm.failLifecycleOperation(ctx, operation.OperationID, cleanupErr == nil, cause, rollbackData)
		}
		if cleanupErr == nil {
			return cause
		}
		return errors.Join(cause, fmt.Errorf("rollback repository initialization: %w", cleanupErr))
	}

	// Create directory structure using directory manager
	if err := rm.dirManager.CreateStructure(cleanPath); err != nil {
		rm.repoAudit(cleanPath).Error("repository.initialize", err, zap.String("repository_name", config.Name))
		return nil, rollback(fmt.Errorf("failed to create repository structure: %w", err))
	}

	// Save configuration file
	if err := config.SaveConfigToFile(cleanPath); err != nil {
		rm.repoAudit(cleanPath).Error("repository.initialize", err, zap.String("repository_name", config.Name))
		return nil, rollback(fmt.Errorf("failed to save configuration: %w", err))
	}
	if operation != nil {
		if err := rm.updateLifecycleOperationPhase(ctx, operation.OperationID, lifecyclePhaseFilesystemApplied, rollbackData); err != nil {
			return nil, rollback(fmt.Errorf("persist repository filesystem phase: %w", err))
		}
	}

	now := time.Now()
	dbRepo, err := rm.queries.CreateRepository(ctx, repo.CreateRepositoryParams{
		RepoID:         repoUUID,
		Name:           config.Name,
		Path:           cleanPath,
		Config:         config,
		Role:           normalizeRepoRole(role),
		Reachability:   dbtypes.RepositoryReachabilityActive,
		Activity:       dbtypes.RepositoryActivityIdle,
		DefaultOwnerID: defaultOwnerID,
		CreatedAt:      dbtypes.NewTimestamp(config.CreatedAt),
		UpdatedAt:      dbtypes.NewTimestamp(now),
		RootID:         associatedRootID,
	})
	if err != nil {
		rm.repoAudit(cleanPath).Error("repository.initialize", err, zap.String("repository_id", config.ID), zap.String("repository_name", config.Name))
		return nil, rollback(fmt.Errorf("failed to create database record: %w", err))
	}
	if operation != nil {
		if err := rm.updateLifecycleOperationPhase(ctx, operation.OperationID, lifecyclePhaseCatalogCommitted, rollbackData); err != nil {
			return nil, fmt.Errorf("repository created but journal commit phase failed: %w", err)
		}
		if err := rm.completeLifecycleOperation(ctx, operation.OperationID, createRepositoryOperationResult{RepositoryID: dbRepo.RepoID.String()}); err != nil {
			return nil, fmt.Errorf("repository created but journal completion failed: %w", err)
		}
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

func prepareRepositoryInitializationTarget(path string) (created bool, err error) {
	info, err := os.Stat(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		if err := os.Mkdir(path, 0o755); err != nil {
			return false, fmt.Errorf("%w: create target directory: %v", ErrRepositoryStorageNotWritable, err)
		}
		created = true
	case err != nil:
		return false, fmt.Errorf("%w: inspect target directory: %v", ErrRepositoryStorageNotWritable, err)
	case !info.IsDir():
		return false, fmt.Errorf("%w: target path is not a directory", ErrRepositoryTargetNotEmpty)
	default:
		entries, err := os.ReadDir(path)
		if err != nil {
			return false, fmt.Errorf("%w: read target directory: %v", ErrRepositoryStorageNotWritable, err)
		}
		if len(entries) != 0 {
			return false, fmt.Errorf("%w: %s", ErrRepositoryTargetNotEmpty, path)
		}
	}

	probe, err := os.CreateTemp(path, ".lumilio-write-test-*")
	if err != nil {
		if created {
			_ = os.RemoveAll(path)
		}
		return false, fmt.Errorf("%w: create probe file: %v", ErrRepositoryStorageNotWritable, err)
	}
	probePath := probe.Name()
	if closeErr := probe.Close(); closeErr != nil {
		_ = os.Remove(probePath)
		if created {
			_ = os.RemoveAll(path)
		}
		return false, fmt.Errorf("%w: close probe file: %v", ErrRepositoryStorageNotWritable, closeErr)
	}
	if removeErr := os.Remove(probePath); removeErr != nil {
		if created {
			_ = os.RemoveAll(path)
		}
		return false, fmt.Errorf("%w: remove probe file: %v", ErrRepositoryStorageNotWritable, removeErr)
	}
	return created, nil
}

func cleanupRepositoryInitializationTarget(path string, targetCreated bool) error {
	if targetCreated {
		return os.RemoveAll(path)
	}

	var cleanupErrors []error
	for _, name := range []string{
		DefaultStructure.ConfigFile,
		DefaultStructure.SystemDir,
		DefaultStructure.InboxDir,
	} {
		target := filepath.Join(path, name)
		if err := os.RemoveAll(target); err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("remove %s: %w", target, err))
		}
	}
	return errors.Join(cleanupErrors...)
}

func (rm *DefaultRepositoryManager) GetRepository(id string) (*repo.Repository, error) {
	repoUUID, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("invalid repository ID: %w", err)
	}

	dbRepo, err := rm.queries.GetRepository(context.Background(), repoUUID)
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

func (rm *DefaultRepositoryManager) PreviewRepositoryRemoval(ctx context.Context, id string) (RepositoryRemovalImpact, error) {
	repoUUID, err := uuid.Parse(id)
	if err != nil {
		return RepositoryRemovalImpact{}, fmt.Errorf("invalid repository ID: %w", err)
	}
	existing, err := rm.queries.GetRepository(ctx, repoUUID)
	if err != nil {
		return RepositoryRemovalImpact{}, fmt.Errorf("load repository removal impact: %w", err)
	}
	impact := RepositoryRemovalImpact{
		RepositoryID: existing.RepoID.String(), RepositoryName: existing.Name,
	}
	if err := rm.readerDatabase.QueryRowContext(ctx, `
		WITH repository_assets AS (
			SELECT occurrence.asset_id, max(occurrence.file_size) AS file_size
			FROM active_asset_occurrences occurrence
			WHERE occurrence.repository_id = ?
			GROUP BY occurrence.asset_id
		)
		SELECT count(*), coalesce(sum(file_size), 0)
		FROM repository_assets
	`, repoUUID).Scan(&impact.AssetCount, &impact.CatalogMediaBytes); err != nil {
		return RepositoryRemovalImpact{}, fmt.Errorf("count repository assets: %w", err)
	}
	if err := rm.readerDatabase.QueryRowContext(ctx, `
		SELECT count(DISTINCT album_assets.album_id)
		FROM album_assets
		JOIN active_asset_occurrences occurrence
		  ON occurrence.asset_id = album_assets.asset_id
		WHERE occurrence.repository_id = ?
	`, repoUUID).Scan(&impact.AlbumCount); err != nil {
		return RepositoryRemovalImpact{}, fmt.Errorf("count affected albums: %w", err)
	}
	if err := rm.readerDatabase.QueryRowContext(ctx, `
		SELECT
		  (SELECT count(*) FROM domain_outbox WHERE delivered_at IS NULL AND (subject_key=? OR EXISTS (SELECT 1 FROM repository_staging_commits staging WHERE staging.commit_id=domain_outbox.subject_key AND staging.repository_id=?)))
		  + (SELECT count(*) FROM repository_observation_state WHERE repository_id=? AND desired_epoch>applied_epoch)
	`, repoUUID.String(), repoUUID, repoUUID).Scan(&impact.ActiveTaskCount); err != nil {
		return RepositoryRemovalImpact{}, fmt.Errorf("count repository catalog work: %w", err)
	}
	if err := rm.readerDatabase.QueryRowContext(ctx, `
		SELECT count(*) FROM cloud_import_runs WHERE repository_id = ?
	`, repoUUID).Scan(&impact.CloudImportCount); err != nil {
		return RepositoryRemovalImpact{}, fmt.Errorf("count repository cloud import receipts: %w", err)
	}
	privatePath := filepath.Join(existing.Path, DefaultStructure.SystemDir)
	if info, statErr := os.Stat(privatePath); statErr == nil && info.IsDir() {
		impact.PrivateStateFound = true
		impact.PrivateStateBytes, err = directoryTreeSize(privatePath)
		if err != nil {
			return RepositoryRemovalImpact{}, fmt.Errorf("measure repository private state: %w", err)
		}
	} else if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
		return RepositoryRemovalImpact{}, fmt.Errorf("inspect repository private state: %w", statErr)
	}
	return impact, nil
}

func (rm *DefaultRepositoryManager) RemoveRepository(ctx context.Context, id string, requests ...LifecycleRequest) error {
	repoUUID, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("invalid repository ID: %w", err)
	}
	existing, err := rm.queries.GetRepository(ctx, repoUUID)
	if err != nil {
		return err
	}
	if existing.Role == dbtypes.RepoRolePrimary {
		return ErrPrimaryRepositoryNotRemovable
	}
	if existing.Activity != dbtypes.RepositoryActivityIdle {
		return fmt.Errorf("%w: repository activity is %s", ErrRepositoryBusy, existing.Activity)
	}
	coordinator := rm.files.AccessCoordinator()
	releaseRoot, err := coordinator.AcquireRootReadContext(ctx, existing.RootID)
	if err != nil {
		return fmt.Errorf("%w: Storage Location is busy: %v", ErrRepositoryBusy, err)
	}
	defer releaseRoot()
	releaseMutation, err := coordinator.AcquireMutationsContext(ctx, []uuid.UUID{repoUUID})
	if err != nil {
		return fmt.Errorf("%w: repository is busy: %v", ErrRepositoryBusy, err)
	}
	defer releaseMutation()

	existing, err = rm.queries.GetRepository(ctx, repoUUID)
	if err != nil {
		return err
	}
	var runningOperations int
	if err := rm.readerDatabase.QueryRowContext(ctx, `
		SELECT count(*) FROM lifecycle_operations
		WHERE target_type = 'repository' AND target_id = ? AND status = 'running'
	`, repoUUID.String()).Scan(&runningOperations); err != nil {
		return fmt.Errorf("inspect repository lifecycle operations: %w", err)
	}
	if runningOperations != 0 {
		return fmt.Errorf("%w: repository has an active lifecycle operation", ErrRepositoryBusy)
	}
	if _, err := rm.queries.BeginRepositoryMaintenance(ctx, repo.BeginRepositoryMaintenanceParams{
		RepoID: repoUUID, UpdatedAt: dbtypes.NewTimestamp(time.Now().UTC()),
	}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("%w: repository changed state before maintenance began", ErrRepositoryBusy)
		}
		return fmt.Errorf("enter repository removal maintenance: %w", err)
	}
	maintenanceCommitted := true
	defer func() {
		if !maintenanceCommitted {
			return
		}
		_, _ = rm.queries.EndRepositoryMaintenance(context.Background(), repo.EndRepositoryMaintenanceParams{
			RepoID: repoUUID, Reachability: existing.Reachability, Activity: existing.Activity,
			UpdatedAt: dbtypes.NewTimestamp(time.Now().UTC()),
		})
	}()
	tx, err := rm.writer.BeginTx(ctx, catalogtx.OperationRepositoryRemove, nil)
	if err != nil {
		return fmt.Errorf("begin repository removal: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	queries := rm.queries.WithTx(tx.Raw())
	// The repository mutation lock prevents new or running repository compute
	// from crossing this transaction. Catalog cascades remove desired state;
	// pending domain commands are explicitly removed for the deleted subject.
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM share_links
		WHERE EXISTS (
			SELECT 1 FROM json_each(share_links.asset_ids) selected
			JOIN active_asset_occurrences target ON target.asset_id = selected.value
			WHERE target.repository_id = ?
			  AND NOT EXISTS (
				SELECT 1 FROM active_asset_occurrences survivor
				WHERE survivor.asset_id = target.asset_id
				  AND survivor.repository_id <> ?
			  )
		)
	`, repoUUID, repoUUID); err != nil {
		return fmt.Errorf("remove repository share links: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM agent_pins
		WHERE EXISTS (
			SELECT 1 FROM json_each(agent_pins.asset_ids) selected
			JOIN active_asset_occurrences target ON target.asset_id = selected.value
			WHERE target.repository_id = ?
			  AND NOT EXISTS (
				SELECT 1 FROM active_asset_occurrences survivor
				WHERE survivor.asset_id = target.asset_id
				  AND survivor.repository_id <> ?
			  )
		)
	`, repoUUID, repoUUID); err != nil {
		return fmt.Errorf("remove repository agent pins: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM domain_outbox
		WHERE (command_kind = 'repository.scan' AND subject_key = ?)
		   OR (command_kind = 'projection.location' AND subject_key LIKE ? || ':%')
		   OR EXISTS (
				SELECT 1
				FROM repository_staging_commits staging
				WHERE staging.commit_id = domain_outbox.subject_key
				  AND staging.repository_id = ?
		   )
	`, repoUUID.String(), repoUUID.String(), repoUUID); err != nil {
		return fmt.Errorf("remove repository domain commands: %w", err)
	}
	// Logical media and stacks are projections of Assets, not repository-owned
	// filesystem entries. Rehome them before deleting the repository whenever a
	// member still has an active Location elsewhere; otherwise the repository
	// foreign-key cascade would discard a valid exact-copy projection.
	if _, err := tx.ExecContext(ctx, `
		UPDATE media_items AS item
		SET repository_id = (
			SELECT occurrence.repository_id
			FROM active_asset_occurrences occurrence
			WHERE occurrence.repository_id <> ?
			  AND (
				occurrence.asset_id = item.primary_asset_id
				OR EXISTS (
					SELECT 1 FROM media_item_assets member
					WHERE member.media_item_id = item.media_item_id
					  AND member.asset_id = occurrence.asset_id
				)
			  )
			ORDER BY occurrence.repository_id
			LIMIT 1
		)
		WHERE item.repository_id = ?
		  AND EXISTS (
			SELECT 1 FROM active_asset_occurrences occurrence
			WHERE occurrence.repository_id <> ?
			  AND (
				occurrence.asset_id = item.primary_asset_id
				OR EXISTS (
					SELECT 1 FROM media_item_assets member
					WHERE member.media_item_id = item.media_item_id
					  AND member.asset_id = occurrence.asset_id
				)
			  )
		  )
	`, repoUUID, repoUUID, repoUUID); err != nil {
		return fmt.Errorf("rehome repository media items: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE asset_stacks AS stack
		SET repository_id = (
			SELECT item.repository_id
			FROM asset_stack_members member
			JOIN media_items item ON item.media_item_id = member.media_item_id
			WHERE member.stack_id = stack.stack_id
			  AND item.repository_id IS NOT NULL
			  AND item.repository_id <> ?
			ORDER BY item.repository_id
			LIMIT 1
		)
		WHERE stack.repository_id = ?
		  AND EXISTS (
			SELECT 1
			FROM asset_stack_members member
			JOIN media_items item ON item.media_item_id = member.media_item_id
			WHERE member.stack_id = stack.stack_id
			  AND item.repository_id IS NOT NULL
			  AND item.repository_id <> ?
		  )
	`, repoUUID, repoUUID, repoUUID); err != nil {
		return fmt.Errorf("rehome repository asset stacks: %w", err)
	}
	// Repository removal is the explicit catalog-GC boundary. Delete an Asset
	// only when every active physical occurrence belongs to this repository.
	// Assets with an exact-copy Location elsewhere survive the node cascade.
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM assets
		WHERE EXISTS (
			SELECT 1 FROM active_asset_occurrences target
			WHERE target.asset_id = assets.asset_id
			  AND target.repository_id = ?
		)
		  AND NOT EXISTS (
			SELECT 1 FROM active_asset_occurrences survivor
			WHERE survivor.asset_id = assets.asset_id
			  AND survivor.repository_id <> ?
		  )
	`, repoUUID, repoUUID); err != nil {
		return fmt.Errorf("garbage-collect repository assets: %w", err)
	}
	if err := queries.DeleteRepository(ctx, repoUUID); err != nil {
		return fmt.Errorf("delete repository catalog: %w", err)
	}
	request := firstLifecycleRequest(requests)
	if _, err := recordLifecycleAuditWithQueries(ctx, queries, LifecycleAuditInput{
		Actor: request.Actor, ActorUserID: request.ActorUserID, HostInstanceID: request.HostInstanceID, RequestID: request.RequestID,
		Action: "remove_repository", TargetType: "repository", TargetID: id,
		Source: auditSourceForActor(request.Actor), ConfirmationType: "exact_repository_name",
		OldPath: existing.Path, Result: AuditResultSucceeded,
		Details: map[string]any{"repository_name": existing.Name, "files_preserved": true},
	}); err != nil {
		return fmt.Errorf("audit repository removal: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit repository removal: %w", err)
	}
	maintenanceCommitted = false

	rm.repoAudit(existing.Path).Operation("repository.remove",
		zap.String("repository_id", id),
		zap.String("repository_name", existing.Name),
		zap.String("preserved_path", existing.Path),
	)
	return nil
}

func firstLifecycleRequest(requests []LifecycleRequest) LifecycleRequest {
	if len(requests) == 0 {
		return LifecycleRequest{Actor: "server"}
	}
	return requests[0]
}

func directoryTreeSize(root string) (int64, error) {
	var total int64
	err := filepath.WalkDir(root, func(_ string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		total += info.Size()
		return nil
	})
	return total, err
}

// BeginRepositoryWork establishes the parent identity lease and persistent
// activity gate used by lifecycle maintenance. Its callers publish only
// catalog/queue intent, so they deliberately do not take the repository's
// filesystem mutation lease: waiting for active media readers would put an
// unbounded filesystem operation on a synchronous request path. The activity
// compare-and-swap serializes enqueue with repository removal, and the worker
// resolves the current Location again before execution.
//
// Callers must invoke the returned release function after their catalog write
// or queue-enqueue section.
func (rm *DefaultRepositoryManager) BeginRepositoryWork(ctx context.Context, id string, activity dbtypes.RepositoryActivity) (*repo.Repository, func() error, error) {
	repositoryID, err := uuid.Parse(strings.TrimSpace(id))
	if err != nil {
		return nil, nil, fmt.Errorf("invalid repository ID: %w", err)
	}
	repository, err := rm.queries.GetRepository(ctx, repositoryID)
	if err != nil {
		return nil, nil, err
	}
	coordinator := rm.files.AccessCoordinator()
	releaseRoot, err := coordinator.AcquireRootReadContext(ctx, repository.RootID)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: Storage Location is busy: %v", ErrRepositoryBusy, err)
	}
	fail := func(cause error) (*repo.Repository, func() error, error) {
		releaseRoot()
		return nil, nil, cause
	}
	repository, err = rm.queries.GetRepository(ctx, repositoryID)
	if err != nil {
		return fail(err)
	}
	if err := rm.files.ValidateRepositoryParent(ctx, repository); err != nil {
		return fail(err)
	}
	started, err := rm.queries.BeginRepositoryActivity(ctx, repo.BeginRepositoryActivityParams{
		RepoID: repositoryID, Activity: activity, UpdatedAt: dbtypes.NewTimestamp(time.Now().UTC()),
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fail(fmt.Errorf("%w: repository is unavailable or has active work", ErrRepositoryBusy))
		}
		return fail(fmt.Errorf("begin repository work: %w", err))
	}
	released := false
	release := func() error {
		if released {
			return nil
		}
		released = true
		_, finishErr := rm.queries.FinishRepositoryActivity(context.Background(), repo.FinishRepositoryActivityParams{
			RepoID: repositoryID, Activity: activity, UpdatedAt: dbtypes.NewTimestamp(time.Now().UTC()),
		})
		releaseRoot()
		return finishErr
	}
	return &started, release, nil
}

func (rm *DefaultRepositoryManager) WriteRepositorySidecar(ctx context.Context, repositoryID, assetID string, data []byte) error {
	id, err := uuid.Parse(strings.TrimSpace(repositoryID))
	if err != nil {
		return err
	}
	repository, err := rm.queries.BeginRepositoryActivity(ctx, repo.BeginRepositoryActivityParams{
		RepoID: id, Activity: dbtypes.RepositoryActivityProcessing, UpdatedAt: dbtypes.NewTimestamp(time.Now().UTC()),
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("%w: repository is unavailable or has active work", ErrRepositoryBusy)
		}
		return err
	}
	defer func() {
		_, _ = rm.queries.FinishRepositoryActivity(context.Background(), repo.FinishRepositoryActivityParams{
			RepoID: id, Activity: dbtypes.RepositoryActivityProcessing, UpdatedAt: dbtypes.NewTimestamp(time.Now().UTC()),
		})
	}()
	repositoryFS, err := rm.files.OpenContext(ctx, repository)
	if err != nil {
		return err
	}
	defer repositoryFS.Close()
	directory, err := ParsePrivateRepositoryPath(DefaultStructure.SidecarsDir)
	if err != nil {
		return err
	}
	if err := repositoryFS.MkdirAllPrivate(directory, 0o755); err != nil {
		return err
	}
	target, err := ParsePrivateRepositoryPath(path.Join(DefaultStructure.SidecarsDir,
		path.Base(strings.ReplaceAll(assetID, "\\", "/"))+".lumilio-sidecar"))
	if err != nil {
		return err
	}
	if err := repositoryFS.VerifyIdentity(); err != nil {
		return err
	}
	_, err = repositoryFS.WritePrivateFileAtomic(target, bytes.NewReader(data), 0o644)
	return err
}

func (rm *DefaultRepositoryManager) ReadRepositorySidecar(ctx context.Context, repositoryID, assetID string) ([]byte, error) {
	id, err := uuid.Parse(strings.TrimSpace(repositoryID))
	if err != nil {
		return nil, err
	}
	repository, err := rm.queries.GetRepository(ctx, id)
	if err != nil {
		return nil, err
	}
	repositoryFS, err := rm.files.OpenContext(ctx, repository)
	if err != nil {
		return nil, err
	}
	defer repositoryFS.Close()
	target, err := ParsePrivateRepositoryPath(path.Join(DefaultStructure.SidecarsDir,
		path.Base(strings.ReplaceAll(assetID, "\\", "/"))+".lumilio-sidecar"))
	if err != nil {
		return nil, err
	}
	content, err := repositoryFS.ReadPrivateFile(target)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	return content, err
}

// RenameRepository changes only the mutable display name. Storage strategy,
// duplicate handling, identity, ownership, role, root and path are copied from
// the authoritative existing marker/catalog record and cannot be supplied by
// the caller.
func (rm *DefaultRepositoryManager) RenameRepository(ctx context.Context, id, name string, requests ...LifecycleRequest) (*repo.Repository, error) {
	if err := ValidateRepositoryName(name); err != nil {
		return nil, err
	}
	repositoryID, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("invalid repository ID: %w", err)
	}
	repository, err := rm.queries.GetRepository(ctx, repositoryID)
	if err != nil {
		return nil, err
	}
	if repository.Reachability != dbtypes.RepositoryReachabilityActive {
		return nil, fmt.Errorf("%w: %s", ErrRepositoryOffline, repository.Path)
	}
	coordinator := rm.files.AccessCoordinator()
	releaseRoot, err := coordinator.AcquireRootReadContext(ctx, repository.RootID)
	if err != nil {
		return nil, fmt.Errorf("%w: Storage Location is busy: %v", ErrRepositoryBusy, err)
	}
	defer releaseRoot()
	releaseRepository, err := coordinator.AcquireMutationsContext(ctx, []uuid.UUID{repositoryID})
	if err != nil {
		return nil, fmt.Errorf("%w: repository is busy: %v", ErrRepositoryBusy, err)
	}
	defer releaseRepository()
	if err := rm.files.ValidateRepositoryParent(ctx, repository); err != nil {
		return nil, fmt.Errorf("validate repository parent before rename: %w", err)
	}

	previousConfig := repository.Config
	updatedConfig := previousConfig
	updatedConfig.Name = name
	request := firstLifecycleRequest(requests)
	operation, existed, err := rm.beginLifecycleOperation(ctx, lifecycleBeginInput{
		RequestID: request.RequestID, Kind: lifecycleKindRenameRepository,
		Payload: renameRepositoryOperationPayload{
			RepositoryID: id, Path: repository.Path, NewName: name,
		},
		Actor: request.Actor, ActorUserID: request.ActorUserID, HostInstanceID: request.HostInstanceID, TargetType: "repository", TargetID: &id,
		RollbackData: renameRepositoryRollbackData{PreviousConfig: previousConfig},
	})
	if err != nil {
		return nil, err
	}
	if existed {
		switch operation.Status {
		case lifecycleStatusCompleted:
			result, err := rm.queries.GetRepository(ctx, repositoryID)
			return &result, err
		case lifecycleStatusRunning:
			if err := rm.recoverRenameRepositoryOperation(ctx, operation); err != nil {
				return nil, err
			}
			result, err := rm.queries.GetRepository(ctx, repositoryID)
			return &result, err
		default:
			return nil, ErrLifecycleOperationFailed
		}
	}

	if err := updatedConfig.SaveConfigToFile(repository.Path); err != nil {
		_ = rm.failLifecycleOperation(ctx, operation.OperationID, true, err,
			renameRepositoryRollbackData{PreviousConfig: previousConfig})
		return nil, fmt.Errorf("write renamed repository marker: %w", err)
	}
	rollback := renameRepositoryRollbackData{PreviousConfig: previousConfig}
	if err := rm.updateLifecycleOperationPhase(ctx, operation.OperationID, lifecyclePhaseFilesystemApplied, rollback); err != nil {
		_ = previousConfig.SaveConfigToFile(repository.Path)
		_ = rm.failLifecycleOperation(ctx, operation.OperationID, true, err, rollback)
		return nil, fmt.Errorf("record renamed repository marker: %w", err)
	}
	updated, err := rm.queries.UpdateRepository(ctx, repo.UpdateRepositoryParams{
		RepoID: repositoryID, Name: name, Config: updatedConfig,
		DefaultOwnerID: repository.DefaultOwnerID,
		UpdatedAt:      dbtypes.NewTimestamp(time.Now().UTC()),
	})
	if err != nil {
		rollbackErr := previousConfig.SaveConfigToFile(repository.Path)
		rolledBack := rollbackErr == nil
		_ = rm.failLifecycleOperation(ctx, operation.OperationID, rolledBack, err, rollback)
		return nil, fmt.Errorf("commit renamed repository: %w", err)
	}
	if err := rm.completeLifecycleOperation(ctx, operation.OperationID,
		renameRepositoryOperationResult{RepositoryID: id, Name: name}); err != nil {
		return nil, err
	}
	return &updated, nil
}

func (rm *DefaultRepositoryManager) repoAudit(repoPath string) logging.RepositoryAuditLogger {
	if rm.auditProvider == nil {
		return logging.NoopRepositoryAuditLogger()
	}
	return rm.auditProvider.ForPath(repoPath)
}

func (rm *DefaultRepositoryManager) acquireRepositoryMutation(repositoryID uuid.UUID) func() {
	if rm == nil || rm.files == nil || rm.files.AccessCoordinator() == nil {
		return func() {}
	}
	return rm.files.AccessCoordinator().AcquireMutation(repositoryID)
}

func (rm *DefaultRepositoryManager) acquireRepositoryRootRead(rootID uuid.UUID) func() {
	if rm == nil || rm.files == nil || rm.files.AccessCoordinator() == nil {
		return func() {}
	}
	return rm.files.AccessCoordinator().AcquireRootRead(rootID)
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
	if repository.Reachability != dbtypes.RepositoryReachabilityActive {
		return "", fmt.Errorf("%w: %s", ErrRepositoryOffline, repository.Name)
	}
	return repository.Path, nil
}
