package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/storage"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	pgvector_go "github.com/pgvector/pgvector-go"
)

// Asset type constants
const (
	AssetTypePhoto = "PHOTO"
	AssetTypeVideo = "VIDEO"
	AssetTypeAudio = "AUDIO"
)

// Error constants for asset service
var (
	ErrInvalidAssetType     = errors.New("invalid asset type")
	ErrAssetFileTooLarge    = errors.New("file too large: maximum file size exceeded")
	ErrUnsupportedAssetType = errors.New("unsupported asset type")
	ErrAssetNotFound        = errors.New("asset not found")
)

// AssetService defines the interface for asset-related operations
type AssetService interface {
	GetAsset(ctx context.Context, id uuid.UUID) (*repo.Asset, error)
	GetAssetWithOptions(ctx context.Context, id uuid.UUID, includeThumbnails, includeTags, includeAlbums bool) (interface{}, error)
	GetAssetsByType(ctx context.Context, assetType string, limit, offset int) ([]repo.Asset, error)
	GetAssetsByOwner(ctx context.Context, ownerID int, limit, offset int) ([]repo.Asset, error)
	DeleteAsset(ctx context.Context, id uuid.UUID) error
	UpdateAssetMetadata(ctx context.Context, id uuid.UUID, metadata []byte) error
	AddAssetToAlbum(ctx context.Context, assetID uuid.UUID, albumID int) error
	RemoveAssetFromAlbum(ctx context.Context, assetID uuid.UUID, albumID int) error
	AddTagToAsset(ctx context.Context, assetID uuid.UUID, tagID int, confidence float32, source string) error
	RemoveTagFromAsset(ctx context.Context, assetID uuid.UUID, tagID int) error
	CreateThumbnail(ctx context.Context, assetID uuid.UUID, size string, thumbnailPath string) (*repo.Thumbnail, error)
	SearchAssets(ctx context.Context, query string, assetType *string, limit, offset int) ([]repo.Asset, error)
	DetectDuplicates(ctx context.Context, hash string) ([]repo.Asset, error)
	SaveAssetIndex(ctx context.Context, taskID string, hash string) error
	CreateAssetRecord(ctx context.Context, params repo.CreateAssetParams) (*repo.Asset, error)

	GetOrCreateTagByName(ctx context.Context, name, category string, isAIGenerated bool) (*repo.Tag, error)
	GetThumbnailByID(ctx context.Context, thumbnailID int) (*repo.Thumbnail, error)
	GetThumbnailByAssetIDAndSize(ctx context.Context, assetID uuid.UUID, size string) (*repo.Thumbnail, error)
	SaveNewAsset(ctx context.Context, fileReader io.Reader, filename string, hash string) (string, error)
	SaveNewThumbnail(ctx context.Context, buffers io.Reader, asset *repo.Asset, size string) error
	SaveNewEmbedding(ctx context.Context, pgUUID pgtype.UUID, embedding []float32) error
	SaveNewSpeciesPredictions(ctx context.Context, pgUUID pgtype.UUID, predictions []dbtypes.SpeciesPredictionMeta) error
}

type assetService struct {
	queries *repo.Queries
	storage storage.Storage
}

// NewAssetService creates a new instance of AssetService with storage configuration
func NewAssetService(q *repo.Queries, s storage.Storage) (AssetService, error) {
	return &assetService{
		queries: q,
		storage: s,
	}, nil
}

// ================================
// Asset CRUD Operations
// ================================

// CreateAssetRecord creates a new asset record in the database
func (s *assetService) CreateAssetRecord(ctx context.Context, params repo.CreateAssetParams) (*repo.Asset, error) {

	asset, err := s.queries.CreateAsset(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to create asset: %w", err)
	}

	return &asset, nil
}

// GetAsset retrieves an asset by its ID
func (s *assetService) GetAsset(ctx context.Context, id uuid.UUID) (*repo.Asset, error) {
	pgUUID := pgtype.UUID{}
	if err := pgUUID.Scan(id.String()); err != nil {
		return nil, fmt.Errorf("invalid UUID: %w", err)
	}

	dbAsset, err := s.queries.GetAssetByID(ctx, pgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get asset: %w", err)
	}

	return &dbAsset, nil
}

