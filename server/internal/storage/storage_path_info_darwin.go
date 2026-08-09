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
	return result
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
