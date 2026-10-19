package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/storage/marker"
	"server/internal/storage/repocfg"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// SetupPrimaryState is the closed vocabulary of what the fixed Primary
// Repository target (<default>/primary) holds while first-run setup has not
// completed. Setup never guesses past it: each state names the one legal
// transition the authenticated administrator may take.
type SetupPrimaryState string

const (
	// SetupPrimaryCreatable: the target is absent or an empty directory on
	// supported storage; setup may create the Primary Repository there.
	SetupPrimaryCreatable SetupPrimaryState = "creatable"
	// SetupPrimaryAdoptable: a compatible, unregistered Repository marker
	// with its layout is at the target; setup may adopt it as the Primary
	// Repository, keeping its identity and originals.
	SetupPrimaryAdoptable SetupPrimaryState = "adoptable"
	// SetupPrimaryLayoutRepairable: a compatible, unregistered marker whose
	// private workspace or inbox is missing; adoption adds only the missing
	// directories, never touching the marker or the originals.
	SetupPrimaryLayoutRepairable SetupPrimaryState = "layout_repairable"
	// SetupPrimaryRegistered: the marker's identity is already registered in
	// this catalog under another role or path; setup cannot adopt it.
	SetupPrimaryRegistered SetupPrimaryState = "registered"
	// SetupPrimaryNonemptyUnmarked: files without a marker; setup never
	// initializes over them.
	SetupPrimaryNonemptyUnmarked SetupPrimaryState = "nonempty_unmarked"
	// SetupPrimaryMarkerInvalid: a marker this build cannot parse; its bytes
	// are preserved and never replaced.
	SetupPrimaryMarkerInvalid SetupPrimaryState = "marker_invalid"
	// SetupPrimaryMarkerUnsupported: a marker a newer build wrote.
	SetupPrimaryMarkerUnsupported SetupPrimaryState = "marker_unsupported"
	// SetupPrimaryUnavailable: the target cannot be observed, is denied, or
	// its storage is unsupported; the assessment's Problem names the cause.
	SetupPrimaryUnavailable SetupPrimaryState = "unavailable"
)

// Setup actions. FreshElsewhere is always legal before the Primary
// Repository exists: the Default Storage Location moves to another folder
// through the host's configuration (Desktop Settings or the manifest) and
// the old tree is left untouched.
const (
	SetupActionCreate         = "create"
	SetupActionAdopt          = "adopt"
	SetupActionFreshElsewhere = "fresh_elsewhere"
)

// SetupPrimaryExisting describes the Repository marker already at the target.
type SetupPrimaryExisting struct {
	RepositoryID   string
	Name           string
	MarkerVersion  string
	CreatedAt      time.Time
	RegisteredRole dbtypes.RepoRole
	RegisteredPath string
}

// SetupPrimaryRecoveryPreview counts the private state an adoption would
// preserve, by class, read from the tree without changing it.
type SetupPrimaryRecoveryPreview struct {
	TrashFiles    int
	TrashBytes    int64
	StudioRecords int
	StagingFiles  int
	UnknownFiles  int
	LogFiles      int
	DerivedFiles  int
	// Receipts counts earlier registrations' recovery receipts found under
	// the private workspace.
	Receipts int
}

// SetupPrimaryAssessment is the Server's review of the Primary Repository
// target for setup: the shared create-target assessment (token, support,
// risks, marker reading) plus the setup state, its legal actions, the
// existing identity, and the private state an adoption would preserve.
type SetupPrimaryAssessment struct {
	*CreateTargetAssessment
	State         SetupPrimaryState
	Actions       []string
	LayoutMissing []string
	Existing      *SetupPrimaryExisting
	Recovery      *SetupPrimaryRecoveryPreview
}

// Adoptable reports whether the explicit Adopt action is legal.
func (a *SetupPrimaryAssessment) Adoptable() bool {
	return a != nil && (a.State == SetupPrimaryAdoptable || a.State == SetupPrimaryLayoutRepairable)
}

var (
	// ErrSetupPrimaryNotAdoptable reports an adoption whose target is not in
	// an adoptable state on the facts sampled by the command.
	ErrSetupPrimaryNotAdoptable = errors.New("primary repository target is not adoptable")
	// ErrRepositoryLayoutObstructed reports a layout repair blocked by a
	// file where a directory belongs or by a denied directory. Nothing is
	// replaced; the user removes the obstruction or chooses elsewhere.
	ErrRepositoryLayoutObstructed = errors.New("repository layout repair is obstructed")
)