func (s *assetService) GetAssetWithOptions(ctx context.Context, id uuid.UUID, includeThumbnails, includeTags, includeAlbums bool) (interface{}, error) {
	pgUUID := pgtype.UUID{}
	if err := pgUUID.Scan(id.String()); err != nil {
		return nil, fmt.Errorf("invalid UUID: %w", err)
	}

	if includeThumbnails && includeTags {
		dbAsset, err := s.queries.GetAssetWithRelations(ctx, pgUUID)
		if err != nil {
			return nil, fmt.Errorf("failed to get asset with relations: %w", err)
		}
		return dbAsset, nil
	} else if includeThumbnails {
		dbAsset, err := s.queries.GetAssetWithThumbnails(ctx, pgUUID)
		if err != nil {
			return nil, fmt.Errorf("failed to get asset with thumbnails: %w", err)
		}
		return dbAsset, nil
	} else if includeTags {
		dbAsset, err := s.queries.GetAssetWithTags(ctx, pgUUID)
		if err != nil {
			return nil, fmt.Errorf("failed to get asset with tags: %w", err)
		}
		return dbAsset, nil
	}

	return s.GetAsset(ctx, id)
}

// GetAssetsByType retrieves assets by type with pagination
func (s *assetService) GetAssetsByType(ctx context.Context, assetType string, limit, offset int) ([]repo.Asset, error) {
	params := repo.GetAssetsByTypeParams{
		Type:   assetType,
		Limit:  int32(limit),
		Offset: int32(offset),
	}

	return s.queries.GetAssetsByType(ctx, params)
}

// GetAssetsByOwner retrieves assets by owner with pagination
func (s *assetService) GetAssetsByOwner(ctx context.Context, ownerID int, limit, offset int) ([]repo.Asset, error) {
	params := repo.GetAssetsByOwnerParams{
		OwnerID: int32PtrFromIntPtr(&ownerID),
		Limit:   int32(limit),
		Offset:  int32(offset),
	}

	return s.queries.GetAssetsByOwner(ctx, params)
}

// SearchAssets searches for assets by query and type
func (s *assetService) SearchAssets(ctx context.Context, query string, assetType *string, limit, offset int) ([]repo.Asset, error) {
	params := repo.SearchAssetsParams{
		Column1: query,
		Limit:   int32(limit),
		Offset:  int32(offset),
	}

	if assetType != nil {
		params.Column2 = *assetType
	}

	return s.queries.SearchAssets(ctx, params)
}

// DetectDuplicates finds assets with the same hash
func (s *assetService) DetectDuplicates(ctx context.Context, hash string) ([]repo.Asset, error) {
	return s.queries.GetAssetsByHash(ctx, &hash)
}

// UpdateAssetMetadata updates the specific metadata of an asset
func (s *assetService) UpdateAssetMetadata(ctx context.Context, id uuid.UUID, metadata []byte) error {
	pgUUID := pgtype.UUID{}
	if err := pgUUID.Scan(id.String()); err != nil {
		return fmt.Errorf("invalid UUID: %w", err)
	}

	params := repo.UpdateAssetMetadataParams{
		AssetID:          pgUUID,
		SpecificMetadata: metadata,
	}

	return s.queries.UpdateAssetMetadata(ctx, params)
}

// DeleteAsset marks an asset as deleted
func (s *assetService) DeleteAsset(ctx context.Context, id uuid.UUID) error {
	pgUUID := pgtype.UUID{}
	if err := pgUUID.Scan(id.String()); err != nil {
		return fmt.Errorf("invalid UUID: %w", err)
	}

	return s.queries.DeleteAsset(ctx, pgUUID)
}

// AddAssetToAlbum adds an asset to an album
func (s *assetService) AddAssetToAlbum(ctx context.Context, assetID uuid.UUID, albumID int) error {
	pgUUID := pgtype.UUID{}
	if err := pgUUID.Scan(assetID.String()); err != nil {
		return fmt.Errorf("invalid UUID: %w", err)
	}

	params := repo.AddAssetToAlbumParams{
		AssetID: pgUUID,
		AlbumID: int32(albumID),
	}

	return s.queries.AddAssetToAlbum(ctx, params)
}

// RemoveAssetFromAlbum removes an asset from an album
func (s *assetService) RemoveAssetFromAlbum(ctx context.Context, assetID uuid.UUID, albumID int) error {
	pgUUID := pgtype.UUID{}
	if err := pgUUID.Scan(assetID.String()); err != nil {
		return fmt.Errorf("invalid UUID: %w", err)
	}

	params := repo.RemoveAssetFromAlbumParams{
		AssetID: pgUUID,
		AlbumID: int32(albumID),
	}

	return s.queries.RemoveAssetFromAlbum(ctx, params)
}

