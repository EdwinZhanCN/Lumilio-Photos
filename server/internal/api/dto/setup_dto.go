package dto

import (
	"time"

	"server/internal/service"
	"server/internal/storage"
)

// SetupStatusDTO reports the durable owner and primary-repository gates. It
// is public, so it carries phase facts only; the administrator's review of
// the Primary Repository target is SetupPrimaryTargetDTO.
type SetupStatusDTO struct {
	Initialized                  bool `json:"initialized"`
	AdminInitialized             bool `json:"admin_initialized"`
	PrimaryRepositoryInitialized bool `json:"primary_repository_initialized"`
	// RuntimeState is independent of bootstrap completion. An initialized
	// instance can be degraded while its default Storage Location is offline.
	RuntimeState  string `json:"runtime_state" example:"active"`
	RuntimeReason string `json:"runtime_reason,omitempty" example:"storage_recovery_required"`
	// NextRegistrationRole is the role the next /auth/register will assign
	// ("admin" while no admin exists yet, "user" afterwards). Folds the former
	// /auth/bootstrap-status semantics into the unified setup status.
	NextRegistrationRole string `json:"next_registration_role"`
}

// ToSetupStatusDTO maps a service SetupStatus to its transport DTO.
func ToSetupStatusDTO(status service.SetupStatus) SetupStatusDTO {
	return SetupStatusDTO{
		Initialized:                  status.Initialized,
		AdminInitialized:             status.AdminInitialized,
		PrimaryRepositoryInitialized: status.PrimaryRepositoryInitialized,
		RuntimeState:                 status.RuntimeState,
		RuntimeReason:                status.RuntimeReason,
		NextRegistrationRole:         status.NextRegistrationRole,
	}
}

// SetupDefaultStorageDTO is what the configured Default Storage Location
// looks like to this catalog: registered or not, and the state of its root
// marker. Path is the configured storage.path (administrator only).
type SetupDefaultStorageDTO struct {
	Path       string `json:"path" example:"/data/storage"`
	Registered bool   `json:"registered"`
	// State names the one condition that decides what setup can do here.
	State       string `json:"state" example:"ready" enums:"ready,offline,path_changed,marker_missing,marker_invalid,marker_unsupported,identity_mismatch,repository_marker,unregistered,unavailable"`
	MarkerState string `json:"marker_state,omitempty" example:"valid" enums:"valid,absent,permission_denied,unknown,corrupt,unsupported_version"`
	Detail      string `json:"detail,omitempty" example:"parse storage location marker: yaml: line 1"`
}