// AssessSetupPrimaryTarget reviews <default>/primary for the setup journey.
// It performs no mutation: the marker, the layout, and the private tree are
// read through the installed observer and plain reads.
func (rm *DefaultRepositoryManager) AssessSetupPrimaryTarget(ctx context.Context) (*SetupPrimaryAssessment, error) {
	location, err := rm.resolveStorageLocationForCreate(ctx, "", dbtypes.RepoRolePrimary)
	if err != nil {
		return nil, err
	}
	repoPath, err := resolveRepositoryCreatePath(location.Path, "primary", dbtypes.RepoRolePrimary)
	if err != nil {
		return nil, err
	}
	return rm.assessSetupPrimaryResolved(ctx, location, repoPath), nil
}

func (rm *DefaultRepositoryManager) assessSetupPrimaryResolved(ctx context.Context, location *repo.StorageLocation, repoPath string) *SetupPrimaryAssessment {
	base := rm.assessResolvedCreateTarget(ctx, IntentSetupPrimary, location, "primary", repoPath)
	result := &SetupPrimaryAssessment{CreateTargetAssessment: base}
	observed := base.Assessment.Observation
	classification := base.Assessment.Classification
	setState := func(state SetupPrimaryState, actions ...string) {
		result.State = state
		result.Actions = append(actions, SetupActionFreshElsewhere)
	}
	if classification.Problem != "" || classification.Support != StorageSupported {
		setState(SetupPrimaryUnavailable)
		return result
	}
	switch observed.Access.State {
	case AccessPresent, AccessAbsent:
	default:
		setState(SetupPrimaryUnavailable)
		return result
	}
	switch observed.RepositoryMarker.State {
	case marker.Absent:
		if base.Assessment.Capabilities.ReviewCreate {
			setState(SetupPrimaryCreatable, SetupActionCreate)
		} else {
			setState(SetupPrimaryNonemptyUnmarked)
		}
		return result
	case marker.Corrupt:
		setState(SetupPrimaryMarkerInvalid)
		return result
	case marker.UnsupportedVersion:
		setState(SetupPrimaryMarkerUnsupported)
		result.Existing = &SetupPrimaryExisting{RepositoryID: observed.RepositoryMarker.UUID, MarkerVersion: observed.RepositoryMarker.Version}
		return result
	case marker.Valid:
	default:
		setState(SetupPrimaryUnavailable)
		return result
	}

	existing := &SetupPrimaryExisting{RepositoryID: observed.RepositoryMarker.UUID, MarkerVersion: observed.RepositoryMarker.Version}
	if config, err := repocfg.LoadConfigFromFile(repoPath); err == nil {
		existing.Name, existing.CreatedAt = config.Name, config.CreatedAt
	}
	result.Existing = existing
	if registered, err := rm.GetRepository(existing.RepositoryID); err == nil && registered != nil {
		existing.RegisteredRole, existing.RegisteredPath = registered.Role, registered.Path
		setState(SetupPrimaryRegistered)
		return result
	}
	for _, directory := range []string{DefaultStructure.SystemDir, DefaultStructure.InboxDir} {
		reading, ok := observed.Layout[directory]
		switch {
		case !ok || reading.State == AccessAbsent:
			result.LayoutMissing = append(result.LayoutMissing, directory)
		case reading.State != AccessPresent:
			setState(SetupPrimaryUnavailable)
			return result
		}
	}
	result.Recovery = previewPrivateRecovery(repoPath, existing.RepositoryID)
	if len(result.LayoutMissing) > 0 {
		setState(SetupPrimaryLayoutRepairable, SetupActionAdopt)
	} else {
		setState(SetupPrimaryAdoptable, SetupActionAdopt)
	}
	return result
}

