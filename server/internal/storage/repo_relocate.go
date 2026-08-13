package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/storage/repocfg"

	"github.com/google/uuid"
	"github.com/mattn/go-sqlite3"
	"go.uber.org/zap"
)

var (
	// ErrRepositoryOffline reports that a repository's on-disk location is not
	// currently reachable — an unplugged external drive is the ordinary case.
	// Callers must distinguish this from "the data is gone".
	ErrRepositoryOffline = errors.New("repository is offline")

	// ErrRepositoryIDMismatch reports that the .lumiliorepo at a path belongs to
	// a different repository than the one being relocated.
	ErrRepositoryIDMismatch     = errors.New("repository ID at path does not match")
	ErrRepositoryOriginalOnline = errors.New("registered repository is still online at its original path")

	// ErrRepositoryBusy reports that a lifecycle operation could not acquire its
	// root/repository barriers before the caller's context expired.
	ErrRepositoryBusy = errors.New("repository resource is busy")
)

// RepositoryConflictError reports that the repository being registered carries
// an ID that is already registered at another path.
//
// This is not an error condition the server can resolve on its own. The obvious
// automatic check — "is the old path still a valid repository with this ID?" —
// gives the wrong answer in the ordinary external-drive sequence: the drive is
// unplugged, the user registers a copy, and later the original drive returns.
// Both locations are then real, and only the user knows which one is the
// library. The caller must present the choice: relocate the existing
// repository, or register this path as a new, independent copy.
type RepositoryConflictError struct {
	RepositoryID   string
	RegisteredPath string
	RequestedPath  string
	Actions        []string
}

func repositoryConflictActions(registeredPath, repositoryID string) []string {
	if marker, err := repocfg.LoadConfigFromFile(registeredPath); err == nil && marker.ID == repositoryID {
		return []string{"copy"}
	}
	return []string{"relocate", "copy"}
}

func (e *RepositoryConflictError) Error() string {
	return fmt.Sprintf("repository %s is already registered at %s", e.RepositoryID, e.RegisteredPath)
}

// RelocateRepository points an existing repository at a new on-disk location.
//
// Assets are untouched by construction: assets.storage_path is
// repository-relative (UNIQUE (repository_id, storage_path)), so every consumer
// re-derives absolute paths from repositories.path. Relocate is one UPDATE.
func (rm *DefaultRepositoryManager) RelocateRepository(ctx context.Context, id string, newPath string, requests ...LifecycleRequest) (*repo.Repository, error) {
	return rm.relocateRepository(ctx, id, newPath, nil, requests...)
}

