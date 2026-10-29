package storage

import (
	"context"
	"database/sql"
	"encoding/json"
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
	// a different repository than the one being reconnected.
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
// library. The caller must present the choice: reconnect the existing
// repository, or register this path as a new, independent copy.
type RepositoryConflictError struct {
	RepositoryID   string
	RegisteredPath string
	RequestedPath  string
	Actions        []string
}

// HostActionResolutions projects the storage-side conflict actions of a
// candidate or native task onto the resolution codes the HTTP contract
// exposes: relocate is the moved-original decision (update_location), copy
// the independent-copy decision (add_separate), and confirm_risk the
// acknowledgement of a storage_risk review. The vocabulary is closed here so
// no handler invents a resolution the commands do not accept.
func HostActionResolutions(actions []string) []string {
	resolutions := make([]string, 0, len(actions))
	for _, action := range actions {
		switch action {
		case "relocate":
			resolutions = append(resolutions, "update_location")
		case "copy":
			resolutions = append(resolutions, "add_separate")
		case "confirm_risk":
			resolutions = append(resolutions, "confirm_risk")
		}
	}
	return resolutions
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

// ReconnectRepository re-points a registered Repository at the tree that now
// carries its identity, after that tree was moved or remounted externally.
// It never moves data (owner decision 5, "Relocate means reconnect only").
//
// Assets and Locations are untouched by construction: repository node paths
// are relative graph projections, so every consumer resolves the new root
// through RepositoryFS. The catalog path update and observation fence commit
// together under a journaled operation, so a retry with the same request ID
// replays the result and startup recovery settles an interrupted one.
func (rm *DefaultRepositoryManager) ReconnectRepository(ctx context.Context, id string, newPath string, requests ...LifecycleRequest) (*repo.Repository, error) {
	return rm.reconnectRepository(ctx, id, newPath, nil, requests...)
}

type reconnectRepositoryOperationPayload struct {
	RepositoryID      string `json:"repository_id"`
	OldPath           string `json:"old_path"`
	NewPath           string `json:"new_path"`
	StorageLocationID string `json:"storage_location_id"`
	ConfirmationType  string `json:"confirmation_type"`
	// Rekey records that the destination volume follows another name rule
	// than the registered one, so the cutover leaves the repository in
	// maintenance until every entry key is rebuilt under PathCase and
	// PathNormalization; recovery finishes that pass after a crash.
	Rekey             bool   `json:"rekey,omitempty"`
	PathCase          string `json:"path_case,omitempty"`
	PathNormalization string `json:"path_normalization,omitempty"`
}

type reconnectRepositoryOperationResult struct {
	RepositoryID      string `json:"repository_id"`
	StorageLocationID string `json:"storage_location_id"`
	FilesPreserved    bool   `json:"files_preserved"`
}

// replayCompletedReconnect answers a retried reconnect before any admission:
// the row it would re-admit is already at the new path, which the policy
// would otherwise read as "original online". A different target under the
// same request ID is a conflict.
func (rm *DefaultRepositoryManager) replayCompletedReconnect(ctx context.Context, requestID, repositoryID, newPath string) (*repo.Repository, bool, error) {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		return nil, false, nil
	}
	operation, err := rm.queries.GetLifecycleOperationByRequestID(ctx, requestID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("find lifecycle request: %w", err)
	}
	if operation.Kind != lifecycleKindReconnectRepository {
		return nil, false, ErrLifecycleRequestConflict
	}
	var payload reconnectRepositoryOperationPayload
	if err := json.Unmarshal(operation.Payload, &payload); err != nil || payload.RepositoryID != repositoryID || payload.NewPath != newPath {
		return nil, false, ErrLifecycleRequestConflict
	}
	if err := lifecycleReplayError(operation); err != nil {
		return nil, false, err
	}
	current, err := rm.GetRepository(repositoryID)
	if err != nil {
		return nil, false, err
	}
	return current, true, nil
}

func (rm *DefaultRepositoryManager) reconnectRepository(ctx context.Context, id string, newPath string, associatedStorageLocationID *uuid.UUID, requests ...LifecycleRequest) (*repo.Repository, error) {
	repoUUID, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("invalid repository ID: %w", err)
	}
	cleanPath, err := CanonicalizeRepositoryPath(newPath)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}
	request := LifecycleRequest{}
	if len(requests) > 0 {
		request = requests[0]
	}
	if replayed, ok, err := rm.replayCompletedReconnect(ctx, request.RequestID, id, cleanPath); err != nil {
		return nil, err
	} else if ok {
		return replayed, nil
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
	// The destination volume's name rule decides the keys; a destination
	// that cannot be qualified is refused before anything else is touched.
	plan, err := rm.planVolumeRekey(*current, cleanPath)
	if err != nil {
		return nil, err
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
	// Under the lease the marker at the registered path decides again: an
	// original that came back online is refused, an unreadable one admits
	// nothing, and only a positively unavailable one is reconnected.
	if err := rm.admitReconnect(ctx, *current); err != nil {
		return nil, err
	}
	if current.Activity != dbtypes.RepositoryActivityIdle {
		// The policy admitted stale activity only because the original is
		// positively unavailable; nothing can finish it, so it is cleared.
		if _, err := rm.queries.FinishRepositoryActivity(ctx, repo.FinishRepositoryActivityParams{
			RepoID: repoUUID, Activity: current.Activity, UpdatedAt: dbtypes.NewTimestamp(time.Now().UTC()),
		}); err != nil {
			return nil, fmt.Errorf("clear stale repository activity before reconnect: %w", err)
		}
		current, err = rm.GetRepository(id)
		if err != nil {
			return nil, err
		}
		if current.Activity != dbtypes.RepositoryActivityIdle {
			return nil, fmt.Errorf("%w: repository activity is %s", ErrRepositoryBusy, current.Activity)
		}
	}
	// A rule change enters maintenance first, in its own statement, so no
	// scan, upload or Trash command can write the index between the
	// preflight and the cutover; the preflight then reads a frozen index.
	// Until the cutover commits, any failure hands the barrier back.
	rekeyMaintenance := false
	if plan.needed() {
		if _, err := rm.queries.BeginRepositoryMaintenance(ctx, repo.BeginRepositoryMaintenanceParams{
			RepoID: repoUUID, UpdatedAt: dbtypes.NewTimestamp(time.Now().UTC()),
		}); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, fmt.Errorf("%w: repository has active work", ErrRepositoryBusy)
			}
			return nil, fmt.Errorf("enter rekey maintenance: %w", err)
		}
		rekeyMaintenance = true
		previousReachability := current.Reachability
		defer func() {
			if !rekeyMaintenance {
				return
			}
			_, _ = rm.queries.EndRepositoryMaintenance(context.Background(), repo.EndRepositoryMaintenanceParams{
				RepoID: repoUUID, Reachability: previousReachability,
				Activity: dbtypes.RepositoryActivityIdle, UpdatedAt: dbtypes.NewTimestamp(time.Now().UTC()),
			})
		}()
	}
	// Every live key must stay distinct under the destination rule; a
	// collision refuses the whole reconnect under the lease, with the pairs
	// named and nothing changed.
	if err := rm.preflightRekey(ctx, repoUUID, plan); err != nil {
		return nil, err
	}
	confirmation := strings.TrimSpace(request.ConfirmationType)
	if confirmation == "" {
		confirmation = "update_location"
	}
	payload := reconnectRepositoryOperationPayload{
		RepositoryID: id, OldPath: current.Path, NewPath: cleanPath,
		StorageLocationID: storageLocationID.String(), ConfirmationType: confirmation,
	}
	if plan.needed() {
		payload.Rekey = true
		payload.PathCase = string(plan.To.Case)
		payload.PathNormalization = string(plan.To.Normalization)
	}
	operation, replay, err := rm.beginLifecycleOperation(ctx, lifecycleBeginInput{
		RequestID: request.RequestID, Kind: lifecycleKindReconnectRepository,
		Payload: payload,
		Actor:   request.Actor, ActorUserID: request.ActorUserID, HostInstanceID: request.HostInstanceID,
		TargetType: "repository", TargetID: &id,
	})
	if err != nil {
		return nil, err
	}
	if replay {
		if err := lifecycleReplayError(operation); err != nil {
			return nil, err
		}
		return rm.GetRepository(id)
	}
	failPrepared := func(cause error) (*repo.Repository, error) {
		_ = rm.failLifecycleOperation(ctx, operation.OperationID, true, cause, nil)
		return nil, cause
	}
	tx, err := rm.writer.BeginTx(ctx, catalogtx.OperationRepositoryRelocate, nil)
	if err != nil {
		return failPrepared(fmt.Errorf("begin repository reconnect: %w", err))
	}
	defer func() { _ = tx.Rollback() }()
	// The catalog has one writer: the reconnect transaction must be rolled
	// back before the journal records the failure in its own transaction.
	failInTx := func(cause error) (*repo.Repository, error) {
		_ = tx.Rollback()
		return failPrepared(cause)
	}
	queries := rm.queries.WithTx(tx.Raw())
	var dbRepo repo.Repository
	if plan.needed() {
		// The path and the rule change together while the repository stays
		// in maintenance: no scan, upload, or Trash command can key against
		// the index until every row follows the new rule.
		dbRepo, err = queries.UpdateRepositoryPathSemantics(ctx, repo.UpdateRepositoryPathSemanticsParams{
			RepoID:            repoUUID,
			Path:              cleanPath,
			StorageLocationID: storageLocationID,
			Reachability:      dbtypes.RepositoryReachabilityMaintenance,
			PathCase:          plan.To.Case,
			PathNormalization: plan.To.Normalization,
			UpdatedAt:         dbtypes.NewTimestamp(now),
		})
	} else {
		dbRepo, err = queries.UpdateRepositoryPath(ctx, repo.UpdateRepositoryPathParams{
			RepoID:            repoUUID,
			Path:              cleanPath,
			StorageLocationID: storageLocationID,
			Reachability:      dbtypes.RepositoryReachabilityActive,
			UpdatedAt:         dbtypes.NewTimestamp(now),
		})
	}
	if err != nil {
		if isUniquePathViolation(err) {
			rm.repoAudit(cleanPath).Error("repository.reconnect", err, zap.String("repository_id", id))
			return failInTx(fmt.Errorf("%w: %s", ErrRepositoryExistsAtPath, cleanPath))
		}
		rm.repoAudit(cleanPath).Error("repository.reconnect", err, zap.String("repository_id", id))
		return failInTx(fmt.Errorf("failed to reconnect repository: %w", err))
	}
	nowStamp := dbtypes.NewTimestamp(now)
	if err := invalidateRepositoryObservationAfterRelocation(ctx, tx, repoUUID, cleanPath, nowStamp); err != nil {
		return failInTx(err)
	}
	if _, err := recordLifecycleAuditWithQueries(ctx, queries, LifecycleAuditInput{
		Actor: request.Actor, ActorUserID: request.ActorUserID, HostInstanceID: request.HostInstanceID,
		RequestID: request.RequestID, Action: "update_repository_location", TargetType: "repository",
		TargetID: id, Source: auditSourceForActor(request.Actor), ConfirmationType: confirmation,
		OldPath: current.Path, NewPath: cleanPath, Result: AuditResultSucceeded,
		Details: map[string]any{"risk_confirmation": request.RiskConfirmation},
	}); err != nil {
		return failInTx(fmt.Errorf("audit repository reconnect: %w", err))
	}
	if err := tx.Commit(); err != nil {
		return failInTx(fmt.Errorf("commit repository reconnect: %w", err))
	}
	if plan.needed() {
		// The cutover is durable and the journal owns the barrier from here:
		// the rekey runs in bounded batches outside the cutover transaction,
		// and a crash or failure leaves the repository in maintenance with
		// the operation running, which startup recovery finishes.
		rekeyMaintenance = false
		if err := rm.updateLifecycleOperationPhase(ctx, operation.OperationID, lifecyclePhaseCatalogCommitted, nil); err != nil {
			return nil, fmt.Errorf("%w: repository reconnected but its journal phase could not be recorded: %v", ErrLifecycleRecoveryRequired, err)
		}
		if err := rm.finishVolumeRekey(ctx, repoUUID); err != nil {
			return nil, fmt.Errorf("%w: repository reconnected at %s but its index is still being rebuilt for the volume: %v", ErrLifecycleRecoveryRequired, cleanPath, err)
		}
		refreshed, err := rm.queries.GetRepository(ctx, repoUUID)
		if err != nil {
			return nil, fmt.Errorf("read rekeyed repository: %w", err)
		}
		dbRepo = refreshed
	}
	if err := rm.completeLifecycleOperation(ctx, operation.OperationID, reconnectRepositoryOperationResult{
		RepositoryID: id, StorageLocationID: storageLocationID.String(), FilesPreserved: true,
	}); err != nil {
		return nil, fmt.Errorf("repository reconnected but journal completion failed: %w", err)
	}

	// The path is now correct and durable. Refreshing the DB's cached copy of
	// the on-disk config is a separate, recoverable step: if it fails the
	// repository is still correctly located and the next boot reconcile will
	// bring the cache forward.
	if refreshed, err := rm.refreshRepositoryConfigCache(ctx, dbRepo, config); err != nil {
		rm.logger.Warn("reconnected repository but failed to refresh config cache",
			zap.String("operation", "repository.reconnect"),
			zap.String("repository_id", id),
			zap.Error(err))
	} else {
		dbRepo = *refreshed
	}

	rm.repoAudit(cleanPath).Operation("repository.reconnect",
		zap.String("repository_id", id),
		zap.String("repository_path", cleanPath),
	)
	rm.logger.Info("repository reconnected",
		zap.String("operation", "repository.reconnect"),
		zap.String("repository_id", id),
		zap.String("repository_path", cleanPath),
	)

	return &dbRepo, nil
}

