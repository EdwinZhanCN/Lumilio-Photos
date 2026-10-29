//go:build linux

package storage

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

func inspectVolume(path string) (uint64, uint64, string, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, 0, "", err
	}
	blockSize := uint64(stat.Bsize)
	filesystem, _ := linuxFilesystemForPath(path)
	return stat.Blocks * blockSize, stat.Bavail * blockSize, filesystem, nil
}

func linuxFilesystemForPath(path string) (string, error) {
	file, err := os.Open("/proc/self/mountinfo")
	if err != nil {
		return "", err
	}
	defer file.Close()
	return filesystemFromMountInfo(file, path)
}

// capacityGroupKeyForPath proves shared backing capacity from statfs. The
// filesystem ID identifies the mounted superblock, so bind mounts and repeated
// mounts of one filesystem agree while the Linux mount ID, which is
// namespace-local and changes across container recreation, is never used.
func capacityGroupKeyForPath(path string, filesystem string) string {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return ""
	}
	return capacityGroupKeyFromStatfsID(capacityGroupKindLinuxStatfs, linuxStatfsFilesystemID(stat.Fsid.X__val), filesystem)
}

// linuxStatfsFilesystemID encodes the two-word statfs filesystem ID. A zero ID
// cannot prove a shared pool, so it stays empty (unknown).
func linuxStatfsFilesystemID(fsid [2]int32) string {
	if fsid[0] == 0 && fsid[1] == 0 {
		return ""
	}
	return fmt.Sprintf("%08x%08x", uint32(fsid[0]), uint32(fsid[1]))
}

func inspectPathPlatform(path string) pathPlatformInfo {
	result := pathPlatformInfo{EffectiveUID: fmt.Sprint(os.Geteuid()), EffectiveGID: fmt.Sprint(os.Getegid())}
	var stat syscall.Stat_t
	if err := syscall.Stat(path, &stat); err == nil {
		result.Device = fmt.Sprintf("%d", stat.Dev)
		result.Inode = stat.Ino
	}
	file, err := os.Open("/proc/self/mountinfo")
	if err != nil {
		return result
	}
	defer file.Close()
	target, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return result
	}
	best := ""
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		separator := -1
		for index, field := range fields {
			if field == "-" {
				separator = index
				break
			}
		}
		if separator < 0 || separator+2 >= len(fields) || len(fields) < 5 {
			continue
		}
		mountPath := filepath.Clean(decodeMountInfoPath(fields[4]))
		inside := mountPath == string(filepath.Separator) || strings.HasPrefix(target, mountPath+string(filepath.Separator))
		if target != mountPath && !inside {
			continue
		}
		if len(mountPath) > len(best) {
			best = mountPath
			result.MountID = fields[0]
			result.MountSource = decodeMountInfoPath(fields[separator+2])
			if result.Device == "" {
				result.Device = fields[2]
			}
		}
	}
	result.MountPath = best
	return result
}

func platformPlaceholderUnavailable(string) (bool, error) { return false, nil }
