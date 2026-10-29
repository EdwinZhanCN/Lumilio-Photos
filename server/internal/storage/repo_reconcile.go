package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/storage/repocfg"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ReconcileAll re-checks every registered repository against what is actually on
// disk and brings the database in line with it. It runs at boot, when an
// external drive may have been unplugged, remounted, or replaced while the
// server was down.
//
// Two rules constrain what it may write:
//
//   - It only updates reachability and never changes repository activity.
//     A maintenance operation owns the reachability barrier until it commits or
//     rolls back, so ordinary reconciliation skips that state.
//   - The on-disk .lumiliorepo is authoritative for repository config; the DB
//     config column is a cache. Reconcile overwrites the cache from disk, so a
//     rename made while a repository was detached is not silently reverted.
func (rm *DefaultRepositoryManager) ReconcileAll(ctx context.Context) error {
	repositories, err := rm.readerQueries.ListRepositories(ctx)
	if err != nil {
		return fmt.Errorf("list repositories: %w", err)
	}

	for i := range repositories {
		if err := rm.reconcileRepository(ctx, repositories[i]); err != nil {
			// One unreachable repository must not stop the others from being
			// reconciled, and must not stop the server from booting.
			rm.logger.Warn("failed to reconcile repository",
				zap.String("operation", "repository.reconcile"),
				zap.String("repository_path", repositories[i].Path),
				zap.Error(err))
		}
	}
	return nil
}

// reconcileWait bounds one repository's reconciliation. The pass is a
// background backstop that retries on the next tick, so it gives up on a busy
// repository instead of holding its lifecycle lease while it waits for
// SQLite's writer, which may itself be waiting for a media handle.
const reconcileWait = 10 * time.Second

func (rm *DefaultRepositoryManager) reconcileRepository(ctx context.Context, current repo.Repository) error {
	if current.Reachability == dbtypes.RepositoryReachabilityMaintenance {
		return nil
	}
	// The steady state changes nothing, so it is decided without the lifecycle
	// lease. Taking that exclusive lease on every tick queued a writer ahead of
	// every new media handle, and deadlocked against a handle that needed a
	// second lease or SQLite's writer before it could close.
	reachability, config, known := rm.inspectRepositoryOnDisk(current)
	if !known {
		rm.logUnreadableRepository(current)
		return nil
	}
	if rm.reconcileLeavesRowUnchanged(current, reachability, config) {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, reconcileWait)
	defer cancel()
	releaseMutation, err := rm.acquireRepositoryMutationContext(ctx, current.RepoID)
	if err != nil {
		return fmt.Errorf("repository is busy; retrying on the next pass: %w", err)
	}
	defer releaseMutation()
	// Re-read after acquiring the lifecycle lease: a relocation may have
	// committed while this pass waited for an earlier media handle.
	current, err = rm.readerQueries.GetRepository(ctx, current.RepoID)
	if err != nil {
		return err
	}
	if current.Reachability == dbtypes.RepositoryReachabilityMaintenance {
		return nil
	}

	reachability, config, known = rm.inspectRepositoryOnDisk(current)
	if !known {
		rm.logUnreadableRepository(current)
		return nil
	}

	if reachability == dbtypes.RepositoryReachabilityActive && config != nil {
		claimed, err := rm.tryClaimRuntimeRepository(ctx, current)
		if err != nil || !claimed {
			return err
		}
		if _, err := rm.refreshRepositoryConfigCache(ctx, current, config); err != nil {
			return fmt.Errorf("refresh config cache: %w", err)
		}
	}

	if current.Reachability == reachability {
		return nil
	}

	if _, err := rm.queries.UpdateRepositoryReachability(ctx, repo.UpdateRepositoryReachabilityParams{
		RepoID:       current.RepoID,
		Reachability: reachability,
		UpdatedAt:    dbtypes.NewTimestamp(time.Now()),
	}); err != nil {
		return fmt.Errorf("update reachability: %w", err)
	}

	rm.logger.Info("repository reachability changed",
		zap.String("operation", "repository.reconcile"),
		zap.String("repository_path", current.Path),
		zap.String("from", string(current.Reachability)),
		zap.String("to", string(reachability)),
	)
	return nil
}

// Denied or unknown is not absence: the row keeps its reachability until the
// path can be read again (R6, no false Missing).
func (rm *DefaultRepositoryManager) logUnreadableRepository(current repo.Repository) {
	rm.logger.Warn("repository path could not be read; reachability left unchanged",
		zap.String("operation", "repository.reconcile"),
		zap.String("repository_path", current.Path))
}

// reconcileLeavesRowUnchanged reports whether reconciliation would write
// nothing: the disk agrees with the row's reachability, the config cache is
// current, and this process already owns an active repository.
func (rm *DefaultRepositoryManager) reconcileLeavesRowUnchanged(current repo.Repository, reachability dbtypes.RepositoryReachability, config *repocfg.RepositoryConfig) bool {
	if reachability != current.Reachability {
		return false
	}
	if reachability != dbtypes.RepositoryReachabilityActive || config == nil {
		return true
	}
	return repositoryConfigCacheCurrent(current, config) && rm.holdsRuntimeOwnership("repository", current.Path)
}

func repositoryConfigCacheCurrent(current repo.Repository, config *repocfg.RepositoryConfig) bool {
	if current.Name != config.Name {
		return false
	}
	cached, err := json.Marshal(current.Config)
	if err != nil {
		return false
	}
	onDisk, err := json.Marshal(config)
	if err != nil {
		return false
	}
	return bytes.Equal(cached, onDisk)
}

// inspectRepositoryOnDisk decides a repository's reachability from its recorded
// path. A positively absent path means offline — the drive is elsewhere, the
// data is not lost. A readable path holding a different or unparseable
// identity means error, which needs a human. A path that cannot be read at
// all (permission denied, an unknown failure) decides nothing: known is
// false and the caller keeps the current reachability.
func (rm *DefaultRepositoryManager) inspectRepositoryOnDisk(current repo.Repository) (dbtypes.RepositoryReachability, *repocfg.RepositoryConfig, bool) {
	switch rm.repositoryOriginalState(current.Path, repositoryIDString(current.RepoID)) {
	case OriginalAbsent:
		return dbtypes.RepositoryReachabilityOffline, nil, true
	case OriginalIdentityDiffers:
		return dbtypes.RepositoryReachabilityIdentityError, nil, true
	case OriginalInaccessible:
		return current.Reachability, nil, false
	}
	config, err := rm.identities().LoadRepository(current.Path)
	if err != nil {
		return dbtypes.RepositoryReachabilityIdentityError, nil, true
	}
	if config.ID != repositoryIDString(current.RepoID) {
		return dbtypes.RepositoryReachabilityIdentityError, nil, true
	}
	return dbtypes.RepositoryReachabilityActive, config, true
}

func repositoryIDString(id uuid.UUID) string {
	return id.String()
}
