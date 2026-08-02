package lumen

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"desktop/internal/control/dto"
	"desktop/internal/operation"
	"desktop/internal/state"
)

type fakeProcess struct {
	Process
	done       chan error
	finishOnce sync.Once
}

func (p *fakeProcess) finish(err error) { p.finishOnce.Do(func() { p.done <- err }) }

type fakeFactory struct {
	mu         sync.Mutex
	processes  []*fakeProcess
	autoStop   bool
	blockReady bool
}

func (f *fakeFactory) Start(_ context.Context, id uint64, profile string) (Process, error) {
	done := make(chan error, 1)
	ready := make(chan ReadyInfo, 1)
	if !f.blockReady {
		ready <- ReadyInfo{Endpoint: "unix:///tmp/lumen-test.sock"}
	}
	process := &fakeProcess{Process: Process{ID: id, Done: done, Ready: ready, Profile: profile}, done: done}
	process.Cancel = func() {
		if f.autoStop {
			process.finish(nil)
		}
	}
	f.mu.Lock()
	f.processes = append(f.processes, process)
	f.mu.Unlock()
	return process.Process, nil
}

func (f *fakeFactory) last() *fakeProcess {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.processes[len(f.processes)-1]
}

type fakeInstaller struct{}

func (fakeInstaller) Install(context.Context, string) (string, error) { return "lumen-test-v1", nil }

type failingDesiredState struct {
	value dto.DesiredState
}

func (s failingDesiredState) Load(context.Context) (dto.DesiredState, error) { return s.value, nil }

func (s failingDesiredState) Save(context.Context, dto.DesiredState) error {
	return errors.New("desired state write failed")
}

func newTestController(t *testing.T, factory *fakeFactory) *Controller {
	t.Helper()
	store := state.NewWithInstanceID("lumen-test")
	controller := NewController(Options{
		Store: store, Operations: operation.New(), Desired: NewMemoryDesiredState(dto.DesiredDisabled),
		Factory: factory, Installer: fakeInstaller{}, Installed: true, InstalledVer: "lumen-test-v1", Profile: "balanced",
		ReadyBudget: 100 * time.Millisecond, StopBudget: 20 * time.Millisecond,
	})
	ctx, cancel := context.WithCancel(context.Background())
	controller.StartActor(ctx)
	t.Cleanup(func() {
		cancel()
		controller.Close()
		store.Close()
	})
	return controller
}

