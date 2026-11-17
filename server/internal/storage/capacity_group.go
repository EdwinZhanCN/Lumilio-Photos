package storage

import (
	"fmt"
	"strings"
)

// This file owns the shared-backing-capacity grouping contract. Sampling is
// platform-specific (capacityGroupKeyForPath in storage_path_info_*.go); the
// decisions that must agree on every platform live here so they can be tested
// deterministically without depending on the host filesystem.

// Capacity group key kinds. The kind is part of the key so keys produced by
// different platforms or different proof mechanisms never compare equal.
const (
	capacityGroupKindLinuxStatfs       = "linux-statfs"
	capacityGroupKindDarwinStatfs      = "darwin-statfs"
	capacityGroupKindWindowsVolumeGUID = "windows-volumeguid"
)

// Win32 drive types (winbase.h) mirrored as plain integers so the Windows
// decision helper stays buildable and testable on every platform. The Windows
// sampler passes windows.GetDriveType's result unchanged.
const (
	capacityGroupDriveUnknown   = 0 // DRIVE_UNKNOWN
	capacityGroupDriveNoRootDir = 1 // DRIVE_NO_ROOT_DIR
	capacityGroupDriveRemovable = 2 // DRIVE_REMOVABLE
	capacityGroupDriveFixed     = 3 // DRIVE_FIXED
	capacityGroupDriveRemote    = 4 // DRIVE_REMOTE
	capacityGroupDriveCDROM     = 5 // DRIVE_CDROM
	capacityGroupDriveRAMDisk   = 6 // DRIVE_RAMDISK
)

// CapacityGroup is one backing capacity pool observed at Repository paths.
//
// A group's figure describes the pool once. Callers must render it once per
// group and must not sum AvailableBytes across groups whose GroupingKnown is
// false, because those entries are independent samples that were never proven
// to be distinct pools.
type CapacityGroup struct {
	// Key is the shared CapacityGroupKey of every member. It is empty exactly
	// when GroupingKnown is false.
	Key string
	// GroupingKnown reports whether Key proves that every member shares one
	// backing capacity pool. Unknown grouping is never merged: each unproven
	// path becomes its own group so no aggregate is invented.
	GroupingKnown bool
	// CapacityKnown reports whether the group carries a sampled figure. It is
	// independent of GroupingKnown: a proven group may have no figure, and an
	// unproven path may have one. TotalBytes and AvailableBytes are zero when
	// it is false.
	CapacityKnown bool
	// TotalBytes and AvailableBytes are the backing volume's raw sampled
	// figures in bytes. Available bytes already reflect stored Repository
	// files, so catalog media size is never subtracted. For a proven group
	// they are the largest total and the smallest available figure observed
	// across members, so a group never overstates the pool.
	TotalBytes     uint64
	AvailableBytes uint64
	// MemberIndices are the input positions of the sampled paths in this
	// group, in input order.
	MemberIndices []int
}

// GroupCapacityByBackingStorage projects sampled StoragePathInfo values into
// one entry per proven backing capacity pool, preserving first-seen input
// order.
//
// A proven group's figure is reported once instead of once per member, so two
// Repositories on one volume are never double counted. A path whose
// CapacityGroupKey is empty is returned as its own group with
// GroupingKnown == false: unknown grouping stays explicit and never becomes an
// invented total. The projection has no media-size input and performs no
// subtraction.
func GroupCapacityByBackingStorage(infos []StoragePathInfo) []CapacityGroup {
	groups := make([]CapacityGroup, 0, len(infos))
	positionByKey := make(map[string]int, len(infos))
	for index, info := range infos {
		key := strings.TrimSpace(info.CapacityGroupKey)
		if key == "" {
			groups = append(groups, ungroupedCapacityGroup(index, info))
			continue
		}
		position, seen := positionByKey[key]
		if !seen {
			positionByKey[key] = len(groups)
			group := ungroupedCapacityGroup(index, info)
			group.Key = key
			group.GroupingKnown = true
			groups = append(groups, group)
			continue
		}
		mergeCapacityGroupMember(&groups[position], index, info)
	}
	return groups
}

func ungroupedCapacityGroup(index int, info StoragePathInfo) CapacityGroup {
	group := CapacityGroup{MemberIndices: []int{index}}
	if !info.CapacityKnown {
		return group
	}
	group.CapacityKnown = true
	group.TotalBytes = info.TotalBytes
	group.AvailableBytes = info.AvailableBytes
	return group
}

