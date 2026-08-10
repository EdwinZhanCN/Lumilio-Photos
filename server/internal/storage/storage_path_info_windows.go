//go:build windows

package storage

import (
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

func inspectVolume(path string) (uint64, uint64, string, error) {
	directory, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, 0, "", err
	}
	var available uint64
	var total uint64
	if err := windows.GetDiskFreeSpaceEx(directory, &available, &total, nil); err != nil {
		return 0, 0, "", err
	}
	volume := filepath.VolumeName(path)
	if volume == "" {
		return total, available, "", nil
	}
	root := volume
	if !strings.HasSuffix(root, `\`) {
		root += `\`
	}
	rootName, err := windows.UTF16PtrFromString(root)
	if err != nil {
		return total, available, "", nil
	}
	filesystemBuffer := make([]uint16, 64)
	if err := windows.GetVolumeInformation(rootName, nil, 0, nil, nil, nil, &filesystemBuffer[0], uint32(len(filesystemBuffer))); err != nil {
		return total, available, "", nil
	}
	return total, available, windows.UTF16ToString(filesystemBuffer), nil
}

func inspectPathPlatform(path string) pathPlatformInfo {
	return pathPlatformInfo{Device: filepath.VolumeName(path), MountSource: filepath.VolumeName(path)}
}

func platformPlaceholderUnavailable(path string) (bool, error) {
	value, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return false, err
	}
	attributes, err := windows.GetFileAttributes(value)
	if err != nil {
		return false, err
	}
	placeholderAttributes := uint32(windows.FILE_ATTRIBUTE_OFFLINE | windows.FILE_ATTRIBUTE_RECALL_ON_OPEN | windows.FILE_ATTRIBUTE_RECALL_ON_DATA_ACCESS)
	return attributes&placeholderAttributes != 0, nil
}