// admitReconnect applies the role-aware policy to a reconnect. The catalog
// may still project an original as active after it went away, so a read of
// the registered path, not the projection, decides whether there is
// something to reconnect: an original that is positively online is
// ErrRepositoryOriginalOnline, an unreadable one withholds the action with
// original_inaccessible (denial is never absence), and any other withheld
// verdict is the typed refusal with its blockers.
func (rm *DefaultRepositoryManager) admitReconnect(ctx context.Context, current repo.Repository) error {
	facts, err := rm.repositoryActionFacts(ctx, current)
	if err != nil {
		return err
	}
	state := rm.repositoryOriginalState(current.Path, current.RepoID.String())
	switch state {
	case OriginalInaccessible:
		facts.OriginalInaccessible = true
	case OriginalAbsent, OriginalIdentityDiffers:
		if facts.Reachability == dbtypes.RepositoryReachabilityActive {
			facts.Reachability = dbtypes.RepositoryReachabilityOffline
		}
	}
	verdict := DeriveRepositoryActions(facts).Verdict(ActionReconnect)
	if verdict.Allowed() {
		if state == OriginalOnline {
			return fmt.Errorf("%w: %s", ErrRepositoryOriginalOnline, current.Path)
		}
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
	newUUID, err := uuid.Parse(newID)
	if err != nil {
		return nil, fmt.Errorf("invalid register-copy target: %w", err)
	}
	releaseStorageLocation := rm.acquireStorageLocationRead(storageLocationID)
	releaseMutation := rm.acquireRepositoryMutation(previousUUID)
	// The new identity is unregistered, so no other holder can order it
	// against previousUUID. Holding it here lets addRepository run with
	// locksHeld: leases are not reentrant, and its own Storage Location read
	// would wait behind a Location mutation that waits for ours.
	releaseNewMutation := rm.acquireRepositoryMutation(newUUID)
	locksReleased := false
	releaseLocks := func() {
		if locksReleased {
			return
		}
		locksReleased = true
		releaseNewMutation()
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

	dbRepo, err := rm.addRepository(context.Background(), cleanPath, defaultOwnerID, role, true, storageLocationID)
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
