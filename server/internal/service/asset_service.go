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
	SearchAssets(ctx context.Context, query string, assetType *string, useVector bool, limit, offset int) ([]repo.Asset, error)
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

	// New filtering and search methods
	FilterAssets(ctx context.Context, assetType *string, ownerID *int32, filenameVal *string, filenameMode *string, dateFrom *time.Time, dateTo *time.Time, isRaw *bool, rating *int, liked *bool, cameraMake *string, lens *string, limit int, offset int) ([]repo.Asset, error)
	SearchAssetsFilename(ctx context.Context, query string, assetType *string, ownerID *int32, filenameVal *string, filenameMode *string, dateFrom *time.Time, dateTo *time.Time, isRaw *bool, rating *int, liked *bool, cameraMake *string, lens *string, limit int, offset int) ([]repo.Asset, error)
	SearchAssetsVector(ctx context.Context, query string, assetType *string, ownerID *int32, filenameVal *string, filenameMode *string, dateFrom *time.Time, dateTo *time.Time, isRaw *bool, rating *int, liked *bool, cameraMake *string, lens *string, limit int, offset int) ([]repo.Asset, error)
	GetDistinctCameraMakes(ctx context.Context) ([]string, error)
	GetDistinctLenses(ctx context.Context) ([]string, error)
}

type assetService struct {
	queries *repo.Queries
	storage storage.Storage
	ml      *MLService
}

// NewAssetService creates a new instance of AssetService with storage configuration
func NewAssetService(q *repo.Queries, s storage.Storage) (AssetService, error) {
	return NewAssetServiceWithML(q, s, nil)
}

