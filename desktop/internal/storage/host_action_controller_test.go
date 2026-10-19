package storage

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"desktop/internal/control/dto"
	"desktop/internal/operation"
	"desktop/internal/platform"
	"desktop/internal/runtime"
	"desktop/internal/state"
)

// fakeStorageControl is the embedded host-action harness: the Desktop-side
// projection of Server's in-process control with the Server replaced by a
// recorder, so the picker-controller paths are proven without a runtime.
type fakeStorageControl struct {
	mu       sync.Mutex
	actions  []runtime.HostAction
	rebinds  []uint64
	executed []executedHostAction
}

type executedHostAction struct {
	actionID, nonce, hostInstanceID, selectedPath string
	acknowledged                                  []string
}

func (f *fakeStorageControl) ListStorageLocations(context.Context) ([]runtime.StorageLocation, error) {
	return nil, nil
}

func (f *fakeStorageControl) ListPendingHostActions(context.Context) ([]runtime.HostAction, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]runtime.HostAction(nil), f.actions...), nil
}

func (f *fakeStorageControl) SetHostActionExpectedVersion(_ context.Context, actionID, nonce string, version uint64) (runtime.HostAction, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for index := range f.actions {
		if f.actions[index].ID == actionID && f.actions[index].Nonce == nonce {
			f.actions[index].ExpectedVersion = version
			f.rebinds = append(f.rebinds, version)
			return f.actions[index], nil
		}
	}
	return runtime.HostAction{}, errors.New("unknown host action")
}

func (f *fakeStorageControl) ExecuteHostAction(_ context.Context, actionID, nonce, hostInstanceID, selectedPath string, acknowledgedRisks []string) (runtime.HostAction, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.executed = append(f.executed, executedHostAction{actionID, nonce, hostInstanceID, selectedPath, append([]string(nil), acknowledgedRisks...)})
	return runtime.HostAction{ID: actionID, Status: "succeeded"}, nil
}

func (f *fakeStorageControl) CancelHostAction(_ context.Context, actionID string) (runtime.HostAction, error) {
	return runtime.HostAction{ID: actionID, Status: "cancelled"}, nil
}

func (f *fakeStorageControl) executions() []executedHostAction {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]executedHostAction(nil), f.executed...)
}

func newHostActionHarness(t *testing.T, storageVersion uint64, actions ...runtime.HostAction) (*Controller, *fakeStorageControl, *state.Store) {
	t.Helper()
	paths, err := platform.NewPaths(filepath.Join(t.TempDir(), "app-data"))
	if err != nil {
		t.Fatalf("paths: %v", err)
	}
	if err := paths.Ensure(); err != nil {
		t.Fatalf("ensure paths: %v", err)
	}
	store := state.NewWithInstanceID("host-action-harness")
	store.Commit(func(snapshot *dto.DesktopSnapshot) { snapshot.Storage.Version = storageVersion })
	control := &fakeStorageControl{actions: actions}
	controller := NewController(Options{Paths: paths, Store: store, Operations: operation.New(), Control: control})
	return controller, control, store
}