func (rm *DefaultRepositoryManager) relocateRepository(ctx context.Context, id string, newPath string, associatedRootID *uuid.UUID, requests ...LifecycleRequest) (*repo.Repository, error) {
	repoUUID, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("invalid repository ID: %w", err)
	}
	cleanPath, err := CanonicalizeRepositoryPath(newPath)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}

	current, err := rm.GetRepository(id)
	if err != nil {
		return nil, err
	}
	if current.Role == dbtypes.RepoRolePrimary {
		return nil, fmt.Errorf("%w: primary repository location follows the default Storage Location", ErrPathNotAllowed)
	}
	if current.Path == cleanPath {
		return current, nil
	}
	if err := rm.claimRuntimeStoragePath(ctx, "repository", cleanPath); err != nil {
		return nil, err
	}
	if current.Activity != dbtypes.RepositoryActivityIdle {
		return nil, fmt.Errorf("%w: repository activity is %s", ErrRepositoryBusy, current.Activity)
	}

	result, err := rm.validateRepository(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("failed to validate repository: %w", err)
	}
	if !result.Valid {
		return nil, fmt.Errorf("invalid repository at %s: %v", cleanPath, result.Errors)
	}

	config, err := repocfg.LoadConfigFromFile(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load repository configuration: %w", err)
	}
	if config.ID != id {
		return nil, fmt.Errorf("%w: %s holds repository %s, not %s",
			ErrRepositoryIDMismatch, cleanPath, config.ID, id)
	}
	if marker, markerErr := repocfg.LoadConfigFromFile(current.Path); markerErr == nil && marker.ID == id {
		return nil, fmt.Errorf("%w: %s", ErrRepositoryOriginalOnline, current.Path)
	}

	now := time.Now()
	rootID := uuid.Nil
	if associatedRootID == nil {
		rootID, err = rm.repositoryRootIDForPath(ctx, cleanPath)
		if err != nil {
			return nil, err
		}
	} else {
		rootID = *associatedRootID
	}
	releaseRoot, err := rm.files.AccessCoordinator().AcquireRootReadContext(ctx, rootID)
	if err != nil {
		return nil, fmt.Errorf("%w: Storage Location is busy: %v", ErrRepositoryBusy, err)
	}
	defer releaseRoot()
	releaseMutation, err := rm.files.AccessCoordinator().AcquireMutationsContext(ctx, []uuid.UUID{repoUUID})
	if err != nil {
		return nil, fmt.Errorf("%w: repository is busy: %v", ErrRepositoryBusy, err)
	}
	defer releaseMutation()
	tx, err := rm.database.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin repository relocation: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	queries := rm.queries.WithTx(tx)
	dbRepo, err := queries.UpdateRepositoryPath(ctx, repo.UpdateRepositoryPathParams{
		RepoID:       repoUUID,
		Path:         cleanPath,
		RootID:       rootID,
		Reachability: dbtypes.RepositoryReachabilityActive,
		UpdatedAt:    dbtypes.NewTimestamp(now),
	})
	if err != nil {
		if isUniquePathViolation(err) {
			rm.repoAudit(cleanPath).Error("repository.relocate", err, zap.String("repository_id", id))
			return nil, fmt.Errorf("%w: %s", ErrRepositoryExistsAtPath, cleanPath)
		}
		rm.repoAudit(cleanPath).Error("repository.relocate", err, zap.String("repository_id", id))
		return nil, fmt.Errorf("failed to relocate repository: %w", err)
	}
	request := LifecycleRequest{}
	if len(requests) > 0 {
		request = requests[0]
	}
	confirmation := strings.TrimSpace(request.ConfirmationType)
	if confirmation == "" {
		confirmation = "update_location"
	}
	if _, err := recordLifecycleAuditWithQueries(ctx, queries, LifecycleAuditInput{
		Actor: request.Actor, ActorUserID: request.ActorUserID, HostInstanceID: request.HostInstanceID,
		RequestID: request.RequestID, Action: "update_repository_location", TargetType: "repository",
		TargetID: id, Source: auditSourceForActor(request.Actor), ConfirmationType: confirmation,
		OldPath: current.Path, NewPath: cleanPath, Result: AuditResultSucceeded,
		Details: map[string]any{"risk_confirmation": request.RiskConfirmation},
	}); err != nil {
		return nil, fmt.Errorf("audit repository relocation: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit repository relocation: %w", err)
	}

	// The path is now correct and durable. Refreshing the DB's cached copy of
	// the on-disk config is a separate, recoverable step: if it fails the
	// repository is still correctly located and the next boot reconcile will
	// bring the cache forward.
	if refreshed, err := rm.refreshRepositoryConfigCache(ctx, dbRepo, config); err != nil {
		rm.logger.Warn("relocated repository but failed to refresh config cache",
			zap.String("operation", "repository.relocate"),
			zap.String("repository_id", id),
			zap.Error(err))
	} else {
		dbRepo = *refreshed
	}

	rm.repoAudit(cleanPath).Operation("repository.relocate",
		zap.String("repository_id", id),
		zap.String("repository_path", cleanPath),
	)
	rm.logger.Info("repository relocated",
		zap.String("operation", "repository.relocate"),
		zap.String("repository_id", id),
		zap.String("repository_path", cleanPath),
	)

	return &dbRepo, nil
}

