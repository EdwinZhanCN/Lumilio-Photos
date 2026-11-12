package update

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"desktop/internal/control/dto"
	"desktop/internal/operation"
	"desktop/internal/state"
)

type fakeProvider struct {
	enabled  bool
	release  *Release
	checkErr error
	channels []string
}

func (p *fakeProvider) Enabled() bool { return p.enabled }

func (p *fakeProvider) Check(_ context.Context, channel string) (*Release, error) {
	p.channels = append(p.channels, channel)
	return p.release, p.checkErr
}

func newTestController(t *testing.T, provider Provider) (*Controller, *operation.Registry) {
	t.Helper()
	operations := operation.New()
	store := state.NewWithInstanceID("update-test")
	t.Cleanup(func() {
		operations.Close()
		store.Close()
	})
	return NewController(Options{Store: store, Operations: operations, Provider: provider, CurrentVersion: "26.1.0"}), operations
}

func TestDownloadInstallsOnlyWhatTheProviderVerified(t *testing.T) {
	installs := 0
	provider := &fakeProvider{enabled: true, release: &Release{
		Version: "26.2.0",
		Install: func(_ context.Context, progress func(int64, int64)) error {
			installs++
			progress(50, 100)
			return nil
		},
	}}
	controller, operations := newTestController(t, provider)
	if snapshot := controller.Snapshot(); !snapshot.ProviderAvailable || snapshot.CurrentVersion != "26.1.0" {
		t.Fatalf("initial snapshot = %#v, want an available provider and the current version", snapshot)
	}

	check, err := controller.Check("check-1", 0)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	waitUpdateOperation(t, operations, check.OperationID, operation.Succeeded)
	if snapshot := controller.Snapshot(); snapshot.Phase != "available" || snapshot.AvailableVersion != "26.2.0" || snapshot.CanApply {
		t.Fatalf("check snapshot = %#v, want available and not applicable", snapshot)
	}
	if installs != 0 {
		t.Fatal("a check must not install")
	}

	download, err := controller.Download("download-1", controller.Snapshot().Version)
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	waitUpdateOperation(t, operations, download.OperationID, operation.Succeeded)
	if snapshot := controller.Snapshot(); snapshot.Phase != "ready" || snapshot.CanApply {
		t.Fatalf("download snapshot = %#v, want ready and gated until the shutdown handoff exists", snapshot)
	}
	if installs != 1 {
		t.Fatalf("installs = %d, want 1", installs)
	}

	controller.SetApply(func(string, uint64) (dto.OperationReceipt, error) {
		return dto.OperationReceipt{OperationID: "shutdown-1"}, nil
	})
	if !controller.Snapshot().CanApply {
		t.Fatal("an installed update did not become applicable once the apply handoff was configured")
	}
	// Switching channels afterwards must not strand what is already on disk.
	if err := controller.SetChannel("beta"); err != nil {
		t.Fatal(err)
	}
	if snapshot := controller.Snapshot(); !snapshot.CanApply || snapshot.AvailableVersion != "26.2.0" {
		t.Fatalf("channel change dropped an installed update: %#v", snapshot)
	}
	receipt, err := controller.RestartAndApply("apply-1", controller.Snapshot().Version)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	waitUpdateOperation(t, operations, receipt.OperationID, operation.Succeeded)
}

func TestCheckReportsCurrentWhenNothingIsNewer(t *testing.T) {
	provider := &fakeProvider{enabled: true}
	controller, operations := newTestController(t, provider)
	if err := controller.SetChannel("beta"); err != nil {
		t.Fatal(err)
	}
	check, err := controller.Check("check-1", controller.Snapshot().Version)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	waitUpdateOperation(t, operations, check.OperationID, operation.Succeeded)
	snapshot := controller.Snapshot()
	if snapshot.Phase != "current" || snapshot.AvailableVersion != "" || snapshot.Error.Code != "" {
		t.Fatalf("snapshot = %#v, want current without a version or an error", snapshot)
	}
	if len(provider.channels) != 1 || provider.channels[0] != "beta" {
		t.Fatalf("the provider was asked for %v, want the selected channel", provider.channels)
	}
}

