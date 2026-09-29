package dto

import "time"

// AdmissionDecisionDTO is the per-operation eligibility projection exposed on
// GET /api/v1/storage/targets. An empty reasons slice means unrestricted.
type AdmissionDecisionDTO struct {
	Allowed bool     `json:"allowed" example:"true"`
	Reasons []string `json:"reasons" example:"offline,paused"`
}

// StorageTargetDTO is one browse/upload selector entry. It intentionally omits
// paths, Storage Location identity, capacity, diagnostics, and verification.
type StorageTargetDTO struct {
	ID     string               `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Name   string               `json:"name" example:"Family Photos"`
	Role   string               `json:"role" example:"regular" enums:"primary,regular"`
	Read   AdmissionDecisionDTO `json:"read"`
	Upload AdmissionDecisionDTO `json:"upload"`
}

// StorageTargetsResponseDTO is the authenticated non-admin storage selector.
type StorageTargetsResponseDTO struct {
	Targets []StorageTargetDTO `json:"targets"`
}

// StorageLocationViewDTO carries registration identity and detach facts only.
// Capacity and health are never authoritative on a Storage Location.
type StorageLocationViewDTO struct {
	ID              string `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Name            string `json:"name" example:"Default Storage"`
	Kind            string `json:"kind" example:"default" enums:"default,external"`
	CanRemove       bool   `json:"can_remove"`
	BlockingReason  string `json:"blocking_reason,omitempty" example:"registered_repositories"`
	FilesPreserved  bool   `json:"files_preserved" example:"true"`
	RepositoryCount int64  `json:"repository_count" example:"2"`
}

// StorageRepositoryWritePolicyDTO exposes manual pause and space facts separately
// from reachability and current activity.
type StorageRepositoryWritePolicyDTO struct {
	Activity    string `json:"activity" example:"idle"`
	PauseReason string `json:"pause_reason,omitempty" example:"manual"`
}

// StorageRepositoryVerificationSummaryDTO summarizes the latest verification
// run when one is already queryable without extra filesystem work.
type StorageRepositoryVerificationSummaryDTO struct {
	OperationID string `json:"operation_id,omitempty" example:"550e8400-e29b-41d4-a716-446655440000"`
	Status      string `json:"status,omitempty" example:"completed"`
	Mode        string `json:"mode,omitempty" example:"manual"`
}

// StorageRepositoryViewDTO is one admin storage grid row with distinct facts.
type StorageRepositoryViewDTO struct {
	ID                string  `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Name              string  `json:"name" example:"Family Photos"`
	Role              string  `json:"role" example:"regular" enums:"primary,regular"`
	StorageLocationID string  `json:"storage_location_id,omitempty" example:"550e8400-e29b-41d4-a716-446655440000"`
	CapacityGroupID   *string `json:"capacity_group_id,omitempty" example:"0"`
	// MountPath is the directory where the backing filesystem is mounted on
	// the host running the Server: "/", "/Volumes/Backup", "/volume1", or a
	// Windows drive root such as "C:". It names the storage an operator
	// recognizes. It is raw host data and is never localized by the Server.
	// Empty when the mount could not be resolved.
	MountPath    string                                   `json:"mount_path,omitempty" example:"/data/storage"`
	Filesystem   string                                   `json:"filesystem,omitempty" example:"ext4"`
	Reachability string                                   `json:"reachability" example:"active"`
	WritePolicy  StorageRepositoryWritePolicyDTO          `json:"write_policy"`
	Activity     string                                   `json:"activity" example:"idle"`
	AssetCount   *int64                                   `json:"asset_count,omitempty" example:"1240"`
	Verification *StorageRepositoryVerificationSummaryDTO `json:"verification,omitempty"`
}

// StorageCapacityGroupDTO is one shared backing capacity pool for admin display.
type StorageCapacityGroupDTO struct {
	ID             string `json:"id" example:"0"`
	GroupingKnown  bool   `json:"grouping_known" example:"true"`
	CapacityKnown  bool   `json:"capacity_known" example:"true"`
	TotalBytes     uint64 `json:"total_bytes,omitempty" example:"1000000000000"`
	AvailableBytes uint64 `json:"available_bytes,omitempty" example:"500000000000"`
}

// StorageViewResponseDTO is the administrator storage read model.
type StorageViewResponseDTO struct {
	StorageLocations []StorageLocationViewDTO   `json:"storage_locations"`
	Repositories     []StorageRepositoryViewDTO `json:"repositories"`
	CapacityGroups   []StorageCapacityGroupDTO  `json:"capacity_groups"`
	ObservedAt       time.Time                  `json:"observed_at"`
}

// StorageLocationRemovalImpactDTO previews Storage Location detach impact.
type StorageLocationRemovalImpactDTO struct {
	StorageLocationID    string `json:"storage_location_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	StorageLocationName  string `json:"storage_location_name" example:"External Archive"`
	Kind                 string `json:"kind" example:"external" enums:"default,external"`
	RepositoryCount      int64  `json:"repository_count" example:"0"`
	ActiveOperationCount int64  `json:"active_operation_count" example:"0"`
	CanRemove            bool   `json:"can_remove"`
	BlockingReason       string `json:"blocking_reason,omitempty" example:"registered_repositories"`
	FilesPreserved       bool   `json:"files_preserved" example:"true"`
}

// SetupPrimaryRepositoryRequestDTO creates the initial primary repository during
// authenticated first-run setup.
type SetupPrimaryRepositoryRequestDTO struct {
	Name             string `json:"name" binding:"required" example:"Primary Repository"`
	StorageStrategy  string `json:"storage_strategy,omitempty" binding:"omitempty,oneof=date flat cas" example:"date"`
	RiskConfirmation bool   `json:"risk_confirmation,omitempty"`
}
