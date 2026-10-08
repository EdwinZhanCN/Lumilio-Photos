package storage_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"server/internal/storage"
	"server/internal/storage/marker"
	"server/internal/storage/repocfg"
	"server/internal/storage/rootcfg"
	"server/internal/storage/testfixture"
)

func factsObserver() *testfixture.Observer {
	return &testfixture.Observer{
		StatFSFunc: func(string) (storage.VolumeFacts, error) {
			return storage.VolumeFacts{Filesystem: "ext4", CapacityGroupKey: "pool", TotalBytes: 1000, AvailableBytes: 500}, nil
		},
		MountInfoFunc: func(string) (storage.MountFacts, error) {
			return storage.MountFacts{Platform: "linux", MountID: "B", Device: "device-B", MountPath: "/"}, nil
		},
		PlaceholderFunc: func(string) (bool, error) { return false, nil },
	}
}

func TestAssessmentAccessErrorsNeverBecomeAbsence(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want storage.AccessState
	}{
		{"denied", fs.ErrPermission, storage.AccessPermissionDenied}, {"unknown", errors.New("device failure"), storage.AccessUnknown}, {"nil-info", nil, storage.AccessUnknown}, {"absent", fs.ErrNotExist, storage.AccessAbsent},
	} {
		t.Run(tc.name, func(t *testing.T) {
			observer := factsObserver()
			observer.StatFunc = func(string) (fs.FileInfo, error) { return nil, tc.err }
			result := storage.AssessStorageTarget(context.Background(), observer, "/target", false, "registered-id")
			if result.Observation.Access.State != tc.want {
				t.Fatalf("got %+v", result)
			}
			if result.Capabilities.Read || result.Capabilities.MarkerWritePreflight || result.Capabilities.ReviewCreate {
				t.Fatalf("unsafe capabilities: %+v", result.Capabilities)
			}
		})
	}
}

func TestTypedMarkerReadings(t *testing.T) {
	for _, tc := range []struct {
		fixture testfixture.Marker
		want    marker.State
	}{{testfixture.Valid, marker.Valid}, {testfixture.Missing, marker.Absent}, {testfixture.Corrupt, marker.Corrupt}, {testfixture.Unsupported, marker.UnsupportedVersion}} {
		location := testfixture.NewLocation(t, tc.fixture)
		repository := location.Repository(t, "child", tc.fixture, testfixture.MissingPrivate)
		observer := factsObserver()
		root := rootcfg.ReadMarker(observer, location.Path)
		child := repocfg.ReadMarker(observer, repository.Path)
		if root.State != tc.want || child.State != tc.want {
			t.Fatalf("marker readings: %+v %+v", root, child)
		}
		if tc.want == marker.Valid && (root.Version != "1.0" || child.Version != "1.0" || child.UUID != repository.Config.ID) {
			t.Fatal("compatible identity lost")
		}
	}
	for _, err := range []error{fs.ErrPermission, errors.New("read failure")} {
		observer := factsObserver()
		observer.ReadFileFunc = func(string) ([]byte, error) { return nil, err }
		want := marker.Unknown
		if errors.Is(err, fs.ErrPermission) {
			want = marker.PermissionDenied
		}
		if rootcfg.ReadMarker(observer, "/root").State != want || repocfg.ReadMarker(observer, "/repo").State != want {
			t.Fatal("read error masked")
		}
	}
}

