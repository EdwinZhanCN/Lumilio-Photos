//go:build windows

package storage

import (
	"path/filepath"

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
	// Resolve the volume that actually contains path. filepath.VolumeName only
	// returns the access-path drive letter, which is wrong for a volume mounted
	// into an NTFS folder.
	root, err := windowsVolumeRootPointer(path)
	if err != nil {
		return total, available, "", nil
	}
	filesystemBuffer := make([]uint16, 64)
	if err := windows.GetVolumeInformation(root, nil, 0, nil, nil, nil, &filesystemBuffer[0], uint32(len(filesystemBuffer))); err != nil {
		return total, available, "", nil
	}
	return total, available, windows.UTF16ToString(filesystemBuffer), nil
}

func inspectPathPlatform(path string) pathPlatformInfo {
	// MountFingerprint inputs stay unchanged: the drive letter is the existing
	// change-detection fact, and the capacity group key is a separate proof.
	result := pathPlatformInfo{Device: filepath.VolumeName(path), MountSource: filepath.VolumeName(path)}
	// GetVolumePathName resolves the volume that contains path to its mount
	// point: `C:\` for a drive letter, or the folder when a volume is mounted
	// into an NTFS directory.
	if mountPath, err := windowsVolumeRootPath(path); err == nil {
		result.MountPath = mountPath
	}
	return result
}

// capacityGroupKeyForPath proves shared backing capacity from the containing
// volume's GUID path and serial number. GetVolumePathName resolves a volume
// mounted into an NTFS folder to that folder's mount point, so C:\Data and D:\
// produce the same key when C:\Data is D:'s mount point. Remote, unknown, and
// unresolvable volumes stay empty (unknown).
func capacityGroupKeyForPath(path string, filesystem string) string {
	root, err := windowsVolumeRootPointer(path)
	if err != nil {
		return ""
	}
	driveType := windows.GetDriveType(root)
	volumeName, err := windowsVolumeGUIDPath(root)
	if err != nil {
		return ""
	}
	serial, err := windowsVolumeSerialNumber(root)
	if err != nil {
		return ""
	}
	return windowsCapacityGroupKey(filesystem, volumeName, serial, driveType)
}

// windowsVolumeRootPath returns the volume mount point that contains path,
// for example C:\ or C:\Data\ for a folder-mounted volume.
func windowsVolumeRootPath(path string) (string, error) {
	pathPointer, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return "", err
	}
	buffer := make([]uint16, windows.MAX_PATH+1)
	if err := windows.GetVolumePathName(pathPointer, &buffer[0], uint32(len(buffer))); err != nil {
		return "", err
	}
	return windows.UTF16ToString(buffer), nil
}

func windowsVolumeRootPointer(path string) (*uint16, error) {
	mountPath, err := windowsVolumeRootPath(path)
	if err != nil {
		return nil, err
	}
	return windows.UTF16PtrFromString(mountPath)
}

func windowsVolumeGUIDPath(root *uint16) (string, error) {
	buffer := make([]uint16, windows.MAX_PATH+1)
	if err := windows.GetVolumeNameForVolumeMountPoint(root, &buffer[0], uint32(len(buffer))); err != nil {
		return "", err
	}
	return windows.UTF16ToString(buffer), nil
}

func windowsVolumeSerialNumber(root *uint16) (uint32, error) {
	var serial uint32
	if err := windows.GetVolumeInformation(root, nil, 0, &serial, nil, nil, nil, 0); err != nil {
		return 0, err
	}
	return serial, nil
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
