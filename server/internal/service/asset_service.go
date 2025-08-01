package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"server/internal/models"
	"server/internal/repository"
	"server/internal/storage"
	"time"

	"github.com/google/uuid"
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
	GetAsset(ctx context.Context, id uuid.UUID) (*models.Asset, error)
	GetAssetWithOptions(ctx context.Context, id uuid.UUID, includeThumbnails, includeTags, includeAlbums bool) (*models.Asset, error)
	GetAssetsByType(ctx context.Context, assetType models.AssetType, limit, offset int) ([]*models.Asset, error)
	GetAssetsByOwner(ctx context.Context, ownerID int, limit, offset int) ([]*models.Asset, error)
	DeleteAsset(ctx context.Context, id uuid.UUID) error
	UpdateAssetMetadata(ctx context.Context, id uuid.UUID, metadata models.SpecificMetadata) error
	AddAssetToAlbum(ctx context.Context, assetID uuid.UUID, albumID int) error
	RemoveAssetFromAlbum(ctx context.Context, assetID uuid.UUID, albumID int) error
	AddTagToAsset(ctx context.Context, assetID uuid.UUID, tagID int, confidence float32, source string) error
	RemoveTagFromAsset(ctx context.Context, assetID uuid.UUID, tagID int) error
	CreateThumbnail(ctx context.Context, assetID uuid.UUID, size string, thumbnailPath string) (*models.Thumbnail, error)
	SearchAssets(ctx context.Context, query string, assetType *models.AssetType, limit, offset int) ([]*models.Asset, error)
	DetectDuplicates(ctx context.Context, hash string) ([]*models.Asset, error)
	// SaveAssetIndex completes the INDEX step for a processed asset
	SaveAssetIndex(ctx context.Context, taskID string, hash string) error
	// CreateAssetRecord creates a new asset record in the database without file upload
	CreateAssetRecord(ctx context.Context, asset *models.Asset) (*models.Asset, error)

	GetOrCreateTagByName(ctx context.Context, name, category string, isAIGenerated bool) (*models.Tag, error)
	GetThumbnailByID(ctx context.Context, thumbnailID int) (*models.Thumbnail, error)
	GetThumbnailByAssetIDAndSize(ctx context.Context, assetID uuid.UUID, size string) (*models.Thumbnail, error)
	SaveNewAsset(ctx context.Context, fileReader io.Reader, filename string, contentType string) (string, error)
	SaveNewThumbnail(ctx context.Context, buffers io.Reader, asset *models.Asset, size string) error
	SaveNewEmbedding(ctx context.Context, assetID uuid.UUID, embedding []float32) error
	SaveNewBioAtlas(ctx context.Context, assetID uuid.UUID, predictions []*models.SpeciesPrediction) error
}

type assetService struct {
	repo      repository.AssetRepository
	tagRepo   repository.TagRepository
	embedRepo repository.EmbeddingRepository
	storage   storage.Storage
}

// NewAssetService creates a new instance of AssetService with storage configuration
func NewAssetService(r repository.AssetRepository, tagR repository.TagRepository, e repository.EmbeddingRepository, s storage.Storage) (AssetService, error) {
	return &assetService{
		repo:      r,
		tagRepo:   tagR,
		storage:   s,
		embedRepo: e,
	}, nil
}

// ================================
// Asset CRUD Operations
// ================================

// CreateAssetRecord creates a new asset record in the database
func (s *assetService) CreateAssetRecord(ctx context.Context, asset *models.Asset) (*models.Asset, error) {
	return asset, s.repo.CreateAsset(ctx, asset)
}

// GetAsset retrieves an asset by its ID
func (s *assetService) GetAsset(ctx context.Context, id uuid.UUID) (*models.Asset, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *assetService) GetAssetWithOptions(ctx context.Context, id uuid.UUID, includeThumbnails, includeTags, includeAlbums bool) (*models.Asset, error) {
	return s.repo.GetByIDWithOptions(ctx, id, includeThumbnails, includeTags, includeAlbums)
}

// GetAssetsByType retrieves assets by type with pagination
func (s *assetService) GetAssetsByType(ctx context.Context, assetType models.AssetType, limit, offset int) ([]*models.Asset, error) {
	return s.repo.GetByType(ctx, assetType, limit, offset)
}

// GetAssetsByOwner retrieves assets by owner with pagination
func (s *assetService) GetAssetsByOwner(ctx context.Context, ownerID int, limit, offset int) ([]*models.Asset, error) {
	return s.repo.GetByOwner(ctx, ownerID, limit, offset)
}

// SearchAssets searches for assets by query and type
func (s *assetService) SearchAssets(ctx context.Context, query string, assetType *models.AssetType, limit, offset int) ([]*models.Asset, error) {
	return s.repo.SearchAssets(ctx, query, assetType, limit, offset)
}

// DetectDuplicates finds assets with the same hash
func (s *assetService) DetectDuplicates(ctx context.Context, hash string) ([]*models.Asset, error) {
	return s.repo.GetAssetsByHash(ctx, hash)
}