// RegisterRepositoryCopy registers a duplicated repository directory as a new,
// independent repository by minting a fresh UUID into its .lumiliorepo. This is
// the `git clone` answer to a same-ID conflict, and it turns a dead-end error
// into an action the user can take.
func (rm *DefaultRepositoryManager) RegisterRepositoryCopy(ctx context.Context, path string, defaultOwnerID *int32, role dbtypes.RepoRole, requests ...LifecycleRequest) (*repo.Repository, error) {
	if normalizeRepoRole(role) == dbtypes.RepoRolePrimary {
		return nil, fmt.Errorf("%w: primary repository cannot be registered as a copy", ErrPathNotAllowed)
	}
	cleanPath, err := CanonicalizeRepositoryPath(path)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}

	config, err := repocfg.LoadConfigFromFile(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load repository configuration: %w", err)
	}

	previousID := config.ID
	previousUUID, err := uuid.Parse(previousID)
	if err != nil {
		return nil, fmt.Errorf("invalid copied repository identity: %w", err)
	}
	if registered, lookupErr := rm.GetRepository(previousID); lookupErr == nil && registered.Role == dbtypes.RepoRolePrimary {
		return nil, fmt.Errorf("%w: a copy of the primary repository cannot be registered as regular", ErrPathNotAllowed)
	} else if lookupErr != nil && !errors.Is(lookupErr, sql.ErrNoRows) {
		return nil, fmt.Errorf("check copied repository role: %w", lookupErr)
	}
	rootID, err := rm.repositoryRootIDForPath(ctx, cleanPath)
	if err != nil {
		return nil, err
	}
	request := LifecycleRequest{}
	if len(requests) > 0 {
		request = requests[0]
	}
	newID := uuid.NewString()
	operation, replay, err := rm.beginLifecycleOperation(ctx, lifecycleBeginInput{
		RequestID: request.RequestID, Kind: lifecycleKindRegisterRepositoryCopy,
		Payload: registerRepositoryCopyOperationPayload{
			Path: cleanPath, RootID: rootID.String(), OwnerID: defaultOwnerID, Role: normalizeRepoRole(role), RiskConfirmation: request.RiskConfirmation,
		},
		Actor: request.Actor, ActorUserID: request.ActorUserID, HostInstanceID: request.HostInstanceID, TargetType: "repository", TargetID: &newID,
		RollbackData: registerRepositoryCopyRollbackData{PreviousRepositoryID: previousID},
	})
	if err != nil {
		return nil, err
	}
	if replay {
		if err := lifecycleReplayError(operation); err != nil {
			return nil, err
		}
		if operation.TargetID == nil {
			return nil, fmt.Errorf("%w: completed register-copy operation has no target", ErrLifecycleRecoveryRequired)
		}
		return rm.GetRepository(*operation.TargetID)
	}
	newID = *operation.TargetID
	releaseRoot := rm.acquireRepositoryRootRead(rootID)
	releaseMutation := rm.acquireRepositoryMutation(previousUUID)
	locksReleased := false
	releaseLocks := func() {
		if locksReleased {
			return
		}
		locksReleased = true
		releaseMutation()
		releaseRoot()
	}
	defer releaseLocks()
	rollbackData, err := planRepositoryPrivateStateIsolation(cleanPath, "copied-from-"+previousID)
	if err != nil {
		_ = rm.failLifecycleOperation(ctx, operation.OperationID, true, err,
			registerRepositoryCopyRollbackData{PreviousRepositoryID: previousID})
		return nil, err
	}
	rollbackData.PreviousRepositoryID = previousID
	if err := rm.updateLifecycleOperationPhase(ctx, operation.OperationID, lifecyclePhasePrepared, rollbackData); err != nil {
		_ = rm.failLifecycleOperation(ctx, operation.OperationID, true, err, rollbackData)
		return nil, fmt.Errorf("persist repository-copy rollback plan: %w", err)
	}
	rollback := func(cause error) error {
		config.ID = previousID
		identityErr := config.SaveConfigToFile(cleanPath)
		stateErr := rollbackRepositoryPrivateStateIsolation(cleanPath, rollbackData)
		rollbackErr := errors.Join(identityErr, stateErr)
		_ = rm.failLifecycleOperation(ctx, operation.OperationID, rollbackErr == nil, cause, rollbackData)
		if rollbackErr != nil {
			return errors.Join(cause, fmt.Errorf("rollback repository copy registration: %w", rollbackErr))
		}
		return cause
	}
	if err := applyRepositoryPrivateStateIsolation(cleanPath, rollbackData); err != nil {
		return nil, rollback(err)
	}
	config.ID = newID
	if err := config.SaveConfigToFile(cleanPath); err != nil {
		return nil, rollback(fmt.Errorf("failed to write new repository identity: %w", err))
	}
	if err := rm.dirManager.CreateStructure(cleanPath); err != nil {
		return nil, rollback(fmt.Errorf("create fresh repository private state: %w", err))
	}
	if err := rm.updateLifecycleOperationPhase(ctx, operation.OperationID, lifecyclePhaseFilesystemApplied, rollbackData); err != nil {
		return nil, rollback(fmt.Errorf("persist register-copy filesystem phase: %w", err))
	}

	dbRepo, err := rm.AddRepository(cleanPath, defaultOwnerID, role, rootID)
	if err != nil {
		return nil, rollback(err)
	}
	if err := rm.updateLifecycleOperationPhase(ctx, operation.OperationID, lifecyclePhaseCatalogCommitted, rollbackData); err != nil {
		return nil, fmt.Errorf("repository copy registered but journal commit phase failed: %w", err)
	}
	if err := rm.completeLifecycleOperation(ctx, operation.OperationID,
		registerRepositoryCopyOperationResult{RepositoryID: dbRepo.RepoID.String(), InitialScanQueued: false}); err != nil {
		return nil, fmt.Errorf("repository copy registered but journal completion failed: %w", err)
	}
	// The scanner's enqueue path establishes a repository work lease of its
	// own. Release the lifecycle locks before entering it, otherwise the
	// enqueue path waits forever for the locks held by this function.
	releaseLocks()
	if rm.initialScan != nil {
		if err := rm.ScheduleInitialRepositoryScan(ctx, dbRepo.RepoID.String()); err != nil {
			rm.logger.Warn("repository copy registered but initial scan could not be queued",
				zap.String("operation", "repository.register_copy.scan"),
				zap.String("repository_id", dbRepo.RepoID.String()),
				zap.Error(err))
		} else if err := rm.markInitialScanQueued(ctx, operation.OperationID); err != nil {
			return nil, err
		}
	}

	rm.repoAudit(cleanPath).Operation("repository.register_copy",
		zap.String("repository_id", config.ID),
		zap.String("copied_from_repository_id", previousID),
	)

	return dbRepo, nil
}

