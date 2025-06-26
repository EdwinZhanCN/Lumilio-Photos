package service

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"path/filepath"
	"server/internal/models"
	"server/internal/repository"
	"server/internal/storage"
	"strings"
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
	UploadAsset(ctx context.Context, file io.Reader, filename string, fileSize int64, ownerID *int) (*models.Asset, error)
	GetAsset(ctx context.Context, id uuid.UUID) (*models.Asset, error)
	GetAssetsByType(ctx context.Context, assetType models.AssetType, limit, offset int) ([]*models.Asset, error)
	GetAssetsByOwner(ctx context.Context, ownerID int, limit, offset int) ([]*models.Asset, error)
	DeleteAsset(ctx context.Context, id uuid.UUID) error
	UpdateAssetMetadata(ctx context.Context, id uuid.UUID, metadata models.SpecificMetadata) error
	AddAssetToAlbum(ctx context.Context, assetID uuid.UUID, albumID int) error
	RemoveAssetFromAlbum(ctx context.Context, assetID uuid.UUID, albumID int) error
	AddTagToAsset(ctx context.Context, assetID uuid.UUID, tagID int, confidence float32, source string) error
	RemoveTagFromAsset(ctx context.Context, assetID uuid.UUID, tagID int) error
	CreateThumbnail(ctx context.Context, assetID uuid.UUID, size string, thumbnailPath string) (*models.Thumbnail, error)
	BatchUploadAssets(ctx context.Context, files []io.Reader, filenames []string, fileSizes []int64, ownerID *int) ([]*models.Asset, []error)
	SearchAssets(ctx context.Context, query string, assetType *models.AssetType, limit, offset int) ([]*models.Asset, error)
	DetectDuplicates(ctx context.Context, hash string) ([]*models.Asset, error)
	// SaveAssetIndex completes the INDEX step for a processed asset
	SaveAssetIndex(ctx context.Context, taskID string, hash string) error
	// CreateAssetRecord creates a new asset record in the database without file upload
	CreateAssetRecord(ctx context.Context, asset *models.Asset) error

	GetOrCreateTagByName(ctx context.Context, name, category string, isAIGenerated bool) (*models.Tag, error)
	GetThumbnailByID(ctx context.Context, thumbnailID int) (*models.Thumbnail, error)
}

type assetService struct {
	repo    repository.AssetRepository
	tagRepo repository.TagRepository
	storage storage.Storage
}

// NewAssetService creates a new instance of AssetService
func NewAssetService(r repository.AssetRepository, tagR repository.TagRepository, s storage.Storage) AssetService {
	return &assetService{
		repo:    r,
		tagRepo: tagR,
		storage: s,
	}
}

// NewAssetServiceWithConfig creates a new instance of AssetService with storage configuration
func NewAssetServiceWithConfig(r repository.AssetRepository, tagR repository.TagRepository, storageConfig storage.StorageConfig) (AssetService, error) {
	s, err := storage.NewStorageWithConfig(storageConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage: %w", err)
	}

	return &assetService{
		repo:    r,
		tagRepo: tagR,
		storage: s,
	}, nil
}

// UploadAsset handles the asset upload process with automatic type detection
func (s *assetService) UploadAsset(ctx context.Context, file io.Reader, filename string, fileSize int64, ownerID *int) (*models.Asset, error) {
	log.Printf("Uploading asset: %s", filename)

	// 1. Detect asset type based on file extension and MIME type
	assetType, err := s.detectAssetType(filename)
	if err != nil {
		return nil, fmt.Errorf("type detection failed: %w", err)
	}

	// 2. Validate file type is supported
	if !s.isValidAssetType(assetType, filename) {
		return nil, ErrInvalidAssetType
	}

	// 3. Calculate file hash for duplicate detection
	hash, fileReader, err := s.calculateHash(file)
	if err != nil {
		return nil, fmt.Errorf("hash calculation failed: %w", err)
	}

	// 4. Check for duplicates
	duplicates, err := s.repo.GetAssetsByHash(ctx, hash)
	if err == nil && len(duplicates) > 0 {
		log.Printf("Duplicate asset detected with hash: %s", hash)
		// Return existing asset or handle as per business logic
		return duplicates[0], nil
	}

	// 5. Upload to storage
	storagePath, err := s.storage.UploadWithMetadata(ctx, fileReader, filename, s.getContentType(filename))
	if err != nil {
		return nil, fmt.Errorf("storage upload failed: %w", err)
	}

	// 6. Create database record
	asset := &models.Asset{
		AssetID:          uuid.New(),
		OwnerID:          ownerID,
		Type:             assetType,
		OriginalFilename: filename,
		StoragePath:      storagePath,
		MimeType:         s.getContentType(filename),
		FileSize:         fileSize,
		Hash:             hash,
		UploadTime:       time.Now(),
		IsDeleted:        false,
		SpecificMetadata: make(models.SpecificMetadata),
	}

	if err := s.repo.CreateAsset(ctx, asset); err != nil {
		// Compensating transaction: delete the uploaded file
		go func() {
			if delErr := s.storage.Delete(context.Background(), storagePath); delErr != nil {
				log.Printf("Failed to delete uploaded file: %v", delErr)
			}
		}()
		return nil, fmt.Errorf("failed to create asset record: %w", err)
	}

	// 7. Queue processing tasks based on asset type
	s.queueProcessingTasks(ctx, asset)

	return asset, nil
}

// GetAsset retrieves an asset by its ID
func (s *assetService) GetAsset(ctx context.Context, id uuid.UUID) (*models.Asset, error) {
	return s.repo.GetByID(ctx, id)
}

