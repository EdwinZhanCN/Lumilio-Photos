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