// AddTagToAsset adds a tag to an asset
func (s *assetService) AddTagToAsset(ctx context.Context, assetID uuid.UUID, tagID int, confidence float32, source string) error {
	pgUUID := pgtype.UUID{}
	if err := pgUUID.Scan(assetID.String()); err != nil {
		return fmt.Errorf("invalid UUID: %w", err)
	}

	confidenceNumeric := pgtype.Numeric{}
	if err := confidenceNumeric.Scan(fmt.Sprintf("%.3f", confidence)); err != nil {
		return fmt.Errorf("failed to convert confidence: %w", err)
	}

	params := repo.AddTagToAssetParams{
		AssetID:    pgUUID,
		TagID:      int32(tagID),
		Confidence: confidenceNumeric,
		Source:     source,
	}

	return s.queries.AddTagToAsset(ctx, params)
}

// RemoveTagFromAsset removes a tag from an asset
func (s *assetService) RemoveTagFromAsset(ctx context.Context, assetID uuid.UUID, tagID int) error {
	pgUUID := pgtype.UUID{}
	if err := pgUUID.Scan(assetID.String()); err != nil {
		return fmt.Errorf("invalid UUID: %w", err)
	}

	params := repo.RemoveTagFromAssetParams{
		AssetID: pgUUID,
		TagID:   int32(tagID),
	}

	return s.queries.RemoveTagFromAsset(ctx, params)
}

// SaveAssetIndex implements the INDEX step: verify asset exists by hash and complete indexing
func (s *assetService) SaveAssetIndex(ctx context.Context, taskID string, hash string) error {
	assets, err := s.queries.GetAssetsByHash(ctx, &hash)
	if err != nil {
		return fmt.Errorf("failed to query asset by hash: %w", err)
	}
	if len(assets) == 0 {
		return fmt.Errorf("no asset found for hash %s", hash)
	}

	// Get the asset for indexing
	asset := assets[0]

	// Update asset metadata to mark it as indexed
	metadata := make(map[string]interface{})
	if len(asset.SpecificMetadata) > 0 {
		if err := json.Unmarshal(asset.SpecificMetadata, &metadata); err != nil {
			return fmt.Errorf("failed to unmarshal existing metadata: %w", err)
		}
	}

	// Add indexing completion metadata
	metadata["indexed"] = true
	metadata["index_task_id"] = taskID
	metadata["index_completed_at"] = time.Now().Format(time.RFC3339)

	// Marshal metadata back to bytes
	metadataBytes, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	params := repo.UpdateAssetMetadataParams{
		AssetID:          asset.AssetID,
		SpecificMetadata: metadataBytes,
	}

	if err := s.queries.UpdateAssetMetadata(ctx, params); err != nil {
		return fmt.Errorf("failed to update asset indexing status: %w", err)
	}

	log.Printf("Asset indexing completed for hash %s, task %s", hash, taskID)
	return nil
}

// SaveNewAsset save the asset to storage, returns asset's storage path and error
func (s *assetService) SaveNewAsset(ctx context.Context, fileReader io.Reader, filename string, hash string) (string, error) {
	storagePath, err := s.storage.UploadWithMetadata(ctx, fileReader, filename, hash)
	if err != nil {
		return "", err
	}

	return storagePath, nil
}

// ================================
// Thumbnail CRUD Operations
// ================================

// CreateThumbnail creates a new thumbnail for an asset
func (s *assetService) CreateThumbnail(ctx context.Context, assetID uuid.UUID, size string, thumbnailPath string) (*repo.Thumbnail, error) {
	pgUUID := pgtype.UUID{}
	if err := pgUUID.Scan(assetID.String()); err != nil {
		return nil, fmt.Errorf("invalid UUID: %w", err)
	}

	params := repo.CreateThumbnailParams{
		AssetID:     pgUUID,
		Size:        size,
		StoragePath: thumbnailPath,
		MimeType:    "image/webp", // Thumbnails are typically WebP
	}

	dbThumbnail, err := s.queries.CreateThumbnail(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to create thumbnail: %w", err)
	}

	return &dbThumbnail, nil
}