func planRepositoryPrivateStateIsolation(repositoryPath, recoveryPrefix string) (registerRepositoryCopyRollbackData, error) {
	privateRoot := filepath.Join(repositoryPath, DefaultStructure.SystemDir)
	entries, err := os.ReadDir(privateRoot)
	if errors.Is(err, os.ErrNotExist) {
		return registerRepositoryCopyRollbackData{PrivateRootExisted: false}, nil
	}
	if err != nil {
		return registerRepositoryCopyRollbackData{}, fmt.Errorf("read repository private state: %w", err)
	}
	recoveryPath := filepath.Join(
		privateRoot,
		"recovery",
		recoveryPrefix+"-"+time.Now().UTC().Format("20060102T150405.000000000Z")+"-"+uuid.NewString()[:8],
	)
	rollback := registerRepositoryCopyRollbackData{RecoveryPath: recoveryPath, PrivateRootExisted: true}
	for _, entry := range entries {
		if entry.Name() != "recovery" {
			rollback.MovedEntries = append(rollback.MovedEntries, entry.Name())
		}
	}
	return rollback, nil
}

func applyRepositoryPrivateStateIsolation(repositoryPath string, rollback registerRepositoryCopyRollbackData) error {
	if !rollback.PrivateRootExisted || strings.TrimSpace(rollback.RecoveryPath) == "" {
		return nil
	}
	privateRoot := filepath.Join(repositoryPath, DefaultStructure.SystemDir)
	recoveryPath := filepath.Clean(rollback.RecoveryPath)
	if err := os.MkdirAll(recoveryPath, 0o755); err != nil {
		return fmt.Errorf("create repository copy recovery directory: %w", err)
	}
	for _, name := range rollback.MovedEntries {
		if filepath.Base(name) != name || name == "." || name == ".." {
			return fmt.Errorf("invalid private-state entry %q", name)
		}
		if err := os.Rename(filepath.Join(privateRoot, name), filepath.Join(recoveryPath, name)); err != nil {
			return fmt.Errorf("isolate repository private state %s: %w", name, err)
		}
	}
	return nil
}

