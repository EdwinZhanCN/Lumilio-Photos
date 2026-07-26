package service

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"server/config"
	"server/internal/storage"
)

// SetupRequest carries the first-run setup payload submitted from the web wizard.
// SQLite is created and migrated before HTTP serving starts, so no database
// credential bootstrap payload is required.
type SetupRequest struct{}

// SetupStatus reports the durable first-run gates.
type SetupStatus struct {
	Initialized                  bool
	DatabaseInitialized          bool
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

// SetupResult is retained as the setup endpoint response contract. SQLite has
// no database role or password to rotate, so credential fields are empty.
type SetupResult struct {
	DatabaseUser   string `json:"database_user"`
	SecretPath     string `json:"secret_path"`
	PasswordLength int    `json:"password_length"`
}

// SetupService exposes first-run state. Opening and migrating the configured
// SQLite file is the database initialization step and happens before this
// service is constructed.
type SetupService struct {
	dbConfig     config.DatabaseConfig
	bootstrap    BootstrapService
	repoDefaults repositoryDefaultsReader
	storageRoot  string
}

func NewSetupService(dbConfig config.DatabaseConfig) *SetupService {
	return &SetupService{dbConfig: dbConfig}
}

// NewSetupServiceWithPool keeps the construction boundary shared by standalone
// and desktop runtimes. The live database proves SQLite initialization; setup
// does not mutate credentials.
func NewSetupServiceWithPool(
	dbConfig config.DatabaseConfig,
	_ *sql.DB,
	bootstrap BootstrapService,
	repoDefaults repositoryDefaultsReader,
	storageRoot string,
) *SetupService {
	return &SetupService{
		dbConfig:     dbConfig,
		bootstrap:    bootstrap,
		repoDefaults: repoDefaults,
		storageRoot:  strings.TrimSpace(storageRoot),
	}
}

func (s *SetupService) Status(ctx context.Context) (SetupStatus, error) {
	status := SetupStatus{DatabaseInitialized: strings.TrimSpace(s.dbConfig.Path) != ""}

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

// Initialize is an idempotent reconciliation endpoint. SQLite requires no
// database role/password setup; durable readiness is determined by the owner
// user and primary repository gates.
func (s *SetupService) Initialize(ctx context.Context, _ SetupRequest) (SetupResult, error) {
	if s.bootstrap != nil {
		if _, err := s.bootstrap.Reconcile(ctx); err != nil {
			return SetupResult{}, fmt.Errorf("reconcile bootstrap phase: %w", err)
		}
	}
	return SetupResult{}, nil
}
