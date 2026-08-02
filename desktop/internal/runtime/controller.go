// Package runtime owns the in-process Server generation. Its actor is the
// only writer of lifecycle state and generation ownership; bindings enqueue
// bounded, non-blocking commands and receive receipts immediately.
package runtime

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"desktop/internal/control"
	"desktop/internal/control/dto"
	"desktop/internal/operation"
	"desktop/internal/state"

	"server/app"
)

const (
	DefaultCommandCapacity = 16
	ServerReadyBudget      = 90 * time.Second
	ServerStopBudget       = 30 * time.Second
)

var (
	ErrControllerClosed = errors.New("runtime controller is closed")
	ErrGenerationActive = operation.NewError(dto.ErrorRuntimeNotReady, "a Server generation is still owned by Desktop")
)

type DesiredStateStore interface {
	Load(context.Context) (dto.DesiredState, error)
	Save(context.Context, dto.DesiredState) error
}

type MemoryDesiredState struct {
	mu    sync.Mutex
	value dto.DesiredState
}

func NewMemoryDesiredState(value dto.DesiredState) *MemoryDesiredState {
	if value == "" {
		value = dto.DesiredStopped
	}
	return &MemoryDesiredState{value: value}
}

func (s *MemoryDesiredState) Load(context.Context) (dto.DesiredState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.value, nil
}

func (s *MemoryDesiredState) Save(_ context.Context, value dto.DesiredState) error {
	s.mu.Lock()
	s.value = value
	s.mu.Unlock()
	return nil
}

type ReadyInfo struct {
	Runtime           app.RuntimeInfo
	RepositoryControl app.RepositoryControl
	ManifestSHA256    string
}

// Generation is the only handle the Desktop runtime controller retains for a
// server generation. Done must be consumed exactly once by the controller.
type Generation struct {
	ID                uint64
	Cancel            context.CancelFunc
	Done              <-chan error
	Ready             <-chan ReadyInfo
	RepositoryControl app.RepositoryControl
	ManifestSHA256    string
	StartedAt         time.Time
}

type GenerationFactory interface {
	Start(context.Context, uint64) (Generation, error)
}

type commandKind string

const (
	commandStart        commandKind = "start"
	commandStop         commandKind = "stop"
	commandQuiesce      commandKind = "quiesce"
	commandRestart      commandKind = "restart"
	commandRetryCleanup commandKind = "retry-cleanup"
)

type command struct {
	kind         commandKind
	receipt      dto.OperationReceipt
	requestID    string
	generationID uint64
}

type resultKind string

const (
	resultStarted   resultKind = "started"
	resultStopped   resultKind = "stopped"
	resultRestarted resultKind = "restarted"
	resultCleanup   resultKind = "cleanup"
	resultFailed    resultKind = "failed"
	resultExited    resultKind = "exited"
)

type workerResult struct {
	operationID string
	kind        resultKind
	generation  *ownedGeneration
	ready       ReadyInfo
	err         error
	ownership   bool
	setStopped  bool
	cancelled   bool
	preserve    bool
	previous    dto.RuntimeSnapshot
}

type generationCompletion struct {
	finished chan struct{}
	mu       sync.RWMutex
	err      error
}

func newGenerationCompletion(done <-chan error) *generationCompletion {
	completion := &generationCompletion{finished: make(chan struct{})}
	go func() {
		var err error
		if done != nil {
			err = <-done
		}
		completion.mu.Lock()
		completion.err = err
		completion.mu.Unlock()
		close(completion.finished)
	}()
	return completion
}

func (c *generationCompletion) Err() error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.err
}

type ownedGeneration struct {
	Generation
	completion *generationCompletion
	stopping   atomic.Bool
}

type Controller struct {
	store       *state.Store
	operations  *operation.Registry
	desired     DesiredStateStore
	factory     GenerationFactory
	onReady     func() error
	configured  bool
	readyBudget time.Duration
	stopBudget  time.Duration
	commands    chan command
	quiesce     chan command

	ctx                context.Context
	cancel             context.CancelFunc
	done               chan struct{}
	startOnce          sync.Once
	closeOnce          sync.Once
	generationSequence atomic.Uint64
	activeCancel       context.CancelFunc
	repositoryMu       sync.RWMutex
	repositoryControl  StorageControl
}

