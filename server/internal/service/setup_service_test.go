package service

import (
	"context"
	"testing"

	"server/internal/storage"
)

type setupBootstrapStub struct {
	phase string
}

type setupStorageStub struct {
	runtime storage.StorageRuntimeStatus
}

func (s *setupStorageStub) GetRepositoryDefaults(context.Context) (storage.RepoDefaults, error) {
	return storage.RepoDefaults{Strategy: "date", DuplicateHandling: "rename"}, nil
}

func (s *setupStorageStub) StorageRuntimeStatus(context.Context) (storage.StorageRuntimeStatus, error) {
	return s.runtime, nil
}

func (s *setupBootstrapStub) Phase(context.Context) (string, error) {
	return s.phase, nil
}

func TestSetupServiceKeepsReadyWhileReportingStorageDegraded(t *testing.T) {
	bootstrap := &setupBootstrapStub{phase: BootstrapPhaseReady}
	repositories := &setupStorageStub{runtime: storage.StorageRuntimeStatus{
		State: storage.StorageRuntimeStateDegraded, Reason: storage.StorageRuntimeReasonRecoveryRequired,
	}}
	service := NewSetupService(bootstrap, repositories, "/storage")

	status, err := service.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !status.Initialized || !status.PrimaryRepositoryInitialized {
		t.Fatalf("durable setup gates regressed: %+v", status)
	}
	if status.RuntimeState != storage.StorageRuntimeStateDegraded || status.RuntimeReason != storage.StorageRuntimeReasonRecoveryRequired {
		t.Fatalf("runtime status = %+v", status)
	}
}

func (s *setupBootstrapStub) Reconcile(context.Context) (string, error) {
	return s.phase, nil
}

func (s *setupBootstrapStub) IsReady(context.Context) (bool, error) {
	return s.phase == BootstrapPhaseReady, nil
}

func TestSetupServiceStatusUsesSQLiteAndBootstrapGates(t *testing.T) {
	bootstrap := &setupBootstrapStub{phase: BootstrapPhaseCatalogReady}
	service := &SetupService{
		bootstrap: bootstrap,
	}

	status, err := service.Status(context.Background())
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if status.AdminInitialized || status.PrimaryRepositoryInitialized || status.Initialized {
		t.Fatalf("unexpected initialized gates: %+v", status)
	}
	if status.NextRegistrationRole != string(UserRoleAdmin) {
		t.Fatalf("next role = %q, want admin", status.NextRegistrationRole)
	}
}

func TestSetupServiceReportsDefaultStoragePlacementRisks(t *testing.T) {
	bootstrap := &setupBootstrapStub{phase: BootstrapPhaseAdminCreated}
	repositories := &setupStorageStub{}
	service := NewSetupService(
		bootstrap,
		repositories,
		"/Users/example/Library/Mobile Documents/Lumilio",
	)

	status, err := service.Status(context.Background())
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if status.RepositoryDefaults == nil || len(status.RepositoryDefaults.RiskWarnings) == 0 {
		t.Fatalf("expected setup storage risks, got %+v", status.RepositoryDefaults)
	}
}
