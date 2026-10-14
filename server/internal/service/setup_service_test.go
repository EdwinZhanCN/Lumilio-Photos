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
	runtime    storage.StorageRuntimeStatus
	assessment *storage.CreateTargetAssessment
	assessErr  error
	assessed   []storage.AssessCreateTargetRequest
}

func (s *setupStorageStub) AssessCreateTarget(_ context.Context, request storage.AssessCreateTargetRequest) (*storage.CreateTargetAssessment, error) {
	s.assessed = append(s.assessed, request)
	if s.assessErr != nil {
		return nil, s.assessErr
	}
	return s.assessment, nil
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

func TestSetupServiceReportsPrimaryTargetAssessment(t *testing.T) {
	bootstrap := &setupBootstrapStub{phase: BootstrapPhaseAdminCreated}
	repositories := &setupStorageStub{assessment: &storage.CreateTargetAssessment{
		Intent: storage.IntentSetupPrimary, DirectoryName: "primary",
		Risks: []storage.StorageRisk{storage.RiskCloudSync}, Token: "v1.setup",
	}}
	service := NewSetupService(bootstrap, repositories, "/Users/example/Library/Mobile Documents/Lumilio")

	status, err := service.Status(context.Background())
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if status.RepositoryDefaults == nil || status.RepositoryDefaults.Assessment == nil ||
		status.RepositoryDefaults.Assessment.Token != "v1.setup" || len(status.RepositoryDefaults.Assessment.Risks) != 1 {
		t.Fatalf("expected the setup_primary assessment, got %+v", status.RepositoryDefaults)
	}
	if len(repositories.assessed) != 1 || repositories.assessed[0].Intent != storage.IntentSetupPrimary {
		t.Fatalf("assessment requests = %+v, want one setup_primary review", repositories.assessed)
	}
}

func TestSetupServiceStatusSurvivesAnUnassessableTarget(t *testing.T) {
	bootstrap := &setupBootstrapStub{phase: BootstrapPhaseAdminCreated}
	repositories := &setupStorageStub{assessErr: storage.ErrStorageLocationInvalid}
	service := NewSetupService(bootstrap, repositories, "/storage")

	status, err := service.Status(context.Background())
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if status.RepositoryDefaults == nil || status.RepositoryDefaults.Assessment != nil {
		t.Fatalf("defaults = %+v, want defaults without an assessment", status.RepositoryDefaults)
	}
}

func TestSetupServiceSkipsTheAssessmentOnceThePrimaryExists(t *testing.T) {
	bootstrap := &setupBootstrapStub{phase: BootstrapPhaseReady}
	repositories := &setupStorageStub{assessment: &storage.CreateTargetAssessment{Token: "v1.unused"}}
	service := NewSetupService(bootstrap, repositories, "/storage")

	if _, err := service.Status(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(repositories.assessed) != 0 {
		t.Fatalf("ready instance still assessed the primary target: %+v", repositories.assessed)
	}
}
