package dto

import (
	"encoding/json"
	"time"

	"server/internal/api/problem"
	"server/internal/storage"
)

type CreateRepositoryRequestDTO struct {
	// Name is the mutable repository display name and is independent of its
	// stable on-disk folder.
	Name string `json:"name" binding:"required" example:"Family Photos"`
	// DirectoryName is the single direct-child folder segment below the selected
	// Storage Location. It is required for regular repositories and omitted for
	// the primary repository, whose folder is always "primary".
	DirectoryName string `json:"directory_name,omitempty" example:"family-photos"`
	// StorageLocationID identifies a registered Storage Location. Empty selects the
	// configured default location. Clients never submit an arbitrary root path.
	StorageLocationID string `json:"storage_location_id,omitempty" example:"550e8400-e29b-41d4-a716-446655440000"`
	Role              string `json:"role,omitempty" binding:"omitempty,oneof=primary regular" example:"regular"`
	StorageStrategy   string `json:"storage_strategy,omitempty" binding:"omitempty,oneof=date flat cas" example:"date"`
	// AssessmentToken is the token returned by the create assessment (or by a
	// storage/confirmation-required Problem) for this exact target. It is
	// required whenever the target carries risks.
	AssessmentToken string `json:"assessment_token,omitempty" example:"v1.0123456789abcdef"`
	// AcknowledgedRisks lists every risk code the administrator acknowledged
	// against AssessmentToken. All risks the assessment reported must appear.
	AcknowledgedRisks []string `json:"acknowledged_risks,omitempty" example:"removable_storage,non_default_location"`
}

// StorageAssessmentRequestDTO selects a create target by registered Storage
// Location and direct-child folder. Clients never submit a filesystem path.
type StorageAssessmentRequestDTO struct {
	Intent string `json:"intent" binding:"required,oneof=create_repository setup_primary" example:"create_repository"`
	// StorageLocationID identifies a registered Storage Location. Empty selects
	// the configured default; setup_primary always uses the default.
	StorageLocationID string `json:"storage_location_id,omitempty" example:"550e8400-e29b-41d4-a716-446655440000"`
	// DirectoryName is the direct-child folder for create_repository; ignored for setup_primary.
	DirectoryName string `json:"directory_name,omitempty" example:"family-photos"`
}

// StorageAssessmentDTO is the Server's read-only review of one create
// target. The token binds a later acknowledgement to exactly these facts;
// capacity fluctuation does not change it, a changed mount, marker, or risk
// vocabulary does.
type StorageAssessmentDTO struct {
	Token               string `json:"token" example:"v1.0123456789abcdef"`
	Intent              string `json:"intent" enums:"create_repository,setup_primary" example:"create_repository"`
	StorageLocationID   string `json:"storage_location_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	StorageLocationKind string `json:"storage_location_kind" enums:"default,external" example:"external"`
	DirectoryName       string `json:"directory_name" example:"family-photos"`
	// Support is supported, unsupported, or unknown; Problem names the typed
	// refusal when Support is not supported or the target needs materialization.
	Support string `json:"support" enums:"supported,unsupported,unknown" example:"supported"`
	Problem string `json:"problem,omitempty" example:"storage/unsupported-filesystem"`
	// Risks are the acknowledgements the create command will require.
	Risks []string `json:"risks" example:"removable_storage,non_default_location"`
	// CanCreate reports whether the target is reviewable for creation: an
	// absent or empty target with no Repository marker on supported storage.
	CanCreate bool `json:"can_create"`
	// ExistingRepositoryID is the identity of a valid .lumiliorepo already at the target, if any.
	ExistingRepositoryID string `json:"existing_repository_id,omitempty" example:"550e8400-e29b-41d4-a716-446655440000"`
	// MarkerState is the typed reading of the target's Repository marker.
	MarkerState string `json:"marker_state" enums:"valid,absent,permission_denied,unknown,corrupt,unsupported_version" example:"absent"`
}

type RepositoryDTO struct {
	ID                string                  `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Name              string                  `json:"name" example:"Family Photos"`
	Path              string                  `json:"path" example:"/data/storage/Family Photos"`
	Role              string                  `json:"role" example:"regular"`
	IsPrimary         bool                    `json:"is_primary" example:"false"`
	StorageLocationID string                  `json:"storage_location_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Reachability      string                  `json:"reachability" example:"active"`
	Activity          string                  `json:"activity" example:"idle"`
	DefaultOwnerID    *int32                  `json:"default_owner_id,omitempty"`
	StorageStrategy   string                  `json:"storage_strategy" example:"date"`
	LocalSettings     RepositoryLocalSettings `json:"local_settings"`
}

type RepositoryLocalSettings struct {
	HandleDuplicateFilenames string `json:"handle_duplicate_filenames" example:"uuid"`
}

type RenameRepositoryRequestDTO struct {
	Name string `json:"name" binding:"required" example:"Family Archive"`
}

// LifecycleReceiptDTO is the durable operation a journaled storage command
// became. A retry that sends the same Idempotency-Key receives the same
// receipt with replayed set, never a second mutation.
type LifecycleReceiptDTO struct {
	OperationID string `json:"operation_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	RequestID   string `json:"request_id" example:"3f0c2d2e-8d6d-4e4e-9b3a-8d2c7b1d9f10"`
	Kind        string `json:"kind" example:"rename_repository" enums:"rename_repository,detach_repository"`
	Status      string `json:"status" example:"completed" enums:"completed"`
	Replayed    bool   `json:"replayed" example:"false"`
}

