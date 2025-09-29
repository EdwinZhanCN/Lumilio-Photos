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
	GetAssetWithOptions(ctx context.Context, id uuid.UUID, includeThumbnails, includeTags, includeAlbums, includeSpecies bool) (interface{}, error)
	GetAssetsByType(ctx context.Context, assetType string, limit, offset int) ([]repo.Asset, error)
	GetAssetsByOwner(ctx context.Context, ownerID int, limit, offset int) ([]repo.Asset, error)
	GetAssetsByOwnerSorted(ctx context.Context, ownerID int, sortOrder string, limit, offset int) ([]repo.Asset, error)
	GetAssetsByTypesSorted(ctx context.Context, assetTypes []string, sortOrder string, limit, offset int) ([]repo.Asset, error)
	GetAssetsByOwnerAndTypes(ctx context.Context, ownerID int, assetTypes []string, sortOrder string, limit, offset int) ([]repo.Asset, error)
	DeleteAsset(ctx context.Context, id uuid.UUID) error

	UpdateAssetMetadata(ctx context.Context, id uuid.UUID, metadata []byte) error

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

	// Video and Audio processing methods
	SaveVideoVersion(ctx context.Context, videoReader io.Reader, asset *repo.Asset, version string) error
	SaveAudioVersion(ctx context.Context, audioReader io.Reader, asset *repo.Asset, version string) error
	UpdateAssetDuration(ctx context.Context, id uuid.UUID, duration float64) error
	UpdateAssetDimensions(ctx context.Context, id uuid.UUID, width, height int32) error
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

func (s *assetService) GetAssetWithOptions(ctx context.Context, id uuid.UUID, includeThumbnails, includeTags, includeAlbums, includeSpecies bool) (interface{}, error) {
	pgUUID := pgtype.UUID{}
	if err := pgUUID.Scan(id.String()); err != nil {
		return nil, fmt.Errorf("invalid UUID: %w", err)
	}

	// 1) Full relations (thumbnails + tags + albums) OR species predictions requested
	if includeSpecies || (includeThumbnails && includeTags && includeAlbums) {
		dbAsset, err := s.queries.GetAssetWithRelations(ctx, pgUUID)
		if err != nil {
			return nil, fmt.Errorf("failed to get asset with relations: %w", err)
		}
		return dbAsset, nil
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
			"embedding":         asset.Embedding,
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

// UpdateAssetMetadata updates the specific metadata of an asset and extracts taken_time
func (s *assetService) UpdateAssetMetadata(ctx context.Context, id uuid.UUID, metadata []byte) error {
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
		if photoMeta, err := dbtypes.UnmarshalPhoto(metadata); err == nil {
			takenTime = photoMeta.TakenTime
		}
	case dbtypes.AssetTypeVideo:
		if videoMeta, err := dbtypes.UnmarshalVideo(metadata); err == nil {
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
	storagePath, err := s.storage.UploadWithMetadata(ctx, buffers, asset.OriginalFilename+"_"+size+".webp", "")
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

	// Fetch enough rows so we can paginate after threshold filtering
	requestLimit := int32(limit + offset)
	if requestLimit <= 0 {
		requestLimit = 1000
	}

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
		Limit:        requestLimit,
		Offset:       0,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to search assets with vector: %w", err)
	}

	// Default distance threshold (env override: SEMANTIC_MAX_DISTANCE)
	maxDistance := 1.235
	if v := os.Getenv("SEMANTIC_MAX_DISTANCE"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 {
			maxDistance = f
		}
	}

	// Filter by distance threshold, keep order (already ordered by distance ASC)
	filtered := make([]repo.Asset, 0, len(results))
	for _, result := range results {
		var dist float64
		switch d := result.Distance.(type) {
		case float32:
			dist = float64(d)
		case float64:
			dist = d
		case int32:
			dist = float64(d)
		case int64:
			dist = float64(d)
		case string:
			if parsed, perr := strconv.ParseFloat(d, 64); perr == nil {
				dist = parsed
			} else {
				continue
			}
		default:
			// Unknown type, skip this row
			continue
		}

		if dist <= maxDistance {
			filtered = append(filtered, repo.Asset{
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
			})
		}
	}

	// Apply pagination after threshold filtering
	if offset < 0 {
		offset = 0
	}
	if limit < 0 {
		limit = 0
	}
	if offset >= len(filtered) {
		return []repo.Asset{}, nil
	}
	end := offset + limit
	if end > len(filtered) {
		end = len(filtered)
	}

	return filtered[offset:end], nil
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

// Video and Audio processing methods implementation

func (s *assetService) SaveVideoVersion(ctx context.Context, videoReader io.Reader, asset *repo.Asset, version string) error {
	// Generate filename with version suffix
	filename := asset.OriginalFilename
	if version != "original" {
		// Remove original extension and add version
		ext := filepath.Ext(filename)
		nameWithoutExt := strings.TrimSuffix(filename, ext)
		filename = fmt.Sprintf("%s_%s.mp4", nameWithoutExt, version)
	}

	// Upload to storage
	hash := ""
	if asset.Hash != nil {
		hash = *asset.Hash
	}
	storagePath, err := s.storage.UploadWithMetadata(ctx, videoReader, filename, hash)
	if err != nil {
		return fmt.Errorf("failed to upload video version %s: %w", version, err)
	}

	// TODO: Store video version metadata in database if needed
	// For now, we're storing versions in storage with different filenames
	log.Printf("Saved video version %s for asset %s at path %s", version, asset.AssetID.Bytes, storagePath)
	return nil
}

func (s *assetService) SaveAudioVersion(ctx context.Context, audioReader io.Reader, asset *repo.Asset, version string) error {
	// Generate filename with version suffix
	filename := asset.OriginalFilename
	if version != "original" {
		// Remove original extension and add version
		ext := filepath.Ext(filename)
		nameWithoutExt := strings.TrimSuffix(filename, ext)
		filename = fmt.Sprintf("%s_%s.mp3", nameWithoutExt, version)
	}

	// Upload to storage
	hash := ""
	if asset.Hash != nil {
		hash = *asset.Hash
	}
	storagePath, err := s.storage.UploadWithMetadata(ctx, audioReader, filename, hash)
	if err != nil {
		return fmt.Errorf("failed to upload audio version %s: %w", version, err)
	}

	// TODO: Store audio version metadata in database if needed
	log.Printf("Saved audio version %s for asset %s at path %s", version, asset.AssetID.Bytes, storagePath)
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
