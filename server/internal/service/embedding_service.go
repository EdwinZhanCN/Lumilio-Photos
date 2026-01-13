package service

import (
	"context"
	"time"

	"server/internal/db/repo"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/pgvector/pgvector-go"
)

// EmbeddingService interface defines the contract for embedding operations
type EmbeddingService interface {
	SaveEmbedding(ctx context.Context, assetID pgtype.UUID, embeddingType EmbeddingType, model string, vector []float32, isPrimary bool) error
	GetEmbedding(ctx context.Context, assetID pgtype.UUID, embeddingType EmbeddingType, model string) (repo.Embedding, error)
	GetAssetEmbeddingInfo(ctx context.Context, assetID pgtype.UUID) (map[EmbeddingType]EmbeddingInfo, error)
	DeleteEmbedding(ctx context.Context, assetID pgtype.UUID, embeddingType EmbeddingType, model string) error
}

type embeddingService struct {
	queries *repo.Queries
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


func NewEmbeddingService(queries *repo.Queries) EmbeddingService {
	return &embeddingService{
		queries: queries,
	}
}

// SaveEmbedding saves any type of embedding with specified primary status
func (e *embeddingService) SaveEmbedding(ctx context.Context, assetID pgtype.UUID, embeddingType EmbeddingType, model string, vector []float32, isPrimary bool) error {
	pgVector := pgvector.NewVector(vector)
	pgVectorPtr := &pgVector

	params := repo.UpsertEmbeddingParams{
		AssetID:             assetID,
		EmbeddingType:       string(embeddingType),
		EmbeddingModel:      model,
		EmbeddingDimensions: int32(len(vector)),
		Vector:              pgVectorPtr,
		IsPrimary:           &isPrimary,
	}

	return e.queries.UpsertEmbedding(ctx, params)
}

// SavePrimaryEmbedding saves embedding and marks it as primary for its type
func (e *embeddingService) SavePrimaryEmbedding(ctx context.Context, assetID pgtype.UUID, embeddingType EmbeddingType, model string, vector []float32) error {
	return e.SaveEmbedding(ctx, assetID, embeddingType, model, vector, true)
}

// SaveSecondaryEmbedding saves embedding as non-primary
func (e *embeddingService) SaveSecondaryEmbedding(ctx context.Context, assetID pgtype.UUID, embeddingType EmbeddingType, model string, vector []float32) error {
	return e.SaveEmbedding(ctx, assetID, embeddingType, model, vector, false)
}

// GetEmbedding retrieves specific embedding by type and model
func (e *embeddingService) GetEmbedding(ctx context.Context, assetID pgtype.UUID, embeddingType EmbeddingType, model string) (repo.Embedding, error) {
	params := repo.GetEmbeddingParams{
		AssetID:        assetID,
		EmbeddingType:  string(embeddingType),
		EmbeddingModel: model,
	}
	return e.queries.GetEmbedding(ctx, params)
}

// GetPrimaryEmbedding retrieves primary embedding for a type
func (e *embeddingService) GetPrimaryEmbedding(ctx context.Context, assetID pgtype.UUID, embeddingType EmbeddingType) (repo.Embedding, error) {
	params := repo.GetPrimaryEmbeddingParams{
		AssetID:       assetID,
		EmbeddingType: string(embeddingType),
	}
	return e.queries.GetPrimaryEmbedding(ctx, params)
}

// GetEmbeddingByType retrieves best embedding for a type (primary first, then latest)
func (e *embeddingService) GetEmbeddingByType(ctx context.Context, assetID pgtype.UUID, embeddingType EmbeddingType) (repo.Embedding, error) {
	params := repo.GetEmbeddingByTypeParams{
		AssetID:       assetID,
		EmbeddingType: string(embeddingType),
	}
	return e.queries.GetEmbeddingByType(ctx, params)
}

// SearchEmbeddingsByType searches using primary embeddings of a specific type
func (e *embeddingService) SearchEmbeddingsByType(ctx context.Context, query []float32, embeddingType EmbeddingType, limit int32) ([]repo.SearchEmbeddingsByTypeRow, error) {
	pgVector := pgvector.NewVector(query)
	pgVectorPtr := &pgVector

	params := repo.SearchEmbeddingsByTypeParams{
		Column1:       pgVectorPtr,
		EmbeddingType: string(embeddingType),
		Limit:         limit,
	}
	return e.queries.SearchEmbeddingsByType(ctx, params)
}

// SearchEmbeddingsByModel searches using specific model embeddings
func (e *embeddingService) SearchEmbeddingsByModel(ctx context.Context, query []float32, embeddingType EmbeddingType, model string, limit int32) ([]repo.SearchEmbeddingsByModelRow, error) {
	pgVector := pgvector.NewVector(query)
	pgVectorPtr := &pgVector

	params := repo.SearchEmbeddingsByModelParams{
		Column1:        pgVectorPtr,
		EmbeddingType:  string(embeddingType),
		EmbeddingModel: model,
		Limit:          limit,
	}
	return e.queries.SearchEmbeddingsByModel(ctx, params)
}

// SetPrimaryEmbeddingForAsset sets specific model as primary for an asset and type
func (e *embeddingService) SetPrimaryEmbeddingForAsset(ctx context.Context, assetID pgtype.UUID, embeddingType EmbeddingType, model string) error {
	params := repo.SetPrimaryEmbeddingForAssetParams{
		AssetID:        assetID,
		EmbeddingType:  string(embeddingType),
		EmbeddingModel: model,
	}
	return e.queries.SetPrimaryEmbeddingForAsset(ctx, params)
}

// GetAllEmbeddingsForAsset retrieves all embeddings for an asset
func (e *embeddingService) GetAllEmbeddingsForAsset(ctx context.Context, assetID pgtype.UUID) ([]repo.GetAllEmbeddingsForAssetRow, error) {
	return e.queries.GetAllEmbeddingsForAsset(ctx, assetID)
}

// GetAssetEmbeddingInfo returns embedding information for an asset
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

		// Keep only the most recent (or primary) embedding for each type
		if existing, exists := result[EmbeddingType(emb.EmbeddingType)]; !exists || *emb.IsPrimary || emb.CreatedAt.Time.After(parseTime(existing.CreatedAt)) {
			result[EmbeddingType(emb.EmbeddingType)] = info
		}
	}

	return result, nil
}

