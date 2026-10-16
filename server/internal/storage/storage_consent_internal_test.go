package storage

import (
	"testing"

	"server/internal/db/dbtypes"
	"server/internal/storage/marker"

	"github.com/google/uuid"
)

func consentFixture() *CreateTargetAssessment {
	a := &CreateTargetAssessment{
		Intent: IntentCreateRepository, StorageLocationID: uuid.MustParse("6b767057-d6f1-465c-b816-d1229f622a20"),
		StorageLocationKind: dbtypes.StorageLocationKindExternal, DirectoryName: "archive", Path: "/Volumes/Backup/archive",
		Risks: []StorageRisk{RiskRemovable, RiskNonDefault},
	}
	a.Assessment.Classification = StorageClassification{Support: StorageSupported, Risks: a.Risks}
	a.Assessment.Observation.Access = AccessReading{State: AccessAbsent}
	a.Assessment.Observation.Volume = VolumeFacts{Filesystem: "apfs", TotalBytes: 1000, AvailableBytes: 500, CapacityGroupKey: "pool-a"}
	a.Assessment.Observation.Mount = MountFacts{Platform: "darwin", MountID: "disk2s1", MountSource: "/dev/disk2s1", MountPath: "/Volumes/Backup", Device: "disk2s1", Inode: 7, Removable: true}
	a.Assessment.Observation.RootMarker.State = marker.Valid
	a.Assessment.Observation.RootMarker.UUID = "6b767057-d6f1-465c-b816-d1229f622a20"
	a.Assessment.Observation.RepositoryMarker.State = marker.Absent
	return a
}

// The fingerprint is the consent contract: capacity and volatile grouping
// facts never change it, while identity, intent, risks, mount, and marker
// facts always do.
func TestCreateTargetFingerprintIgnoresCapacityButBindsEveryDecidingFact(t *testing.T) {
	base := createTargetFingerprint(consentFixture())

	for name, mutate := range map[string]func(*CreateTargetAssessment){
		"available bytes": func(a *CreateTargetAssessment) { a.Assessment.Observation.Volume.AvailableBytes = 1 },
		"total bytes":     func(a *CreateTargetAssessment) { a.Assessment.Observation.Volume.TotalBytes = 999999 },
		"capacity group":  func(a *CreateTargetAssessment) { a.Assessment.Observation.Volume.CapacityGroupKey = "pool-b" },
		"inode":           func(a *CreateTargetAssessment) { a.Assessment.Observation.Mount.Inode = 99 },
	} {
		a := consentFixture()
		mutate(a)
		if createTargetFingerprint(a) != base {
			t.Fatalf("%s changed the consent fingerprint", name)
		}
	}

	for name, mutate := range map[string]func(*CreateTargetAssessment){
		"intent":           func(a *CreateTargetAssessment) { a.Intent = IntentSetupPrimary },
		"storage location": func(a *CreateTargetAssessment) { a.StorageLocationID = uuid.New() },
		"directory name":   func(a *CreateTargetAssessment) { a.DirectoryName = "other" },
		"risk set":         func(a *CreateTargetAssessment) { a.Risks = []StorageRisk{RiskNonDefault} },
		"support":          func(a *CreateTargetAssessment) { a.Assessment.Classification.Support = StorageUnsupported },
		"problem":          func(a *CreateTargetAssessment) { a.Assessment.Classification.Problem = ProblemUnsupportedFilesystem },
		"mount device":     func(a *CreateTargetAssessment) { a.Assessment.Observation.Mount.Device = "disk3s1" },
		"mount source":     func(a *CreateTargetAssessment) { a.Assessment.Observation.Mount.MountSource = "/dev/disk3s1" },
		"removable fact":   func(a *CreateTargetAssessment) { a.Assessment.Observation.Mount.Removable = false },
		"cloud provider":   func(a *CreateTargetAssessment) { a.Assessment.Observation.Mount.CloudProvider = "icloud" },
		"filesystem":       func(a *CreateTargetAssessment) { a.Assessment.Observation.Volume.Filesystem = "exfat" },
		"target present":   func(a *CreateTargetAssessment) { a.Assessment.Observation.Access.State = AccessPresent },
		"root marker":      func(a *CreateTargetAssessment) { a.Assessment.Observation.RootMarker.State = marker.Corrupt },
		"repository marker": func(a *CreateTargetAssessment) {
			a.Assessment.Observation.RepositoryMarker.State = marker.Valid
			a.Assessment.Observation.RepositoryMarker.UUID = uuid.NewString()
		},
	} {
		a := consentFixture()
		mutate(a)
		if createTargetFingerprint(a) == base {
			t.Fatalf("%s did not change the consent fingerprint", name)
		}
	}

	// Risk order is irrelevant; the vocabulary is what binds.
	reordered := consentFixture()
	reordered.Risks = []StorageRisk{RiskNonDefault, RiskRemovable}
	if createTargetFingerprint(reordered) != base {
		t.Fatal("risk order changed the consent fingerprint")
	}
}

func TestConsentTokenIsOpaqueAndStableWithinOneManager(t *testing.T) {
	first := &DefaultRepositoryManager{}
	second := &DefaultRepositoryManager{}
	fingerprint := createTargetFingerprint(consentFixture())
	token := first.consentToken(fingerprint)
	if token != first.consentToken(fingerprint) {
		t.Fatal("token is not deterministic for one manager")
	}
	if token == second.consentToken(fingerprint) {
		t.Fatal("token leaked across managers: it must be keyed per process")
	}
	if token == fingerprint || len(token) < len(consentTokenVersion)+64 {
		t.Fatalf("token %q is not an opaque keyed digest", token)
	}
	if !acknowledgesRisks([]string{"non_default_location", " removable_storage ", "extra"}, []StorageRisk{RiskRemovable, RiskNonDefault}) {
		t.Fatal("exact codes with whitespace and extras must acknowledge")
	}
	if acknowledgesRisks([]string{"removable_storage"}, []StorageRisk{RiskRemovable, RiskNonDefault}) {
		t.Fatal("a missing required risk must not acknowledge")
	}
}
