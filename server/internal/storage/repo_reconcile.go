package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/storage/repocfg"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"
)

// ReconcileAll re-checks every registered repository against what is actually on
// disk and brings the database in line with it. It runs at boot, when an
// external drive may have been unplugged, remounted, or replaced while the
// server was down.
//
// Two rules constrain what it may write:
//
//   - It only transitions within {active, offline, error} and never touches
//     scanning. repositories.status carries two orthogonal axes — activity and
//     reachability — in one column, so a scan interrupted by a restart would
//     otherwise be silently reclassified as idle. (Splitting reachability into
//     its own column is the cleaner fix if this keeps getting in the way.)
//   - The on-disk .lumiliorepo is authoritative for repository config; the DB
//     config column is a cache. Reconcile overwrites the cache from disk, so a
//     rename made while a repository was detached is not silently reverted.
func (rm *DefaultRepositoryManager) ReconcileAll(ctx context.Context) error {
	repositories, err := rm.queries.ListRepositories(ctx)
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
	if current.Status == dbtypes.RepoStatusScanning {
		return nil
	}

	status, config := rm.inspectRepositoryOnDisk(current)

	if status == dbtypes.RepoStatusActive && config != nil {
		if _, err := rm.refreshRepositoryConfigCache(ctx, current, config); err != nil {
			return fmt.Errorf("refresh config cache: %w", err)
		}
	}

	if current.Status == status {
		return nil
	}

	if _, err := rm.queries.UpdateRepositoryStatus(ctx, repo.UpdateRepositoryStatusParams{
		RepoID:    current.RepoID,
		Status:    status,
		UpdatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}); err != nil {
		return fmt.Errorf("update status: %w", err)
	}

	rm.logger.Info("repository reachability changed",
		zap.String("operation", "repository.reconcile"),
		zap.String("repository_path", current.Path),
		zap.String("from", string(current.Status)),
		zap.String("to", string(status)),
	)
	return nil
}

// inspectRepositoryOnDisk decides a repository's reachability from its recorded
// path. An unreadable path means offline — the drive is elsewhere, the data is
// not lost. A readable path holding a different or unparseable identity means
// error, which needs a human.
func (rm *DefaultRepositoryManager) inspectRepositoryOnDisk(current repo.Repository) (dbtypes.RepoStatus, *repocfg.RepositoryConfig) {
	if _, err := os.Stat(filepath.Join(current.Path, ".lumiliorepo")); err != nil {
		return dbtypes.RepoStatusOffline, nil
	}

	config, err := repocfg.LoadConfigFromFile(current.Path)
	if err != nil {
		return dbtypes.RepoStatusError, nil
	}

	if config.ID != repositoryIDString(current.RepoID) {
		return dbtypes.RepoStatusError, nil
	}

	return dbtypes.RepoStatusActive, config
}

func repositoryIDString(id pgtype.UUID) string {
	if !id.Valid {
		return ""
	}
	return uuid.UUID(id.Bytes).String()
}
