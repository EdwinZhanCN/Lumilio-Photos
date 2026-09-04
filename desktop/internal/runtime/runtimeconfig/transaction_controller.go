package runtimeconfig

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"desktop/internal/control"
	"desktop/internal/control/dto"
	"desktop/internal/operation"
	"desktop/internal/state"
)

const transactionBudget = 45 * time.Second

type Lifecycle interface {
	Snapshot() dto.DesktopSnapshot
	Restart(string, uint64) (dto.OperationReceipt, error)
}

// stagedLifecycle is implemented by the real RuntimeController. Quiesce is
// used for config transactions instead of Stop because applying a candidate
// must preserve the user's desired running state across the stop/start gap.
type stagedLifecycle interface {
	Quiesce(string, uint64) (dto.OperationReceipt, error)
	Start(string, uint64) (dto.OperationReceipt, error)
}

type configuredLifecycle interface {
	SetConfigured(bool)
	Start(string, uint64) (dto.OperationReceipt, error)
}

// TransactionController owns the strict intent/pointer/journal transaction.
// It only calls the lifecycle controller through its typed surface; no
// service or frontend path can write runtime files directly.
type TransactionController struct {
	store      *Store
	state      *state.Store
	operations *operation.Registry
	lifecycle  Lifecycle
}

func NewTransactionController(store *Store, snapshotStore *state.Store, operations *operation.Registry, lifecycle Lifecycle) *TransactionController {
	if snapshotStore == nil {
		snapshotStore = state.New()
	}
	if operations == nil {
		operations = operation.New()
	}
	return &TransactionController{store: store, state: snapshotStore, operations: operations, lifecycle: lifecycle}
}

func (c *TransactionController) ReadDraft() (dto.RuntimeConfigDraft, error) {
	if c == nil || c.store == nil {
		return dto.RuntimeConfigDraft{}, errors.New("runtime config store is unavailable")
	}
	return c.store.ReadDraft()
}

func (c *TransactionController) PatchDraft(candidate string, settings dto.RuntimeConfigSettings) (dto.RuntimeConfigDraft, error) {
	if c == nil || c.store == nil {
		return dto.RuntimeConfigDraft{}, errors.New("runtime config store is unavailable")
	}
	return c.store.PatchDraft(candidate, settings)
}

func (c *TransactionController) Validate(candidate string) (dto.ConfigValidation, error) {
	if strings.TrimSpace(candidate) == "" {
		return dto.ConfigValidation{Issues: []dto.ConfigIssue{{Path: "", Message: "runtime configuration is required"}}}, nil
	}
	validation, err := c.validate(candidate)
	if err != nil {
		return dto.ConfigValidation{Issues: []dto.ConfigIssue{{Path: "", Message: redactConfigError(err)}}}, nil
	}
	return dto.ConfigValidation{Valid: true, Fingerprint: validation.Fingerprint}, nil
}

