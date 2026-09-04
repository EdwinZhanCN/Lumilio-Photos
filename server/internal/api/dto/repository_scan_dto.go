package dto

import (
	"encoding/json"
	"time"

	"server/internal/api/problem"
)

type CreateRepositoryRequestDTO struct {
	// Name is the mutable repository display name and is independent of its
	// stable on-disk folder.
	Name string `json:"name" binding:"required" example:"Family Photos"`
	// DirectoryName is the single direct-child folder segment below the selected
	// Storage Location. It is required for regular repositories and omitted for
	// the primary repository, whose folder is always "primary".
	DirectoryName string `json:"directory_name,omitempty" example:"family-photos"`
	// RootID identifies a registered Storage Location. Empty selects the
	// configured default location. Clients never submit an arbitrary root path.
	RootID           string `json:"root_id,omitempty" example:"550e8400-e29b-41d4-a716-446655440000"`
	Role             string `json:"role,omitempty" binding:"omitempty,oneof=primary regular" example:"regular"`
	StorageStrategy  string `json:"storage_strategy,omitempty" binding:"omitempty,oneof=date flat cas" example:"date"`
	RiskConfirmation bool   `json:"risk_confirmation,omitempty"`
}

type RepositoryDTO struct {
	ID              string                  `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Name            string                  `json:"name" example:"Family Photos"`
	Path            string                  `json:"path" example:"/data/storage/Family Photos"`
	Role            string                  `json:"role" example:"regular"`
	IsPrimary       bool                    `json:"is_primary" example:"false"`
	RootID          string                  `json:"root_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Reachability    string                  `json:"reachability" example:"active"`
	Activity        string                  `json:"activity" example:"idle"`
	DefaultOwnerID  *int32                  `json:"default_owner_id,omitempty"`
	StorageStrategy string                  `json:"storage_strategy" example:"date"`
	LocalSettings   RepositoryLocalSettings `json:"local_settings"`
}

type RepositoryLocalSettings struct {
	HandleDuplicateFilenames string `json:"handle_duplicate_filenames" example:"uuid"`
}

type RenameRepositoryRequestDTO struct {
	Name string `json:"name" binding:"required" example:"Family Archive"`
}

type RepositoryRemovalImpactDTO struct {
	RepositoryID      string `json:"repository_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	RepositoryName    string `json:"repository_name" example:"Family Photos"`
	AssetCount        int64  `json:"asset_count" example:"1240"`
	CatalogMediaBytes int64  `json:"catalog_media_bytes" example:"4294967296"`
	AlbumCount        int64  `json:"album_count" example:"12"`
	ActiveTaskCount   int64  `json:"active_task_count" example:"0"`
	CloudImportCount  int64  `json:"cloud_import_count" example:"2"`
	PrivateStateBytes int64  `json:"private_state_bytes" example:"1048576"`
	PrivateStateFound bool   `json:"private_state_found" example:"true"`
	FilesPreserved    bool   `json:"files_preserved" example:"true"`
}

type RemoveRepositoryRequestDTO struct {
	// ConfirmationName must exactly match the current repository display name.
	ConfirmationName string `json:"confirmation_name" binding:"required" example:"Family Photos"`
}

type ListRepositoriesResponseDTO struct {
	Repositories []RepositoryDTO `json:"repositories"`
}

type CreateRepositoryResponseDTO struct {
	Repository RepositoryDTO `json:"repository"`
	// Warnings are non-fatal notes about the chosen location, such as it being
	// inside a cloud-sync folder. The repository was created regardless.
	Warnings []string `json:"warnings,omitempty"`
}

type RepositoryRootDTO struct {
	ID                         string   `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Name                       string   `json:"name" example:"External Archive"`
	Path                       string   `json:"path" example:"/Volumes/Photos"`
	Kind                       string   `json:"kind" example:"external"`
	Status                     string   `json:"status" example:"active"`
	Writable                   bool     `json:"writable"`
	CapacityKnown              bool     `json:"capacity_known"`
	TotalBytes                 uint64   `json:"total_bytes,omitempty" example:"1000000000000"`
	AvailableBytes             uint64   `json:"available_bytes,omitempty" example:"500000000000"`
	Filesystem                 string   `json:"filesystem,omitempty" example:"apfs"`
	RepositoryCount            int64    `json:"repository_count" example:"2"`
	ActiveOperationCount       int64    `json:"active_operation_count" example:"0"`
	CanRemove                  bool     `json:"can_remove"`
	RemovalBlockedBy           string   `json:"removal_blocked_by,omitempty" example:"registered_repositories"`
	FilesPreserved             bool     `json:"files_preserved"`
	RiskWarnings               []string `json:"risk_warnings,omitempty"`
	MountFingerprint           string   `json:"mount_fingerprint,omitempty"`
	RegisteredMountFingerprint string   `json:"registered_mount_fingerprint,omitempty"`
	MountFingerprintChanged    bool     `json:"mount_fingerprint_changed"`
}