func rollbackRepositoryPrivateStateIsolation(repositoryPath string, rollback registerRepositoryCopyRollbackData) error {
	privateRoot := filepath.Join(repositoryPath, DefaultStructure.SystemDir)
	if !rollback.PrivateRootExisted && strings.TrimSpace(rollback.RecoveryPath) == "" {
		if err := os.RemoveAll(privateRoot); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove fresh repository private state: %w", err)
		}
		return nil
	}
	if strings.TrimSpace(rollback.RecoveryPath) == "" {
		return nil
	}
	cleanRecovery := filepath.Clean(rollback.RecoveryPath)
	recoveryRoot := filepath.Join(privateRoot, "recovery") + string(os.PathSeparator)
	if !strings.HasPrefix(cleanRecovery+string(os.PathSeparator), recoveryRoot) {
		return fmt.Errorf("recovery path escapes repository private state")
	}
	var rollbackErrors []error
	planned := make(map[string]struct{}, len(rollback.MovedEntries))
	for _, name := range rollback.MovedEntries {
		planned[name] = struct{}{}
	}
	entries, readErr := os.ReadDir(privateRoot)
	if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
		rollbackErrors = append(rollbackErrors, readErr)
	}
	for _, entry := range entries {
		if entry.Name() == "recovery" {
			continue
		}
		if _, wasPlanned := planned[entry.Name()]; wasPlanned {
			if _, sourceErr := os.Stat(filepath.Join(cleanRecovery, entry.Name())); errors.Is(sourceErr, os.ErrNotExist) {
				// A crash may have happened before this planned entry moved. Keep
				// the original in place instead of mistaking it for fresh state.
				continue
			}
		}
		if err := os.RemoveAll(filepath.Join(privateRoot, entry.Name())); err != nil {
			rollbackErrors = append(rollbackErrors, fmt.Errorf("remove fresh private-state entry %s: %w", entry.Name(), err))
		}
	}
	for _, name := range rollback.MovedEntries {
		if filepath.Base(name) != name || name == "." || name == ".." {
			rollbackErrors = append(rollbackErrors, fmt.Errorf("invalid private-state entry %q", name))
			continue
		}
		source := filepath.Join(cleanRecovery, name)
		destination := filepath.Join(privateRoot, name)
		if _, err := os.Stat(source); errors.Is(err, os.ErrNotExist) {
			if _, destinationErr := os.Stat(destination); destinationErr == nil {
				continue
			}
			rollbackErrors = append(rollbackErrors, fmt.Errorf("private-state entry %s is missing from both locations", name))
			continue
		} else if err != nil {
			rollbackErrors = append(rollbackErrors, err)
			continue
		}
		if err := os.Rename(source, destination); err != nil {
			rollbackErrors = append(rollbackErrors, err)
		}
	}
	if len(rollbackErrors) == 0 {
		_ = os.Remove(cleanRecovery)
	}
	return errors.Join(rollbackErrors...)
}

// refreshRepositoryConfigCache writes the on-disk config into the database's
// cached copy. Disk is authoritative; the DB column is a cache.
func (rm *DefaultRepositoryManager) refreshRepositoryConfigCache(ctx context.Context, current repo.Repository, config *repocfg.RepositoryConfig) (*repo.Repository, error) {
	updated, err := rm.queries.UpdateRepository(ctx, repo.UpdateRepositoryParams{
		RepoID:         current.RepoID,
		Name:           config.Name,
		Config:         *config,
		DefaultOwnerID: current.DefaultOwnerID,
		UpdatedAt:      dbtypes.NewTimestamp(time.Now()),
	})
	if err != nil {
		return nil, err
	}
	return &updated, nil
}

func isUniquePathViolation(err error) bool {
	var sqliteErr sqlite3.Error
	if !errors.As(err, &sqliteErr) {
		return false
	}
	return sqliteErr.ExtendedCode == sqlite3.ErrConstraintUnique ||
		sqliteErr.ExtendedCode == sqlite3.ErrConstraintPrimaryKey
}
