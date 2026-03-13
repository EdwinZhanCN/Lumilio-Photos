package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"server/internal/db/repo"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pgvector/pgvector-go"
)

const embeddingDistanceMetricL2 = "l2"

// EmbeddingService interface defines the contract for embedding operations.
type EmbeddingService interface {
	SaveEmbedding(ctx context.Context, assetID pgtype.UUID, embeddingType EmbeddingType, model string, vector []float32, isPrimary bool) error
	GetEmbedding(ctx context.Context, assetID pgtype.UUID, embeddingType EmbeddingType, model string) (repo.Embedding, error)
	GetAssetEmbeddingInfo(ctx context.Context, assetID pgtype.UUID) (map[EmbeddingType]EmbeddingInfo, error)
	DeleteEmbedding(ctx context.Context, assetID pgtype.UUID, embeddingType EmbeddingType, model string) error
	ResolveDefaultSearchSpace(ctx context.Context, embeddingType EmbeddingType, model string, dimensions int) (repo.EmbeddingSpace, error)
}

type embeddingService struct {
	queries *repo.Queries
	pool    *pgxpool.Pool
}

type EmbeddingType string

const (
	EmbeddingTypeCLIP EmbeddingType = "clip"
	EmbeddingTypeFace EmbeddingType = "face"
)

type EmbeddingInfo struct {
	Model      string `json:"model"`
	Dimensions int    `json:"dimensions"`
	Type       string `json:"type"`
	IsPrimary  bool   `json:"is_primary"`
	CreatedAt  string `json:"created_at"`
}

func NewEmbeddingService(queries *repo.Queries, pool *pgxpool.Pool) EmbeddingService {
	return &embeddingService{
		queries: queries,
		pool:    pool,
	}
}

// SaveEmbedding saves any type of embedding with specified primary status.
func (e *embeddingService) SaveEmbedding(ctx context.Context, assetID pgtype.UUID, embeddingType EmbeddingType, model string, vector []float32, isPrimary bool) error {
	if len(vector) == 0 {
		return fmt.Errorf("embedding vector is empty")
	}

	tx, err := e.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin embedding transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	queries := e.queries.WithTx(tx)

	space, err := e.upsertEmbeddingSpace(ctx, queries, embeddingType, model, len(vector))
	if err != nil {
		return err
	}

	if embeddingType == EmbeddingTypeCLIP {
		defaultSpace, err := e.ensureDefaultSpace(ctx, queries, embeddingType, space)
		if err != nil {
			return err
		}
		if defaultSpace.ID == space.ID && defaultSpace.SearchEnabled {
			space = defaultSpace
			if err := e.ensureSearchIndexForSpace(ctx, tx, space); err != nil {
				return err
			}
		}
	}

	pgVector := pgvector.NewVector(vector)
	params := repo.UpsertEmbeddingParams{
		AssetID:             assetID,
		EmbeddingType:       string(embeddingType),
		EmbeddingModel:      model,
		EmbeddingDimensions: int32(len(vector)),
		SpaceID:             space.ID,
		Vector:              &pgVector,
		IsPrimary:           &isPrimary,
	}
	if err := queries.UpsertEmbedding(ctx, params); err != nil {
		return fmt.Errorf("upsert embedding: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit embedding transaction: %w", err)
	}

	return nil
}