func TestBuildWithoutUpdatesStaysOffline(t *testing.T) {
	for name, provider := range map[string]Provider{"no provider": nil, "disabled provider": &fakeProvider{}} {
		t.Run(name, func(t *testing.T) {
			controller, operations := newTestController(t, provider)
			if controller.Snapshot().ProviderAvailable {
				t.Fatal("a build without updates reported an available provider")
			}
			check, err := controller.Check("check-1", 0)
			if err != nil {
				t.Fatalf("check: %v", err)
			}
			waitUpdateOperation(t, operations, check.OperationID, operation.Succeeded)
			if snapshot := controller.Snapshot(); snapshot.Phase != "offline" || snapshot.Error.Code != dto.ErrorRuntimeNotReady {
				t.Fatalf("snapshot = %#v, want offline with runtime_not_ready", snapshot)
			}
		})
	}
}

func TestRefusedSignatureNeverBecomesApplicable(t *testing.T) {
	provider := &fakeProvider{enabled: true, release: &Release{
		Version: "26.2.0",
		Install: func(context.Context, func(int64, int64)) error {
			return fmt.Errorf("archive: %w", ErrSignature)
		},
	}}
	controller, operations := newTestController(t, provider)
	controller.SetApply(func(string, uint64) (dto.OperationReceipt, error) {
		t.Fatal("apply ran for an update whose signature was refused")
		return dto.OperationReceipt{}, nil
	})
	download, err := controller.Download("download-1", 0)
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	waitUpdateOperation(t, operations, download.OperationID, operation.Failed)
	snapshot := controller.Snapshot()
	if snapshot.Phase != "failed" || snapshot.Error.Code != dto.ErrorSignatureInvalid || snapshot.CanApply {
		t.Fatalf("snapshot = %#v, want failed with signature_invalid and no apply", snapshot)
	}
	if _, err := controller.RestartAndApply("apply-1", snapshot.Version); operation.ErrorCodeOf(err) != dto.ErrorInvalidArgument {
		t.Fatalf("apply error = %v, want invalid_argument", err)
	}
}

func TestFailedDownloadIsRetryable(t *testing.T) {
	attempts := 0
	provider := &fakeProvider{enabled: true, release: &Release{
		Version: "26.2.0",
		Install: func(context.Context, func(int64, int64)) error {
			attempts++
			if attempts == 1 {
				return errors.New("connection reset")
			}
			return nil
		},
	}}
	controller, operations := newTestController(t, provider)
	first, err := controller.Download("download-1", 0)
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	waitUpdateOperation(t, operations, first.OperationID, operation.Succeeded)
	if snapshot := controller.Snapshot(); snapshot.Phase != "offline" || snapshot.AvailableVersion != "26.2.0" {
		t.Fatalf("snapshot = %#v, want offline and the version still offered", snapshot)
	}
	second, err := controller.Download("download-2", controller.Snapshot().Version)
	if err != nil {
		t.Fatalf("retry: %v", err)
	}
	waitUpdateOperation(t, operations, second.OperationID, operation.Succeeded)
	if snapshot := controller.Snapshot(); snapshot.Phase != "ready" {
		t.Fatalf("snapshot = %#v, want ready after the retry", snapshot)
	}
}

func waitUpdateOperation(t *testing.T, registry *operation.Registry, operationID string, want operation.State) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		item, ok := registry.Get(operationID)
		if ok && (item.State == string(operation.Succeeded) || item.State == string(operation.Failed)) {
			if item.State != string(want) {
				t.Fatalf("operation %s ended %s, want %s: %#v", operationID, item.State, want, item)
			}
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("operation %s did not finish", operationID)
}
