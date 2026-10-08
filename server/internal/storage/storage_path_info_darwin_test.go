//go:build darwin

package storage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDarwinMountPathResolvesToTheSampledFilesystem locks the storage label the
// admin storage view shows next to a Repository's capacity: the reported mount
// path must name an absolute existing directory that resolves to the same
// filesystem identity the capacity was sampled from. A label pointing at a
// different volume would misattribute free space to the wrong storage.
func TestDarwinMountPathResolvesToTheSampledFilesystem(t *testing.T) {
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
	// firmlinks mean the mount path is not always a string prefix of the
	// canonical path, so prove the identity through statfs instead.
	_, _, filesystem, err := inspectVolume(info.MountPath)
	if err != nil {
		t.Fatalf("mount path %q could not be sampled: %v", info.MountPath, err)
	}
	if key := capacityGroupKeyForPath(info.MountPath, filesystem); key != info.CapacityGroupKey {
		t.Fatalf("mount path %q resolves to %q, want the sampled key %q", info.MountPath, key, info.CapacityGroupKey)
	}
}

func TestDarwinStatfsFilesystemIDRejectsZeroIdentity(t *testing.T) {
	if encoded := darwinStatfsFilesystemID([2]int32{0, 0}); encoded != "" {
		t.Fatalf("zero filesystem ID encoded as %q, want unknown", encoded)
	}
	encoded := darwinStatfsFilesystemID([2]int32{0x01000012, 0x1a})
	if encoded != "010000120000001a" {
		t.Fatalf("encoded filesystem ID = %q, want 010000120000001a", encoded)
	}
	if encoded != darwinStatfsFilesystemID([2]int32{0x01000012, 0x1a}) {
		t.Fatal("filesystem ID encoding is not deterministic")
	}
	if encoded == darwinStatfsFilesystemID([2]int32{0x01000013, 0x1a}) {
		t.Fatal("different filesystem IDs must not collide")
	}
}

func TestDarwinCapacityGroupKeyUsesStatfsIdentity(t *testing.T) {
	info := InspectStoragePath(t.TempDir())
	if !info.CapacityKnown {
		t.Fatalf("temporary directory capacity was not sampled: %#v", info)
	}
	if !strings.HasPrefix(info.CapacityGroupKey, capacityGroupKindDarwinStatfs+":") {
		t.Fatalf("capacity group key = %q, want the darwin statfs proof", info.CapacityGroupKey)
	}
	if !strings.Contains(info.CapacityGroupKey, strings.ToLower(info.Filesystem)) {
		t.Fatalf("capacity group key %q does not carry filesystem %q", info.CapacityGroupKey, info.Filesystem)
	}
}