type Options struct {
	Store           *state.Store
	Operations      *operation.Registry
	Desired         DesiredStateStore
	Factory         GenerationFactory
	Configured      bool
	OnReady         func() error
	CommandCapacity int
	ReadyBudget     time.Duration
	StopBudget      time.Duration
}

func NewController(options Options) *Controller {
	if options.Store == nil {
		options.Store = state.New()
	}
	if options.Operations == nil {
		options.Operations = operation.New()
	}
	if options.Desired == nil {
		options.Desired = NewMemoryDesiredState(dto.DesiredStopped)
	}
	if options.CommandCapacity < 1 {
		options.CommandCapacity = DefaultCommandCapacity
	}
	if options.ReadyBudget <= 0 {
		options.ReadyBudget = ServerReadyBudget
	}
	if options.StopBudget <= 0 {
		options.StopBudget = ServerStopBudget
	}
	c := &Controller{
		store: options.Store, operations: options.Operations, desired: options.Desired,
		factory: options.Factory, onReady: options.OnReady, configured: options.Configured,
		readyBudget: options.ReadyBudget, stopBudget: options.StopBudget,
		commands: make(chan command, options.CommandCapacity), quiesce: make(chan command, 1), done: make(chan struct{}),
	}
	if desired, err := options.Desired.Load(context.Background()); err == nil && desired != "" {
		c.store.Commit(func(snapshot *dto.DesktopSnapshot) {
			snapshot.Runtime.DesiredState = desired
		})
	}
	c.commitRuntime(func(snapshot *dto.RuntimeSnapshot) {
		snapshot.Configured = options.Configured
	})
	return c
}

func (c *Controller) StartActor(ctx context.Context) {
	c.startOnce.Do(func() {
		c.ctx, c.cancel = context.WithCancel(ctx)
		go c.run()
	})
}

func (c *Controller) Close() {
	c.closeOnce.Do(func() {
		if c.cancel != nil {
			c.cancel()
		}
		select {
		case <-c.done:
		case <-time.After(time.Second):
		}
	})
}

func (c *Controller) Snapshot() dto.DesktopSnapshot { return c.store.Get() }

func (c *Controller) RuntimeSnapshot() dto.RuntimeSnapshot { return c.store.Get().Runtime }

func (c *Controller) RepositoryControl() StorageControl {
	c.repositoryMu.RLock()
	defer c.repositoryMu.RUnlock()
	return c.repositoryControl
}

func (c *Controller) SetConfigured(configured bool) {
	c.commitRuntime(func(snapshot *dto.RuntimeSnapshot) {
		if snapshot.Configured != configured {
			snapshot.Version++
		}
		snapshot.Configured = configured
	})
}

func (c *Controller) Start(requestID string, expectedVersion uint64) (dto.OperationReceipt, error) {
	return c.submit(commandStart, requestID, expectedVersion)
}

func (c *Controller) Stop(requestID string, expectedVersion uint64) (dto.OperationReceipt, error) {
	return c.submit(commandStop, requestID, expectedVersion)
}

// Quiesce stops the current generation for quit/update without changing the
// persisted desired state. It is intentionally not exposed as a normal
// RuntimeService command; ShutdownCoordinator is its only caller.
func (c *Controller) Quiesce(requestID string, expectedVersion uint64) (dto.OperationReceipt, error) {
	if c.ctx == nil {
		return dto.OperationReceipt{}, operation.NewError(dto.ErrorRuntimeNotReady, "runtime actor has not started")
	}
	if existing, ok := c.operations.ReceiptForRequest(requestID); ok {
		return existing, nil
	}
	snapshot := c.store.Get()
	if expectedVersion != 0 && expectedVersion != snapshot.Runtime.Version {
		return dto.OperationReceipt{}, operation.NewError(dto.ErrorStaleVersion, "runtime snapshot version is stale")
	}
	// Quiesce has a separate high-priority aggregate. A quit request must be
	// able to cancel a readiness wait already occupying the normal lifecycle
	// aggregate instead of being rejected as an ordinary conflicting mutation.
	receipt, err := c.operations.Accept(requestID, "runtime-quiesce", snapshot.Runtime.Version, false)
	if err != nil {
		return dto.OperationReceipt{}, err
	}
	c.syncOperations()
	select {
	case c.quiesce <- command{kind: commandQuiesce, receipt: receipt}:
		return receipt, nil
	default:
		busy := operation.WithOperation(operation.NewError(dto.ErrorControllerBusy, "runtime quiesce queue is full"), receipt.OperationID)
		_ = c.operations.Fail(receipt.OperationID, busy)
		c.syncOperations()
		return dto.OperationReceipt{}, busy
	}
}

