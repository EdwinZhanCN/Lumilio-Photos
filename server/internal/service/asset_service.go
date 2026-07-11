package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"path/filepath"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	aggregatesearch "server/internal/search"
	"server/internal/utils/geohash"
	"strings"
	"time"

	"github.com/edwinzhancn/lumen-sdk/pkg/types"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pgvector/pgvector-go"
	"go.uber.org/zap"
)

// Asset type constants
const (
	AssetTypePhoto     = "PHOTO"
	AssetTypeVideo     = "VIDEO"
	AssetTypeAudio     = "AUDIO"
	StackModeCollapsed = "collapsed"
	StackModeExpanded  = "expanded"
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
	GetAssetAny(ctx context.Context, id uuid.UUID) (*repo.Asset, error)
	GetAssetRelations(ctx context.Context, id uuid.UUID) (repo.GetAssetWithRelationsRow, error)
	GetAssetExifRaw(ctx context.Context, id uuid.UUID) (json.RawMessage, error)
	GetAssetsByType(ctx context.Context, assetType string, limit, offset int) ([]repo.Asset, error)
	GetAssetsByOwner(ctx context.Context, ownerID int, limit, offset int) ([]repo.Asset, error)
	GetAssetsByOwnerSorted(ctx context.Context, ownerID int, sortOrder string, limit, offset int) ([]repo.Asset, error)
	GetAssetsByTypesSorted(ctx context.Context, assetTypes []string, sortOrder string, limit, offset int) ([]repo.Asset, error)
	GetAssetsByOwnerAndTypes(ctx context.Context, ownerID int, assetTypes []string, sortOrder string, limit, offset int) ([]repo.Asset, error)
	DeleteAsset(ctx context.Context, id uuid.UUID) error
	RestoreAsset(ctx context.Context, id uuid.UUID) error

	UpdateAssetMetadata(ctx context.Context, id uuid.UUID, metadata dbtypes.SpecificMetadata) error
	UpdateAssetMetadataWithExifRaw(ctx context.Context, id uuid.UUID, metadata dbtypes.SpecificMetadata, exifRaw json.RawMessage) error

	// Rating management methods
	UpdateAssetRating(ctx context.Context, id uuid.UUID, rating int) error
	UpdateAssetLike(ctx context.Context, id uuid.UUID, liked bool) error
	UpdateAssetRatingAndLike(ctx context.Context, id uuid.UUID, rating int, liked bool) error
	UpdateAssetDescription(ctx context.Context, id uuid.UUID, description string) error
	GetAssetsByRating(ctx context.Context, rating int, ownerID *int32, limit, offset int) ([]repo.Asset, error)
	GetLikedAssets(ctx context.Context, ownerID *int32, limit, offset int) ([]repo.Asset, error)

	AddAssetToAlbum(ctx context.Context, assetID uuid.UUID, albumID int) error
	RemoveAssetFromAlbum(ctx context.Context, assetID uuid.UUID, albumID int) error

	AddTagToAsset(ctx context.Context, assetID uuid.UUID, tagID int, confidence float32, source string) error
	RemoveTagFromAsset(ctx context.Context, assetID uuid.UUID, tagID int) error
	// AddManualTagToAsset resolves (creating if needed) a tag by name and links
	// it to the asset with the "manual" source. Returns the resolved tag.
	AddManualTagToAsset(ctx context.Context, assetID uuid.UUID, tagName string) (*repo.Tag, error)
	// GetAssetTags returns all tags linked to an asset (any source) as the raw
	// JSON aggregate (tag_id, tag_name, category, confidence, source).
	GetAssetTags(ctx context.Context, assetID uuid.UUID) (json.RawMessage, error)
	// SearchTags returns tag definitions for autocomplete; empty query lists all.
	SearchTags(ctx context.Context, query string, limit int) ([]repo.Tag, error)

	CreateThumbnail(ctx context.Context, assetID pgtype.UUID, size string, thumbnailPath string) (*repo.Thumbnail, error)
	DetectDuplicates(ctx context.Context, hash string) ([]repo.Asset, error)
	SaveAssetIndex(ctx context.Context, taskID string, hash string) error
	CreateAssetRecord(ctx context.Context, params repo.CreateAssetParams) (*repo.Asset, error)

	GetOrCreateTagByName(ctx context.Context, name, category string, isAIGenerated bool) (*repo.Tag, error)
	GetThumbnailByID(ctx context.Context, thumbnailID int) (*repo.Thumbnail, error)
	GetThumbnailByAssetIDAndSize(ctx context.Context, assetID uuid.UUID, size string) (*repo.Thumbnail, error)

	SaveNewAsset(ctx context.Context, fileReader io.Reader, filename string, hash string) (string, error)
	SaveNewThumbnail(ctx context.Context, repoPath string, buffers io.Reader, asset *repo.Asset, size string) error
	GetDistinctCameraModels(ctx context.Context) ([]string, error)
	GetDistinctLenses(ctx context.Context) ([]string, error)

	// Video and Audio processing methods
	SaveVideoVersion(ctx context.Context, repoPath string, videoReader io.Reader, asset *repo.Asset, version string) error
	SaveAudioVersion(ctx context.Context, repoPath string, audioReader io.Reader, asset *repo.Asset, version string) error
	UpdateAssetDuration(ctx context.Context, id uuid.UUID, duration float64) error
	UpdateAssetDimensions(ctx context.Context, id uuid.UUID, width, height int32) error

	// Unified query API
	QueryAssets(ctx context.Context, params QueryAssetsParams) ([]repo.Asset, int64, error)
	QueryBrowseItems(ctx context.Context, params QueryAssetsParams) (BrowseQueryResult, error)
	SearchAssets(ctx context.Context, params SearchAssetsParams) (SearchAssetsResult, error)
	SearchBrowseItems(ctx context.Context, params SearchAssetsParams) (SearchBrowseResult, error)
	QueryPhotoMapPoints(ctx context.Context, params QueryPhotoMapPointsParams) ([]PhotoMapPoint, int64, error)

	// Single-retriever set search (agent producer path and the search Results
	// tier). The semantic channel applies a per-query calibrated relevance
	// cutoff instead of fixed TopK; the OCR channel is naturally thresholded
	// by tsquery matching. Rankings are the retrievers' own orders.
	SearchAssetIDsSemantic(ctx context.Context, query string, strictness aggregatesearch.SetStrictness, maxResults int) ([]uuid.UUID, aggregatesearch.SetMeta, error)
	SearchAssetIDsOCR(ctx context.Context, query string, maxResults int) ([]uuid.UUID, error)

	// Folders and tags are derived/vocabulary collection views (no folder
	// entity exists; "folders" come from assets.storage_path prefixes).
	ListFolderSummaries(ctx context.Context, ownerID *int32, repositoryID *string, parentPath string) ([]FolderSummary, error)
	GetFolderSummary(ctx context.Context, ownerID *int32, repositoryID string, folderPath string) (FolderSummary, error)
	ListTagSummaries(ctx context.Context, ownerID *int32, repositoryID *string, source *string, query *string, limit, offset int) ([]TagSummary, error)
}

