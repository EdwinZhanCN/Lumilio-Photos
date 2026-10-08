package storage

import (
	"os"
	"path/filepath"
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
	if (runtime.GOOS == "linux" || runtime.GOOS == "darwin") &&
		(info.EffectiveUID == "" || info.EffectiveGID == "") {
		t.Fatalf("diagnostic process/case facts were not resolved: %#v", info)
	}
	if !info.CaseBehaviorKnown {
		t.Fatalf("diagnostic case facts were not resolved: %#v", info)
	}
}

func TestFilesystemFromMountInfoPrefersDockerChildMount(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux mount-info paths are only meaningful on Linux")
	}
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

// TestInspectStoragePathProvesSharedCapacityGroupAtCanonicalPath is the native
// temporary-directory proof: a real filesystem must yield a non-empty key that
// is stable across samples, shared by sibling repositories, and independent of
// the mount observation identity.
func TestInspectStoragePathProvesSharedCapacityGroupAtCanonicalPath(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("capacity group keys are proven from statfs on Linux and Darwin only")
	}
	root := t.TempDir()
	first := filepath.Join(root, "repository-a")
	second := filepath.Join(root, "repository-b")
	for _, directory := range []string{first, second} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	firstInfo := InspectStoragePath(first)
	secondInfo := InspectStoragePath(second)
	if !firstInfo.CapacityKnown || !secondInfo.CapacityKnown {
		t.Fatalf("temporary directory capacity was not sampled: %#v %#v", firstInfo, secondInfo)
	}
	if firstInfo.CanonicalPath == "" {
		t.Fatalf("canonical path was not resolved: %#v", firstInfo)
	}
	if firstInfo.Filesystem == "" {
		t.Fatalf("%s did not resolve a filesystem type: %#v", runtime.GOOS, firstInfo)
	}
	if firstInfo.CapacityGroupKey == "" {
		t.Fatalf("real %s filesystem did not prove a capacity group: %#v", runtime.GOOS, firstInfo)
	}
	kind := capacityGroupKindLinuxStatfs
	if runtime.GOOS == "darwin" {
		kind = capacityGroupKindDarwinStatfs
	}
	if !strings.HasPrefix(firstInfo.CapacityGroupKey, kind+":") {
		t.Fatalf("capacity group key %q does not carry the %s proof", firstInfo.CapacityGroupKey, kind)
	}
	if !strings.Contains(firstInfo.CapacityGroupKey, strings.ToLower(firstInfo.Filesystem)) {
		t.Fatalf("capacity group key %q does not carry filesystem %q", firstInfo.CapacityGroupKey, firstInfo.Filesystem)
	}
	if secondInfo.CapacityGroupKey != firstInfo.CapacityGroupKey {
		t.Fatalf("sibling repositories on one filesystem reported different groups: %q vs %q",
			firstInfo.CapacityGroupKey, secondInfo.CapacityGroupKey)
	}
	if repeated := InspectStoragePath(first); repeated.CapacityGroupKey != firstInfo.CapacityGroupKey {
		t.Fatalf("capacity group key changed between identical samples: %q vs %q",
			firstInfo.CapacityGroupKey, repeated.CapacityGroupKey)
	}
	// Grouping is a separate fact from mount observation identity.
	if firstInfo.CapacityGroupKey == firstInfo.MountID {
		t.Fatalf("capacity group key reused MountID %q", firstInfo.MountID)
	}
	if firstInfo.CapacityGroupKey == firstInfo.MountFingerprint {
		t.Fatalf("capacity group key reused MountFingerprint %q", firstInfo.MountFingerprint)
	}
	if strings.Contains(firstInfo.CapacityGroupKey, firstInfo.CanonicalPath) {
		t.Fatalf("capacity group key %q embeds the canonical path", firstInfo.CapacityGroupKey)
	}
	groups := GroupCapacityByBackingStorage([]StoragePathInfo{firstInfo, secondInfo})
	if len(groups) != 1 {
		t.Fatalf("shared filesystem projected into %d groups: %#v", len(groups), groups)
	}
	if len(groups[0].MemberIndices) != 2 {
		t.Fatalf("shared group members = %v, want both repositories", groups[0].MemberIndices)
	}
	if groups[0].AvailableBytes != firstInfo.AvailableBytes && groups[0].AvailableBytes != secondInfo.AvailableBytes {
		t.Fatalf("group available bytes %d match neither raw sample %d/%d",
			groups[0].AvailableBytes, firstInfo.AvailableBytes, secondInfo.AvailableBytes)
	}
}

// TestInspectStoragePathSamplesCanonicalPathForGrouping proves the key is
// sampled at the filesystem that actually backs the requested path rather than
// at a symlink that merely names it.
func TestInspectStoragePathSamplesCanonicalPathForGrouping(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("capacity group keys are proven from statfs on Linux and Darwin only")
	}
	root := t.TempDir()
	target := filepath.Join(root, "repository")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "repository-link")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks are unavailable: %v", err)
	}
	targetInfo := InspectStoragePath(target)
	linkInfo := InspectStoragePath(link)
	if !targetInfo.CapacityKnown || !linkInfo.CapacityKnown {
		t.Fatalf("capacity was not sampled through the symlink: %#v %#v", targetInfo, linkInfo)
	}
	if linkInfo.CanonicalPath != targetInfo.CanonicalPath {
		t.Fatalf("canonical path = %q, want %q", linkInfo.CanonicalPath, targetInfo.CanonicalPath)
	}
	if linkInfo.Filesystem != targetInfo.Filesystem {
		t.Fatalf("filesystem = %q, want %q", linkInfo.Filesystem, targetInfo.Filesystem)
	}
	if linkInfo.CapacityGroupKey == "" || linkInfo.CapacityGroupKey != targetInfo.CapacityGroupKey {
		t.Fatalf("capacity group key through symlink = %q, want %q",
			linkInfo.CapacityGroupKey, targetInfo.CapacityGroupKey)
	}
}
