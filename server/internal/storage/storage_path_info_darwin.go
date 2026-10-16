//go:build darwin

package storage

import (
	"fmt"
	"os"
	"syscall"
)

func inspectVolume(path string) (uint64, uint64, string, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, 0, "", err
	}
	blockSize := uint64(stat.Bsize)
	return stat.Blocks * blockSize, stat.Bavail * blockSize, int8CString(stat.Fstypename[:]), nil
}

func inspectPathPlatform(path string) pathPlatformInfo {
	result := pathPlatformInfo{EffectiveUID: fmt.Sprint(os.Geteuid()), EffectiveGID: fmt.Sprint(os.Getegid())}
	var stat syscall.Stat_t
	if err := syscall.Stat(path, &stat); err == nil {
		result.Device = fmt.Sprintf("%d", stat.Dev)
		result.Inode = stat.Ino
	}
	// statfs reports the mount point of the volume backing the path, which is
	// how macOS names storage to people: "/" for the boot volume and
	// "/Volumes/<name>" for anything else.
	var fsstat syscall.Statfs_t
	if err := syscall.Statfs(path, &fsstat); err == nil {
		result.MountPath = int8CString(fsstat.Mntonname[:])
	}
	return result
}

// capacityGroupKeyForPath proves shared backing capacity from statfs. The
// filesystem ID is per mounted volume; the macOS mount point name is never
// used because the same volume can be reached through firmlinks and several
// mount points.
func capacityGroupKeyForPath(path string, filesystem string) string {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return ""
	}
	return capacityGroupKeyFromStatfsID(capacityGroupKindDarwinStatfs, darwinStatfsFilesystemID(stat.Fsid.Val), filesystem)
}

// darwinStatfsFilesystemID encodes the two-word statfs filesystem ID. A zero
// ID cannot prove a shared pool, so it stays empty (unknown).
func darwinStatfsFilesystemID(fsid [2]int32) string {
	if fsid[0] == 0 && fsid[1] == 0 {
		return ""
	}
	return fmt.Sprintf("%08x%08x", uint32(fsid[0]), uint32(fsid[1]))
}

func platformPlaceholderUnavailable(path string) (bool, error) {
	var stat syscall.Stat_t
	if err := syscall.Lstat(path, &stat); err != nil {
		return false, err
	}
	// UF_OFFLINE: file contents are not resident and must be recalled.
	return stat.Flags&0x00000100 != 0, nil
}

func int8CString(value []int8) string {
	result := make([]byte, 0, len(value))
	for _, character := range value {
		if character == 0 {
			break
		}
		result = append(result, byte(character))
	}
	return string(result)
}
