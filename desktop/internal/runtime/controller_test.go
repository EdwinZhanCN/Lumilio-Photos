package runtime

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"desktop/internal/control/dto"
	"desktop/internal/operation"
	"desktop/internal/state"

	"server/app"
)

type fakeRepositoryControl struct{}

func (fakeRepositoryControl) ListStorageLocations(context.Context) ([]app.StorageLocationInfo, error) {
	return nil, nil
}

func (fakeRepositoryControl) AddStorageLocation(context.Context, string, string, string) (app.StorageLocationInfo, []string, error) {
	return app.StorageLocationInfo{}, nil, nil
}

func (fakeRepositoryControl) ResolveStorageLocationConflict(context.Context, string, string) (app.StorageLocationInfo, error) {
	return app.StorageLocationInfo{}, nil
}

func (fakeRepositoryControl) RemoveStorageLocation(context.Context, string) error { return nil }

func (fakeRepositoryControl) AttachRepository(context.Context, string) (app.RepositoryInfo, error) {
	return app.RepositoryInfo{}, nil
}

func (fakeRepositoryControl) ResolveRepositoryConflict(context.Context, string, string, string, ...string) (app.RepositoryInfo, error) {
	return app.RepositoryInfo{}, nil
}

func (fakeRepositoryControl) ListPendingHostActions(context.Context) ([]app.HostActionInfo, error) {
	return nil, nil
}
func (fakeRepositoryControl) SetHostActionExpectedVersion(context.Context, string, string, uint64) (app.HostActionInfo, error) {
	return app.HostActionInfo{}, nil
}

func (fakeRepositoryControl) ExecuteHostAction(context.Context, string, string, string, string, bool) (app.HostActionInfo, error) {
	return app.HostActionInfo{}, nil
}

func (fakeRepositoryControl) CancelHostAction(context.Context, string) (app.HostActionInfo, error) {
	return app.HostActionInfo{}, nil
}

type fakeGeneration struct {
	Generation
	done       chan error
	finishOnce sync.Once
}

func (g *fakeGeneration) finish(err error) {
	g.finishOnce.Do(func() { g.done <- err })
}

type fakeFactory struct {
	mu          sync.Mutex
	generations []*fakeGeneration
	autoStop    bool
	blockReady  bool
}

type failingDesiredState struct {
	value dto.DesiredState
}

func (s failingDesiredState) Load(context.Context) (dto.DesiredState, error) { return s.value, nil }

func (s failingDesiredState) Save(context.Context, dto.DesiredState) error {
	return errors.New("desired state write failed")
}

func (f *fakeFactory) Start(_ context.Context, id uint64) (Generation, error) {
	done := make(chan error, 1)
	ready := make(chan ReadyInfo, 1)
	if !f.blockReady {
		ready <- ReadyInfo{
			Runtime:           app.RuntimeInfo{Listen: "127.0.0.1:6680", ProductURL: "http://127.0.0.1:6680"},
			RepositoryControl: fakeRepositoryControl{},
			ManifestSHA256:    "sha256:test-manifest",
		}
	}
	generation := &fakeGeneration{Generation: Generation{
		ID:                id,
		Done:              done,
		Ready:             ready,
		RepositoryControl: fakeRepositoryControl{},
		ManifestSHA256:    "sha256:test-manifest",
		StartedAt:         time.Now(),
	}, done: done}
	generation.Cancel = func() {
		if f.autoStop {
			generation.finish(nil)
		}
	}
	f.mu.Lock()
	f.generations = append(f.generations, generation)
	f.mu.Unlock()
	return generation.Generation, nil
}

func (f *fakeFactory) last() *fakeGeneration {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.generations) == 0 {
		return nil
	}
	return f.generations[len(f.generations)-1]
}

func newTestController(t *testing.T, factory *fakeFactory, desired dto.DesiredState) (*Controller, context.CancelFunc) {
	t.Helper()
	store := state.NewWithInstanceID("test-instance")
	operations := operation.New()
	controller := NewController(Options{
		Store: store, Operations: operations, Desired: NewMemoryDesiredState(desired), Factory: factory,
		Configured: true, ReadyBudget: 100 * time.Millisecond, StopBudget: 25 * time.Millisecond,
	})
	ctx, cancel := context.WithCancel(context.Background())
	controller.StartActor(ctx)
	t.Cleanup(func() {
		cancel()
		controller.Close()
		store.Close()
	})
	return controller, cancel
}

