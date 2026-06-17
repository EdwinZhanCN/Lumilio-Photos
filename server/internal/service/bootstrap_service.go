package service

import (
	"context"
	"fmt"
	"os"
	"strings"

	"server/internal/db/repo"
)

// Bootstrap phases. The system progresses fresh → db_rotated → admin_created →
// ready as first-run setup completes its gates.
const (
	BootstrapPhaseFresh        = "fresh"
	BootstrapPhaseDBRotated    = "db_rotated"
	BootstrapPhaseAdminCreated = "admin_created"
	BootstrapPhaseReady        = "ready"
)

// BootstrapService is the single source of truth for the first-run bootstrap
// phase. The phase is computed from the setup gates (rotated DB credential,
// admin user, exactly one active primary repository) in one place and cached in
// system_state, so request paths read one column instead of re-probing.
type BootstrapService interface {
	// Phase returns the cached bootstrap phase.
	Phase(ctx context.Context) (string, error)
	// Reconcile recomputes the phase from the gates, persists it, and returns it.
	// Called at startup and after each setup transition.
	Reconcile(ctx context.Context) (string, error)
	// IsReady reports whether the system has completed first-run setup.
	IsReady(ctx context.Context) (bool, error)
}

type bootstrapService struct {
	queries        *repo.Queries
	dbPasswordFile string
}

// NewBootstrapService wires the bootstrap service. dbPasswordFile is the rotated
// database password secret whose presence marks the db_rotated gate.
func NewBootstrapService(queries *repo.Queries, dbPasswordFile string) BootstrapService {
	return &bootstrapService{
		queries:        queries,
		dbPasswordFile: strings.TrimSpace(dbPasswordFile),
	}
}

func (s *bootstrapService) Phase(ctx context.Context) (string, error) {
	state, err := s.queries.GetSystemState(ctx)
	if err != nil {
		return "", fmt.Errorf("get system state: %w", err)
	}
	// Once ready, trust the cached value (fast path). While still setting up,
	// recompute from the gates so the phase advances as setup progresses without
	// every transition having to call Reconcile.
	if state.BootstrapPhase == BootstrapPhaseReady {
		return BootstrapPhaseReady, nil
	}
	return s.Reconcile(ctx)
}

func (s *bootstrapService) IsReady(ctx context.Context) (bool, error) {
	phase, err := s.Phase(ctx)
	if err != nil {
		return false, err
	}
	return phase == BootstrapPhaseReady, nil
}

func (s *bootstrapService) Reconcile(ctx context.Context) (string, error) {
	phase, err := s.compute(ctx)
	if err != nil {
		return "", err
	}
	if _, err := s.queries.SetBootstrapPhase(ctx, phase); err != nil {
		return "", fmt.Errorf("persist bootstrap phase: %w", err)
	}
	return phase, nil
}

// compute derives the phase from the setup gates. It is the only place these
// gates are evaluated.
func (s *bootstrapService) compute(ctx context.Context) (string, error) {
	if !s.dbCredentialRotated() {
		return BootstrapPhaseFresh, nil
	}

	admins, err := s.queries.CountActiveUsersByRole(ctx, string(UserRoleAdmin))
	if err != nil {
		return "", fmt.Errorf("count admin users: %w", err)
	}
	if admins == 0 {
		return BootstrapPhaseDBRotated, nil
	}

	primaries, err := s.queries.CountActivePrimaryRepositories(ctx)
	if err != nil {
		return "", fmt.Errorf("count primary repositories: %w", err)
	}
	if primaries != 1 {
		return BootstrapPhaseAdminCreated, nil
	}

	return BootstrapPhaseReady, nil
}

func (s *bootstrapService) dbCredentialRotated() bool {
	if s.dbPasswordFile == "" {
		return false
	}
	data, err := os.ReadFile(s.dbPasswordFile)
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(data)) != ""
}
