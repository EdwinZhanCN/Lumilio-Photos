package storage

import (
	"errors"
	"fmt"

	"server/internal/db/dbtypes"
	"server/internal/db/repo"
)

var (
	// ErrRepositoryIdentityError reports that the repository marker identity is invalid.
	ErrRepositoryIdentityError = errors.New("repository identity is invalid")
	// ErrRepositoryRecoveryRequired reports that the repository needs explicit recovery.
	ErrRepositoryRecoveryRequired = errors.New("repository recovery is required")
	// ErrRepositoryUploadPaused reports that manual pause blocks upload and import.
	ErrRepositoryUploadPaused = errors.New("repository upload is paused")
	// ErrRepositoryUploadLowSpace reports that low-space policy blocks upload at the catalog boundary.
	ErrRepositoryUploadLowSpace = errors.New("repository upload is blocked by low space policy")
)

// Closed admission reason codes for GET /api/v1/storage/targets and I/O
// rechecks. Display summaries may pick one dominant reason; admission retains
// every simultaneous condition.
const (
	AdmissionOffline          = "offline"
	AdmissionIdentityError    = "identity_error"
	AdmissionReadOnly         = "read_only"
	AdmissionLowSpace         = "low_space"
	AdmissionPaused           = "paused"
	AdmissionBusy             = "busy"
	AdmissionRecoveryRequired = "recovery_required"
)

// AdmissionDecision is the per-operation eligibility projection. An empty
// Reasons slice means the operation is unrestricted.
type AdmissionDecision struct {
	Allowed bool
	Reasons []string
}

// WriteFacts are optional live I/O observations. Foreground list/status reads
// leave them zero so unknown writability and space never become invented
// refusals. Upload/cloud materialization rechecks them at the I/O boundary.
type WriteFacts struct {
	Known      bool
	Writable   bool
	SpaceKnown bool
	LowSpace   bool
}

// OpenOriginalAdmission admits opening an original from catalog reachability.
// Location status, capacity, and manual write pause do not participate.
func OpenOriginalAdmission(repository repo.Repository) AdmissionDecision {
	return admissionFromReasons(openOriginalReasons(repository))
}

// UploadAdmission admits upload and cloud materialization. Verification
// activity does not refuse writes. Manual pause and space policy do.
func UploadAdmission(repository repo.Repository, facts WriteFacts) AdmissionDecision {
	reasons := openOriginalReasons(repository)
	if repository.Reachability == dbtypes.RepositoryReachabilityActive &&
		repository.Activity == dbtypes.RepositoryActivityPaused {
		switch repository.PauseReason {
		case "manual":
			reasons = append(reasons, AdmissionPaused)
		case "low_space":
			reasons = append(reasons, AdmissionLowSpace)
		case "maintenance":
			reasons = append(reasons, AdmissionBusy)
		default:
			reasons = append(reasons, AdmissionPaused)
		}
	}
	if facts.Known && !facts.Writable {
		reasons = appendReason(reasons, AdmissionReadOnly)
	}
	if facts.SpaceKnown && facts.LowSpace {
		reasons = appendReason(reasons, AdmissionLowSpace)
	}
	return admissionFromReasons(reasons)
}

func openOriginalReasons(repository repo.Repository) []string {
	switch repository.Reachability {
	case dbtypes.RepositoryReachabilityActive:
		return nil
	case dbtypes.RepositoryReachabilityOffline:
		return []string{AdmissionOffline}
	case dbtypes.RepositoryReachabilityIdentityError:
		return []string{AdmissionIdentityError}
	case dbtypes.RepositoryReachabilityRecoveryRequired:
		return []string{AdmissionRecoveryRequired}
	case dbtypes.RepositoryReachabilityMaintenance:
		return []string{AdmissionBusy}
	default:
		return []string{AdmissionOffline}
	}
}

func admissionFromReasons(reasons []string) AdmissionDecision {
	if len(reasons) == 0 {
		return AdmissionDecision{Allowed: true}
	}
	return AdmissionDecision{Allowed: false, Reasons: reasons}
}

func appendReason(reasons []string, reason string) []string {
	for _, existing := range reasons {
		if existing == reason {
			return reasons
		}
	}
	return append(reasons, reason)
}

// WriteFactsFromCapacity maps a live capacity inspection into upload admission
// facts. Callers must not treat unknown capacity as low space. A zero decision
// (lookup/parse failure) must not invent a read-only refusal. Read-only is not
// low space even when the volume size is known.
func WriteFactsFromCapacity(decision CapacityDecision) WriteFacts {
	if decision.RepositoryID == "" && decision.RepositoryPath == "" {
		return WriteFacts{}
	}
	return WriteFacts{
		Known:      true,
		Writable:   decision.Writable,
		SpaceKnown: decision.CapacityKnown,
		LowSpace:   decision.CapacityKnown && decision.Writable && !decision.Allowed,
	}
}

// CheckUploadAdmission evaluates upload/cloud materialization admission and
// returns a stable sentinel error for HTTP and service boundaries.
func CheckUploadAdmission(repository repo.Repository, facts WriteFacts) error {
	decision := UploadAdmission(repository, facts)
	if decision.Allowed {
		return nil
	}
	name := repository.Name
	for _, reason := range decision.Reasons {
		switch reason {
		case AdmissionOffline:
			return fmt.Errorf("%w: %s", ErrRepositoryOffline, name)
		case AdmissionIdentityError:
			return fmt.Errorf("%w: %s", ErrRepositoryIdentityError, name)
		case AdmissionRecoveryRequired:
			return fmt.Errorf("%w: %s", ErrRepositoryRecoveryRequired, name)
		case AdmissionBusy:
			return fmt.Errorf("%w: %s", ErrRepositoryBusy, name)
		case AdmissionPaused:
			return fmt.Errorf("%w: %s", ErrRepositoryUploadPaused, name)
		case AdmissionLowSpace:
			return fmt.Errorf("%w: %s", ErrRepositoryUploadLowSpace, name)
		case AdmissionReadOnly:
			return fmt.Errorf("%w: %s", ErrRepositoryReadOnly, name)
		}
	}
	return fmt.Errorf("%w: %s", ErrRepositoryOffline, name)
}
