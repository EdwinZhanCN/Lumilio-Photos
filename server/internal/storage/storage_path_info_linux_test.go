//go:build linux

package storage

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// TestLinuxMountPathResolvesToTheSampledFilesystem locks the storage label the
// admin storage view shows next to a Repository's capacity: the reported mount
// path must name an absolute existing directory that resolves to the same
// filesystem identity the capacity was sampled from. A label pointing at a
// different mount (the parent of a Docker child mount, for example) would
// misattribute free space to the wrong storage.
func TestLinuxMountPathResolvesToTheSampledFilesystem(t *testing.T) {
	info := InspectStoragePath(t.TempDir())
	if info.MountPath == "" {
		t.Fatalf("mount path was not sampled: %#v", info)
	}
	if !filepath.IsAbs(info.MountPath) {
		t.Fatalf("mount path %q is not absolute", info.MountPath)
	}
	stat, err := os.Stat(info.MountPath)
	if err != nil || !stat.IsDir() {
		t.Fatalf("mount path %q is not an existing directory: %v", info.MountPath, err)
	}
	_, _, filesystem, err := inspectVolume(info.MountPath)
	if err != nil {
		t.Fatalf("mount path %q could not be sampled: %v", info.MountPath, err)
	}
	if key := capacityGroupKeyForPath(info.MountPath, filesystem); key != info.CapacityGroupKey {
		t.Fatalf("mount path %q resolves to %q, want the sampled key %q", info.MountPath, key, info.CapacityGroupKey)
	}
}

func TestLinuxStatfsFilesystemIDRejectsZeroIdentity(t *testing.T) {
	if encoded := linuxStatfsFilesystemID([2]int32{0, 0}); encoded != "" {
		t.Fatalf("zero filesystem ID encoded as %q, want unknown", encoded)
	}
	encoded := linuxStatfsFilesystemID([2]int32{0x01000012, 0x1a})
	if encoded != "010000120000001a" {
		t.Fatalf("encoded filesystem ID = %q, want 010000120000001a", encoded)
	}
	if encoded != linuxStatfsFilesystemID([2]int32{0x01000012, 0x1a}) {
		t.Fatal("filesystem ID encoding is not deterministic")
	}
	if encoded == linuxStatfsFilesystemID([2]int32{0x01000013, 0x1a}) {
		t.Fatal("different filesystem IDs must not collide")
	}
}

func TestLinuxCapacityGroupKeyUsesStatfsIdentity(t *testing.T) {
	info := InspectStoragePath(t.TempDir())
	if !info.CapacityKnown {
		t.Fatalf("temporary directory capacity was not sampled: %#v", info)
	}
	if !strings.HasPrefix(info.CapacityGroupKey, capacityGroupKindLinuxStatfs+":") {
		t.Fatalf("capacity group key = %q, want the linux statfs proof", info.CapacityGroupKey)
	}
	if !strings.Contains(info.CapacityGroupKey, strings.ToLower(info.Filesystem)) {
		t.Fatalf("capacity group key %q does not carry filesystem %q", info.CapacityGroupKey, info.Filesystem)
	}
}

// TestInspectStoragePathProvesSharedCapacityGroupAcrossBindMount is the native
// bind-mount proof: paths on one superblock must share a statfs-derived group
// key and project as one capacity pool even when Linux reports distinct mount
// IDs for each bind target.
func TestInspectStoragePathProvesSharedCapacityGroupAcrossBindMount(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	mountPoint := filepath.Join(root, "bind-mount")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "marker"), []byte("bind-mount-proof"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(mountPoint, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := syscall.Mount(source, mountPoint, "", syscall.MS_BIND, ""); err != nil {
		if errors.Is(err, syscall.EPERM) || errors.Is(err, syscall.EACCES) {
			t.Skipf("bind mount requires privileges (CAP_SYS_ADMIN): %v", err)
		}
		t.Fatalf("bind mount: %v", err)
	}
	t.Cleanup(func() {
		if err := syscall.Unmount(mountPoint, syscall.MNT_DETACH); err != nil {
			t.Errorf("unmount bind target: %v", err)
		}
	})

	sourceInfo := InspectStoragePath(source)
	mountInfo := InspectStoragePath(mountPoint)
	if !sourceInfo.CapacityKnown || !mountInfo.CapacityKnown {
		t.Fatalf("bind-mounted paths did not sample capacity: %#v %#v", sourceInfo, mountInfo)
	}
	if sourceInfo.Filesystem == "" || mountInfo.Filesystem == "" {
		t.Fatalf("bind-mounted paths did not resolve a filesystem: %#v %#v", sourceInfo, mountInfo)
	}
	if sourceInfo.CapacityGroupKey == "" || mountInfo.CapacityGroupKey == "" {
		t.Fatalf("bind-mounted paths did not prove a capacity group: %#v %#v", sourceInfo, mountInfo)
	}
	if !strings.HasPrefix(sourceInfo.CapacityGroupKey, capacityGroupKindLinuxStatfs+":") {
		t.Fatalf("capacity group key %q does not carry the linux statfs proof", sourceInfo.CapacityGroupKey)
	}
	if mountInfo.CapacityGroupKey != sourceInfo.CapacityGroupKey {
		t.Fatalf("bind mount reported different capacity groups: source %q mount %q",
			sourceInfo.CapacityGroupKey, mountInfo.CapacityGroupKey)
	}
	for _, info := range []StoragePathInfo{sourceInfo, mountInfo} {
		if info.CapacityGroupKey == info.MountID {
			t.Fatalf("capacity group key reused MountID %q", info.MountID)
		}
		if info.CapacityGroupKey == info.MountFingerprint {
			t.Fatalf("capacity group key reused MountFingerprint %q", info.MountFingerprint)
		}
		if strings.Contains(info.CapacityGroupKey, info.CanonicalPath) {
			t.Fatalf("capacity group key %q embeds the canonical path %q", info.CapacityGroupKey, info.CanonicalPath)
		}
	}
	if sourceInfo.MountID == "" || mountInfo.MountID == "" {
		t.Fatalf("bind mount proof requires non-empty mount IDs: %#v %#v", sourceInfo, mountInfo)
	}
	if sourceInfo.MountID == mountInfo.MountID {
		t.Fatalf("bind mount did not produce distinct mount IDs (%q); cannot prove grouping ignores mount identity",
			sourceInfo.MountID)
	}

	groups := GroupCapacityByBackingStorage([]StoragePathInfo{sourceInfo, mountInfo})
	if len(groups) != 1 {
		t.Fatalf("bind-mounted paths projected into %d groups: %#v", len(groups), groups)
	}
	if len(groups[0].MemberIndices) != 2 {
		t.Fatalf("shared group members = %v, want both paths", groups[0].MemberIndices)
	}
	if groups[0].AvailableBytes != sourceInfo.AvailableBytes && groups[0].AvailableBytes != mountInfo.AvailableBytes {
		t.Fatalf("group available bytes %d match neither raw sample %d/%d",
			groups[0].AvailableBytes, sourceInfo.AvailableBytes, mountInfo.AvailableBytes)
	}
	if total := sumGroupAvailableBytes(groups); total != groups[0].AvailableBytes {
		t.Fatalf("shared pool projected as %d available bytes, want the %d byte pool once",
			total, groups[0].AvailableBytes)
	}
}
