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
	RuntimeState                 string
	RuntimeReason                string
	NextRegistrationRole         string
	RepositoryDefaults           *RepositoryDefaults
}

const SetupRuntimeStateInitializing = "initializing"

// RepositoryDefaults is the setup-wizard view of the storage-owned repository
// defaults plus the immutable default root (the storage root).
type RepositoryDefaults struct {
	DefaultRoot       string
	Strategy          string
	DuplicateHandling string
	RiskWarnings      []string
}

type repositoryDefaultsReader interface {
	GetRepositoryDefaults(ctx context.Context) (storage.RepoDefaults, error)
}

type storageRuntimeReader interface {
	StorageRuntimeStatus(ctx context.Context) (storage.StorageRuntimeStatus, error)
}

type setupRepositoryReader interface {
	repositoryDefaultsReader
	storageRuntimeReader
}

// SetupService exposes first-run state after the catalog has opened and
// migrated. There is no separate database initialization endpoint.
type SetupService struct {
	bootstrap    BootstrapService
	repoDefaults repositoryDefaultsReader
	storage      storageRuntimeReader
	storageRoot  string
}

func NewSetupService(
	bootstrap BootstrapService,
	repositories setupRepositoryReader,
	storageRoot string,
) *SetupService {
	return &SetupService{
		bootstrap:    bootstrap,
		repoDefaults: repositories,
		storage:      repositories,
		storageRoot:  strings.TrimSpace(storageRoot),
	}
}

func (s *SetupService) Status(ctx context.Context) (SetupStatus, error) {
	status := SetupStatus{RuntimeState: SetupRuntimeStateInitializing}

	if s.bootstrap != nil {
		phase, err := s.bootstrap.Phase(ctx)
		if err != nil {
			return SetupStatus{}, fmt.Errorf("load bootstrap phase: %w", err)
		}
		status.AdminInitialized = phase == BootstrapPhaseAdminCreated || phase == BootstrapPhaseReady
		status.PrimaryRepositoryInitialized = phase == BootstrapPhaseReady
		status.Initialized = phase == BootstrapPhaseReady
	}
	if status.Initialized && s.storage != nil {
		runtimeStatus, err := s.storage.StorageRuntimeStatus(ctx)
		if err != nil {
			return SetupStatus{}, fmt.Errorf("load storage runtime status: %w", err)
		}
		status.RuntimeState = runtimeStatus.State
		status.RuntimeReason = runtimeStatus.Reason
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
			RiskWarnings:      storage.StoragePlacementWarnings(s.storageRoot),
		}
	}

	return status, nil
}
