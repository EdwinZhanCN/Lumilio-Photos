package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/storage"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/pgvector/pgvector-go"
)

// Asset type constants
const (
	AssetTypePhoto = "PHOTO"
	AssetTypeVideo = "VIDEO"
	AssetTypeAudio = "AUDIO"
)

// Error constants for asset service
var (
	ErrInvalidAssetType          = errors.New("invalid asset type")
	ErrAssetFileTooLarge         = errors.New("file too large: maximum file size exceeded")
	ErrUnsupportedAssetType      = errors.New("unsupported asset type")
	ErrAssetNotFound             = errors.New("asset not found")
	ErrSemanticSearchUnavailable = errors.New("semantic search unavailable")
)

// AssetService defines the interface for asset-related operations
type AssetService interface {
	GetAsset(ctx context.Context, id uuid.UUID) (*repo.Asset, error)
	GetAssetWithOptions(ctx context.Context, id uuid.UUID, includeThumbnails, includeTags, includeAlbums, includeSpecies, includeOCR, includeFaces, includeCaptions bool) (interface{}, error)
	GetAssetsByType(ctx context.Context, assetType string, limit, offset int) ([]repo.Asset, error)
	GetAssetsByOwner(ctx context.Context, ownerID int, limit, offset int) ([]repo.Asset, error)
	GetAssetsByOwnerSorted(ctx context.Context, ownerID int, sortOrder string, limit, offset int) ([]repo.Asset, error)
	GetAssetsByTypesSorted(ctx context.Context, assetTypes []string, sortOrder string, limit, offset int) ([]repo.Asset, error)
	GetAssetsByOwnerAndTypes(ctx context.Context, ownerID int, assetTypes []string, sortOrder string, limit, offset int) ([]repo.Asset, error)
	DeleteAsset(ctx context.Context, id uuid.UUID) error

	UpdateAssetMetadata(ctx context.Context, id uuid.UUID, metadata dbtypes.SpecificMetadata) error

	// Rating management methods
	UpdateAssetRating(ctx context.Context, id uuid.UUID, rating int) error
	UpdateAssetLike(ctx context.Context, id uuid.UUID, liked bool) error
	UpdateAssetRatingAndLike(ctx context.Context, id uuid.UUID, rating int, liked bool) error
	UpdateAssetDescription(ctx context.Context, id uuid.UUID, description string) error
	GetAssetsByRating(ctx context.Context, rating int, limit, offset int) ([]repo.Asset, error)
	GetLikedAssets(ctx context.Context, limit, offset int) ([]repo.Asset, error)

	AddAssetToAlbum(ctx context.Context, assetID uuid.UUID, albumID int) error
	RemoveAssetFromAlbum(ctx context.Context, assetID uuid.UUID, albumID int) error

	AddTagToAsset(ctx context.Context, assetID uuid.UUID, tagID int, confidence float32, source string) error
	RemoveTagFromAsset(ctx context.Context, assetID uuid.UUID, tagID int) error

	CreateThumbnail(ctx context.Context, assetID pgtype.UUID, size string, thumbnailPath string) (*repo.Thumbnail, error)
	DetectDuplicates(ctx context.Context, hash string) ([]repo.Asset, error)
	SaveAssetIndex(ctx context.Context, taskID string, hash string) error
	CreateAssetRecord(ctx context.Context, params repo.CreateAssetParams) (*repo.Asset, error)

	GetOrCreateTagByName(ctx context.Context, name, category string, isAIGenerated bool) (*repo.Tag, error)
	GetThumbnailByID(ctx context.Context, thumbnailID int) (*repo.Thumbnail, error)
	GetThumbnailByAssetIDAndSize(ctx context.Context, assetID uuid.UUID, size string) (*repo.Thumbnail, error)

	SaveNewAsset(ctx context.Context, fileReader io.Reader, filename string, hash string) (string, error)
	SaveNewThumbnail(ctx context.Context, repoPath string, buffers io.Reader, asset *repo.Asset, size string) error
	GetDistinctCameraMakes(ctx context.Context) ([]string, error)
	GetDistinctLenses(ctx context.Context) ([]string, error)

	// Video and Audio processing methods
	SaveVideoVersion(ctx context.Context, repoPath string, videoReader io.Reader, asset *repo.Asset, version string) error
	SaveAudioVersion(ctx context.Context, repoPath string, audioReader io.Reader, asset *repo.Asset, version string) error
	UpdateAssetDuration(ctx context.Context, id uuid.UUID, duration float64) error
	UpdateAssetDimensions(ctx context.Context, id uuid.UUID, width, height int32) error

	// Unified query API
	QueryAssets(ctx context.Context, params QueryAssetsParams) ([]repo.Asset, int64, error)
	QueryPhotoMapPoints(ctx context.Context, params QueryPhotoMapPointsParams) ([]PhotoMapPoint, int64, error)
}