func (c *TransactionController) Save(requestID string, expectedVersion uint64, baseFingerprint, candidate string) (dto.OperationReceipt, error) {
	if existing, ok := c.operations.ReceiptForRequest(requestID); ok {
		return existing, nil
	}
	validation, err := c.validate(candidate)
	if err != nil {
		return dto.OperationReceipt{}, operation.NewError(dto.ErrorInvalidArgument, redactConfigError(err))
	}
	if err := c.checkVersion(expectedVersion, baseFingerprint); err != nil {
		return dto.OperationReceipt{}, err
	}
	snapshot := c.state.Get()
	if snapshot.Runtime.Phase != dto.RuntimeStopped && snapshot.Runtime.Phase != dto.RuntimeFailed {
		return dto.OperationReceipt{}, operation.NewError(dto.ErrorOperationConflict, "save configuration requires a stopped runtime")
	}
	receipt, err := c.accept(requestID, snapshot.Runtime.Version)
	if err != nil {
		return dto.OperationReceipt{}, err
	}
	c.commitRuntime(func(runtime *dto.RuntimeSnapshot) {
		runtime.Phase = dto.RuntimeSavingConfig
		runtime.Version++
	})
	if err := c.store.WriteIntent(validation); err != nil {
		return c.fail(receipt, err)
	}
	current, err := c.store.CurrentPointer()
	if err != nil {
		return c.fail(receipt, err)
	}
	if current.Fingerprint != "" {
		lkg, lkgErr := c.store.LastKnownGoodPointer()
		if lkgErr != nil {
			return c.fail(receipt, lkgErr)
		}
		if lkg.Fingerprint == "" {
			if err := c.store.WritePointer(c.store.Paths().RuntimeLKG, current.Fingerprint); err != nil {
				return c.fail(receipt, err)
			}
		}
	}
	if err := c.store.WritePointer(c.store.Paths().RuntimeCurrent, validation.Fingerprint); err != nil {
		return c.fail(receipt, err)
	}
	if !snapshot.Runtime.Configured {
		settings, err := LoadSettings(c.store.Paths().SettingsFile)
		if err != nil {
			return c.fail(receipt, err)
		}
		settings.OnboardingComplete = true
		settings.RuntimeDesiredState = dto.DesiredRunning
		if err := SaveSettings(c.store.Paths().SettingsFile, settings); err != nil {
			return c.fail(receipt, err)
		}
	}
	c.commitRuntime(func(runtime *dto.RuntimeSnapshot) {
		runtime.PendingConfigValidation = true
		runtime.Phase = dto.RuntimeStopped
		runtime.Version++
	})
	if !snapshot.Runtime.Configured {
		if lifecycle, ok := c.lifecycle.(configuredLifecycle); ok {
			lifecycle.SetConfigured(true)
			if _, err := lifecycle.Start("onboarding-runtime-"+receipt.OperationID, 0); err != nil {
				c.commitRuntime(func(runtime *dto.RuntimeSnapshot) { runtime.RecoveryCause = operation.ErrorCodeOf(err) })
			}
		}
	}
	return c.succeed(receipt)
}