func (e *embeddingService) ResolveDefaultSearchSpace(ctx context.Context, embeddingType EmbeddingType, model string, dimensions int) (repo.EmbeddingSpace, error) {
	if dimensions <= 0 {
		return repo.EmbeddingSpace{}, fmt.Errorf("invalid embedding dimensions: %d", dimensions)
	}

	tx, err := e.pool.Begin(ctx)
	if err != nil {
		return repo.EmbeddingSpace{}, fmt.Errorf("begin embedding space transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	queries := e.queries.WithTx(tx)

	space, err := e.upsertEmbeddingSpace(ctx, queries, embeddingType, model, dimensions)
	if err != nil {
		return repo.EmbeddingSpace{}, err
	}

	defaultSpace, err := e.ensureDefaultSpace(ctx, queries, embeddingType, space)
	if err != nil {
		return repo.EmbeddingSpace{}, err
	}

	if defaultSpace.ModelID != model || int(defaultSpace.Dimensions) != dimensions {
		return repo.EmbeddingSpace{}, fmt.Errorf(
			"%w: default %s search space is %s/%d, query embedding is %s/%d",
			ErrSemanticSearchUnavailable,
			embeddingType,
			defaultSpace.ModelID,
			defaultSpace.Dimensions,
			model,
			dimensions,
		)
	}

	if defaultSpace.SearchEnabled {
		if err := e.ensureSearchIndexForSpace(ctx, tx, defaultSpace); err != nil {
			return repo.EmbeddingSpace{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return repo.EmbeddingSpace{}, fmt.Errorf("commit embedding space transaction: %w", err)
	}

	return defaultSpace, nil
}

// GetEmbedding retrieves specific embedding by type and model.
func (e *embeddingService) GetEmbedding(ctx context.Context, assetID pgtype.UUID, embeddingType EmbeddingType, model string) (repo.Embedding, error) {
	params := repo.GetEmbeddingParams{
		AssetID:        assetID,
		EmbeddingType:  string(embeddingType),
		EmbeddingModel: model,
	}
	row, err := e.queries.GetEmbedding(ctx, params)
	if err != nil {
		return repo.Embedding{}, err
	}
	return mapEmbeddingRow(row), nil
}

// GetPrimaryEmbedding retrieves primary embedding for a type.
func (e *embeddingService) GetPrimaryEmbedding(ctx context.Context, assetID pgtype.UUID, embeddingType EmbeddingType) (repo.Embedding, error) {
	params := repo.GetPrimaryEmbeddingParams{
		AssetID:       assetID,
		EmbeddingType: string(embeddingType),
	}
	row, err := e.queries.GetPrimaryEmbedding(ctx, params)
	if err != nil {
		return repo.Embedding{}, err
	}
	return mapPrimaryEmbeddingRow(row), nil
}

// GetEmbeddingByType retrieves best embedding for a type (primary first, then latest).
func (e *embeddingService) GetEmbeddingByType(ctx context.Context, assetID pgtype.UUID, embeddingType EmbeddingType) (repo.Embedding, error) {
	params := repo.GetEmbeddingByTypeParams{
		AssetID:       assetID,
		EmbeddingType: string(embeddingType),
	}
	row, err := e.queries.GetEmbeddingByType(ctx, params)
	if err != nil {
		return repo.Embedding{}, err
	}
	return mapEmbeddingByTypeRow(row), nil
}

// SetPrimaryEmbeddingForAsset sets specific model as primary for an asset and type.
func (e *embeddingService) SetPrimaryEmbeddingForAsset(ctx context.Context, assetID pgtype.UUID, embeddingType EmbeddingType, model string) error {
	params := repo.SetPrimaryEmbeddingForAssetParams{
		AssetID:        assetID,
		EmbeddingType:  string(embeddingType),
		EmbeddingModel: model,
	}
	return e.queries.SetPrimaryEmbeddingForAsset(ctx, params)
}

// GetAllEmbeddingsForAsset retrieves all embeddings for an asset.
func (e *embeddingService) GetAllEmbeddingsForAsset(ctx context.Context, assetID pgtype.UUID) ([]repo.GetAllEmbeddingsForAssetRow, error) {
	return e.queries.GetAllEmbeddingsForAsset(ctx, assetID)
}

// GetAssetEmbeddingInfo returns embedding information for an asset.
func (e *embeddingService) GetAssetEmbeddingInfo(ctx context.Context, assetID pgtype.UUID) (map[EmbeddingType]EmbeddingInfo, error) {
	embeddings, err := e.GetAllEmbeddingsForAsset(ctx, assetID)
	if err != nil {
		return nil, err
	}

	result := make(map[EmbeddingType]EmbeddingInfo)

	for _, emb := range embeddings {
		info := EmbeddingInfo{
			Model:      emb.EmbeddingModel,
			Dimensions: int(emb.EmbeddingDimensions),
			Type:       emb.EmbeddingType,
			IsPrimary:  *emb.IsPrimary,
			CreatedAt:  emb.CreatedAt.Time.Format("2006-01-02T15:04:05Z"),
		}

		if existing, exists := result[EmbeddingType(emb.EmbeddingType)]; !exists || *emb.IsPrimary || emb.CreatedAt.Time.After(parseTime(existing.CreatedAt)) {
			result[EmbeddingType(emb.EmbeddingType)] = info
		}
	}

	return result, nil
}

// DeleteEmbedding removes specific embedding.
func (e *embeddingService) DeleteEmbedding(ctx context.Context, assetID pgtype.UUID, embeddingType EmbeddingType, model string) error {
	params := repo.DeleteEmbeddingParams{
		AssetID:        assetID,
		EmbeddingType:  string(embeddingType),
		EmbeddingModel: model,
	}
	return e.queries.DeleteEmbedding(ctx, params)
}

// DeleteAllEmbeddingsForAsset removes all embeddings for an asset.
func (e *embeddingService) DeleteAllEmbeddingsForAsset(ctx context.Context, assetID pgtype.UUID) error {
	return e.queries.DeleteAllEmbeddingsForAsset(ctx, assetID)
}

// GetAvailableModels returns all available models for an embedding type.
func (e *embeddingService) GetAvailableModels(ctx context.Context, embeddingType EmbeddingType) ([]repo.GetEmbeddingModelsRow, error) {
	return e.queries.GetEmbeddingModels(ctx, string(embeddingType))
}

// CountEmbeddingsByType returns count of primary embeddings for a type.
func (e *embeddingService) CountEmbeddingsByType(ctx context.Context, embeddingType EmbeddingType) (int64, error) {
	result, err := e.queries.CountEmbeddingsByType(ctx, string(embeddingType))
	if err != nil {
		return 0, err
	}
	return result, nil
}

func (e *embeddingService) upsertEmbeddingSpace(ctx context.Context, queries *repo.Queries, embeddingType EmbeddingType, model string, dimensions int) (repo.EmbeddingSpace, error) {
	space, err := queries.UpsertEmbeddingSpace(ctx, repo.UpsertEmbeddingSpaceParams{
		EmbeddingType:  string(embeddingType),
		ModelID:        model,
		Dimensions:     int32(dimensions),
		DistanceMetric: embeddingDistanceMetricL2,
	})
	if err != nil {
		return repo.EmbeddingSpace{}, fmt.Errorf("upsert embedding space: %w", err)
	}
	return space, nil
}

func (e *embeddingService) ensureDefaultSpace(ctx context.Context, queries *repo.Queries, embeddingType EmbeddingType, space repo.EmbeddingSpace) (repo.EmbeddingSpace, error) {
	promoted, err := queries.PromoteEmbeddingSpaceAsDefaultIfNone(ctx, repo.PromoteEmbeddingSpaceAsDefaultIfNoneParams{
		ID:            space.ID,
		EmbeddingType: string(embeddingType),
	})
	if err == nil {
		return promoted, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return repo.EmbeddingSpace{}, fmt.Errorf("promote embedding space as default: %w", err)
	}

	defaultSpace, err := queries.GetDefaultEmbeddingSpaceByType(ctx, string(embeddingType))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return space, nil
		}
		return repo.EmbeddingSpace{}, fmt.Errorf("load default embedding space: %w", err)
	}
	return defaultSpace, nil
}

func (e *embeddingService) ensureSearchIndexForSpace(ctx context.Context, tx pgx.Tx, space repo.EmbeddingSpace) error {
	if !space.SearchEnabled {
		return nil
	}
	if space.ID <= 0 {
		return fmt.Errorf("invalid embedding space id: %d", space.ID)
	}
	if space.Dimensions <= 0 {
		return fmt.Errorf("invalid embedding space dimensions: %d", space.Dimensions)
	}

	indexName := fmt.Sprintf("embeddings_space_%d_primary_hnsw_l2_idx", space.ID)
	query := fmt.Sprintf(
		"CREATE INDEX IF NOT EXISTS %s ON embeddings USING hnsw ((vector::vector(%d)) vector_l2_ops) WHERE space_id = %d AND is_primary = true",
		indexName,
		space.Dimensions,
		space.ID,
	)
	if _, err := tx.Exec(ctx, query); err != nil {
		return fmt.Errorf("ensure embedding search index: %w", err)
	}
	return nil
}

func mapEmbeddingRow(row repo.GetEmbeddingRow) repo.Embedding {
	return repo.Embedding{
		ID:                  row.ID,
		AssetID:             row.AssetID,
		EmbeddingType:       row.EmbeddingType,
		EmbeddingModel:      row.EmbeddingModel,
		EmbeddingDimensions: row.EmbeddingDimensions,
		Vector:              row.Vector,
		IsPrimary:           row.IsPrimary,
		CreatedAt:           row.CreatedAt,
		UpdatedAt:           row.UpdatedAt,
		SpaceID:             row.SpaceID,
	}
}

func mapPrimaryEmbeddingRow(row repo.GetPrimaryEmbeddingRow) repo.Embedding {
	return repo.Embedding{
		ID:                  row.ID,
		AssetID:             row.AssetID,
		EmbeddingType:       row.EmbeddingType,
		EmbeddingModel:      row.EmbeddingModel,
		EmbeddingDimensions: row.EmbeddingDimensions,
		Vector:              row.Vector,
		IsPrimary:           row.IsPrimary,
		CreatedAt:           row.CreatedAt,
		UpdatedAt:           row.UpdatedAt,
		SpaceID:             row.SpaceID,
	}
}

func mapEmbeddingByTypeRow(row repo.GetEmbeddingByTypeRow) repo.Embedding {
	return repo.Embedding{
		ID:                  row.ID,
		AssetID:             row.AssetID,
		EmbeddingType:       row.EmbeddingType,
		EmbeddingModel:      row.EmbeddingModel,
		EmbeddingDimensions: row.EmbeddingDimensions,
		Vector:              row.Vector,
		IsPrimary:           row.IsPrimary,
		CreatedAt:           row.CreatedAt,
		UpdatedAt:           row.UpdatedAt,
		SpaceID:             row.SpaceID,
	}
}

func parseTime(timeStr string) time.Time {
	if t, err := time.Parse("2006-01-02T15:04:05Z", timeStr); err == nil {
		return t
	}
	return time.Time{}
}