// QueryAssetsParams contains all parameters for the unified asset query
type QueryAssetsParams struct {
	Query        string // Filename search query (empty for list-only)
	SearchType   string // "filename" (default) | "semantic"
	RepositoryID *string
	AssetType    *string  // Single type filter
	AssetTypes   []string // Multiple types filter
	OwnerID      *int32
	AlbumID      *int32
	DateFrom     *time.Time
	DateTo       *time.Time
	IsRaw        *bool
	Rating       *int
	Liked        *bool
	CameraModel  *string
	LensModel    *string
	GroupBy      string // Grouping strategy for server-side sorting (e.g., "type")
	Limit        int
	Offset       int
}

type QueryPhotoMapPointsParams struct {
	RepositoryID *string
	Limit        int
	Offset       int
}

type PhotoMapPoint struct {
	AssetID          string
	OriginalFilename string
	UploadTime       time.Time
	TakenTime        *time.Time
	GPSLatitude      float64
	GPSLongitude     float64
}

type assetService struct {
	queries          *repo.Queries
	lumen            LumenService
	repoManager      *storage.RepositoryManager
	embeddingService EmbeddingService
}

func NewAssetService(q *repo.Queries, l LumenService, r *storage.RepositoryManager, e EmbeddingService) (AssetService, error) {
	return &assetService{
		queries:          q,
		lumen:            l,
		repoManager:      r,
		embeddingService: e,
	}, nil
}

// ================================
// Asset CRUD Operations
// ================================

