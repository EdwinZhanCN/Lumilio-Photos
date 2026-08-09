package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
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
	ErrPrimaryRepositoryExists               = errors.New("primary repository already exists")
	ErrPrimaryRepositoryRequired             = errors.New("primary repository must be created first")
	ErrRepositoryExistsAtPath                = errors.New("repository already exists at path")
	ErrInvalidRepositoryName                 = errors.New("invalid repository name")
	ErrInvalidRepositoryDirectory            = errors.New("invalid repository storage folder")
	ErrRepositoryDirectoryConflict           = errors.New("repository storage folder conflicts with an existing directory")
	ErrRepositoryTargetNotEmpty              = errors.New("repository target directory is not empty")
	ErrRepositoryStorageNotWritable          = errors.New("repository storage is not writable")
	ErrRepositoryExistingTargetNotMountPoint = errors.New("existing empty repository target is not a Linux mount point")
	ErrRepositoryRiskConfirmationRequired    = errors.New("storage placement risk confirmation is required")
)

const (
	maxRepositoryNameRunes = 80
	maxRepositoryNameBytes = 240
)

// CreateRepositorySpec describes a repository to create. StorageStrategy and
// DuplicateHandling are optional; empty values fall back to the storage-owned
// repository defaults.
type CreateRepositorySpec struct {
	RequestID         string
	Actor             string
	ActorUserID       *int32
	HostInstanceID    string
	Name              string
	DirectoryName     string
	Role              dbtypes.RepoRole
	Root              string
	RootID            string
	OwnerID           *int32
	StorageStrategy   string
	DuplicateHandling string
	RiskConfirmation  bool
}

// ExistingRepositoryFoundError reports that Create derived a target carrying
// a valid, unregistered .lumiliorepo. Create never opens it implicitly; the
// caller must hand the user to the explicit Open Existing Repository task.
type ExistingRepositoryFoundError struct {
	RepositoryID  string
	RequestedPath string
}

func (e *ExistingRepositoryFoundError) Error() string {
	return fmt.Sprintf("existing repository %s found at %s", e.RepositoryID, e.RequestedPath)
}

// RepositoryMarkerInvalidError reports a Create target with a .lumiliorepo
// that cannot be parsed or validated. It is a recovery candidate, not an empty
// directory that Create may overwrite.
type RepositoryMarkerInvalidError struct {
	RequestedPath string
	Cause         error
}

func (e *RepositoryMarkerInvalidError) Error() string {
	return fmt.Sprintf("invalid repository marker at %s: %v", e.RequestedPath, e.Cause)
}

func (e *RepositoryMarkerInvalidError) Unwrap() error { return e.Cause }

// CreateRepositoryResult carries the created repository plus any non-fatal
// warnings the path policy raised about its location.
type CreateRepositoryResult struct {
	Repository *repo.Repository
	Warnings   []string
}