// SetupPrimaryExistingDTO describes the Repository marker already at the
// primary target. No path is exposed.
type SetupPrimaryExistingDTO struct {
	RepositoryID  string     `json:"repository_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Name          string     `json:"name,omitempty" example:"Primary Storage"`
	MarkerVersion string     `json:"marker_version,omitempty" example:"1.0"`
	CreatedAt     *time.Time `json:"created_at,omitempty"`
	// RegisteredRole is set when the identity is already registered in this
	// catalog; setup cannot adopt it then.
	RegisteredRole string `json:"registered_role,omitempty" example:"regular" enums:"primary,regular"`
}

// SetupPrimaryRecoveryDTO counts the private state an adoption preserves
// by class: Trash under a hold, Studio records awaiting proof, staging
// kept for export, unknown files kept untouched.
type SetupPrimaryRecoveryDTO struct {
	TrashFiles    int   `json:"trash_files" example:"2"`
	TrashBytes    int64 `json:"trash_bytes" example:"4096"`
	StudioRecords int   `json:"studio_records" example:"1"`
	StagingFiles  int   `json:"staging_files" example:"2"`
	UnknownFiles  int   `json:"unknown_files" example:"1"`
	LogFiles      int   `json:"log_files" example:"2"`
	DerivedFiles  int   `json:"derived_files" example:"12"`
	Receipts      int   `json:"receipts" example:"0"`
}

// SetupPrimaryReviewDTO is the Server's review of <default>/primary: the
// state, the actions setup may take, the shared assessment with its token
// and risks, and what an adoption would find.
type SetupPrimaryReviewDTO struct {
	State string `json:"state" example:"adoptable" enums:"creatable,adoptable,layout_repairable,registered,nonempty_unmarked,marker_invalid,marker_unsupported,unavailable"`
	// Actions are the legal transitions; fresh_elsewhere is always listed
	// before the Primary Repository exists.
	Actions    []string             `json:"actions" enums:"create,adopt,fresh_elsewhere"`
	Assessment StorageAssessmentDTO `json:"assessment"`
	// LayoutMissing lists the layout directories an adoption would add.
	LayoutMissing []string                 `json:"layout_missing,omitempty" example:".lumilio"`
	Existing      *SetupPrimaryExistingDTO `json:"existing,omitempty"`
	Recovery      *SetupPrimaryRecoveryDTO `json:"recovery,omitempty"`
}

// SetupPrimaryTargetDTO is the authenticated administrator's setup review.
// Target is absent while the Default Storage Location is not usable (its
// state says why) and once setup is complete.
type SetupPrimaryTargetDTO struct {
	SetupComplete bool `json:"setup_complete"`
	// NativeHost reports a Desktop host: a fresh Default Storage Location is
	// chosen in Desktop Settings and applied; without it the operator edits
	// storage.path in the manifest and restarts.
	NativeHost         bool                   `json:"native_host"`
	RepositoryDefaults RepositoryDefaultsDTO  `json:"repository_defaults"`
	DefaultStorage     SetupDefaultStorageDTO `json:"default_storage"`
	Target             *SetupPrimaryReviewDTO `json:"target,omitempty"`
}

// ToSetupPrimaryTargetDTO projects the service review onto the wire.
func ToSetupPrimaryTargetDTO(target service.SetupPrimaryTarget, nativeHost bool) SetupPrimaryTargetDTO {
	result := SetupPrimaryTargetDTO{
		SetupComplete: target.SetupComplete,
		NativeHost:    nativeHost,
		RepositoryDefaults: RepositoryDefaultsDTO{
			StorageLocation:   target.RepositoryDefaults.StorageLocation,
			Strategy:          target.RepositoryDefaults.Strategy,
			DuplicateHandling: target.RepositoryDefaults.DuplicateHandling,
		},
		DefaultStorage: SetupDefaultStorageDTO{
			Path:        target.DefaultStorage.ConfiguredPath,
			Registered:  target.DefaultStorage.Registered,
			State:       string(target.DefaultStorage.State),
			MarkerState: string(target.DefaultStorage.MarkerState),
			Detail:      target.DefaultStorage.Detail,
		},
	}
	if target.Target != nil {
		review := ToSetupPrimaryReviewDTO(target.Target)
		result.Target = &review
	}
	return result
}

// ToSetupPrimaryReviewDTO projects one setup review. Paths stay on the Server.
func ToSetupPrimaryReviewDTO(review *storage.SetupPrimaryAssessment) SetupPrimaryReviewDTO {
	actions := append([]string(nil), review.Actions...)
	if actions == nil {
		actions = []string{}
	}
	result := SetupPrimaryReviewDTO{
		State:         string(review.State),
		Actions:       actions,
		Assessment:    ToStorageAssessmentDTO(review.CreateTargetAssessment),
		LayoutMissing: append([]string(nil), review.LayoutMissing...),
	}
	if review.Existing != nil {
		existing := &SetupPrimaryExistingDTO{
			RepositoryID: review.Existing.RepositoryID, Name: review.Existing.Name,
			MarkerVersion: review.Existing.MarkerVersion, RegisteredRole: string(review.Existing.RegisteredRole),
		}
		if !review.Existing.CreatedAt.IsZero() {
			createdAt := review.Existing.CreatedAt
			existing.CreatedAt = &createdAt
		}
		result.Existing = existing
	}
	if review.Recovery != nil {
		result.Recovery = toSetupPrimaryRecoveryDTO(*review.Recovery)
	}
	return result
}

func toSetupPrimaryRecoveryDTO(preview storage.SetupPrimaryRecoveryPreview) *SetupPrimaryRecoveryDTO {
	return &SetupPrimaryRecoveryDTO{
		TrashFiles: preview.TrashFiles, TrashBytes: preview.TrashBytes, StudioRecords: preview.StudioRecords,
		StagingFiles: preview.StagingFiles, UnknownFiles: preview.UnknownFiles, LogFiles: preview.LogFiles,
		DerivedFiles: preview.DerivedFiles, Receipts: preview.Receipts,
	}
}

// AdoptPrimaryRepositoryRequestDTO is the explicit adoption of the
// Repository already at the primary target. Every field binds the command
// to the review the administrator saw: the identity, the token, and the
// risks acknowledged for that placement.
type AdoptPrimaryRepositoryRequestDTO struct {
	ExpectedRepositoryID string   `json:"expected_repository_id" binding:"required" example:"550e8400-e29b-41d4-a716-446655440000"`
	AssessmentToken      string   `json:"assessment_token" binding:"required" example:"v1.0123456789abcdef"`
	AcknowledgedRisks    []string `json:"acknowledged_risks,omitempty" example:"removable_storage"`
}

// AdoptPrimaryRepositoryResponseDTO returns the adopted Primary Repository,
// the acknowledged risks, and what the adoption preserved. The recovery
// inventory on the Storage page carries every item afterwards.
type AdoptPrimaryRepositoryResponseDTO struct {
	Repository        RepositoryDTO `json:"repository"`
	AcknowledgedRisks []string      `json:"acknowledged_risks"`
	// RecoveryReceiptID names the adopt receipt; empty when the tree carried
	// no private state.
	RecoveryReceiptID string                   `json:"recovery_receipt_id,omitempty" example:"550e8400-e29b-41d4-a716-446655440000"`
	Recovery          *SetupPrimaryRecoveryDTO `json:"recovery,omitempty"`
	LayoutRepaired    []string                 `json:"layout_repaired,omitempty" example:".lumilio"`
	Replayed          bool                     `json:"replayed"`
}

// ToAdoptPrimaryRepositoryResponseDTO projects the adoption result.
func ToAdoptPrimaryRepositoryResponseDTO(result *storage.AdoptPrimaryResult, repository RepositoryDTO) AdoptPrimaryRepositoryResponseDTO {
	risks := storage.RiskCodes(result.AcknowledgedRisks)
	if risks == nil {
		risks = []string{}
	}
	response := AdoptPrimaryRepositoryResponseDTO{
		Repository: repository, AcknowledgedRisks: risks, RecoveryReceiptID: result.ReceiptID,
		LayoutRepaired: append([]string(nil), result.LayoutRepaired...), Replayed: result.Replayed,
	}
	if result.Receipt != nil {
		preview := storage.SetupPrimaryRecoveryPreview{
			TrashFiles: result.Receipt.Trash.Files, TrashBytes: result.Receipt.Trash.Bytes,
			LogFiles: result.Receipt.Logs.Files, DerivedFiles: result.Receipt.Derived.Files,
		}
		for _, item := range result.Receipt.Items {
			switch item.Class {
			case storage.PrivateClassStudio:
				preview.StudioRecords++
			case storage.PrivateClassStaging:
				preview.StagingFiles++
			default:
				preview.UnknownFiles++
			}
		}
		preview.UnknownFiles += result.Receipt.ItemsTruncated
		response.Recovery = toSetupPrimaryRecoveryDTO(preview)
	}
	return response
}
