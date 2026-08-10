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
	return result
}

func platformPlaceholderUnavailable(string) (bool, error) { return false, nil }
