package storage

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

var ErrUnavailableCloudPlaceholder = errors.New("repository contains an unavailable cloud placeholder")

// StoragePathInfo describes the filesystem that actually backs one Storage
// Location. Capacity is inspected per path so Docker child mounts are never
// presented as though they shared the parent filesystem's free space.
type StoragePathInfo struct {
	Writable       bool
	CapacityKnown  bool
	TotalBytes     uint64
	AvailableBytes uint64
	Filesystem     string
	CanonicalPath  string
	// CapacityGroupKey identifies the backing capacity pool shared by every
	// path that reports the same key. It is sampled at CanonicalPath, the
	// filesystem that actually backs the requested path:
	//
	//   - Linux and Darwin: a nonzero statfs filesystem ID plus the filesystem
	//     type. Network and FUSE filesystems assign identities outside this
	//     host's control, so they stay empty (unknown).
	//   - Windows: the containing volume's GUID path plus serial number,
	//     resolved through GetVolumePathName so a volume mounted into an NTFS
	//     folder groups with its drive letter. Remote and unresolvable volumes
	//     stay empty.
	//
	// Empty means grouping is not proven. Empty is independent of
	// CapacityKnown: a proven group may have no figure, and a path with a
	// figure may have no proven group. The key is never MountID, a mount
	// fingerprint, a Storage Location id, or a user path. Callers must not
	// invent an aggregate total for an empty key; see
	// GroupCapacityByBackingStorage.
	CapacityGroupKey string
	MountID          string
	MountSource      string
	// MountPath is the directory where the backing filesystem is mounted, as
	// this host sees it: "/", "/Volumes/Backup", "/volume1", or `C:\`. It is
	// the human-facing storage identity an operator recognizes, and it is
	// deliberately independent of CapacityGroupKey: two paths can share one
	// mount path without a proof, and one proven pool can be reached through
	// several mount points. Empty means the mount could not be resolved.
	MountPath         string
	Device            string
	Inode             uint64
	EffectiveUID      string
	EffectiveGID      string
	CaseBehaviorKnown bool
	CaseSensitive     bool
	MountFingerprint  string
	NetworkFilesystem bool
	RemovableLikely   bool
	CloudSyncProvider string
	RiskWarnings      []string
}

func InspectStoragePath(path string) StoragePathInfo {
	return inspectStoragePath(path, true)
}

// InspectStoragePathReadOnly returns mount identity and classification without
// creating permission/case probe files. Observation change feeds use it so
// taking a cursor never generates its own filesystem event.
func InspectStoragePathReadOnly(path string) StoragePathInfo {
	return inspectStoragePath(path, false)
}