// DeleteEmbedding removes specific embedding
func (e *embeddingService) DeleteEmbedding(ctx context.Context, assetID pgtype.UUID, embeddingType EmbeddingType, model string) error {
	params := repo.DeleteEmbeddingParams{
		AssetID:        assetID,
		EmbeddingType:  string(embeddingType),
		EmbeddingModel: model,
	}
	return e.queries.DeleteEmbedding(ctx, params)
}

// DeleteAllEmbeddingsForAsset removes all embeddings for an asset
func (e *embeddingService) DeleteAllEmbeddingsForAsset(ctx context.Context, assetID pgtype.UUID) error {
	return e.queries.DeleteAllEmbeddingsForAsset(ctx, assetID)
}

// GetAvailableModels returns all available models for an embedding type
func (e *embeddingService) GetAvailableModels(ctx context.Context, embeddingType EmbeddingType) ([]repo.GetEmbeddingModelsRow, error) {
	return e.queries.GetEmbeddingModels(ctx, string(embeddingType))
}

// CountEmbeddingsByType returns count of primary embeddings for a type
func (e *embeddingService) CountEmbeddingsByType(ctx context.Context, embeddingType EmbeddingType) (int64, error) {
	result, err := e.queries.CountEmbeddingsByType(ctx, string(embeddingType))
	if err != nil {
		return 0, err
	}
	return result, nil
}

// Helper function to parse time
func parseTime(timeStr string) time.Time {
	if t, err := time.Parse("2006-01-02T15:04:05Z", timeStr); err == nil {
		return t
	}
	return time.Time{}
}
