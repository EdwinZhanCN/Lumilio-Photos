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
	Writable          bool
	CapacityKnown     bool
	TotalBytes        uint64
	AvailableBytes    uint64
	Filesystem        string
	CanonicalPath     string
	MountID           string
	MountSource       string
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
	total, available, filesystem, err := inspectVolume(path)
	if err != nil {
		return info
	}
	info.CapacityKnown = true
	info.TotalBytes = total
	info.AvailableBytes = available
	info.Filesystem = filesystem
	info.CanonicalPath, _ = filepath.EvalSymlinks(path)
	if info.CanonicalPath == "" {
		info.CanonicalPath, _ = filepath.Abs(filepath.Clean(path))
	}
	platform := inspectPathPlatform(path)
	info.MountID = platform.MountID
	info.MountSource = platform.MountSource
	info.Device = platform.Device
	info.Inode = platform.Inode
	info.EffectiveUID = platform.EffectiveUID
	info.EffectiveGID = platform.EffectiveGID
	if probeMutability {
		info.CaseBehaviorKnown, info.CaseSensitive = inspectCaseBehavior(path)
	}
	info.NetworkFilesystem = isNetworkFilesystem(filesystem)
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

// StoragePlacementWarnings returns the complete set of non-fatal placement
// risks that require an administrator decision before a repository is created
// at path. Keep setup/status and the create mutation on this shared contract so
// first-run onboarding can present the same decision that Server enforces.
func StoragePlacementWarnings(path string) []string {
	info := InspectStoragePath(path)
	warnings := append(RepositoryRootWarnings(path), info.RiskWarnings...)
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