// previewPrivateRecovery runs the read-only half of private preservation
// and folds the plan into class counts. A tree that cannot be read yields an
// empty preview rather than a failed review; the adoption itself reports
// the exact cause.
func previewPrivateRecovery(repoPath, repositoryID string) *SetupPrimaryRecoveryPreview {
	preview := &SetupPrimaryRecoveryPreview{}
	plan, err := planPrivatePreservation(repoPath, RecoveryEventAdopt, "", "", repositoryID, repositoryID, time.Now())
	if err != nil || plan.Receipt == nil {
		return preview
	}
	receipt := plan.Receipt
	preview.TrashFiles, preview.TrashBytes = receipt.Trash.Files, receipt.Trash.Bytes
	preview.LogFiles, preview.DerivedFiles = receipt.Logs.Files, receipt.Derived.Files
	for _, item := range receipt.Items {
		switch item.Class {
		case PrivateClassStudio:
			preview.StudioRecords++
		case PrivateClassStaging:
			preview.StagingFiles++
		default:
			preview.UnknownFiles++
		}
	}
	preview.UnknownFiles += receipt.ItemsTruncated
	if receipts, globErr := filepath.Glob(filepath.Join(repoPath, filepath.FromSlash(RepositoryRecoveryDirectory), "*.json")); globErr == nil {
		preview.Receipts = len(receipts)
	}
	return preview
}

// AdoptPrimarySpec is the explicit adoption command. ExpectedRepositoryID is
// the identity the administrator reviewed; AssessmentToken and
// AcknowledgedRisks bind the command to that review exactly as a create does.
type AdoptPrimarySpec struct {
	RequestID            string
	Actor                string
	ActorUserID          *int32
	HostInstanceID       string
	OwnerID              *int32
	ExpectedRepositoryID string
	AssessmentToken      string
	AcknowledgedRisks    []string
}

// AdoptPrimaryResult carries the adopted Primary Repository, the risks the
// administrator acknowledged, the recovery receipt the adoption wrote (nil
// when the tree carried no private state), and the layout directories the
// adoption added.
type AdoptPrimaryResult struct {
	Repository        *repo.Repository
	AcknowledgedRisks []StorageRisk
	ReceiptID         string
	Receipt           *RecoveryReceipt
	LayoutRepaired    []string
	Replayed          bool
}

type adoptPrimaryOperationPayload struct {
	Path              string   `json:"path"`
	StorageLocationID string   `json:"storage_location_id"`
	RepositoryID      string   `json:"repository_id"`
	OwnerID           *int32   `json:"owner_id,omitempty"`
	AcknowledgedRisks []string `json:"acknowledged_risks,omitempty"`
	LayoutRepair      []string `json:"layout_repair,omitempty"`
}

type adoptPrimaryRollbackData struct {
	Preservation privatePreservationPlan `json:"preservation"`
	// InboxCreated records that the repair created inbox/, so a rollback
	// removes it again (only while it is still empty).
	InboxCreated bool `json:"inbox_created,omitempty"`
}

type adoptPrimaryOperationResult struct {
	RepositoryID      string   `json:"repository_id"`
	ReceiptID         string   `json:"receipt_id,omitempty"`
	LayoutRepaired    []string `json:"layout_repaired,omitempty"`
	InitialScanQueued bool     `json:"initial_scan_queued"`
}