// RenameRepositoryResponseDTO is the renamed Repository with its receipt.
type RenameRepositoryResponseDTO struct {
	Repository RepositoryDTO       `json:"repository"`
	Operation  LifecycleReceiptDTO `json:"operation"`
}

// DetachRepositoryResponseDTO confirms a detach: the registration is gone,
// every file stays, and the receipt answers a retry.
type DetachRepositoryResponseDTO struct {
	Operation      LifecycleReceiptDTO `json:"operation"`
	FilesPreserved bool                `json:"files_preserved" example:"true"`
}

// RepositoryActionBlockDTO explains one withheld row action with stable codes.
type RepositoryActionBlockDTO struct {
	Action       string   `json:"action" example:"rename" enums:"verify,rename,detach,reconnect"`
	Blockers     []string `json:"blockers" example:"primary_protected"`
	Alternatives []string `json:"alternatives,omitempty" example:"default_configuration"`
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

// ToStorageAssessmentDTO projects a Server assessment onto the wire. The
// resolved path and raw observation stay on the Server.
func ToStorageAssessmentDTO(assessment *storage.CreateTargetAssessment) StorageAssessmentDTO {
	risks := storage.RiskCodes(assessment.Risks)
	if risks == nil {
		risks = []string{}
	}
	return StorageAssessmentDTO{
		Token:                assessment.Token,
		Intent:               string(assessment.Intent),
		StorageLocationID:    assessment.StorageLocationID.String(),
		StorageLocationKind:  string(assessment.StorageLocationKind),
		DirectoryName:        assessment.DirectoryName,
		Support:              string(assessment.Assessment.Classification.Support),
		Problem:              string(assessment.Problem()),
		Risks:                risks,
		CanCreate:            assessment.Assessment.Capabilities.ReviewCreate,
		ExistingRepositoryID: assessment.ExistingRepositoryID(),
		MarkerState:          string(assessment.Assessment.Observation.RepositoryMarker.State),
	}
}

// CreateRepositoryResponseDTO returns the created Repository and the risks
// the administrator acknowledged to place it there.
type CreateRepositoryResponseDTO struct {
	Repository RepositoryDTO `json:"repository"`
	// AcknowledgedRisks are the risk codes the administrator acknowledged for
	// this placement; empty when the target carried none.
	AcknowledgedRisks []string `json:"acknowledged_risks,omitempty" example:"removable_storage"`
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

type RepositoryScanQueuedDTO struct {
	OperationID  string `json:"operation_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	RepositoryID string `json:"repository_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	// Mode is the scan trigger: manual, periodic, watcher, startup, or settle.
	Mode      string `json:"mode" example:"manual" enums:"manual,periodic,watcher,startup,settle"`
	Status    string `json:"status" example:"queued"`
	Inserted  bool   `json:"inserted" example:"true"`
	Coalesced bool   `json:"coalesced" example:"false"`
}

// RepositoryScanRunDTO is one scan of the repository index. Counters are
// incremental: seen entries, new and changed entries, hashed files, entries
// marked missing, restored, removed as moves, deferred inside the settle
// window, and read errors.
type RepositoryScanRunDTO struct {
	OperationID  string `json:"operation_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	RepositoryID string `json:"repository_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	// Mode is the scan trigger: manual, periodic, watcher, startup, or settle.
	Mode        string  `json:"mode" example:"manual" enums:"manual,periodic,watcher,startup,settle"`
	RequestedBy *string `json:"requested_by,omitempty" example:"edwin"`
	// ScopePath is the scanned subtree; empty means the whole repository.
	ScopePath             string             `json:"scope_path" example:""`
	Status                string             `json:"status" example:"completed" enums:"queued,walking,sweeping,completed,offline,failed,cancelled"`
	CreatedAt             time.Time          `json:"created_at"`
	StartedAt             *time.Time         `json:"started_at,omitempty"`
	FinishedAt            *time.Time         `json:"finished_at,omitempty"`
	Seen                  int64              `json:"seen" example:"1200"`
	NewEntries            int64              `json:"new_entries" example:"12"`
	Changed               int64              `json:"changed" example:"3"`
	Hashed                int64              `json:"hashed" example:"15"`
	BytesHashed           int64              `json:"bytes_hashed" example:"524288"`
	Missing               int64              `json:"missing" example:"1"`
	Restored              int64              `json:"restored" example:"0"`
	Moved                 int64              `json:"moved" example:"2"`
	Deferred              int64              `json:"deferred" example:"0"`
	Errors                int64              `json:"errors" example:"0"`
	CancellationRequested bool               `json:"cancellation_requested" example:"false"`
	Problem               *problem.Reference `json:"problem,omitempty"`
}

type RepositoryScanRunListDTO struct {
	Scans []RepositoryScanRunDTO `json:"scans"`
}
