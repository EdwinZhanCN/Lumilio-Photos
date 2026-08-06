package lumen

import (
	"context"
	"errors"
	"path/filepath"
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
	mu          sync.Mutex
	processes   []*fakeProcess
	statuses    <-chan dto.LumenControlStatus
	autoStop    bool
	blockReady  bool
	starts      int
	startErrors map[int]error
}

func (f *fakeFactory) Start(_ context.Context, id uint64, profile string) (Process, error) {
	f.mu.Lock()
	f.starts++
	attempt := f.starts
	startErr := f.startErrors[attempt]
	f.mu.Unlock()
	if startErr != nil {
		return Process{}, startErr
	}
	done := make(chan error, 1)
	ready := make(chan ReadyInfo, 1)
	if !f.blockReady {
		ready <- ReadyInfo{Endpoint: "unix:///tmp/lumen-test.sock"}
	}
	process := &fakeProcess{Process: Process{ID: id, Done: done, Ready: ready, Status: f.statuses, Profile: profile}, done: done}
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

type recordingSetupStore struct {
	mu       sync.Mutex
	preset   string
	cacheDir string
	calls    [][2]string
}

func (s *recordingSetupStore) SaveSetup(_ context.Context, preset, cacheDir string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.preset, s.cacheDir = preset, cacheDir
	s.calls = append(s.calls, [2]string{preset, cacheDir})
	return nil
}

func (s *recordingSetupStore) snapshot() (string, string, [][2]string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.preset, s.cacheDir, append([][2]string(nil), s.calls...)
}

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
		Preset: "basic", Presets: []string{"minimal", "basic", "brave"},
		CacheDir:    t.TempDir(),
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

	cacheDir := controller.Snapshot().CacheDir
	wantCacheDir, err := CanonicalCacheDirectory(cacheDir)
	if err != nil {
		t.Fatal(err)
	}
	install, err := controller.Install("install", 0, "balanced", "brave", cacheDir)
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
	if got := controller.Snapshot().Preset; got != "brave" {
		t.Fatalf("preset = %q, want brave", got)
	}
	if got := controller.Snapshot().CacheDir; got != wantCacheDir {
		t.Fatalf("cache dir = %q, want %q", got, wantCacheDir)
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

func TestControllerRejectsUnavailableInstallChoices(t *testing.T) {
	controller := newTestController(t, &fakeFactory{})
	cacheDir := controller.Snapshot().CacheDir
	if _, err := controller.Install("bad-profile", 0, "windows-x64-gpu", "basic", cacheDir); operation.ErrorCodeOf(err) != dto.ErrorInvalidArgument {
		t.Fatalf("profile error = %v", err)
	}
	if _, err := controller.Install("bad-preset", 0, "balanced", "huge", cacheDir); operation.ErrorCodeOf(err) != dto.ErrorInvalidArgument {
		t.Fatalf("preset error = %v", err)
	}
	if _, err := controller.Install("bad-cache", 0, "balanced", "basic", string(filepath.Separator)); operation.ErrorCodeOf(err) != dto.ErrorInvalidArgument {
		t.Fatalf("cache error = %v", err)
	}
}

func TestControllerPicksCanonicalCacheDirectory(t *testing.T) {
	controller := newTestController(t, &fakeFactory{})
	want := t.TempDir()
	wantCanonical, err := CanonicalCacheDirectory(want)
	if err != nil {
		t.Fatal(err)
	}
	controller.SetPickDirectory(func(title string) (string, error) {
		if title != "Choose cache" {
			t.Fatalf("picker title = %q", title)
		}
		return want, nil
	})
	got, err := controller.PickCacheDirectory("Choose cache")
	if err != nil {
		t.Fatal(err)
	}
	if got != wantCanonical {
		t.Fatalf("picked cache = %q, want %q", got, wantCanonical)
	}
}

func TestControllerValidatesControlLogRequests(t *testing.T) {
	controller := newTestController(t, &fakeFactory{})
	if _, err := controller.Logs(200, "INFO"); operation.ErrorCodeOf(err) != dto.ErrorRuntimeNotReady {
		t.Fatalf("stopped log error = %v", err)
	}
	if _, err := controller.Start("start-for-logs", 0); err != nil {
		t.Fatal(err)
	}
	waitFor(t, controller, func(snapshot dto.LumenSnapshot) bool { return snapshot.ProcessPhase == dto.LumenRunning })
	if _, err := controller.Logs(501, "INFO"); operation.ErrorCodeOf(err) != dto.ErrorInvalidArgument {
		t.Fatalf("backlog error = %v", err)
	}
	if _, err := controller.Logs(20, "VERBOSE"); operation.ErrorCodeOf(err) != dto.ErrorInvalidArgument {
		t.Fatalf("level error = %v", err)
	}
}

func TestControllerPublishesControlStatusWithoutInvalidatingLifecycleVersion(t *testing.T) {
	statuses := make(chan dto.LumenControlStatus, 3)
	controller := newTestController(t, &fakeFactory{statuses: statuses})
	if _, err := controller.Start("start-for-status", 0); err != nil {
		t.Fatal(err)
	}
	running := waitFor(t, controller, func(snapshot dto.LumenSnapshot) bool {
		return snapshot.ProcessPhase == dto.LumenRunning
	})

	statuses <- dto.LumenControlStatus{
		Connected: true, Phase: dto.LumenControlDownloading,
		StartedAtUnixMS: 100, Sequence: 2,
	}
	observed := waitFor(t, controller, func(snapshot dto.LumenSnapshot) bool {
		return snapshot.Control.Sequence == 2
	})
	if observed.Version != running.Version {
		t.Fatalf("control status changed lifecycle version from %d to %d", running.Version, observed.Version)
	}

	statuses <- dto.LumenControlStatus{
		Connected: true, Phase: dto.LumenControlStarting,
		StartedAtUnixMS: 100, Sequence: 1,
	}
	time.Sleep(10 * time.Millisecond)
	if got := controller.Snapshot().Control.Sequence; got != 2 {
		t.Fatalf("stale control sequence replaced current sequence: got %d, want 2", got)
	}

	statuses <- dto.LumenControlStatus{
		Connected: true, Phase: dto.LumenControlReady,
		StartedAtUnixMS: 200, Sequence: 1,
	}
	newProcessStatus := waitFor(t, controller, func(snapshot dto.LumenSnapshot) bool {
		return snapshot.Control.StartedAtUnixMS == 200
	})
	if newProcessStatus.Control.Sequence != 1 {
		t.Fatalf("new process sequence = %d, want 1", newProcessStatus.Control.Sequence)
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

func TestControllerReconfiguresStoppedLumenAfterValidation(t *testing.T) {
	store := &recordingSetupStore{preset: "basic"}
	validated := false
	controller := NewController(Options{
		Store: state.NewWithInstanceID("lumen-reconfigure-stopped"), Operations: operation.New(),
		Desired: NewMemoryDesiredState(dto.DesiredDisabled), Factory: &fakeFactory{},
		Installer: fakeInstaller{}, SetupStore: store,
		ValidateSetup: func(_ context.Context, preset, cacheDir string) error {
			validated = preset == "brave" && cacheDir != ""
			return nil
		},
		Installed: true, InstalledVer: "lumen-test-v1", Profile: "balanced", Preset: "basic",
		CacheDir: t.TempDir(), Profiles: []string{"balanced"}, Presets: []string{"minimal", "basic", "brave"},
		ReadyBudget: 100 * time.Millisecond, StopBudget: 20 * time.Millisecond,
	})
	ctx, cancel := context.WithCancel(context.Background())
	controller.StartActor(ctx)
	t.Cleanup(func() { cancel(); controller.Close(); controller.store.Close(); controller.operations.Close() })

	newCache := t.TempDir()
	receipt, err := controller.Install("configure-stopped", 0, "balanced", "brave", newCache)
	if err != nil {
		t.Fatal(err)
	}
	waitFor(t, controller, func(snapshot dto.LumenSnapshot) bool {
		item, ok := controller.operations.Get(receipt.OperationID)
		return ok && item.State == string(operation.Succeeded) && snapshot.Preset == "brave"
	})
	if !validated {
		t.Fatal("candidate setup was not validated")
	}
	preset, _, calls := store.snapshot()
	if preset != "brave" || len(calls) != 1 {
		t.Fatalf("saved setup = %q, calls=%v", preset, calls)
	}
}

func TestControllerReconfiguresRunningLumenWithControlledRestart(t *testing.T) {
	factory := &fakeFactory{autoStop: true}
	setup := &recordingSetupStore{preset: "basic"}
	controller := newTestController(t, factory)
	controller.setupStore = setup
	controller.validateSetup = func(context.Context, string, string) error { return nil }
	if _, err := controller.Start("start-before-configure", 0); err != nil {
		t.Fatal(err)
	}
	running := waitFor(t, controller, func(snapshot dto.LumenSnapshot) bool { return snapshot.ProcessPhase == dto.LumenRunning })
	newCache := t.TempDir()
	receipt, err := controller.Install("configure-running", running.Version, "balanced", "minimal", newCache)
	if err != nil {
		t.Fatal(err)
	}
	updated := waitFor(t, controller, func(snapshot dto.LumenSnapshot) bool {
		item, ok := controller.operations.Get(receipt.OperationID)
		return ok && item.State == string(operation.Succeeded) && snapshot.ProcessPhase == dto.LumenRunning && snapshot.Preset == "minimal"
	})
	if updated.DesiredState != dto.DesiredRunning {
		t.Fatalf("desired = %q", updated.DesiredState)
	}
	factory.mu.Lock()
	starts := factory.starts
	factory.mu.Unlock()
	if starts != 2 {
		t.Fatalf("start attempts = %d, want 2", starts)
	}
}

func TestControllerFailedReconfigureRestoresPreviousRunningSetup(t *testing.T) {
	factory := &fakeFactory{autoStop: true, startErrors: map[int]error{2: errors.New("candidate failed")}}
	setup := &recordingSetupStore{preset: "basic"}
	controller := newTestController(t, factory)
	controller.setupStore = setup
	controller.validateSetup = func(context.Context, string, string) error { return nil }
	if _, err := controller.Start("start-before-rollback", 0); err != nil {
		t.Fatal(err)
	}
	running := waitFor(t, controller, func(snapshot dto.LumenSnapshot) bool { return snapshot.ProcessPhase == dto.LumenRunning })
	oldCache := running.CacheDir
	receipt, err := controller.Install("configure-with-rollback", running.Version, "balanced", "brave", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	restored := waitFor(t, controller, func(snapshot dto.LumenSnapshot) bool {
		item, ok := controller.operations.Get(receipt.OperationID)
		return ok && item.State == string(operation.Failed) && snapshot.ProcessPhase == dto.LumenRunning
	})
	if restored.Preset != "basic" || restored.CacheDir != oldCache || restored.DesiredState != dto.DesiredRunning {
		t.Fatalf("restored snapshot = %#v", restored)
	}
	preset, cacheDir, calls := setup.snapshot()
	if preset != "basic" || cacheDir != oldCache || len(calls) != 2 {
		t.Fatalf("rollback setup = %q %q, calls=%v", preset, cacheDir, calls)
	}
	factory.mu.Lock()
	starts := factory.starts
	factory.mu.Unlock()
	if starts != 3 {
		t.Fatalf("start attempts = %d, want 3", starts)
	}
}

func TestControllerRejectedCandidateLeavesRunningGenerationUntouched(t *testing.T) {
	factory := &fakeFactory{autoStop: true}
	setup := &recordingSetupStore{preset: "basic"}
	controller := newTestController(t, factory)
	controller.setupStore = setup
	controller.validateSetup = func(context.Context, string, string) error { return errors.New("invalid candidate") }
	if _, err := controller.Start("start-before-validation", 0); err != nil {
		t.Fatal(err)
	}
	running := waitFor(t, controller, func(snapshot dto.LumenSnapshot) bool { return snapshot.ProcessPhase == dto.LumenRunning })
	receipt, err := controller.Install("invalid-configure", running.Version, "balanced", "brave", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	unchanged := waitFor(t, controller, func(snapshot dto.LumenSnapshot) bool {
		item, ok := controller.operations.Get(receipt.OperationID)
		return ok && item.State == string(operation.Failed) && snapshot.ProcessPhase == dto.LumenRunning
	})
	if unchanged.Preset != "basic" {
		t.Fatalf("preset changed: %#v", unchanged)
	}
	_, _, calls := setup.snapshot()
	if len(calls) != 0 {
		t.Fatalf("invalid candidate was persisted: %v", calls)
	}
	factory.mu.Lock()
	starts := factory.starts
	factory.mu.Unlock()
	if starts != 1 {
		t.Fatalf("running process was restarted: starts=%d", starts)
	}
}