// AdoptPrimaryRepository registers the compatible Repository already at
// <default>/primary as this catalog's Primary Repository. The marker and
// the originals are not written; the private state is preserved by class
// under an adopt receipt (P4), a missing layout is completed with empty
// directories only, and the registration is journaled so a crash at any
// point rolls back to the tree the user had.
func (rm *DefaultRepositoryManager) AdoptPrimaryRepository(ctx context.Context, spec AdoptPrimarySpec) (*AdoptPrimaryResult, error) {
	location, err := rm.resolveStorageLocationForCreate(ctx, "", dbtypes.RepoRolePrimary)
	if err != nil {
		return nil, err
	}
	repoPath, err := resolveRepositoryCreatePath(location.Path, "primary", dbtypes.RepoRolePrimary)
	if err != nil {
		return nil, err
	}
	review := rm.assessSetupPrimaryResolved(ctx, location, repoPath)
	acknowledged, err := rm.admitAssessedTarget(ctx, consentAdmission{
		Actor: spec.Actor, ActorUserID: spec.ActorUserID, HostInstanceID: spec.HostInstanceID,
		RequestID: spec.RequestID, Action: lifecycleKindAdoptPrimaryRepository,
		AssessmentToken: spec.AssessmentToken, AcknowledgedRisks: spec.AcknowledgedRisks, TokenRequired: true,
	}, review.CreateTargetAssessment)
	if err != nil {
		return nil, err
	}
	expected := strings.TrimSpace(spec.ExpectedRepositoryID)
	if expected == "" || review.Existing == nil || !strings.EqualFold(expected, review.Existing.RepositoryID) {
		return nil, fmt.Errorf("%w: the Repository identity at the target is not the one reviewed", ErrStorageAssessmentStale)
	}
	repositoryID, err := uuid.Parse(review.Existing.RepositoryID)
	if err != nil {
		return nil, fmt.Errorf("invalid repository identity: %w", err)
	}

	// The journal comes before the state checks so a replay of a completed
	// adoption answers from its result even though the target is now
	// registered; a refused first attempt records its cause under the same
	// request id and replays that refusal.
	targetID := repositoryID.String()
	operation, replay, err := rm.beginLifecycleOperation(ctx, lifecycleBeginInput{
		RequestID: spec.RequestID,
		Kind:      lifecycleKindAdoptPrimaryRepository,
		Payload: adoptPrimaryOperationPayload{
			Path: repoPath, StorageLocationID: location.StorageLocationID.String(), RepositoryID: targetID,
			OwnerID: spec.OwnerID, AcknowledgedRisks: RiskCodes(acknowledged), LayoutRepair: review.LayoutMissing,
		},
		Actor: spec.Actor, ActorUserID: spec.ActorUserID, HostInstanceID: spec.HostInstanceID,
		TargetType: "repository", TargetID: &targetID,
		RollbackData: adoptPrimaryRollbackData{},
	})
	if err != nil {
		return nil, err
	}
	if replay {
		if err := lifecycleReplayError(operation); err != nil {
			return nil, err
		}
		if operation.Result == nil {
			return nil, fmt.Errorf("%w: completed adoption has no result", ErrLifecycleRecoveryRequired)
		}
		var result adoptPrimaryOperationResult
		if err := json.Unmarshal(*operation.Result, &result); err != nil {
			return nil, fmt.Errorf("decode completed adoption result: %w", err)
		}
		adopted, err := rm.GetRepository(result.RepositoryID)
		if err != nil {
			return nil, fmt.Errorf("load adopted repository: %w", err)
		}
		return &AdoptPrimaryResult{
			Repository: adopted, AcknowledgedRisks: acknowledged, ReceiptID: result.ReceiptID,
			Receipt: readRecoveryReceiptFile(repoPath, result.ReceiptID), LayoutRepaired: result.LayoutRepaired, Replayed: true,
		}, nil
	}
	failPrepared := func(cause error) (*AdoptPrimaryResult, error) {
		_ = rm.failLifecycleOperation(ctx, operation.OperationID, true, cause, adoptPrimaryRollbackData{})
		return nil, cause
	}
	if !review.Adoptable() {
		if review.State == SetupPrimaryRegistered && review.Existing.RegisteredPath != "" {
			return failPrepared(&RepositoryConflictError{
				RepositoryID: targetID, RegisteredPath: review.Existing.RegisteredPath, RequestedPath: repoPath,
				Actions: repositoryConflictActions(review.Existing.RegisteredPath, targetID),
			})
		}
		return failPrepared(fmt.Errorf("%w: target is %s", ErrSetupPrimaryNotAdoptable, review.State))
	}
	if primaryExists, err := rm.primaryRepositoryExists(ctx); err != nil || primaryExists {
		if err != nil {
			return failPrepared(err)
		}
		return failPrepared(ErrPrimaryRepositoryExists)
	}
	if registered, lookupErr := rm.GetRepository(targetID); lookupErr == nil && registered != nil {
		return failPrepared(&RepositoryConflictError{
			RepositoryID: targetID, RegisteredPath: registered.Path, RequestedPath: repoPath,
			Actions: repositoryConflictActions(registered.Path, targetID),
		})
	} else if lookupErr != nil && !errors.Is(lookupErr, sql.ErrNoRows) {
		return failPrepared(fmt.Errorf("look up repository identity: %w", lookupErr))
	}
	if err := rm.claimRuntimeStoragePath(ctx, "repository", repoPath); err != nil {
		return failPrepared(fmt.Errorf("%w: %v", ErrRepositoryOwnershipUnavailable, err))
	}
	if err := requireMaterializableRepository(repoPath); err != nil {
		return failPrepared(err)
	}
	if err := layoutRepairObstruction(rm.dirManager, repoPath); err != nil {
		return failPrepared(err)
	}

	releaseStorageLocation := rm.acquireStorageLocationRead(location.StorageLocationID)
	releaseMutation := rm.acquireRepositoryMutation(repositoryID)
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

	plan, err := planPrivatePreservation(repoPath, RecoveryEventAdopt, operation.Actor, operation.RequestID, targetID, targetID, time.Now())
	if err != nil {
		return failPrepared(err)
	}
	inboxPath := filepath.Join(repoPath, DefaultStructure.InboxDir)
	_, inboxErr := os.Stat(inboxPath)
	rollbackData := adoptPrimaryRollbackData{Preservation: plan, InboxCreated: errors.Is(inboxErr, fs.ErrNotExist)}
	if err := rm.updateLifecycleOperationPhase(ctx, operation.OperationID, lifecyclePhasePrepared, rollbackData); err != nil {
		return failPrepared(fmt.Errorf("persist adoption rollback plan: %w", err))
	}
	rollback := func(cause error) error {
		rollbackErr := rollbackAdoptedLayout(repoPath, rollbackData)
		_ = rm.failLifecycleOperation(ctx, operation.OperationID, rollbackErr == nil, cause, rollbackData)
		if rollbackErr != nil {
			return errors.Join(cause, fmt.Errorf("rollback primary adoption: %w", rollbackErr))
		}
		return cause
	}
	if err := applyPrivatePreservation(repoPath, plan); err != nil {
		return nil, rollback(err)
	}
	if err := rm.dirManager.CreateStructure(repoPath); err != nil {
		return nil, rollback(fmt.Errorf("%w: %v", ErrRepositoryLayoutObstructed, err))
	}
	if err := rm.updateLifecycleOperationPhase(ctx, operation.OperationID, lifecyclePhaseFilesystemApplied, rollbackData); err != nil {
		return nil, rollback(fmt.Errorf("persist adoption filesystem phase: %w", err))
	}
	adopted, err := rm.addRepository(ctx, repoPath, spec.OwnerID, dbtypes.RepoRolePrimary, true, location.StorageLocationID)
	if err != nil {
		return nil, rollback(err)
	}
	if err := rm.updateLifecycleOperationPhase(ctx, operation.OperationID, lifecyclePhaseCatalogCommitted, rollbackData); err != nil {
		return nil, fmt.Errorf("primary adopted but journal commit phase failed: %w", err)
	}
	result := adoptPrimaryOperationResult{RepositoryID: adopted.RepoID.String(), ReceiptID: plan.ReceiptID(), LayoutRepaired: review.LayoutMissing}
	if err := rm.completeLifecycleOperation(ctx, operation.OperationID, result); err != nil {
		return nil, fmt.Errorf("primary adopted but journal completion failed: %w", err)
	}
	releaseLocks()
	if err := rm.ScheduleInitialRepositoryScan(ctx, adopted.RepoID.String()); err != nil {
		rm.logger.Warn("primary adopted but initial scan could not be queued",
			zap.String("operation", "repository.adopt.scan"),
			zap.String("repository_id", adopted.RepoID.String()),
			zap.Error(err))
	} else if err := rm.markInitialScanQueued(ctx, operation.OperationID); err != nil {
		return nil, err
	}
	rm.repoAudit(repoPath).Operation("repository.adopt",
		zap.String("repository_id", adopted.RepoID.String()),
		zap.String("recovery_receipt_id", plan.ReceiptID()),
		zap.Strings("layout_repaired", review.LayoutMissing))
	return &AdoptPrimaryResult{
		Repository: adopted, AcknowledgedRisks: acknowledged, ReceiptID: plan.ReceiptID(),
		Receipt: plan.Receipt, LayoutRepaired: review.LayoutMissing,
	}, nil
}

