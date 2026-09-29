package storage

import (
	"context"
	"fmt"
	"os"
	"sort"
	"time"

	"server/internal/db/dbtypes"
	"server/internal/db/repo"

	"go.uber.org/zap"
)

// AcquireRuntimeStorageOwnership claims reachable repositories independently.
// A Location is a registration scope, not a lifetime lock over its children.
// Future paths are claimed by lifecycle entry points before becoming writable.
func (rm *DefaultRepositoryManager) AcquireRuntimeStorageOwnership(ctx context.Context) (func(), error) {
	rm.ownershipMu.Lock()
	if rm.ownershipOn {
		rm.ownershipMu.Unlock()
		return nil, fmt.Errorf("runtime storage ownership is already active")
	}
	rm.ownershipOn = true
	rm.ownership = make(map[string]func())
	rm.ownershipMu.Unlock()

	repositories, err := rm.queries.ListRepositories(ctx)
	if err != nil {
		rm.releaseRuntimeStorageOwnership()
		return nil, fmt.Errorf("list repositories for runtime ownership: %w", err)
	}
	targets := make([]repo.Repository, 0, len(repositories))
	for _, repository := range repositories {
		if repository.Reachability == dbtypes.RepositoryReachabilityActive && existingDirectory(repository.Path) {
			targets = append(targets, repository)
		}
	}
	sort.Slice(targets, func(i, j int) bool {
		return targets[i].Path < targets[j].Path
	})
	for _, target := range targets {
		if err := ctx.Err(); err != nil {
			rm.releaseRuntimeStorageOwnership()
			return nil, err
		}
		if _, err := rm.tryClaimRuntimeRepository(ctx, target); err != nil {
			rm.releaseRuntimeStorageOwnership()
			return nil, err
		}
	}
	return rm.releaseRuntimeStorageOwnership, nil
}

// A cancelled attempt context makes the native lock adapter perform one
// immediate nonblocking attempt. An occupied disk never consumes the startup
// deadline needed to claim unrelated disks.
func (rm *DefaultRepositoryManager) tryClaimRuntimeRepository(ctx context.Context, repository repo.Repository) (bool, error) {
	probe, cancel := context.WithCancel(ctx)
	cancel()
	if err := rm.claimRuntimeStoragePath(probe, "repository", repository.Path); err != nil {
		rm.logger.Warn("repository ownership unavailable",
			zap.String("repository_id", repository.RepoID.String()), zap.Error(err))
		if _, updateErr := rm.queries.UpdateRepositoryReachability(ctx, repo.UpdateRepositoryReachabilityParams{
			RepoID: repository.RepoID, Reachability: dbtypes.RepositoryReachabilityOffline,
			UpdatedAt: dbtypes.NewTimestamp(time.Now().UTC()),
		}); updateErr != nil {
			return false, fmt.Errorf("record unavailable repository ownership: %w", updateErr)
		}
		return false, nil
	}
	return true, nil
}

func existingDirectory(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func (rm *DefaultRepositoryManager) claimRuntimeStoragePath(ctx context.Context, kind, path string) error {
	key := kind + ":" + path
	rm.ownershipMu.Lock()
	defer rm.ownershipMu.Unlock()
	if !rm.ownershipOn {
		return nil
	}
	if _, exists := rm.ownership[key]; exists {
		return nil
	}
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
	}
	var (
		release func()
		err     error
	)
	if kind == "storage_location" {
		release, err = acquireStorageLocationPathLock(ctx, path, true)
	} else {
		release, err = acquireRepositoryPathLock(ctx, path, true)
	}
	if err != nil {
		return fmt.Errorf("claim runtime %s ownership for %s: %w", kind, path, err)
	}
	rm.ownership[key] = release
	return nil
}

func (rm *DefaultRepositoryManager) releaseRuntimeStorageOwnership() {
	rm.ownershipMu.Lock()
	defer rm.ownershipMu.Unlock()
	keys := make([]string, 0, len(rm.ownership))
	for key := range rm.ownership {
		keys = append(keys, key)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(keys)))
	for _, key := range keys {
		rm.ownership[key]()
	}
	rm.ownership = nil
	rm.ownershipOn = false
}
