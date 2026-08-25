package storage

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"server/internal/db/catalogtx"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/storage/repocfg"
	"server/internal/storage/rootcfg"

	"github.com/google/uuid"
)

const (
	lifecycleKindCreateRepository       = "create_repository"
	lifecycleKindCreateStorageLocation  = "create_storage_location"
	lifecycleKindOpenRepository         = "open_repository"
	lifecycleKindRegisterRepositoryCopy = "register_repository_copy"
	lifecycleKindSwitchDefaultStorage   = "switch_default_storage_location"
	lifecycleKindRelocateStorage        = "relocate_storage_location"
	lifecycleKindRenameRepository       = "rename_repository"

	lifecyclePhasePrepared          = "prepared"
	lifecyclePhaseFilesystemApplied = "filesystem_applied"
	lifecyclePhaseCatalogCommitted  = "catalog_committed"
	lifecyclePhaseRollbackRequired  = "rollback_required"
	lifecyclePhaseFailed            = "failed"

	lifecycleStatusRunning    = "running"
	lifecycleStatusCompleted  = "completed"
	lifecycleStatusFailed     = "failed"
	lifecycleStatusRolledBack = "rolled_back"
)

// LifecycleRequest carries host/API idempotency identity into the durable
// journal. Actor is audit context, not part of payload equality.
type LifecycleRequest struct {
	RequestID        string
	Actor            string
	ActorUserID      *int32
	HostInstanceID   string
	ConfirmationType string
	RiskConfirmation bool
}

var (
	ErrLifecycleRequestConflict  = errors.New("lifecycle request ID was reused with a different payload")
	ErrLifecycleOperationRunning = errors.New("lifecycle operation is still running")
	ErrLifecycleOperationFailed  = errors.New("lifecycle operation already failed")
	ErrLifecycleRecoveryRequired = errors.New("lifecycle operation requires recovery")
)

type lifecycleBeginInput struct {
	RequestID      string
	Kind           string
	Payload        any
	Actor          string
	ActorUserID    *int32
	HostInstanceID string
	TargetType     string
	TargetID       *string
	RollbackData   any
}

type createRepositoryOperationPayload struct {
	Name              string           `json:"name"`
	Path              string           `json:"path"`
	RootID            string           `json:"root_id"`
	Role              dbtypes.RepoRole `json:"role"`
	OwnerID           *int32           `json:"owner_id,omitempty"`
	StorageStrategy   string           `json:"storage_strategy"`
	DuplicateHandling string           `json:"duplicate_handling"`
	RiskConfirmation  bool             `json:"risk_confirmation"`
}

type createRepositoryRollbackData struct {
	Path          string `json:"path"`
	TargetCreated bool   `json:"target_created"`
}

type createRepositoryOperationResult struct {
	RepositoryID string `json:"repository_id"`
}

type createStorageLocationOperationPayload struct {
	Path string                     `json:"path"`
	Name string                     `json:"name"`
	Kind dbtypes.RepositoryRootKind `json:"kind"`
}

type createStorageLocationRollbackData struct {
	Path          string `json:"path"`
	MarkerCreated bool   `json:"marker_created"`
}

type createStorageLocationOperationResult struct {
	RootID string `json:"root_id"`
}

type registerRepositoryCopyOperationPayload struct {
	Path             string           `json:"path"`
	RootID           string           `json:"root_id"`
	OwnerID          *int32           `json:"owner_id,omitempty"`
	Role             dbtypes.RepoRole `json:"role"`
	RiskConfirmation bool             `json:"risk_confirmation"`
}

type registerRepositoryCopyRollbackData struct {
	PreviousRepositoryID string   `json:"previous_repository_id"`
	RecoveryPath         string   `json:"recovery_path,omitempty"`
	MovedEntries         []string `json:"moved_entries,omitempty"`
	PrivateRootExisted   bool     `json:"private_root_existed,omitempty"`
}

type registerRepositoryCopyOperationResult struct {
	RepositoryID      string `json:"repository_id"`
	InitialScanQueued bool   `json:"initial_scan_queued"`
}

type openRepositoryOperationPayload struct {
	Path             string           `json:"path"`
	RootID           string           `json:"root_id"`
	OwnerID          *int32           `json:"owner_id,omitempty"`
	Role             dbtypes.RepoRole `json:"role"`
	RiskConfirmation bool             `json:"risk_confirmation"`
}

type openRepositoryOperationResult struct {
	RepositoryID      string `json:"repository_id"`
	InitialScanQueued bool   `json:"initial_scan_queued"`
}

type renameRepositoryOperationPayload struct {
	RepositoryID string `json:"repository_id"`
	Path         string `json:"path"`
	NewName      string `json:"new_name"`
}

type renameRepositoryRollbackData struct {
	PreviousConfig repocfg.RepositoryConfig `json:"previous_config"`
}