func (c *Controller) Restart(requestID string, expectedVersion uint64) (dto.OperationReceipt, error) {
	return c.submit(commandRestart, requestID, expectedVersion)
}

func (c *Controller) RetryCleanup(requestID string, expectedVersion uint64) (dto.OperationReceipt, error) {
	return c.submit(commandRetryCleanup, requestID, expectedVersion)
}

func (c *Controller) submit(kind commandKind, requestID string, expectedVersion uint64) (dto.OperationReceipt, error) {
	if c.ctx == nil {
		return dto.OperationReceipt{}, operation.NewError(dto.ErrorRuntimeNotReady, "runtime actor has not started")
	}
	if existing, ok := c.operations.ReceiptForRequest(requestID); ok {
		return existing, nil
	}
	snapshot := c.store.Get()
	if expectedVersion != 0 && expectedVersion != snapshot.Runtime.Version {
		return dto.OperationReceipt{}, operation.NewError(dto.ErrorStaleVersion, "runtime snapshot version is stale")
	}
	receipt, err := c.operations.Accept(requestID, "runtime", snapshot.Runtime.Version, true)
	if err != nil {
		return dto.OperationReceipt{}, err
	}
	c.syncOperations()
	select {
	case c.commands <- command{kind: kind, receipt: receipt, requestID: requestID}:
		return receipt, nil
	default:
		busy := operation.WithOperation(operation.NewError(dto.ErrorControllerBusy, "runtime command queue is full"), receipt.OperationID)
		_ = c.operations.Fail(receipt.OperationID, busy)
		c.syncOperations()
		return dto.OperationReceipt{}, busy
	}
}

func (c *Controller) run() {
	defer close(c.done)
	var active *command
	var pendingQuiesce *command
	results := make(chan workerResult, 1)
	var generation *ownedGeneration

	for {
		select {
		case <-c.ctx.Done():
			return
		case result := <-results:
			active = nil
			c.activeCancel = nil
			c.handleResult(result, &generation, results)
			if pendingQuiesce != nil {
				quiesce := *pendingQuiesce
				pendingQuiesce = nil
				c.beginCommand(quiesce, generation, &active, results)
			}
		case quiesce := <-c.quiesce:
			if active != nil {
				if pendingQuiesce != nil {
					_ = c.operations.Fail(quiesce.receipt.OperationID, operation.NewError(dto.ErrorOperationConflict, "runtime quiesce is already pending"))
					c.syncOperations()
					continue
				}
				pendingQuiesce = &quiesce
				// The active worker owns the cancellable readiness/installation
				// wait. Its result returns the generation to this actor, which
				// then starts the pending quiesce against that same owner.
				if c.activeCancel != nil {
					c.activeCancel()
				}
				continue
			}
			c.beginCommand(quiesce, generation, &active, results)
		case cmd := <-c.commands:
			if active != nil {
				_ = c.operations.Fail(cmd.receipt.OperationID, operation.NewError(dto.ErrorOperationConflict, "runtime operation is already active"))
				c.syncOperations()
				continue
			}
			c.beginCommand(cmd, generation, &active, results)
		}
	}
}

