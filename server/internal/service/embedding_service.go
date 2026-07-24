package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
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
	SaveVideoFrameEmbeddings(ctx context.Context, assetID pgtype.UUID, model string, frames []VideoFrameEmbedding) error
	SaveAestheticScore(ctx context.Context, assetID pgtype.UUID, score float32, modelVersion string) error
	GetEmbedding(ctx context.Context, assetID pgtype.UUID, embeddingType EmbeddingType, model string) (repo.Embedding, error)
	GetAssetEmbeddingInfo(ctx context.Context, assetID pgtype.UUID) (map[EmbeddingType]EmbeddingInfo, error)
	DeleteEmbedding(ctx context.Context, assetID pgtype.UUID, embeddingType EmbeddingType, model string) error
	ResolveDefaultSearchSpace(ctx context.Context, embeddingType EmbeddingType, model string, dimensions int) (repo.EmbeddingSpace, error)
	GetPrimaryEmbeddingVector(ctx context.Context, assetID pgtype.UUID, embeddingType EmbeddingType) (PrimaryEmbedding, error)
}

// VideoFrameEmbedding is one sampled video frame's semantic vector.
type VideoFrameEmbedding struct {
	FrameTsMs int32
	Vector    []float32
}

// PrimaryEmbedding is the decoded primary embedding for an asset/type, returned
// as a plain []float32 so callers (e.g. the classification worker) need no
// pgvector import.
type PrimaryEmbedding struct {
	Vector     []float32
	Model      string
	Dimensions int
}

type embeddingService struct {
	queries *repo.Queries
	pool    *pgxpool.Pool
}

type EmbeddingType string

