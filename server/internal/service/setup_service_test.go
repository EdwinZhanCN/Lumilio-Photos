package service

import (
	"context"
	"testing"

	"server/config"
)

type setupBootstrapStub struct {
	phase      string
	reconciles int
}

func (s *setupBootstrapStub) Phase(context.Context) (string, error) {
	return s.phase, nil
}

func (s *setupBootstrapStub) Reconcile(context.Context) (string, error) {
	s.reconciles++
	return s.phase, nil
}

func (s *setupBootstrapStub) IsReady(context.Context) (bool, error) {
	return s.phase == BootstrapPhaseReady, nil
}

func TestSetupServiceStatusUsesSQLiteAndBootstrapGates(t *testing.T) {
	bootstrap := &setupBootstrapStub{phase: BootstrapPhaseDBRotated}
	service := &SetupService{
		dbConfig:  config.DatabaseConfig{Path: "/tmp/lumilio.sqlite3"},
		bootstrap: bootstrap,
	}

	status, err := service.Status(context.Background())
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if !status.DatabaseInitialized {
		t.Fatal("open SQLite path must satisfy the database initialization gate")
	}
	if status.AdminInitialized || status.PrimaryRepositoryInitialized || status.Initialized {
		t.Fatalf("unexpected initialized gates: %+v", status)
	}
	if status.NextRegistrationRole != string(UserRoleAdmin) {
		t.Fatalf("next role = %q, want admin", status.NextRegistrationRole)
	}
}

func TestSetupServiceInitializeIsIdempotentReconciliation(t *testing.T) {
	bootstrap := &setupBootstrapStub{phase: BootstrapPhaseReady}
	service := &SetupService{
		dbConfig:  config.DatabaseConfig{Path: "/tmp/lumilio.sqlite3"},
		bootstrap: bootstrap,
	}

	for i := 0; i < 2; i++ {
		result, err := service.Initialize(context.Background(), SetupRequest{})
		if err != nil {
			t.Fatalf("initialize %d: %v", i, err)
		}
		if result.DatabaseUser != "" || result.SecretPath != "" || result.PasswordLength != 0 {
			t.Fatalf("SQLite setup must not return credentials: %+v", result)
		}
	}
	if bootstrap.reconciles != 2 {
		t.Fatalf("reconciles = %d, want 2", bootstrap.reconciles)
	}
}
