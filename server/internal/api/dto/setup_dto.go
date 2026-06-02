package dto

import "server/internal/service"

// SetupStatusDTO reports whether the system configuration payload exists on disk.
type SetupStatusDTO struct {
	Initialized bool `json:"initialized"`
}

// SetupRequestDTO is the first-run setup payload submitted from the web wizard.
type SetupRequestDTO struct {
	SiteName      string `json:"site_name"`
	AdminUsername string `json:"admin_username"`
}

// SetupResultDTO summarises a completed first-run initialization.
type SetupResultDTO struct {
	SiteName       string `json:"site_name"`
	AdminUsername  string `json:"admin_username"`
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
		SiteName:       result.SiteName,
		AdminUsername:  result.AdminUsername,
		DatabaseUser:   result.DatabaseUser,
		PasswordLength: result.PasswordLength,
	}
}
