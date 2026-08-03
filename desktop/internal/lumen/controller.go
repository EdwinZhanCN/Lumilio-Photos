// Package lumen owns the optional Lumen Hub child process. The controller is
// deliberately independent from the Server generation: the Hub is an
// externally supervised process tree, while all UI-facing state still flows
// through the Desktop snapshot.
package lumen

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"desktop/internal/control"
	"desktop/internal/control/dto"
	"desktop/internal/operation"
	"desktop/internal/state"
)

const (
	DefaultCommandCapacity = 16
	ReadyBudget            = 30 * time.Second
	StopBudget             = 15 * time.Second
)

var ErrControllerClosed = errors.New("Lumen controller is closed")

// Process is the only child-process handle retained by the controller. The
// factory owns platform-specific process-group/Job Object setup; the actor
// owns the single Done consumer.
type Process struct {
	ID       uint64
	Cancel   context.CancelFunc
	Lifetime context.Context
	Done     <-chan error
	Ready    <-chan ReadyInfo
	Status   <-chan dto.LumenControlStatus
	Profile  string
}

type ReadyInfo struct {
	Endpoint string
}

type Factory interface {
	Start(context.Context, uint64, string) (Process, error)
}

type Installer interface {
	Install(context.Context, string) (version string, err error)
}

type DesiredStateStore interface {
	Load(context.Context) (dto.DesiredState, error)
	Save(context.Context, dto.DesiredState) error
}

type SetupStore interface {
	SaveSetup(context.Context, string, string) error
}

type MemorySetupStore struct {
	Preset   string
	CacheDir string
}

func (s *MemorySetupStore) SaveSetup(_ context.Context, preset, cacheDir string) error {
	s.Preset = preset
	s.CacheDir = cacheDir
	return nil
}

type MemoryDesiredState struct {
	mu    sync.Mutex
	value dto.DesiredState
}