// QueryAssetsParams contains all parameters for the unified asset query
type QueryAssetsParams struct {
	Query            string // Filename search query (empty for list-only)
	SearchType       string // "filename" (default) | "semantic"
	ViewerTimeZone   string
	RepositoryID     *string
	PersonID         *int32
	AssetType        *string  // Single type filter
	AssetTypes       []string // Multiple types filter
	OwnerID          *int32
	AlbumID          *int32
	FilenameValue    *string
	FilenameOperator *string
	DateFrom         *time.Time
	DateTo           *time.Time
	IsRaw            *bool
	IsDeleted        *bool
	Rating           *int
	Liked            *bool
	CameraModel      *string
	LensModel        *string
	TagName          *string
	TagSource        *string
	TagNames         []string
	FolderPath       *string
	FolderRecursive  *bool
	LocationNorth    *float64
	LocationSouth    *float64
	LocationEast     *float64
	LocationWest     *float64
	SortBy           string
	StackMode        string
	Source           *AssetSetSource
	Limit            int
	Offset           int
}

type AssetSetSourceKind string

const (
	AssetSetSourceLibrary   AssetSetSourceKind = "library"
	AssetSetSourcePin       AssetSetSourceKind = "pin"
	AssetSetSourceRef       AssetSetSourceKind = "ref"
	AssetSetSourceShareLink AssetSetSourceKind = "share_link"
)

// AssetSetSource scopes a query to an internally resolved asset set.
// Handlers construct this after source-specific authorization; it is not a
// public asset filter DTO field.
type AssetSetSource struct {
	Kind                  AssetSetSourceKind
	AssetIDs              []uuid.UUID
	PreserveSnapshotOrder bool
}

type SearchEnhancementMode string

const (
	SearchEnhancementModeAuto SearchEnhancementMode = "auto"
	SearchEnhancementModeOff  SearchEnhancementMode = "off"
	SearchEnhancementModeOnly SearchEnhancementMode = "only"
)

type SearchAssetsParams struct {
	QueryAssetsParams
	EnhancementMode SearchEnhancementMode
	TopResultsLimit int
	Debug           bool
}

type SearchTopResultsMeta struct {
	Enabled           bool
	Degraded          bool
	Reason            string
	SourceTypes       []string
	CandidateCount    int
	CandidatePoolSize int
	Sources           []SearchSourceMeta
	Debug             []SearchDebugItem
}

type SearchSourceMeta struct {
	Type           string
	Weight         float64
	CandidateCount int
	DurationMs     int64
	Error          string
}

type SearchDebugContribution struct {
	Rank     int
	Weight   float64
	RRFScore float64
	RawScore float64
}

type SearchDebugItem struct {
	AssetID       string
	Score         float64
	Contributions map[string]SearchDebugContribution
}

type SearchAssetsResult struct {
	TopResults     []repo.Asset
	TopResultsMeta SearchTopResultsMeta
	Results        []repo.Asset
	ResultsTotal   int64
}

type QueryPhotoMapPointsParams struct {
	RepositoryID *string
	OwnerID      *int32
	South        *float64
	North        *float64
	West         *float64
	East         *float64
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
	queries                *repo.Queries
	pool                   *pgxpool.Pool
	lumen                  LumenService
	embeddingService       EmbeddingService
	aggregateSearch        aggregatesearch.Service
	semanticRetriever      *aggregatesearch.EmbeddingRetriever
	ocrRetriever           *aggregatesearch.TextRetriever
	placeRetriever         *aggregatesearch.TextRetriever
	queryAssetsUnifiedFn   func(ctx context.Context, params QueryAssetsParams) ([]repo.Asset, int64, error)
	searchAssetsFusedSetFn func(ctx context.Context, params SearchAssetsParams) (fusedSearchSet, bool)
	hydrateAssetsInOrderFn func(ctx context.Context, ids []uuid.UUID, isDeleted *bool) ([]repo.Asset, error)
	pageAssetsBySortFn     func(ctx context.Context, ids []uuid.UUID, sortBy string, limit, offset int, isDeleted *bool) ([]repo.Asset, error)
}

