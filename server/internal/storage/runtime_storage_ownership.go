package storage

import (
	"context"
	"fmt"
	"os"
	"sort"
	"time"

	"server/internal/db/dbtypes"
)

// AcquireRuntimeStorageOwnership claims every currently active portable root
// and repository for the lifetime of one Server generation. Future paths are
// claimed by lifecycle entry points before they become writable catalog state.
func (rm *DefaultRepositoryManager) AcquireRuntimeStorageOwnership(ctx context.Context) (func(), error) {
	rm.ownershipMu.Lock()
	if rm.ownershipOn {
		rm.ownershipMu.Unlock()
		return nil, fmt.Errorf("runtime storage ownership is already active")
	}
	rm.ownershipOn = true
	rm.ownership = make(map[string]func())
	rm.ownershipMu.Unlock()

	roots, err := rm.queries.ListRepositoryRoots(ctx)
	if err != nil {
		rm.releaseRuntimeStorageOwnership()
		return nil, fmt.Errorf("list Storage Locations for runtime ownership: %w", err)
	}
	repositories, err := rm.queries.ListRepositories(ctx)
	if err != nil {
		rm.releaseRuntimeStorageOwnership()
		return nil, fmt.Errorf("list repositories for runtime ownership: %w", err)
	}
	type target struct{ kind, path string }
	targets := make([]target, 0, len(roots)+len(repositories))
	for _, root := range roots {
		if root.Status == dbtypes.RepositoryRootStatusActive && existingDirectory(root.Path) {
			targets = append(targets, target{kind: "root", path: root.Path})
		}
	}
	for _, repository := range repositories {
		if repository.Reachability == dbtypes.RepositoryReachabilityActive && existingDirectory(repository.Path) {
			targets = append(targets, target{kind: "repository", path: repository.Path})
		}
	}
	sort.Slice(targets, func(i, j int) bool {
		if targets[i].path == targets[j].path {
			return targets[i].kind < targets[j].kind
		}
		return targets[i].path < targets[j].path
	})
	for _, target := range targets {
		if err := rm.claimRuntimeStoragePath(ctx, target.kind, target.path); err != nil {
			rm.releaseRuntimeStorageOwnership()
			return nil, err
		}
	}
	return rm.releaseRuntimeStorageOwnership, nil
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
	if kind == "root" {
		release, err = acquireRootPathLock(ctx, path, true)
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
