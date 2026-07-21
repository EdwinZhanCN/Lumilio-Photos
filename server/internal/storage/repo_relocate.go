package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/storage/repocfg"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"
)

var (
	// ErrRepositoryOffline reports that a repository's on-disk location is not
	// currently reachable — an unplugged external drive is the ordinary case.
	// Callers must distinguish this from "the data is gone".
	ErrRepositoryOffline = errors.New("repository is offline")

	// ErrRepositoryIDMismatch reports that the .lumiliorepo at a path belongs to
	// a different repository than the one being relocated.
	ErrRepositoryIDMismatch = errors.New("repository ID at path does not match")
)

// RepositoryConflictError reports that the repository being registered carries
// an ID that is already registered at another path.
//
// This is not an error condition the server can resolve on its own. The obvious
// automatic check — "is the old path still a valid repository with this ID?" —
// gives the wrong answer in the ordinary external-drive sequence: the drive is
// unplugged, the user registers a copy, and later the original drive returns.
// Both locations are then real, and only the user knows which one is the
// library. The caller must present the choice: relocate the existing
// repository, or register this path as a new, independent copy.
type RepositoryConflictError struct {
	RepositoryID   string
	RegisteredPath string
	RequestedPath  string
}

func (e *RepositoryConflictError) Error() string {
	return fmt.Sprintf("repository %s is already registered at %s", e.RepositoryID, e.RegisteredPath)
}

// RelocateRepository points an existing repository at a new on-disk location.
//
// Assets are untouched by construction: assets.storage_path is
// repository-relative (UNIQUE (repository_id, storage_path)), so every consumer
// re-derives absolute paths from repositories.path. Relocate is one UPDATE.
func (rm *DefaultRepositoryManager) RelocateRepository(ctx context.Context, id string, newPath string) (*repo.Repository, error) {
	repoUUID, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("invalid repository ID: %w", err)
	}

	cleanPath, err := CanonicalizeRepositoryPath(newPath)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}

	if _, err := rm.GetRepository(id); err != nil {
		return nil, err
	}

	result, err := rm.validateRepository(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("failed to validate repository: %w", err)
	}
	if !result.Valid {
		return nil, fmt.Errorf("invalid repository at %s: %v", cleanPath, result.Errors)
	}

	config, err := repocfg.LoadConfigFromFile(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load repository configuration: %w", err)
	}
	if config.ID != id {
		return nil, fmt.Errorf("%w: %s holds repository %s, not %s",
			ErrRepositoryIDMismatch, cleanPath, config.ID, id)
	}

	now := time.Now()
	dbRepo, err := rm.queries.UpdateRepositoryPath(ctx, repo.UpdateRepositoryPathParams{
		RepoID:    pgtype.UUID{Bytes: repoUUID, Valid: true},
		Path:      cleanPath,
		Status:    dbtypes.RepoStatusActive,
		UpdatedAt: pgtype.Timestamptz{Time: now, Valid: true},
	})
	if err != nil {
		if isUniquePathViolation(err) {
			rm.repoAudit(cleanPath).Error("repository.relocate", err, zap.String("repository_id", id))
			return nil, fmt.Errorf("%w: %s", ErrRepositoryExistsAtPath, cleanPath)
		}
		rm.repoAudit(cleanPath).Error("repository.relocate", err, zap.String("repository_id", id))
		return nil, fmt.Errorf("failed to relocate repository: %w", err)
	}

	// The path is now correct and durable. Refreshing the DB's cached copy of
	// the on-disk config is a separate, recoverable step: if it fails the
	// repository is still correctly located and the next boot reconcile will
	// bring the cache forward.
	if refreshed, err := rm.refreshRepositoryConfigCache(ctx, dbRepo, config); err != nil {
		rm.logger.Warn("relocated repository but failed to refresh config cache",
			zap.String("operation", "repository.relocate"),
			zap.String("repository_id", id),
			zap.Error(err))
	} else {
		dbRepo = *refreshed
	}

	rm.repoAudit(cleanPath).Operation("repository.relocate",
		zap.String("repository_id", id),
		zap.String("repository_path", cleanPath),
	)
	rm.logger.Info("repository relocated",
		zap.String("operation", "repository.relocate"),
		zap.String("repository_id", id),
		zap.String("repository_path", cleanPath),
	)

	return &dbRepo, nil
}

// RegisterRepositoryCopy registers a duplicated repository directory as a new,
// independent repository by minting a fresh UUID into its .lumiliorepo. This is
// the `git clone` answer to a same-ID conflict, and it turns a dead-end error
// into an action the user can take.
func (rm *DefaultRepositoryManager) RegisterRepositoryCopy(ctx context.Context, path string, defaultOwnerID *int32, role dbtypes.RepoRole) (*repo.Repository, error) {
	cleanPath, err := CanonicalizeRepositoryPath(path)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}

	config, err := repocfg.LoadConfigFromFile(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load repository configuration: %w", err)
	}

	previousID := config.ID
	config.ID = uuid.New().String()
	if err := config.SaveConfigToFile(cleanPath); err != nil {
		return nil, fmt.Errorf("failed to write new repository identity: %w", err)
	}

	dbRepo, err := rm.AddRepository(cleanPath, defaultOwnerID, role)
	if err != nil {
		// Put the original identity back so the directory is not left with an
		// orphaned UUID that belongs to no database row.
		config.ID = previousID
		if restoreErr := config.SaveConfigToFile(cleanPath); restoreErr != nil {
			rm.logger.Error("failed to restore repository identity after failed copy registration",
				zap.String("operation", "repository.register_copy"),
				zap.String("repository_path", cleanPath),
				zap.Error(restoreErr))
		}
		return nil, err
	}

	rm.repoAudit(cleanPath).Operation("repository.register_copy",
		zap.String("repository_id", config.ID),
		zap.String("copied_from_repository_id", previousID),
	)

	return dbRepo, nil
}

// refreshRepositoryConfigCache writes the on-disk config into the database's
// cached copy. Disk is authoritative; the DB column is a cache.
func (rm *DefaultRepositoryManager) refreshRepositoryConfigCache(ctx context.Context, current repo.Repository, config *repocfg.RepositoryConfig) (*repo.Repository, error) {
	updated, err := rm.queries.UpdateRepository(ctx, repo.UpdateRepositoryParams{
		RepoID:         current.RepoID,
		Name:           config.Name,
		Config:         *config,
		DefaultOwnerID: current.DefaultOwnerID,
		UpdatedAt:      pgtype.Timestamptz{Time: time.Now(), Valid: true},
	})
	if err != nil {
		return nil, err
	}
	return &updated, nil
}

func isUniquePathViolation(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	return pgErr.Code == "23505" && pgErr.ConstraintName == "repositories_path_key"
}