func (c *Controller) beginCommand(cmd command, generation *ownedGeneration, active **command, results chan<- workerResult) {
	*active = &cmd
	_ = c.operations.MarkRunning(cmd.receipt.OperationID)
	c.syncOperations()
	previous := c.store.Get().Runtime
	if cmd.kind == commandStart || cmd.kind == commandRestart {
		cmd.generationID = c.generationSequence.Add(1)
	}
	if generation != nil {
		switch cmd.kind {
		case commandStop, commandQuiesce, commandRetryCleanup:
			c.commitRuntime(func(snapshot *dto.RuntimeSnapshot) { snapshot.Phase = dto.RuntimeStopping })
		case commandRestart:
			c.commitRuntime(func(snapshot *dto.RuntimeSnapshot) { snapshot.Phase = dto.RuntimeRestarting })
		}
	} else if cmd.kind == commandStart {
		c.commitRuntime(func(snapshot *dto.RuntimeSnapshot) { snapshot.Phase = dto.RuntimeStarting })
	}
	workerCtx, cancel := context.WithCancel(c.ctx)
	c.activeCancel = cancel
	go c.runWorker(cmd, generation, results, workerCtx, previous)
}

func (c *Controller) runWorker(cmd command, generation *ownedGeneration, results chan<- workerResult, workerCtx context.Context, previous dto.RuntimeSnapshot) {
	snapshot := c.store.Get()
	result := workerResult{operationID: cmd.receipt.OperationID, previous: previous}
	switch cmd.kind {
	case commandStart:
		if !snapshot.Runtime.Configured {
			result.kind = resultFailed
			result.err = operation.NewError(dto.ErrorRuntimeNotConfigured, "runtime configuration is incomplete")
			results <- result
			return
		}
		if generation != nil {
			result.kind = resultFailed
			result.err = ErrGenerationActive
			result.ownership = true
			results <- result
			return
		}
		if err := c.desired.Save(workerCtx, dto.DesiredRunning); err != nil {
			result.kind = resultFailed
			result.err = fmt.Errorf("persist runtime desired state: %w", err)
			result.preserve = true
			results <- result
			return
		}
		result = c.startWorker(result, cmd.generationID, workerCtx)
	case commandStop:
		if snapshot.Runtime.Ownership == dto.OwnershipHeld && snapshot.Runtime.Phase == dto.RuntimeFailed {
			result.kind = resultFailed
			result.err = operation.NewError(dto.ErrorRuntimeNotReady, "runtime ownership requires cleanup")
			result.ownership = true
			results <- result
			return
		}
		if err := c.desired.Save(workerCtx, dto.DesiredStopped); err != nil {
			result.kind = resultFailed
			result.err = fmt.Errorf("persist runtime desired state: %w", err)
			result.preserve = true
			results <- result
			return
		}
		result.setStopped = true
		if generation == nil {
			result.kind = resultStopped
			results <- result
			return
		}
		result = c.stopWorker(result, generation)
	case commandQuiesce:
		if generation == nil {
			result.kind = resultStopped
			results <- result
			return
		}
		result = c.stopWorker(result, generation)
	case commandRetryCleanup:
		if generation == nil {
			result.kind = resultFailed
			result.err = operation.NewError(dto.ErrorRuntimeNotReady, "no retained runtime ownership exists")
			results <- result
			return
		}
		result = c.stopWorker(result, generation)
	case commandRestart:
		if generation == nil || (snapshot.Runtime.Phase != dto.RuntimeRunning && snapshot.Runtime.Phase != dto.RuntimeApplyingConfig) || snapshot.Runtime.DesiredState != dto.DesiredRunning {
			result.kind = resultFailed
			result.err = operation.NewError(dto.ErrorRuntimeNotReady, "runtime is not ready to restart")
			result.ownership = generation != nil
			results <- result
			return
		}
		result = c.restartWorker(result, generation, cmd.generationID, workerCtx)
	default:
		result.kind = resultFailed
		result.err = operation.NewError(dto.ErrorInvalidArgument, "unknown runtime command")
		results <- result
		return
	}
	result.cancelled = workerCtx.Err() != nil && cmd.kind != commandQuiesce
	results <- result
}