const (
	EmbeddingTypeSemantic EmbeddingType = "semantic"
	EmbeddingTypeFace     EmbeddingType = "face"
	EmbeddingTypePHash    EmbeddingType = "phash"
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

	// Semantic vectors are canonicalized (MRL-truncated to CanonicalEmbeddingDim
	// and L2-normalized) so stored image vectors share one comparable, unit-length
	// space with text queries and zero-shot prototypes. Other embedding types
	// (e.g. pHash) carry their own semantics and are stored verbatim.
	if embeddingType == EmbeddingTypeSemantic {
		vector = canonicalizeSemanticVector(vector)
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

	pgVector := pgvector.NewVector(vector)

	if embeddingType == EmbeddingTypeSemantic {
		// Semantic vectors live in the dedicated fixed-dimension search_embeddings
		// table (cosine HNSW). The default space records the active model for query
		// routing and model-change detection; the vector index itself is static
		// (migration 000012), so no per-space index creation is needed here.
		space, err = e.ensureDefaultSpace(ctx, queries, embeddingType, space)
		if err != nil {
			return err
		}

		// Replace the asset's primary (whole-asset) semantic row. Video frame rows
		// (frame_ts_ms IS NOT NULL) are written by SaveVideoFrameEmbeddings.
		if err := queries.DeleteSearchEmbeddingsByAsset(ctx, assetID); err != nil {
			return fmt.Errorf("clear search embeddings: %w", err)
		}
		if err := queries.InsertSearchEmbedding(ctx, repo.InsertSearchEmbeddingParams{
			AssetID:   assetID,
			SpaceID:   space.ID,
			FrameTsMs: nil,
			Vector:    &pgVector,
			ModelID:   model,
		}); err != nil {
			return fmt.Errorf("insert search embedding: %w", err)
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit embedding transaction: %w", err)
		}
		return nil
	}

	// Non-search vectors (e.g. pHash) stay in the generic exact-scan table.
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

// SaveVideoFrameEmbeddings replaces all semantic rows for a video asset with
// N frame rows (frame_ts_ms set). There is no NULL-primary row for videos.
func (e *embeddingService) SaveVideoFrameEmbeddings(ctx context.Context, assetID pgtype.UUID, model string, frames []VideoFrameEmbedding) error {
	if len(frames) == 0 {
		return fmt.Errorf("video frame embeddings are empty")
	}
	if strings.TrimSpace(model) == "" {
		return fmt.Errorf("embedding model is empty")
	}

	canonical := make([]VideoFrameEmbedding, 0, len(frames))
	for _, frame := range frames {
		if len(frame.Vector) == 0 {
			return fmt.Errorf("frame embedding vector is empty at ts=%d", frame.FrameTsMs)
		}
		if frame.FrameTsMs < 0 {
			return fmt.Errorf("frame timestamp must be non-negative: %d", frame.FrameTsMs)
		}
		canonical = append(canonical, VideoFrameEmbedding{
			FrameTsMs: frame.FrameTsMs,
			Vector:    canonicalizeSemanticVector(frame.Vector),
		})
	}

	tx, err := e.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin video frame embedding transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	queries := e.queries.WithTx(tx)

	space, err := e.upsertEmbeddingSpace(ctx, queries, EmbeddingTypeSemantic, model, len(canonical[0].Vector))
	if err != nil {
		return err
	}
	space, err = e.ensureDefaultSpace(ctx, queries, EmbeddingTypeSemantic, space)
	if err != nil {
		return err
	}

	if err := queries.DeleteSearchEmbeddingsByAsset(ctx, assetID); err != nil {
		return fmt.Errorf("clear search embeddings: %w", err)
	}

	seenTS := make(map[int32]struct{}, len(canonical))
	for _, frame := range canonical {
		if _, dup := seenTS[frame.FrameTsMs]; dup {
			return fmt.Errorf("duplicate frame timestamp: %d", frame.FrameTsMs)
		}
		seenTS[frame.FrameTsMs] = struct{}{}

		ts := frame.FrameTsMs
		pgVector := pgvector.NewVector(frame.Vector)
		if err := queries.InsertSearchEmbedding(ctx, repo.InsertSearchEmbeddingParams{
			AssetID:   assetID,
			SpaceID:   space.ID,
			FrameTsMs: &ts,
			Vector:    &pgVector,
			ModelID:   model,
		}); err != nil {
			return fmt.Errorf("insert frame embedding at ts=%d: %w", frame.FrameTsMs, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit video frame embedding transaction: %w", err)
	}
	return nil
}

// SaveAestheticScore upserts a per-asset aesthetic quality score.
func (e *embeddingService) SaveAestheticScore(ctx context.Context, assetID pgtype.UUID, score float32, modelVersion string) error {
	_, err := e.queries.UpsertAssetQualityScore(ctx, repo.UpsertAssetQualityScoreParams{
		AssetID:      assetID,
		Score:        score,
		ModelVersion: modelVersion,
	})
	if err != nil {
		return fmt.Errorf("upsert aesthetic score: %w", err)
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

// GetPrimaryEmbeddingVector returns the asset's primary embedding for a type as
// a decoded []float32 (plus model/dimensions). Reuses the existing
// GetPrimaryEmbedding query; the worker never re-runs the ML model.
func (e *embeddingService) GetPrimaryEmbeddingVector(ctx context.Context, assetID pgtype.UUID, embeddingType EmbeddingType) (PrimaryEmbedding, error) {
	if embeddingType == EmbeddingTypeSemantic {
		row, err := e.queries.GetPrimarySearchEmbedding(ctx, assetID)
		if err != nil {
			return PrimaryEmbedding{}, err
		}
		if row.Vector == nil {
			return PrimaryEmbedding{}, fmt.Errorf("primary %s embedding has no vector", embeddingType)
		}
		vec := row.Vector.Slice()
		return PrimaryEmbedding{
			Vector:     vec,
			Model:      row.ModelID,
			Dimensions: len(vec),
		}, nil
	}

	row, err := e.GetPrimaryEmbedding(ctx, assetID, embeddingType)
	if err != nil {
		return PrimaryEmbedding{}, err
	}
	if row.Vector == nil {
		return PrimaryEmbedding{}, fmt.Errorf("primary %s embedding has no vector", embeddingType)
	}
	return PrimaryEmbedding{
		Vector:     row.Vector.Slice(),
		Model:      row.EmbeddingModel,
		Dimensions: int(row.EmbeddingDimensions),
	}, nil
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