type ListRepositoryRootsResponseDTO struct {
	Roots []RepositoryRootDTO `json:"roots"`
}

type LifecycleAuditEventDTO struct {
	EventID          string          `json:"event_id"`
	OccurredAt       time.Time       `json:"occurred_at"`
	Actor            string          `json:"actor"`
	ActorUserID      *int32          `json:"actor_user_id,omitempty"`
	HostInstanceID   string          `json:"host_instance_id,omitempty"`
	RequestID        string          `json:"request_id,omitempty"`
	OperationID      string          `json:"operation_id,omitempty"`
	Action           string          `json:"action"`
	TargetType       string          `json:"target_type"`
	TargetID         string          `json:"target_id,omitempty"`
	Source           string          `json:"source"`
	ConfirmationType string          `json:"confirmation_type"`
	OldPath          string          `json:"old_path,omitempty"`
	NewPath          string          `json:"new_path,omitempty"`
	Result           string          `json:"result"`
	FailureStage     string          `json:"failure_stage,omitempty"`
	Details          json.RawMessage `json:"details" swaggertype:"object"`
}

type ListLifecycleAuditEventsResponseDTO struct {
	Events []LifecycleAuditEventDTO `json:"events"`
}

type StorageDiagnosticDTO struct {
	TargetType                 string     `json:"target_type"`
	TargetID                   string     `json:"target_id"`
	ParentTargetID             string     `json:"parent_target_id,omitempty"`
	Kind                       string     `json:"kind,omitempty" example:"default" enums:"default,external"`
	Role                       string     `json:"role,omitempty" example:"primary" enums:"primary,regular"`
	Name                       string     `json:"name"`
	Path                       string     `json:"path"`
	CanonicalPath              string     `json:"canonical_path"`
	Reachability               string     `json:"reachability"`
	Writable                   bool       `json:"writable"`
	CapacityKnown              bool       `json:"capacity_known"`
	TotalBytes                 uint64     `json:"total_bytes,omitempty"`
	AvailableBytes             uint64     `json:"available_bytes,omitempty"`
	SafetyMarginBytes          uint64     `json:"safety_margin_bytes,omitempty"`
	WritableBudgetBytes        uint64     `json:"writable_budget_bytes,omitempty"`
	Filesystem                 string     `json:"filesystem,omitempty"`
	MountID                    string     `json:"mount_id,omitempty"`
	MountSource                string     `json:"mount_source,omitempty"`
	Device                     string     `json:"device,omitempty"`
	Inode                      uint64     `json:"inode,omitempty"`
	EffectiveUID               string     `json:"effective_uid,omitempty"`
	EffectiveGID               string     `json:"effective_gid,omitempty"`
	CaseBehaviorKnown          bool       `json:"case_behavior_known"`
	CaseSensitive              bool       `json:"case_sensitive"`
	LockHolder                 string     `json:"lock_holder,omitempty"`
	LastCoordination           *time.Time `json:"last_coordination,omitempty"`
	MarkerUUID                 string     `json:"marker_uuid,omitempty"`
	MountFingerprint           string     `json:"mount_fingerprint,omitempty"`
	RegisteredMountFingerprint string     `json:"registered_mount_fingerprint,omitempty"`
	MountFingerprintChanged    bool       `json:"mount_fingerprint_changed"`
	NetworkFilesystem          bool       `json:"network_filesystem"`
	RemovableLikely            bool       `json:"removable_likely"`
	CloudSyncProvider          string     `json:"cloud_sync_provider,omitempty"`
	RiskWarnings               []string   `json:"risk_warnings,omitempty"`
}