func NewMemoryDesiredState(value dto.DesiredState) *MemoryDesiredState {
	if value == "" {
		value = dto.DesiredDisabled
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

type Options struct {
	Store           *state.Store
	Operations      *operation.Registry
	Desired         DesiredStateStore
	Factory         Factory
	Installer       Installer
	SetupStore      SetupStore
	Installed       bool
	InstalledVer    string
	Profile         string
	Preset          string
	CacheDir        string
	Profiles        []string
	Presets         []string
	Endpoint        string
	CommandCapacity int
	ReadyBudget     time.Duration
	StopBudget      time.Duration
}

type commandKind string

const (
	commandInstall      commandKind = "install"
	commandStart        commandKind = "start"
	commandStop         commandKind = "stop"
	commandQuiesce      commandKind = "quiesce"
	commandRestart      commandKind = "restart"
	commandRetryCleanup commandKind = "retry-cleanup"
)

type command struct {
	kind         commandKind
	receipt      dto.OperationReceipt
	generationID uint64
	profile      string
	preset       string
	cacheDir     string
}

type resultKind string

const (
	resultInstalled resultKind = "installed"
	resultStarted   resultKind = "started"
	resultStopped   resultKind = "stopped"
	resultFailed    resultKind = "failed"
	resultExited    resultKind = "exited"
	resultStable    resultKind = "stable"
	resultStatus    resultKind = "status"
)

type workerResult struct {
	operationID string
	kind        resultKind
	process     *ownedProcess
	profile     string
	preset      string
	cacheDir    string
	version     string
	err         error
	ownership   bool
	setDisabled bool
	installing  bool
	cancelled   bool
	preserve    bool
	previous    dto.LumenSnapshot
	status      dto.LumenControlStatus
}

type completion struct {
	done chan struct{}
	mu   sync.RWMutex
	err  error
}

func newCompletion(done <-chan error) *completion {
	c := &completion{done: make(chan struct{})}
	go func() {
		var err error
		if done != nil {
			err = <-done
		}
		c.mu.Lock()
		c.err = err
		c.mu.Unlock()
		close(c.done)
	}()
	return c
}

func (c *completion) Err() error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.err
}

type ownedProcess struct {
	Process
	completion *completion
	stopping   atomic.Bool
}

type Controller struct {
	store       *state.Store
	operations  *operation.Registry
	desired     DesiredStateStore
	factory     Factory
	installer   Installer
	setupStore  SetupStore
	readyBudget time.Duration
	stopBudget  time.Duration
	commands    chan command
	quiesce     chan command

	ctx          context.Context
	cancel       context.CancelFunc
	done         chan struct{}
	startOnce    sync.Once
	closeOnce    sync.Once
	sequence     atomic.Uint64
	activeCancel context.CancelFunc
	quiesced     atomic.Bool
	retryCount   int

	mu        sync.Mutex
	installed bool
	version   string
	profile   string
	preset    string
	cacheDir  string
	endpoint  string
	pick      func(string) (string, error)
}

func NewController(options Options) *Controller {
	if options.Store == nil {
		options.Store = state.New()
	}
	if options.Operations == nil {
		options.Operations = operation.New()
	}
	if options.Desired == nil {
		options.Desired = NewMemoryDesiredState(dto.DesiredDisabled)
	}
	if options.SetupStore == nil {
		options.SetupStore = &MemorySetupStore{}
	}
	if options.Preset == "" {
		options.Preset = "basic"
	}
	if len(options.Profiles) == 0 && options.Profile != "" {
		options.Profiles = []string{options.Profile}
	}
	if len(options.Presets) == 0 {
		options.Presets = []string{"minimal", "basic", "brave"}
	}
	if options.Endpoint == "" {
		options.Endpoint = DefaultEndpoint
	}
	if options.CommandCapacity < 1 {
		options.CommandCapacity = DefaultCommandCapacity
	}
	if options.ReadyBudget <= 0 {
		options.ReadyBudget = ReadyBudget
	}
	if options.StopBudget <= 0 {
		options.StopBudget = StopBudget
	}
	controller := &Controller{
		store: options.Store, operations: options.Operations, desired: options.Desired,
		factory: options.Factory, installer: options.Installer, setupStore: options.SetupStore,
		readyBudget: options.ReadyBudget, stopBudget: options.StopBudget,
		commands: make(chan command, options.CommandCapacity), quiesce: make(chan command, 1), done: make(chan struct{}),
		installed: options.Installed, version: options.InstalledVer, profile: options.Profile, preset: options.Preset, cacheDir: options.CacheDir, endpoint: options.Endpoint,
	}
	if desired, err := options.Desired.Load(context.Background()); err == nil && desired != "" {
		controller.commit(func(snapshot *dto.LumenSnapshot) { snapshot.DesiredState = desired })
	}
	controller.commit(func(snapshot *dto.LumenSnapshot) {
		if controller.installed {
			snapshot.InstallPhase = dto.LumenInstalled
		} else {
			snapshot.InstallPhase = dto.LumenAbsent
		}
		snapshot.Profile = controller.profile
		snapshot.Preset = controller.preset
		snapshot.CacheDir = controller.cacheDir
		snapshot.AvailableProfiles = append([]string(nil), options.Profiles...)
		snapshot.AvailablePresets = append([]string(nil), options.Presets...)
		snapshot.InstallerAvailable = controller.installer != nil
		snapshot.ProcessAvailable = controller.factory != nil
	})
	return controller
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

func (c *Controller) Snapshot() dto.LumenSnapshot { return c.store.Get().Lumen }

func (c *Controller) Logs(backlog uint32, minLevel string) ([]dto.LumenLogEntry, error) {
	if c.store.Get().Lumen.ProcessPhase != dto.LumenRunning {
		return nil, operation.NewError(dto.ErrorRuntimeNotReady, "Lumen must be running to read logs")
	}
	if backlog == 0 {
		backlog = 200
	}
	if backlog > 500 {
		return nil, operation.NewError(dto.ErrorInvalidArgument, "Lumen log backlog exceeds 500 lines")
	}
	minLevel = strings.ToUpper(strings.TrimSpace(minLevel))
	if minLevel == "" {
		minLevel = "INFO"
	}
	if !slices.Contains([]string{"TRACE", "DEBUG", "INFO", "WARN", "ERROR"}, minLevel) {
		return nil, operation.NewError(dto.ErrorInvalidArgument, "unsupported Lumen log level")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	logs, err := readControlLogs(ctx, c.endpoint, backlog, minLevel)
	if err != nil {
		return nil, operation.NewError(dto.ErrorRuntimeNotReady, "Lumen logs are unavailable")
	}
	return logs, nil
}

func (c *Controller) LumenSnapshot() dto.LumenSnapshot { return c.Snapshot() }

func (c *Controller) SetPickDirectory(pick func(string) (string, error)) {
	c.mu.Lock()
	c.pick = pick
	c.mu.Unlock()
}

func (c *Controller) PickCacheDirectory(title string) (string, error) {
	c.mu.Lock()
	pick := c.pick
	c.mu.Unlock()
	if pick == nil {
		return "", operation.NewError(dto.ErrorRuntimeNotReady, "directory picker is unavailable")
	}
	path, err := pick(strings.TrimSpace(title))
	if err != nil {
		return "", operation.NewError(dto.ErrorInvalidArgument, "directory selection was cancelled or unavailable")
	}
	if strings.TrimSpace(path) == "" {
		return "", nil
	}
	canonical, err := CanonicalCacheDirectory(path)
	if err != nil {
		return "", operation.NewError(dto.ErrorInvalidArgument, "selected Lumen cache directory is unavailable")
	}
	return canonical, nil
}

func (c *Controller) SetInstalled(version, profile string) {
	c.mu.Lock()
	c.installed = true
	c.version = version
	c.profile = profile
	c.mu.Unlock()
	c.commit(func(snapshot *dto.LumenSnapshot) {
		snapshot.InstallPhase = dto.LumenInstalled
		snapshot.Profile = profile
		snapshot.Version++
	})
}

func (c *Controller) Install(requestID string, expectedVersion uint64, profile, preset, cacheDir string) (dto.OperationReceipt, error) {
	snapshot := c.store.Get().Lumen
	if profile == "" {
		profile = snapshot.Profile
	}
	if preset == "" {
		preset = snapshot.Preset
	}
	if cacheDir == "" {
		cacheDir = snapshot.CacheDir
	}
	if !slices.Contains(snapshot.AvailableProfiles, profile) {
		return dto.OperationReceipt{}, operation.NewError(dto.ErrorInvalidArgument, "unsupported Lumen release profile")
	}
	if !slices.Contains(snapshot.AvailablePresets, preset) {
		return dto.OperationReceipt{}, operation.NewError(dto.ErrorInvalidArgument, "unsupported Lumen preset")
	}
	canonical, err := CanonicalCacheDirectory(cacheDir)
	if err != nil {
		return dto.OperationReceipt{}, operation.NewError(dto.ErrorInvalidArgument, "Lumen cache directory must be an available non-root directory")
	}
	return c.submit(command{kind: commandInstall, profile: profile, preset: preset, cacheDir: canonical}, requestID, expectedVersion)
}

func (c *Controller) Start(requestID string, expectedVersion uint64) (dto.OperationReceipt, error) {
	return c.submit(command{kind: commandStart}, requestID, expectedVersion)
}

func (c *Controller) Stop(requestID string, expectedVersion uint64) (dto.OperationReceipt, error) {
	return c.submit(command{kind: commandStop}, requestID, expectedVersion)
}

func (c *Controller) Quiesce(requestID string, expectedVersion uint64) (dto.OperationReceipt, error) {
	if c.ctx == nil {
		return dto.OperationReceipt{}, operation.NewError(dto.ErrorRuntimeNotReady, "Lumen actor has not started")
	}
	if existing, ok := c.operations.ReceiptForRequest(requestID); ok {
		return existing, nil
	}
	snapshot := c.store.Get()
	if expectedVersion != 0 && expectedVersion != snapshot.Lumen.Version {
		return dto.OperationReceipt{}, operation.NewError(dto.ErrorStaleVersion, "Lumen snapshot version is stale")
	}
	receipt, err := c.operations.Accept(requestID, "lumen-quiesce", snapshot.Lumen.Version, false)
	if err != nil {
		return dto.OperationReceipt{}, err
	}
	c.syncOperations()
	select {
	case c.quiesce <- command{kind: commandQuiesce, receipt: receipt}:
		return receipt, nil
	default:
		busy := operation.WithOperation(operation.NewError(dto.ErrorControllerBusy, "Lumen quiesce queue is full"), receipt.OperationID)
		_ = c.operations.Fail(receipt.OperationID, busy)
		c.syncOperations()
		return dto.OperationReceipt{}, busy
	}
}

func (c *Controller) Restart(requestID string, expectedVersion uint64) (dto.OperationReceipt, error) {
	return c.submit(command{kind: commandRestart}, requestID, expectedVersion)
}

func (c *Controller) RetryCleanup(requestID string, expectedVersion uint64) (dto.OperationReceipt, error) {
	return c.submit(command{kind: commandRetryCleanup}, requestID, expectedVersion)
}

func (c *Controller) submit(item command, requestID string, expectedVersion uint64) (dto.OperationReceipt, error) {
	if c.ctx == nil {
		return dto.OperationReceipt{}, operation.NewError(dto.ErrorRuntimeNotReady, "Lumen actor has not started")
	}
	if existing, ok := c.operations.ReceiptForRequest(requestID); ok {
		return existing, nil
	}
	snapshot := c.store.Get()
	if expectedVersion != 0 && expectedVersion != snapshot.Lumen.Version {
		return dto.OperationReceipt{}, operation.NewError(dto.ErrorStaleVersion, "Lumen snapshot version is stale")
	}
	receipt, err := c.operations.Accept(requestID, "lumen", snapshot.Lumen.Version, true)
	if err != nil {
		return dto.OperationReceipt{}, err
	}
	c.syncOperations()
	item.receipt = receipt
	select {
	case c.commands <- item:
		return receipt, nil
	default:
		busy := operation.WithOperation(operation.NewError(dto.ErrorControllerBusy, "Lumen command queue is full"), receipt.OperationID)
		_ = c.operations.Fail(receipt.OperationID, busy)
		c.syncOperations()
		return dto.OperationReceipt{}, busy
	}
}

func (c *Controller) run() {
	defer close(c.done)
	var active *command
	var pendingQuiesce *command
	var process *ownedProcess
	results := make(chan workerResult, 1)
	events := make(chan workerResult, 16)
	for {
		select {
		case <-c.ctx.Done():
			return
		case result := <-results:
			active = nil
			c.activeCancel = nil
			c.handleResult(result, &process, events)
			if pendingQuiesce != nil {
				quiesce := *pendingQuiesce
				pendingQuiesce = nil
				c.beginCommand(quiesce, process, &active, results)
			}
		case event := <-events:
			c.handleResult(event, &process, events)
		case quiesce := <-c.quiesce:
			if active != nil {
				if pendingQuiesce != nil {
					_ = c.operations.Fail(quiesce.receipt.OperationID, operation.NewError(dto.ErrorOperationConflict, "Lumen quiesce is already pending"))
					c.syncOperations()
					continue
				}
				pendingQuiesce = &quiesce
				if c.activeCancel != nil {
					c.activeCancel()
				}
				continue
			}
			c.beginCommand(quiesce, process, &active, results)
		case item := <-c.commands:
			if active != nil {
				_ = c.operations.Fail(item.receipt.OperationID, operation.NewError(dto.ErrorOperationConflict, "Lumen operation is already active"))
				c.syncOperations()
				continue
			}
			c.beginCommand(item, process, &active, results)
		}
	}
}

func (c *Controller) beginCommand(item command, process *ownedProcess, active **command, results chan<- workerResult) {
	*active = &item
	_ = c.operations.MarkRunning(item.receipt.OperationID)
	c.syncOperations()
	previous := c.store.Get().Lumen
	if item.kind == commandStart || item.kind == commandRestart {
		item.generationID = c.sequence.Add(1)
	}
	if item.kind == commandQuiesce {
		c.quiesced.Store(true)
	} else if item.kind == commandStart {
		c.quiesced.Store(false)
		if !strings.HasPrefix(item.receipt.RequestID, "lumen-auto-retry-") {
			c.retryCount = 0
		}
	} else if item.kind == commandStop {
		c.retryCount = 0
	}
	if process != nil {
		switch item.kind {
		case commandStop, commandQuiesce, commandRetryCleanup, commandRestart:
			c.commit(func(snapshot *dto.LumenSnapshot) { snapshot.ProcessPhase = dto.LumenStopping })
		}
	} else if item.kind == commandStart {
		c.commit(func(snapshot *dto.LumenSnapshot) { snapshot.ProcessPhase = dto.LumenStarting })
	} else if item.kind == commandInstall {
		c.commit(func(snapshot *dto.LumenSnapshot) { snapshot.InstallPhase = dto.LumenInstalling })
	}
	workerCtx, cancel := context.WithCancel(c.ctx)
	c.activeCancel = cancel
	go c.worker(item, process, results, workerCtx, previous)
}

func (c *Controller) worker(item command, process *ownedProcess, results chan<- workerResult, workerCtx context.Context, previous dto.LumenSnapshot) {
	snapshot := c.store.Get().Lumen
	result := workerResult{operationID: item.receipt.OperationID, previous: previous}
	switch item.kind {
	case commandInstall:
		if process != nil || snapshot.ProcessPhase != dto.LumenStopped {
			result.kind = resultFailed
			result.err = operation.NewError(dto.ErrorOperationConflict, "Lumen must be stopped before installation")
			result.ownership = process != nil
			results <- result
			return
		}
		result.installing = true
		if c.installer == nil {
			result.kind = resultFailed
			result.err = operation.NewError(dto.ErrorLumenNotInstalled, "Lumen installer is unavailable")
			results <- result
			return
		}
		profile := item.profile
		if profile == "" {
			profile = snapshot.Profile
		}
		if err := c.setupStore.SaveSetup(workerCtx, item.preset, item.cacheDir); err != nil {
			result.kind = resultFailed
			result.err = fmt.Errorf("persist Lumen setup: %w", err)
			results <- result
			return
		}
		version, err := c.installer.Install(workerCtx, profile)
		if err != nil {
			result.kind = resultFailed
			result.err = err
			results <- result
			return
		}
		result.kind, result.version, result.profile, result.preset, result.cacheDir = resultInstalled, version, profile, item.preset, item.cacheDir
	case commandStart:
		if snapshot.InstallPhase != dto.LumenInstalled {
			result.kind = resultFailed
			result.err = operation.NewError(dto.ErrorLumenNotInstalled, "Lumen is not installed")
			results <- result
			return
		}
		if process != nil {
			result.kind, result.err, result.ownership = resultFailed, operation.NewError(dto.ErrorLumenOwnerBusy, "a Lumen process is still owned"), true
			results <- result
			return
		}
		if err := c.desired.Save(workerCtx, dto.DesiredRunning); err != nil {
			result.kind, result.err = resultFailed, fmt.Errorf("persist Lumen desired state: %w", err)
			result.preserve = true
			results <- result
			return
		}
		result = c.startWorker(result, item.generationID, snapshot.Profile, workerCtx)
	case commandStop:
		if err := c.desired.Save(workerCtx, dto.DesiredDisabled); err != nil {
			result.kind, result.err = resultFailed, fmt.Errorf("persist Lumen desired state: %w", err)
			result.preserve = true
			results <- result
			return
		}
		result.setDisabled = true
		if process == nil {
			result.kind = resultStopped
			results <- result
			return
		}
		result = c.stopWorker(result, process)
	case commandQuiesce:
		if process == nil {
			result.kind = resultStopped
			results <- result
			return
		}
		result = c.stopWorker(result, process)
	case commandRetryCleanup:
		if process == nil {
			result.kind, result.err = resultFailed, operation.NewError(dto.ErrorRuntimeNotReady, "no retained Lumen ownership exists")
			results <- result
			return
		}
		result = c.stopWorker(result, process)
	case commandRestart:
		if process == nil || snapshot.ProcessPhase != dto.LumenRunning || snapshot.DesiredState != dto.DesiredRunning {
			result.kind, result.err, result.ownership = resultFailed, operation.NewError(dto.ErrorRuntimeNotReady, "Lumen is not ready to restart"), process != nil
			results <- result
			return
		}
		result = c.restartWorker(result, process, item.generationID, snapshot.Profile, workerCtx)
	}
	result.cancelled = workerCtx.Err() != nil && item.kind != commandQuiesce
	results <- result
}

func (c *Controller) startWorker(result workerResult, id uint64, profile string, workerCtx context.Context) workerResult {
	if c.factory == nil {
		result.kind, result.err = resultFailed, operation.NewError(dto.ErrorRuntimeNotReady, "Lumen process factory is unavailable")
		return result
	}
	process, err := c.factory.Start(workerCtx, id, profile)
	if err != nil {
		result.kind, result.err = resultFailed, err
		return result
	}
	owned := &ownedProcess{Process: process, completion: newCompletion(process.Done)}
	result.process, result.profile = owned, profile
	result.kind = resultStarted
	if process.Ready == nil {
		result.kind, result.err, result.ownership = resultFailed, operation.NewError(dto.ErrorReadinessTimeout, "Lumen did not publish readiness"), true
		return result
	}
	timer := time.NewTimer(c.readyBudget)
	defer timer.Stop()
	select {
	case <-process.Ready:
		return result
	case <-owned.completion.done:
		result.kind, result.err = resultFailed, owned.completion.Err()
		if result.err == nil {
			result.err = operation.NewError(dto.ErrorRuntimeNotReady, "Lumen exited before readiness")
		}
		result.ownership = true
	case <-timer.C:
		result.kind, result.err, result.ownership = resultFailed, operation.NewError(dto.ErrorReadinessTimeout, "Lumen readiness timed out"), true
	case <-workerCtx.Done():
		result.kind, result.err, result.ownership = resultFailed, workerCtx.Err(), true
	}
	if result.kind == resultFailed && owned.Cancel != nil {
		owned.Cancel()
	}
	return result
}

func (c *Controller) stopWorker(result workerResult, process *ownedProcess) workerResult {
	result.process = process
	process.stopping.Store(true)
	if process.Cancel != nil {
		process.Cancel()
	}
	timer := time.NewTimer(c.stopBudget)
	defer timer.Stop()
	select {
	case <-process.completion.done:
		result.kind = resultStopped
	case <-timer.C:
		result.kind, result.err, result.ownership = resultFailed, operation.NewError(dto.ErrorStopTimeout, "Lumen process tree did not stop within the shutdown budget"), true
	}
	return result
}

func (c *Controller) restartWorker(result workerResult, process *ownedProcess, id uint64, profile string, workerCtx context.Context) workerResult {
	result.process = process
	process.stopping.Store(true)
	if process.Cancel != nil {
		process.Cancel()
	}
	if !waitCompletion(process.completion, c.stopBudget) {
		result.kind, result.err, result.ownership = resultFailed, operation.NewError(dto.ErrorStopTimeout, "Lumen process tree did not stop before restart"), true
		return result
	}
	return c.startWorker(result, id, profile, workerCtx)
}

func waitCompletion(value *completion, budget time.Duration) bool {
	timer := time.NewTimer(budget)
	defer timer.Stop()
	select {
	case <-value.done:
		return true
	case <-timer.C:
		return false
	}
}

func (c *Controller) handleResult(result workerResult, process **ownedProcess, results chan<- workerResult) {
	switch result.kind {
	case resultInstalled:
		c.mu.Lock()
		c.installed, c.version, c.profile, c.preset, c.cacheDir = true, result.version, result.profile, result.preset, result.cacheDir
		c.mu.Unlock()
		_ = c.operations.Succeed(result.operationID)
		c.commit(func(snapshot *dto.LumenSnapshot) {
			snapshot.InstallPhase = dto.LumenInstalled
			snapshot.ProcessPhase = dto.LumenStopped
			snapshot.Profile = result.profile
			snapshot.Preset = result.preset
			snapshot.CacheDir = result.cacheDir
			snapshot.Version++
		})
	case resultStarted:
		*process = result.process
		_ = c.operations.Succeed(result.operationID)
		c.commit(func(snapshot *dto.LumenSnapshot) {
			snapshot.DesiredState = dto.DesiredRunning
			snapshot.ProcessPhase = dto.LumenRunning
			snapshot.Ownership = dto.OwnershipHeld
			snapshot.Profile = result.profile
			snapshot.RecoveryCause = ""
			snapshot.Version++
		})
		c.watch(result.process, results)
		c.resetRetryAfterStable(result.process, results)
		c.watchControlStatus(result.process, results)
	case resultStatus:
		if *process != result.process {
			return
		}
		current := c.store.Get().Lumen.Control
		if current.StartedAtUnixMS == result.status.StartedAtUnixMS &&
			current.Sequence != 0 && result.status.Sequence != 0 && result.status.Sequence <= current.Sequence {
			return
		}
		c.commit(func(snapshot *dto.LumenSnapshot) {
			snapshot.Control = result.status
		})
	case resultStopped:
		if *process == result.process {
			*process = nil
		}
		if result.cancelled {
			_ = c.operations.Cancel(result.operationID, operation.NewError(dto.ErrorShutdownInProgress, "Lumen operation was cancelled for shutdown"))
		} else {
			_ = c.operations.Succeed(result.operationID)
		}
		c.commit(func(snapshot *dto.LumenSnapshot) {
			if result.setDisabled {
				snapshot.DesiredState = dto.DesiredDisabled
			}
			snapshot.ProcessPhase = dto.LumenStopped
			snapshot.Ownership = dto.OwnershipNone
			snapshot.RecoveryCause = ""
			snapshot.Control = dto.LumenControlStatus{Phase: dto.LumenControlUnspecified}
			snapshot.Version++
		})
	case resultExited:
		if *process != result.process {
			return
		}
		*process = nil
		c.commit(func(snapshot *dto.LumenSnapshot) {
			snapshot.ProcessPhase = dto.LumenFailed
			snapshot.Ownership = dto.OwnershipNone
			snapshot.RecoveryCause = dto.ErrorRuntimeNotReady
			snapshot.Control.Connected = false
			snapshot.Control.InferenceReady = false
			snapshot.Version++
		})
		if c.store.Get().Lumen.DesiredState == dto.DesiredRunning && !c.quiesced.Load() && c.retryCount < 3 {
			delays := []time.Duration{time.Second, 5 * time.Second, 20 * time.Second}
			delay := delays[c.retryCount]
			c.retryCount++
			c.commit(func(snapshot *dto.LumenSnapshot) { snapshot.ProcessPhase = dto.LumenBackoff })
			go c.scheduleRetry(delay, c.retryCount)
		}
	case resultStable:
		if *process == result.process && c.store.Get().Lumen.ProcessPhase == dto.LumenRunning && c.store.Get().Lumen.DesiredState == dto.DesiredRunning {
			c.retryCount = 0
		}
	case resultFailed:
		if result.preserve {
			c.commit(func(snapshot *dto.LumenSnapshot) {
				*snapshot = result.previous
				snapshot.Version++
			})
			if result.cancelled {
				_ = c.operations.Cancel(result.operationID, operation.NewError(dto.ErrorShutdownInProgress, "Lumen operation was cancelled for shutdown"))
			} else if result.err != nil {
				_ = c.operations.Fail(result.operationID, result.err)
			}
			c.syncOperations()
			return
		}
		if result.cancelled {
			_ = c.operations.Cancel(result.operationID, operation.NewError(dto.ErrorShutdownInProgress, "Lumen operation was cancelled for shutdown"))
		} else {
			_ = c.operations.Fail(result.operationID, result.err)
		}
		if !result.ownership {
			*process = nil
		} else if result.process != nil {
			*process = result.process
		}
		c.commit(func(snapshot *dto.LumenSnapshot) {
			if result.installing {
				snapshot.InstallPhase = dto.LumenInstallFailed
				snapshot.ProcessPhase = dto.LumenStopped
				snapshot.Ownership = dto.OwnershipNone
			} else if result.ownership {
				snapshot.ProcessPhase = dto.LumenFailed
				snapshot.Ownership = dto.OwnershipHeld
			} else {
				snapshot.ProcessPhase = dto.LumenFailed
				snapshot.Ownership = dto.OwnershipNone
			}
			snapshot.RecoveryCause = operation.ErrorCodeOf(result.err)
			snapshot.Version++
		})
	}
	c.syncOperations()
}

func (c *Controller) watchControlStatus(process *ownedProcess, results chan<- workerResult) {
	if process.Status == nil {
		return
	}
	go func() {
		for {
			select {
			case <-c.ctx.Done():
				return
			case status, ok := <-process.Status:
				if !ok {
					return
				}
				select {
				case results <- workerResult{kind: resultStatus, process: process, status: status}:
				case <-c.ctx.Done():
					return
				}
			}
		}
	}()
}

func (c *Controller) scheduleRetry(delay time.Duration, retryNumber int) {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-c.ctx.Done():
		return
	case <-timer.C:
		if c.quiesced.Load() || c.store.Get().Lumen.DesiredState != dto.DesiredRunning {
			return
		}
		_, _ = c.Start(fmt.Sprintf("lumen-auto-retry-%d", retryNumber), c.store.Get().Lumen.Version)
	}
}

func (c *Controller) resetRetryAfterStable(process *ownedProcess, results chan<- workerResult) {
	go func() {
		timer := time.NewTimer(5 * time.Minute)
		defer timer.Stop()
		select {
		case <-c.ctx.Done():
		case <-process.completion.done:
		case <-timer.C:
			select {
			case results <- workerResult{kind: resultStable, process: process}:
			case <-c.ctx.Done():
			}
		}
	}()
}

func (c *Controller) watch(process *ownedProcess, results chan<- workerResult) {
	go func() {
		<-process.completion.done
		if process.stopping.Load() {
			return
		}
		select {
		case <-c.ctx.Done():
		case results <- workerResult{kind: resultExited, process: process}:
		}
	}()
}

func (c *Controller) commit(update func(*dto.LumenSnapshot)) {
	c.store.Commit(func(snapshot *dto.DesktopSnapshot) {
		update(&snapshot.Lumen)
		*snapshot = control.ProjectCapabilities(*snapshot)
	})
}

func (c *Controller) syncOperations() {
	items := c.operations.Snapshot()
	c.store.Commit(func(snapshot *dto.DesktopSnapshot) { snapshot.Operations = items })
}

var _ interface {
	LumenSnapshot() dto.LumenSnapshot
	Quiesce(string, uint64) (dto.OperationReceipt, error)
	Start(string, uint64) (dto.OperationReceipt, error)
} = (*Controller)(nil)
