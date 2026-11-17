package storage

import (
	"reflect"
	"strings"
	"testing"
)

const (
	testSharedCapacityGroupKey = "linux-statfs:ext4:010000120000001a"
	testOtherCapacityGroupKey  = "linux-statfs:xfs:010000130000001a"
	testWindowsVolumeGUIDPath  = `\\?\Volume{1f7c2d3a-4b5c-6d7e-8f90-a1b2c3d4e5f6}\`
)

// sumGroupAvailableBytes exists only to prove the no-double-count property in
// tests. Production callers must not sum figures across groups whose grouping
// is unknown.
func sumGroupAvailableBytes(groups []CapacityGroup) uint64 {
	var total uint64
	for _, group := range groups {
		total += group.AvailableBytes
	}
	return total
}

func TestGroupCapacityByBackingStorageCountsSharedPoolOnce(t *testing.T) {
	groups := GroupCapacityByBackingStorage([]StoragePathInfo{
		{CanonicalPath: "/srv/repository-a", CapacityKnown: true, TotalBytes: 1000, AvailableBytes: 400, CapacityGroupKey: testSharedCapacityGroupKey},
		{CanonicalPath: "/srv/repository-b", CapacityKnown: true, TotalBytes: 1000, AvailableBytes: 400, CapacityGroupKey: testSharedCapacityGroupKey},
	})
	if len(groups) != 1 {
		t.Fatalf("group count = %d, want 1: %#v", len(groups), groups)
	}
	group := groups[0]
	if !group.GroupingKnown || !group.CapacityKnown {
		t.Fatalf("group flags = %#v, want proven grouping with known capacity", group)
	}
	if group.Key != testSharedCapacityGroupKey {
		t.Fatalf("group key = %q, want %q", group.Key, testSharedCapacityGroupKey)
	}
	if group.TotalBytes != 1000 || group.AvailableBytes != 400 {
		t.Fatalf("group figure = %d/%d, want 1000/400", group.TotalBytes, group.AvailableBytes)
	}
	if !reflect.DeepEqual(group.MemberIndices, []int{0, 1}) {
		t.Fatalf("group members = %v, want [0 1]", group.MemberIndices)
	}
	if total := sumGroupAvailableBytes(groups); total != 400 {
		t.Fatalf("shared pool projected as %d available bytes, want the 400 byte pool once", total)
	}
}

func TestGroupCapacityByBackingStorageSeparatesDistinctPools(t *testing.T) {
	groups := GroupCapacityByBackingStorage([]StoragePathInfo{
		{CapacityKnown: true, TotalBytes: 1000, AvailableBytes: 400, CapacityGroupKey: testSharedCapacityGroupKey},
		{CapacityKnown: true, TotalBytes: 2000, AvailableBytes: 600, CapacityGroupKey: testOtherCapacityGroupKey},
	})
	if len(groups) != 2 {
		t.Fatalf("group count = %d, want 2: %#v", len(groups), groups)
	}
	if groups[0].Key != testSharedCapacityGroupKey || groups[1].Key != testOtherCapacityGroupKey {
		t.Fatalf("group order = %q, %q, want first-seen order", groups[0].Key, groups[1].Key)
	}
	if total := sumGroupAvailableBytes(groups); total != 1000 {
		t.Fatalf("distinct pools projected as %d available bytes, want 400+600", total)
	}
}

func TestGroupCapacityByBackingStorageKeepsUnknownGroupingExplicit(t *testing.T) {
	groups := GroupCapacityByBackingStorage([]StoragePathInfo{
		{CanonicalPath: "/srv/repository-a", CapacityKnown: true, TotalBytes: 1000, AvailableBytes: 400},
		{CanonicalPath: "/srv/repository-b", CapacityKnown: true, TotalBytes: 2000, AvailableBytes: 600},
	})
	if len(groups) != 2 {
		t.Fatalf("group count = %d, want 2 unmerged unknown paths: %#v", len(groups), groups)
	}
	for index, group := range groups {
		if group.GroupingKnown {
			t.Fatalf("unproven path %d reported proven grouping: %#v", index, group)
		}
		if group.Key != "" {
			t.Fatalf("unproven path %d reported key %q, want empty", index, group.Key)
		}
		if !group.CapacityKnown {
			t.Fatalf("unproven path %d lost its sampled figure: %#v", index, group)
		}
		if !reflect.DeepEqual(group.MemberIndices, []int{index}) {
			t.Fatalf("unproven path %d members = %v, want [%d]", index, group.MemberIndices, index)
		}
	}
	if groups[0].AvailableBytes != 400 || groups[1].AvailableBytes != 600 {
		t.Fatalf("figures = %d/%d, want each path to keep its own sample", groups[0].AvailableBytes, groups[1].AvailableBytes)
	}
}

func TestGroupCapacityByBackingStorageKeepsProvenGroupWithoutFigure(t *testing.T) {
	for _, testCase := range []struct {
		name  string
		infos []StoragePathInfo
	}{
		{
			name: "known then unknown",
			infos: []StoragePathInfo{
				{CapacityKnown: true, TotalBytes: 1000, AvailableBytes: 400, CapacityGroupKey: testSharedCapacityGroupKey},
				{CapacityKnown: false, CapacityGroupKey: testSharedCapacityGroupKey},
			},
		},
		{
			name: "unknown then known",
			infos: []StoragePathInfo{
				{CapacityKnown: false, CapacityGroupKey: testSharedCapacityGroupKey},
				{CapacityKnown: true, TotalBytes: 1000, AvailableBytes: 400, CapacityGroupKey: testSharedCapacityGroupKey},
			},
		},
		{
			name: "all unknown",
			infos: []StoragePathInfo{
				{CapacityKnown: false, CapacityGroupKey: testSharedCapacityGroupKey},
				{CapacityKnown: false, CapacityGroupKey: testSharedCapacityGroupKey},
			},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			groups := GroupCapacityByBackingStorage(testCase.infos)
			if len(groups) != 1 {
				t.Fatalf("group count = %d, want 1 proven pool: %#v", len(groups), groups)
			}
			group := groups[0]
			if !group.GroupingKnown {
				t.Fatalf("proven pool reported unknown grouping: %#v", group)
			}
			if group.CapacityKnown {
				t.Fatalf("group reported a figure one sample could not confirm: %#v", group)
			}
			if group.TotalBytes != 0 || group.AvailableBytes != 0 {
				t.Fatalf("unproven figure = %d/%d, want no invented total", group.TotalBytes, group.AvailableBytes)
			}
			if len(group.MemberIndices) != len(testCase.infos) {
				t.Fatalf("group members = %v, want %d entries", group.MemberIndices, len(testCase.infos))
			}
		})
	}
}

func TestGroupCapacityByBackingStorageReturnsRawAvailableBytes(t *testing.T) {
	// The figure stands for a volume that already stores Repository media. The
	// projection has no catalog-size input, so the raw sampled value must come
	// back unchanged and media bytes must never be subtracted again.
	const rawAvailable = uint64(123_456_789)
	info := StoragePathInfo{
		CapacityKnown:    true,
		TotalBytes:       500_000_000_000,
		AvailableBytes:   rawAvailable,
		CapacityGroupKey: testSharedCapacityGroupKey,
	}
	groups := GroupCapacityByBackingStorage([]StoragePathInfo{info, info})
	if len(groups) != 1 {
		t.Fatalf("group count = %d, want 1: %#v", len(groups), groups)
	}
	if groups[0].AvailableBytes != rawAvailable {
		t.Fatalf("available bytes = %d, want the raw %d without subtracting catalog size", groups[0].AvailableBytes, uint64(rawAvailable))
	}
}

func TestGroupCapacityByBackingStorageNeverOverstatesPool(t *testing.T) {
	groups := GroupCapacityByBackingStorage([]StoragePathInfo{
		{CapacityKnown: true, TotalBytes: 1000, AvailableBytes: 400, CapacityGroupKey: testSharedCapacityGroupKey},
		{CapacityKnown: true, TotalBytes: 1000, AvailableBytes: 380, CapacityGroupKey: testSharedCapacityGroupKey},
	})
	if len(groups) != 1 {
		t.Fatalf("group count = %d, want 1: %#v", len(groups), groups)
	}
	if groups[0].AvailableBytes != 380 {
		t.Fatalf("group available bytes = %d, want the smallest observed 380", groups[0].AvailableBytes)
	}
	if groups[0].TotalBytes != 1000 {
		t.Fatalf("group total bytes = %d, want 1000", groups[0].TotalBytes)
	}
}

func TestGroupCapacityByBackingStoragePreservesFirstSeenOrder(t *testing.T) {
	groups := GroupCapacityByBackingStorage([]StoragePathInfo{
		{CapacityKnown: true, TotalBytes: 1, AvailableBytes: 1, CapacityGroupKey: "b"},
		{CapacityKnown: true, TotalBytes: 1, AvailableBytes: 1, CapacityGroupKey: "a"},
		{CapacityKnown: true, TotalBytes: 1, AvailableBytes: 1, CapacityGroupKey: "b"},
		{CapacityKnown: true, TotalBytes: 1, AvailableBytes: 1, CapacityGroupKey: "c"},
		{CapacityKnown: true, TotalBytes: 1, AvailableBytes: 1, CapacityGroupKey: "a"},
	})
	if len(groups) != 3 {
		t.Fatalf("group count = %d, want 3: %#v", len(groups), groups)
	}
	wantKeys := []string{"b", "a", "c"}
	wantMembers := [][]int{{0, 2}, {1, 4}, {3}}
	for index := range groups {
		if groups[index].Key != wantKeys[index] {
			t.Fatalf("group %d key = %q, want %q", index, groups[index].Key, wantKeys[index])
		}
		if !reflect.DeepEqual(groups[index].MemberIndices, wantMembers[index]) {
			t.Fatalf("group %d members = %v, want %v", index, groups[index].MemberIndices, wantMembers[index])
		}
	}
}

func TestGroupCapacityByBackingStorageHandlesNoInput(t *testing.T) {
	if groups := GroupCapacityByBackingStorage(nil); len(groups) != 0 {
		t.Fatalf("nil input produced %#v", groups)
	}
	if groups := GroupCapacityByBackingStorage([]StoragePathInfo{}); len(groups) != 0 {
		t.Fatalf("empty input produced %#v", groups)
	}
}

func TestCapacityGroupKeyFromStatfsIDRejectsUnprovableIdentities(t *testing.T) {
	const filesystemID = "010000120000001a"
	for _, filesystem := range []string{
		"",
		"nfs", "nfs4", "cifs", "smbfs", "sshfs", "fuse.sshfs", "9p", "ceph", "afs", "davfs",
		"webdav", "autofs", "fuse.rclone", "macfuse", "vboxsf", "glusterfs",
	} {
		if key := capacityGroupKeyFromStatfsID(capacityGroupKindLinuxStatfs, filesystemID, filesystem); key != "" {
			t.Fatalf("filesystem %q produced key %q, want unknown grouping", filesystem, key)
		}
	}
	if key := capacityGroupKeyFromStatfsID(capacityGroupKindLinuxStatfs, "", "ext4"); key != "" {
		t.Fatalf("zero filesystem ID produced key %q, want unknown grouping", key)
	}
	if key := capacityGroupKeyFromStatfsID(capacityGroupKindLinuxStatfs, "   ", "ext4"); key != "" {
		t.Fatalf("blank filesystem ID produced key %q, want unknown grouping", key)
	}
	if key := capacityGroupKeyFromStatfsID("", filesystemID, "ext4"); key != "" {
		t.Fatalf("missing proof kind produced key %q, want unknown grouping", key)
	}
}

// TestCapacityGroupingRejectsVirtioAndDriveShares locks the rule that a
// hypervisor- or agent-assigned filesystem identity cannot prove a shared
// capacity pool. Docker Desktop (virtiofs, gRPC-FUSE) and WSL drive mounts
// (drvfs) repeat or zero the identity outside this host's control, so they
// must stay ungrouped instead of risking a false shared-capacity claim.
func TestCapacityGroupingRejectsVirtioAndDriveShares(t *testing.T) {
	const filesystemID = "010000120000001a"
	for _, filesystem := range []string{"virtiofs", "drvfs", "VIRTIOFS", " drvfs "} {
		if key := capacityGroupKeyFromStatfsID(capacityGroupKindLinuxStatfs, filesystemID, filesystem); key != "" {
			t.Fatalf("filesystem %q produced key %q, want unknown grouping", filesystem, key)
		}
		if key := capacityGroupKeyFromStatfsID(capacityGroupKindDarwinStatfs, filesystemID, filesystem); key != "" {
			t.Fatalf("filesystem %q produced key %q on darwin, want unknown grouping", filesystem, key)
		}
	}
	if key := windowsCapacityGroupKey("virtiofs", `\\?\Volume{11111111-2222-3333-4444-555555555555}\`, 0x1234, capacityGroupDriveFixed); key != "" {
		t.Fatalf("windows virtiofs produced key %q, want unknown grouping", key)
	}
}

func TestCapacityGroupKeyFromStatfsIDNormalizesAndKeepsProofsSeparate(t *testing.T) {
	linux := capacityGroupKeyFromStatfsID(capacityGroupKindLinuxStatfs, " 010000120000001A ", " EXT4 ")
	if linux != "linux-statfs:ext4:010000120000001a" {
		t.Fatalf("linux key = %q, want normalized ext4 key", linux)
	}
	darwin := capacityGroupKeyFromStatfsID(capacityGroupKindDarwinStatfs, "010000120000001a", "apfs")
	if darwin != "darwin-statfs:apfs:010000120000001a" {
		t.Fatalf("darwin key = %q, want normalized apfs key", darwin)
	}
	if linux == darwin {
		t.Fatal("the same filesystem ID and type must not collide across platforms")
	}
	if capacityGroupKeyFromStatfsID(capacityGroupKindLinuxStatfs, "010000120000001a", "ext4") ==
		capacityGroupKeyFromStatfsID(capacityGroupKindLinuxStatfs, "010000120000001b", "ext4") {
		t.Fatal("different filesystem IDs must not share a key")
	}
	if capacityGroupKeyFromStatfsID(capacityGroupKindLinuxStatfs, "010000120000001a", "ext4") ==
		capacityGroupKeyFromStatfsID(capacityGroupKindLinuxStatfs, "010000120000001a", "xfs") {
		t.Fatal("different filesystem types must not share a key")
	}
}

func TestWindowsCapacityGroupKeyGroupsFolderMountWithDriveLetter(t *testing.T) {
	// C:\Data\repository and D:\repository both resolve through
	// GetVolumePathName and GetVolumeNameForVolumeMountPoint to the same
	// volume GUID when C:\Data is D:'s mount point, so the resolved GUID path
	// below stands for either access path.
	folderMount := windowsCapacityGroupKey("NTFS", testWindowsVolumeGUIDPath, 0x1a2b3c4d, capacityGroupDriveFixed)
	driveLetter := windowsCapacityGroupKey("ntfs", `\\?\VOLUME{1F7C2D3A-4B5C-6D7E-8F90-A1B2C3D4E5F6}\`, 0x1a2b3c4d, capacityGroupDriveFixed)
	if folderMount == "" {
		t.Fatal("proven local volume produced an unknown key")
	}
	if folderMount != driveLetter {
		t.Fatalf("folder mount key %q != drive letter key %q", folderMount, driveLetter)
	}
	if folderMount != "windows-volumeguid:ntfs:1f7c2d3a-4b5c-6d7e-8f90-a1b2c3d4e5f6:1a2b3c4d" {
		t.Fatalf("windows key = %q, want the volume GUID and serial proof", folderMount)
	}
}

func TestWindowsCapacityGroupKeyRequiresProvenVolumeIdentity(t *testing.T) {
	for _, testCase := range []struct {
		name      string
		driveType uint32
	}{
		{name: "unknown drive", driveType: capacityGroupDriveUnknown},
		{name: "no root directory", driveType: capacityGroupDriveNoRootDir},
		{name: "remote drive", driveType: capacityGroupDriveRemote},
		{name: "unexpected drive type", driveType: 99},
	} {
		if key := windowsCapacityGroupKey("NTFS", testWindowsVolumeGUIDPath, 7, testCase.driveType); key != "" {
			t.Fatalf("%s produced key %q, want unknown grouping", testCase.name, key)
		}
	}
	for _, guidPath := range []string{
		"",
		`C:\`,
		`D:`,
		`\\server\share\`,
		`\\?\Volume{not-a-guid}\`,
		`\\?\Volume{1f7c2d3a4b5c6d7e8f90a1b2c3d4e5f6}\`,
	} {
		if key := windowsCapacityGroupKey("NTFS", guidPath, 7, capacityGroupDriveFixed); key != "" {
			t.Fatalf("volume path %q produced key %q, want unknown grouping", guidPath, key)
		}
	}
	if key := windowsCapacityGroupKey("fuse.rclone", testWindowsVolumeGUIDPath, 7, capacityGroupDriveFixed); key != "" {
		t.Fatalf("unprovable filesystem produced key %q, want unknown grouping", key)
	}
	if key := windowsCapacityGroupKey("", testWindowsVolumeGUIDPath, 7, capacityGroupDriveFixed); key != "" {
		t.Fatalf("missing filesystem produced key %q, want unknown grouping", key)
	}
	if windowsCapacityGroupKey("NTFS", testWindowsVolumeGUIDPath, 1, capacityGroupDriveFixed) ==
		windowsCapacityGroupKey("NTFS", testWindowsVolumeGUIDPath, 2, capacityGroupDriveFixed) {
		t.Fatal("different volume serial numbers must not share a key")
	}
	if windowsCapacityGroupKey("NTFS", testWindowsVolumeGUIDPath, 1, capacityGroupDriveFixed) ==
		windowsCapacityGroupKey("NTFS", `\\?\Volume{2a8d3e4b-5c6d-7e8f-9012-b3c4d5e6f708}\`, 1, capacityGroupDriveFixed) {
		t.Fatal("different volume GUIDs must not share a key")
	}
	if windowsCapacityGroupKey("NTFS", testWindowsVolumeGUIDPath, 1, capacityGroupDriveFixed) ==
		windowsCapacityGroupKey("ReFS", testWindowsVolumeGUIDPath, 1, capacityGroupDriveFixed) {
		t.Fatal("different filesystems on the same volume must not share a key")
	}
}

func TestNormalizeWindowsVolumeGUIDRequiresVolumeGUIDShape(t *testing.T) {
	if got := normalizeWindowsVolumeGUID(testWindowsVolumeGUIDPath); got != "1f7c2d3a-4b5c-6d7e-8f90-a1b2c3d4e5f6" {
		t.Fatalf("normalized GUID = %q", got)
	}
	if got := normalizeWindowsVolumeGUID(`\\?\Volume{1f7c2d3a-4b5c-6d7e-8f90-a1b2c3d4e5f6}`); got != "1f7c2d3a-4b5c-6d7e-8f90-a1b2c3d4e5f6" {
		t.Fatalf("normalized GUID without trailing separator = %q", got)
	}
	for _, malformed := range []string{
		"",
		`C:\Data\`,
		`\\?\Volume{1f7c2d3a4b5c6d7e8f90a1b2c3d4e5f6}\`,
		`\\?\Volume{1f7c2d3a-4b5c-6d7e-8f90-a1b2c3d4e5f6-extra}\`,
		`\\?\Volume{1f7c2d3g-4b5c-6d7e-8f90-a1b2c3d4e5f6}\`,
	} {
		if got := normalizeWindowsVolumeGUID(malformed); got != "" {
			t.Fatalf("malformed volume path %q normalized to %q, want empty", malformed, got)
		}
	}
	if !strings.HasPrefix(testWindowsVolumeGUIDPath, `\\?\Volume{`) {
		t.Fatal("test fixture is not a volume GUID path")
	}
}