// canonicalPath mirrors the observer's symlink resolution so injected facts
// keyed by path also match on macOS, where TempDir lives under /var -> /private/var.
func canonicalPath(t *testing.T, path string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

func TestAssessmentReadOnlyAndChildIndependence(t *testing.T) {
	for _, parent := range []testfixture.Marker{testfixture.Valid, testfixture.Corrupt, testfixture.Missing, testfixture.Unsupported} {
		location := testfixture.NewLocation(t, parent)
		repository := location.Repository(t, "child", testfixture.Valid, testfixture.MissingPrivate)
		before := treeSnapshot(t, location.Path)
		observer := factsObserver()
		locationPath := canonicalPath(t, location.Path)
		observer.MountInfoFunc = func(p string) (storage.MountFacts, error) {
			if p == locationPath {
				return storage.MountFacts{Platform: "linux", MountID: "A", Device: "A", MountPath: "/"}, nil
			}
			return storage.MountFacts{Platform: "linux", MountID: "B", Device: "B", MountPath: canonicalPath(t, repository.Path), Removable: true}, nil
		}
		parentResult := storage.AssessStorageTarget(context.Background(), observer, location.Path, false, "")
		child := storage.AssessStorageTarget(context.Background(), observer, repository.Path, true, repository.Config.ID)
		if !child.Capabilities.Read || !child.Capabilities.Verify || child.Classification.Support != storage.StorageSupported {
			t.Fatalf("parent fault gates healthy child: %+v", child)
		}
		if !reflect.DeepEqual(child.Classification.Risks, []storage.StorageRisk{storage.RiskRemovable, storage.RiskNonDefault}) {
			t.Fatalf("wrong child risks: %+v", child.Classification)
		}
		if child.Observation.RepositoryMarker.State != marker.Valid || child.Observation.Layout[".lumilio"].State != storage.AccessAbsent {
			t.Fatal("missing layout destroyed compatible identity")
		}
		repeated := storage.AssessStorageTarget(context.Background(), observer, repository.Path, true, repository.Config.ID)
		if storage.MountContinuityChanged(child.Observation.Mount, repeated.Observation.Mount) {
			t.Fatal("unchanged child continuity changed")
		}
		replacement := repeated.Observation.Mount
		replacement.Device = "replacement"
		if !storage.MountContinuityChanged(child.Observation.Mount, replacement) {
			t.Fatal("replacement not detected")
		}
		if parentResult.Observation.Mount.MountID == child.Observation.Mount.MountID {
			t.Fatal("child inherited parent mount")
		}
		if !reflect.DeepEqual(before, treeSnapshot(t, location.Path)) {
			t.Fatal("assessment changed tree")
		}
		for _, call := range observer.Calls {
			allowed := false
			for _, prefix := range []string{"stat:", "read:", "readdir:", "resolve:", "statfs:", "mount:", "attributes:"} {
				allowed = allowed || strings.HasPrefix(call, prefix)
			}
			if !allowed {
				t.Fatalf("unexpected event %s", call)
			}
		}
		// Exact interface method set excludes even temporary writes/locks/probes.
		iface := reflect.TypeOf((*storage.StorageObserver)(nil)).Elem()
		if iface.NumMethod() != 7 {
			t.Fatalf("observer gained a method: %v", iface)
		}
	}
}

func TestSharedStorageClassifierAcrossPlatforms(t *testing.T) {
	for _, platform := range []string{"linux", "darwin", "windows"} {
		for _, network := range []string{"nfs", "nfs4", "smb", "smbfs", "cifs", "afp", "afpfs", "sshfs", "fuse.sshfs", "webdav", "davfs", "fuse.davfs"} {
			t.Run(platform+"/"+network, func(t *testing.T) {
				observed := storage.StorageObservation{Access: storage.AccessReading{State: storage.AccessPresent}, Volume: storage.VolumeFacts{Filesystem: network}, Mount: storage.MountFacts{Platform: platform}}
				got := storage.ClassifyStorage(observed, storage.StoragePolicyInput{})
				if got.Support != storage.StorageUnsupported || got.Problem != storage.ProblemUnsupportedFilesystem {
					t.Fatalf("%+v", got)
				}
			})
		}
	}
	for _, tc := range []struct {
		mount storage.MountFacts
		want  storage.StorageSupport
		risks []storage.StorageRisk
	}{
		{storage.MountFacts{Platform: "windows", Remote: true}, storage.StorageUnsupported, nil},
		{storage.MountFacts{Platform: "windows", Removable: true}, storage.StorageSupported, []storage.StorageRisk{storage.RiskRemovable}},
		{storage.MountFacts{Platform: "darwin", CloudProvider: "icloud"}, storage.StorageSupported, []storage.StorageRisk{storage.RiskCloudSync}},
	} {
		got := storage.ClassifyStorage(storage.StorageObservation{Access: storage.AccessReading{State: storage.AccessPresent}, Volume: storage.VolumeFacts{Filesystem: "ntfs"}, Mount: tc.mount}, storage.StoragePolicyInput{})
		if got.Support != tc.want || !reflect.DeepEqual(got.Risks, tc.risks) {
			t.Fatalf("%+v", got)
		}
	}
}

func TestDockerEmptyTargetConstraint(t *testing.T) {
	for _, platform := range []string{"linux", "darwin", "windows"} {
		for _, mount := range []string{"/", "/target"} {
			observed := storage.StorageObservation{SamplePath: "/target", Access: storage.AccessReading{State: storage.AccessPresent}, Volume: storage.VolumeFacts{Filesystem: "ext4"}, Mount: storage.MountFacts{Platform: platform, MountPath: mount}}
			got := storage.ClassifyStorage(observed, storage.StoragePolicyInput{ExistingEmptyTarget: true})
			if platform == "linux" && mount != "/target" {
				if got.Problem != storage.ProblemNotMountedVolume {
					t.Fatal(got)
				}
			} else if got.Support != storage.StorageSupported {
				t.Fatal(got)
			}
		}
	}
	if !strings.Contains(storage.ErrRepositoryExistingTargetNotMountPoint.Error(), "container is recreated") || !strings.Contains(storage.NotMountedVolumeMessage, "compose") {
		t.Fatal("Docker explanation missing")
	}
}

func TestAbsentTargetUsesNearestProvenAncestor(t *testing.T) {
	location := testfixture.NewLocation(t, testfixture.Valid)
	observer := factsObserver()
	target := filepath.Join(location.Path, "absent", "child")
	result := storage.AssessStorageTarget(context.Background(), observer, target, true, "")
	if result.Observation.SamplePath != canonicalPath(t, location.Path) || result.Observation.Access.State != storage.AccessAbsent || !result.Capabilities.ReviewCreate {
		t.Fatalf("%+v", result)
	}
	observer.StatFunc = func(p string) (fs.FileInfo, error) {
		if p == target {
			return nil, fs.ErrPermission
		}
		return os.Stat(p)
	}
	result = storage.AssessStorageTarget(context.Background(), observer, target, true, "")
	if result.Observation.SamplePath != "" || result.Capabilities.ReviewCreate {
		t.Fatal("denial sampled parent")
	}
}

func TestPlaceholderErrorsRemainAccessErrors(t *testing.T) {
	location := testfixture.NewLocation(t, testfixture.Valid)
	repository := location.Repository(t, "child", testfixture.Valid, testfixture.Full)
	for _, err := range []error{nil, fs.ErrPermission, errors.New("attribute failure")} {
		observer := factsObserver()
		observer.PlaceholderFunc = func(string) (bool, error) { return true, err }
		got := storage.AssessStorageTarget(context.Background(), observer, repository.Path, false, repository.Config.ID)
		want := storage.ProblemMaterializationRequired
		if errors.Is(err, fs.ErrPermission) {
			want = storage.ProblemPermissionDenied
		} else if err != nil {
			want = storage.ProblemObservationUnknown
		}
		if got.Classification.Problem != want || got.Capabilities.Read || (err != nil && got.Observation.Placeholder) {
			t.Fatalf("%+v", got)
		}
	}
}

func treeSnapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	result := map[string]string{}
	err := filepath.WalkDir(root, func(p string, e fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := e.Info()
		if err != nil {
			return err
		}
		data := []byte{}
		if !e.IsDir() {
			data, err = os.ReadFile(p)
			if err != nil {
				return err
			}
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		result[rel] = fmt.Sprintf("%v/%d/%d/%x", info.Mode(), info.Size(), info.ModTime().UnixNano(), sha256.Sum256(data))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestProductionAssessmentDoesNotChangeTree(t *testing.T) {
	location, repository := testfixture.NewLeftoverPrimary(t)
	before := treeSnapshot(t, location.Path)
	result := storage.AssessStorageTarget(context.Background(), storage.OSStorageObserver{}, repository.Path, false, repository.Config.ID)
	if result.Observation.RepositoryMarker.State != marker.Valid {
		t.Fatalf("production reader: %+v", result.Observation.RepositoryMarker)
	}
	if !reflect.DeepEqual(before, treeSnapshot(t, location.Path)) {
		t.Fatal("production observation changed filesystem")
	}
}

func TestObservationFailuresBlockMutation(t *testing.T) {
	location := testfixture.NewLocation(t, testfixture.Valid)
	repository := location.Repository(t, "child", testfixture.Valid, testfixture.Full)
	for _, stage := range []string{"resolve", "statfs", "mount", "readdir"} {
		for _, cause := range []error{fs.ErrPermission, errors.New("observation failure")} {
			observer := factsObserver()
			switch stage {
			case "resolve":
				observer.EvalSymlinksFunc = func(string) (string, error) { return "", cause }
			case "statfs":
				observer.StatFSFunc = func(string) (storage.VolumeFacts, error) { return storage.VolumeFacts{}, cause }
			case "mount":
				observer.MountInfoFunc = func(string) (storage.MountFacts, error) { return storage.MountFacts{}, cause }
			case "readdir":
				observer.ReadDirFunc = func(string) ([]fs.DirEntry, error) { return nil, cause }
			}
			got := storage.AssessStorageTarget(context.Background(), observer, repository.Path, false, repository.Config.ID)
			want := storage.ProblemObservationUnknown
			if errors.Is(cause, fs.ErrPermission) {
				want = storage.ProblemPermissionDenied
			}
			if got.Classification.Problem != want || got.Capabilities.Read || got.Capabilities.MarkerWritePreflight || got.Capabilities.ReviewOpen || got.Capabilities.ReviewCreate {
				t.Fatalf("%s: %+v", stage, got)
			}
		}
	}
}

func TestChildSupportUsesActualFilesystem(t *testing.T) {
	location := testfixture.NewLocation(t, testfixture.Valid)
	child := location.Repository(t, "network-child", testfixture.Valid, testfixture.Full)
	observer := factsObserver()
	childPath := canonicalPath(t, child.Path)
	observer.StatFSFunc = func(p string) (storage.VolumeFacts, error) {
		filesystem := "ext4"
		if p == childPath {
			filesystem = "cifs"
		}
		return storage.VolumeFacts{Filesystem: filesystem, CapacityGroupKey: "same-pool"}, nil
	}
	parent := storage.AssessStorageTarget(context.Background(), observer, location.Path, false, "")
	assessed := storage.AssessStorageTarget(context.Background(), observer, child.Path, true, child.Config.ID)
	if parent.Classification.Support != storage.StorageSupported || assessed.Classification.Support != storage.StorageUnsupported || assessed.Classification.Problem != storage.ProblemUnsupportedFilesystem {
		t.Fatalf("parent=%+v child=%+v", parent.Classification, assessed.Classification)
	}
	if assessed.Capabilities.MarkerWritePreflight || assessed.Capabilities.ReviewOpen {
		t.Fatal("unsupported child admitted for mutation")
	}
	oldMount := assessed.Observation.Mount
	observer.StatFSFunc = func(string) (storage.VolumeFacts, error) {
		return storage.VolumeFacts{Filesystem: "cifs", CapacityGroupKey: "other-pool", AvailableBytes: 99}, nil
	}
	next := storage.AssessStorageTarget(context.Background(), observer, child.Path, true, child.Config.ID)
	if storage.MountContinuityChanged(oldMount, next.Observation.Mount) {
		t.Fatal("capacity grouping changed mount continuity")
	}
}
