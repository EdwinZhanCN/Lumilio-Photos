// Package shutdown is the only owner allowed to arm the Wails application for
// exit. Runtime and Lumen controllers are asked to quiesce through their typed
// interfaces; this package never creates processes or calls os.Exit.
package shutdown

import (
	"context"
	"sync"
	"time"

	"desktop/internal/control"
	"desktop/internal/control/dto"
	"desktop/internal/operation"
	"desktop/internal/runtime"
	"desktop/internal/state"
)

const (
	DefaultBudget = 45 * time.Second
	PollInterval  = 25 * time.Millisecond
)

type RuntimeOwner interface {
	RuntimeSnapshot() dto.RuntimeSnapshot
	Quiesce(string, uint64) (dto.OperationReceipt, error)
	Start(string, uint64) (dto.OperationReceipt, error)
}

type LumenOwner interface {
	LumenSnapshot() dto.LumenSnapshot
	Quiesce(string, uint64) (dto.OperationReceipt, error)
	Start(string, uint64) (dto.OperationReceipt, error)
}

type Coordinator struct {
	store      *state.Store
	operations *operation.Registry
	runtime    RuntimeOwner
	lumen      LumenOwner
	budget     time.Duration
	onArmed    func()

	mu         sync.Mutex
	active     dto.OperationReceipt
	shouldQuit bool
}

func New(store *state.Store, operations *operation.Registry, runtimeOwner RuntimeOwner, lumenOwner LumenOwner) *Coordinator {
	if store == nil {
		store = state.New()
	}
	if operations == nil {
		operations = operation.New()
	}
	return &Coordinator{store: store, operations: operations, runtime: runtimeOwner, lumen: lumenOwner, budget: DefaultBudget}
}

func (c *Coordinator) SetBudget(budget time.Duration) {
	if budget > 0 {
		c.budget = budget
	}
}

func (c *Coordinator) SetOnArmed(callback func()) {
	c.mu.Lock()
	c.onArmed = callback
	c.mu.Unlock()
}

func (c *Coordinator) ShouldQuit() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.shouldQuit
}

func (c *Coordinator) RequestQuit(requestID string, expectedVersion uint64) (dto.OperationReceipt, error) {
	if existing, ok := c.operations.ReceiptForRequest(requestID); ok {
		return existing, nil
	}
	c.mu.Lock()
	if c.active.OperationID != "" {
		active := c.active
		c.mu.Unlock()
		return active, nil
	}
	snapshot := c.store.Get()
	if expectedVersion != 0 && expectedVersion != snapshot.Revision {
		c.mu.Unlock()
		return dto.OperationReceipt{}, operation.NewError(dto.ErrorStaleVersion, "shutdown snapshot version is stale")
	}
	receipt, err := c.operations.Accept(requestID, "shutdown", snapshot.Revision, false)
	if err != nil {
		c.mu.Unlock()
		if active := c.activeReceipt(); active.OperationID != "" {
			return active, nil
		}
		return dto.OperationReceipt{}, err
	}
	c.active = receipt
	c.mu.Unlock()
	c.commitShutdown(dto.ShutdownQuiescing, receipt.OperationID, dto.Error{})
	c.syncOperations()
	go c.quiesce(receipt)
	return receipt, nil
}

func (c *Coordinator) ResumeAfterFailedShutdown(requestID string, expectedVersion uint64) (dto.OperationReceipt, error) {
	return c.resume(requestID, expectedVersion)
}

func (c *Coordinator) ForceQuit(requestID string, expectedVersion uint64, _ string) (dto.OperationReceipt, error) {
	if existing, ok := c.operations.ReceiptForRequest(requestID); ok {
		return existing, nil
	}
	if active := c.activeReceipt(); active.OperationID != "" {
		_ = c.operations.Succeed(active.OperationID)
		c.mu.Lock()
		c.shouldQuit = true
		c.mu.Unlock()
		c.commitShutdown(dto.ShutdownArmed, active.OperationID, dto.Error{Code: dto.ErrorRecoveryRequired, Message: "forced quit; graceful cleanup was not confirmed", OperationID: active.OperationID})
		c.syncOperations()
		c.notifyArmed()
		return active, nil
	}
	snapshot := c.store.Get()
	if expectedVersion != 0 && expectedVersion != snapshot.Revision {
		return dto.OperationReceipt{}, operation.NewError(dto.ErrorStaleVersion, "shutdown snapshot is stale")
	}
	receipt, err := c.operations.Accept(requestID, "shutdown", snapshot.Revision, false)
	if err != nil {
		if active := c.activeReceipt(); active.OperationID != "" {
			return active, nil
		}
		return dto.OperationReceipt{}, err
	}
	_ = c.operations.Succeed(receipt.OperationID)
	c.mu.Lock()
	c.active = receipt
	c.shouldQuit = true
	c.mu.Unlock()
	c.commitShutdown(dto.ShutdownArmed, receipt.OperationID, dto.Error{})
	c.syncOperations()
	c.notifyArmed()
	return receipt, nil
}

