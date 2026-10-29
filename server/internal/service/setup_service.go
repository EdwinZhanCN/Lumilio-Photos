package service

import (
	"context"
	"fmt"
	"strings"

	"server/internal/storage"
)

// SetupStatus reports the durable first-run gates. It is public (the
// instance has no users yet when setup starts), so it carries phase facts
// only: no host path, no marker reading, no review token.
type SetupStatus struct {
	Initialized                  bool
	AdminInitialized             bool
	PrimaryRepositoryInitialized bool
	RuntimeState                 string
	RuntimeReason                string
	NextRegistrationRole         string
}

const SetupRuntimeStateInitializing = "initializing"

// RepositoryDefaults is the setup-wizard view of the storage-owned repository
// defaults plus the immutable Default Storage Location path.
type RepositoryDefaults struct {
	StorageLocation   string
	Strategy          string
	DuplicateHandling string
}

// SetupPrimaryTarget is the authenticated administrator's view of the
// Primary Repository setup: the defaults, what the configured Default
// Storage Location looks like, and the Server's review of <default>/primary
// with its legal actions. Target is nil while the Default Storage Location
// is not usable; DefaultStorage then names why.
type SetupPrimaryTarget struct {
	SetupComplete      bool
	RepositoryDefaults RepositoryDefaults
	DefaultStorage     storage.DefaultStorageAssessment
	Target             *storage.SetupPrimaryAssessment
}

type repositoryDefaultsReader interface {
	GetRepositoryDefaults(ctx context.Context) (storage.RepoDefaults, error)
}

type setupTargetAssessor interface {
	AssessSetupPrimaryTarget(ctx context.Context) (*storage.SetupPrimaryAssessment, error)
	AssessDefaultStorageLocation(ctx context.Context, configuredPath string) (storage.DefaultStorageAssessment, error)
}

type storageRuntimeReader interface {
	StorageRuntimeStatus(ctx context.Context) (storage.StorageRuntimeStatus, error)
}

type setupRepositoryReader interface {
	repositoryDefaultsReader
	storageRuntimeReader
	setupTargetAssessor
}

// SetupService exposes first-run state after the catalog has opened and
// migrated. There is no separate database initialization endpoint.
type SetupService struct {
	bootstrap       BootstrapService
	repoDefaults    repositoryDefaultsReader
	storage         storageRuntimeReader
	assessor        setupTargetAssessor
	storageLocation string
}

func NewSetupService(
	bootstrap BootstrapService,
	repositories setupRepositoryReader,
	storageLocation string,
) *SetupService {
	return &SetupService{
		bootstrap:       bootstrap,
		repoDefaults:    repositories,
		storage:         repositories,
		assessor:        repositories,
		storageLocation: strings.TrimSpace(storageLocation),
	}
}

// Status is the public, phase-only setup status.
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
	return status, nil
}

// PrimaryTarget is the authenticated administrator's setup review. It reads
// the Default Storage Location and the primary target without changing
// either; once the primary exists it reports only that setup is complete.
func (s *SetupService) PrimaryTarget(ctx context.Context) (SetupPrimaryTarget, error) {
	result := SetupPrimaryTarget{RepositoryDefaults: RepositoryDefaults{StorageLocation: s.storageLocation}}
	if s.bootstrap != nil {
		ready, err := s.bootstrap.IsReady(ctx)
		if err != nil {
			return SetupPrimaryTarget{}, fmt.Errorf("load bootstrap phase: %w", err)
		}
		result.SetupComplete = ready
	}
	if s.repoDefaults != nil {
		defaults, err := s.repoDefaults.GetRepositoryDefaults(ctx)
		if err != nil {
			return SetupPrimaryTarget{}, fmt.Errorf("load repository defaults: %w", err)
		}
		result.RepositoryDefaults.Strategy = defaults.Strategy
		result.RepositoryDefaults.DuplicateHandling = defaults.DuplicateHandling
	}
	if s.assessor == nil {
		return result, nil
	}
	defaultStorage, err := s.assessor.AssessDefaultStorageLocation(ctx, s.storageLocation)
	if err != nil {
		return SetupPrimaryTarget{}, fmt.Errorf("assess default Storage Location: %w", err)
	}
	result.DefaultStorage = defaultStorage
	if result.SetupComplete || !defaultStorage.Usable() {
		return result, nil
	}
	// A failed review is not a status failure: the Default Storage Location
	// facts above already say what is wrong, and the commands report the
	// exact typed cause.
	if target, err := s.assessor.AssessSetupPrimaryTarget(ctx); err == nil {
		result.Target = target
	}
	return result, nil
}
