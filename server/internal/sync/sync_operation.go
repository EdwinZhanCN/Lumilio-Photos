package sync

import (
	"context"
	"fmt"
	"time"

	"server/internal/db/repo"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// SyncOperationStore handles database operations for sync operations using sqlc
type SyncOperationStore struct {
	queries *repo.Queries
}

// NewSyncOperationStore creates a new sync operation store
func NewSyncOperationStore(queries *repo.Queries) *SyncOperationStore {
	return &SyncOperationStore{queries: queries}
}

// SyncStats holds statistics for a sync operation
type SyncStats struct {
	FilesScanned int
	FilesAdded   int
	FilesUpdated int
	FilesRemoved int
}

// CreateSyncOperation creates a new sync operation record
func (s *SyncOperationStore) CreateSyncOperation(ctx context.Context, repoID uuid.UUID, operationType string, startTime time.Time) (*repo.SyncOperation, error) {
	status := "running"
	filesScanned := int32(0)
	filesAdded := int32(0)
	filesUpdated := int32(0)
	filesRemoved := int32(0)

	params := repo.CreateSyncOperationParams{
		RepositoryID:  pgtype.UUID{Bytes: repoID, Valid: true},
		OperationType: operationType,
		FilesScanned:  &filesScanned,
		FilesAdded:    &filesAdded,
		FilesUpdated:  &filesUpdated,
		FilesRemoved:  &filesRemoved,
		StartTime:     pgtype.Timestamptz{Time: startTime, Valid: true},
		EndTime:       pgtype.Timestamptz{Valid: false},
		DurationMs:    nil,
		Status:        &status,
		ErrorMessage:  nil,
	}

	operation, err := s.queries.CreateSyncOperation(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to create sync operation: %w", err)
	}

	return &operation, nil
}

// UpdateSyncOperation updates an existing sync operation
func (s *SyncOperationStore) UpdateSyncOperation(ctx context.Context, opID int64, stats SyncStats, endTime time.Time, durationMs int64, status string, errorMessage *string) error {
	filesScanned := int32(stats.FilesScanned)
	filesAdded := int32(stats.FilesAdded)
	filesUpdated := int32(stats.FilesUpdated)
	filesRemoved := int32(stats.FilesRemoved)

	params := repo.UpdateSyncOperationParams{
		ID:           opID,
		FilesScanned: &filesScanned,
		FilesAdded:   &filesAdded,
		FilesUpdated: &filesUpdated,
		FilesRemoved: &filesRemoved,
		EndTime:      pgtype.Timestamptz{Time: endTime, Valid: true},
		DurationMs:   &durationMs,
		Status:       &status,
		ErrorMessage: errorMessage,
	}

	_, err := s.queries.UpdateSyncOperation(ctx, params)
	if err != nil {
		return fmt.Errorf("failed to update sync operation: %w", err)
	}

	return nil
}

// GetSyncOperation retrieves a sync operation by ID
func (s *SyncOperationStore) GetSyncOperation(ctx context.Context, id int64) (*repo.SyncOperation, error) {
	operation, err := s.queries.GetSyncOperation(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("sync operation not found: %w", err)
	}

	return &operation, nil
}

// ListSyncOperations returns recent sync operations for a repository
func (s *SyncOperationStore) ListSyncOperations(ctx context.Context, repoID uuid.UUID, limit int) ([]repo.SyncOperation, error) {
	params := repo.ListSyncOperationsParams{
		RepositoryID: pgtype.UUID{Bytes: repoID, Valid: true},
		Limit:        int32(limit),
	}

	operations, err := s.queries.ListSyncOperations(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to list sync operations: %w", err)
	}

	return operations, nil
}

// GetLatestSyncOperation returns the most recent sync operation for a repository
func (s *SyncOperationStore) GetLatestSyncOperation(ctx context.Context, repoID uuid.UUID) (*repo.SyncOperation, error) {
	operation, err := s.queries.GetLatestSyncOperation(ctx, pgtype.UUID{Bytes: repoID, Valid: true})
	if err != nil {
		// Return nil if no operations exist yet (not an error)
		return nil, nil
	}

	return &operation, nil
}

// GetRunningSyncOperations returns all running sync operations for a repository
func (s *SyncOperationStore) GetRunningSyncOperations(ctx context.Context, repoID uuid.UUID) ([]repo.SyncOperation, error) {
	operations, err := s.queries.GetRunningSyncOperations(ctx, pgtype.UUID{Bytes: repoID, Valid: true})
	if err != nil {
		return nil, fmt.Errorf("failed to get running sync operations: %w", err)
	}

	return operations, nil
}

// GetFailedSyncOperations returns recent failed sync operations
func (s *SyncOperationStore) GetFailedSyncOperations(ctx context.Context, repoID uuid.UUID, limit int) ([]repo.SyncOperation, error) {
	params := repo.GetFailedSyncOperationsParams{
		RepositoryID: pgtype.UUID{Bytes: repoID, Valid: true},
		Limit:        int32(limit),
	}

	operations, err := s.queries.GetFailedSyncOperations(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to get failed sync operations: %w", err)
	}

	return operations, nil
}

// CountSyncOperationsByStatus returns count of operations by status
func (s *SyncOperationStore) CountSyncOperationsByStatus(ctx context.Context, repoID uuid.UUID, status string) (int64, error) {
	params := repo.CountSyncOperationsByStatusParams{
		RepositoryID: pgtype.UUID{Bytes: repoID, Valid: true},
		Status:       &status,
	}

	count, err := s.queries.CountSyncOperationsByStatus(ctx, params)
	if err != nil {
		return 0, fmt.Errorf("failed to count sync operations: %w", err)
	}

	return count, nil
}