func mergeCapacityGroupMember(group *CapacityGroup, index int, info StoragePathInfo) {
	group.MemberIndices = append(group.MemberIndices, index)
	if !info.CapacityKnown {
		// One unproven member makes the whole pool figure unproven: a group
		// never reports a figure that one of its own samples could not confirm.
		group.CapacityKnown = false
		group.TotalBytes = 0
		group.AvailableBytes = 0
		return
	}
	if !group.CapacityKnown {
		return
	}
	if info.TotalBytes > group.TotalBytes {
		group.TotalBytes = info.TotalBytes
	}
	if info.AvailableBytes < group.AvailableBytes {
		group.AvailableBytes = info.AvailableBytes
	}
}

// capacityGroupKeyFromStatfsID builds the Linux/macOS proof: a filesystem type
// plus a nonzero statfs filesystem ID. An empty result means grouping is not
// proven and must stay unknown.
func capacityGroupKeyFromStatfsID(kind, filesystemID, filesystemType string) string {
	id := normalizeCapacityGroupToken(filesystemID)
	filesystem := normalizeCapacityGroupToken(filesystemType)
	if kind == "" || id == "" || !capacityGroupingProvable(filesystem) {
		return ""
	}
	return kind + ":" + filesystem + ":" + id
}

// windowsCapacityGroupKey builds the Windows proof from the containing
// volume's GUID path, serial number, filesystem, and drive type. The caller
// resolves the volume with GetVolumePathName so a volume mounted into an NTFS
// folder is grouped with its drive letter. An empty result means grouping is
// not proven and must stay unknown.
func windowsCapacityGroupKey(filesystemType, volumeGUIDPath string, serial uint32, driveType uint32) string {
	switch driveType {
	case capacityGroupDriveRemovable, capacityGroupDriveFixed, capacityGroupDriveCDROM, capacityGroupDriveRAMDisk:
		// Locally attached media with a locally provable capacity pool.
	default:
		// DRIVE_UNKNOWN, DRIVE_NO_ROOT_DIR, DRIVE_REMOTE, and any unexpected
		// value: a remote or unresolvable volume shares capacity outside this
		// host's control.
		return ""
	}
	filesystem := normalizeCapacityGroupToken(filesystemType)
	guid := normalizeWindowsVolumeGUID(volumeGUIDPath)
	if guid == "" || !capacityGroupingProvable(filesystem) {
		return ""
	}
	return capacityGroupKindWindowsVolumeGUID + ":" + filesystem + ":" + guid + ":" + fmt.Sprintf("%08x", serial)
}

// capacityGroupingProvable reports whether a filesystem type can prove a
// shared capacity pool from a filesystem ID on this host. Network, FUSE, and
// hypervisor-shared filesystems assign identities outside this host's control
// (a server, a userspace daemon, or a VM agent may repeat or zero them), so
// they stay unknown instead of risking a false shared-capacity claim.
func capacityGroupingProvable(filesystem string) bool {
	switch {
	case filesystem == "":
		return false
	case isNetworkFilesystem(filesystem):
		return false
	case strings.HasPrefix(filesystem, "nfs"):
		return false
	case strings.HasPrefix(filesystem, "smb"):
		return false
	case strings.HasPrefix(filesystem, "fuse"):
		return false
	case strings.HasPrefix(filesystem, "macfuse"):
		return false
	case strings.HasPrefix(filesystem, "osxfuse"):
		return false
	case filesystem == "webdav",
		filesystem == "autofs",
		filesystem == "glusterfs",
		filesystem == "vboxsf",
		filesystem == "prl_fs",
		filesystem == "vmhgfs",
		// virtiofs and drvfs are shared folders presented by a VM agent
		// (Docker Desktop on macOS and Windows, WSL drive mounts). The guest
		// sees a synthetic filesystem, so two independent host disks can
		// present one repeated identity.
		filesystem == "virtiofs",
		filesystem == "drvfs":
		return false
	default:
		return true
	}
}

func normalizeCapacityGroupToken(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

// normalizeWindowsVolumeGUID validates a \\?\Volume{GUID}\ path and returns
// the bare lowercase GUID. Anything else is not a proven volume identity.
func normalizeWindowsVolumeGUID(volumeGUIDPath string) string {
	const prefix = `\\?\volume{`
	value := strings.ToLower(strings.TrimSpace(volumeGUIDPath))
	value = strings.TrimSuffix(value, `\`)
	if !strings.HasPrefix(value, prefix) || !strings.HasSuffix(value, `}`) {
		return ""
	}
	guid := value[len(prefix) : len(value)-1]
	if !validWindowsVolumeGUID(guid) {
		return ""
	}
	return guid
}

func validWindowsVolumeGUID(guid string) bool {
	if len(guid) != 36 {
		return false
	}
	for index, character := range guid {
		switch index {
		case 8, 13, 18, 23:
			if character != '-' {
				return false
			}
		default:
			if !isLowercaseHexDigit(character) {
				return false
			}
		}
	}
	return true
}

func isLowercaseHexDigit(character rune) bool {
	return (character >= '0' && character <= '9') || (character >= 'a' && character <= 'f')
}
