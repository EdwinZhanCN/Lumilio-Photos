package dto

import (
	"time"

	"server/internal/api/problem"
)

// NativeHostCapabilityDTO tells the Web client whether this Server process has
// an in-process Desktop host that can present a native directory picker.
type NativeHostCapabilityDTO struct {
	Available bool `json:"available"`
}

type CreateHostActionRequestDTO struct {
	Kind              string `json:"kind" binding:"required,oneof=authorize_storage_location open_repository locate_storage_location locate_repository" example:"open_repository"`
	SessionID         string `json:"session_id,omitempty" example:"web-session-4d4d"`
	Purpose           string `json:"purpose,omitempty" example:"Open an existing photo repository"`
	Name              string `json:"name,omitempty" example:"External Archive"`
	StorageLocationID string `json:"storage_location_id,omitempty" example:"550e8400-e29b-41d4-a716-446655440000"`
	RepositoryID      string `json:"repository_id,omitempty" example:"550e8400-e29b-41d4-a716-446655440000"`
	ExpectedVersion   uint64 `json:"expected_version,omitempty" example:"0"`
	ExpiresInSeconds  int64  `json:"expires_in_seconds,omitempty" example:"600"`
}

// ResolveHostActionRequestDTO answers a review the native task stopped at. A
// `confirm_risk` resolution acknowledges the `storage_risk` review by its
// exact risk codes, the same consent shape as the create command; an
// acknowledgement that does not cover every reported code is refused.
type ResolveHostActionRequestDTO struct {
	Resolution        string   `json:"resolution" binding:"required,oneof=update_location add_separate confirm_risk" example:"add_separate"`
	AcknowledgedRisks []string `json:"acknowledged_risks,omitempty" example:"removable_storage"`
}

// HostActionRefusalDTO is the role-aware policy verdict that withheld a
// native task, with the same action, blocker, and alternative vocabulary as
// the storage/action-unavailable Problem of the HTTP command.
type HostActionRefusalDTO struct {
	Action       string   `json:"action" example:"reconnect" enums:"verify,rename,detach,reconnect"`
	Blockers     []string `json:"blockers,omitempty" example:"primary_protected"`
	Alternatives []string `json:"alternatives,omitempty" example:"default_configuration"`
}

// HostActionConflictDTO is a review the task stopped at: an identity conflict
// with its legal resolutions, or a `storage_risk` review whose risk_warnings
// are the stable risk codes of the shared storage assessment.
type HostActionConflictDTO struct {
	Type               string   `json:"type" example:"repository_identity" enums:"repository_identity,storage_location_identity,storage_risk"`
	RepositoryID       string   `json:"repository_id,omitempty" example:"550e8400-e29b-41d4-a716-446655440000"`
	StorageLocationID  string   `json:"storage_location_id,omitempty" example:"550e8400-e29b-41d4-a716-446655440000"`
	AllowedResolutions []string `json:"allowed_resolutions,omitempty" example:"add_separate"`
	RiskWarnings       []string `json:"risk_warnings,omitempty" example:"removable_storage"`
}

type HostActionResultDTO struct {
	RepositoryID      string                 `json:"repository_id,omitempty" example:"550e8400-e29b-41d4-a716-446655440000"`
	StorageLocationID string                 `json:"storage_location_id,omitempty" example:"550e8400-e29b-41d4-a716-446655440000"`
	Name              string                 `json:"name,omitempty" example:"External Archive"`
	Conflict          *HostActionConflictDTO `json:"conflict,omitempty"`
}

// HostActionDTO intentionally omits both the native-host nonce and the selected
// filesystem path. Those values stay inside the in-process Desktop control
// plane and never enter the shared HTTP contract.
type HostActionDTO struct {
	ID                string               `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	RequestID         string               `json:"request_id" example:"web-host-action-4d4d"`
	Kind              string               `json:"kind" example:"open_repository"`
	Actor             string               `json:"actor" example:"web:user:1"`
	Purpose           string               `json:"purpose,omitempty" example:"Open an existing photo repository"`
	Name              string               `json:"name,omitempty" example:"External Archive"`
	StorageLocationID string               `json:"storage_location_id,omitempty" example:"550e8400-e29b-41d4-a716-446655440000"`
	RepositoryID      string               `json:"repository_id,omitempty" example:"550e8400-e29b-41d4-a716-446655440000"`
	ExpectedVersion   uint64               `json:"expected_version,omitempty"`
	Status            string               `json:"status" example:"pending" enums:"pending,running,needs_decision,succeeded,failed,cancelled,expired"`
	Result            *HostActionResultDTO `json:"result,omitempty"`
	// Problem names why a failed or expired task ended: the typed storage
	// Problem of the refusal when the cause has one, else the generic
	// host-action failure. Refusal carries the policy verdict's blockers.
	Problem     *problem.Reference    `json:"problem,omitempty"`
	Refusal     *HostActionRefusalDTO `json:"refusal,omitempty"`
	ExpiresAt   time.Time             `json:"expires_at"`
	CreatedAt   time.Time             `json:"created_at"`
	UpdatedAt   time.Time             `json:"updated_at"`
	CompletedAt *time.Time            `json:"completed_at,omitempty"`
}
