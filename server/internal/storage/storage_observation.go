package storage

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"server/internal/storage/marker"
	"server/internal/storage/repocfg"
	"server/internal/storage/rootcfg"
)

type AccessState string

const (
	AccessPresent          AccessState = "present"
	AccessAbsent           AccessState = "absent"
	AccessPermissionDenied AccessState = "permission_denied"
	AccessUnknown          AccessState = "unknown"
)

type AccessReading struct {
	State AccessState
	Info  fs.FileInfo
	Err   error
}

func ReadAccess(info fs.FileInfo, err error) AccessReading {
	state := AccessPresent
	switch {
	case errors.Is(err, fs.ErrPermission):
		state = AccessPermissionDenied
	case errors.Is(err, fs.ErrNotExist):
		state = AccessAbsent
	case err != nil:
		state = AccessUnknown
	case info == nil:
		state = AccessUnknown
		err = fmt.Errorf("stat returned nil FileInfo without an error")
	}
	return AccessReading{State: state, Info: info, Err: err}
}

func isPermissionError(errs ...error) bool {
	for _, err := range errs {
		if errors.Is(err, fs.ErrPermission) {
			return true
		}
	}
	return false
}

type VolumeFacts struct {
	Filesystem                 string
	TotalBytes, AvailableBytes uint64
	CapacityGroupKey           string
}
type MountFacts struct {
	Platform                                string
	MountID, MountSource, MountPath, Device string
	Inode                                   uint64
	Remote, Removable                       bool
	CloudProvider                           string
}

// StorageObserver exposes only read operations. It cannot create permission or
// case probes, acquire locks, reconcile catalog state, or expire host tasks.
type StorageObserver interface {
	marker.Reader
	Stat(string) (fs.FileInfo, error)
	ReadDir(string) ([]fs.DirEntry, error)
	EvalSymlinks(string) (string, error)
	StatFS(string) (VolumeFacts, error)
	MountInfo(string) (MountFacts, error)
	Placeholder(string) (bool, error)
}
type OSStorageObserver struct{}

func (OSStorageObserver) ReadFile(p string) ([]byte, error)       { return os.ReadFile(p) }
func (OSStorageObserver) Stat(p string) (fs.FileInfo, error)      { return os.Stat(p) }
func (OSStorageObserver) ReadDir(p string) ([]fs.DirEntry, error) { return os.ReadDir(p) }
func (OSStorageObserver) EvalSymlinks(p string) (string, error)   { return filepath.EvalSymlinks(p) }
func (OSStorageObserver) Placeholder(p string) (bool, error) {
	return platformPlaceholderUnavailable(p)
}
func (OSStorageObserver) StatFS(p string) (VolumeFacts, error) {
	total, available, filesystem, err := inspectVolume(p)
	return VolumeFacts{Filesystem: filesystem, TotalBytes: total, AvailableBytes: available, CapacityGroupKey: capacityGroupKeyForPath(p, filesystem)}, err
}
func (OSStorageObserver) MountInfo(p string) (MountFacts, error) {
	info := inspectPathPlatform(p)
	facts := MountFacts{Platform: runtime.GOOS, MountID: info.MountID, MountSource: info.MountSource, MountPath: info.MountPath, Device: info.Device, Inode: info.Inode, Remote: info.Remote, Removable: info.Removable, CloudProvider: cloudSyncProvider(p)}
	clean := strings.ToLower(strings.ReplaceAll(p, `\`, "/"))
	facts.Removable = facts.Removable || strings.HasPrefix(clean, "/media/") || strings.HasPrefix(clean, "/run/media/") || strings.HasPrefix(clean, "/volumes/")
	if info.ObservationErr != nil {
		return facts, info.ObservationErr
	}
	if info.MountPath == "" {
		return facts, fmt.Errorf("mount information unavailable for %q", p)
	}
	return facts, nil
}

type StorageObservation struct {
	Path, SamplePath string
	Access           AccessReading
	Volume           VolumeFacts
	Mount            MountFacts
	FactsErr         error
	RootMarker       marker.Reading[rootcfg.RootConfig]
	RepositoryMarker marker.Reading[repocfg.RepositoryConfig]
	Empty            bool
	DirectoryErr     error
	Placeholder      bool
	PlaceholderErr   error
	Layout           map[string]AccessReading
}

// ObserveStorageTarget samples an existing target on its own filesystem. Only
// positive absence permits walking upward to the nearest proven ancestor.
// Placeholder sampling is bounded to the target; deep traversal is preflight.
func ObserveStorageTarget(ctx context.Context, observer StorageObserver, target string) StorageObservation {
	result := StorageObservation{Path: filepath.Clean(target), Layout: map[string]AccessReading{}}
	if err := ctx.Err(); err != nil {
		result.Access = ReadAccess(nil, err)
		return result
	}
	result.Access = ReadAccess(observer.Stat(result.Path))
	result.RootMarker = rootcfg.ReadMarker(observer, result.Path)
	result.RepositoryMarker = repocfg.ReadMarker(observer, result.Path)
	sample := result.Path
	access := result.Access
	for access.State == AccessAbsent {
		parent := filepath.Dir(sample)
		if parent == sample {
			break
		}
		sample = parent
		if err := ctx.Err(); err != nil {
			result.FactsErr = err
			return result
		}
		access = ReadAccess(observer.Stat(sample))
	}
	if access.State != AccessPresent || !access.Info.IsDir() {
		result.FactsErr = access.Err
		if result.FactsErr == nil {
			result.FactsErr = fmt.Errorf("no proven containing directory")
		}
		return result
	}
	canonical, err := observer.EvalSymlinks(sample)
	if err != nil {
		result.FactsErr = err
		return result
	}
	result.SamplePath = canonical
	result.Volume, err = observer.StatFS(canonical)
	if err != nil {
		result.FactsErr = err
		return result
	}
	result.Mount, err = observer.MountInfo(canonical)
	if err != nil {
		result.FactsErr = err
		return result
	}
	if result.Access.State == AccessPresent {
		entries, err := observer.ReadDir(result.Path)
		result.DirectoryErr = err
		result.Empty = err == nil && len(entries) == 0
		result.Placeholder, result.PlaceholderErr = observer.Placeholder(result.Path)
		// Denial/unknown attributes are never a positive placeholder observation.
		if result.PlaceholderErr != nil {
			result.Placeholder = false
		}
		for _, dir := range []string{".lumilio", "inbox"} {
			result.Layout[dir] = ReadAccess(observer.Stat(filepath.Join(result.Path, dir)))
		}
	}
	return result
}
