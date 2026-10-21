package storage

import (
	"context"
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

func (rm *DefaultRepositoryManager) reconcileRepository(ctx context.Context, current repo.Repository) error {
	releaseMutation := rm.acquireRepositoryMutation(current.RepoID)
	defer releaseMutation()
	if current.Reachability == dbtypes.RepositoryReachabilityMaintenance {
		return nil
	}
	// Re-read after acquiring the lifecycle lease: a relocation may have
	// committed while this pass waited for an earlier media handle.
	current, err := rm.readerQueries.GetRepository(ctx, current.RepoID)
	if err != nil {
		return err
	}
	if current.Reachability == dbtypes.RepositoryReachabilityMaintenance {
		return nil
	}

	reachability, config, known := rm.inspectRepositoryOnDisk(current)
	if !known {
		// Denied or unknown is not absence: the row keeps its reachability
		// until the path can be read again (R6, no false Missing).
		rm.logger.Warn("repository path could not be read; reachability left unchanged",
			zap.String("operation", "repository.reconcile"),
			zap.String("repository_path", current.Path))
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
