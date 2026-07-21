package dto

import "time"

// CloudProviderFieldDTO describes one provider-specific form input.
// Label, Placeholder, and HelpText carry frontend i18n keys.
type CloudProviderFieldDTO struct {
	Name         string   `json:"name" example:"username"`
	Label        string   `json:"label" example:"cloudProvider.icloud.field.username"`
	Type         string   `json:"type" example:"email"`
	Required     bool     `json:"required" example:"true"`
	Placeholder  string   `json:"placeholder,omitempty" example:"you@example.com"`
	HelpText     string   `json:"help_text,omitempty"`
	Options      []Option `json:"options,omitempty"`
	Autocomplete string   `json:"autocomplete,omitempty" example:"username"`
}

// Option is a generic select option.
type Option struct {
	Value string `json:"value" example:"com"`
	Label string `json:"label" example:"Global"`
}

// CloudProviderDTO describes a cloud provider that can create credentials.
// Title, Description, and SecurityNote carry frontend i18n keys.
type CloudProviderDTO struct {
	ID              string                  `json:"id" example:"icloud"`
	Title           string                  `json:"title" example:"cloudProvider.icloud.title"`
	Description     string                  `json:"description" example:"cloudProvider.icloud.description"`
	Status          string                  `json:"status" example:"enabled"`
	FormFields      []CloudProviderFieldDTO `json:"form_fields"`
	ChallengeFields []CloudProviderFieldDTO `json:"challenge_fields,omitempty"`
	SecurityNote    string                  `json:"security_note,omitempty"`
}

// ListCloudProvidersResponse is the response for listing provider descriptors.
type ListCloudProvidersResponse struct {
	Providers []CloudProviderDTO `json:"providers"`
}

// CreateCloudCredentialRequest is the provider-neutral request body for creating a cloud credential.
type CreateCloudCredentialRequest struct {
	Provider    string            `json:"provider" binding:"required" example:"icloud"`
	DisplayName string            `json:"display_name,omitempty" example:"Personal cloud account"`
	Inputs      map[string]string `json:"inputs" binding:"required"`
}

// VerifyCloudAuthChallengeRequest submits provider-specific challenge inputs.
type VerifyCloudAuthChallengeRequest struct {
	Inputs map[string]string `json:"inputs" binding:"required"`
}

// ReconnectCloudCredentialRequest is the request for reconnecting a credential.
type ReconnectCloudCredentialRequest struct {
	Inputs map[string]string `json:"inputs,omitempty"`
}

// CloudAuthChallengeDTO describes a pending credential authentication challenge.
// Title and Description carry frontend i18n keys; Params holds interpolation values.
type CloudAuthChallengeDTO struct {
	Type        string                  `json:"type" example:"verification_code"`
	Title       string                  `json:"title" example:"cloudProvider.icloud.challenge.sms.title"`
	Description string                  `json:"description" example:"cloudProvider.icloud.challenge.sms.description"`
	Params      map[string]string       `json:"params,omitempty"`
	Fields      []CloudProviderFieldDTO `json:"fields"`
}

// CloudCredentialDTO is a safe public view of a saved cloud credential.
type CloudCredentialDTO struct {
	ID             string            `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Provider       string            `json:"provider" example:"icloud"`
	ProviderTitle  string            `json:"provider_title" example:"cloudProvider.icloud.title"`
	DisplayName    string            `json:"display_name" example:"Personal cloud account"`
	MaskedIdentity string            `json:"masked_identity" example:"u***r@example.com"`
	Status         string            `json:"status" example:"connected"`
	PublicConfig   map[string]string `json:"public_config,omitempty"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
}

// CreateCloudCredentialResponse is the response for credential creation.
type CreateCloudCredentialResponse struct {
	Credential CloudCredentialDTO     `json:"credential"`
	AuthStatus string                 `json:"auth_status" example:"connected"`
	Challenge  *CloudAuthChallengeDTO `json:"challenge,omitempty"`
}

// VerifyCloudAuthChallengeResponse is returned after challenge verification.
type VerifyCloudAuthChallengeResponse struct {
	Credential CloudCredentialDTO `json:"credential"`
	AuthStatus string             `json:"auth_status" example:"connected"`
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
