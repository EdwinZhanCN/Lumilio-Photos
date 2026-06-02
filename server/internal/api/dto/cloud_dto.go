package dto

import "time"

// CreateICloudCredentialRequest is the request body for creating an iCloud credential.
type CreateICloudCredentialRequest struct {
	Username    string `json:"username" binding:"required" example:"user@icloud.com"`
	Password    string `json:"password" binding:"required" example:"app-specific-password"`
	Domain      string `json:"domain,omitempty" binding:"omitempty,oneof=com cn" example:"com"`
	DisplayName string `json:"display_name,omitempty" example:"Personal iCloud"`
}

// VerifyICloud2FARequest is the request body for submitting a 2FA code.
type VerifyICloud2FARequest struct {
	Code string `json:"code" binding:"required" example:"123456"`
}

// CloudCredentialDTO is a safe public view of a saved cloud credential.
type CloudCredentialDTO struct {
	ID            string    `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Provider      string    `json:"provider" example:"icloud"`
	DisplayName   string    `json:"display_name" example:"Personal iCloud"`
	MaskedAccount string    `json:"masked_account" example:"u***r@icloud.com"`
	Domain        string    `json:"domain" example:"com"`
	Status        string    `json:"status" example:"connected"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// CreateICloudCredentialResponse is the response for iCloud credential creation.
type CreateICloudCredentialResponse struct {
	Credential CloudCredentialDTO `json:"credential"`
	Needs2FA   bool               `json:"needs_2fa"`
}

// ListCloudCredentialsResponse is the response for listing cloud credentials.
type ListCloudCredentialsResponse struct {
	Credentials []CloudCredentialDTO `json:"credentials"`
}

// CloudImportRunDTO is a safe public view of a repo-scoped cloud import run.
type CloudImportRunDTO struct {
	ID              string     `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	RepositoryID    string     `json:"repository_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	CredentialID    string     `json:"credential_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Provider        string     `json:"provider" example:"icloud"`
	Status          string     `json:"status" example:"running"`
	TotalSeen       int64      `json:"total_seen" example:"120"`
	DownloadedCount int64      `json:"downloaded_count" example:"80"`
	ImportedCount   int64      `json:"imported_count" example:"75"`
	SkippedCount    int64      `json:"skipped_count" example:"40"`
	FailedCount     int64      `json:"failed_count" example:"5"`
	Error           *string    `json:"error,omitempty"`
	StartedAt       *time.Time `json:"started_at,omitempty"`
	FinishedAt      *time.Time `json:"finished_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// StartCloudImportResponse is returned when a cloud import run is queued.
type StartCloudImportResponse struct {
	Run CloudImportRunDTO `json:"run"`
}

// RepositoryCloudStatusDTO describes a repository's cloud binding.
type RepositoryCloudStatusDTO struct {
	Provider      string              `json:"provider,omitempty" example:"icloud"`
	Enabled       bool                `json:"enabled" example:"true"`
	Credential    *CloudCredentialDTO `json:"credential,omitempty"`
	LatestRun     *CloudImportRunDTO  `json:"latest_run,omitempty"`
	LastImportRun string              `json:"last_import_run_id,omitempty"`
}