func inspectStoragePath(path string, probeMutability bool) StoragePathInfo {
	info := StoragePathInfo{}
	if probeMutability {
		info.Writable = directoryWritable(path)
	}
	// Sample at the canonical path so capacity, mount classification, and the
	// capacity group key all describe the filesystem that actually backs the
	// requested path instead of a symlink that merely names it.
	info.CanonicalPath = canonicalStoragePath(path)
	samplePath := path
	if info.CanonicalPath != "" {
		samplePath = info.CanonicalPath
	}
	total, available, filesystem, err := inspectVolume(samplePath)
	if err != nil {
		return info
	}
	info.CapacityKnown = true
	info.TotalBytes = total
	info.AvailableBytes = available
	info.Filesystem = filesystem
	platform := inspectPathPlatform(samplePath)
	info.MountID = platform.MountID
	info.MountSource = platform.MountSource
	info.MountPath = platform.MountPath
	info.Device = platform.Device
	info.Inode = platform.Inode
	info.EffectiveUID = platform.EffectiveUID
	info.EffectiveGID = platform.EffectiveGID
	if probeMutability {
		info.CaseBehaviorKnown, info.CaseSensitive = inspectCaseBehavior(path)
	}
	info.NetworkFilesystem = isNetworkFilesystem(filesystem)
	info.CapacityGroupKey = capacityGroupKeyForPath(samplePath, filesystem)
	clean := strings.ToLower(strings.ReplaceAll(info.CanonicalPath, `\`, "/"))
	info.RemovableLikely = strings.HasPrefix(clean, "/media/") || strings.HasPrefix(clean, "/run/media/") || strings.HasPrefix(clean, "/volumes/")
	info.CloudSyncProvider = cloudSyncProvider(info.CanonicalPath)
	fingerprint := sha256.Sum256([]byte(strings.Join([]string{platform.MountID, platform.MountSource, platform.Device, filesystem}, "\x00")))
	info.MountFingerprint = hex.EncodeToString(fingerprint[:])
	if info.NetworkFilesystem {
		info.RiskWarnings = append(info.RiskWarnings, "network_filesystem")
	}
	if info.RemovableLikely {
		info.RiskWarnings = append(info.RiskWarnings, "removable_storage")
	}
	if info.CloudSyncProvider != "" {
		info.RiskWarnings = append(info.RiskWarnings, "cloud_sync_directory")
	}
	return info
}

// canonicalStoragePath resolves symlinks so every fact about a path describes
// the filesystem that actually backs it. It falls back to a cleaned absolute
// path when the path cannot be resolved.
func canonicalStoragePath(path string) string {
	if canonical, err := filepath.EvalSymlinks(path); err == nil && canonical != "" {
		return canonical
	}
	canonical, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return ""
	}
	return canonical
}

// capacityGroupKeyForPath is implemented per platform (storage_path_info_*.go).
// It returns the proven shared-backing-capacity key for path, or "" when
// grouping is unknown. filesystem is the classification inspectVolume already
// resolved for the same path so the key and the reported Filesystem fact never
// disagree.

// StoragePlacementWarnings returns the complete set of non-fatal placement
// risks that require an administrator decision before a repository is created
// at path. Keep setup/status and the create mutation on this shared contract so
// first-run onboarding can present the same decision that Server enforces.
func StoragePlacementWarnings(path string) []string {
	info := InspectStoragePath(path)
	warnings := append(StorageLocationWarnings(path), info.RiskWarnings...)
	return uniqueStrings(warnings)
}

// filesystemFromMountInfo selects the longest mount-point match. This makes a
// repository on a Docker child mount use that mount's filesystem identity
// instead of inheriting the container parent mount's classification.
func filesystemFromMountInfo(reader io.Reader, path string) (string, error) {
	target, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return "", err
	}
	bestMount := ""
	bestFilesystem := ""
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		separator := -1
		for index, field := range fields {
			if field == "-" {
				separator = index
				break
			}
		}
		if separator < 0 || separator+1 >= len(fields) || len(fields) < 5 {
			continue
		}
		mountPath := filepath.Clean(decodeMountInfoPath(fields[4]))
		insideMount := mountPath == string(filepath.Separator) || strings.HasPrefix(target, mountPath+string(filepath.Separator))
		if target != mountPath && !insideMount {
			continue
		}
		if len(mountPath) > len(bestMount) {
			bestMount = mountPath
			bestFilesystem = fields[separator+1]
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("scan Linux mount information: %w", err)
	}
	return bestFilesystem, nil
}

type pathPlatformInfo struct {
	MountID      string
	MountSource  string
	MountPath    string
	Device       string
	Inode        uint64
	EffectiveUID string
	EffectiveGID string
}

func isNetworkFilesystem(filesystem string) bool {
	switch strings.ToLower(strings.TrimSpace(filesystem)) {
	case "nfs", "nfs4", "cifs", "smbfs", "sshfs", "9p", "ceph", "afs", "davfs", "fuse.sshfs":
		return true
	default:
		return false
	}
}

func directoryWritable(path string) bool {
	file, err := os.CreateTemp(path, ".lumilio_permission_test-*")
	if err != nil {
		return false
	}
	name := file.Name()
	if err := file.Close(); err != nil {
		_ = os.Remove(name)
		return false
	}
	return os.Remove(name) == nil
}

func inspectCaseBehavior(path string) (known bool, sensitive bool) {
	file, err := os.CreateTemp(path, ".lumilio_case_probe-a*")
	if err != nil {
		return false, false
	}
	name := file.Name()
	if closeErr := file.Close(); closeErr != nil {
		_ = os.Remove(name)
		return false, false
	}
	defer os.Remove(name)
	upperName := filepath.Join(filepath.Dir(name), strings.ToUpper(filepath.Base(name)))
	_, statErr := os.Stat(upperName)
	if statErr == nil {
		return true, false
	}
	if errors.Is(statErr, os.ErrNotExist) {
		return true, true
	}
	return false, false
}

func requireDirectoryWritable(path string) error {
	if !directoryWritable(path) {
		return fmt.Errorf("cannot write to directory %q", path)
	}
	return nil
}

// requireMaterializableRepository rejects provider placeholders before an
// existing Repository is registered. Directory enumeration does not open file
// contents or trigger a cloud download.
func requireMaterializableRepository(path string) error {
	return filepath.WalkDir(path, func(candidate string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("%w: inspect %q: %v", ErrUnavailableCloudPlaceholder, candidate, walkErr)
		}
		if entry.IsDir() {
			return nil
		}
		unavailable, err := platformPlaceholderUnavailable(candidate)
		if err != nil {
			return fmt.Errorf("%w: inspect placeholder attributes for %q: %v", ErrUnavailableCloudPlaceholder, candidate, err)
		}
		if unavailable {
			return fmt.Errorf("%w: %q must be materialized before opening", ErrUnavailableCloudPlaceholder, candidate)
		}
		name := strings.ToLower(entry.Name())
		if strings.HasPrefix(name, ".") && strings.HasSuffix(name, ".icloud") {
			return fmt.Errorf("%w: %q must be downloaded before opening", ErrUnavailableCloudPlaceholder, candidate)
		}
		return nil
	})
}