func (c *TransactionController) Apply(requestID string, expectedVersion uint64, baseFingerprint, candidate string) (dto.OperationReceipt, error) {
	if existing, ok := c.operations.ReceiptForRequest(requestID); ok {
		return existing, nil
	}
	validation, err := c.validate(candidate)
	if err != nil {
		return dto.OperationReceipt{}, operation.NewError(dto.ErrorInvalidArgument, redactConfigError(err))
	}
	if err := c.checkVersion(expectedVersion, baseFingerprint); err != nil {
		return dto.OperationReceipt{}, err
	}
	snapshot := c.state.Get()
	if snapshot.Runtime.Phase != dto.RuntimeRunning || snapshot.Runtime.Ownership != dto.OwnershipHeld {
		return dto.OperationReceipt{}, operation.NewError(dto.ErrorOperationConflict, "apply configuration requires a running runtime")
	}
	receipt, err := c.accept(requestID, snapshot.Runtime.Version)
	if err != nil {
		return dto.OperationReceipt{}, err
	}
	current, err := c.store.CurrentPointer()
	if err != nil {
		return c.fail(receipt, err)
	}
	if err := c.store.WriteIntent(validation); err != nil {
		return c.fail(receipt, err)
	}
	previous := current.Fingerprint
	journal := Journal{
		OperationID: receipt.OperationID, Mode: "apply", Phase: PhasePrepared,
		PreviousFingerprint: previous, CandidateFingerprint: validation.Fingerprint,
	}
	if err := c.store.WriteJournal(journal); err != nil {
		return c.fail(receipt, err)
	}
	c.commitRuntime(func(runtime *dto.RuntimeSnapshot) {
		runtime.Phase = dto.RuntimeApplyingConfig
		runtime.PendingConfigValidation = true
		runtime.Version++
	})

	if c.lifecycle == nil {
		return c.rollback(receipt, previous, operation.NewError(dto.ErrorRuntimeNotReady, "runtime lifecycle is unavailable"))
	}

	if lifecycle, ok := c.lifecycle.(stagedLifecycle); ok {
		journal.Phase = PhaseStoppingPrevious
		if err := c.store.WriteJournal(journal); err != nil {
			return c.fail(receipt, err)
		}
		if _, err := lifecycle.Quiesce("config-quiesce-"+receipt.OperationID, c.state.Get().Runtime.Version); err != nil {
			return c.fail(receipt, err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), transactionBudget)
		err = c.waitStopped(ctx)
		cancel()
		if err != nil {
			return c.fail(receipt, err)
		}

		journal.Phase = PhasePreviousStopped
		if err := c.store.WriteJournal(journal); err != nil {
			return c.fail(receipt, err)
		}
		if err := c.store.WritePointer(c.store.Paths().RuntimeCurrent, validation.Fingerprint); err != nil {
			return c.fail(receipt, err)
		}
		journal.Phase = PhaseCandidateSelected
		if err := c.store.WriteJournal(journal); err != nil {
			return c.fail(receipt, err)
		}
		startVersion := c.state.Get().Runtime.Version
		if _, err := lifecycle.Start("config-start-"+receipt.OperationID, startVersion); err != nil {
			return c.rollbackCandidate(receipt, journal, err)
		}
		ctx, cancel = context.WithTimeout(context.Background(), transactionBudget)
		err = c.waitRunningFrom(ctx, startVersion)
		cancel()
		if err != nil {
			return c.rollbackCandidate(receipt, journal, err)
		}
	} else {
		// Keep a narrow compatibility path for test doubles and older host
		// adapters. The real controller always implements stagedLifecycle, so
		// production transactions still preserve every journal boundary.
		if err := c.store.WritePointer(c.store.Paths().RuntimeCurrent, validation.Fingerprint); err != nil {
			return c.fail(receipt, err)
		}
		journal.Phase = PhaseCandidateSelected
		if err := c.store.WriteJournal(journal); err != nil {
			return c.fail(receipt, err)
		}
		startVersion := c.state.Get().Runtime.Version
		if _, err := c.lifecycle.Restart("config-restart-"+receipt.OperationID, startVersion); err != nil {
			return c.rollbackCandidate(receipt, journal, err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), transactionBudget)
		err = c.waitRunningFrom(ctx, startVersion)
		cancel()
		if err != nil {
			return c.rollbackCandidate(receipt, journal, err)
		}
	}
	journal.Phase = PhaseCommitting
	if err := c.store.WriteJournal(journal); err != nil {
		return c.fail(receipt, err)
	}
	if err := c.store.WritePointer(c.store.Paths().RuntimeLKG, validation.Fingerprint); err != nil {
		return c.fail(receipt, err)
	}
	_ = c.store.ClearJournal()
	c.commitRuntime(func(runtime *dto.RuntimeSnapshot) {
		runtime.PendingConfigValidation = false
		runtime.Phase = dto.RuntimeRunning
		if snapshot.Runtime.Phase != dto.RuntimeRunning {
			runtime.Phase = dto.RuntimeStopped
		}
		runtime.Version++
	})
	return c.succeed(receipt)
}

func (c *TransactionController) rollbackCandidate(receipt dto.OperationReceipt, journal Journal, cause error) (dto.OperationReceipt, error) {
	// A failed candidate that still owns a process cannot be replaced under it.
	// Preserve the journal and pointer for explicit recovery instead of guessing
	// that the process tree is gone.
	if snapshot := c.state.Get().Runtime; snapshot.Ownership == dto.OwnershipHeld {
		journal.Phase = PhaseStoppingCandidate
		_ = c.store.WriteJournal(journal)
		return c.fail(receipt, cause)
	}
	journal.Phase = PhaseStoppingCandidate
	if err := c.store.WriteJournal(journal); err != nil {
		return c.fail(receipt, err)
	}
	if journal.PreviousFingerprint != "" {
		if err := c.store.WritePointer(c.store.Paths().RuntimeCurrent, journal.PreviousFingerprint); err != nil {
			return c.fail(receipt, err)
		}
	}
	journal.Phase = PhaseRollbackSelected
	if err := c.store.WriteJournal(journal); err != nil {
		return c.fail(receipt, err)
	}
	if lifecycle, ok := c.lifecycle.(stagedLifecycle); ok && journal.PreviousFingerprint != "" {
		startVersion := c.state.Get().Runtime.Version
		if _, err := lifecycle.Start("config-rollback-"+receipt.OperationID, startVersion); err != nil {
			return c.fail(receipt, err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), transactionBudget)
		err := c.waitRunningFrom(ctx, startVersion)
		cancel()
		if err != nil {
			return c.fail(receipt, err)
		}
	}
	_ = c.store.ClearJournal()
	c.commitRuntime(func(runtime *dto.RuntimeSnapshot) {
		runtime.Phase = dto.RuntimeRunning
		runtime.PendingConfigValidation = true
		runtime.RecoveryCause = operation.ErrorCodeOf(cause)
		runtime.Version++
	})
	return c.fail(receipt, cause)
}

