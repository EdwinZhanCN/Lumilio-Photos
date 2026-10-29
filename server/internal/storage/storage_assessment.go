package storage

import (
	"context"
	"path/filepath"
	"strings"

	"server/internal/storage/marker"
)

type StorageProblemCode string

const (
	ProblemUnsupportedFilesystem   StorageProblemCode = "storage/unsupported-filesystem"
	ProblemNotMountedVolume        StorageProblemCode = "storage/not-a-mounted-volume"
	ProblemObservationUnknown      StorageProblemCode = "storage/observation-unknown"
	ProblemPermissionDenied        StorageProblemCode = "storage/permission-denied"
	ProblemMaterializationRequired StorageProblemCode = "storage/materialization-required"
)
const NotMountedVolumeMessage = "This folder isn't a mounted volume; data stored here would be lost when the container is recreated. Mount a host directory in your compose file."

type StorageSupport string

const (
	StorageSupported      StorageSupport = "supported"
	StorageUnsupported    StorageSupport = "unsupported"
	StorageSupportUnknown StorageSupport = "unknown"
)

type StorageRisk string

const (
	RiskRemovable  StorageRisk = "removable_storage"
	RiskNonDefault StorageRisk = "non_default_location"
	RiskCloudSync  StorageRisk = "cloud_sync_directory"
)

type StorageClassification struct {
	Support StorageSupport
	Problem StorageProblemCode
	Risks   []StorageRisk
}
type StoragePolicyInput struct{ ExistingEmptyTarget, NonDefault bool }

// ClassifyStorage is the common platform-independent policy. Platform adapters
// report facts only. Legacy mutation admission is cut over in later phases.
func ClassifyStorage(observed StorageObservation, input StoragePolicyInput) StorageClassification {
	result := StorageClassification{Support: StorageSupported}
	if observed.Access.State == AccessPermissionDenied {
		result.Support = StorageSupportUnknown
		result.Problem = ProblemPermissionDenied
		return result
	}
	if observed.FactsErr != nil || (observed.Access.State != AccessPresent && observed.Access.State != AccessAbsent) || observed.DirectoryErr != nil || observed.PlaceholderErr != nil {
		result.Support = StorageSupportUnknown
		result.Problem = ProblemObservationUnknown
		if isPermissionError(observed.FactsErr, observed.DirectoryErr, observed.PlaceholderErr) {
			result.Problem = ProblemPermissionDenied
		}
		return result
	}
	if networkStorageFilesystem(observed.Volume.Filesystem) || observed.Mount.Remote {
		result.Support = StorageUnsupported
		result.Problem = ProblemUnsupportedFilesystem
		return result
	}
	if strings.TrimSpace(observed.Volume.Filesystem) == "" {
		result.Support = StorageSupportUnknown
		result.Problem = ProblemObservationUnknown
		return result
	}
	if input.ExistingEmptyTarget && observed.Mount.Platform == "linux" && filepath.Clean(observed.SamplePath) != filepath.Clean(observed.Mount.MountPath) {
		result.Support = StorageUnsupported
		result.Problem = ProblemNotMountedVolume
		return result
	}
	if observed.Mount.Removable {
		result.Risks = append(result.Risks, RiskRemovable)
	}
	if input.NonDefault {
		result.Risks = append(result.Risks, RiskNonDefault)
	}
	if observed.Mount.CloudProvider != "" {
		result.Risks = append(result.Risks, RiskCloudSync)
	}
	if observed.Placeholder {
		result.Problem = ProblemMaterializationRequired
	}
	return result
}

func networkStorageFilesystem(filesystem string) bool {
	value := strings.ToLower(strings.TrimSpace(filesystem))
	for _, prefix := range []string{"nfs", "cifs", "smb", "afp", "sshfs", "fuse.sshfs", "9p", "webdav", "davfs", "fuse.davfs", "ceph", "afs"} {
		if value == prefix || strings.HasPrefix(value, prefix+".") || strings.HasPrefix(value, prefix+"fs") || (prefix == "nfs" && strings.HasPrefix(value, "nfs")) {
			return true
		}
	}
	return false
}

type StorageCapabilities struct {
	Read, Verify bool
	// These are review/preflight possibilities, never unconditional mutation permits.
	ReviewCreate, ReviewOpen bool
	MarkerWritePreflight     bool
}

// DeriveStorageCapabilities does not consult a parent marker. A registered
// child is usable on its own identity/access facts even when its parent fails.
func DeriveStorageCapabilities(observed StorageObservation, classification StorageClassification, expectedRepositoryID string) StorageCapabilities {
	var result StorageCapabilities
	valid := observed.RepositoryMarker.State == marker.Valid
	identity := expectedRepositoryID == "" || observed.RepositoryMarker.UUID == expectedRepositoryID
	result.Read = observed.Access.State == AccessPresent && valid && identity && observed.FactsErr == nil && observed.DirectoryErr == nil && observed.PlaceholderErr == nil && !observed.Placeholder
	result.Verify = result.Read
	if classification.Support != StorageSupported || classification.Problem != "" {
		return result
	}
	result.ReviewOpen = result.Read && expectedRepositoryID == ""
	result.ReviewCreate = expectedRepositoryID == "" && observed.RepositoryMarker.State == marker.Absent && (observed.Access.State == AccessAbsent || observed.Empty)
	result.MarkerWritePreflight = valid && identity || result.ReviewCreate
	return result
}

type StorageAssessment struct {
	Observation    StorageObservation
	Classification StorageClassification
	Capabilities   StorageCapabilities
}

func AssessStorageTarget(ctx context.Context, observer StorageObserver, path string, nonDefault bool, expectedRepositoryID string) StorageAssessment {
	observed := ObserveStorageTarget(ctx, observer, path)
	classification := ClassifyStorage(observed, StoragePolicyInput{ExistingEmptyTarget: observed.Access.State == AccessPresent && observed.Empty, NonDefault: nonDefault})
	return StorageAssessment{Observation: observed, Classification: classification, Capabilities: DeriveStorageCapabilities(observed, classification, expectedRepositoryID)}
}

// MountContinuityChanged compares the subject with its own prior mount facts;
// parent identity and capacity-pool grouping never participate.
func MountContinuityChanged(previous, current MountFacts) bool {
	return previous.MountID != current.MountID || previous.MountSource != current.MountSource || previous.Device != current.Device
}
