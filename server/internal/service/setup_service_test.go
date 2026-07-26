package service

import (
	"context"
	"testing"
)

type setupBootstrapStub struct {
	phase string
}

func (s *setupBootstrapStub) Phase(context.Context) (string, error) {
	return s.phase, nil
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