// layoutRepairObstruction refuses a repair that would need to replace or
// read through something that is not the expected directory. Missing
// directories are what the repair adds; anything else blocks it.
func layoutRepairObstruction(manager DirectoryManager, repoPath string) error {
	validation, err := manager.ValidateStructure(repoPath)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrRepositoryLayoutObstructed, err)
	}
	if len(validation.InvalidPaths) > 0 {
		return fmt.Errorf("%w: %s", ErrRepositoryLayoutObstructed, strings.Join(validation.InvalidPaths, "; "))
	}
	for _, issue := range validation.PermissionIssues {
		// A missing directory has no permissions to check yet; a present one
		// that cannot be read or written blocks the repair.
		if !strings.Contains(issue, "no such file") && !strings.Contains(issue, "cannot find") {
			return fmt.Errorf("%w: %s", ErrRepositoryLayoutObstructed, issue)
		}
	}
	return nil
}

// rollbackAdoptedLayout undoes what an adoption changed on disk: staging
// moves and a fresh private workspace through the preservation plan, and an
// inbox the repair created while it is still empty.
func rollbackAdoptedLayout(repoPath string, data adoptPrimaryRollbackData) error {
	if err := rollbackPrivateState(repoPath, data.Preservation); err != nil {
		return err
	}
	if data.InboxCreated {
		inboxPath := filepath.Join(repoPath, DefaultStructure.InboxDir)
		if err := os.Remove(inboxPath); err != nil && !errors.Is(err, fs.ErrNotExist) {
			if entries, readErr := os.ReadDir(inboxPath); readErr == nil && len(entries) > 0 {
				return nil
			}
			return fmt.Errorf("remove repaired inbox: %w", err)
		}
	}
	return nil
}

