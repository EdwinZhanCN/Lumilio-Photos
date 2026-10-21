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

// StorageLocationViewDTO carries registration identity and the Location
// commands the kind-aware policy admits. Capacity and health are never
// authoritative on a Storage Location.
type StorageLocationViewDTO struct {
	ID              string `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Name            string `json:"name" example:"Default Storage"`
	Kind            string `json:"kind" example:"default" enums:"default,external"`
	CanRemove       bool   `json:"can_remove"`
	BlockingReason  string `json:"blocking_reason,omitempty" example:"registered_repositories"`
	FilesPreserved  bool   `json:"files_preserved" example:"true"`
	RepositoryCount int64  `json:"repository_count" example:"2"`
	// Actions are the Location commands the policy admits on the current
	// facts (reconnect needs a positively unavailable original; detach needs
	// an empty external Location); UnavailableActions explain the withheld
	// ones with the shared blocker vocabulary. Menus render from these and
	// never invent a protected action.
	Actions            []string                   `json:"actions" example:"reconnect" enums:"reconnect,detach"`
	UnavailableActions []RepositoryActionBlockDTO `json:"unavailable_actions"`
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
	MountPath    string                          `json:"mount_path,omitempty" example:"/data/storage"`
	Filesystem   string                          `json:"filesystem,omitempty" example:"ext4"`
	Reachability string                          `json:"reachability" example:"active"`
	WritePolicy  StorageRepositoryWritePolicyDTO `json:"write_policy"`
	Activity     string                          `json:"activity" example:"idle"`
	MissingCount int64                           `json:"missing_count"`
	TrashCount   int64                           `json:"trash_count"`
	TrashBytes   int64                           `json:"trash_bytes"`
	// TrashHeldCount is how many trashed files are inherited and held out of
	// automatic expiry; RecoveryItemCount is how many preserved private files
	// still await a decision in the Repository's recovery inventory.
	TrashHeldCount    int64                                    `json:"trash_held_count"`
	RecoveryItemCount int64                                    `json:"recovery_item_count"`
	AssetCount        *int64                                   `json:"asset_count,omitempty" example:"1240"`
	Verification      *StorageRepositoryVerificationSummaryDTO `json:"verification,omitempty"`
	// Actions are the row commands the role-aware policy admits on the
	// current facts; UnavailableActions explain the withheld ones. Menus
	// render from these and never invent a protected action.
	Actions            []string                   `json:"actions" example:"verify,rename,detach" enums:"verify,rename,detach,reconnect"`
	UnavailableActions []RepositoryActionBlockDTO `json:"unavailable_actions"`
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
// SetupPrimaryRepositoryRequestDTO creates the Primary Repository during
// authenticated setup. The acknowledgement binds to the assessment token the
// setup status reports for the Default Storage Location's primary target.
type SetupPrimaryRepositoryRequestDTO struct {
	Name            string `json:"name" binding:"required" example:"Primary Repository"`
	StorageStrategy string `json:"storage_strategy,omitempty" binding:"omitempty,oneof=date flat cas" example:"date"`
	// AssessmentToken and AcknowledgedRisks mirror CreateRepositoryRequestDTO;
	// the token comes from the setup status assessment or a
	// storage/confirmation-required Problem.
	AssessmentToken   string   `json:"assessment_token,omitempty" example:"v1.0123456789abcdef"`
	AcknowledgedRisks []string `json:"acknowledged_risks,omitempty" example:"removable_storage"`
}

// RepositoryRecoveryReceiptDTO is one registration's private-recovery
// receipt: what an open, copy, or adoption found under the Repository's
// private workspace. Counts are by class; files that need a decision are the
// inventory items.
type RepositoryRecoveryReceiptDTO struct {
	ID                 string    `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Event              string    `json:"event" example:"open" enums:"open,register_copy,adopt"`
	RecordedAt         time.Time `json:"recorded_at"`
	Actor              string    `json:"actor,omitempty" example:"web:user:1"`
	SourceRepositoryID string    `json:"source_repository_id,omitempty" example:"550e8400-e29b-41d4-a716-446655440000"`
	TrashFiles         int       `json:"trash_files" example:"2"`
	TrashBytes         int64     `json:"trash_bytes" example:"4096"`
	LogFiles           int       `json:"log_files" example:"2"`
	DerivedFiles       int       `json:"derived_files" example:"12"`
	ItemsTruncated     int       `json:"items_truncated,omitempty" example:"0"`
}

// RepositoryRecoveryItemDTO is one preserved file awaiting or past a
// decision. Path is relative to the Repository's private workspace and
// never a host path.
type RepositoryRecoveryItemDTO struct {
	ID        string `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	ReceiptID string `json:"receipt_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Class     string `json:"class" example:"staging" enums:"trash,studio,staging,logs,derived,unknown"`
	Path      string `json:"path" example:".lumilio/staging/incoming/upload.part"`
	Size      int64  `json:"size" example:"1048576"`
	// Disposition is what became of the item: preserved or moved bytes await
	// a decision; reassociated is a Studio record attached to its proven
	// Asset; ambiguous and unreadable records stay for review or export;
	// deleted was removed explicitly.
	Disposition string `json:"disposition" example:"moved" enums:"preserved,moved,reassociated,ambiguous,unreadable,deleted"`
	Present     bool   `json:"present" example:"true"`
	// Actions are the explicit decisions available for this item.
	Actions         []string   `json:"actions" enums:"export,delete"`
	AssetID         string     `json:"asset_id,omitempty" example:"550e8400-e29b-41d4-a716-446655440000"`
	ResolvedAssetID string     `json:"resolved_asset_id,omitempty" example:"550e8400-e29b-41d4-a716-446655440000"`
	ResolvedAt      *time.Time `json:"resolved_at,omitempty"`
	Detail          string     `json:"detail,omitempty" example:"no present file in the Repository has the record's content"`
}

// RepositoryRecoveryTrashDTO counts the inherited trashed files held out of
// automatic expiry until the user decides.
type RepositoryRecoveryTrashDTO struct {
	HeldCount int64 `json:"held_count" example:"3"`
	HeldBytes int64 `json:"held_bytes" example:"6291456"`
}

// RepositoryRecoveryDTO is the private-recovery inventory of one Repository.
type RepositoryRecoveryDTO struct {
	RepositoryID       string                         `json:"repository_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Receipts           []RepositoryRecoveryReceiptDTO `json:"receipts"`
	Items              []RepositoryRecoveryItemDTO    `json:"items"`
	Trash              RepositoryRecoveryTrashDTO     `json:"trash"`
	UnreadableReceipts []string                       `json:"unreadable_receipts"`
}