func waitForOperation(t *testing.T, controller *Controller, operationID string) dto.OperationSnapshot {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		for _, item := range controller.operations.Snapshot() {
			if item.OperationID == operationID && item.State != "accepted" && item.State != "running" {
				return item
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("operation %s did not settle", operationID)
	return dto.OperationSnapshot{}
}

// Presenting a request binds it to the storage snapshot the person sees. A
// request bound to an older snapshot is rebound when it is listed again, so a
// changed snapshot never leaves it permanently un-approvable.
func TestListHostActionsRebindsAStaleExpectedVersion(t *testing.T) {
	controller, control, _ := newHostActionHarness(t, 5,
		runtime.HostAction{ID: "stale", Nonce: "nonce-stale", Kind: "open_repository", Status: "pending", ExpectedVersion: 3},
		runtime.HostAction{ID: "current", Nonce: "nonce-current", Kind: "open_repository", Status: "pending", ExpectedVersion: 5},
	)
	tickets, err := controller.ListHostActions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(tickets) != 2 || tickets[0].ExpectedVersion != 5 || tickets[1].ExpectedVersion != 5 {
		t.Fatalf("tickets = %#v", tickets)
	}
	if !reflect.DeepEqual(control.rebinds, []uint64{5}) {
		t.Fatalf("rebinds = %v, want exactly the stale request rebound once", control.rebinds)
	}
	if _, err := controller.ListHostActions(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(control.rebinds) != 1 {
		t.Fatalf("a current binding was rebound: %v", control.rebinds)
	}
}

// Dismissing the native picker is a normal outcome, reported as such: the
// request stays pending, nothing runs, and no operation is recorded.
func TestApproveHostActionReportsADismissedPickerAsCancelled(t *testing.T) {
	controller, control, _ := newHostActionHarness(t, 5,
		runtime.HostAction{ID: "open", Nonce: "nonce-open", Kind: "open_repository", Status: "pending", ExpectedVersion: 5},
	)
	controller.SetPickDirectory(func(string) (string, error) { return "", nil })
	_, err := controller.ApproveHostAction("request-cancel", 5, "open", "nonce-open")
	if operation.ErrorCodeOf(err) != dto.ErrorSelectionCancelled {
		t.Fatalf("dismissed picker error = %v, want selection_cancelled", err)
	}
	if executions := control.executions(); len(executions) != 0 {
		t.Fatalf("a dismissed picker ran the task: %#v", executions)
	}
	if snapshot := controller.operations.Snapshot(); len(snapshot) != 0 {
		t.Fatalf("a dismissed picker recorded an operation: %#v", snapshot)
	}
	tickets, err := controller.ListHostActions(context.Background())
	if err != nil || len(tickets) != 1 || tickets[0].Status != "pending" {
		t.Fatalf("request after a dismissed picker = %#v, err=%v", tickets, err)
	}
}

// The first approval hands the Server the canonical folder the person chose
// and no acknowledgement; the Server reviews the folder itself.
func TestApproveHostActionSendsTheCanonicalSelectionWithoutAcknowledgements(t *testing.T) {
	controller, control, store := newHostActionHarness(t, 5,
		runtime.HostAction{ID: "open", Nonce: "nonce-open", Kind: "open_repository", Status: "pending", ExpectedVersion: 5},
	)
	chosen := filepath.Join(t.TempDir(), "Photos")
	if err := os.Mkdir(chosen, 0o755); err != nil {
		t.Fatal(err)
	}
	controller.SetPickDirectory(func(string) (string, error) { return chosen, nil })
	receipt, err := controller.ApproveHostAction("request-open", 5, "open", "nonce-open")
	if err != nil {
		t.Fatal(err)
	}
	if settled := waitForOperation(t, controller, receipt.OperationID); settled.State != "succeeded" {
		t.Fatalf("operation = %#v", settled)
	}
	canonical, err := canonicalShortcutPath(chosen)
	if err != nil {
		t.Fatal(err)
	}
	executions := control.executions()
	if len(executions) != 1 || executions[0].selectedPath != canonical || executions[0].acknowledged != nil ||
		executions[0].nonce != "nonce-open" || executions[0].hostInstanceID != store.Get().InstanceID {
		t.Fatalf("executions = %#v", executions)
	}
}

// Approving a presented risk review acknowledges exactly the risk codes the
// ticket showed, without opening the picker again: the Server already holds
// the folder and re-samples it under that acknowledgement.
func TestApproveHostActionAcknowledgesThePresentedRiskCodes(t *testing.T) {
	controller, control, _ := newHostActionHarness(t, 5,
		runtime.HostAction{
			ID: "risky", Nonce: "nonce-risky", Kind: "authorize_storage_location", Status: "needs_decision",
			ExpectedVersion: 5, RiskWarnings: []string{"removable_storage", "non_default_location"},
		},
	)
	controller.SetPickDirectory(func(string) (string, error) {
		t.Fatal("a risk decision must not reopen the picker")
		return "", nil
	})
	receipt, err := controller.ApproveHostAction("request-risk", 5, "risky", "nonce-risky")
	if err != nil {
		t.Fatal(err)
	}
	if settled := waitForOperation(t, controller, receipt.OperationID); settled.State != "succeeded" {
		t.Fatalf("operation = %#v", settled)
	}
	executions := control.executions()
	if len(executions) != 1 || executions[0].selectedPath != "" ||
		!reflect.DeepEqual(executions[0].acknowledged, []string{"removable_storage", "non_default_location"}) {
		t.Fatalf("executions = %#v", executions)
	}
}

// A review that awaits a decision only the Web can make (an identity
// conflict) is not approvable from Desktop.
func TestApproveHostActionRefusesAnIdentityDecisionWithoutRisks(t *testing.T) {
	controller, control, _ := newHostActionHarness(t, 5,
		runtime.HostAction{ID: "conflict", Nonce: "nonce-conflict", Kind: "open_repository", Status: "needs_decision", ExpectedVersion: 5},
	)
	_, err := controller.ApproveHostAction("request-conflict", 5, "conflict", "nonce-conflict")
	if operation.ErrorCodeOf(err) != dto.ErrorInvalidArgument {
		t.Fatalf("identity decision approve error = %v", err)
	}
	if executions := control.executions(); len(executions) != 0 {
		t.Fatalf("an identity decision ran from Desktop: %#v", executions)
	}
}
