package dto

import "server/internal/service"

// SetupStatusDTO reports the durable owner and primary-repository gates.
type SetupStatusDTO struct {
	Initialized                  bool `json:"initialized"`
	AdminInitialized             bool `json:"admin_initialized"`
	PrimaryRepositoryInitialized bool `json:"primary_repository_initialized"`
	// RuntimeState is independent of bootstrap completion. An initialized
	// instance can be degraded while its default Storage Location is offline.
	RuntimeState  string `json:"runtime_state" example:"active"`
	RuntimeReason string `json:"runtime_reason,omitempty" example:"storage_recovery_required"`
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
		RuntimeState:                 status.RuntimeState,
		RuntimeReason:                status.RuntimeReason,
		NextRegistrationRole:         status.NextRegistrationRole,
	}
	if status.RepositoryDefaults != nil {
		result.RepositoryDefaults = &RepositoryDefaultsDTO{
			DefaultRoot:       status.RepositoryDefaults.DefaultRoot,
			Strategy:          status.RepositoryDefaults.Strategy,
			DuplicateHandling: status.RepositoryDefaults.DuplicateHandling,
			RiskWarnings:      status.RepositoryDefaults.RiskWarnings,
		}
	}
	return result
}
