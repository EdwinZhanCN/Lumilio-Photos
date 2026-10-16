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
// defaults plus the immutable Default Storage Location path.
type RepositoryDefaults struct {
	StorageLocation   string
	Strategy          string
	DuplicateHandling string
	// Assessment is the Server's review of the Primary Repository target. It
	// is nil when the Default Storage Location is not registered yet or the
	// target cannot be resolved; the setup create then reports the exact cause.
	Assessment *storage.CreateTargetAssessment
}

type repositoryDefaultsReader interface {
	GetRepositoryDefaults(ctx context.Context) (storage.RepoDefaults, error)
}

type createTargetAssessor interface {
	AssessCreateTarget(ctx context.Context, request storage.AssessCreateTargetRequest) (*storage.CreateTargetAssessment, error)
}

type storageRuntimeReader interface {
	StorageRuntimeStatus(ctx context.Context) (storage.StorageRuntimeStatus, error)
}

type setupRepositoryReader interface {
	repositoryDefaultsReader
	storageRuntimeReader
	createTargetAssessor
}

// SetupService exposes first-run state after the catalog has opened and
// migrated. There is no separate database initialization endpoint.
type SetupService struct {
	bootstrap       BootstrapService
	repoDefaults    repositoryDefaultsReader
	storage         storageRuntimeReader
	assessor        createTargetAssessor
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
			StorageLocation:   s.storageLocation,
			Strategy:          defaults.Strategy,
			DuplicateHandling: defaults.DuplicateHandling,
		}
		if s.assessor != nil && !status.PrimaryRepositoryInitialized {
			// Read-only review of <default>/primary. A failure here is not a
			// status failure: the setup create reports the exact typed cause.
			if assessment, err := s.assessor.AssessCreateTarget(ctx, storage.AssessCreateTargetRequest{Intent: storage.IntentSetupPrimary}); err == nil {
				status.RepositoryDefaults.Assessment = assessment
			}
		}
	}

	return status, nil
}
