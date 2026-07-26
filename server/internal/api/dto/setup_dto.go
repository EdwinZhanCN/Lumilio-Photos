package dto

import "server/internal/service"

// SetupStatusDTO reports the durable owner and primary-repository gates.
type SetupStatusDTO struct {
	Initialized                  bool `json:"initialized"`
	AdminInitialized             bool `json:"admin_initialized"`
	PrimaryRepositoryInitialized bool `json:"primary_repository_initialized"`
	// NextRegistrationRole is the role the next /auth/register will assign
	// ("admin" while no admin exists yet, "user" afterwards). Folds the former
	// /auth/bootstrap-status semantics into the unified setup status.
	NextRegistrationRole string                 `json:"next_registration_role"`
	RepositoryDefaults   *RepositoryDefaultsDTO `json:"repository_defaults,omitempty"`
}

// ToSetupStatusDTO maps a service SetupStatus to its transport DTO.
func ToSetupStatusDTO(status service.SetupStatus) SetupStatusDTO {
	result := SetupStatusDTO{
		Initialized:                  status.Initialized,
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
