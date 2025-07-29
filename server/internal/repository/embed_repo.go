package repository

import (
	"context"
	"github.com/google/uuid"
)

type EmbeddingRepository interface {
	// UpsertEmbedding 保存或更新某个 Asset 的 embedding 向量
	UpsertEmbedding(ctx context.Context, assetID uuid.UUID, embedding []float32) error

	// SearchNearest 根据向量，做 k-NN 检索，返回最近的 assetIDs
	SearchNearest(ctx context.Context, query []float32, k int) ([]uuid.UUID, error)
}
