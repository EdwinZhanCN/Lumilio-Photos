package service

import (
	"context"
	"fmt"

	"server/internal/db/repo"
)

// Bootstrap phases. SQLite is already open and migrated when reconciliation
// runs, so catalog_ready denotes "owner registration pending".
const (
	BootstrapPhaseFresh        = "fresh"
	BootstrapPhaseCatalogReady = "catalog_ready"
	BootstrapPhaseAdminCreated = "admin_created"
	BootstrapPhaseReady        = "ready"
)

// BootstrapService is the single source of truth for the first-run bootstrap
// phase. The phase is computed from the setup gates (admin user and exactly one
// active primary repository) in one place and cached in system_state. Request
// paths trust the terminal cached value and otherwise derive the current phase
// through the query-only reader; they never reconcile or wait for the writer.
type BootstrapService interface {
	// Phase returns the terminal cached phase or derives the current phase using
	// read-only setup gates.
	Phase(ctx context.Context) (string, error)
	// Reconcile recomputes the phase from the gates, persists it, and returns it.
	// Called at startup or another explicit maintenance boundary, never from a
	// foreground status read.
	Reconcile(ctx context.Context) (string, error)
	// IsReady reports whether the system has completed first-run setup.
	IsReady(ctx context.Context) (bool, error)
}

type bootstrapService struct {
	queries     *repo.Queries
	readQueries *repo.Queries
}

func NewBootstrapService(queries *repo.Queries) BootstrapService {
	return &bootstrapService{queries: queries, readQueries: queries}
}

func NewBootstrapServiceWithReader(queries *repo.Queries, readQueries *repo.Queries) BootstrapService {
	return &bootstrapService{queries: queries, readQueries: readQueries}
}

func (s *bootstrapService) Phase(ctx context.Context) (string, error) {
	state, err := s.readQueries.GetSystemState(ctx)
	if err != nil {
		return "", fmt.Errorf("get system state: %w", err)
	}
	// Once ready, trust the durable cached value (fast path). While still setting
	// up, derive the live phase strictly through the reader. Setup status and the
	// initialization middleware are foreground request paths and must not queue
	// behind SQLite's sole writer merely to persist this cache.
	if state.BootstrapPhase == BootstrapPhaseReady {
		return BootstrapPhaseReady, nil
	}
	return s.compute(ctx)
}

func (s *bootstrapService) IsReady(ctx context.Context) (bool, error) {
	phase, err := s.Phase(ctx)
	if err != nil {
		return false, err
	}
	return phase == BootstrapPhaseReady, nil
}

func (s *bootstrapService) Reconcile(ctx context.Context) (string, error) {
	state, err := s.readQueries.GetSystemState(ctx)
	if err != nil {
		return "", fmt.Errorf("get system state: %w", err)
	}
	// Setup completion is a durable fact. Runtime storage availability is
	// reported separately and must never send an established instance back
	// through owner registration or primary-repository setup.
	if state.BootstrapPhase == BootstrapPhaseReady {
		return BootstrapPhaseReady, nil
	}

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
	admins, err := s.readQueries.CountActiveUsersByRole(ctx, string(UserRoleAdmin))
	if err != nil {
		return "", fmt.Errorf("count admin users: %w", err)
	}
	if admins == 0 {
		return BootstrapPhaseCatalogReady, nil
	}

	primaries, err := s.readQueries.CountPrimaryRepositories(ctx)
	if err != nil {
		return "", fmt.Errorf("count primary repositories: %w", err)
	}
	if primaries != 1 {
		return BootstrapPhaseAdminCreated, nil
	}

	return BootstrapPhaseReady, nil
}
