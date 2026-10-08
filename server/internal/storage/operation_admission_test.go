package storage

import (
	"reflect"
	"testing"

	"server/internal/db/dbtypes"
	"server/internal/db/repo"
)

func TestOpenOriginalAdmissionIgnoresPauseAndWritePolicy(t *testing.T) {
	paused := repo.Repository{
		Reachability: dbtypes.RepositoryReachabilityActive,
		Activity:     dbtypes.RepositoryActivityPaused,
		PauseReason:  "manual",
	}
	got := OpenOriginalAdmission(paused)
	if !got.Allowed || len(got.Reasons) != 0 {
		t.Fatalf("open original while paused = %+v, want allowed", got)
	}

	lowSpace := paused
	lowSpace.PauseReason = "low_space"
	got = OpenOriginalAdmission(lowSpace)
	if !got.Allowed || len(got.Reasons) != 0 {
		t.Fatalf("open original while low_space = %+v, want allowed", got)
	}

	scanning := repo.Repository{
		Reachability: dbtypes.RepositoryReachabilityActive,
		Activity:     dbtypes.RepositoryActivityScanning,
	}
	got = OpenOriginalAdmission(scanning)
	if !got.Allowed {
		t.Fatalf("open original while scanning = %+v, want allowed", got)
	}
}

func TestOpenOriginalAdmissionUsesOwnReachability(t *testing.T) {
	cases := []struct {
		reachability dbtypes.RepositoryReachability
		want         []string
	}{
		{dbtypes.RepositoryReachabilityActive, nil},
		{dbtypes.RepositoryReachabilityOffline, []string{AdmissionOffline}},
		{dbtypes.RepositoryReachabilityIdentityError, []string{AdmissionIdentityError}},
		{dbtypes.RepositoryReachabilityRecoveryRequired, []string{AdmissionRecoveryRequired}},
		{dbtypes.RepositoryReachabilityMaintenance, []string{AdmissionBusy}},
	}
	for _, test := range cases {
		got := OpenOriginalAdmission(repo.Repository{Reachability: test.reachability})
		if test.want == nil {
			if !got.Allowed || len(got.Reasons) != 0 {
				t.Fatalf("reachability %s = %+v, want allowed", test.reachability, got)
			}
			continue
		}
		if got.Allowed || !reflect.DeepEqual(got.Reasons, test.want) {
			t.Fatalf("reachability %s = %+v, want reasons %v", test.reachability, got, test.want)
		}
	}
}

func TestUploadAdmissionKeepsPauseAndSpaceDistinctFromVerification(t *testing.T) {
	scanning := repo.Repository{
		Reachability: dbtypes.RepositoryReachabilityActive,
		Activity:     dbtypes.RepositoryActivityScanning,
	}
	got := UploadAdmission(scanning, WriteFacts{})
	if !got.Allowed {
		t.Fatalf("upload during verification = %+v, want allowed", got)
	}

	manual := repo.Repository{
		Reachability: dbtypes.RepositoryReachabilityActive,
		Activity:     dbtypes.RepositoryActivityPaused,
		PauseReason:  "manual",
	}
	got = UploadAdmission(manual, WriteFacts{})
	if got.Allowed || !reflect.DeepEqual(got.Reasons, []string{AdmissionPaused}) {
		t.Fatalf("manual pause upload = %+v", got)
	}

	lowSpace := manual
	lowSpace.PauseReason = "low_space"
	got = UploadAdmission(lowSpace, WriteFacts{})
	if got.Allowed || !reflect.DeepEqual(got.Reasons, []string{AdmissionLowSpace}) {
		t.Fatalf("low_space pause upload = %+v", got)
	}

	offline := repo.Repository{Reachability: dbtypes.RepositoryReachabilityOffline}
	got = UploadAdmission(offline, WriteFacts{Known: true, Writable: false, SpaceKnown: true, LowSpace: true})
	if got.Allowed || !reflect.DeepEqual(got.Reasons, []string{AdmissionOffline, AdmissionReadOnly, AdmissionLowSpace}) {
		t.Fatalf("offline unwritable low-space upload = %+v", got)
	}
}

func TestUploadAdmissionUnknownWriteFactsDoNotInventReadOnly(t *testing.T) {
	active := repo.Repository{
		Reachability: dbtypes.RepositoryReachabilityActive,
		Activity:     dbtypes.RepositoryActivityIdle,
	}
	got := UploadAdmission(active, WriteFacts{})
	if !got.Allowed || len(got.Reasons) != 0 {
		t.Fatalf("catalog-only upload admission = %+v, want unrestricted", got)
	}
}

func TestWriteFactsFromCapacityDistinguishesReadOnlyFromLowSpace(t *testing.T) {
	empty := WriteFactsFromCapacity(CapacityDecision{})
	if empty != (WriteFacts{}) {
		t.Fatalf("zero capacity decision = %+v, want empty write facts", empty)
	}

	readOnly := WriteFactsFromCapacity(CapacityDecision{
		Allowed: false, CapacityKnown: true, Writable: false,
		RepositoryID: "repo", RepositoryPath: "/tmp/repo",
	})
	if !readOnly.Known || readOnly.Writable || !readOnly.SpaceKnown || readOnly.LowSpace {
		t.Fatalf("read-only facts = %+v", readOnly)
	}

	lowSpace := WriteFactsFromCapacity(CapacityDecision{
		Allowed: false, CapacityKnown: true, Writable: true,
		RepositoryID: "repo", RepositoryPath: "/tmp/repo",
	})
	if !lowSpace.Known || !lowSpace.Writable || !lowSpace.SpaceKnown || !lowSpace.LowSpace {
		t.Fatalf("low-space facts = %+v", lowSpace)
	}

	unknown := WriteFactsFromCapacity(CapacityDecision{
		Allowed: true, CapacityKnown: false, Writable: true,
		RepositoryID: "repo", RepositoryPath: "/tmp/repo",
	})
	if !unknown.Known || !unknown.Writable || unknown.SpaceKnown || unknown.LowSpace {
		t.Fatalf("unknown-space facts = %+v", unknown)
	}
}