func (c *Coordinator) quiesce(receipt dto.OperationReceipt) {
	ctx, cancel := context.WithTimeout(context.Background(), c.budget)
	defer cancel()

	// Submit both high-priority quiesce requests together. The controllers
	// retain their own ownership and safety budgets; this coordinator only
	// waits for the aggregate ownership projections to settle.
	errCh := make(chan error, 2)
	var submissions int
	if c.runtime != nil {
		submissions++
		go func() {
			_, err := c.runtime.Quiesce("shutdown-runtime-"+receipt.OperationID, 0)
			errCh <- err
		}()
	}
	if c.lumen != nil {
		submissions++
		go func() {
			_, err := c.lumen.Quiesce("shutdown-lumen-"+receipt.OperationID, 0)
			errCh <- err
		}()
	}
	for i := 0; i < submissions; i++ {
		select {
		case err := <-errCh:
			if err != nil {
				c.fail(receipt, err)
				return
			}
		case <-ctx.Done():
			c.fail(receipt, operation.NewError(dto.ErrorStopTimeout, "quiesce requests could not be submitted"))
			return
		}
	}

	ticker := time.NewTicker(PollInterval)
	defer ticker.Stop()
	for {
		if c.ownersReleased() {
			_ = c.operations.Succeed(receipt.OperationID)
			c.mu.Lock()
			c.shouldQuit = true
			c.mu.Unlock()
			c.commitShutdown(dto.ShutdownArmed, receipt.OperationID, dto.Error{})
			c.syncOperations()
			c.notifyArmed()
			return
		}
		select {
		case <-ctx.Done():
			c.fail(receipt, operation.NewError(dto.ErrorStopTimeout, "Desktop owners did not quiesce within the shutdown budget"))
			return
		case <-ticker.C:
		}
	}
}

func (c *Coordinator) ownersReleased() bool {
	if c.runtime != nil {
		runtimeSnapshot := c.runtime.RuntimeSnapshot()
		if runtimeSnapshot.Ownership == dto.OwnershipHeld || (runtimeSnapshot.Phase != dto.RuntimeStopped && runtimeSnapshot.Phase != dto.RuntimeFailed) {
			return false
		}
	}
	if c.lumen != nil {
		lumenSnapshot := c.lumen.LumenSnapshot()
		if lumenSnapshot.Ownership == dto.OwnershipHeld || (lumenSnapshot.ProcessPhase != dto.LumenStopped && lumenSnapshot.ProcessPhase != dto.LumenFailed) {
			return false
		}
	}
	return true
}

func (c *Coordinator) resume(requestID string, expectedVersion uint64) (dto.OperationReceipt, error) {
	if existing, ok := c.operations.ReceiptForRequest(requestID); ok {
		return existing, nil
	}
	snapshot := c.store.Get()
	if snapshot.Host.Shutdown.Phase != dto.ShutdownFailed {
		return dto.OperationReceipt{}, operation.NewError(dto.ErrorInvalidArgument, "shutdown is not in recovery")
	}
	if expectedVersion != 0 && expectedVersion != snapshot.Revision {
		return dto.OperationReceipt{}, operation.NewError(dto.ErrorStaleVersion, "shutdown snapshot is stale")
	}
	receipt, err := c.operations.Accept(requestID, "shutdown", snapshot.Revision, false)
	if err != nil {
		return dto.OperationReceipt{}, err
	}
	c.mu.Lock()
	c.active = receipt
	c.mu.Unlock()
	if !c.ownersReleased() {
		c.fail(receipt, operation.NewError(dto.ErrorStopTimeout, "owned process cleanup is still required"))
		return receipt, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), c.budget)
	defer cancel()
	if err := c.resumeOwners(ctx, receipt.OperationID); err != nil {
		c.fail(receipt, err)
		return receipt, nil
	}
	_ = c.operations.Succeed(receipt.OperationID)
	c.mu.Lock()
	c.active = dto.OperationReceipt{}
	c.shouldQuit = false
	c.mu.Unlock()
	c.commitShutdown(dto.ShutdownIdle, "", dto.Error{})
	c.syncOperations()
	return receipt, nil
}