// CreateAssetRecord creates a new asset record in the database
func (s *assetService) CreateAssetRecord(ctx context.Context, params repo.CreateAssetParams) (*repo.Asset, error) {
	// Note: taken_time will be set to NULL initially and updated later when EXIF is processed
	// This is because we need to extract the time from the actual file content, not just the parameters
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

func (s *assetService) GetAssetWithOptions(ctx context.Context, id uuid.UUID, includeThumbnails, includeTags, includeAlbums, includeSpecies, includeOCR, includeFaces, includeCaptions bool) (interface{}, error) {
	pgUUID := pgtype.UUID{}
	if err := pgUUID.Scan(id.String()); err != nil {
		return nil, fmt.Errorf("invalid UUID: %w", err)
	}

	// 1) Full relations (thumbnails + tags + albums) OR species predictions OR any AI data requested
	if includeSpecies || includeOCR || includeFaces || includeCaptions || (includeThumbnails && includeTags && includeAlbums) {
		dbAsset, err := s.queries.GetAssetWithRelations(ctx, pgUUID)
		if err != nil {
			return nil, fmt.Errorf("failed to get asset with relations: %w", err)
		}

		// Create a map to conditionally include AI data based on flags
		result := map[string]interface{}{
			"asset_id":            dbAsset.AssetID,
			"owner_id":            dbAsset.OwnerID,
			"type":                dbAsset.Type,
			"original_filename":   dbAsset.OriginalFilename,
			"storage_path":        dbAsset.StoragePath,
			"mime_type":           dbAsset.MimeType,
			"file_size":           dbAsset.FileSize,
			"hash":                dbAsset.Hash,
			"width":               dbAsset.Width,
			"height":              dbAsset.Height,
			"duration":            dbAsset.Duration,
			"taken_time":          dbAsset.TakenTime,
			"upload_time":         dbAsset.UploadTime,
			"is_deleted":          dbAsset.IsDeleted,
			"deleted_at":          dbAsset.DeletedAt,
			"specific_metadata":   dbAsset.SpecificMetadata,
			"rating":              dbAsset.Rating,
			"liked":               dbAsset.Liked,
			"repository_id":       dbAsset.RepositoryID,
			"status":              dbAsset.Status,
			"thumbnails":          dbAsset.Thumbnails,
			"tags":                dbAsset.Tags,
			"albums":              dbAsset.Albums,
			"species_predictions": dbAsset.SpeciesPredictions,
		}

		// Only include AI data if specifically requested
		if includeOCR {
			result["ocr_result"] = dbAsset.OcrResult
		}
		if includeFaces {
			result["face_result"] = dbAsset.FaceResult
		}
		if includeCaptions {
			result["caption"] = dbAsset.Caption
		}

		return result, nil
	}

	// 2) Thumbnails + Tags (albums not requested) -> still use relations query (albums will be empty in SQL)
	if includeThumbnails && includeTags {
		dbAsset, err := s.queries.GetAssetWithRelations(ctx, pgUUID)
		if err != nil {
			return nil, fmt.Errorf("failed to get asset with relations: %w", err)
		}
		return dbAsset, nil
	}

	// 3) Any case where albums are requested (but not both thumbnails & tags simultaneously handled above)
	//    Manually compose result to avoid creating many specialized SQL queries.
	if includeAlbums {
		asset, err := s.queries.GetAssetByID(ctx, pgUUID)
		if err != nil {
			return nil, fmt.Errorf("failed to get asset: %w", err)
		}

		// Thumbnails (optional)
		var thumbnails interface{} = []interface{}{}
		if includeThumbnails {
			tList, err := s.queries.GetThumbnailsByAsset(ctx, pgUUID)
			if err != nil {
				return nil, fmt.Errorf("failed to get thumbnails: %w", err)
			}
			thumbnails = tList
		}

		// Tags (optional)
		var tags interface{} = []interface{}{}
		if includeTags {
			tagsRow, err := s.queries.GetAssetWithTags(ctx, pgUUID)
			if err != nil {
				return nil, fmt.Errorf("failed to get tags: %w", err)
			}
			tags = tagsRow.Tags
		}

		// Albums
		albums, err := s.queries.GetAssetAlbums(ctx, pgUUID)
		if err != nil {
			return nil, fmt.Errorf("failed to get asset albums: %w", err)
		}

		result := map[string]interface{}{
			"asset_id":          asset.AssetID,
			"owner_id":          asset.OwnerID,
			"type":              asset.Type,
			"original_filename": asset.OriginalFilename,
			"storage_path":      asset.StoragePath,
			"mime_type":         asset.MimeType,
			"file_size":         asset.FileSize,
			"hash":              asset.Hash,
			"width":             asset.Width,
			"height":            asset.Height,
			"duration":          asset.Duration,
			"upload_time":       asset.UploadTime,
			"is_deleted":        asset.IsDeleted,
			"deleted_at":        asset.DeletedAt,
			"specific_metadata": asset.SpecificMetadata,
			"thumbnails":        thumbnails,
			"tags":              tags,
			"albums":            albums,
		}
		return result, nil
	}

	// 4) Only thumbnails
	if includeThumbnails {
		dbAsset, err := s.queries.GetAssetWithThumbnails(ctx, pgUUID)
		if err != nil {
			return nil, fmt.Errorf("failed to get asset with thumbnails: %w", err)
		}
		return dbAsset, nil
	}

	// 5) Only tags
	if includeTags {
		dbAsset, err := s.queries.GetAssetWithTags(ctx, pgUUID)
		if err != nil {
			return nil, fmt.Errorf("failed to get asset with tags: %w", err)
		}
		return dbAsset, nil
	}

	// 6) Plain asset
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

// GetAssetsByOwnerSorted retrieves assets by owner sorted by taken_time
func (s *assetService) GetAssetsByOwnerSorted(ctx context.Context, ownerID int, sortOrder string, limit, offset int) ([]repo.Asset, error) {
	params := repo.GetAssetsByOwnerSortedParams{
		OwnerID: int32PtrFromIntPtr(&ownerID),
		Column2: sortOrder,
		Limit:   int32(limit),
		Offset:  int32(offset),
	}

	return s.queries.GetAssetsByOwnerSorted(ctx, params)
}

// GetAssetsByTypesSorted retrieves assets by multiple types sorted by taken_time
func (s *assetService) GetAssetsByTypesSorted(ctx context.Context, assetTypes []string, sortOrder string, limit, offset int) ([]repo.Asset, error) {
	params := repo.GetAssetsByTypesSortedParams{
		Types:     assetTypes,
		SortOrder: sortOrder,
		Limit:     int32(limit),
		Offset:    int32(offset),
	}

	return s.queries.GetAssetsByTypesSorted(ctx, params)
}

// GetAssetsByOwnerAndTypes retrieves assets by owner and multiple types sorted by taken_time
func (s *assetService) GetAssetsByOwnerAndTypes(ctx context.Context, ownerID int, assetTypes []string, sortOrder string, limit, offset int) ([]repo.Asset, error) {
	params := repo.GetAssetsByOwnerAndTypesSortedParams{
		OwnerID:   int32PtrFromIntPtr(&ownerID),
		Types:     assetTypes,
		SortOrder: sortOrder,
		Limit:     int32(limit),
		Offset:    int32(offset),
	}

	return s.queries.GetAssetsByOwnerAndTypesSorted(ctx, params)
}

// DetectDuplicates finds assets with the same hash
func (s *assetService) DetectDuplicates(ctx context.Context, hash string) ([]repo.Asset, error) {
	return s.queries.GetAssetsByHash(ctx, &hash)
}

// UpdateAssetMetadata updates the specific metadata of an asset and extracts taken_time
func (s *assetService) UpdateAssetMetadata(ctx context.Context, id uuid.UUID, metadata dbtypes.SpecificMetadata) error {
	pgUUID := pgtype.UUID{}
	if err := pgUUID.Scan(id.String()); err != nil {
		return fmt.Errorf("invalid UUID: %w", err)
	}

	// Get the asset to determine its type for taken_time extraction
	asset, err := s.queries.GetAssetByID(ctx, pgUUID)
	if err != nil {
		return fmt.Errorf("failed to get asset for metadata update: %w", err)
	}

	// Extract taken_time from metadata based on asset type
	var takenTime *time.Time
	assetType := dbtypes.AssetType(asset.Type)

	switch assetType {
	case dbtypes.AssetTypePhoto:
		if photoMeta, err := metadata.UnmarshalPhoto(); err == nil {
			takenTime = photoMeta.TakenTime
		}
	case dbtypes.AssetTypeVideo:
		if videoMeta, err := metadata.UnmarshalVideo(); err == nil {
			takenTime = videoMeta.RecordedTime
		}
	case dbtypes.AssetTypeAudio:
		// Audio doesn't have taken time
		takenTime = nil
	}

	// Use the new query that updates both metadata and taken_time
	var takenTimeParam pgtype.Timestamptz
	if takenTime != nil {
		takenTimeParam = pgtype.Timestamptz{
			Time:  *takenTime,
			Valid: true,
		}
	}

	params := repo.UpdateAssetMetadataWithTakenTimeParams{
		AssetID:          pgUUID,
		SpecificMetadata: metadata,
		TakenTime:        takenTimeParam,
	}

	return s.queries.UpdateAssetMetadataWithTakenTime(ctx, params)
}

// DeleteAsset marks an asset as deleted, and move the asset to the trash folder
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

// SaveNewAsset is deprecated - assets are now saved through repository staging system
// This is kept for backward compatibility but should not be used
func (s *assetService) SaveNewAsset(ctx context.Context, fileReader io.Reader, filename string, hash string) (string, error) {
	return "", fmt.Errorf("SaveNewAsset is deprecated - use repository staging system instead")
}

// ================================
// Thumbnail CRUD Operations
// ================================

// CreateThumbnail creates a new thumbnail for an asset
func (s *assetService) CreateThumbnail(ctx context.Context, assetID pgtype.UUID, size string, thumbnailPath string) (*repo.Thumbnail, error) {
	params := repo.CreateThumbnailParams{
		AssetID:     assetID,
		Size:        size,
		StoragePath: thumbnailPath,
		MimeType:    "image/webp",
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

// SaveNewThumbnail saves thumbnail file to repository and creates database record
//
// asset repo.Asset must be valid in following cases:
//   - asset ID is not empty
//   - asset hash is not empty
//   - asset storage path is not empty
func (s *assetService) SaveNewThumbnail(ctx context.Context, repoPath string, buffers io.Reader, asset *repo.Asset, size string) error {
	// Require: valid inputs
	if buffers == nil {
		return fmt.Errorf("buffers cannot be nil")
	}
	if asset == nil {
		return fmt.Errorf("asset cannot be nil")
	}
	if size == "" {
		return fmt.Errorf("size cannot be empty")
	}
	if asset.Hash == nil || *asset.Hash == "" {
		return fmt.Errorf("asset hash is required")
	}
	if repoPath == "" {
		return fmt.Errorf("repository path is required")
	}

	// Generate thumbnail filename using hash and size
	filename := fmt.Sprintf("%s_%s.webp", *asset.Hash, size)

	// Construct full path: .lumilio/assets/thumbnails/{size}/{hash}_{size}.webp
	thumbnailDir := filepath.Join(repoPath, ".lumilio/assets/thumbnails", size)
	thumbnailPath := filepath.Join(thumbnailDir, filename)

	// Ensure directory exists
	if err := os.MkdirAll(thumbnailDir, 0755); err != nil {
		return fmt.Errorf("failed to create thumbnail directory: %w", err)
	}

	// Write the thumbnail file
	file, err := os.Create(thumbnailPath)
	if err != nil {
		return fmt.Errorf("failed to create thumbnail file: %w", err)
	}
	defer file.Close()

	written, err := io.Copy(file, buffers)
	if err != nil {
		// Clean up partial file on error
		os.Remove(thumbnailPath)
		return fmt.Errorf("failed to write thumbnail: %w", err)
	}

	// Ensure: file was written
	if written == 0 {
		os.Remove(thumbnailPath)
		return fmt.Errorf("no data written for thumbnail")
	}

	assetUUID, _ := uuid.FromBytes(asset.AssetID.Bytes[:])
	log.Printf("Saved thumbnail for asset %s: size=%s, path=%s, bytes=%d", assetUUID.String(), size, thumbnailPath, written)

	// Create database record with relative path
	relPath := filepath.Join(".lumilio/assets/thumbnails", size, filename)
	_, err = s.CreateThumbnail(ctx, asset.AssetID, size, relPath)
	if err != nil {
		// Clean up file if database insertion fails
		os.Remove(thumbnailPath)
		return fmt.Errorf("failed to create thumbnail database record: %w", err)
	}

	return nil
}

// ================================
// Helper functions
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

// Helper function for filename matching
func matchFilename(filename, pattern, mode string) bool {
	switch mode {
	case "contains":
		return strings.Contains(strings.ToLower(filename), strings.ToLower(pattern))
	case "startswith":
		return strings.HasPrefix(strings.ToLower(filename), strings.ToLower(pattern))
	case "endswith":
		return strings.HasSuffix(strings.ToLower(filename), strings.ToLower(pattern))
	case "matches":
		// Could implement regex matching here if needed
		return strings.Contains(strings.ToLower(filename), strings.ToLower(pattern))
	default:
		return strings.Contains(strings.ToLower(filename), strings.ToLower(pattern))
	}
}

func (s *assetService) GetDistinctCameraMakes(ctx context.Context) ([]string, error) {
	rows, err := s.queries.GetDistinctCameraMakes(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get distinct camera makes: %w", err)
	}

	makes := make([]string, 0, len(rows))
	for _, row := range rows {
		if str, ok := row.(string); ok && str != "" {
			makes = append(makes, str)
		}
	}

	return makes, nil
}

func (s *assetService) GetDistinctLenses(ctx context.Context) ([]string, error) {
	results, err := s.queries.GetDistinctLenses(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get distinct lenses: %w", err)
	}

	lenses := make([]string, 0, len(results))
	for _, result := range results {
		if lens, ok := result.(string); ok && lens != "" {
			lenses = append(lenses, lens)
		}
	}

	return lenses, nil
}

// Rating management methods implementation

func (s *assetService) UpdateAssetRating(ctx context.Context, id uuid.UUID, rating int) error {
	pgUUID := pgtype.UUID{}
	if err := pgUUID.Scan(id.String()); err != nil {
		return fmt.Errorf("invalid UUID: %w", err)
	}

	params := repo.UpdateAssetRatingParams{
		AssetID: pgUUID,
		Rating:  int32(rating),
	}

	return s.queries.UpdateAssetRating(ctx, params)
}

func (s *assetService) UpdateAssetLike(ctx context.Context, id uuid.UUID, liked bool) error {
	pgUUID := pgtype.UUID{}
	if err := pgUUID.Scan(id.String()); err != nil {
		return fmt.Errorf("invalid UUID: %w", err)
	}

	params := repo.UpdateAssetLikeParams{
		AssetID: pgUUID,
		Liked:   liked,
	}

	return s.queries.UpdateAssetLike(ctx, params)
}

func (s *assetService) UpdateAssetRatingAndLike(ctx context.Context, id uuid.UUID, rating int, liked bool) error {
	pgUUID := pgtype.UUID{}
	if err := pgUUID.Scan(id.String()); err != nil {
		return fmt.Errorf("invalid UUID: %w", err)
	}

	params := repo.UpdateAssetRatingAndLikeParams{
		AssetID: pgUUID,
		Rating:  int32(rating),
		Liked:   liked,
	}

	return s.queries.UpdateAssetRatingAndLike(ctx, params)
}

func (s *assetService) UpdateAssetDescription(ctx context.Context, id uuid.UUID, description string) error {
	pgUUID := pgtype.UUID{}
	if err := pgUUID.Scan(id.String()); err != nil {
		return fmt.Errorf("invalid UUID: %w", err)
	}

	params := repo.UpdateAssetDescriptionParams{
		AssetID:     pgUUID,
		Description: description,
	}

	return s.queries.UpdateAssetDescription(ctx, params)
}

func (s *assetService) GetAssetsByRating(ctx context.Context, rating int, limit, offset int) ([]repo.Asset, error) {
	params := repo.GetAssetsByRatingParams{
		Rating: int32(rating),
		Limit:  int32(limit),
		Offset: int32(offset),
	}

	return s.queries.GetAssetsByRating(ctx, params)
}

func (s *assetService) GetLikedAssets(ctx context.Context, limit, offset int) ([]repo.Asset, error) {
	params := repo.GetLikedAssetsParams{
		Limit:  int32(limit),
		Offset: int32(offset),
	}

	return s.queries.GetLikedAssets(ctx, params)
}

// SaveVideoVersion Video and Audio processing methods implementation
//
// asset repo.Asset must be valid in following cases:
//   - asset ID is not empty
//   - asset hash is not empty
//   - asset storage path is not empty
func (s *assetService) SaveVideoVersion(ctx context.Context, repoPath string, videoReader io.Reader, asset *repo.Asset, version string) error {
	// Require: valid inputs
	if videoReader == nil {
		return fmt.Errorf("videoReader cannot be nil")
	}
	if asset == nil {
		return fmt.Errorf("asset cannot be nil")
	}
	if version == "" {
		return fmt.Errorf("version cannot be empty")
	}
	if asset.Hash == nil || *asset.Hash == "" {
		return fmt.Errorf("asset hash is required")
	}
	if repoPath == "" {
		return fmt.Errorf("repository path is required")
	}

	// Generate filename using hash and version
	filename := fmt.Sprintf("%s_%s.mp4", *asset.Hash, version)

	// Construct full path: .lumilio/assets/videos/web/{hash}_{version}.mp4
	videoDir := filepath.Join(repoPath, ".lumilio/assets/videos", version)
	videoPath := filepath.Join(videoDir, filename)

	// Ensure directory exists
	if err := os.MkdirAll(videoDir, 0755); err != nil {
		return fmt.Errorf("failed to create video directory: %w", err)
	}

	// Write the video file
	file, err := os.Create(videoPath)
	if err != nil {
		return fmt.Errorf("failed to create video file: %w", err)
	}
	defer file.Close()

	written, err := io.Copy(file, videoReader)
	if err != nil {
		// Clean up partial file on error
		os.Remove(videoPath)
		return fmt.Errorf("failed to write video: %w", err)
	}

	// Ensure: file was written
	if written == 0 {
		os.Remove(videoPath)
		return fmt.Errorf("no data written for video version")
	}

	assetUUID, _ := uuid.FromBytes(asset.AssetID.Bytes[:])
	log.Printf("Saved video version %s for asset %s at path %s, bytes=%d", version, assetUUID.String(), videoPath, written)
	return nil
}

// SaveAudioVersion saves an audio version of an asset.
//
// asset repo.Asset must be valid in following cases:
//   - asset ID is not empty
//   - asset hash is not empty
//   - asset storage path is not empty
func (s *assetService) SaveAudioVersion(ctx context.Context, repoPath string, audioReader io.Reader, asset *repo.Asset, version string) error {
	// Require: valid inputs
	if audioReader == nil {
		return fmt.Errorf("audioReader cannot be nil")
	}
	if asset == nil {
		return fmt.Errorf("asset cannot be nil")
	}
	if version == "" {
		return fmt.Errorf("version cannot be empty")
	}
	if asset.Hash == nil || *asset.Hash == "" {
		return fmt.Errorf("asset hash is required")
	}
	if repoPath == "" {
		return fmt.Errorf("repository path is required")
	}

	// Generate filename using hash and version
	filename := fmt.Sprintf("%s_%s.mp3", *asset.Hash, version)

	// Construct full path: .lumilio/assets/audios/web/{hash}_{version}.mp3
	audioDir := filepath.Join(repoPath, ".lumilio/assets/audios", version)
	audioPath := filepath.Join(audioDir, filename)

	// Ensure directory exists
	if err := os.MkdirAll(audioDir, 0755); err != nil {
		return fmt.Errorf("failed to create audio directory: %w", err)
	}

	// Write the audio file
	file, err := os.Create(audioPath)
	if err != nil {
		return fmt.Errorf("failed to create audio file: %w", err)
	}
	defer file.Close()

	written, err := io.Copy(file, audioReader)
	if err != nil {
		// Clean up partial file on error
		os.Remove(audioPath)
		return fmt.Errorf("failed to write audio: %w", err)
	}

	// Ensure: file was written
	if written == 0 {
		os.Remove(audioPath)
		return fmt.Errorf("no data written for audio version")
	}

	assetUUID, _ := uuid.FromBytes(asset.AssetID.Bytes[:])
	log.Printf("Saved audio version %s for asset %s at path %s, bytes=%d", version, assetUUID.String(), audioPath, written)
	return nil
}

func (s *assetService) UpdateAssetDuration(ctx context.Context, id uuid.UUID, duration float64) error {
	pgUUID := pgtype.UUID{}
	if err := pgUUID.Scan(id.String()); err != nil {
		return fmt.Errorf("invalid UUID: %w", err)
	}

	params := repo.UpdateAssetDurationParams{
		AssetID:  pgUUID,
		Duration: &duration,
	}

	return s.queries.UpdateAssetDuration(ctx, params)
}

func (s *assetService) UpdateAssetDimensions(ctx context.Context, id uuid.UUID, width, height int32) error {
	pgUUID := pgtype.UUID{}
	if err := pgUUID.Scan(id.String()); err != nil {
		return fmt.Errorf("invalid UUID: %w", err)
	}

	params := repo.UpdateAssetDimensionsParams{
		AssetID: pgUUID,
		Width:   &width,
		Height:  &height,
	}

	return s.queries.UpdateAssetDimensions(ctx, params)
}

// ================================
// Unified Query API
// ================================

// QueryAssets is the unified method for listing, filtering, and searching assets.
func (s *assetService) QueryAssets(ctx context.Context, params QueryAssetsParams) ([]repo.Asset, int64, error) {
	if params.SearchType == "semantic" && params.Query != "" {
		return s.queryAssetsVector(ctx, params)
	}
	return s.queryAssetsUnified(ctx, params)
}

func (s *assetService) queryAssetsUnified(ctx context.Context, params QueryAssetsParams) ([]repo.Asset, int64, error) {
	var repoUUID pgtype.UUID
	if params.RepositoryID != nil && *params.RepositoryID != "" {
		parsedUUID, err := uuid.Parse(*params.RepositoryID)
		if err != nil {
			return nil, 0, fmt.Errorf("invalid repository ID: %w", err)
		}
		repoUUID = pgtype.UUID{Bytes: parsedUUID, Valid: true}
	}

	var ratingPtr *int32
	if params.Rating != nil {
		r := int32(*params.Rating)
		ratingPtr = &r
	}

	var fromTime, toTime pgtype.Timestamptz
	if params.DateFrom != nil {
		fromTime = pgtype.Timestamptz{Time: *params.DateFrom, Valid: true}
	}
	if params.DateTo != nil {
		toTime = pgtype.Timestamptz{Time: *params.DateTo, Valid: true}
	}

	var queryPtr *string
	if params.Query != "" {
		queryPtr = &params.Query
	}

	// Determine SortBy based on GroupBy for server-side sorting
	// This ensures that when grouping by type, all assets of the same type
	// are returned together, maintaining group continuity across pagination
	var sortByPtr *string
	if params.GroupBy == "type" {
		s := "type"
		sortByPtr = &s
	}
	// For other GroupBy values (e.g., "date", "album", or empty),
	// sortByPtr remains nil, which defaults to time-based sorting in SQL

	// Get total count
	countResult, err := s.queries.CountAssetsUnified(ctx, repo.CountAssetsUnifiedParams{
		AssetType:    params.AssetType,
		AssetTypes:   params.AssetTypes,
		RepositoryID: repoUUID,
		OwnerID:      params.OwnerID,
		AlbumID:      params.AlbumID,
		Query:        queryPtr,
		IsRaw:        params.IsRaw,
		Rating:       ratingPtr,
		Liked:        params.Liked,
		CameraModel:  params.CameraModel,
		LensModel:    params.LensModel,
		DateFrom:     fromTime,
		DateTo:       toTime,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count assets: %w", err)
	}

	// Get assets
	assets, err := s.queries.GetAssetsUnified(ctx, repo.GetAssetsUnifiedParams{
		AssetType:    params.AssetType,
		AssetTypes:   params.AssetTypes,
		RepositoryID: repoUUID,
		OwnerID:      params.OwnerID,
		AlbumID:      params.AlbumID,
		Query:        queryPtr,
		IsRaw:        params.IsRaw,
		Rating:       ratingPtr,
		Liked:        params.Liked,
		CameraModel:  params.CameraModel,
		LensModel:    params.LensModel,
		SortBy:       sortByPtr,
		DateFrom:     fromTime,
		DateTo:       toTime,
		Limit:        int32(params.Limit),
		Offset:       int32(params.Offset),
	})
	if err != nil {
		return nil, 0, err
	}

	return assets, countResult, nil
}

func (s *assetService) queryAssetsVector(ctx context.Context, params QueryAssetsParams) ([]repo.Asset, int64, error) {
	if s.lumen == nil {
		return nil, 0, fmt.Errorf("%w: lumen service not available", ErrSemanticSearchUnavailable)
	}
	if !s.lumen.IsTaskAvailable("clip_text_embed") {
		return nil, 0, fmt.Errorf("%w: clip_text_embed task not available", ErrSemanticSearchUnavailable)
	}

	embeddingResult, err := s.lumen.ClipTextEmbed(ctx, []byte(params.Query))
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get query embedding: %w", err)
	}

	queryVector := make([]float32, len(embeddingResult.Vector))
	for i, v := range embeddingResult.Vector {
		queryVector[i] = float32(v)
	}

	pgVector := pgvector.NewVector(queryVector)

	var repoUUID pgtype.UUID
	if params.RepositoryID != nil && *params.RepositoryID != "" {
		parsedUUID, err := uuid.Parse(*params.RepositoryID)
		if err != nil {
			return nil, 0, fmt.Errorf("invalid repository ID: %w", err)
		}
		repoUUID = pgtype.UUID{Bytes: parsedUUID, Valid: true}
	}

	maxDistance := 0.5
	if v := os.Getenv("SEMANTIC_MAX_DISTANCE"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 {
			maxDistance = f
		}
	}

	var ratingPtr *int32
	if params.Rating != nil {
		r := int32(*params.Rating)
		ratingPtr = &r
	}

	var fromTime, toTime pgtype.Timestamptz
	if params.DateFrom != nil {
		fromTime = pgtype.Timestamptz{Time: *params.DateFrom, Valid: true}
	}
	if params.DateTo != nil {
		toTime = pgtype.Timestamptz{Time: *params.DateTo, Valid: true}
	}

	// Get total count
	countResult, err := s.queries.CountAssetsVectorUnified(ctx, repo.CountAssetsVectorUnifiedParams{
		Embedding:     &pgVector,
		EmbeddingType: string(EmbeddingTypeCLIP),
		MaxDistance:   &maxDistance,
		AssetType:     params.AssetType,
		AssetTypes:    params.AssetTypes,
		RepositoryID:  repoUUID,
		OwnerID:       params.OwnerID,
		AlbumID:       params.AlbumID,
		IsRaw:         params.IsRaw,
		Rating:        ratingPtr,
		Liked:         params.Liked,
		CameraModel:   params.CameraModel,
		LensModel:     params.LensModel,
		DateFrom:      fromTime,
		DateTo:        toTime,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count assets: %w", err)
	}

	// Get assets
	results, err := s.queries.SearchAssetsVectorUnified(ctx, repo.SearchAssetsVectorUnifiedParams{
		Embedding:     &pgVector,
		EmbeddingType: string(EmbeddingTypeCLIP),
		MaxDistance:   &maxDistance,
		AssetType:     params.AssetType,
		AssetTypes:    params.AssetTypes,
		RepositoryID:  repoUUID,
		OwnerID:       params.OwnerID,
		AlbumID:       params.AlbumID,
		IsRaw:         params.IsRaw,
		Rating:        ratingPtr,
		Liked:         params.Liked,
		CameraModel:   params.CameraModel,
		LensModel:     params.LensModel,
		DateFrom:      fromTime,
		DateTo:        toTime,
		Limit:         int32(params.Limit),
		Offset:        int32(params.Offset),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("failed to search assets: %w", err)
	}

	assets := make([]repo.Asset, len(results))
	for i, r := range results {
		assets[i] = repo.Asset{
			AssetID: r.AssetID, OwnerID: r.OwnerID, Type: r.Type,
			OriginalFilename: r.OriginalFilename, StoragePath: r.StoragePath,
			MimeType: r.MimeType, FileSize: r.FileSize, Hash: r.Hash,
			Width: r.Width, Height: r.Height, Duration: r.Duration,
			TakenTime: r.TakenTime, UploadTime: r.UploadTime,
			IsDeleted: r.IsDeleted, DeletedAt: r.DeletedAt,
			SpecificMetadata: r.SpecificMetadata, Rating: r.Rating,
			Liked: r.Liked, RepositoryID: r.RepositoryID, Status: r.Status,
		}
	}
	return assets, countResult, nil
}

func (s *assetService) QueryPhotoMapPoints(ctx context.Context, params QueryPhotoMapPointsParams) ([]PhotoMapPoint, int64, error) {
	var repoUUID pgtype.UUID
	if params.RepositoryID != nil && *params.RepositoryID != "" {
		parsedUUID, err := uuid.Parse(*params.RepositoryID)
		if err != nil {
			return nil, 0, fmt.Errorf("invalid repository ID: %w", err)
		}
		repoUUID = pgtype.UUID{Bytes: parsedUUID, Valid: true}
	}

	total, err := s.queries.CountPhotoMapPoints(ctx, repoUUID)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count photo map points: %w", err)
	}

	rows, err := s.queries.GetPhotoMapPoints(ctx, repo.GetPhotoMapPointsParams{
		RepositoryID: repoUUID,
		Limit:        int32(params.Limit),
		Offset:       int32(params.Offset),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query photo map points: %w", err)
	}

	points := make([]PhotoMapPoint, 0, len(rows))
	for _, row := range rows {
		if !row.AssetID.Valid || !row.UploadTime.Valid {
			continue
		}

		assetID, err := uuid.FromBytes(row.AssetID.Bytes[:])
		if err != nil {
			continue
		}

		var takenTime *time.Time
		if row.TakenTime.Valid {
			t := row.TakenTime.Time
			takenTime = &t
		}

		points = append(points, PhotoMapPoint{
			AssetID:          assetID.String(),
			OriginalFilename: row.OriginalFilename,
			UploadTime:       row.UploadTime.Time,
			TakenTime:        takenTime,
			GPSLatitude:      row.GpsLatitude,
			GPSLongitude:     row.GpsLongitude,
		})
	}

	return points, total, nil
}
