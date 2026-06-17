package storage

import (
	"context"
	"fmt"

	"server/internal/db/repo"
)

// RepoDefaults is the storage-owned, runtime-mutable behaviour applied to newly
// created repositories. The default root is the immutable storage root (config),
// so it is not part of these defaults.
type RepoDefaults struct {
	Strategy          string
	DuplicateHandling string
}

// GetRepositoryDefaults returns the current repository defaults (single row,
// seeded by migration).
func (rm *DefaultRepositoryManager) GetRepositoryDefaults(ctx context.Context) (RepoDefaults, error) {
	row, err := rm.queries.GetRepositoryDefaults(ctx)
	if err != nil {
		return RepoDefaults{}, fmt.Errorf("get repository defaults: %w", err)
	}
	return RepoDefaults{Strategy: row.Strategy, DuplicateHandling: row.DuplicateHandling}, nil
}

// UpdateRepositoryDefaults persists new repository defaults and returns them.
func (rm *DefaultRepositoryManager) UpdateRepositoryDefaults(ctx context.Context, defaults RepoDefaults) (RepoDefaults, error) {
	row, err := rm.queries.UpsertRepositoryDefaults(ctx, repo.UpsertRepositoryDefaultsParams{
		Strategy:          defaults.Strategy,
		DuplicateHandling: defaults.DuplicateHandling,
	})
	if err != nil {
		return RepoDefaults{}, fmt.Errorf("update repository defaults: %w", err)
	}
	return RepoDefaults{Strategy: row.Strategy, DuplicateHandling: row.DuplicateHandling}, nil
}