// GetThumbnailByID retrieves thumbnails by their ID
func (s *assetService) GetThumbnailByID(ctx context.Context, thumbnailID int) (*repo.Thumbnail, error) {
	dbThumbnail, err := s.queries.GetThumbnailByID(ctx, int32(thumbnailID))
	if err != nil {
		return nil, fmt.Errorf("failed to get thumbnail: %w", err)
	}

	return &dbThumbnail, nil
}

// GetThumbnailByAssetIDAndSize retrieves a thumbnail by asset ID and size
func (s *assetService) GetThumbnailByAssetIDAndSize(ctx context.Context, assetID uuid.UUID, size string) (*repo.Thumbnail, error) {
	pgUUID := pgtype.UUID{}
	if err := pgUUID.Scan(assetID.String()); err != nil {
		return nil, fmt.Errorf("invalid UUID: %w", err)
	}

	params := repo.GetThumbnailByAssetAndSizeParams{
		AssetID: pgUUID,
		Size:    size,
	}

	dbThumbnail, err := s.queries.GetThumbnailByAssetAndSize(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to get thumbnail: %w", err)
	}

	return &dbThumbnail, nil
}

// SaveNewThumbnail TODO: Refine this
func (s *assetService) SaveNewThumbnail(ctx context.Context, buffers io.Reader, asset *repo.Asset, size string) error {
	// TODO: Upload Thumbnail to different folder
	storagePath, err := s.storage.UploadWithMetadata(ctx, buffers, "thumbnail", "")
	if err != nil {
		return err
	}

	var assetUUID uuid.UUID
	if asset.AssetID.Valid {
		assetUUID, err = uuid.FromBytes(asset.AssetID.Bytes[:])
		if err != nil {
			return fmt.Errorf("invalid asset UUID: %w", err)
		}
	} else {
		return fmt.Errorf("asset has no valid UUID")
	}

	if _, err := s.CreateThumbnail(ctx, assetUUID, size, storagePath); err != nil {
		s.storage.Delete(ctx, storagePath)
		return err
	}
	return nil
}

// ================================
// ML CRUD Operations
// ================================

func (s *assetService) SaveNewEmbedding(ctx context.Context, pgUUID pgtype.UUID, embedding []float32) error {
	// Convert []float32 to pgvector.Vector
	vector := pgvector_go.NewVector(embedding)

	params := repo.UpsertEmbeddingParams{
		AssetID:   pgUUID,
		Embedding: &vector,
	}

	return s.queries.UpsertEmbedding(ctx, params)
}

func (s *assetService) SaveNewSpeciesPredictions(ctx context.Context, pgUUID pgtype.UUID, predictions []dbtypes.SpeciesPredictionMeta) error {
	// First, delete existing predictions for the asset
	if err := s.queries.DeleteSpeciesPredictionsByAsset(ctx, pgUUID); err != nil {
		return fmt.Errorf("failed to delete existing species predictions: %w", err)
	}

	// Insert new predictions
	for _, pred := range predictions {
		params := repo.CreateSpeciesPredictionParams{
			AssetID: pgUUID,
			Label:   pred.Label,
			Score:   pred.Score,
		}
		if _, err := s.queries.CreateSpeciesPrediction(ctx, params); err != nil {
			return fmt.Errorf("failed to create species prediction: %w", err)
		}
	}

	return nil

}

// ================================
// Helper Functions
// ================================

func (s *assetService) GetOrCreateTagByName(ctx context.Context, name, category string, isAIGenerated bool) (*repo.Tag, error) {
	tag, err := s.queries.GetTagByName(ctx, name)
	if err == nil {
		return &tag, nil
	}

	// Tag doesn't exist, create it
	params := repo.CreateTagParams{
		TagName:       name,
		IsAiGenerated: &isAIGenerated,
	}

	if category != "" {
		params.Category = &category
	}

	dbTag, err := s.queries.CreateTag(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to create tag: %w", err)
	}

	return &dbTag, nil
}

// ================================
// Utility Functions
// ================================

// Business logic helpers
func IsPhoto(assetType string) bool {
	return assetType == AssetTypePhoto
}

func IsVideo(assetType string) bool {
	return assetType == AssetTypeVideo
}

func IsAudio(assetType string) bool {
	return assetType == AssetTypeAudio
}

// Helper functions for type conversions
func int32PtrFromIntPtr(i *int) *int32 {
	if i == nil {
		return nil
	}
	i32 := int32(*i)
	return &i32
}

func intPtrFromInt32Ptr(i32 *int32) *int {
	if i32 == nil {
		return nil
	}
	i := int(*i32)
	return &i
}