func readRecoveryReceiptFile(repoPath, receiptID string) *RecoveryReceipt {
	if receiptID == "" {
		return nil
	}
	data, err := os.ReadFile(filepath.Join(repoPath, filepath.FromSlash(RepositoryRecoveryDirectory), receiptID+".json"))
	if err != nil {
		return nil
	}
	receipt, err := ParseRecoveryReceipt(data)
	if err != nil {
		return nil
	}
	return &receipt
}

// recoverAdoptPrimaryOperation finishes or undoes an adoption a crash
// interrupted: a committed registration whose disk identity still matches
// is completed; anything else rolls the tree back and records the failure.
func (rm *DefaultRepositoryManager) recoverAdoptPrimaryOperation(ctx context.Context, operation repo.LifecycleOperation) error {
	var payload adoptPrimaryOperationPayload
	if err := json.Unmarshal(operation.Payload, &payload); err != nil {
		return fmt.Errorf("%w: decode adoption payload: %v", ErrLifecycleRecoveryRequired, err)
	}
	repositoryID, err := uuid.Parse(payload.RepositoryID)
	if err != nil {
		return fmt.Errorf("%w: invalid adoption target ID", ErrLifecycleRecoveryRequired)
	}
	databaseRepository, databaseErr := rm.queries.GetRepository(ctx, repositoryID)
	diskConfig, diskErr := repocfg.LoadConfigFromFile(payload.Path)
	rollback := adoptPrimaryRollbackData{}
	if operation.RollbackData != nil {
		if err := json.Unmarshal(*operation.RollbackData, &rollback); err != nil {
			return fmt.Errorf("%w: decode adoption rollback data: %v", ErrLifecycleRecoveryRequired, err)
		}
	}
	if databaseErr == nil && diskErr == nil && diskConfig.ID == repositoryID.String() && databaseRepository.Path == payload.Path {
		return rm.completeRecoveredLifecycleOperation(ctx, operation.OperationID,
			adoptPrimaryOperationResult{RepositoryID: repositoryID.String(), ReceiptID: rollback.Preservation.ReceiptID(), LayoutRepaired: payload.LayoutRepair})
	}
	if databaseErr != nil && !errors.Is(databaseErr, sql.ErrNoRows) {
		return fmt.Errorf("recover adoption catalog lookup: %w", databaseErr)
	}
	if databaseErr == nil {
		return fmt.Errorf("%w: adopted repository row exists but disk identity is invalid", ErrLifecycleRecoveryRequired)
	}
	if diskErr != nil {
		return fmt.Errorf("%w: adopted repository marker is unavailable", ErrLifecycleRecoveryRequired)
	}
	if diskConfig.ID != repositoryID.String() {
		return fmt.Errorf("%w: adopted repository path now carries a different identity", ErrLifecycleRecoveryRequired)
	}
	if err := rollbackAdoptedLayout(payload.Path, rollback); err != nil {
		_ = rm.recoverLifecycleOperationFailure(ctx, operation.OperationID, false, err, rollback)
		return fmt.Errorf("%w: restore adopted repository private state: %v", ErrLifecycleRecoveryRequired, err)
	}
	return rm.recoverLifecycleOperationFailure(ctx, operation.OperationID, true, errors.New("rolled back incomplete primary adoption"), rollback)
}