type StorageDiagnosticsResponseDTO struct {
	GeneratedAt time.Time              `json:"generated_at"`
	Items       []StorageDiagnosticDTO `json:"items"`
}

type StorageSupportBundleDTO struct {
	GeneratedAt   time.Time                `json:"generated_at"`
	PathsRedacted bool                     `json:"paths_redacted"`
	Diagnostics   []StorageDiagnosticDTO   `json:"diagnostics"`
	AuditEvents   []LifecycleAuditEventDTO `json:"audit_events"`
}

type RepositoryCandidateDTO struct {
	DirectoryName      string   `json:"directory_name" example:"family-archive"`
	Classification     string   `json:"classification" example:"existing_repository"`
	RepositoryID       string   `json:"repository_id,omitempty" example:"550e8400-e29b-41d4-a716-446655440000"`
	Name               string   `json:"name,omitempty" example:"Family Archive"`
	Writable           bool     `json:"writable"`
	MountPoint         bool     `json:"mount_point"`
	CanCreate          bool     `json:"can_create"`
	CanOpen            bool     `json:"can_open"`
	AllowedResolutions []string `json:"allowed_resolutions,omitempty" example:"update_location,add_separate"`
	CapacityKnown      bool     `json:"capacity_known"`
	TotalBytes         uint64   `json:"total_bytes,omitempty"`
	AvailableBytes     uint64   `json:"available_bytes,omitempty"`
	Filesystem         string   `json:"filesystem,omitempty"`
	RiskWarnings       []string `json:"risk_warnings,omitempty"`
}

type ListRepositoryCandidatesResponseDTO struct {
	Candidates []RepositoryCandidateDTO `json:"candidates"`
}

type OpenRepositoryCandidateRequestDTO struct {
	// DirectoryName is a portable direct-child folder segment below the
	// configured default Storage Location, never an arbitrary host path.
	DirectoryName    string `json:"directory_name" binding:"required" example:"family-archive"`
	RiskConfirmation bool   `json:"risk_confirmation,omitempty"`
}

type ResolveRepositoryCandidateRequestDTO struct {
	DirectoryName    string `json:"directory_name" binding:"required" example:"family-archive"`
	Resolution       string `json:"resolution" binding:"required,oneof=update_location add_separate" example:"update_location"`
	RiskConfirmation bool   `json:"risk_confirmation,omitempty"`
}

type RepositoryScanRequestDTO struct {
	Force bool `json:"force" example:"false"`
}

type RepositoryScanQueuedDTO struct {
	OperationID  string `json:"operation_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	RepositoryID string `json:"repository_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Mode         string `json:"mode" example:"manual"`
	Status       string `json:"status" example:"queued"`
	Inserted     bool   `json:"inserted" example:"true"`
	Coalesced    bool   `json:"coalesced" example:"false"`
}

type RepositoryScanRunDTO struct {
	OperationID              string             `json:"operation_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	RepositoryID             string             `json:"repository_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	RequestedEpoch           int64              `json:"requested_epoch" example:"2"`
	Mode                     string             `json:"mode" example:"manual"`
	RequestedBy              *string            `json:"requested_by,omitempty" example:"edwin"`
	CoalescedCount           int64              `json:"coalesced_count" example:"1"`
	Status                   string             `json:"status" example:"completed"`
	CreatedAt                time.Time          `json:"created_at"`
	StartedAt                *time.Time         `json:"started_at,omitempty"`
	FinishedAt               *time.Time         `json:"finished_at,omitempty"`
	DirectoriesObserved      int64              `json:"directories_observed" example:"10"`
	FilesObserved            int64              `json:"files_observed" example:"120"`
	BytesQueued              int64              `json:"bytes_queued" example:"1048576"`
	BytesHashed              int64              `json:"bytes_hashed" example:"524288"`
	AuthoritativeDirectories int64              `json:"authoritative_directories" example:"8"`
	ErrorDirectories         int64              `json:"error_directories" example:"1"`
	OutboxDepth              int64              `json:"outbox_depth" example:"12"`
	PartialCoverage          bool               `json:"partial_coverage" example:"true"`
	CancellationRequested    bool               `json:"cancellation_requested" example:"false"`
	Problem                  *problem.Reference `json:"problem,omitempty"`
}

type RepositoryScanRunListDTO struct {
	Scans []RepositoryScanRunDTO `json:"scans"`
}
