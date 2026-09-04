package core

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"server/internal/db/dbtypes"
	"server/internal/db/repo"
)

// CheckpointStore persists agent checkpoints in the library catalog.
type CheckpointStore struct {
	q *repo.Queries
}

func NewCheckpointStore(q *repo.Queries) *CheckpointStore {
	return &CheckpointStore{q: q}
}

func (s *CheckpointStore) Get(ctx context.Context, key string) ([]byte, bool, error) {
	data, err := s.q.GetCheckpoint(ctx, key)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("get checkpoint: %w", err)
	}
	return data, true, nil
}

func (s *CheckpointStore) Set(ctx context.Context, key string, data []byte) error {
	err := s.q.UpsertCheckpoint(ctx, repo.UpsertCheckpointParams{
		ID:        key,
		Data:      data,
		UpdatedAt: dbtypes.NewTimestamp(time.Now()),
	})
	if err != nil {
		return fmt.Errorf("save checkpoint: %w", err)
	}
	return nil
}

func (s *CheckpointStore) Delete(ctx context.Context, key string) error {
	if err := s.q.DeleteCheckpoint(ctx, key); err != nil {
		return fmt.Errorf("delete checkpoint: %w", err)
	}
	return nil
}