func NewAssetService(q *repo.Queries, pool *pgxpool.Pool, l LumenService, e EmbeddingService, loggers ...*zap.Logger) (AssetService, error) {
	logger := zap.NewNop()
	if len(loggers) > 0 && loggers[0] != nil {
		logger = loggers[0]
	}
	svc := &assetService{
		queries:          q,
		pool:             pool,
		lumen:            l,
		embeddingService: e,
	}
	svc.semanticRetriever = aggregatesearch.NewEmbeddingRetriever(
		pool,
		func(ctx context.Context, query string, fast bool) (aggregatesearch.QueryEmbedding, error) {
			embedding, err := svc.resolveSemanticQueryEmbedding(ctx, query, fast)
			if err != nil {
				return aggregatesearch.QueryEmbedding{}, err
			}
			return aggregatesearch.QueryEmbedding{
				Model:  embedding.ModelID,
				Vector: embedding.Vector,
			}, nil
		},
		func(ctx context.Context, model string, dimensions int) (repo.EmbeddingSpace, error) {
			if svc.embeddingService == nil {
				return repo.EmbeddingSpace{}, fmt.Errorf("%w: embedding service not available", ErrSemanticSearchUnavailable)
			}
			return svc.embeddingService.ResolveDefaultSearchSpace(ctx, EmbeddingTypeSemantic, model, dimensions)
		},
		1.0,
	)
	svc.ocrRetriever = aggregatesearch.NewOCRRetriever(pool, 0.7)
	svc.placeRetriever = aggregatesearch.NewPlaceRetriever(pool, 0.8)
	svc.aggregateSearch = aggregatesearch.NewAggregateService(pool, []aggregatesearch.Retriever{
		svc.semanticRetriever,
		svc.ocrRetriever,
		svc.placeRetriever,
	}, logger.Named("aggregate_search"))
	return svc, nil
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

// GetAssetAny retrieves an asset by ID regardless of Trash state.
func (s *assetService) GetAssetAny(ctx context.Context, id uuid.UUID) (*repo.Asset, error) {
	pgUUID := pgtype.UUID{}
	if err := pgUUID.Scan(id.String()); err != nil {
		return nil, fmt.Errorf("invalid UUID: %w", err)
	}

	dbAsset, err := s.queries.GetAssetByIDAny(ctx, pgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get asset: %w", err)
	}

	return &dbAsset, nil
}

func (s *assetService) GetAssetExifRaw(ctx context.Context, id uuid.UUID) (json.RawMessage, error) {
	pgUUID := pgtype.UUID{}
	if err := pgUUID.Scan(id.String()); err != nil {
		return nil, fmt.Errorf("invalid UUID: %w", err)
	}

	exifRaw, err := s.queries.GetAssetExifRaw(ctx, pgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get asset exif: %w", err)
	}

	return exifRaw, nil
}

// GetAssetRelations returns a single asset together with its aggregated
// relations (thumbnails, tags, albums, species predictions, OCR, and face
// results) in one query. The handler projects this into a typed
// dto.AssetDetailDTO, honoring the include_* query flags. Trash state is not
// filtered here; handler auth decides access.
func (s *assetService) GetAssetRelations(ctx context.Context, id uuid.UUID) (repo.GetAssetWithRelationsRow, error) {
	pgUUID := pgtype.UUID{}
	if err := pgUUID.Scan(id.String()); err != nil {
		return repo.GetAssetWithRelationsRow{}, fmt.Errorf("invalid UUID: %w", err)
	}

	row, err := s.queries.GetAssetWithRelations(ctx, pgUUID)
	if err != nil {
		return repo.GetAssetWithRelationsRow{}, fmt.Errorf("failed to get asset with relations: %w", err)
	}
	return row, nil
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
	return s.UpdateAssetMetadataWithExifRaw(ctx, id, metadata, nil)
}

func (s *assetService) UpdateAssetMetadataWithExifRaw(ctx context.Context, id uuid.UUID, metadata dbtypes.SpecificMetadata, exifRaw json.RawMessage) error {
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
	var captureOffsetMinutes *int16
	var gpsLatitude *float64
	var gpsLongitude *float64
	var gpsGeohash5 *string
	var gpsGeohash7 *string
	assetType := dbtypes.AssetType(asset.Type)

	switch assetType {
	case dbtypes.AssetTypePhoto:
		if photoMeta, err := metadata.UnmarshalPhoto(); err == nil {
			takenTime = photoMeta.TakenTime
			captureOffsetMinutes = photoMeta.CaptureOffsetMinutes
			gpsLatitude, gpsLongitude = normalizedGPS(photoMeta.GPSLatitude, photoMeta.GPSLongitude)
		}
	case dbtypes.AssetTypeVideo:
		if videoMeta, err := metadata.UnmarshalVideo(); err == nil {
			takenTime = videoMeta.RecordedTime
			captureOffsetMinutes = videoMeta.CaptureOffsetMinutes
			gpsLatitude, gpsLongitude = normalizedGPS(videoMeta.GPSLatitude, videoMeta.GPSLongitude)
		}
	case dbtypes.AssetTypeAudio:
		// Audio doesn't have taken time
		takenTime = nil
	}
	gpsGeohash5, gpsGeohash7 = geohashesForGPS(gpsLatitude, gpsLongitude)

	// Use the new query that updates both metadata and taken_time
	var takenTimeParam pgtype.Timestamptz
	if takenTime != nil {
		takenTimeParam = pgtype.Timestamptz{
			Time:  *takenTime,
			Valid: true,
		}
	}

	params := repo.UpdateAssetMetadataWithTakenTimeParams{
		AssetID:              pgUUID,
		SpecificMetadata:     metadata,
		ExifRaw:              []byte(exifRaw),
		TakenTime:            takenTimeParam,
		CaptureOffsetMinutes: captureOffsetMinutes,
		GpsLatitude:          gpsLatitude,
		GpsLongitude:         gpsLongitude,
		GpsGeohash5:          gpsGeohash5,
		GpsGeohash7:          gpsGeohash7,
	}

	return s.queries.UpdateAssetMetadataWithTakenTime(ctx, params)
}

func normalizedGPS(latitude, longitude *float64) (*float64, *float64) {
	if latitude == nil || longitude == nil {
		return nil, nil
	}
	lat := *latitude
	lng := *longitude
	if math.IsNaN(lat) || math.IsInf(lat, 0) || math.IsNaN(lng) || math.IsInf(lng, 0) {
		return nil, nil
	}
	if lat < -90 || lat > 90 || lng < -180 || lng > 180 {
		return nil, nil
	}
	return &lat, &lng
}

func geohashesForGPS(latitude, longitude *float64) (*string, *string) {
	if latitude == nil || longitude == nil {
		return nil, nil
	}
	hash5, ok5 := geohash.Encode(*latitude, *longitude, 5)
	hash7, ok7 := geohash.Encode(*latitude, *longitude, 7)
	if !ok5 || !ok7 {
		return nil, nil
	}
	return &hash5, &hash7
}

// DeleteAsset moves an asset into the app Trash via a database soft-delete.
func (s *assetService) DeleteAsset(ctx context.Context, id uuid.UUID) error {
	pgUUID := pgtype.UUID{}
	if err := pgUUID.Scan(id.String()); err != nil {
		return fmt.Errorf("invalid UUID: %w", err)
	}

	return s.queries.DeleteAsset(ctx, pgUUID)
}

// RestoreAsset restores an asset from the app Trash.
func (s *assetService) RestoreAsset(ctx context.Context, id uuid.UUID) error {
	pgUUID := pgtype.UUID{}
	if err := pgUUID.Scan(id.String()); err != nil {
		return fmt.Errorf("invalid UUID: %w", err)
	}

	return s.queries.RestoreAsset(ctx, pgUUID)
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

// AddManualTagToAsset resolves a tag by name (creating it if absent) and links
// it to the asset with the manual source and full confidence.
func (s *assetService) AddManualTagToAsset(ctx context.Context, assetID uuid.UUID, tagName string) (*repo.Tag, error) {
	name := strings.TrimSpace(tagName)
	if name == "" {
		return nil, fmt.Errorf("tag name must not be empty")
	}

	tag, err := s.GetOrCreateTagByName(ctx, name, "", false)
	if err != nil {
		return nil, err
	}

	if err := s.AddTagToAsset(ctx, assetID, int(tag.TagID), 1.0, AssetTagSourceUser); err != nil {
		return nil, err
	}

	return tag, nil
}

// GetAssetTags returns the raw JSON tag aggregate for an asset.
func (s *assetService) GetAssetTags(ctx context.Context, assetID uuid.UUID) (json.RawMessage, error) {
	pgUUID := pgtype.UUID{}
	if err := pgUUID.Scan(assetID.String()); err != nil {
		return nil, fmt.Errorf("invalid UUID: %w", err)
	}

	row, err := s.queries.GetAssetWithTags(ctx, pgUUID)
	if err != nil {
		return nil, err
	}

	// pgx decodes the json aggregate column into interface{}; normalize to raw
	// JSON bytes for the caller to unmarshal.
	switch v := row.Tags.(type) {
	case nil:
		return json.RawMessage("[]"), nil
	case []byte:
		return json.RawMessage(v), nil
	case string:
		return json.RawMessage(v), nil
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("marshal tags: %w", err)
		}
		return json.RawMessage(b), nil
	}
}

// SearchTags returns tag definitions matching query (empty lists all), capped at limit.
func (s *assetService) SearchTags(ctx context.Context, query string, limit int) ([]repo.Tag, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	var q *string
	if trimmed := strings.TrimSpace(query); trimmed != "" {
		q = &trimmed
	}

	return s.queries.SearchTagsByName(ctx, repo.SearchTagsByNameParams{
		Limit: int32(limit),
		Query: q,
	})
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

// CreateThumbnail creates or updates a thumbnail record for an asset
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

func (s *assetService) GetDistinctCameraModels(ctx context.Context) ([]string, error) {
	rows, err := s.queries.GetDistinctCameraModels(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get distinct camera models: %w", err)
	}

	models := make([]string, 0, len(rows))
	for _, row := range rows {
		if str, ok := row.(string); ok && str != "" {
			models = append(models, str)
		}
	}

	return models, nil
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

func (s *assetService) GetAssetsByRating(ctx context.Context, rating int, ownerID *int32, limit, offset int) ([]repo.Asset, error) {
	params := repo.GetAssetsByRatingParams{
		Rating:  int32(rating),
		OwnerID: ownerID,
		Limit:   int32(limit),
		Offset:  int32(offset),
	}

	return s.queries.GetAssetsByRating(ctx, params)
}

func (s *assetService) GetLikedAssets(ctx context.Context, ownerID *int32, limit, offset int) ([]repo.Asset, error) {
	params := repo.GetLikedAssetsParams{
		OwnerID: ownerID,
		Limit:   int32(limit),
		Offset:  int32(offset),
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

func normalizeSearchEnhancementMode(raw SearchEnhancementMode) SearchEnhancementMode {
	switch strings.ToLower(strings.TrimSpace(string(raw))) {
	case string(SearchEnhancementModeOff):
		return SearchEnhancementModeOff
	case string(SearchEnhancementModeOnly):
		return SearchEnhancementModeOnly
	default:
		return SearchEnhancementModeAuto
	}
}

func normalizeSearchAssetsParams(params SearchAssetsParams) SearchAssetsParams {
	params.Query = strings.TrimSpace(params.Query)
	params.EnhancementMode = normalizeSearchEnhancementMode(params.EnhancementMode)
	if params.TopResultsLimit <= 0 || params.TopResultsLimit > 200 {
		params.TopResultsLimit = 200
	}
	return params
}

func cloneStringSlice(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	cloned := make([]string, len(values))
	copy(cloned, values)
	return cloned
}

func cloneUUIDSlice(values []uuid.UUID) []uuid.UUID {
	if len(values) == 0 {
		return []uuid.UUID{}
	}
	cloned := make([]uuid.UUID, len(values))
	copy(cloned, values)
	return cloned
}

func assetSetSourceUUIDs(source *AssetSetSource) []uuid.UUID {
	if source == nil {
		return nil
	}
	return cloneUUIDSlice(source.AssetIDs)
}

func assetSetSourcePgUUIDs(source *AssetSetSource) []pgtype.UUID {
	if source == nil {
		return nil
	}
	ids := make([]pgtype.UUID, 0, len(source.AssetIDs))
	for _, id := range source.AssetIDs {
		if id == uuid.Nil {
			continue
		}
		ids = append(ids, pgtype.UUID{Bytes: id, Valid: true})
	}
	return ids
}

func (s *assetService) runQueryAssetsUnified(ctx context.Context, params QueryAssetsParams) ([]repo.Asset, int64, error) {
	if s.queryAssetsUnifiedFn != nil {
		return s.queryAssetsUnifiedFn(ctx, params)
	}
	return s.queryAssetsUnified(ctx, params)
}

// SearchAssets runs the unified search pipeline (see asset_search_fused.go):
// all channels fuse into one confidence-ordered set. Results is that whole
// set under the presentation sort; Best Results (TopResults) is its
// confidence-ordered Top-N subset — a pure subset, no dedup. When no channel
// can run at all the legacy filename path is the fallback.
func (s *assetService) SearchAssets(ctx context.Context, params SearchAssetsParams) (SearchAssetsResult, error) {
	params = normalizeSearchAssetsParams(params)

	result := SearchAssetsResult{
		TopResults:     []repo.Asset{},
		TopResultsMeta: SearchTopResultsMeta{Enabled: false, SourceTypes: []string{}},
		Results:        []repo.Asset{},
	}

	query := strings.TrimSpace(params.Query)
	enhanced := query != "" && params.EnhancementMode != SearchEnhancementModeOff

	if enhanced {
		if fused, ok := s.runSearchAssetsFusedSet(ctx, params); ok {
			result.TopResultsMeta = fused.meta()
			ids := fused.ids()

			// Best Results exists only when the set is larger than the
			// showcase size; otherwise everything lives in Results.
			if len(ids) >= params.TopResultsLimit {
				topResults, err := s.runHydrateAssetsInOrder(ctx, ids[:params.TopResultsLimit], params.IsDeleted)
				if err != nil {
					return SearchAssetsResult{}, err
				}
				result.TopResults = topResults
			}

			if params.EnhancementMode != SearchEnhancementModeOnly {
				page, err := s.runPageAssetsBySort(ctx, ids, params.SortBy, params.Limit, params.Offset, params.IsDeleted)
				if err != nil {
					return SearchAssetsResult{}, err
				}
				result.Results = page
				result.ResultsTotal = int64(len(ids))
			}
			return result, nil
		}

		if params.EnhancementMode == SearchEnhancementModeOnly {
			return SearchAssetsResult{}, fmt.Errorf("aggregate search failed")
		}
		// No channel could run: degrade to filename, flag semantic missing.
		result.TopResultsMeta = SearchTopResultsMeta{
			Enabled:     true,
			Degraded:    true,
			Reason:      semanticUnavailableReason,
			SourceTypes: []string{},
		}
	}

	if params.EnhancementMode != SearchEnhancementModeOnly {
		filenameParams := params.QueryAssetsParams
		filenameParams.Query = query
		filenameParams.SearchType = "filename"

		filenameResults, total, err := s.runQueryAssetsUnified(ctx, filenameParams)
		if err != nil {
			return SearchAssetsResult{}, err
		}
		result.Results = filenameResults
		result.ResultsTotal = total
	}

	if !enhanced {
		switch {
		case params.EnhancementMode == SearchEnhancementModeOff:
			result.TopResultsMeta = SearchTopResultsMeta{Enabled: false, Reason: "disabled", SourceTypes: []string{}}
		case query == "":
			result.TopResultsMeta = SearchTopResultsMeta{Enabled: false, Reason: "empty_query", SourceTypes: []string{}}
		}
	}

	return result, nil
}

// QueryAssets is the unified method for listing, filtering, and searching assets.
func (s *assetService) QueryAssets(ctx context.Context, params QueryAssetsParams) ([]repo.Asset, int64, error) {
	if params.SearchType == "semantic" && params.Query != "" {
		return s.queryAssetsAggregate(ctx, params)
	}
	return s.queryAssetsUnified(ctx, params)
}

func (s *assetService) queryAssetsAggregate(ctx context.Context, params QueryAssetsParams) ([]repo.Asset, int64, error) {
	if s.aggregateSearch == nil {
		return nil, 0, fmt.Errorf("%w: aggregate search service not available", ErrSemanticSearchUnavailable)
	}
	filter, err := buildAggregateSearchFilter(params)
	if err != nil {
		return nil, 0, err
	}
	response, err := s.aggregateSearch.Search(ctx, aggregatesearch.Request{
		Query:      params.Query,
		Filter:     filter,
		Limit:      params.Limit,
		Offset:     params.Offset,
		CountTotal: true,
		Debug:      false,
	})
	if err != nil {
		return nil, 0, err
	}
	return response.Assets, int64(response.TotalCandidates), nil
}

func buildAggregateSearchFilter(params QueryAssetsParams) (aggregatesearch.Filter, error) {
	var repositoryID *uuid.UUID
	if params.RepositoryID != nil && *params.RepositoryID != "" {
		parsed, err := uuid.Parse(*params.RepositoryID)
		if err != nil {
			return aggregatesearch.Filter{}, fmt.Errorf("invalid repository ID: %w", err)
		}
		repositoryID = &parsed
	}
	return aggregatesearch.Filter{
		AssetIDs:         assetSetSourceUUIDs(params.Source),
		RepositoryID:     repositoryID,
		PersonID:         params.PersonID,
		AssetType:        params.AssetType,
		AssetTypes:       cloneStringSlice(params.AssetTypes),
		OwnerID:          params.OwnerID,
		AlbumID:          params.AlbumID,
		FilenameValue:    params.FilenameValue,
		FilenameOperator: params.FilenameOperator,
		DateFrom:         params.DateFrom,
		DateTo:           params.DateTo,
		IsRaw:            params.IsRaw,
		IsDeleted:        params.IsDeleted,
		Rating:           params.Rating,
		Liked:            params.Liked,
		CameraModel:      params.CameraModel,
		LensModel:        params.LensModel,
		TagName:          params.TagName,
		TagSource:        params.TagSource,
		TagNames:         params.TagNames,
		LocationNorth:    params.LocationNorth,
		LocationSouth:    params.LocationSouth,
		LocationEast:     params.LocationEast,
		LocationWest:     params.LocationWest,
	}, nil
}

func aggregateCandidatePoolSize(limit, offset int) int {
	topK := (limit + offset) * aggregatesearch.DefaultCandidateMultiplier
	if topK < aggregatesearch.DefaultCandidatePoolMin {
		return aggregatesearch.DefaultCandidatePoolMin
	}
	if topK > aggregatesearch.DefaultCandidatePoolMax {
		return aggregatesearch.DefaultCandidatePoolMax
	}
	return topK
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

	var sortByPtr *string
	switch params.SortBy {
	case "recently_added":
		s := "recently_added"
		sortByPtr = &s
	case "date_captured":
		s := "date_captured"
		sortByPtr = &s
	}
	sourceAssetIDs := assetSetSourcePgUUIDs(params.Source)

	// Get total count
	countResult, err := s.queries.CountAssetsUnified(ctx, repo.CountAssetsUnifiedParams{
		AssetIds:         sourceAssetIDs,
		AssetType:        params.AssetType,
		AssetTypes:       params.AssetTypes,
		RepositoryID:     repoUUID,
		PersonID:         params.PersonID,
		OwnerID:          params.OwnerID,
		AlbumID:          params.AlbumID,
		Query:            queryPtr,
		FilenameVal:      params.FilenameValue,
		FilenameOperator: params.FilenameOperator,
		IsRaw:            params.IsRaw,
		Rating:           ratingPtr,
		Liked:            params.Liked,
		CameraModel:      params.CameraModel,
		LensModel:        params.LensModel,
		TagName:          params.TagName,
		TagSource:        params.TagSource,
		TagNames:         params.TagNames,
		FolderPath:       params.FolderPath,
		FolderRecursive:  params.FolderRecursive,
		LocationNorth:    params.LocationNorth,
		LocationSouth:    params.LocationSouth,
		LocationEast:     params.LocationEast,
		LocationWest:     params.LocationWest,
		DateFrom:         fromTime,
		DateTo:           toTime,
		IsDeleted:        params.IsDeleted,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count assets: %w", err)
	}

	// Get assets
	assets, err := s.queries.GetAssetsUnified(ctx, repo.GetAssetsUnifiedParams{
		AssetIds:         sourceAssetIDs,
		AssetType:        params.AssetType,
		AssetTypes:       params.AssetTypes,
		RepositoryID:     repoUUID,
		PersonID:         params.PersonID,
		OwnerID:          params.OwnerID,
		AlbumID:          params.AlbumID,
		Query:            queryPtr,
		FilenameVal:      params.FilenameValue,
		FilenameOperator: params.FilenameOperator,
		IsRaw:            params.IsRaw,
		Rating:           ratingPtr,
		Liked:            params.Liked,
		CameraModel:      params.CameraModel,
		LensModel:        params.LensModel,
		TagName:          params.TagName,
		TagSource:        params.TagSource,
		TagNames:         params.TagNames,
		FolderPath:       params.FolderPath,
		FolderRecursive:  params.FolderRecursive,
		LocationNorth:    params.LocationNorth,
		LocationSouth:    params.LocationSouth,
		LocationEast:     params.LocationEast,
		LocationWest:     params.LocationWest,
		SortBy:           sortByPtr,
		DateFrom:         fromTime,
		DateTo:           toTime,
		IsDeleted:        params.IsDeleted,
		Limit:            int32(params.Limit),
		Offset:           int32(params.Offset),
	})
	if err != nil {
		return nil, 0, err
	}

	return assets, countResult, nil
}

func (s *assetService) queryAssetsVector(ctx context.Context, params QueryAssetsParams) ([]repo.Asset, int64, error) {
	embeddingResult, err := s.resolveSemanticQueryEmbedding(ctx, params.Query, false)
	if err != nil {
		return nil, 0, err
	}

	return s.searchAssetsInResolvedSpace(ctx, params, embeddingResult.ModelID, embeddingResult.Vector, params.Limit, params.Offset, true)
}

func (s *assetService) resolveSemanticQueryEmbedding(ctx context.Context, query string, fast bool) (*types.EmbeddingV1, error) {
	if s.lumen == nil {
		return nil, fmt.Errorf("%w: lumen service not available", ErrSemanticSearchUnavailable)
	}
	if s.embeddingService == nil {
		return nil, fmt.Errorf("%w: embedding service not available", ErrSemanticSearchUnavailable)
	}

	var (
		embeddingResult *types.EmbeddingV1
		err             error
	)
	if fast {
		embeddingResult, err = s.lumen.SemanticTextEmbedFast(ctx, []byte(query))
	} else {
		embeddingResult, err = s.lumen.SemanticTextEmbed(ctx, []byte(query))
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get query embedding: %w", err)
	}
	if embeddingResult == nil || len(embeddingResult.Vector) == 0 {
		return nil, fmt.Errorf("%w: semantic_text_embed returned empty embedding", ErrSemanticSearchUnavailable)
	}
	return embeddingResult, nil
}

func (s *assetService) searchAssetsInResolvedSpace(ctx context.Context, params QueryAssetsParams, model string, vector []float32, limit, offset int, includeCount bool) ([]repo.Asset, int64, error) {
	space, err := s.embeddingService.ResolveDefaultSearchSpace(ctx, EmbeddingTypeSemantic, model, len(vector))
	if err != nil {
		return nil, 0, err
	}

	queryVector := pgvector.NewVector(vector)
	assets, err := s.searchAssetsBySemanticSpace(ctx, params, space, &queryVector, limit, offset)
	if err != nil {
		return nil, 0, err
	}

	if !includeCount {
		return assets, 0, nil
	}

	total, err := s.countAssetsBySemanticSpace(ctx, params, space, &queryVector)
	if err != nil {
		return nil, 0, err
	}

	return assets, total, nil
}

type semanticSQLBuilder struct {
	args []any
}

func (b *semanticSQLBuilder) addArg(value any) string {
	b.args = append(b.args, value)
	return fmt.Sprintf("$%d", len(b.args))
}

func (s *assetService) searchAssetsBySemanticSpace(ctx context.Context, params QueryAssetsParams, space repo.EmbeddingSpace, vector *pgvector.Vector, limit, offset int) ([]repo.Asset, error) {
	builder := &semanticSQLBuilder{}
	baseSQL, distanceExpr, err := s.buildSemanticSearchBaseSQL(builder, params, space, vector)
	if err != nil {
		return nil, err
	}

	limitPlaceholder := builder.addArg(limit)
	offsetPlaceholder := builder.addArg(offset)
	query := fmt.Sprintf(`
WITH candidate_ids AS MATERIALIZED (
  SELECT
    a.asset_id,
    %s AS distance
  %s
  ORDER BY %s, a.asset_id DESC
  LIMIT %s OFFSET %s
)
SELECT a.*
FROM candidate_ids c
JOIN assets a ON a.asset_id = c.asset_id
ORDER BY c.distance, c.asset_id DESC
`, distanceExpr, baseSQL, distanceExpr, limitPlaceholder, offsetPlaceholder)

	rows, err := s.pool.Query(ctx, query, builder.args...)
	if err != nil {
		return nil, fmt.Errorf("failed to search assets: %w", err)
	}
	defer rows.Close()

	assets, err := pgx.CollectRows(rows, pgx.RowToStructByName[repo.Asset])
	if err != nil {
		return nil, fmt.Errorf("failed to decode semantic search rows: %w", err)
	}

	return assets, nil
}

func (s *assetService) countAssetsBySemanticSpace(ctx context.Context, params QueryAssetsParams, space repo.EmbeddingSpace, vector *pgvector.Vector) (int64, error) {
	builder := &semanticSQLBuilder{}
	baseSQL, _, err := s.buildSemanticSearchBaseSQL(builder, params, space, vector)
	if err != nil {
		return 0, err
	}

	query := "SELECT COUNT(*) " + baseSQL
	var count int64
	if err := s.pool.QueryRow(ctx, query, builder.args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("failed to count assets: %w", err)
	}

	return count, nil
}

func (s *assetService) buildSemanticSearchBaseSQL(builder *semanticSQLBuilder, params QueryAssetsParams, space repo.EmbeddingSpace, vector *pgvector.Vector) (string, string, error) {
	if vector == nil {
		return "", "", fmt.Errorf("semantic query vector is nil")
	}
	if space.ID <= 0 || space.Dimensions <= 0 {
		return "", "", fmt.Errorf("invalid semantic search space")
	}

	embeddingPlaceholder := builder.addArg(vector)
	spacePlaceholder := builder.addArg(space.ID)

	distanceExpr := fmt.Sprintf("(e.vector::vector(%d) <-> %s::vector(%d))", space.Dimensions, embeddingPlaceholder, space.Dimensions)
	isDeleted := false
	if params.IsDeleted != nil {
		isDeleted = *params.IsDeleted
	}
	conditions := []string{
		fmt.Sprintf("a.is_deleted = %s", builder.addArg(isDeleted)),
		fmt.Sprintf("e.space_id = %s", spacePlaceholder),
		"e.is_primary = true",
	}

	if params.Source != nil {
		conditions = append(conditions, fmt.Sprintf("a.asset_id = ANY(%s::uuid[])", builder.addArg(assetSetSourceUUIDs(params.Source))))
	}
	if params.AssetType != nil {
		conditions = append(conditions, fmt.Sprintf("a.type = %s", builder.addArg(*params.AssetType)))
	}
	if len(params.AssetTypes) > 0 {
		conditions = append(conditions, fmt.Sprintf("a.type = ANY(%s::text[])", builder.addArg(params.AssetTypes)))
	}
	if params.OwnerID != nil {
		conditions = append(conditions, fmt.Sprintf("a.owner_id = %s", builder.addArg(*params.OwnerID)))
	}
	if params.RepositoryID != nil && *params.RepositoryID != "" {
		repositoryID, err := uuid.Parse(*params.RepositoryID)
		if err != nil {
			return "", "", fmt.Errorf("invalid repository ID: %w", err)
		}
		conditions = append(conditions, fmt.Sprintf("a.repository_id = %s", builder.addArg(repositoryID)))
	}
	if params.PersonID != nil {
		personPlaceholder := builder.addArg(*params.PersonID)
		conditions = append(conditions, fmt.Sprintf(`EXISTS (
			SELECT 1
			FROM face_cluster_members fcm
			JOIN face_items fi_person ON fi_person.id = fcm.face_id
			WHERE fcm.cluster_id = %s
			  AND fi_person.asset_id = a.asset_id
		)`, personPlaceholder))
	}
	if params.AlbumID != nil {
		albumPlaceholder := builder.addArg(*params.AlbumID)
		conditions = append(conditions, fmt.Sprintf(`EXISTS (
			SELECT 1
			FROM album_assets aa
			WHERE aa.asset_id = a.asset_id
			  AND aa.album_id = %s
		)`, albumPlaceholder))
	}
	if params.TagName != nil {
		tagNamePlaceholder := builder.addArg(*params.TagName)
		tagSourceCondition := ""
		if params.TagSource != nil {
			tagSourcePlaceholder := builder.addArg(*params.TagSource)
			tagSourceCondition = fmt.Sprintf("\n			  AND at.source = %s", tagSourcePlaceholder)
		}
		conditions = append(conditions, fmt.Sprintf(`EXISTS (
			SELECT 1
			FROM asset_tags at
			JOIN tags t ON t.tag_id = at.tag_id
			WHERE at.asset_id = a.asset_id
			  AND t.tag_name = %s%s
		)`, tagNamePlaceholder, tagSourceCondition))
	}
	if len(params.TagNames) > 0 {
		tagNamesPlaceholder := builder.addArg(params.TagNames)
		// Match assets carrying every requested tag (AND semantics).
		conditions = append(conditions, fmt.Sprintf(`(
			SELECT COUNT(DISTINCT t.tag_name)
			FROM asset_tags at
			JOIN tags t ON t.tag_id = at.tag_id
			WHERE at.asset_id = a.asset_id
			  AND t.tag_name = ANY(%s::text[])
		) = cardinality(%s::text[])`, tagNamesPlaceholder, tagNamesPlaceholder))
	}
	if params.FilenameValue != nil {
		filenamePlaceholder := builder.addArg(*params.FilenameValue)
		switch {
		case params.FilenameOperator != nil && *params.FilenameOperator == "matches":
			conditions = append(conditions, fmt.Sprintf("a.original_filename ILIKE %s", filenamePlaceholder))
		case params.FilenameOperator != nil && *params.FilenameOperator == "starts_with":
			conditions = append(conditions, fmt.Sprintf("a.original_filename ILIKE %s || '%%'", filenamePlaceholder))
		case params.FilenameOperator != nil && *params.FilenameOperator == "ends_with":
			conditions = append(conditions, fmt.Sprintf("a.original_filename ILIKE '%%' || %s", filenamePlaceholder))
		default:
			conditions = append(conditions, fmt.Sprintf("a.original_filename ILIKE '%%' || %s || '%%'", filenamePlaceholder))
		}
	}
	if params.DateFrom != nil {
		conditions = append(conditions, fmt.Sprintf("COALESCE(a.taken_time, a.upload_time) >= %s", builder.addArg(*params.DateFrom)))
	}
	if params.DateTo != nil {
		conditions = append(conditions, fmt.Sprintf("COALESCE(a.taken_time, a.upload_time) <= %s", builder.addArg(*params.DateTo)))
	}
	if params.IsRaw != nil {
		if *params.IsRaw {
			conditions = append(conditions, "a.specific_metadata->>'is_raw' = 'true'")
		} else {
			conditions = append(conditions, "(a.specific_metadata->>'is_raw' = 'false' OR a.specific_metadata->>'is_raw' IS NULL)")
		}
	}
	if params.Rating != nil {
		if *params.Rating == 0 {
			conditions = append(conditions, "(a.rating IS NULL OR a.rating = 0)")
		} else {
			conditions = append(conditions, fmt.Sprintf("a.rating = %s", builder.addArg(*params.Rating)))
		}
	}
	if params.Liked != nil {
		if *params.Liked {
			conditions = append(conditions, "a.liked = true")
		} else {
			conditions = append(conditions, "(a.liked IS NULL OR a.liked = false)")
		}
	}
	if params.CameraModel != nil {
		conditions = append(conditions, fmt.Sprintf("a.specific_metadata->>'camera_model' = %s", builder.addArg(*params.CameraModel)))
	}
	if params.LensModel != nil {
		conditions = append(conditions, fmt.Sprintf("a.specific_metadata->>'lens_model' = %s", builder.addArg(*params.LensModel)))
	}
	if params.LocationNorth != nil && params.LocationSouth != nil && params.LocationEast != nil && params.LocationWest != nil {
		northPlaceholder := builder.addArg(*params.LocationNorth)
		southPlaceholder := builder.addArg(*params.LocationSouth)
		eastPlaceholder := builder.addArg(*params.LocationEast)
		westPlaceholder := builder.addArg(*params.LocationWest)
		conditions = append(conditions, fmt.Sprintf(`a.gps_latitude IS NOT NULL
  AND a.gps_longitude IS NOT NULL
  AND a.gps_latitude BETWEEN LEAST(%s::float8, %s::float8) AND GREATEST(%s::float8, %s::float8)
  AND (
    CASE
      WHEN %s::float8 <= %s::float8 THEN a.gps_longitude BETWEEN %s::float8 AND %s::float8
      ELSE a.gps_longitude >= %s::float8 OR a.gps_longitude <= %s::float8
    END
  )`, southPlaceholder, northPlaceholder, southPlaceholder, northPlaceholder, westPlaceholder, eastPlaceholder, westPlaceholder, eastPlaceholder, westPlaceholder, eastPlaceholder))
	}

	baseSQL := fmt.Sprintf(`
FROM embeddings e
JOIN assets a ON a.asset_id = e.asset_id
WHERE %s`, strings.Join(conditions, "\n  AND "))

	return baseSQL, distanceExpr, nil
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

	total, err := s.queries.CountPhotoMapPoints(ctx, repo.CountPhotoMapPointsParams{
		RepositoryID: repoUUID,
		OwnerID:      params.OwnerID,
		South:        params.South,
		North:        params.North,
		West:         params.West,
		East:         params.East,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count photo map points: %w", err)
	}

	rows, err := s.queries.GetPhotoMapPoints(ctx, repo.GetPhotoMapPointsParams{
		RepositoryID: repoUUID,
		OwnerID:      params.OwnerID,
		South:        params.South,
		North:        params.North,
		West:         params.West,
		East:         params.East,
		Limit:        int32(params.Limit),
		Offset:       int32(params.Offset),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query photo map points: %w", err)
	}

	points := make([]PhotoMapPoint, 0, len(rows))
	for _, row := range rows {
		if !row.AssetID.Valid || !row.UploadTime.Valid || row.GpsLatitude == nil || row.GpsLongitude == nil {
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
			GPSLatitude:      *row.GpsLatitude,
			GPSLongitude:     *row.GpsLongitude,
		})
	}

	return points, total, nil
}

func candidateIDs(candidates []aggregatesearch.Candidate) []uuid.UUID {
	ids := make([]uuid.UUID, len(candidates))
	for i, c := range candidates {
		ids[i] = c.AssetID
	}
	return ids
}

// SearchAssetIDsSemantic returns the set of asset ids within the per-query
// calibrated relevance cutoff, in similarity order.
func (s *assetService) SearchAssetIDsSemantic(ctx context.Context, query string, strictness aggregatesearch.SetStrictness, maxResults int) ([]uuid.UUID, aggregatesearch.SetMeta, error) {
	if s.semanticRetriever == nil {
		return nil, aggregatesearch.SetMeta{}, ErrSemanticSearchUnavailable
	}
	candidates, meta, err := s.semanticRetriever.RetrieveSet(ctx, aggregatesearch.Request{Query: query}, strictness, maxResults)
	if err != nil {
		return nil, meta, err
	}
	return candidateIDs(candidates), meta, nil
}

// SearchAssetIDsOCR returns asset ids ranked by OCR full-text relevance.
// tsquery matching is the membership test, so no calibration is needed.
func (s *assetService) SearchAssetIDsOCR(ctx context.Context, query string, maxResults int) ([]uuid.UUID, error) {
	if s.ocrRetriever == nil {
		return nil, ErrSemanticSearchUnavailable
	}
	candidates, err := s.ocrRetriever.Retrieve(ctx, aggregatesearch.Request{Query: query, TopK: maxResults})
	if err != nil {
		return nil, err
	}
	return candidateIDs(candidates), nil
}

// filenameMembershipParams mirrors the query's filter for the filename
// channel of the Results tier.
func filenameMembershipParams(params QueryAssetsParams) repo.GetAssetIDsUnifiedParams {
	out := repo.GetAssetIDsUnifiedParams{Limit: fusedSetCap}
	out.AssetIds = assetSetSourcePgUUIDs(params.Source)
	if params.Query != "" {
		operator := "contains"
		filename := params.Query
		out.FilenameVal = &filename
		out.FilenameOperator = &operator
	}
	out.AssetType = params.AssetType
	out.AssetTypes = params.AssetTypes
	out.OwnerID = params.OwnerID
	out.PersonID = params.PersonID
	out.AlbumID = params.AlbumID
	out.TagName = params.TagName
	out.TagNames = params.TagNames
	out.TagSource = params.TagSource
	out.FolderPath = params.FolderPath
	out.FolderRecursive = params.FolderRecursive
	if params.RepositoryID != nil && *params.RepositoryID != "" {
		if parsed, err := uuid.Parse(strings.TrimSpace(*params.RepositoryID)); err == nil {
			out.RepositoryID = pgtype.UUID{Bytes: parsed, Valid: true}
		}
	}
	if params.DateFrom != nil {
		out.DateFrom = pgtype.Timestamptz{Time: *params.DateFrom, Valid: true}
	}
	if params.DateTo != nil {
		out.DateTo = pgtype.Timestamptz{Time: *params.DateTo, Valid: true}
	}
	out.IsRaw = params.IsRaw
	out.IsDeleted = params.IsDeleted
	if params.Rating != nil {
		rating := int32(*params.Rating)
		out.Rating = &rating
	}
	out.Liked = params.Liked
	out.CameraModel = params.CameraModel
	out.LensModel = params.LensModel
	return out
}
