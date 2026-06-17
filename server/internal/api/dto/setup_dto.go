package dto

import "server/internal/service"

// SetupStatusDTO reports whether database credential rotation has completed.
type SetupStatusDTO struct {
	Initialized                  bool `json:"initialized"`
	DatabaseInitialized          bool `json:"database_initialized"`
	AdminInitialized             bool `json:"admin_initialized"`
	PrimaryRepositoryInitialized bool `json:"primary_repository_initialized"`
	// NextRegistrationRole is the role the next /auth/register will assign
	// ("admin" while no admin exists yet, "user" afterwards). Folds the former
	// /auth/bootstrap-status semantics into the unified setup status.
	NextRegistrationRole string                 `json:"next_registration_role"`
	RepositoryDefaults   *RepositoryDefaultsDTO `json:"repository_defaults,omitempty"`
}

// SetupRequestDTO is intentionally empty: first-run setup is a server preflight
// action that rotates and persists the database credential.
type SetupRequestDTO struct{}

// SetupResultDTO summarises a completed first-run initialization.
type SetupResultDTO struct {
	DatabaseUser   string `json:"database_user"`
	PasswordLength int    `json:"password_length"`
}

// ToSetupStatusDTO maps a service SetupStatus to its transport DTO.
func ToSetupStatusDTO(status service.SetupStatus) SetupStatusDTO {
	result := SetupStatusDTO{
		Initialized:                  status.Initialized,
		DatabaseInitialized:          status.DatabaseInitialized,
		AdminInitialized:             status.AdminInitialized,
		PrimaryRepositoryInitialized: status.PrimaryRepositoryInitialized,
		NextRegistrationRole:         status.NextRegistrationRole,
	}
	if status.RepositoryDefaults != nil {
		result.RepositoryDefaults = &RepositoryDefaultsDTO{
			DefaultRoot:       status.RepositoryDefaults.DefaultRoot,
			Strategy:          status.RepositoryDefaults.Strategy,
			DuplicateHandling: status.RepositoryDefaults.DuplicateHandling,
		}
	}
	return result
}

// ToSetupResultDTO maps a service SetupResult to its transport DTO. Sensitive
// values (secret/config paths, the password itself) are intentionally omitted.
func ToSetupResultDTO(result service.SetupResult) SetupResultDTO {
	return SetupResultDTO{
		DatabaseUser:   result.DatabaseUser,
		PasswordLength: result.PasswordLength,
	}
}
