package dto

import "server/internal/service"

// SetupStatusDTO reports whether database credential rotation has completed.
type SetupStatusDTO struct {
	Initialized bool `json:"initialized"`
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
	return SetupStatusDTO{Initialized: status.Initialized}
}

// ToSetupResultDTO maps a service SetupResult to its transport DTO. Sensitive
// values (secret/config paths, the password itself) are intentionally omitted.
func ToSetupResultDTO(result service.SetupResult) SetupResultDTO {
	return SetupResultDTO{
		DatabaseUser:   result.DatabaseUser,
		PasswordLength: result.PasswordLength,
	}
}
