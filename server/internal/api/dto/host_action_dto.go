package dto

import "time"

// NativeHostCapabilityDTO tells the Web client whether this Server process has
// an in-process Desktop host that can present a native directory picker.
type NativeHostCapabilityDTO struct {
	Available bool `json:"available"`
}

type CreateHostActionRequestDTO struct {
	Kind             string `json:"kind" binding:"required,oneof=authorize_storage_location open_repository locate_storage_location locate_repository" example:"open_repository"`
	SessionID        string `json:"session_id,omitempty" example:"web-session-4d4d"`
	Purpose          string `json:"purpose,omitempty" example:"Open an existing photo repository"`
	Name             string `json:"name,omitempty" example:"External Archive"`
	RootID           string `json:"root_id,omitempty" example:"550e8400-e29b-41d4-a716-446655440000"`
	RepositoryID     string `json:"repository_id,omitempty" example:"550e8400-e29b-41d4-a716-446655440000"`
	ExpectedVersion  uint64 `json:"expected_version,omitempty" example:"0"`
	ExpiresInSeconds int64  `json:"expires_in_seconds,omitempty" example:"600"`
}

type ResolveHostActionRequestDTO struct {
	Resolution       string `json:"resolution" binding:"required,oneof=update_location add_separate confirm_risk" example:"add_separate"`
	RiskConfirmation bool   `json:"risk_confirmation,omitempty"`
}

type HostActionConflictDTO struct {
	Type               string   `json:"type" example:"repository_identity"`
	RepositoryID       string   `json:"repository_id,omitempty" example:"550e8400-e29b-41d4-a716-446655440000"`
	RootID             string   `json:"root_id,omitempty" example:"550e8400-e29b-41d4-a716-446655440000"`
	AllowedResolutions []string `json:"allowed_resolutions,omitempty" example:"add_separate"`
	RiskWarnings       []string `json:"risk_warnings,omitempty"`
}

type HostActionResultDTO struct {
	RepositoryID string                 `json:"repository_id,omitempty" example:"550e8400-e29b-41d4-a716-446655440000"`
	RootID       string                 `json:"root_id,omitempty" example:"550e8400-e29b-41d4-a716-446655440000"`
	Name         string                 `json:"name,omitempty" example:"External Archive"`
	Conflict     *HostActionConflictDTO `json:"conflict,omitempty"`
}

// HostActionDTO intentionally omits both the native-host nonce and the selected
// filesystem path. Those values stay inside the in-process Desktop control
// plane and never enter the shared HTTP contract.
type HostActionDTO struct {
	ID              string               `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	RequestID       string               `json:"request_id" example:"web-host-action-4d4d"`
	Kind            string               `json:"kind" example:"open_repository"`
	Actor           string               `json:"actor" example:"web:user:1"`
	Purpose         string               `json:"purpose,omitempty" example:"Open an existing photo repository"`
	Name            string               `json:"name,omitempty" example:"External Archive"`
	RootID          string               `json:"root_id,omitempty" example:"550e8400-e29b-41d4-a716-446655440000"`
	RepositoryID    string               `json:"repository_id,omitempty" example:"550e8400-e29b-41d4-a716-446655440000"`
	ExpectedVersion uint64               `json:"expected_version,omitempty"`
	Status          string               `json:"status" example:"pending"`
	Result          *HostActionResultDTO `json:"result,omitempty"`
	ErrorCode       string               `json:"error_code,omitempty" example:"expired"`
	ErrorMessage    string               `json:"error_message,omitempty" example:"Native host approval expired"`
	ExpiresAt       time.Time            `json:"expires_at"`
	CreatedAt       time.Time            `json:"created_at"`
	UpdatedAt       time.Time            `json:"updated_at"`
	CompletedAt     *time.Time           `json:"completed_at,omitempty"`
}