func (c *Coordinator) fail(receipt dto.OperationReceipt, err error) {
	_ = c.operations.Fail(receipt.OperationID, err)
	c.mu.Lock()
	c.shouldQuit = false
	if c.active.OperationID == receipt.OperationID {
		c.active = dto.OperationReceipt{}
	}
	c.mu.Unlock()
	c.commitShutdown(dto.ShutdownFailed, receipt.OperationID, dto.Error{Code: operation.ErrorCodeOf(err), Message: err.Error(), OperationID: receipt.OperationID})
	c.syncOperations()
}

func (c *Coordinator) activeReceipt() dto.OperationReceipt {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.active
}

func (c *Coordinator) resumeOwners(ctx context.Context, operationID string) error {
	if c.runtime != nil {
		runtimeSnapshot := c.runtime.RuntimeSnapshot()
		if runtimeSnapshot.DesiredState == dto.DesiredRunning {
			if _, err := c.runtime.Start("resume-runtime-"+operationID, runtimeSnapshot.Version); err != nil {
				return err
			}
			if err := waitForRuntime(ctx, c.runtime, true); err != nil {
				return err
			}
		}
	}
	if c.lumen != nil {
		lumenSnapshot := c.lumen.LumenSnapshot()
		if lumenSnapshot.DesiredState == dto.DesiredRunning {
			if _, err := c.lumen.Start("resume-lumen-"+operationID, lumenSnapshot.Version); err != nil {
				return err
			}
			if err := waitForLumen(ctx, c.lumen, true); err != nil {
				return err
			}
		}
	}
	return nil
}

func waitForRuntime(ctx context.Context, owner RuntimeOwner, running bool) error {
	ticker := time.NewTicker(PollInterval)
	defer ticker.Stop()
	for {
		snapshot := owner.RuntimeSnapshot()
		if running && snapshot.Phase == dto.RuntimeRunning && snapshot.Ownership == dto.OwnershipHeld {
			return nil
		}
		if !running && snapshot.Phase == dto.RuntimeStopped && snapshot.Ownership == dto.OwnershipNone {
			return nil
		}
		if snapshot.Phase == dto.RuntimeFailed {
			return operation.NewError(dto.ErrorRuntimeNotReady, "runtime failed while resuming after shutdown")
		}
		select {
		case <-ctx.Done():
			return operation.NewError(dto.ErrorReadinessTimeout, "runtime did not become ready while resuming after shutdown")
		case <-ticker.C:
		}
	}
}

func waitForLumen(ctx context.Context, owner LumenOwner, running bool) error {
	ticker := time.NewTicker(PollInterval)
	defer ticker.Stop()
	for {
		snapshot := owner.LumenSnapshot()
		if running && snapshot.ProcessPhase == dto.LumenRunning && snapshot.Ownership == dto.OwnershipHeld {
			return nil
		}
		if !running && snapshot.ProcessPhase == dto.LumenStopped && snapshot.Ownership == dto.OwnershipNone {
			return nil
		}
		if snapshot.ProcessPhase == dto.LumenFailed {
			return operation.NewError(dto.ErrorRuntimeNotReady, "Lumen failed while resuming after shutdown")
		}
		select {
		case <-ctx.Done():
			return operation.NewError(dto.ErrorReadinessTimeout, "Lumen did not become ready while resuming after shutdown")
		case <-ticker.C:
		}
	}
}

func (c *Coordinator) commitShutdown(phase dto.ShutdownPhase, operationID string, itemError dto.Error) {
	c.store.Commit(func(snapshot *dto.DesktopSnapshot) {
		snapshot.Host.Shutdown = dto.ShutdownSnapshot{Phase: phase, OperationID: operationID, Error: itemError}
		*snapshot = control.ProjectCapabilities(*snapshot)
	})
}

func (c *Coordinator) syncOperations() {
	items := c.operations.Snapshot()
	c.store.Commit(func(snapshot *dto.DesktopSnapshot) {
		snapshot.Operations = items
	})
}

func (c *Coordinator) notifyArmed() {
	c.mu.Lock()
	callback := c.onArmed
	c.mu.Unlock()
	if callback != nil {
		callback()
	}
}

// Keep the runtime package in this boundary even when the first host starts
// with no factory. This compile-time assertion documents the intended adapter.
var _ RuntimeOwner = (*runtime.Controller)(nil)
