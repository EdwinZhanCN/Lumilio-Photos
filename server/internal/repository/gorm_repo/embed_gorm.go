package gorm_repo

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/pgvector/pgvector-go"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"math"
	"server/internal/models"
	"server/internal/repository"
)

type embedRepo struct {
	db *gorm.DB
}

func NewEmbedRepository(db *gorm.DB) repository.EmbeddingRepository {
	return &embedRepo{db: db}
}

// UpsertEmbedding 插入或更新 embedding 列
func (r *embedRepo) UpsertEmbedding(ctx context.Context, assetID uuid.UUID, embedding []float32) error {
	vec := pgvector.NewVector(embedding) // []float32 -> pgvector.Vector
	return r.db.WithContext(ctx).
		Model(&models.Asset{}).
		Where("asset_id = ?", assetID).
		Update("embedding", vec).
		Error
}

// SearchNearest 使用 pgvector 的 <-> 操作做 L2 最近邻搜索
// TODO: 改成你实际的维度
const embedDim = 768 // 改成你实际的维度

func (r *embedRepo) SearchNearest(ctx context.Context, query []float32, k int) ([]uuid.UUID, error) {
	if len(query) != embedDim {
		return nil, fmt.Errorf("query dim %d mismatch, expect %d", len(query), embedDim)
	}

	vec := pgvector.NewVector(query)

	tx := r.db.WithContext(ctx)

	// hnsw.ef_search 至少 >= k，简单点设成 2*k
	if err := tx.Exec("SET LOCAL hnsw.ef_search = ?", int(math.Max(float64(2*k), 40))).Error; err != nil {
		return nil, err
	}

	var ids []uuid.UUID
	err := tx.
		Model(&models.Asset{}).
		Select("asset_id").
		Order(clause.Expr{SQL: "embedding <-> ?", Vars: []any{vec}}).
		Limit(k).
		Pluck("asset_id", &ids).Error

	return ids, err
}
