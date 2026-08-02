package shutdown

import (
	"sync"
	"testing"
	"time"

	"desktop/internal/control/dto"
	"desktop/internal/operation"
	"desktop/internal/state"
)

type fakeRuntimeOwner struct {
	mu       sync.Mutex
	snapshot dto.RuntimeSnapshot
	failStop bool
}

func (f *fakeRuntimeOwner) RuntimeSnapshot() dto.RuntimeSnapshot {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.snapshot
}

func (f *fakeRuntimeOwner) Quiesce(string, uint64) (dto.OperationReceipt, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failStop {
		return dto.OperationReceipt{}, operation.NewError(dto.ErrorStopTimeout, "fake runtime did not stop")
	}
	f.snapshot.Phase = dto.RuntimeStopped
	f.snapshot.Ownership = dto.OwnershipNone
	return dto.OperationReceipt{OperationID: "runtime-quiesce"}, nil
}

func (f *fakeRuntimeOwner) Start(string, uint64) (dto.OperationReceipt, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.snapshot.Phase = dto.RuntimeRunning
	f.snapshot.Ownership = dto.OwnershipHeld
	return dto.OperationReceipt{OperationID: "runtime-start"}, nil
}

func (f *fakeRuntimeOwner) release() {
	f.mu.Lock()
	f.snapshot.Phase = dto.RuntimeStopped
	f.snapshot.Ownership = dto.OwnershipNone
	f.mu.Unlock()
}

func waitForShutdown(t *testing.T, store *state.Store, phase dto.ShutdownPhase) dto.DesktopSnapshot {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		snapshot := store.Get()
		if snapshot.Host.Shutdown.Phase == phase {
			return snapshot
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("shutdown did not reach %q; got %#v", phase, store.Get().Host.Shutdown)
	return dto.DesktopSnapshot{}
}

func TestRequestQuitArmsOnceAndReusesActiveOperation(t *testing.T) {
	store := state.NewWithInstanceID("shutdown-test")
	operations := operation.New()
	runtimeOwner := &fakeRuntimeOwner{snapshot: dto.RuntimeSnapshot{
		Version: 1, DesiredState: dto.DesiredRunning, Phase: dto.RuntimeRunning, Ownership: dto.OwnershipHeld,
	}}
	coordinator := New(store, operations, runtimeOwner, nil)
	coordinator.SetBudget(time.Second)
	armed := make(chan struct{}, 1)
	coordinator.SetOnArmed(func() { armed <- struct{}{} })

	first, err := coordinator.RequestQuit("quit-one", 0)
	if err != nil {
		t.Fatalf("request quit: %v", err)
	}
	second, err := coordinator.RequestQuit("quit-two", 0)
	if err != nil {
		t.Fatalf("duplicate request quit: %v", err)
	}
	if second.OperationID != first.OperationID {
		t.Fatalf("duplicate operation = %q, want %q", second.OperationID, first.OperationID)
	}
	waitForShutdown(t, store, dto.ShutdownArmed)
	select {
	case <-armed:
	case <-time.After(time.Second):
		t.Fatal("shutdown did not notify armed callback")
	}
	if !coordinator.ShouldQuit() {
		t.Fatal("coordinator did not arm quit")
	}
}

func TestResumeAfterFailedShutdownRestoresDesiredRuntime(t *testing.T) {
	store := state.NewWithInstanceID("shutdown-test")
	operations := operation.New()
	runtimeOwner := &fakeRuntimeOwner{failStop: true, snapshot: dto.RuntimeSnapshot{
		Version: 1, DesiredState: dto.DesiredRunning, Phase: dto.RuntimeRunning, Ownership: dto.OwnershipHeld,
	}}
	coordinator := New(store, operations, runtimeOwner, nil)
	coordinator.SetBudget(100 * time.Millisecond)

	if _, err := coordinator.RequestQuit("quit-one", 0); err != nil {
		t.Fatalf("request quit: %v", err)
	}
	waitForShutdown(t, store, dto.ShutdownFailed)
	if coordinator.ShouldQuit() {
		t.Fatal("failed shutdown unexpectedly armed quit")
	}

	// Cleanup is a separate owner operation. Resume must not take ownership of
	// the retained process until it has actually been released.
	runtimeOwner.release()
	receipt, err := coordinator.ResumeAfterFailedShutdown("resume-one", 0)
	if err != nil {
		t.Fatalf("resume: %v", err)
	}
	if receipt.OperationID == "" {
		t.Fatal("resume did not return an operation receipt")
	}
	waitForShutdown(t, store, dto.ShutdownIdle)
	if got := runtimeOwner.RuntimeSnapshot(); got.Phase != dto.RuntimeRunning || got.Ownership != dto.OwnershipHeld {
		t.Fatalf("runtime was not restored: %#v", got)
	}
	if coordinator.ShouldQuit() {
		t.Fatal("coordinator remained armed after resume")
	}
}

var _ RuntimeOwner = (*fakeRuntimeOwner)(nil)