// CreateRepository is the single entry point for creating a repository: it
// enforces the primary-first / single-primary policy, resolves a direct-child
// path inside Root, applies repository defaults, and initializes a new
// repository. Existing markers are returned as structured recovery facts and
// are never registered implicitly.
func (rm *DefaultRepositoryManager) CreateRepository(ctx context.Context, spec CreateRepositorySpec) (*CreateRepositoryResult, error) {
	if err := ValidateRepositoryName(spec.Name); err != nil {
		return nil, err
	}

	role := normalizeRepoRole(spec.Role)
	if role != dbtypes.RepoRolePrimary {
		if err := ValidateRepositoryDirectoryName(spec.DirectoryName); err != nil {
			return nil, err
		}
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

	repoPath, err := resolveRepositoryCreatePath(spec.Root, spec.DirectoryName, role)
	if err != nil {
		return nil, err
	}
	warnings := StoragePlacementWarnings(spec.Root)
	if len(warnings) > 0 && !spec.RiskConfirmation && !strings.HasPrefix(spec.Actor, "server:") {
		if _, auditErr := rm.RecordLifecycleAudit(ctx, LifecycleAuditInput{
			Actor: spec.Actor, ActorUserID: spec.ActorUserID, HostInstanceID: spec.HostInstanceID,
			RequestID: spec.RequestID, Action: lifecycleKindCreateRepository, TargetType: "repository",
			Source: auditSourceForActor(spec.Actor), ConfirmationType: "none", NewPath: repoPath,
			Result: AuditResultRejected, FailureStage: "risk_confirmation",
			Details: map[string]any{"risk_confirmation": false, "risk_warnings": warnings},
		}); auditErr != nil {
			return nil, fmt.Errorf("%w: persist rejected storage risk decision: %v", ErrRepositoryRiskConfirmationRequired, auditErr)
		}
		return nil, fmt.Errorf("%w: %s", ErrRepositoryRiskConfirmationRequired, strings.Join(warnings, ", "))
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
	associatedRootID, err := rm.resolveRepositoryAssociation(ctx, repoPath, rootIDs)
	if err != nil {
		return nil, err
	}
	targetID := cfg.ID
	operation, replay, err := rm.beginLifecycleOperation(ctx, lifecycleBeginInput{
		RequestID: spec.RequestID,
		Kind:      lifecycleKindCreateRepository,
		Payload: createRepositoryOperationPayload{
			Name: spec.Name, Path: repoPath, RootID: associatedRootID.String(), Role: role,
			OwnerID: spec.OwnerID, StorageStrategy: cfg.StorageStrategy,
			DuplicateHandling: cfg.LocalSettings.HandleDuplicateFilenames,
			RiskConfirmation:  spec.RiskConfirmation,
		},
		Actor:          spec.Actor,
		ActorUserID:    spec.ActorUserID,
		HostInstanceID: spec.HostInstanceID,
		TargetType:     "repository",
		TargetID:       &targetID,
		RollbackData:   createRepositoryRollbackData{Path: repoPath},
	})
	if err != nil {
		return nil, err
	}
	if replay {
		if err := lifecycleReplayError(operation); err != nil {
			return nil, err
		}
		if operation.Result == nil {
			return nil, fmt.Errorf("%w: completed create operation has no result", ErrLifecycleRecoveryRequired)
		}
		var result createRepositoryOperationResult
		if err := json.Unmarshal(*operation.Result, &result); err != nil {
			return nil, fmt.Errorf("decode completed create result: %w", err)
		}
		dbRepo, err := rm.GetRepository(result.RepositoryID)
		if err != nil {
			return nil, fmt.Errorf("load completed repository create result: %w", err)
		}
		return &CreateRepositoryResult{Repository: dbRepo, Warnings: warnings}, nil
	}
	failPrepared := func(cause error) (*CreateRepositoryResult, error) {
		_ = rm.failLifecycleOperation(ctx, operation.OperationID, true, cause, createRepositoryRollbackData{Path: repoPath})
		return nil, cause
	}

	primaryExists, err := rm.primaryRepositoryExists(ctx)
	if err != nil {
		return failPrepared(err)
	}
	if role == dbtypes.RepoRolePrimary && primaryExists {
		return failPrepared(ErrPrimaryRepositoryExists)
	}
	if role != dbtypes.RepoRolePrimary && !primaryExists {
		return failPrepared(ErrPrimaryRepositoryRequired)
	}

	if existing, lookupErr := rm.GetRepositoryByPath(repoPath); lookupErr == nil && existing != nil {
		return failPrepared(fmt.Errorf("%w: %s", ErrRepositoryExistsAtPath, repoPath))
	} else if lookupErr != nil && !errors.Is(lookupErr, sql.ErrNoRows) {
		return failPrepared(fmt.Errorf("look up repository create target: %w", lookupErr))
	}

	if repocfg.IsRepositoryRoot(repoPath) {
		config, loadErr := repocfg.LoadConfigFromFile(repoPath)
		if loadErr != nil {
			return failPrepared(&RepositoryMarkerInvalidError{RequestedPath: repoPath, Cause: loadErr})
		}
		if registered, lookupErr := rm.GetRepository(config.ID); lookupErr == nil {
			return failPrepared(&RepositoryConflictError{
				RepositoryID:   config.ID,
				RegisteredPath: registered.Path,
				RequestedPath:  repoPath,
				Actions:        repositoryConflictActions(registered.Path, config.ID),
			})
		} else if !errors.Is(lookupErr, sql.ErrNoRows) {
			return failPrepared(fmt.Errorf("look up existing repository marker: %w", lookupErr))
		}
		return failPrepared(&ExistingRepositoryFoundError{RepositoryID: config.ID, RequestedPath: repoPath})
	}
	if runtime.GOOS == "linux" {
		existingEmpty, inspectErr := existingEmptyRepositoryTarget(repoPath)
		if inspectErr != nil {
			return failPrepared(fmt.Errorf("%w: inspect existing target: %v", ErrRepositoryStorageNotWritable, inspectErr))
		}
		if existingEmpty {
			mounted, mountErr := isLinuxMountPoint(repoPath)
			if mountErr != nil {
				return failPrepared(fmt.Errorf("%w: %v", ErrRepositoryExistingTargetNotMountPoint, mountErr))
			}
			if !mounted {
				return failPrepared(ErrRepositoryExistingTargetNotMountPoint)
			}
		}
	}

	dbRepo, err := rm.initializeRepository(ctx, repoPath, *cfg, spec.OwnerID, role, &operation, associatedRootID)
	if err != nil {
		if current, loadErr := rm.queries.GetLifecycleOperation(ctx, operation.OperationID); loadErr == nil && current.Phase == lifecyclePhasePrepared {
			_ = rm.failLifecycleOperation(ctx, operation.OperationID, true, err, createRepositoryRollbackData{Path: repoPath})
		}
		return nil, err
	}
	return &CreateRepositoryResult{Repository: dbRepo, Warnings: warnings}, nil
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

// EnsurePrimaryRepository idempotently ensures a primary repository exists at
// <root>/primary, returning the existing one if already present.
func (rm *DefaultRepositoryManager) EnsurePrimaryRepository(ctx context.Context, root string, ownerID *int32) (*repo.Repository, error) {
	if existing, err := rm.queries.GetPrimaryRepositoryRecord(ctx); err == nil {
		return &existing, nil
	}
	result, err := rm.CreateRepository(ctx, CreateRepositorySpec{
		RequestID:     "bootstrap-primary:" + strings.TrimSpace(root),
		Actor:         "server:bootstrap",
		Name:          "Primary Storage",
		DirectoryName: "primary",
		Role:          dbtypes.RepoRolePrimary,
		Root:          root,
		OwnerID:       ownerID,
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
// repository uses the explicit stable storage-folder segment independently of
// its mutable display name. The result is a direct child of root.
func resolveRepositoryCreatePath(root, directoryName string, role dbtypes.RepoRole) (string, error) {
	trimmed := strings.TrimSpace(root)
	if trimmed == "" {
		return "", errors.New("storage root is not configured")
	}
	cleanRoot, err := CanonicalizeRepositoryPath(trimmed)
	if err != nil {
		return "", fmt.Errorf("invalid storage root: %w", err)
	}

	folderName := directoryName
	if role == dbtypes.RepoRolePrimary {
		folderName = "primary"
	} else if err := ValidateRepositoryDirectoryName(directoryName); err != nil {
		return "", err
	}

	repoPath, err := filepath.Abs(filepath.Join(cleanRoot, folderName))
	if err != nil {
		return "", fmt.Errorf("invalid repository path: %w", err)
	}
	if !pathIsDirectChild(cleanRoot, repoPath) {
		return "", errors.New("repository path must be a direct child of storage root")
	}
	if err := rejectCaseInsensitiveRepositoryDirectoryConflict(cleanRoot, folderName); err != nil {
		return "", err
	}
	return repoPath, nil
}

// ValidateRepositoryName validates the mutable display name. Filesystem-safe
// segment rules belong to ValidateRepositoryDirectoryName instead.
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
		if unicode.IsControl(r) {
			return fmt.Errorf("%w: control character %q is not allowed", ErrInvalidRepositoryName, r)
		}
	}
	return nil
}

// ValidateRepositoryDirectoryName applies the portable single-segment
// storage-folder contract used by every deployment. Values are never
// lowercased, trimmed, slugified, or repaired behind the user's back.
func ValidateRepositoryDirectoryName(name string) error {
	if err := ValidateRepositoryName(name); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidRepositoryDirectory, err)
	}
	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == ' ' || r == '-' || r == '_' {
			continue
		}
		return fmt.Errorf("%w: character %q is not allowed", ErrInvalidRepositoryDirectory, r)
	}
	return nil
}

func rejectCaseInsensitiveRepositoryDirectoryConflict(root, requestedName string) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		return fmt.Errorf("list storage root: %w", err)
	}
	for _, entry := range entries {
		existingName := entry.Name()
		if existingName != requestedName && strings.EqualFold(existingName, requestedName) {
			return fmt.Errorf("%w: %q already exists as %q", ErrRepositoryDirectoryConflict, requestedName, existingName)
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
