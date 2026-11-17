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
	runtime        storage.StorageRuntimeStatus
	assessment     *storage.SetupPrimaryAssessment
	assessErr      error
	assessed       int
	defaultStorage storage.DefaultStorageAssessment
}

func (s *setupStorageStub) AssessSetupPrimaryTarget(context.Context) (*storage.SetupPrimaryAssessment, error) {
	s.assessed++
	if s.assessErr != nil {
		return nil, s.assessErr
	}
	return s.assessment, nil
}

func (s *setupStorageStub) AssessDefaultStorageLocation(_ context.Context, path string) (storage.DefaultStorageAssessment, error) {
	result := s.defaultStorage
	result.ConfiguredPath = path
	if result.State == "" {
		result.State = storage.DefaultStorageReady
		result.Registered = true
	}
	return result, nil
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

func TestSetupServicePublicStatusCarriesNoHostFacts(t *testing.T) {
	bootstrap := &setupBootstrapStub{phase: BootstrapPhaseAdminCreated}
	repositories := &setupStorageStub{assessment: &storage.SetupPrimaryAssessment{State: storage.SetupPrimaryAdoptable}}
	service := NewSetupService(bootstrap, repositories, "/Users/example/Library/Mobile Documents/Lumilio")

	status, err := service.Status(context.Background())
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if status.AdminInitialized != true || status.PrimaryRepositoryInitialized || status.NextRegistrationRole != string(UserRoleUser) {
		t.Fatalf("status = %+v", status)
	}
	if repositories.assessed != 0 {
		t.Fatal("the public status reviewed the primary target")
	}
}

func TestSetupServicePrimaryTargetReportsTheReviewForTheAdministrator(t *testing.T) {
	bootstrap := &setupBootstrapStub{phase: BootstrapPhaseAdminCreated}
	repositories := &setupStorageStub{assessment: &storage.SetupPrimaryAssessment{
		CreateTargetAssessment: &storage.CreateTargetAssessment{
			Intent: storage.IntentSetupPrimary, DirectoryName: "primary",
			Risks: []storage.StorageRisk{storage.RiskCloudSync}, Token: "v1.setup",
		},
		State: storage.SetupPrimaryAdoptable, Actions: []string{storage.SetupActionAdopt, storage.SetupActionFreshElsewhere},
	}}
	service := NewSetupService(bootstrap, repositories, "/Users/example/Library/Mobile Documents/Lumilio")

	target, err := service.PrimaryTarget(context.Background())
	if err != nil {
		t.Fatalf("primary target: %v", err)
	}
	if target.SetupComplete || target.RepositoryDefaults.StorageLocation != "/Users/example/Library/Mobile Documents/Lumilio" ||
		target.RepositoryDefaults.Strategy != "date" || !target.DefaultStorage.Usable() {
		t.Fatalf("target = %+v", target)
	}
	if target.Target == nil || target.Target.State != storage.SetupPrimaryAdoptable || target.Target.Token != "v1.setup" || len(target.Target.Risks) != 1 {
		t.Fatalf("expected the setup_primary review, got %+v", target.Target)
	}
	if repositories.assessed != 1 {
		t.Fatalf("assessment requests = %d, want one", repositories.assessed)
	}
}

func TestSetupServicePrimaryTargetSurvivesAnUnusableDefaultStorageLocation(t *testing.T) {
	bootstrap := &setupBootstrapStub{phase: BootstrapPhaseAdminCreated}
	repositories := &setupStorageStub{
		assessErr:      storage.ErrStorageLocationInvalid,
		defaultStorage: storage.DefaultStorageAssessment{State: storage.DefaultStorageMarkerInvalid, Detail: "parse storage location marker"},
	}
	service := NewSetupService(bootstrap, repositories, "/storage")

	target, err := service.PrimaryTarget(context.Background())
	if err != nil {
		t.Fatalf("primary target: %v", err)
	}
	if target.Target != nil || target.DefaultStorage.State != storage.DefaultStorageMarkerInvalid || target.DefaultStorage.ConfiguredPath != "/storage" {
		t.Fatalf("target = %+v, want the default-storage diagnosis without a review", target)
	}
	if repositories.assessed != 0 {
		t.Fatal("an unusable default Storage Location was still reviewed")
	}
}

func TestSetupServiceSkipsTheReviewOnceThePrimaryExists(t *testing.T) {
	bootstrap := &setupBootstrapStub{phase: BootstrapPhaseReady}
	repositories := &setupStorageStub{assessment: &storage.SetupPrimaryAssessment{State: storage.SetupPrimaryRegistered}}
	service := NewSetupService(bootstrap, repositories, "/storage")

	target, err := service.PrimaryTarget(context.Background())
	if err != nil {
		t.Fatalf("primary target: %v", err)
	}
	if !target.SetupComplete || target.Target != nil || repositories.assessed != 0 {
		t.Fatalf("completed setup still reviews the primary target: %+v (assessed %d)", target, repositories.assessed)
	}
}
