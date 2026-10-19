package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"server/internal/db/catalogtx"
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
	if marker, err := (LocalRepositoryIdentityDetector{}).LoadRepository(registeredPath); err == nil && marker.ID == repositoryID {
		return []string{"copy"}
	}
	return []string{"relocate", "copy"}
}

func (e *RepositoryConflictError) Error() string {
	return fmt.Sprintf("repository %s is already registered at %s", e.RepositoryID, e.RegisteredPath)
}

// RelocateRepository points an existing repository at a new on-disk location.
//
// Assets and Locations are untouched by construction: repository node paths
// are relative graph projections, so every consumer resolves the new root
// through RepositoryFS. The catalog path update and observation fence commit
// together; original media bytes are never moved.
func (rm *DefaultRepositoryManager) RelocateRepository(ctx context.Context, id string, newPath string, requests ...LifecycleRequest) (*repo.Repository, error) {
	return rm.relocateRepository(ctx, id, newPath, nil, requests...)
}

func (rm *DefaultRepositoryManager) relocateRepository(ctx context.Context, id string, newPath string, associatedStorageLocationID *uuid.UUID, requests ...LifecycleRequest) (*repo.Repository, error) {
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
	// The role-aware policy on the projected row, before the destination is
	// even looked at; the decision that counts is re-derived under the lease.
	if err := rm.admitReconnect(ctx, *current); err != nil {
		return nil, err
	}
	if current.Path == cleanPath {
		return current, nil
	}
	if err := rm.claimRuntimeStoragePath(ctx, "repository", cleanPath); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRepositoryOwnershipUnavailable, err)
	}

	result, err := rm.validateRepository(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("failed to validate repository: %w", err)
	}
	if !result.Valid {
		return nil, fmt.Errorf("invalid repository at %s: %v", cleanPath, result.Errors)
	}

	config, err := rm.identities().LoadRepository(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load repository configuration: %w", err)
	}
	if config.ID != id {
		return nil, fmt.Errorf("%w: %s holds repository %s, not %s",
			ErrRepositoryIDMismatch, cleanPath, config.ID, id)
	}
	if marker, markerErr := rm.identities().LoadRepository(current.Path); markerErr == nil && marker.ID == id {
		return nil, fmt.Errorf("%w: %s", ErrRepositoryOriginalOnline, current.Path)
	}

	now := time.Now()
	storageLocationID := uuid.Nil
	if associatedStorageLocationID == nil {
		storageLocationID, err = rm.storageLocationIDForPath(ctx, cleanPath)
		if err != nil {
			return nil, err
		}
	} else {
		storageLocationID = *associatedStorageLocationID
	}
	releaseMutation, err := rm.files.AccessCoordinator().AcquireMutationsContext(ctx, []uuid.UUID{repoUUID})
	if err != nil {
		return nil, fmt.Errorf("%w: repository is busy: %v", ErrRepositoryBusy, err)
	}
	defer releaseMutation()
	current, err = rm.GetRepository(id)
	if err != nil {
		return nil, err
	}
	oldMarker, oldMarkerErr := rm.identities().LoadRepository(current.Path)
	if oldMarkerErr == nil && oldMarker.ID == id {
		return nil, fmt.Errorf("%w: %s", ErrRepositoryOriginalOnline, current.Path)
	}
	if err := rm.admitReconnect(ctx, *current); err != nil {
		return nil, err
	}
	if current.Activity != dbtypes.RepositoryActivityIdle {
		if oldMarkerErr == nil && !repositoryReachabilityAllowsStaleActivityDetach(current.Reachability) {
			return nil, fmt.Errorf("%w: repository activity is %s", ErrRepositoryBusy, current.Activity)
		}
		if _, err := rm.queries.FinishRepositoryActivity(ctx, repo.FinishRepositoryActivityParams{
			RepoID: repoUUID, Activity: current.Activity, UpdatedAt: dbtypes.NewTimestamp(time.Now().UTC()),
		}); err != nil {
			return nil, fmt.Errorf("clear stale repository activity before relocate: %w", err)
		}
		current, err = rm.GetRepository(id)
		if err != nil {
			return nil, err
		}
		if current.Activity != dbtypes.RepositoryActivityIdle {
			return nil, fmt.Errorf("%w: repository activity is %s", ErrRepositoryBusy, current.Activity)
		}
	}
	tx, err := rm.writer.BeginTx(ctx, catalogtx.OperationRepositoryRelocate, nil)
	if err != nil {
		return nil, fmt.Errorf("begin repository relocation: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	queries := rm.queries.WithTx(tx.Raw())
	dbRepo, err := queries.UpdateRepositoryPath(ctx, repo.UpdateRepositoryPathParams{
		RepoID:            repoUUID,
		Path:              cleanPath,
		StorageLocationID: storageLocationID,
		Reachability:      dbtypes.RepositoryReachabilityActive,
		UpdatedAt:         dbtypes.NewTimestamp(now),
	})
	if err != nil {
		if isUniquePathViolation(err) {
			rm.repoAudit(cleanPath).Error("repository.relocate", err, zap.String("repository_id", id))
			return nil, fmt.Errorf("%w: %s", ErrRepositoryExistsAtPath, cleanPath)
		}
		rm.repoAudit(cleanPath).Error("repository.relocate", err, zap.String("repository_id", id))
		return nil, fmt.Errorf("failed to relocate repository: %w", err)
	}
	nowStamp := dbtypes.NewTimestamp(now)
	if err := invalidateRepositoryObservationAfterRelocation(ctx, tx, repoUUID, cleanPath, nowStamp); err != nil {
		return nil, err
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

// admitReconnect applies the role-aware policy to a reconnect. The catalog
// may still project an original as active after it went away, so the marker
// probe, not the projection, decides whether there is something to
// reconnect: an original that is positively online is ErrRepositoryOriginalOnline,
// any other withheld verdict is the typed refusal with its blockers.
func (rm *DefaultRepositoryManager) admitReconnect(ctx context.Context, current repo.Repository) error {
	facts, err := rm.repositoryActionFacts(ctx, current)
	if err != nil {
		return err
	}
	oldMarker, oldMarkerErr := rm.identities().LoadRepository(current.Path)
	originalOnline := oldMarkerErr == nil && oldMarker.ID == current.RepoID.String()
	if facts.Reachability == dbtypes.RepositoryReachabilityActive && !originalOnline {
		facts.Reachability = dbtypes.RepositoryReachabilityOffline
	}
	verdict := DeriveRepositoryActions(facts).Verdict(ActionReconnect)
	if verdict.Allowed() {
		return nil
	}
	if len(verdict.Blockers) == 1 && verdict.Blockers[0] == BlockerOriginalAvailable {
		return fmt.Errorf("%w: %s", ErrRepositoryOriginalOnline, current.Path)
	}
	return &RepositoryActionUnavailableError{
		RepositoryID: current.RepoID.String(), Action: ActionReconnect,
		Blockers: verdict.Blockers, Alternatives: verdict.Alternatives,
	}
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

	config, err := rm.identities().LoadRepository(cleanPath)
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
	storageLocationID, err := rm.storageLocationIDForPath(ctx, cleanPath)
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
			Path: cleanPath, StorageLocationID: storageLocationID.String(), OwnerID: defaultOwnerID, Role: normalizeRepoRole(role), RiskConfirmation: request.RiskConfirmation,
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
	releaseStorageLocation := rm.acquireStorageLocationRead(storageLocationID)
	releaseMutation := rm.acquireRepositoryMutation(previousUUID)
	locksReleased := false
	releaseLocks := func() {
		if locksReleased {
			return
		}
		locksReleased = true
		releaseMutation()
		releaseStorageLocation()
	}
	defer releaseLocks()
	plan, err := planPrivatePreservation(cleanPath, RecoveryEventRegisterCopy, operation.Actor, operation.RequestID, previousID, newID, time.Now())
	if err != nil {
		_ = rm.failLifecycleOperation(ctx, operation.OperationID, true, err,
			registerRepositoryCopyRollbackData{PreviousRepositoryID: previousID})
		return nil, err
	}
	rollbackData := registerRepositoryCopyRollbackData{PreviousRepositoryID: previousID, Preservation: plan}
	if err := rm.updateLifecycleOperationPhase(ctx, operation.OperationID, lifecyclePhasePrepared, rollbackData); err != nil {
		_ = rm.failLifecycleOperation(ctx, operation.OperationID, true, err, rollbackData)
		return nil, fmt.Errorf("persist repository-copy rollback plan: %w", err)
	}
	rollback := func(cause error) error {
		config.ID = previousID
		identityErr := config.SaveConfigToFile(cleanPath)
		stateErr := rollbackPrivateState(cleanPath, plan)
		rollbackErr := errors.Join(identityErr, stateErr)
		_ = rm.failLifecycleOperation(ctx, operation.OperationID, rollbackErr == nil, cause, rollbackData)
		if rollbackErr != nil {
			return errors.Join(cause, fmt.Errorf("rollback repository copy registration: %w", rollbackErr))
		}
		return cause
	}
	if err := applyPrivatePreservation(cleanPath, plan); err != nil {
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

	dbRepo, err := rm.AddRepository(cleanPath, defaultOwnerID, role, storageLocationID)
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
	// The observation request establishes a repository work lease of its own.
	// Release the lifecycle locks before entering it, otherwise the request
	// waits forever for the locks held by this function.
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
		zap.String("recovery_receipt_id", plan.ReceiptID()),
	)

	return dbRepo, nil
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