// GetAssetsByType retrieves assets by type with pagination
func (s *assetService) GetAssetsByType(ctx context.Context, assetType models.AssetType, limit, offset int) ([]*models.Asset, error) {
	return s.repo.GetByType(ctx, assetType, limit, offset)
}

// GetAssetsByOwner retrieves assets by owner with pagination
func (s *assetService) GetAssetsByOwner(ctx context.Context, ownerID int, limit, offset int) ([]*models.Asset, error) {
	return s.repo.GetByOwner(ctx, ownerID, limit, offset)
}

// DeleteAsset marks an asset as deleted
func (s *assetService) DeleteAsset(ctx context.Context, id uuid.UUID) error {
	asset, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to find asset: %w", err)
	}

	return s.repo.DeleteAsset(ctx, asset.AssetID)
}

// UpdateAssetMetadata updates the specific metadata of an asset
func (s *assetService) UpdateAssetMetadata(ctx context.Context, id uuid.UUID, metadata models.SpecificMetadata) error {
	return s.repo.UpdateAssetMetadata(ctx, id, metadata)
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

// CreateThumbnail creates a new thumbnail for an asset
func (s *assetService) CreateThumbnail(ctx context.Context, assetID uuid.UUID, size string, thumbnailPath string) (*models.Thumbnail, error) {
	thumbnail := &models.Thumbnail{
		AssetID:     assetID,
		Size:        size,
		StoragePath: thumbnailPath,
		MimeType:    "image/jpeg", // Thumbnails are typically JPEG
		CreatedAt:   time.Now(),
	}

	err := s.repo.CreateThumbnail(ctx, thumbnail)
	return thumbnail, err
}

// BatchUploadAssets handles multiple asset uploads
func (s *assetService) BatchUploadAssets(ctx context.Context, files []io.Reader, filenames []string, fileSizes []int64, ownerID *int) ([]*models.Asset, []error) {
	assets := make([]*models.Asset, len(files))
	errors := make([]error, len(files))

	for i := range files {
		asset, err := s.UploadAsset(ctx, files[i], filenames[i], fileSizes[i], ownerID)
		assets[i] = asset
		errors[i] = err
	}

	return assets, errors
}

// SearchAssets searches for assets by query and type
func (s *assetService) SearchAssets(ctx context.Context, query string, assetType *models.AssetType, limit, offset int) ([]*models.Asset, error) {
	return s.repo.SearchAssets(ctx, query, assetType, limit, offset)
}

// DetectDuplicates finds assets with the same hash
func (s *assetService) DetectDuplicates(ctx context.Context, hash string) ([]*models.Asset, error) {
	return s.repo.GetAssetsByHash(ctx, hash)
}

// Private helper methods

func (s *assetService) detectAssetType(filename string) (models.AssetType, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	mimeType := s.getContentType(filename)

	// Detect based on MIME type
	switch {
	case strings.HasPrefix(mimeType, "image/"):
		return models.AssetTypePhoto, nil
	case strings.HasPrefix(mimeType, "video/"):
		return models.AssetTypeVideo, nil
	case strings.HasPrefix(mimeType, "audio/"):
		return models.AssetTypeAudio, nil
	case strings.Contains(mimeType, "pdf") || strings.Contains(mimeType, "document"):
		return models.AssetTypeDocument, nil
	}

	// Fallback to extension-based detection
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".webp", ".tiff":
		return models.AssetTypePhoto, nil
	case ".mp4", ".avi", ".mov", ".wmv", ".flv", ".webm", ".mkv":
		return models.AssetTypeVideo, nil
	case ".mp3", ".wav", ".flac", ".aac", ".ogg", ".wma":
		return models.AssetTypeAudio, nil
	case ".pdf", ".doc", ".docx", ".txt", ".rtf":
		return models.AssetTypeDocument, nil
	}

	return "", ErrUnsupportedAssetType
}

func (s *assetService) isValidAssetType(assetType models.AssetType, filename string) bool {
	return assetType.Valid()
}

func (s *assetService) getContentType(filename string) string {
	ext := filepath.Ext(filename)
	contentType := mime.TypeByExtension(ext)
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	return contentType
}

func (s *assetService) calculateHash(file io.Reader) (string, io.Reader, error) {
	// Read the entire file to calculate hash
	data, err := io.ReadAll(file)
	if err != nil {
		return "", nil, err
	}

	// Calculate SHA256 hash
	hash := sha256.Sum256(data)
	hashString := fmt.Sprintf("%x", hash)

	// Return a new reader with the same data
	return hashString, strings.NewReader(string(data)), nil
}

func (s *assetService) queueProcessingTasks(ctx context.Context, asset *models.Asset) {
	// Queue different processing tasks based on asset type
	switch asset.Type {
	case models.AssetTypePhoto:
		// Queue EXIF extraction, thumbnail generation, AI classification
		log.Printf("Queuing photo processing tasks for asset %s", asset.AssetID)
	case models.AssetTypeVideo:
		// Queue video thumbnail generation, metadata extraction
		log.Printf("Queuing video processing tasks for asset %s", asset.AssetID)
	case models.AssetTypeAudio:
		// Queue audio metadata extraction
		log.Printf("Queuing audio processing tasks for asset %s", asset.AssetID)
	default:
		log.Printf("No specific processing tasks for asset type %s", asset.Type)
	}
}

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

// CreateAssetRecord creates a new asset record in the database
func (s *assetService) CreateAssetRecord(ctx context.Context, asset *models.Asset) error {
	return s.repo.CreateAsset(ctx, asset)
}

// GetThumbnailByID retrieves thumbnails by their ID
func (s *assetService) GetThumbnailByID(ctx context.Context, thumbnailID int) (*models.Thumbnail, error) {
	return s.repo.GetThumbnailByID(ctx, thumbnailID)
}