func (c *Controller) startWorker(result workerResult, generationID uint64, workerCtx context.Context) workerResult {
	if c.factory == nil {
		result.kind = resultFailed
		result.err = operation.NewError(dto.ErrorRuntimeNotReady, "runtime generation factory is unavailable")
		return result
	}
	generation, err := c.factory.Start(workerCtx, generationID)
	if err != nil {
		result.kind = resultFailed
		result.err = err
		return result
	}
	owned := &ownedGeneration{Generation: generation, completion: newGenerationCompletion(generation.Done)}
	result.generation = owned
	result.kind = resultStarted
	result.ready, result.err = waitReady(workerCtx, owned, c.readyBudget)
	if result.err != nil {
		result.kind = resultFailed
		result.ownership = true
		if owned.Cancel != nil {
			owned.Cancel()
		}
		return result
	}
	owned.RepositoryControl = result.ready.RepositoryControl
	owned.ManifestSHA256 = result.ready.ManifestSHA256
	return result
}

func (c *Controller) stopWorker(result workerResult, generation *ownedGeneration) workerResult {
	result.generation = generation
	generation.stopping.Store(true)
	if generation.Cancel != nil {
		generation.Cancel()
	}
	if generation.completion.wait(c.stopBudget) {
		result.kind = resultStopped
		return result
	}
	result.kind = resultFailed
	result.err = operation.NewError(dto.ErrorStopTimeout, "Server generation did not stop within the shutdown budget")
	result.ownership = true
	return result
}

func (c *Controller) restartWorker(result workerResult, generation *ownedGeneration, generationID uint64, workerCtx context.Context) workerResult {
	result.generation = generation
	generation.stopping.Store(true)
	if generation.Cancel != nil {
		generation.Cancel()
	}
	if !generation.completion.wait(c.stopBudget) {
		result.kind = resultFailed
		result.err = operation.NewError(dto.ErrorStopTimeout, "Server generation did not stop before restart")
		result.ownership = true
		return result
	}
	return c.startWorker(result, generationID, workerCtx)
}

func waitReady(ctx context.Context, generation *ownedGeneration, budget time.Duration) (ReadyInfo, error) {
	if generation.Ready == nil {
		return ReadyInfo{}, operation.NewError(dto.ErrorReadinessTimeout, "Server generation did not publish readiness")
	}
	timer := time.NewTimer(budget)
	defer timer.Stop()
	select {
	case ready := <-generation.Ready:
		if ready.RepositoryControl == nil {
			return ReadyInfo{}, operation.NewError(dto.ErrorRepositoryControlUnavailable, "Server readiness did not publish repository control")
		}
		return ready, nil
	case <-generation.completion.finished:
		err := generation.completion.Err()
		if err == nil {
			return ReadyInfo{}, operation.NewError(dto.ErrorRuntimeNotReady, "Server generation exited before readiness")
		}
		return ReadyInfo{}, err
	case <-timer.C:
		return ReadyInfo{}, operation.NewError(dto.ErrorReadinessTimeout, "Server generation readiness timed out")
	case <-ctx.Done():
		return ReadyInfo{}, ctx.Err()
	}
}

func (c *generationCompletion) wait(budget time.Duration) bool {
	timer := time.NewTimer(budget)
	defer timer.Stop()
	select {
	case <-c.finished:
		return true
	case <-timer.C:
		return false
	}
}