// UpdateAssetMetadata updates the specific metadata of an asset
func (s *assetService) UpdateAssetMetadata(ctx context.Context, id uuid.UUID, metadata models.SpecificMetadata) error {
	return s.repo.UpdateAssetMetadata(ctx, id, metadata)
}

// DeleteAsset marks an asset as deleted
func (s *assetService) DeleteAsset(ctx context.Context, id uuid.UUID) error {
	asset, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to find asset: %w", err)
	}

	return s.repo.DeleteAsset(ctx, asset.AssetID)
}

// AddAssetToAlbum adds an asset to an album
func (s *assetService) AddAssetToAlbum(ctx context.Context, assetID uuid.UUID, albumID int) error {
	return s.repo.AddAssetToAlbum(ctx, assetID, albumID)
}

// RemoveAssetFromAlbum removes an asset from an album
func (s *assetService) RemoveAssetFromAlbum(ctx context.Context, assetID uuid.UUID, albumID int) error {
	return s.repo.RemoveAssetFromAlbum(ctx, assetID, albumID)
}

// AddTagToAsset adds a tag to an asset
func (s *assetService) AddTagToAsset(ctx context.Context, assetID uuid.UUID, tagID int, confidence float32, source string) error {
	return s.repo.AddTagToAsset(ctx, assetID, tagID, confidence, source)
}

// RemoveTagFromAsset removes a tag from an asset
func (s *assetService) RemoveTagFromAsset(ctx context.Context, assetID uuid.UUID, tagID int) error {
	return s.repo.RemoveTagFromAsset(ctx, assetID, tagID)
}

// SaveAssetIndex implements the INDEX step: verify asset exists by hash and complete indexing
func (s *assetService) SaveAssetIndex(ctx context.Context, taskID string, hash string) error {
	assets, err := s.repo.GetAssetsByHash(ctx, hash)
	if err != nil {
		return fmt.Errorf("failed to query asset by hash: %w", err)
	}
	if len(assets) == 0 {
		return fmt.Errorf("no asset found for hash %s", hash)
	}

	// Get the asset for indexing
	asset := assets[0]

	// Update asset metadata to mark it as indexed
	if asset.SpecificMetadata == nil {
		asset.SpecificMetadata = make(models.SpecificMetadata)
	}

	// Add indexing completion metadata
	asset.SpecificMetadata["indexed"] = true
	asset.SpecificMetadata["index_task_id"] = taskID
	asset.SpecificMetadata["index_completed_at"] = time.Now().Format(time.RFC3339)

	// Update the asset in the database
	if err := s.repo.UpdateAsset(ctx, asset); err != nil {
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
func (s *assetService) CreateThumbnail(ctx context.Context, assetID uuid.UUID, size string, thumbnailPath string) (*models.Thumbnail, error) {
	thumbnail := &models.Thumbnail{
		AssetID:     assetID,
		Size:        size,
		StoragePath: thumbnailPath,
		MimeType:    "image/webp", // Thumbnails are typically JPEG
		CreatedAt:   time.Now(),
	}

	err := s.repo.CreateThumbnail(ctx, thumbnail)
	return thumbnail, err
}

// GetThumbnailByID retrieves thumbnails by their ID
func (s *assetService) GetThumbnailByID(ctx context.Context, thumbnailID int) (*models.Thumbnail, error) {
	return s.repo.GetThumbnailByID(ctx, thumbnailID)
}

// GetThumbnailByAssetIDAndSize retrieves a thumbnail by asset ID and size
func (s *assetService) GetThumbnailByAssetIDAndSize(ctx context.Context, assetID uuid.UUID, size string) (*models.Thumbnail, error) {
	return s.repo.GetThumbnailByAssetIDAndSize(ctx, assetID, size)
}

// SaveNewThumbnail TODO: Refine this
func (s *assetService) SaveNewThumbnail(ctx context.Context, buffers io.Reader, asset *models.Asset, size string) error {
	// TODO: Upload Thumbanil to different folder
	storagePath, err := s.storage.UploadWithMetadata(ctx, buffers, asset.OriginalFilename, "")
	if err != nil {
		return err
	}
	if _, err := s.CreateThumbnail(ctx, asset.AssetID, size, storagePath); err != nil {
		s.storage.Delete(ctx, storagePath)
		return err
	}
	return nil
}

// ================================
// Embedding CRUD Operations
// ================================

func (s *assetService) SaveNewEmbedding(ctx context.Context, assetID uuid.UUID, embedding []float32) error {
	if err := s.embedRepo.UpsertEmbedding(ctx, assetID, embedding); err != nil {
		return err
	}
	return nil
}

// ================================
// Helper Functions
// ================================

func (s *assetService) GetOrCreateTagByName(ctx context.Context, name, category string, isAIGenerated bool) (*models.Tag, error) {
	tag, err := s.tagRepo.GetByName(ctx, name)
	if err == nil {
		return tag, nil
	}
	// Assuming gorm.ErrRecordNotFound is imported or handled at repo layer
	tag = &models.Tag{
		TagName:       name,
		Category:      category,
		IsAIGenerated: isAIGenerated,
		Assets:        []models.Asset{},
	}
	if err := s.tagRepo.Create(ctx, tag); err != nil {
		return nil, err
	}
	return tag, nil
}
