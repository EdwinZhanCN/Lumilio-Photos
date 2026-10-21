package storage

import (
	"context"
	"fmt"
	"io/fs"
	"os"

	"github.com/google/uuid"
	"server/internal/storage/repocfg"
	"server/internal/storage/rootcfg"
)

// RepositoryLockProvider owns local lock acquisition and diagnostic inspection.
// Install alternatives before starting a manager, never while it is serving.
type RepositoryLockProvider interface {
	Acquire(context.Context, string, bool) (func(), error)
	Inspect(string, string) (RepositoryLockInfo, error)
}
type OSRepositoryLockProvider struct{}

func (OSRepositoryLockProvider) Acquire(ctx context.Context, lockPath string, exclusive bool) (func(), error) {
	return acquireLocalPathLock(ctx, lockPath, exclusive)
}
func (OSRepositoryLockProvider) Inspect(path, targetType string) (RepositoryLockInfo, error) {
	return inspectLocalRepositoryLock(path, targetType)
}

// RepositoryIdentityDetector keeps portable identity verification and native
// file/move identity behind one boundary. Reads remain rooted by RepositoryFS.
type RepositoryIdentityDetector interface {
	VerifyMarker([]byte, uuid.UUID) error
	LoadRepository(string) (*repocfg.RepositoryConfig, error)
	LoadLocation(string) (*rootcfg.RootConfig, error)
	FileIdentity(*os.File, fs.FileInfo) (*string, *string, *int64)
}
type LocalRepositoryIdentityDetector struct{}

// Path loaders intentionally preserve legacy parsing/error semantics. P2/P3
// will consume typed observations without treating unknown as positive absence.
func (LocalRepositoryIdentityDetector) LoadRepository(path string) (*repocfg.RepositoryConfig, error) {
	return repocfg.LoadConfigFromFile(path)
}
func (LocalRepositoryIdentityDetector) LoadLocation(path string) (*rootcfg.RootConfig, error) {
	return rootcfg.Load(path)
}
func (rm *DefaultRepositoryManager) identities() RepositoryIdentityDetector {
	if rm.files != nil {
		return rm.files.identities()
	}
	return LocalRepositoryIdentityDetector{}
}

func (LocalRepositoryIdentityDetector) VerifyMarker(data []byte, expected uuid.UUID) error {
	config, err := repocfg.ParseConfig(data)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrRepositoryMarkerInvalid, err)
	}
	id, err := uuid.Parse(config.ID)
	if err != nil {
		return fmt.Errorf("%w: invalid marker UUID: %v", ErrRepositoryMarkerInvalid, err)
	}
	if id != expected {
		return fmt.Errorf("%w: marker=%s catalog=%s", ErrRepositoryIDMismatch, id, expected)
	}
	return nil
}
func (LocalRepositoryIdentityDetector) FileIdentity(file *os.File, info fs.FileInfo) (*string, *string, *int64) {
	return platformFileIdentity(file, info)
}

// SetObservationBackend installs read-only facts for validation/assessment.
// Like constructor wiring, it must be called before serving requests.
func (rm *DefaultRepositoryManager) SetObservationBackend(observer StorageObserver) {
	rm.observer = observer
}
func (rm *DefaultRepositoryManager) SetLockProvider(provider RepositoryLockProvider) {
	rm.lockProvider = provider
}
func (rm *DefaultRepositoryManager) locks() RepositoryLockProvider {
	if rm.lockProvider != nil {
		return rm.lockProvider
	}
	return OSRepositoryLockProvider{}
}
func (f *RepositoryFSFactory) SetIdentityDetector(detector RepositoryIdentityDetector) {
	f.identity = detector
}
func (f *RepositoryFSFactory) identities() RepositoryIdentityDetector {
	if f.identity != nil {
		return f.identity
	}
	return LocalRepositoryIdentityDetector{}
}
func (r *RepositoryFS) identities() RepositoryIdentityDetector {
	if r.identity != nil {
		return r.identity
	}
	return LocalRepositoryIdentityDetector{}
}