func waitFor(t *testing.T, controller *Controller, predicate func(dto.LumenSnapshot) bool) dto.LumenSnapshot {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		snapshot := controller.Snapshot()
		if predicate(snapshot) {
			return snapshot
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("Lumen did not reach expected state: %#v", controller.Snapshot())
	return dto.LumenSnapshot{}
}

func TestControllerStartStopAndInstall(t *testing.T) {
	factory := &fakeFactory{autoStop: true}
	controller := newTestController(t, factory)

	install, err := controller.Install("install", 0, "balanced")
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	waitFor(t, controller, func(snapshot dto.LumenSnapshot) bool {
		item, ok := controller.operations.Get(install.OperationID)
		return ok && item.State == string(operation.Succeeded)
	})
	if got := controller.Snapshot().InstallPhase; got != dto.LumenInstalled {
		t.Fatalf("install phase = %q, want installed", got)
	}

	if _, err := controller.Start("start", 0); err != nil {
		t.Fatalf("start: %v", err)
	}
	running := waitFor(t, controller, func(snapshot dto.LumenSnapshot) bool {
		return snapshot.ProcessPhase == dto.LumenRunning
	})
	if running.Ownership != dto.OwnershipHeld || running.DesiredState != dto.DesiredRunning {
		t.Fatalf("running snapshot = %#v", running)
	}
	if _, err := controller.Stop("stop", running.Version); err != nil {
		t.Fatalf("stop: %v", err)
	}
	stopped := waitFor(t, controller, func(snapshot dto.LumenSnapshot) bool {
		return snapshot.ProcessPhase == dto.LumenStopped && snapshot.Ownership == dto.OwnershipNone
	})
	if stopped.DesiredState != dto.DesiredDisabled {
		t.Fatalf("desired state = %q, want disabled", stopped.DesiredState)
	}
}

func TestControllerDesiredStateWriteFailurePreservesStoppedLumen(t *testing.T) {
	store := state.NewWithInstanceID("lumen-desired-write-failure")
	operations := operation.New()
	factory := &fakeFactory{}
	controller := NewController(Options{
		Store: store, Operations: operations, Desired: failingDesiredState{value: dto.DesiredDisabled},
		Factory: factory, Installed: true, InstalledVer: "lumen-test-v1", Profile: "balanced",
		ReadyBudget: 100 * time.Millisecond, StopBudget: 20 * time.Millisecond,
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
	waitFor(t, controller, func(snapshot dto.LumenSnapshot) bool {
		item, ok := operations.Get(receipt.OperationID)
		return ok && item.State == string(operation.Failed) && snapshot.ProcessPhase == dto.LumenStopped
	})
	snapshot := controller.Snapshot()
	if snapshot.ProcessPhase != dto.LumenStopped || snapshot.Ownership != dto.OwnershipNone || snapshot.DesiredState != dto.DesiredDisabled {
		t.Fatalf("Lumen changed after desired-state write failure: %#v", snapshot)
	}
	if len(factory.processes) != 0 {
		t.Fatal("Lumen factory was called after desired-state write failure")
	}
}

func TestControllerStopTimeoutRetainsOwnership(t *testing.T) {
	factory := &fakeFactory{}
	controller := newTestController(t, factory)
	controller.SetInstalled("lumen-test-v1", "balanced")
	if _, err := controller.Start("start", 0); err != nil {
		t.Fatalf("start: %v", err)
	}
	running := waitFor(t, controller, func(snapshot dto.LumenSnapshot) bool {
		return snapshot.ProcessPhase == dto.LumenRunning
	})
	if _, err := controller.Stop("stop", running.Version); err != nil {
		t.Fatalf("stop: %v", err)
	}
	failed := waitFor(t, controller, func(snapshot dto.LumenSnapshot) bool {
		return snapshot.ProcessPhase == dto.LumenFailed && snapshot.Ownership == dto.OwnershipHeld
	})
	if failed.RecoveryCause != dto.ErrorStopTimeout {
		t.Fatalf("recovery cause = %q, want %q", failed.RecoveryCause, dto.ErrorStopTimeout)
	}
	factory.last().finish(nil)
	if _, err := controller.RetryCleanup("cleanup", failed.Version); err != nil {
		t.Fatalf("retry cleanup: %v", err)
	}
	waitFor(t, controller, func(snapshot dto.LumenSnapshot) bool {
		return snapshot.ProcessPhase == dto.LumenStopped && snapshot.Ownership == dto.OwnershipNone
	})
}

func TestControllerCrashPreservesRunningIntent(t *testing.T) {
	factory := &fakeFactory{}
	controller := newTestController(t, factory)
	if _, err := controller.Start("start", 0); err != nil {
		t.Fatalf("start: %v", err)
	}
	waitFor(t, controller, func(snapshot dto.LumenSnapshot) bool {
		return snapshot.ProcessPhase == dto.LumenRunning
	})
	factory.last().finish(errors.New("hub crashed"))
	failed := waitFor(t, controller, func(snapshot dto.LumenSnapshot) bool {
		return (snapshot.ProcessPhase == dto.LumenFailed || snapshot.ProcessPhase == dto.LumenBackoff) && snapshot.Ownership == dto.OwnershipNone
	})
	if failed.DesiredState != dto.DesiredRunning || failed.Ownership != dto.OwnershipNone {
		t.Fatalf("crashed snapshot = %#v", failed)
	}
}

func TestControllerQuiescePreemptsReadinessWait(t *testing.T) {
	factory := &fakeFactory{autoStop: true, blockReady: true}
	controller := newTestController(t, factory)
	start, err := controller.Start("start-request", 0)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	waitFor(t, controller, func(snapshot dto.LumenSnapshot) bool {
		return snapshot.ProcessPhase == dto.LumenStarting
	})
	quiesce, err := controller.Quiesce("quiesce-request", 0)
	if err != nil {
		t.Fatalf("quiesce: %v", err)
	}
	waitFor(t, controller, func(snapshot dto.LumenSnapshot) bool {
		return snapshot.ProcessPhase == dto.LumenStopped && snapshot.Ownership == dto.OwnershipNone
	})
	if item, ok := controller.operations.Get(start.OperationID); !ok || item.State != string(operation.Cancelled) {
		t.Fatalf("start operation = %#v, want cancelled", item)
	}
	if item, ok := controller.operations.Get(quiesce.OperationID); !ok || item.State != string(operation.Succeeded) {
		t.Fatalf("quiesce operation = %#v, want succeeded", item)
	}
}
