package runtimeconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"desktop/internal/control/dto"
	"desktop/internal/operation"
	"desktop/internal/platform"
	"desktop/internal/state"
)

type fakeLifecycle struct {
	store *state.Store
}

func (f *fakeLifecycle) Snapshot() dto.DesktopSnapshot { return f.store.Get() }

func (f *fakeLifecycle) Restart(string, uint64) (dto.OperationReceipt, error) {
	f.store.Commit(func(snapshot *dto.DesktopSnapshot) {
		snapshot.Runtime.Phase = dto.RuntimeRunning
		snapshot.Runtime.Ownership = dto.OwnershipHeld
		snapshot.Runtime.Version++
	})
	return dto.OperationReceipt{OperationID: "restart"}, nil
}

func testConfigCandidate(t *testing.T) string {
	t.Helper()
	path := filepath.Join("..", "..", "..", "..", "server", "config", "examples", "dev", "vite.toml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config fixture: %v", err)
	}
	return string(data)
}

func TestTransactionValidateAndSaveTracksFingerprint(t *testing.T) {
	paths, err := platform.NewPaths(filepath.Join(t.TempDir(), "app-data"))
	if err != nil {
		t.Fatal(err)
	}
	if err := paths.Ensure(); err != nil {
		t.Fatal(err)
	}
	store := state.NewWithInstanceID("config-test")
	operations := operation.New()
	configStore := NewStore(paths)
	controller := NewTransactionController(configStore, store, operations, &fakeLifecycle{store: store})
	candidate := testConfigCandidate(t)
	validation, err := controller.Validate(candidate)
	if err != nil || !validation.Valid || validation.Fingerprint == "" {
		t.Fatalf("validation = %#v, err = %v", validation, err)
	}
	receipt, err := controller.Save("save-1", store.Get().Runtime.Version, "", candidate)
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if receipt.OperationID == "" {
		t.Fatal("save did not return receipt")
	}
	current, err := configStore.CurrentPointer()
	if err != nil || current.Fingerprint != validation.Fingerprint {
		t.Fatalf("current pointer = %#v, err = %v", current, err)
	}
	if got := store.Get().Runtime.PendingConfigValidation; !got {
		t.Fatal("save did not mark pending validation")
	}
	if item, ok := operations.Get(receipt.OperationID); !ok || item.State != string(operation.Succeeded) {
		t.Fatalf("operation = %#v, ok = %v", item, ok)
	}
}

func TestTransactionRejectsStaleBaseFingerprint(t *testing.T) {
	paths, err := platform.NewPaths(filepath.Join(t.TempDir(), "app-data"))
	if err != nil {
		t.Fatal(err)
	}
	if err := paths.Ensure(); err != nil {
		t.Fatal(err)
	}
	store := state.NewWithInstanceID("config-test")
	controller := NewTransactionController(NewStore(paths), store, operation.New(), nil)
	_, err = controller.Save("save-1", 0, "sha256:"+strings.Repeat("0", 64), testConfigCandidate(t))
	if operation.ErrorCodeOf(err) != dto.ErrorStaleConfig {
		t.Fatalf("stale base error = %v, code = %q", err, operation.ErrorCodeOf(err))
	}
}

// onboardingLifecycle is the lifecycle surface the first-run Save uses: the
// real runtime controller is marked configured and started once the intent
// and pointers are durable.
type onboardingLifecycle struct {
	fakeLifecycle
	configured []bool
	starts     []string
}

func (f *onboardingLifecycle) SetConfigured(configured bool) {
	f.configured = append(f.configured, configured)
	f.store.Commit(func(snapshot *dto.DesktopSnapshot) {
		snapshot.Runtime.Configured = configured
		snapshot.Runtime.Version++
	})
}

func (f *onboardingLifecycle) Start(requestID string, _ uint64) (dto.OperationReceipt, error) {
	f.starts = append(f.starts, requestID)
	return dto.OperationReceipt{OperationID: "start-" + requestID}, nil
}

// The first launch has no settings file, no current pointer and no last
// known-good pointer. The wizard's Save must leave the runtime configured,
// desired running and started exactly once, with onboarding recorded in the
// settings the next launch reads.
func TestTransactionFirstSaveCompletesOnboardingAndStartsRuntime(t *testing.T) {
	paths, err := platform.NewPaths(filepath.Join(t.TempDir(), "app-data"))
	if err != nil {
		t.Fatal(err)
	}
	if err := paths.Ensure(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(paths.SettingsFile); !os.IsNotExist(err) {
		t.Fatalf("first launch must start without a settings file: %v", err)
	}
	store := state.NewWithInstanceID("onboarding-test")
	operations := operation.New()
	configStore := NewStore(paths)
	lifecycle := &onboardingLifecycle{fakeLifecycle: fakeLifecycle{store: store}}
	controller := NewTransactionController(configStore, store, operations, lifecycle)
	candidate := testConfigCandidate(t)

	receipt, err := controller.Save("onboarding-save", store.Get().Runtime.Version, "", candidate)
	if err != nil {
		t.Fatalf("first save: %v", err)
	}
	if item, ok := operations.Get(receipt.OperationID); !ok || item.State != string(operation.Succeeded) {
		t.Fatalf("operation = %#v, ok = %v", item, ok)
	}
	settings, err := LoadSettings(paths.SettingsFile)
	if err != nil {
		t.Fatalf("load settings written by the first save: %v", err)
	}
	if !settings.OnboardingComplete || settings.RuntimeDesiredState != dto.DesiredRunning {
		t.Fatalf("first save must record onboarding and a running desired state, got %#v", settings)
	}
	if len(lifecycle.configured) != 1 || !lifecycle.configured[0] {
		t.Fatalf("first save must mark the runtime configured once, got %v", lifecycle.configured)
	}
	if len(lifecycle.starts) != 1 || lifecycle.starts[0] != "onboarding-runtime-"+receipt.OperationID {
		t.Fatalf("first save must start the runtime once under its operation, got %v", lifecycle.starts)
	}
	current, err := configStore.CurrentPointer()
	if err != nil || current.Fingerprint == "" {
		t.Fatalf("current pointer = %#v, err = %v", current, err)
	}
	lkg, err := configStore.LastKnownGoodPointer()
	if err != nil || lkg.Fingerprint != "" {
		t.Fatalf("the first intent is not known-good until the runtime is ready, got %#v, err = %v", lkg, err)
	}
	snapshot := store.Get().Runtime
	if !snapshot.Configured || !snapshot.PendingConfigValidation || snapshot.Phase != dto.RuntimeStopped {
		t.Fatalf("runtime snapshot after the first save = %#v", snapshot)
	}

	// A later Save on the configured, stopped runtime is an ordinary
	// transaction: it neither re-runs onboarding nor starts the runtime.
	draft, err := configStore.ReadDraft()
	if err != nil {
		t.Fatal(err)
	}
	next := strings.Replace(candidate, `level = "debug"`, `level = "info"`, 1)
	if next == candidate {
		t.Fatal("test fixture no longer carries the expected logging level")
	}
	if _, err := controller.Save("second-save", store.Get().Runtime.Version, draft.BaseFingerprint, next); err != nil {
		t.Fatalf("second save: %v", err)
	}
	if len(lifecycle.configured) != 1 || len(lifecycle.starts) != 1 {
		t.Fatalf("a configured save must not re-run onboarding: configured = %v, starts = %v", lifecycle.configured, lifecycle.starts)
	}
}
