package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const repositoryLockMetadataOffset int64 = 1

var (
	ErrRepositoryLockUnavailable = errors.New("repository OS lock is unavailable")
	ErrRepositoryLockUnsupported = errors.New("filesystem does not provide supported repository lock semantics")
)

type repositoryOSLock struct {
	file *os.File
}

type repositoryLockMetadata struct {
	Holder     string    `json:"holder"`
	AcquiredAt time.Time `json:"acquired_at"`
}

type RepositoryLockInfo struct {
	Holder     string
	AcquiredAt time.Time
}

func acquireRepositoryPathLock(ctx context.Context, repositoryPath string, exclusive bool) (func(), error) {
	return acquirePathLock(ctx, filepath.Join(repositoryPath, ".lumiliorepo.lock"), exclusive)
}

func acquireRootPathLock(ctx context.Context, rootPath string, exclusive bool) (func(), error) {
	return acquirePathLock(ctx, filepath.Join(rootPath, ".lumilioroot.lock"), exclusive)
}

func acquireRepositoryPathLocks(ctx context.Context, paths []string, exclusive bool) (func(), error) {
	lockPaths := make([]string, 0, len(paths))
	for _, path := range paths {
		lockPaths = append(lockPaths, filepath.Join(path, ".lumiliorepo.lock"))
	}
	return acquirePathLocks(ctx, lockPaths, exclusive)
}

func acquireRootPathLocks(ctx context.Context, paths []string, exclusive bool) (func(), error) {
	lockPaths := make([]string, 0, len(paths))
	for _, path := range paths {
		lockPaths = append(lockPaths, filepath.Join(path, ".lumilioroot.lock"))
	}
	return acquirePathLocks(ctx, lockPaths, exclusive)
}

func acquirePathLocks(ctx context.Context, paths []string, exclusive bool) (func(), error) {
	ordered := append([]string(nil), paths...)
	sort.Strings(ordered)
	releases := make([]func(), 0, len(ordered))
	for index, path := range ordered {
		if index > 0 && path == ordered[index-1] {
			continue
		}
		release, err := acquirePathLock(ctx, path, exclusive)
		if err != nil {
			for i := len(releases) - 1; i >= 0; i-- {
				releases[i]()
			}
			return nil, err
		}
		releases = append(releases, release)
	}
	return func() {
		for i := len(releases) - 1; i >= 0; i-- {
			releases[i]()
		}
	}, nil
}

func acquirePathLock(ctx context.Context, lockPath string, exclusive bool) (func(), error) {
	if ctx == nil {
		return nil, fmt.Errorf("%w: context is required", ErrRepositoryLockUnavailable)
	}
	parent := filepath.Dir(lockPath)
	info := InspectStoragePath(parent)
	if networkFilesystem(info.Filesystem) {
		return nil, fmt.Errorf("%w: %s", ErrRepositoryLockUnsupported, info.Filesystem)
	}
	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("%w: open %s: %v", ErrRepositoryLockUnavailable, lockPath, err)
	}
	if err := platformAcquireFileLock(ctx, file, exclusive); err != nil {
		_ = file.Close()
		return nil, err
	}
	metadata := repositoryLockMetadata{Holder: fmt.Sprintf("pid:%d", os.Getpid()), AcquiredAt: time.Now().UTC()}
	encoded, err := json.Marshal(metadata)
	if err != nil {
		_ = platformReleaseFileLock(file)
		_ = file.Close()
		return nil, fmt.Errorf("%w: encode lock metadata: %v", ErrRepositoryLockUnavailable, err)
	}
	if err := file.Truncate(repositoryLockMetadataOffset); err == nil {
		_, err = file.WriteAt(encoded, repositoryLockMetadataOffset)
	}
	if err == nil {
		err = file.Sync()
	}
	if err != nil {
		_ = platformReleaseFileLock(file)
		_ = file.Close()
		return nil, fmt.Errorf("%w: persist lock metadata: %v", ErrRepositoryLockUnavailable, err)
	}
	lock := &repositoryOSLock{file: file}
	return func() { lock.release() }, nil
}

func InspectRepositoryLock(path, targetType string) (RepositoryLockInfo, error) {
	name := ".lumiliorepo.lock"
	if targetType == "storage_location" {
		name = ".lumilioroot.lock"
	}
	lockPath := filepath.Join(path, name)
	file, err := os.OpenFile(lockPath, os.O_RDWR, 0o600)
	if errors.Is(err, os.ErrNotExist) {
		return RepositoryLockInfo{}, nil
	}
	if err != nil {
		return RepositoryLockInfo{}, err
	}
	defer file.Close()

	// The lock file is durable evidence, not proof that a process still owns the
	// advisory lock. Probe the kernel lock before exposing its metadata so a
	// normal release or process crash cannot leave a phantom holder in
	// diagnostics. A pre-cancelled context keeps the probe non-blocking while
	// still allowing platformAcquireFileLock to make one immediate attempt.
	probeCtx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := platformAcquireFileLock(probeCtx, file, true); err == nil {
		_ = platformReleaseFileLock(file)
		return RepositoryLockInfo{}, nil
	} else if !errors.Is(err, ErrRepositoryLockUnavailable) {
		return RepositoryLockInfo{}, err
	}
	if _, err := file.Seek(repositoryLockMetadataOffset, 0); err != nil {
		return RepositoryLockInfo{}, err
	}
	data, err := io.ReadAll(file)
	if err != nil {
		return RepositoryLockInfo{}, err
	}
	var metadata repositoryLockMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		// Lock files written before metadata was moved away from the locked
		// byte remain readable after an upgrade.
		if _, seekErr := file.Seek(0, 0); seekErr != nil {
			return RepositoryLockInfo{}, seekErr
		}
		legacyData, readErr := io.ReadAll(file)
		if readErr != nil {
			return RepositoryLockInfo{}, readErr
		}
		if legacyErr := json.Unmarshal(legacyData, &metadata); legacyErr != nil {
			return RepositoryLockInfo{}, fmt.Errorf("decode repository lock metadata: %w", err)
		}
	}
	return RepositoryLockInfo{Holder: metadata.Holder, AcquiredAt: metadata.AcquiredAt}, nil
}

func (l *repositoryOSLock) release() {
	if l == nil || l.file == nil {
		return
	}
	_ = platformReleaseFileLock(l.file)
	_ = l.file.Close()
	l.file = nil
}

func networkFilesystem(filesystem string) bool {
	value := strings.ToLower(strings.TrimSpace(filesystem))
	for _, prefix := range []string{"nfs", "cifs", "smb", "afp", "sshfs", "fuse.sshfs", "9p", "webdav"} {
		if value == prefix || strings.HasPrefix(value, prefix+".") || strings.HasPrefix(value, prefix+"fs") || (prefix == "nfs" && strings.HasPrefix(value, "nfs")) {
			return true
		}
	}
	return false
}