func NewAssetServiceWithML(q *repo.Queries, s storage.Storage, ml *MLService) (AssetService, error) {
	return &assetService{
		queries: q,
		storage: s,
		ml:      ml,
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
func (s *assetService) SearchAssets(ctx context.Context, query string, assetType *string, useVector bool, limit, offset int) ([]repo.Asset, error) {
	log.Printf("SearchAssets: query=%q type=%v useVector=%t limit=%d offset=%d", query, func() interface{} {
		if assetType != nil {
			return *assetType
		}
		return nil
	}(), useVector, limit, offset)
	// Try smart vector search first when we have an ML client and a non-empty query.
	if useVector && s.ml != nil && query != "" {
		if emb, err := s.ml.ClipEmbed(ctx, query); err == nil && emb != nil && len(emb.Vector) > 0 {
			// Convert []float64 -> []float32 to fit pgvector-go.
			fv := make([]float32, len(emb.Vector))
			for i, v := range emb.Vector {
				fv[i] = float32(v)
			}
			vec := pgvector_go.NewVector(fv)

			// We fetch limit+offset items from ANN, then apply optional type filter and offset locally.
			fetch := limit + offset
			if fetch <= 0 {
				fetch = limit
			}
			if fetch <= 0 {
				fetch = 50
			}

			rows, vErr := s.queries.SearchNearestAssets(ctx, repo.SearchNearestAssetsParams{
				Column1: vec,
				Limit:   int32(fetch),
			})
			if vErr != nil {
				log.Printf("Vector search: ANN error: %v", vErr)
			} else {
				log.Printf("Vector search: ANN returned %d candidates", len(rows))
			}
			if vErr == nil && len(rows) > 0 {
				// Concurrently fetch assets to reduce latency, then filter in original ANN order.
				type fetchRes struct {
					idx   int
					asset *repo.Asset
				}
				total := len(rows)
				fetched := make([]*repo.Asset, total)
				sem := make(chan struct{}, 8) // limit concurrent DB fetches
				out := make(chan fetchRes, total)

				for i, r := range rows {
					sem <- struct{}{}
					go func(i int, id pgtype.UUID) {
						defer func() { <-sem }()
						a, err := s.queries.GetAssetByID(ctx, id)
						if err == nil {
							out <- fetchRes{idx: i, asset: &a}
						} else {
							out <- fetchRes{idx: i, asset: nil}
						}
					}(i, r.AssetID)
				}

				// Collect all fetch results
				for i := 0; i < total; i++ {
					res := <-out
					if res.asset != nil {
						fetched[res.idx] = res.asset
					}
				}

				// Log how many assets were successfully fetched from DB
				countFetched := 0
				for _, fa := range fetched {
					if fa != nil {
						countFetched++
					}
				}
				log.Printf("Vector search: fetched %d/%d assets from DB", countFetched, total)
				results := make([]repo.Asset, 0, limit)
				skipped := 0
				for i := 0; i < total && len(results) < limit; i++ {
					a := fetched[i]
					if a == nil {
						continue
					}
					// Optional type filter
					if assetType != nil && *assetType != "" && a.Type != *assetType {
						continue
					}
					// Apply offset after filtering
					if skipped < offset {
						skipped++
						continue
					}
					results = append(results, *a)
				}

				log.Printf("Vector search: after filtering (type=%v) and offset=%d -> skipped=%d returned=%d", func() interface{} {
					if assetType != nil {
						return *assetType
					}
					return nil
				}(), offset, skipped, len(results))
				if len(results) > 0 {
					return results, nil
				}
			}
		}
	}
	log.Printf("Vector search: no usable vector results; falling back to filename search")
	// Fallback to filename search.
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

// SaveNewThumbnail TODO: Refine this
func (s *assetService) SaveNewThumbnail(ctx context.Context, buffers io.Reader, asset *repo.Asset, size string) error {
	// TODO: Upload Thumbnail to different folder
	storagePath, err := s.storage.UploadWithMetadata(ctx, buffers, asset.OriginalFilename+"_"+size, "")
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

// ================================
// New filtering and search methods
// ================================

func (s *assetService) FilterAssets(ctx context.Context, assetType *string, ownerID *int32, filenameVal *string, filenameMode *string, dateFrom *time.Time, dateTo *time.Time, isRaw *bool, rating *int, liked *bool, cameraMake *string, lens *string, limit int, offset int) ([]repo.Asset, error) {
	// Convert rating pointer for SQL
	var ratingPtr *int32
	if rating != nil {
		r := int32(*rating)
		ratingPtr = &r
	}

	// Convert dates to pgtype.Timestamptz
	var fromTime, toTime pgtype.Timestamptz
	if dateFrom != nil {
		fromTime = pgtype.Timestamptz{Time: *dateFrom, Valid: true}
	}
	if dateTo != nil {
		toTime = pgtype.Timestamptz{Time: *dateTo, Valid: true}
	}

	return s.queries.FilterAssets(ctx, repo.FilterAssetsParams{
		AssetType:    assetType,
		OwnerID:      ownerID,
		FilenameVal:  filenameVal,
		FilenameMode: filenameMode,
		DateFrom:     fromTime,
		DateTo:       toTime,
		IsRaw:        isRaw,
		Rating:       ratingPtr,
		Liked:        liked,
		CameraModel:  cameraMake,
		LensModel:    lens,
		Limit:        int32(limit),
		Offset:       int32(offset),
	})
}

func (s *assetService) SearchAssetsFilename(ctx context.Context, query string, assetType *string, ownerID *int32, filenameVal *string, filenameMode *string, dateFrom *time.Time, dateTo *time.Time, isRaw *bool, rating *int, liked *bool, cameraMake *string, lens *string, limit int, offset int) ([]repo.Asset, error) {
	// Convert rating pointer for SQL
	var ratingPtr *int32
	if rating != nil {
		r := int32(*rating)
		ratingPtr = &r
	}

	// Convert dates to pgtype.Timestamptz
	var fromTime, toTime pgtype.Timestamptz
	if dateFrom != nil {
		fromTime = pgtype.Timestamptz{Time: *dateFrom, Valid: true}
	}
	if dateTo != nil {
		toTime = pgtype.Timestamptz{Time: *dateTo, Valid: true}
	}

	return s.queries.SearchAssetsFilename(ctx, repo.SearchAssetsFilenameParams{
		Query:        &query,
		AssetType:    assetType,
		OwnerID:      ownerID,
		FilenameVal:  filenameVal,
		FilenameMode: filenameMode,
		DateFrom:     fromTime,
		DateTo:       toTime,
		IsRaw:        isRaw,
		Rating:       ratingPtr,
		Liked:        liked,
		CameraModel:  cameraMake,
		LensModel:    lens,
		Limit:        int32(limit),
		Offset:       int32(offset),
	})
}

func (s *assetService) SearchAssetsVector(ctx context.Context, query string, assetType *string, ownerID *int32, filenameVal *string, filenameMode *string, dateFrom *time.Time, dateTo *time.Time, isRaw *bool, rating *int, liked *bool, cameraMake *string, lens *string, limit int, offset int) ([]repo.Asset, error) {
	if s.ml == nil {
		return nil, fmt.Errorf("ML service not available for semantic search")
	}

	// Get query embedding
	embeddingResult, err := s.ml.ClipEmbed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get query embedding: %w", err)
	}

	// Convert rating pointer for SQL
	var ratingPtr *int32
	if rating != nil {
		r := int32(*rating)
		ratingPtr = &r
	}

	// Convert dates to pgtype.Timestamptz
	var fromTime, toTime pgtype.Timestamptz
	if dateFrom != nil {
		fromTime = pgtype.Timestamptz{Time: *dateFrom, Valid: true}
	}
	if dateTo != nil {
		toTime = pgtype.Timestamptz{Time: *dateTo, Valid: true}
	}

	// Convert embedding to pgvector format
	pgEmbeddingFloat32 := make([]float32, len(embeddingResult.Vector))
	for i, v := range embeddingResult.Vector {
		pgEmbeddingFloat32[i] = float32(v)
	}
	pgEmbedding := pgvector_go.NewVector(pgEmbeddingFloat32)

	results, err := s.queries.SearchAssetsVector(ctx, repo.SearchAssetsVectorParams{
		Embedding:    pgEmbedding,
		AssetType:    assetType,
		OwnerID:      ownerID,
		FilenameVal:  filenameVal,
		FilenameMode: filenameMode,
		DateFrom:     fromTime,
		DateTo:       toTime,
		IsRaw:        isRaw,
		Rating:       ratingPtr,
		Liked:        liked,
		CameraModel:  cameraMake,
		LensModel:    lens,
		Limit:        int32(limit),
		Offset:       int32(offset),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to search assets with vector: %w", err)
	}

	// Extract assets from results (ignore distance for now)
	assets := make([]repo.Asset, len(results))
	for i, result := range results {
		assets[i] = repo.Asset{
			AssetID:          result.AssetID,
			OwnerID:          result.OwnerID,
			Type:             result.Type,
			OriginalFilename: result.OriginalFilename,
			StoragePath:      result.StoragePath,
			MimeType:         result.MimeType,
			FileSize:         result.FileSize,
			Hash:             result.Hash,
			Width:            result.Width,
			Height:           result.Height,
			Duration:         result.Duration,
			UploadTime:       result.UploadTime,
			IsDeleted:        result.IsDeleted,
			DeletedAt:        result.DeletedAt,
			SpecificMetadata: result.SpecificMetadata,
			Embedding:        result.Embedding,
		}
	}

	return assets, nil
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
	rows, err := s.queries.GetDistinctLenses(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get distinct lenses: %w", err)
	}

	lenses := make([]string, 0, len(rows))
	for _, row := range rows {
		if str, ok := row.(string); ok && str != "" {
			lenses = append(lenses, str)
		}
	}

	return lenses, nil
}