type renameRepositoryOperationResult struct {
	RepositoryID string `json:"repository_id"`
	Name         string `json:"name"`
}

func (rm *DefaultRepositoryManager) markInitialScanQueued(ctx context.Context, operationID string) error {
	result, err := rm.writer.ExecContext(ctx, catalogtx.OperationRepositoryLifecycleMarkScanQueued, `
		UPDATE lifecycle_operations
		SET result = json_set(result, '$.initial_scan_queued', json('true')), updated_at = ?
		WHERE operation_id = ? AND status = 'completed'
	`, dbtypes.NewTimestamp(time.Now().UTC()), operationID)
	if err != nil {
		return fmt.Errorf("persist initial scan receipt: %w", err)
	}
	if rows, _ := result.RowsAffected(); rows != 1 {
		return fmt.Errorf("persist initial scan receipt: lifecycle operation is unavailable")
	}
	return nil
}

// RetryPendingInitialRepositoryScans closes the crash window between durable
// repository registration and River insertion. The lifecycle result remains
// false until the queue has accepted the scan, so restart and a low-frequency
// runtime retry can safely finish the handoff.
func (rm *DefaultRepositoryManager) RetryPendingInitialRepositoryScans(ctx context.Context) error {
	if rm.initialScan == nil {
		return errors.New("initial repository scan queue is unavailable")
	}
	rows, err := rm.readerDatabase.QueryContext(ctx, `
		SELECT operation_id, target_id
		FROM lifecycle_operations
		WHERE kind IN ('open_repository', 'register_repository_copy')
		  AND status = 'completed'
		  AND target_id IS NOT NULL
		  AND COALESCE(json_extract(result, '$.initial_scan_queued'), 0) = 0
		  AND EXISTS (SELECT 1 FROM repositories WHERE repo_id = lifecycle_operations.target_id)
		ORDER BY created_at
	`)
	if err != nil {
		return fmt.Errorf("list pending initial scans: %w", err)
	}
	defer rows.Close()
	type pending struct{ operationID, repositoryID string }
	var items []pending
	for rows.Next() {
		var item pending
		if err := rows.Scan(&item.operationID, &item.repositoryID); err != nil {
			return err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, item := range items {
		if err := rm.ScheduleInitialRepositoryScan(ctx, item.repositoryID); err != nil {
			return fmt.Errorf("queue pending initial scan for %s: %w", item.repositoryID, err)
		}
		if err := rm.markInitialScanQueued(ctx, item.operationID); err != nil {
			return err
		}
	}
	return nil
}

type switchDefaultStorageOperationPayload struct {
	RootID           string `json:"root_id"`
	OldPath          string `json:"old_path"`
	NewPath          string `json:"new_path"`
	ConfirmationType string `json:"confirmation_type"`
	RepositoryCount  int64  `json:"repository_count"`
}

type switchDefaultStorageOperationResult struct {
	RootID          string `json:"root_id"`
	RepositoryCount int64  `json:"repository_count"`
	FilesPreserved  bool   `json:"files_preserved"`
}

func (rm *DefaultRepositoryManager) beginLifecycleOperation(ctx context.Context, input lifecycleBeginInput) (repo.LifecycleOperation, bool, error) {
	if rm == nil || rm.queries == nil {
		return repo.LifecycleOperation{}, false, errors.New("lifecycle operation catalog is unavailable")
	}
	payload, err := marshalLifecycleJSON(input.Payload)
	if err != nil {
		return repo.LifecycleOperation{}, false, fmt.Errorf("marshal lifecycle payload: %w", err)
	}
	digest := sha256.Sum256(payload)
	payloadHash := hex.EncodeToString(digest[:])
	requestID := strings.TrimSpace(input.RequestID)
	if requestID == "" {
		requestID = uuid.NewString()
	}
	actor := strings.TrimSpace(input.Actor)
	if actor == "" {
		actor = "server"
	}

	if existing, err := rm.queries.GetLifecycleOperationByRequestID(ctx, requestID); err == nil {
		if existing.Kind != input.Kind || existing.PayloadHash != payloadHash {
			return repo.LifecycleOperation{}, false, ErrLifecycleRequestConflict
		}
		return existing, true, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return repo.LifecycleOperation{}, false, fmt.Errorf("find lifecycle request: %w", err)
	}

	rollback, err := marshalOptionalLifecycleJSON(input.RollbackData)
	if err != nil {
		return repo.LifecycleOperation{}, false, fmt.Errorf("marshal lifecycle rollback data: %w", err)
	}
	now := dbtypes.NewTimestamp(time.Now().UTC())
	created, err := rm.queries.CreateLifecycleOperation(ctx, repo.CreateLifecycleOperationParams{
		OperationID:    uuid.NewString(),
		RequestID:      requestID,
		Kind:           input.Kind,
		PayloadHash:    payloadHash,
		Payload:        payload,
		Actor:          actor,
		ActorUserID:    lifecycleActorUserID(input.ActorUserID),
		HostInstanceID: strings.TrimSpace(input.HostInstanceID),
		TargetType:     input.TargetType,
		TargetID:       input.TargetID,
		RollbackData:   rollback,
		CreatedAt:      now,
	})
	if err == nil {
		return created, false, nil
	}
	// A concurrent retry may win the UNIQUE(request_id) race. Re-read and apply
	// the same payload comparison instead of surfacing a database detail.
	existing, readErr := rm.queries.GetLifecycleOperationByRequestID(ctx, requestID)
	if readErr != nil {
		return repo.LifecycleOperation{}, false, fmt.Errorf("create lifecycle operation: %w", err)
	}
	if existing.Kind != input.Kind || existing.PayloadHash != payloadHash {
		return repo.LifecycleOperation{}, false, ErrLifecycleRequestConflict
	}
	return existing, true, nil
}

func lifecycleActorUserID(value *int32) *int64 {
	if value == nil {
		return nil
	}
	converted := int64(*value)
	return &converted
}

func (rm *DefaultRepositoryManager) updateLifecycleOperationPhase(
	ctx context.Context,
	operationID string,
	phase string,
	rollbackData any,
) error {
	rollback, err := marshalOptionalLifecycleJSON(rollbackData)
	if err != nil {
		return err
	}
	_, err = rm.queries.UpdateLifecycleOperationPhase(ctx, repo.UpdateLifecycleOperationPhaseParams{
		OperationID:  operationID,
		Phase:        phase,
		RollbackData: rollback,
		UpdatedAt:    dbtypes.NewTimestamp(time.Now().UTC()),
	})
	return err
}

func (rm *DefaultRepositoryManager) completeLifecycleOperation(ctx context.Context, operationID string, result any) error {
	return rm.completeLifecycleOperationWithAuditResult(ctx, operationID, result, AuditResultSucceeded)
}

func (rm *DefaultRepositoryManager) completeRecoveredLifecycleOperation(ctx context.Context, operationID string, result any) error {
	return rm.completeLifecycleOperationWithAuditResult(ctx, operationID, result, AuditResultRecovered)
}

func (rm *DefaultRepositoryManager) completeLifecycleOperationWithAuditResult(ctx context.Context, operationID string, result any, auditResult string) error {
	encoded, err := marshalOptionalLifecycleJSON(result)
	if err != nil {
		return err
	}
	tx, err := rm.writer.BeginTx(ctx, catalogtx.OperationRepositoryLifecycleComplete, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	queries := rm.queries.WithTx(tx.Raw())
	operation, lookupErr := queries.GetLifecycleOperation(ctx, operationID)
	if lookupErr != nil {
		return lookupErr
	}
	_, err = queries.CompleteLifecycleOperation(ctx, repo.CompleteLifecycleOperationParams{
		OperationID: operationID,
		Result:      encoded,
		UpdatedAt:   dbtypes.NewTimestamp(time.Now().UTC()),
	})
	if err != nil {
		return err
	}
	if err := rm.recordJournalAudit(ctx, queries, operation, auditResult, "", result); err != nil {
		return err
	}
	return tx.Commit()
}

func (rm *DefaultRepositoryManager) failLifecycleOperation(
	ctx context.Context,
	operationID string,
	rolledBack bool,
	cause error,
	rollbackData any,
) error {
	return rm.failLifecycleOperationWithAuditResult(ctx, operationID, rolledBack, cause, rollbackData, AuditResultFailed)
}

func (rm *DefaultRepositoryManager) recoverLifecycleOperationFailure(
	ctx context.Context,
	operationID string,
	rolledBack bool,
	cause error,
	rollbackData any,
) error {
	return rm.failLifecycleOperationWithAuditResult(ctx, operationID, rolledBack, cause, rollbackData, AuditResultRecovered)
}

func (rm *DefaultRepositoryManager) failLifecycleOperationWithAuditResult(
	ctx context.Context,
	operationID string,
	rolledBack bool,
	cause error,
	rollbackData any,
	auditResult string,
) error {
	rollback, err := marshalOptionalLifecycleJSON(rollbackData)
	if err != nil {
		return err
	}
	phase := lifecyclePhaseRollbackRequired
	status := lifecycleStatusFailed
	if rolledBack {
		phase = lifecyclePhaseFailed
		status = lifecycleStatusRolledBack
	}
	var errorText *string
	if cause != nil {
		message := cause.Error()
		errorText = &message
	}
	tx, beginErr := rm.writer.BeginTx(ctx, catalogtx.OperationRepositoryLifecycleFail, nil)
	if beginErr != nil {
		return beginErr
	}
	defer func() { _ = tx.Rollback() }()
	queries := rm.queries.WithTx(tx.Raw())
	operation, lookupErr := queries.GetLifecycleOperation(ctx, operationID)
	if lookupErr != nil && !errors.Is(lookupErr, sql.ErrNoRows) {
		return lookupErr
	}
	_, err = queries.FailLifecycleOperation(ctx, repo.FailLifecycleOperationParams{
		OperationID:  operationID,
		Phase:        phase,
		Status:       status,
		Error:        errorText,
		RollbackData: rollback,
		UpdatedAt:    dbtypes.NewTimestamp(time.Now().UTC()),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return tx.Commit()
	}
	if err != nil {
		return err
	}
	if operation.OperationID == "" {
		return tx.Commit()
	}
	details := map[string]any{"rolled_back": rolledBack}
	if cause != nil {
		details["error"] = cause.Error()
	}
	if err := rm.recordJournalAudit(ctx, queries, operation, auditResult, phase, details); err != nil {
		return err
	}
	return tx.Commit()
}

func (rm *DefaultRepositoryManager) recordJournalAudit(
	ctx context.Context,
	queries *repo.Queries,
	operation repo.LifecycleOperation,
	result string,
	failureStage string,
	resultDetails any,
) error {
	var payload map[string]any
	_ = json.Unmarshal([]byte(operation.Payload), &payload)
	details := map[string]any{"journal_phase": operation.Phase}
	if resultDetails != nil {
		details["outcome"] = resultDetails
	}
	targetID := ""
	if operation.TargetID != nil {
		targetID = *operation.TargetID
	}
	path, _ := payload["path"].(string)
	oldPath, _ := payload["old_path"].(string)
	newPath, _ := payload["new_path"].(string)
	if newPath == "" {
		newPath = path
	}
	confirmation := "none"
	if operation.Kind == lifecycleKindRegisterRepositoryCopy {
		confirmation = "independent_identity"
	} else if operation.Kind == lifecycleKindSwitchDefaultStorage {
		if value, _ := payload["confirmation_type"].(string); strings.TrimSpace(value) != "" {
			confirmation = value
		}
	} else if confirmed, _ := payload["risk_confirmation"].(bool); confirmed {
		confirmation = "storage_risk"
	}
	source := auditSourceForActor(operation.Actor)
	if result == AuditResultRecovered {
		source = "recovery"
	}
	_, err := recordLifecycleAuditWithQueries(ctx, queries, LifecycleAuditInput{
		Actor: operation.Actor, ActorUserID: lifecycleAuditActorUserID(operation.ActorUserID), HostInstanceID: operation.HostInstanceID, RequestID: operation.RequestID,
		OperationID: operation.OperationID, Action: operation.Kind, TargetType: operation.TargetType,
		TargetID: targetID, Source: source, ConfirmationType: confirmation,
		OldPath: oldPath, NewPath: newPath, Result: result, FailureStage: failureStage, Details: details,
	})
	return err
}

func lifecycleAuditActorUserID(value *int64) *int32 {
	if value == nil {
		return nil
	}
	converted := int32(*value)
	return &converted
}

// RecoverLifecycleOperations resolves filesystem/catalog gaps before workers
// start. Unknown operation kinds fail closed so a new filesystem mutation is
// never guessed by an older binary.
func (rm *DefaultRepositoryManager) RecoverLifecycleOperations(ctx context.Context) error {
	operations, err := rm.queries.ListIncompleteLifecycleOperations(ctx)
	if err != nil {
		return fmt.Errorf("list incomplete lifecycle operations: %w", err)
	}
	for _, operation := range operations {
		if err := rm.claimLifecycleRecoveryTargets(ctx, operation); err != nil {
			return fmt.Errorf("claim lifecycle recovery target for %s: %w", operation.OperationID, err)
		}
		switch operation.Kind {
		case lifecycleKindCreateRepository:
			if err := rm.recoverCreateRepositoryOperation(ctx, operation); err != nil {
				return err
			}
		case lifecycleKindCreateStorageLocation:
			if err := rm.recoverCreateStorageLocationOperation(ctx, operation); err != nil {
				return err
			}
		case lifecycleKindOpenRepository:
			if err := rm.recoverOpenRepositoryOperation(ctx, operation); err != nil {
				return err
			}
		case lifecycleKindRegisterRepositoryCopy:
			if err := rm.recoverRegisterRepositoryCopyOperation(ctx, operation); err != nil {
				return err
			}
		case lifecycleKindSwitchDefaultStorage, lifecycleKindRelocateStorage:
			if err := rm.recoverSwitchDefaultStorageOperation(ctx, operation); err != nil {
				return err
			}
		case lifecycleKindRenameRepository:
			if err := rm.recoverRenameRepositoryOperation(ctx, operation); err != nil {
				return err
			}
		default:
			return fmt.Errorf("%w: unsupported operation kind %q", ErrLifecycleRecoveryRequired, operation.Kind)
		}
	}
	return nil
}

// claimLifecycleRecoveryTargets closes the startup gap for paths which are
// present only in an incomplete journal and therefore were not part of the
// active catalog snapshot claimed by AcquireRuntimeStorageOwnership. The
// acquired lock is retained by runtime ownership until Server shutdown.
func (rm *DefaultRepositoryManager) claimLifecycleRecoveryTargets(ctx context.Context, operation repo.LifecycleOperation) error {
	var target struct {
		Path    string `json:"path"`
		NewPath string `json:"new_path"`
	}
	if err := json.Unmarshal(operation.Payload, &target); err != nil {
		return fmt.Errorf("%w: decode lifecycle target: %v", ErrLifecycleRecoveryRequired, err)
	}
	path := strings.TrimSpace(target.Path)
	kind := "repository"
	switch operation.Kind {
	case lifecycleKindCreateStorageLocation:
		kind = "root"
	case lifecycleKindSwitchDefaultStorage, lifecycleKindRelocateStorage:
		kind = "root"
		path = strings.TrimSpace(target.NewPath)
	case lifecycleKindCreateRepository, lifecycleKindOpenRepository,
		lifecycleKindRegisterRepositoryCopy, lifecycleKindRenameRepository:
		// The common payload path is the repository mutation target.
	default:
		return fmt.Errorf("%w: unsupported operation kind %q", ErrLifecycleRecoveryRequired, operation.Kind)
	}
	if path == "" {
		return fmt.Errorf("%w: lifecycle recovery target path is missing", ErrLifecycleRecoveryRequired)
	}
	cleanPath, err := CanonicalizeRepositoryPath(path)
	if err != nil {
		return fmt.Errorf("%w: invalid lifecycle recovery target: %v", ErrLifecycleRecoveryRequired, err)
	}
	return rm.claimRuntimeStoragePath(ctx, kind, cleanPath)
}

func (rm *DefaultRepositoryManager) recoverRenameRepositoryOperation(ctx context.Context, operation repo.LifecycleOperation) error {
	var payload renameRepositoryOperationPayload
	if err := json.Unmarshal(operation.Payload, &payload); err != nil {
		return fmt.Errorf("%w: decode rename payload: %v", ErrLifecycleRecoveryRequired, err)
	}
	repositoryID, err := uuid.Parse(payload.RepositoryID)
	if err != nil {
		return fmt.Errorf("%w: invalid rename repository ID", ErrLifecycleRecoveryRequired)
	}
	databaseRepository, databaseErr := rm.queries.GetRepository(ctx, repositoryID)
	diskConfig, diskErr := repocfg.LoadConfigFromFile(payload.Path)
	if databaseErr != nil || diskErr != nil || diskConfig.ID != repositoryID.String() {
		return fmt.Errorf("%w: rename target identity is unavailable", ErrLifecycleRecoveryRequired)
	}
	if err := rm.files.ValidateRepositoryParent(ctx, databaseRepository); err != nil {
		return fmt.Errorf("%w: validate rename parent: %v", ErrLifecycleRecoveryRequired, err)
	}

	// The requested new name is the deterministic roll-forward state. Either
	// side may have committed immediately before a crash.
	diskConfig.Name = payload.NewName
	if err := diskConfig.SaveConfigToFile(payload.Path); err != nil {
		return fmt.Errorf("recover rename marker: %w", err)
	}
	if _, err := rm.queries.UpdateRepository(ctx, repo.UpdateRepositoryParams{
		RepoID: repositoryID, Name: payload.NewName, Config: *diskConfig,
		DefaultOwnerID: databaseRepository.DefaultOwnerID,
		UpdatedAt:      dbtypes.NewTimestamp(time.Now().UTC()),
	}); err != nil {
		return fmt.Errorf("recover rename catalog: %w", err)
	}
	return rm.completeRecoveredLifecycleOperation(ctx, operation.OperationID,
		renameRepositoryOperationResult{RepositoryID: repositoryID.String(), Name: payload.NewName})
}

func (rm *DefaultRepositoryManager) recoverOpenRepositoryOperation(ctx context.Context, operation repo.LifecycleOperation) error {
	var payload openRepositoryOperationPayload
	if err := json.Unmarshal(operation.Payload, &payload); err != nil {
		return fmt.Errorf("%w: decode open-repository payload: %v", ErrLifecycleRecoveryRequired, err)
	}
	if operation.TargetID == nil {
		return fmt.Errorf("%w: open-repository target ID is missing", ErrLifecycleRecoveryRequired)
	}
	repositoryID, err := uuid.Parse(*operation.TargetID)
	if err != nil {
		return fmt.Errorf("%w: invalid open-repository target ID", ErrLifecycleRecoveryRequired)
	}
	databaseRepository, databaseErr := rm.queries.GetRepository(ctx, repositoryID)
	diskConfig, diskErr := repocfg.LoadConfigFromFile(payload.Path)
	if databaseErr == nil && diskErr == nil && diskConfig.ID == repositoryID.String() && databaseRepository.Path == payload.Path {
		return rm.completeRecoveredLifecycleOperation(ctx, operation.OperationID,
			openRepositoryOperationResult{RepositoryID: repositoryID.String(), InitialScanQueued: false})
	}
	if databaseErr != nil && !errors.Is(databaseErr, sql.ErrNoRows) {
		return fmt.Errorf("recover open-repository catalog lookup: %w", databaseErr)
	}
	if databaseErr == nil {
		return fmt.Errorf("%w: opened repository row exists but disk identity is invalid", ErrLifecycleRecoveryRequired)
	}

	rollback := registerRepositoryCopyRollbackData{}
	if operation.RollbackData != nil {
		if err := json.Unmarshal(*operation.RollbackData, &rollback); err != nil {
			return fmt.Errorf("%w: decode open-repository rollback data: %v", ErrLifecycleRecoveryRequired, err)
		}
	}
	if diskErr != nil {
		return fmt.Errorf("%w: opened repository marker is unavailable", ErrLifecycleRecoveryRequired)
	}
	if diskConfig.ID != repositoryID.String() {
		return fmt.Errorf("%w: opened repository path now carries a different identity", ErrLifecycleRecoveryRequired)
	}
	if err := rollbackRepositoryPrivateStateIsolation(payload.Path, rollback); err != nil {
		_ = rm.recoverLifecycleOperationFailure(ctx, operation.OperationID, false, err, rollback)
		return fmt.Errorf("%w: restore reopened repository private state: %v", ErrLifecycleRecoveryRequired, err)
	}
	return rm.recoverLifecycleOperationFailure(ctx, operation.OperationID, true, errors.New("rolled back incomplete repository open"), rollback)
}

func (rm *DefaultRepositoryManager) recoverSwitchDefaultStorageOperation(ctx context.Context, operation repo.LifecycleOperation) error {
	var payload switchDefaultStorageOperationPayload
	if err := json.Unmarshal(operation.Payload, &payload); err != nil {
		return fmt.Errorf("%w: decode default Storage Location switch payload: %v", ErrLifecycleRecoveryRequired, err)
	}
	rootID, err := uuid.Parse(payload.RootID)
	if err != nil {
		return fmt.Errorf("%w: invalid default Storage Location switch identity", ErrLifecycleRecoveryRequired)
	}
	root, err := rm.queries.GetRepositoryRoot(ctx, rootID)
	if err != nil {
		return fmt.Errorf("recover default Storage Location switch: %w", err)
	}
	marker, markerErr := rootcfg.Load(payload.NewPath)
	if markerErr != nil || marker.ID != rootID.String() {
		return fmt.Errorf("%w: configured default Storage Location marker is invalid", ErrLifecycleRecoveryRequired)
	}
	if root.Path == payload.OldPath {
		// A process crash may have persisted the maintenance barrier before the
		// atomic path transaction. Release that orphaned barrier under the
		// lifecycle journal, then replay the fully validating relocation.
		if root.Status == dbtypes.RepositoryRootStatusMaintenance {
			_, _ = rm.queries.UpdateRepositoryRootFromDisk(ctx, repo.UpdateRepositoryRootFromDiskParams{
				RootID: rootID, Name: root.Name, Status: dbtypes.RepositoryRootStatusActive,
				UpdatedAt: dbtypes.NewTimestamp(time.Now().UTC()),
			})
			repositories, listErr := rm.queries.ListRepositories(ctx)
			if listErr != nil {
				return listErr
			}
			for _, repository := range repositories {
				if repository.RootID == rootID && repository.Reachability == dbtypes.RepositoryReachabilityMaintenance {
					_, _ = rm.queries.EndRepositoryMaintenance(ctx, repo.EndRepositoryMaintenanceParams{
						RepoID: repository.RepoID, Reachability: dbtypes.RepositoryReachabilityActive,
						Activity: dbtypes.RepositoryActivityIdle, UpdatedAt: dbtypes.NewTimestamp(time.Now().UTC()),
					})
				}
			}
		}
		if _, err := rm.relocateRepositoryRoot(ctx, rootID.String(), payload.NewPath, operation.Kind == lifecycleKindSwitchDefaultStorage); err != nil {
			return fmt.Errorf("%w: roll forward default Storage Location switch: %v", ErrLifecycleRecoveryRequired, err)
		}
	} else if root.Path != payload.NewPath {
		return fmt.Errorf("%w: default Storage Location path changed outside the operation", ErrLifecycleRecoveryRequired)
	}
	return rm.completeRecoveredLifecycleOperation(ctx, operation.OperationID,
		createStorageLocationOperationResult{RootID: rootID.String()})
}

func (rm *DefaultRepositoryManager) recoverRegisterRepositoryCopyOperation(ctx context.Context, operation repo.LifecycleOperation) error {
	var payload registerRepositoryCopyOperationPayload
	if err := json.Unmarshal(operation.Payload, &payload); err != nil {
		return fmt.Errorf("%w: decode register-copy payload: %v", ErrLifecycleRecoveryRequired, err)
	}
	if operation.TargetID == nil {
		return fmt.Errorf("%w: register-copy target ID is missing", ErrLifecycleRecoveryRequired)
	}
	repositoryID, err := uuid.Parse(*operation.TargetID)
	if err != nil {
		return fmt.Errorf("%w: invalid register-copy target ID", ErrLifecycleRecoveryRequired)
	}
	databaseRepository, databaseErr := rm.queries.GetRepository(ctx, repositoryID)
	diskConfig, diskErr := repocfg.LoadConfigFromFile(payload.Path)
	if databaseErr == nil && diskErr == nil && diskConfig.ID == repositoryID.String() && databaseRepository.Path == payload.Path {
		return rm.completeRecoveredLifecycleOperation(ctx, operation.OperationID,
			registerRepositoryCopyOperationResult{RepositoryID: repositoryID.String(), InitialScanQueued: false})
	}
	if databaseErr != nil && !errors.Is(databaseErr, sql.ErrNoRows) {
		return fmt.Errorf("recover register-copy catalog lookup: %w", databaseErr)
	}
	if databaseErr == nil {
		return fmt.Errorf("%w: copied repository row exists but disk identity is invalid", ErrLifecycleRecoveryRequired)
	}

	rollback := registerRepositoryCopyRollbackData{}
	if operation.RollbackData == nil || json.Unmarshal(*operation.RollbackData, &rollback) != nil || rollback.PreviousRepositoryID == "" {
		return fmt.Errorf("%w: register-copy rollback identity is missing", ErrLifecycleRecoveryRequired)
	}
	if diskErr == nil {
		if diskConfig.ID != repositoryID.String() && diskConfig.ID != rollback.PreviousRepositoryID {
			return fmt.Errorf("%w: copied path now carries a different identity", ErrLifecycleRecoveryRequired)
		}
		if diskConfig.ID == repositoryID.String() {
			diskConfig.ID = rollback.PreviousRepositoryID
			if err := diskConfig.SaveConfigToFile(payload.Path); err != nil {
				_ = rm.recoverLifecycleOperationFailure(ctx, operation.OperationID, false, err, rollback)
				return fmt.Errorf("%w: restore copied repository identity: %v", ErrLifecycleRecoveryRequired, err)
			}
		}
	}
	if err := rollbackRepositoryPrivateStateIsolation(payload.Path, rollback); err != nil {
		_ = rm.recoverLifecycleOperationFailure(ctx, operation.OperationID, false, err, rollback)
		return fmt.Errorf("%w: restore copied repository private state: %v", ErrLifecycleRecoveryRequired, err)
	}
	return rm.recoverLifecycleOperationFailure(ctx, operation.OperationID, true, errors.New("rolled back incomplete repository copy registration"), rollback)
}

func (rm *DefaultRepositoryManager) recoverCreateStorageLocationOperation(ctx context.Context, operation repo.LifecycleOperation) error {
	var payload createStorageLocationOperationPayload
	if err := json.Unmarshal(operation.Payload, &payload); err != nil {
		return fmt.Errorf("%w: decode Storage Location payload: %v", ErrLifecycleRecoveryRequired, err)
	}
	if operation.TargetID == nil {
		return fmt.Errorf("%w: Storage Location target ID is missing", ErrLifecycleRecoveryRequired)
	}
	rootID, err := uuid.Parse(*operation.TargetID)
	if err != nil {
		return fmt.Errorf("%w: invalid Storage Location target ID", ErrLifecycleRecoveryRequired)
	}
	databaseRoot, databaseErr := rm.queries.GetRepositoryRoot(ctx, rootID)
	diskConfig, diskErr := rootcfg.Load(payload.Path)
	if databaseErr == nil && diskErr == nil && diskConfig.ID == rootID.String() && databaseRoot.Path == payload.Path {
		return rm.completeRecoveredLifecycleOperation(ctx, operation.OperationID, createStorageLocationOperationResult{RootID: rootID.String()})
	}
	if databaseErr != nil && !errors.Is(databaseErr, sql.ErrNoRows) {
		return fmt.Errorf("recover Storage Location catalog lookup: %w", databaseErr)
	}
	if databaseErr == nil {
		return fmt.Errorf("%w: Storage Location row exists but disk identity is invalid", ErrLifecycleRecoveryRequired)
	}

	rollback := createStorageLocationRollbackData{Path: payload.Path}
	if operation.RollbackData != nil {
		if err := json.Unmarshal(*operation.RollbackData, &rollback); err != nil {
			return fmt.Errorf("%w: decode Storage Location rollback data: %v", ErrLifecycleRecoveryRequired, err)
		}
	}
	if diskErr == nil && diskConfig.ID != rootID.String() {
		return fmt.Errorf("%w: Storage Location path now carries a different identity", ErrLifecycleRecoveryRequired)
	}
	if rollback.MarkerCreated {
		if err := os.Remove(filepath.Join(rollback.Path, rootcfg.FileName)); err != nil && !errors.Is(err, os.ErrNotExist) {
			_ = rm.recoverLifecycleOperationFailure(ctx, operation.OperationID, false, err, rollback)
			return fmt.Errorf("%w: rollback Storage Location marker: %v", ErrLifecycleRecoveryRequired, err)
		}
	}
	return rm.recoverLifecycleOperationFailure(ctx, operation.OperationID, true, errors.New("rolled back incomplete Storage Location registration"), rollback)
}

func (rm *DefaultRepositoryManager) recoverCreateRepositoryOperation(ctx context.Context, operation repo.LifecycleOperation) error {
	var payload createRepositoryOperationPayload
	if err := json.Unmarshal(operation.Payload, &payload); err != nil {
		return fmt.Errorf("%w: decode create repository payload: %v", ErrLifecycleRecoveryRequired, err)
	}
	if operation.TargetID == nil {
		return fmt.Errorf("%w: create repository target ID is missing", ErrLifecycleRecoveryRequired)
	}
	repositoryID, err := uuid.Parse(*operation.TargetID)
	if err != nil {
		return fmt.Errorf("%w: invalid create repository target ID", ErrLifecycleRecoveryRequired)
	}
	databaseRepository, databaseErr := rm.queries.GetRepository(ctx, repositoryID)
	diskConfig, diskErr := repocfg.LoadConfigFromFile(payload.Path)
	if databaseErr == nil && diskErr == nil && diskConfig.ID == repositoryID.String() && databaseRepository.Path == payload.Path {
		return rm.completeRecoveredLifecycleOperation(ctx, operation.OperationID, createRepositoryOperationResult{RepositoryID: repositoryID.String()})
	}
	if databaseErr != nil && !errors.Is(databaseErr, sql.ErrNoRows) {
		return fmt.Errorf("recover create repository catalog lookup: %w", databaseErr)
	}
	if databaseErr == nil {
		return fmt.Errorf("%w: repository row exists but disk identity is invalid", ErrLifecycleRecoveryRequired)
	}

	rollback := createRepositoryRollbackData{Path: payload.Path}
	if operation.RollbackData != nil {
		if err := json.Unmarshal(*operation.RollbackData, &rollback); err != nil {
			return fmt.Errorf("%w: decode create repository rollback data: %v", ErrLifecycleRecoveryRequired, err)
		}
	}
	if diskErr == nil && diskConfig.ID != repositoryID.String() {
		return fmt.Errorf("%w: create target now carries a different repository identity", ErrLifecycleRecoveryRequired)
	}
	if _, statErr := os.Stat(rollback.Path); statErr == nil {
		if err := cleanupRepositoryInitializationTarget(rollback.Path, rollback.TargetCreated); err != nil {
			_ = rm.recoverLifecycleOperationFailure(ctx, operation.OperationID, false, err, rollback)
			return fmt.Errorf("%w: rollback incomplete create: %v", ErrLifecycleRecoveryRequired, err)
		}
	}
	return rm.recoverLifecycleOperationFailure(ctx, operation.OperationID, true, errors.New("rolled back incomplete repository creation"), rollback)
}

func lifecycleReplayError(operation repo.LifecycleOperation) error {
	switch operation.Status {
	case lifecycleStatusRunning:
		return ErrLifecycleOperationRunning
	case lifecycleStatusFailed, lifecycleStatusRolledBack:
		if operation.Error != nil {
			return fmt.Errorf("%w: %s", ErrLifecycleOperationFailed, *operation.Error)
		}
		return ErrLifecycleOperationFailed
	default:
		return nil
	}
}

func marshalLifecycleJSON(value any) (dbtypes.JSON, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return dbtypes.JSON(encoded), nil
}

func marshalOptionalLifecycleJSON(value any) (*dbtypes.JSON, error) {
	if value == nil {
		return nil, nil
	}
	encoded, err := marshalLifecycleJSON(value)
	if err != nil {
		return nil, err
	}
	return &encoded, nil
}