func (c *Controller) handleResult(result workerResult, generation **ownedGeneration, results chan<- workerResult) {
	switch result.kind {
	case resultStarted:
		*generation = result.generation
		c.repositoryMu.Lock()
		c.repositoryControl = adaptRepositoryControl(result.ready.RepositoryControl)
		c.repositoryMu.Unlock()
		readyErr := error(nil)
		if c.onReady != nil {
			readyErr = c.onReady()
		}
		_ = c.operations.Succeed(result.operationID)
		c.commitRuntime(func(snapshot *dto.RuntimeSnapshot) {
			snapshot.DesiredState = dto.DesiredRunning
			snapshot.Phase = dto.RuntimeRunning
			snapshot.Ownership = dto.OwnershipHeld
			snapshot.RecoveryCause = ""
			if readyErr != nil {
				snapshot.PendingConfigValidation = true
				snapshot.RecoveryCause = dto.ErrorRecoveryRequired
			}
			snapshot.ManifestSHA256 = result.generation.ManifestSHA256
			snapshot.ProductURL = result.ready.Runtime.ProductURL
			snapshot.Version++
		})
		c.watchGeneration(result.generation, results)
	case resultStopped, resultCleanup:
		if *generation == result.generation {
			*generation = nil
		}
		c.repositoryMu.Lock()
		c.repositoryControl = nil
		c.repositoryMu.Unlock()
		if result.cancelled {
			_ = c.operations.Cancel(result.operationID, operation.NewError(dto.ErrorShutdownInProgress, "runtime operation was cancelled for shutdown"))
		} else {
			_ = c.operations.Succeed(result.operationID)
		}
		c.commitRuntime(func(snapshot *dto.RuntimeSnapshot) {
			if result.setStopped {
				snapshot.DesiredState = dto.DesiredStopped
			}
			snapshot.Phase = dto.RuntimeStopped
			snapshot.Ownership = dto.OwnershipNone
			snapshot.ProductURL = ""
			snapshot.RecoveryCause = ""
			snapshot.Version++
		})
	case resultRestarted:
		_ = c.operations.Succeed(result.operationID)
	case resultFailed, resultExited:
		if result.kind == resultExited {
			if *generation != result.generation {
				return
			}
			*generation = nil
			c.repositoryMu.Lock()
			c.repositoryControl = nil
			c.repositoryMu.Unlock()
			c.commitRuntime(func(snapshot *dto.RuntimeSnapshot) {
				snapshot.Phase = dto.RuntimeFailed
				snapshot.Ownership = dto.OwnershipNone
				snapshot.RecoveryCause = dto.ErrorRuntimeNotReady
				snapshot.Version++
			})
			return
		}
		if result.preserve {
			c.commitRuntime(func(snapshot *dto.RuntimeSnapshot) {
				*snapshot = result.previous
				snapshot.Version++
			})
			if result.cancelled {
				_ = c.operations.Cancel(result.operationID, operation.NewError(dto.ErrorShutdownInProgress, "runtime operation was cancelled for shutdown"))
			} else if result.err != nil {
				_ = c.operations.Fail(result.operationID, result.err)
			}
			c.syncOperations()
			return
		}
		if result.cancelled {
			_ = c.operations.Cancel(result.operationID, operation.NewError(dto.ErrorShutdownInProgress, "runtime operation was cancelled for shutdown"))
		} else if result.err != nil {
			_ = c.operations.Fail(result.operationID, result.err)
		} else {
			_ = c.operations.Fail(result.operationID, operation.NewError(dto.ErrorRuntimeNotReady, "runtime generation failed"))
		}
		if !result.ownership {
			*generation = nil
			c.repositoryMu.Lock()
			c.repositoryControl = nil
			c.repositoryMu.Unlock()
		} else if result.generation != nil {
			*generation = result.generation
		}
		c.commitRuntime(func(snapshot *dto.RuntimeSnapshot) {
			snapshot.Phase = dto.RuntimeFailed
			if result.ownership {
				snapshot.Ownership = dto.OwnershipHeld
			} else {
				snapshot.Ownership = dto.OwnershipNone
			}
			snapshot.RecoveryCause = operation.ErrorCodeOf(result.err)
			snapshot.Version++
		})
	}
	c.syncOperations()
}

func (c *Controller) watchGeneration(generation *ownedGeneration, results chan<- workerResult) {
	if generation == nil || generation.completion == nil {
		return
	}
	go func() {
		<-generation.completion.finished
		if generation.stopping.Load() {
			return
		}
		result := workerResult{kind: resultExited, generation: generation, err: generation.completion.Err()}
		select {
		case <-c.ctx.Done():
		case results <- result:
		}
	}()
}

func (c *Controller) commitRuntime(update func(*dto.RuntimeSnapshot)) {
	c.store.Commit(func(snapshot *dto.DesktopSnapshot) {
		update(&snapshot.Runtime)
		*snapshot = control.ProjectCapabilities(*snapshot)
	})
}

func (c *Controller) syncOperations() {
	items := c.operations.Snapshot()
	c.store.Commit(func(snapshot *dto.DesktopSnapshot) {
		snapshot.Operations = items
	})
}
