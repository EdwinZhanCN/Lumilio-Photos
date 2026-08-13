package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/storage/repocfg"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// OpenRepository durably registers an existing repository after isolating all
// repository-private state from its prior registration. Original media and the
// .lumiliorepo marker are never moved. The subsequent authoritative scan is the
// only mechanism that rebuilds current catalog-derived state.
func (rm *DefaultRepositoryManager) OpenRepository(
	ctx context.Context,
	path string,
	defaultOwnerID *int32,
	role dbtypes.RepoRole,
	request LifecycleRequest,
) (*repo.Repository, error) {
	if normalizeRepoRole(role) == dbtypes.RepoRolePrimary {
		return nil, fmt.Errorf("%w: an existing repository cannot become the primary repository", ErrPathNotAllowed)
	}
	cleanPath, err := CanonicalizeRepositoryPath(path)
	if err != nil {
		return nil, fmt.Errorf("invalid repository path: %w", err)
	}
	if err := rm.claimRuntimeStoragePath(ctx, "repository", cleanPath); err != nil {
		return nil, err
	}
	validation, err := rm.validateRepository(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("validate repository before opening: %w", err)
	}
	if !validation.Valid {
		return nil, fmt.Errorf("invalid repository at %s: %v", cleanPath, validation.Errors)
	}
	if err := requireMaterializableRepository(cleanPath); err != nil {
		return nil, err
	}
	config, err := repocfg.LoadConfigFromFile(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("load repository marker: %w", err)
	}
	repositoryID, err := uuid.Parse(config.ID)
	if err != nil {
		return nil, fmt.Errorf("invalid repository identity: %w", err)
	}
	rootID, err := rm.repositoryRootIDForPath(ctx, cleanPath)
	if err != nil {
		return nil, err
	}

	operation, replay, err := rm.beginLifecycleOperation(ctx, lifecycleBeginInput{
		RequestID: request.RequestID,
		Kind:      lifecycleKindOpenRepository,
		Payload: openRepositoryOperationPayload{
			Path: cleanPath, RootID: rootID.String(), OwnerID: defaultOwnerID, Role: dbtypes.RepoRoleRegular, RiskConfirmation: request.RiskConfirmation,
		},
		Actor:          request.Actor,
		ActorUserID:    request.ActorUserID,
		HostInstanceID: request.HostInstanceID,
		TargetType:     "repository",
		TargetID:       &config.ID,
		RollbackData:   registerRepositoryCopyRollbackData{},
	})
	if err != nil {
		return nil, err
	}
	if replay {
		if err := lifecycleReplayError(operation); err != nil {
			return nil, err
		}
		if operation.TargetID == nil {
			return nil, fmt.Errorf("%w: completed open operation has no target", ErrLifecycleRecoveryRequired)
		}
		return rm.GetRepository(*operation.TargetID)
	}
	failPrepared := func(cause error) (*repo.Repository, error) {
		_ = rm.failLifecycleOperation(ctx, operation.OperationID, true, cause, registerRepositoryCopyRollbackData{})
		return nil, cause
	}
	if registered, lookupErr := rm.GetRepositoryByPath(cleanPath); lookupErr == nil && registered != nil {
		return failPrepared(fmt.Errorf("%w: %s", ErrRepositoryExistsAtPath, cleanPath))
	} else if lookupErr != nil && !errors.Is(lookupErr, sql.ErrNoRows) {
		return failPrepared(fmt.Errorf("look up repository path: %w", lookupErr))
	}
	if registered, lookupErr := rm.GetRepository(config.ID); lookupErr == nil {
		return failPrepared(&RepositoryConflictError{
			RepositoryID: config.ID, RegisteredPath: registered.Path, RequestedPath: cleanPath,
			Actions: repositoryConflictActions(registered.Path, config.ID),
		})
	} else if lookupErr != nil && !errors.Is(lookupErr, sql.ErrNoRows) {
		return failPrepared(fmt.Errorf("look up repository identity: %w", lookupErr))
	}

	releaseRoot := rm.acquireRepositoryRootRead(rootID)
	releaseMutation := rm.acquireRepositoryMutation(repositoryID)
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

	rollbackData, err := planRepositoryPrivateStateIsolation(cleanPath, "reopened-"+config.ID)
	if err != nil {
		return failPrepared(err)
	}
	if err := rm.updateLifecycleOperationPhase(ctx, operation.OperationID, lifecyclePhasePrepared, rollbackData); err != nil {
		return failPrepared(fmt.Errorf("persist repository-open rollback plan: %w", err))
	}
	rollback := func(cause error) error {
		rollbackErr := rollbackRepositoryPrivateStateIsolation(cleanPath, rollbackData)
		_ = rm.failLifecycleOperation(ctx, operation.OperationID, rollbackErr == nil, cause, rollbackData)
		if rollbackErr != nil {
			return errors.Join(cause, fmt.Errorf("rollback repository open: %w", rollbackErr))
		}
		return cause
	}
	if err := applyRepositoryPrivateStateIsolation(cleanPath, rollbackData); err != nil {
		return nil, rollback(err)
	}
	if err := rm.dirManager.CreateStructure(cleanPath); err != nil {
		return nil, rollback(fmt.Errorf("create fresh repository private state: %w", err))
	}
	if err := rm.updateLifecycleOperationPhase(ctx, operation.OperationID, lifecyclePhaseFilesystemApplied, rollbackData); err != nil {
		return nil, rollback(fmt.Errorf("persist open-repository filesystem phase: %w", err))
	}

	databaseRepository, err := rm.addRepository(ctx, cleanPath, defaultOwnerID, dbtypes.RepoRoleRegular, true, rootID)
	if err != nil {
		return nil, rollback(err)
	}
	if err := rm.updateLifecycleOperationPhase(ctx, operation.OperationID, lifecyclePhaseCatalogCommitted, rollbackData); err != nil {
		return nil, fmt.Errorf("repository opened but journal commit phase failed: %w", err)
	}
	if err := rm.completeLifecycleOperation(ctx, operation.OperationID,
		openRepositoryOperationResult{RepositoryID: databaseRepository.RepoID.String(), InitialScanQueued: false}); err != nil {
		return nil, fmt.Errorf("repository opened but journal completion failed: %w", err)
	}
	// The scanner's enqueue path establishes a repository work lease of its
	// own. Release the lifecycle locks before entering it, otherwise the
	// enqueue path waits forever for the locks held by this function.
	releaseLocks()
	if err := rm.ScheduleInitialRepositoryScan(ctx, databaseRepository.RepoID.String()); err != nil {
		rm.logger.Warn("repository opened but initial scan could not be queued",
			zap.String("operation", "repository.open.scan"),
			zap.String("repository_id", databaseRepository.RepoID.String()),
			zap.Error(err))
	} else if err := rm.markInitialScanQueued(ctx, operation.OperationID); err != nil {
		return nil, err
	}
	rm.repoAudit(cleanPath).Operation("repository.open",
		zap.String("repository_id", databaseRepository.RepoID.String()),
		zap.String("private_state_recovery_path", rollbackData.RecoveryPath))
	return databaseRepository, nil
}