func waitForRuntime(t *testing.T, controller *Controller, predicate func(dto.RuntimeSnapshot) bool) dto.RuntimeSnapshot {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		snapshot := controller.RuntimeSnapshot()
		if predicate(snapshot) {
			return snapshot
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("runtime did not reach expected state; got %#v", controller.RuntimeSnapshot())
	return dto.RuntimeSnapshot{}
}

func TestControllerStartStopPersistsDesiredState(t *testing.T) {
	factory := &fakeFactory{autoStop: true}
	controller, _ := newTestController(t, factory, dto.DesiredStopped)

	start, err := controller.Start("start-request", 0)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if start.OperationID == "" {
		t.Fatal("start did not return an operation receipt")
	}
	running := waitForRuntime(t, controller, func(snapshot dto.RuntimeSnapshot) bool {
		return snapshot.Phase == dto.RuntimeRunning
	})
	if running.Ownership != dto.OwnershipHeld || running.ProductURL == "" {
		t.Fatalf("running snapshot lost ownership or readiness: %#v", running)
	}
	if got := factory.last().ID; got != 1 {
		t.Fatalf("generation ID = %d, want 1", got)
	}

	if _, err := controller.Stop("stop-request", running.Version); err != nil {
		t.Fatalf("stop: %v", err)
	}
	stopped := waitForRuntime(t, controller, func(snapshot dto.RuntimeSnapshot) bool {
		return snapshot.Phase == dto.RuntimeStopped && snapshot.Ownership == dto.OwnershipNone
	})
	if stopped.DesiredState != dto.DesiredStopped {
		t.Fatalf("desired state = %q, want stopped", stopped.DesiredState)
	}
	if desired, err := controller.desired.Load(context.Background()); err != nil || desired != dto.DesiredStopped {
		t.Fatalf("persisted desired state = %q, err = %v", desired, err)
	}
}

func TestControllerDesiredStateWriteFailurePreservesStoppedRuntime(t *testing.T) {
	store := state.NewWithInstanceID("desired-write-failure")
	operations := operation.New()
	factory := &fakeFactory{}
	controller := NewController(Options{
		Store: store, Operations: operations, Desired: failingDesiredState{value: dto.DesiredStopped}, Factory: factory,
		Configured: true, ReadyBudget: 100 * time.Millisecond, StopBudget: 25 * time.Millisecond,
	})
	ctx, cancel := context.WithCancel(context.Background())
	controller.StartActor(ctx)
	t.Cleanup(func() {
		cancel()
		controller.Close()
		operations.Close()
		store.Close()
	})

	receipt, err := controller.Start("start-write-failure", 0)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	waitForRuntime(t, controller, func(snapshot dto.RuntimeSnapshot) bool {
		item, ok := operations.Get(receipt.OperationID)
		return ok && item.State == string(operation.Failed) && snapshot.Phase == dto.RuntimeStopped
	})
	snapshot := controller.RuntimeSnapshot()
	if snapshot.Phase != dto.RuntimeStopped || snapshot.Ownership != dto.OwnershipNone || snapshot.DesiredState != dto.DesiredStopped {
		t.Fatalf("runtime changed after desired-state write failure: %#v", snapshot)
	}
	if factory.last() != nil {
		t.Fatal("runtime factory was called after desired-state write failure")
	}
}

func TestControllerStopTimeoutRetainsOwnershipUntilRetryCleanup(t *testing.T) {
	factory := &fakeFactory{}
	controller, _ := newTestController(t, factory, dto.DesiredStopped)

	if _, err := controller.Start("start-request", 0); err != nil {
		t.Fatalf("start: %v", err)
	}
	running := waitForRuntime(t, controller, func(snapshot dto.RuntimeSnapshot) bool {
		return snapshot.Phase == dto.RuntimeRunning
	})
	if _, err := controller.Stop("stop-request", running.Version); err != nil {
		t.Fatalf("stop: %v", err)
	}
	failed := waitForRuntime(t, controller, func(snapshot dto.RuntimeSnapshot) bool {
		return snapshot.Phase == dto.RuntimeFailed && snapshot.Ownership == dto.OwnershipHeld
	})
	if failed.RecoveryCause != dto.ErrorStopTimeout {
		t.Fatalf("recovery cause = %q, want %q", failed.RecoveryCause, dto.ErrorStopTimeout)
	}
	blockedReceipt, err := controller.Start("blocked-start", failed.Version)
	if err != nil {
		t.Fatalf("start should be accepted for actor-level ownership check: %v", err)
	}
	blocked := waitForRuntime(t, controller, func(snapshot dto.RuntimeSnapshot) bool {
		item, ok := controller.operations.Get(blockedReceipt.OperationID)
		return ok && item.State == string(operation.Failed)
	})
	if code := controller.operationsErrorCode(blockedReceipt.OperationID); code != dto.ErrorRuntimeNotReady {
		t.Fatalf("blocked start error code = %q, want %q", code, dto.ErrorRuntimeNotReady)
	}

	factory.last().finish(nil)
	if _, err := controller.RetryCleanup("cleanup-request", blocked.Version); err != nil {
		t.Fatalf("retry cleanup: %v", err)
	}
	waitForRuntime(t, controller, func(snapshot dto.RuntimeSnapshot) bool {
		return snapshot.Phase == dto.RuntimeStopped && snapshot.Ownership == dto.OwnershipNone
	})
}

func TestControllerUnexpectedExitClearsOwnershipAndKeepsDesiredRunning(t *testing.T) {
	factory := &fakeFactory{}
	controller, _ := newTestController(t, factory, dto.DesiredStopped)
	if _, err := controller.Start("start-request", 0); err != nil {
		t.Fatalf("start: %v", err)
	}
	waitForRuntime(t, controller, func(snapshot dto.RuntimeSnapshot) bool {
		return snapshot.Phase == dto.RuntimeRunning
	})
	factory.last().finish(errors.New("fake server crashed"))
	failed := waitForRuntime(t, controller, func(snapshot dto.RuntimeSnapshot) bool {
		return snapshot.Phase == dto.RuntimeFailed && snapshot.Ownership == dto.OwnershipNone
	})
	if failed.DesiredState != dto.DesiredRunning {
		t.Fatalf("desired state = %q, want running after unexpected exit", failed.DesiredState)
	}
	if failed.RecoveryCause != dto.ErrorRuntimeNotReady {
		t.Fatalf("recovery cause = %q, want %q", failed.RecoveryCause, dto.ErrorRuntimeNotReady)
	}
}

func TestControllerQuiescePreemptsReadinessWait(t *testing.T) {
	factory := &fakeFactory{autoStop: true, blockReady: true}
	controller, _ := newTestController(t, factory, dto.DesiredStopped)
	start, err := controller.Start("start-request", 0)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	waitForRuntime(t, controller, func(snapshot dto.RuntimeSnapshot) bool {
		return snapshot.Phase == dto.RuntimeStarting
	})
	quiesce, err := controller.Quiesce("quiesce-request", 0)
	if err != nil {
		t.Fatalf("quiesce: %v", err)
	}
	waitForRuntime(t, controller, func(snapshot dto.RuntimeSnapshot) bool {
		return snapshot.Phase == dto.RuntimeStopped && snapshot.Ownership == dto.OwnershipNone
	})
	if item, ok := controller.operations.Get(start.OperationID); !ok || item.State != string(operation.Cancelled) {
		t.Fatalf("start operation = %#v, want cancelled", item)
	}
	if item, ok := controller.operations.Get(quiesce.OperationID); !ok || item.State != string(operation.Succeeded) {
		t.Fatalf("quiesce operation = %#v, want succeeded", item)
	}
}

// operationsError is intentionally test-only: controller operations are
// private to the actor, but the registry remains the source of operation DTOs.
func (c *Controller) operationsErrorCode(operationID string) dto.ErrorCode {
	item, ok := c.operations.Get(operationID)
	if !ok {
		return ""
	}
	return item.Error.Code
}