func (c *TransactionController) RestoreLastKnownGood(requestID string, expectedVersion uint64) (dto.OperationReceipt, error) {
	if existing, ok := c.operations.ReceiptForRequest(requestID); ok {
		return existing, nil
	}
	snapshot := c.state.Get()
	if expectedVersion != 0 && expectedVersion != snapshot.Runtime.Version {
		return dto.OperationReceipt{}, operation.NewError(dto.ErrorStaleVersion, "runtime snapshot version is stale")
	}
	if snapshot.Runtime.Ownership == dto.OwnershipHeld {
		return dto.OperationReceipt{}, operation.NewError(dto.ErrorStopTimeout, "runtime ownership must be released before restoring configuration")
	}
	lkg, err := c.store.LastKnownGoodPointer()
	if err != nil || lkg.Fingerprint == "" {
		return dto.OperationReceipt{}, operation.NewError(dto.ErrorRecoveryRequired, "last known-good runtime configuration is unavailable")
	}
	receipt, err := c.accept(requestID, snapshot.Runtime.Version)
	if err != nil {
		return dto.OperationReceipt{}, err
	}
	if err := c.store.WritePointer(c.store.Paths().RuntimeCurrent, lkg.Fingerprint); err != nil {
		return c.fail(receipt, err)
	}
	_ = c.store.ClearJournal()
	c.commitRuntime(func(runtime *dto.RuntimeSnapshot) {
		runtime.PendingConfigValidation = false
		runtime.RecoveryCause = ""
		runtime.Version++
	})
	if lifecycle, ok := c.lifecycle.(configuredLifecycle); ok {
		lifecycle.SetConfigured(true)
	}
	return c.succeed(receipt)
}

func (c *TransactionController) validate(candidate string) (Validation, error) {
	if c == nil || c.store == nil {
		return Validation{}, errors.New("runtime config store is unavailable")
	}
	path := c.store.Paths().RuntimeIntents + "/candidate.toml"
	return c.store.Validate(path, []byte(candidate))
}

func (c *TransactionController) checkVersion(expectedVersion uint64, baseFingerprint string) error {
	snapshot := c.state.Get()
	if expectedVersion != 0 && expectedVersion != snapshot.Runtime.Version {
		return operation.NewError(dto.ErrorStaleVersion, "runtime snapshot version is stale")
	}
	current, err := c.store.CurrentPointer()
	if err != nil {
		return operation.NewError(dto.ErrorStaleConfig, "current runtime configuration is unavailable")
	}
	if current.Fingerprint != strings.TrimSpace(baseFingerprint) {
		return operation.NewError(dto.ErrorStaleConfig, "runtime configuration changed since it was read")
	}
	return nil
}

func (c *TransactionController) accept(requestID string, version uint64) (dto.OperationReceipt, error) {
	if existing, ok := c.operations.ReceiptForRequest(requestID); ok {
		return existing, nil
	}
	receipt, err := c.operations.Accept(requestID, "runtime-config", version, true)
	if err != nil {
		return dto.OperationReceipt{}, err
	}
	_ = c.operations.MarkRunning(receipt.OperationID)
	c.syncOperations()
	return receipt, nil
}

func (c *TransactionController) succeed(receipt dto.OperationReceipt) (dto.OperationReceipt, error) {
	_ = c.operations.Succeed(receipt.OperationID)
	c.syncOperations()
	return receipt, nil
}

