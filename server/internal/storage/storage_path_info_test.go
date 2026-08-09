package storage

import (
	"runtime"
	"strings"
	"testing"
)

func TestInspectStoragePathReportsPerPathCapacityAndWritability(t *testing.T) {
	info := InspectStoragePath(t.TempDir())
	if !info.Writable {
		t.Fatal("temporary directory should be writable")
	}
	if !info.CapacityKnown {
		t.Fatal("temporary directory capacity should be available on supported platforms")
	}
	if info.TotalBytes == 0 || info.AvailableBytes > info.TotalBytes {
		t.Fatalf("invalid capacity: %#v", info)
	}
	if runtime.GOOS == "linux" && info.Filesystem == "" {
		t.Fatalf("Linux filesystem type was not resolved: %#v", info)
	}
	if info.CanonicalPath == "" || info.MountFingerprint == "" {
		t.Fatalf("diagnostic identity was not resolved: %#v", info)
	}
	if info.EffectiveUID == "" || info.EffectiveGID == "" || !info.CaseBehaviorKnown {
		t.Fatalf("diagnostic process/case facts were not resolved: %#v", info)
	}
}

func TestFilesystemFromMountInfoPrefersDockerChildMount(t *testing.T) {
	mountInfo := strings.Join([]string{
		"20 1 0:20 / / rw,relatime - overlay overlay rw",
		"21 20 0:21 / /srv/lumilio rw,relatime - ext4 /dev/root rw",
		"22 21 0:22 / /srv/lumilio/media rw,relatime - xfs /dev/mapper/media rw",
	}, "\n")
	filesystem, err := filesystemFromMountInfo(strings.NewReader(mountInfo), "/srv/lumilio/media/repository-a")
	if err != nil {
		t.Fatal(err)
	}
	if filesystem != "xfs" {
		t.Fatalf("child mount filesystem = %q, want xfs", filesystem)
	}
}
