package service

import (
	"context"
	"fmt"
	"strings"

	"server/internal/storage"
)

// SetupStatus reports the durable first-run gates.
type SetupStatus struct {
	Initialized                  bool
	AdminInitialized             bool
	PrimaryRepositoryInitialized bool
	NextRegistrationRole         string
	RepositoryDefaults           *RepositoryDefaults
}

// RepositoryDefaults is the setup-wizard view of the storage-owned repository
// defaults plus the immutable default root (the storage root).
type RepositoryDefaults struct {
	DefaultRoot       string
	Strategy          string
	DuplicateHandling string
}

type repositoryDefaultsReader interface {
	GetRepositoryDefaults(ctx context.Context) (storage.RepoDefaults, error)
}

// SetupService exposes first-run state after the catalog has opened and
// migrated. There is no separate database initialization endpoint.
type SetupService struct {
	bootstrap    BootstrapService
	repoDefaults repositoryDefaultsReader
	storageRoot  string
}

func NewSetupService(
	bootstrap BootstrapService,
	repoDefaults repositoryDefaultsReader,
	storageRoot string,
) *SetupService {
	return &SetupService{
		bootstrap:    bootstrap,
		repoDefaults: repoDefaults,
		storageRoot:  strings.TrimSpace(storageRoot),
	}
}

func (s *SetupService) Status(ctx context.Context) (SetupStatus, error) {
	status := SetupStatus{}

	if s.bootstrap != nil {
		phase, err := s.bootstrap.Phase(ctx)
		if err != nil {
			return SetupStatus{}, fmt.Errorf("load bootstrap phase: %w", err)
		}
		status.AdminInitialized = phase == BootstrapPhaseAdminCreated || phase == BootstrapPhaseReady
		status.PrimaryRepositoryInitialized = phase == BootstrapPhaseReady
		status.Initialized = phase == BootstrapPhaseReady
	}

	if status.AdminInitialized {
		status.NextRegistrationRole = string(UserRoleUser)
	} else {
		status.NextRegistrationRole = string(UserRoleAdmin)
	}

	if s.repoDefaults != nil {
		defaults, err := s.repoDefaults.GetRepositoryDefaults(ctx)
		if err != nil {
			return SetupStatus{}, fmt.Errorf("load repository defaults: %w", err)
		}
		status.RepositoryDefaults = &RepositoryDefaults{
			DefaultRoot:       s.storageRoot,
			Strategy:          defaults.Strategy,
			DuplicateHandling: defaults.DuplicateHandling,
		}
	}

	return status, nil
}
