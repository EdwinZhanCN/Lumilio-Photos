package core

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"server/internal/db/repo" // 假设 sqlc 生成的代码在这个包
)

// PostgresStore 实现 compose.CheckPointStore
type PostgresStore struct {
	q *repo.Queries // sqlc 生成的 queries 对象
}

func NewPostgresStore(q *repo.Queries) *PostgresStore {
	return &PostgresStore{q: q}
}

// Get 实现接口：读取快照
func (s *PostgresStore) Get(ctx context.Context, key string) ([]byte, bool, error) {
	data, err := s.q.GetCheckpoint(ctx, key)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, false, nil // 没找到不报错，返回 exist=false
		}
		return nil, false, fmt.Errorf("db get checkpoint error: %w", err)
	}
	return data, true, nil
}

// Set 实现接口：保存快照
func (s *PostgresStore) Set(ctx context.Context, key string, data []byte) error {
	err := s.q.UpsertCheckpoint(ctx, repo.UpsertCheckpointParams{
		ID:   key,
		Data: data,
	})
	if err != nil {
		return fmt.Errorf("db save checkpoint error: %w", err)
	}
	return nil
}