func (c *TransactionController) fail(receipt dto.OperationReceipt, err error) (dto.OperationReceipt, error) {
	wrapped := operation.WithOperation(operation.NewError(dto.ErrorRecoveryRequired, redactConfigError(err)), receipt.OperationID)
	_ = c.operations.Fail(receipt.OperationID, wrapped)
	c.commitRuntime(func(runtime *dto.RuntimeSnapshot) {
		if runtime.Phase == dto.RuntimeSavingConfig || runtime.Phase == dto.RuntimeApplyingConfig {
			runtime.Phase = dto.RuntimeFailed
			runtime.PendingConfigValidation = true
			runtime.RecoveryCause = operation.ErrorCodeOf(wrapped)
			runtime.Version++
		}
	})
	c.syncOperations()
	return dto.OperationReceipt{}, wrapped
}

func (c *TransactionController) rollback(receipt dto.OperationReceipt, previous string, cause error) (dto.OperationReceipt, error) {
	if previous != "" {
		_ = c.store.WritePointer(c.store.Paths().RuntimeCurrent, previous)
	}
	_ = c.store.ClearJournal()
	c.commitRuntime(func(runtime *dto.RuntimeSnapshot) {
		runtime.Phase = dto.RuntimeFailed
		runtime.PendingConfigValidation = true
		runtime.RecoveryCause = operation.ErrorCodeOf(cause)
		runtime.Version++
	})
	return c.fail(receipt, cause)
}

func (c *TransactionController) waitRunningFrom(ctx context.Context, startVersion uint64) error {
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		snapshot := c.lifecycle.Snapshot().Runtime
		if snapshot.Phase == dto.RuntimeRunning && snapshot.Ownership == dto.OwnershipHeld {
			return nil
		}
		if snapshot.Version > startVersion && snapshot.Phase == dto.RuntimeFailed {
			return operation.NewError(dto.ErrorRuntimeNotReady, "candidate runtime configuration failed to start")
		}
		select {
		case <-ctx.Done():
			return operation.NewError(dto.ErrorReadinessTimeout, "candidate runtime configuration did not become ready")
		case <-ticker.C:
		}
	}
}

func (c *TransactionController) waitStopped(ctx context.Context) error {
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		snapshot := c.lifecycle.Snapshot().Runtime
		if snapshot.Phase == dto.RuntimeStopped && snapshot.Ownership == dto.OwnershipNone {
			return nil
		}
		if snapshot.Ownership == dto.OwnershipHeld && snapshot.Phase == dto.RuntimeFailed {
			return operation.NewError(dto.ErrorStopTimeout, "previous runtime ownership was not released")
		}
		select {
		case <-ctx.Done():
			return operation.NewError(dto.ErrorStopTimeout, "previous runtime did not stop before the transaction budget")
		case <-ticker.C:
		}
	}
}

func (c *TransactionController) commitRuntime(update func(*dto.RuntimeSnapshot)) {
	c.state.Commit(func(snapshot *dto.DesktopSnapshot) {
		update(&snapshot.Runtime)
		*snapshot = control.ProjectCapabilities(*snapshot)
	})
}

func (c *TransactionController) syncOperations() {
	items := c.operations.Snapshot()
	c.state.Commit(func(snapshot *dto.DesktopSnapshot) { snapshot.Operations = items })
}

func redactConfigError(err error) string {
	if err == nil {
		return ""
	}
	return fmt.Sprintf("invalid runtime configuration: %v", err)
}

var _ interface {
	ReadDraft() (dto.RuntimeConfigDraft, error)
	PatchDraft(string, dto.RuntimeConfigSettings) (dto.RuntimeConfigDraft, error)
	Validate(string) (dto.ConfigValidation, error)
	Save(string, uint64, string, string) (dto.OperationReceipt, error)
	Apply(string, uint64, string, string) (dto.OperationReceipt, error)
	RestoreLastKnownGood(string, uint64) (dto.OperationReceipt, error)
} = (*TransactionController)(nil)
