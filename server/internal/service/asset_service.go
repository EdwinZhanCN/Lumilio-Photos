package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/event"
	"server/internal/queue/jobs"
	aggregatesearch "server/internal/search"
	"server/internal/search/bleveocr"
	"server/internal/utils/geohash"
	"strings"
	"time"

	"github.com/edwinzhancn/lumen-sdk/pkg/types"
	"github.com/google/uuid"
	"github.com/riverqueue/river"
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

	CreateThumbnail(ctx context.Context, assetID uuid.UUID, size string, thumbnailPath string) (*repo.Thumbnail, error)
	DetectDuplicates(ctx context.Context, hash string) ([]repo.Asset, error)
	SaveAssetIndex(ctx context.Context, taskID string, hash string) error
	CreateAssetRecord(ctx context.Context, params repo.CreateAssetParams) (*repo.Asset, error)

	GetOrCreateTagByName(ctx context.Context, name, category string, isAIGenerated bool) (*repo.Tag, error)
	GetThumbnailByID(ctx context.Context, thumbnailID int) (*repo.Thumbnail, error)
	GetThumbnailByAssetIDAndSize(ctx context.Context, assetID uuid.UUID, size string) (*repo.Thumbnail, error)

	SaveNewAsset(ctx context.Context, fileReader io.Reader, filename string, hash string) (string, error)
	GetDistinctCameraModels(ctx context.Context) ([]string, error)
	GetDistinctLenses(ctx context.Context) ([]string, error)

	// Video and Audio processing methods
	UpdateAssetDuration(ctx context.Context, id uuid.UUID, duration float64) error
	UpdateAssetDimensions(ctx context.Context, id uuid.UUID, width, height int32) error

	// Unified query API
	QueryAssets(ctx context.Context, params QueryAssetsParams) ([]repo.Asset, int64, error)
	QueryBrowseItems(ctx context.Context, params QueryAssetsParams) (BrowseQueryResult, error)
	QueryMediaItems(ctx context.Context, params QueryAssetsParams) ([]BrowseMediaItem, int64, error)
	CountMediaItems(ctx context.Context, params QueryAssetsParams) (int64, error)
	CountMediaItemFiles(ctx context.Context, params QueryAssetsParams) (int64, error)
	SearchAssets(ctx context.Context, params SearchAssetsParams) (SearchAssetsResult, error)
	SearchBrowseItems(ctx context.Context, params SearchAssetsParams) (SearchBrowseResult, error)
	QueryPhotoMapPoints(ctx context.Context, params QueryPhotoMapPointsParams) ([]PhotoMapPoint, int64, error)

	// Single-retriever set search (agent producer path and the search Results
	// tier). The semantic channel applies a per-query calibrated relevance
	// cutoff instead of fixed TopK; the OCR channel is naturally thresholded
	// by tsquery matching. Rankings are the retrievers' own orders.
	SearchAssetIDsSemantic(ctx context.Context, query string, strictness aggregatesearch.SetStrictness, maxResults int) ([]uuid.UUID, aggregatesearch.SetMeta, error)
	SearchAssetIDsOCR(ctx context.Context, query string, maxResults int) ([]uuid.UUID, error)
	SearchAssetIDsSemanticForOwner(ctx context.Context, ownerID int32, query string, strictness aggregatesearch.SetStrictness, maxResults int) ([]uuid.UUID, aggregatesearch.SetMeta, error)
	SearchAssetIDsOCRForOwner(ctx context.Context, ownerID int32, query string, maxResults int) ([]uuid.UUID, error)

	// Folders and tags are derived/vocabulary collection views (no folder
	// entity exists; "folders" come from assets.storage_path prefixes).
	ListFolderSummaries(ctx context.Context, ownerID *int32, repositoryID *string, parentPath string) ([]FolderSummary, error)
	GetFolderSummary(ctx context.Context, ownerID *int32, repositoryID string, folderPath string) (FolderSummary, error)
	ListTagSummaries(ctx context.Context, ownerID *int32, repositoryID *string, source *string, query *string, limit, offset int) ([]TagSummary, error)
}

// MediaComposition filters logical media items by their component makeup.
type MediaComposition string

const (
	MediaCompositionContainsRAW MediaComposition = "contains_raw"
	MediaCompositionJPEGRAW     MediaComposition = "jpeg_raw"
	MediaCompositionRAWUnpaired MediaComposition = "raw_unpaired"
	MediaCompositionNoRAW       MediaComposition = "no_raw"
	MediaCompositionLivePhoto   MediaComposition = "live_photo"
)

// StackMembership filters media items by presentation-stack membership.
type StackMembership string

const (
	StackMembershipStacked   StackMembership = "stacked"
	StackMembershipUnstacked StackMembership = "unstacked"
)

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
	EventID          *string
	FilenameValue    *string
	FilenameOperator *string
	DateFrom         *time.Time
	DateTo           *time.Time
	MediaComposition MediaComposition // media-item component makeup filter (empty = all)
	StackMembership  StackMembership  // presentation-stack membership filter (empty = all)
	StackKinds       []string         // presentation-stack kind filter (non-empty implies stacked)
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
	AssetSetSourceEvent     AssetSetSourceKind = "event"
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
	pool                   *sql.DB
	lumen                  LumenService
	embeddingService       EmbeddingService
	aggregateSearch        aggregatesearch.Service
	semanticRetriever      *aggregatesearch.EmbeddingRetriever
	ocrRetriever           *aggregatesearch.BleveOCRRetriever
	placeRetriever         *aggregatesearch.TextRetriever
	queryAssetsUnifiedFn   func(ctx context.Context, params QueryAssetsParams) ([]repo.Asset, int64, error)
	searchAssetsFusedSetFn func(ctx context.Context, params SearchAssetsParams) (fusedSearchSet, bool)
	hydrateAssetsInOrderFn func(ctx context.Context, ids []uuid.UUID, isDeleted *bool) ([]repo.Asset, error)
	pageAssetsBySortFn     func(ctx context.Context, ids []uuid.UUID, sortBy string, limit, offset int, isDeleted *bool) ([]repo.Asset, error)
	eventQueue             *river.Client[*sql.Tx]
}

func NewAssetService(
	q *repo.Queries,
	pool *sql.DB,
	l LumenService,
	e EmbeddingService,
	ocrIndex *bleveocr.Index,
	loggers ...*zap.Logger,
) (AssetService, error) {
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
	svc.ocrRetriever = aggregatesearch.NewBleveOCRRetriever(pool, ocrIndex, 0.7)
	svc.placeRetriever = aggregatesearch.NewPlaceRetriever(pool, 0.8)
	svc.aggregateSearch = aggregatesearch.NewAggregateService(pool, []aggregatesearch.Retriever{
		svc.semanticRetriever,
		svc.ocrRetriever,
		svc.placeRetriever,
	}, logger.Named("aggregate_search"))
	return svc, nil
}

// NewAssetServiceWithQueue is the runtime constructor.  The queue is kept
// optional so unit tests and embedded callers can still exercise asset CRUD;
// when present, Event fact invalidation and the deduplicated rebuild enqueue
// commit in the same SQLite transaction.
func NewAssetServiceWithQueue(
	q *repo.Queries,
	pool *sql.DB,
	l LumenService,
	e EmbeddingService,
	ocrIndex *bleveocr.Index,
	queueClient *river.Client[*sql.Tx],
	loggers ...*zap.Logger,
) (AssetService, error) {
	created, err := NewAssetService(q, pool, l, e, ocrIndex, loggers...)
	if err != nil {
		return nil, err
	}
	if concrete, ok := created.(*assetService); ok {
		concrete.eventQueue = queueClient
	}
	return created, nil
}

// ================================
// Asset CRUD Operations
// ================================

// CreateAssetRecord creates a new asset record in the database
func (s *assetService) CreateAssetRecord(ctx context.Context, params repo.CreateAssetParams) (*repo.Asset, error) {
	// Note: taken_time will be set to NULL initially and updated later when EXIF is processed
	// This is because we need to extract the time from the actual file content, not just the parameters
	if params.AssetID == uuid.Nil {
		params.AssetID = uuid.New()
	}
	asset, err := s.queries.CreateAsset(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to create asset: %w", err)
	}

	return &asset, nil
}

// GetAsset retrieves an asset by its ID
func (s *assetService) GetAsset(ctx context.Context, id uuid.UUID) (*repo.Asset, error) {
	dbAsset, err := s.queries.GetAssetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get asset: %w", err)
	}

	return &dbAsset, nil
}

// GetAssetAny retrieves an asset by ID regardless of Trash state.
func (s *assetService) GetAssetAny(ctx context.Context, id uuid.UUID) (*repo.Asset, error) {
	dbAsset, err := s.queries.GetAssetByIDAny(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get asset: %w", err)
	}

	return &dbAsset, nil
}

func (s *assetService) GetAssetExifRaw(ctx context.Context, id uuid.UUID) (json.RawMessage, error) {
	exifRaw, err := s.queries.GetAssetExifRaw(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get asset exif: %w", err)
	}

	return json.RawMessage(exifRaw), nil
}

// GetAssetRelations returns a single asset together with its aggregated
// relations (thumbnails, tags, albums, species predictions, OCR, and face
// results) in one query. The handler projects this into a typed
// dto.AssetDetailDTO, honoring the include_* query flags. Trash state is not
// filtered here; handler auth decides access.
func (s *assetService) GetAssetRelations(ctx context.Context, id uuid.UUID) (repo.GetAssetWithRelationsRow, error) {
	row, err := s.queries.GetAssetWithRelations(ctx, id)
	if err != nil {
		return repo.GetAssetWithRelationsRow{}, fmt.Errorf("failed to get asset with relations: %w", err)
	}
	return row, nil
}

// GetAssetsByType retrieves assets by type with pagination
func (s *assetService) GetAssetsByType(ctx context.Context, assetType string, limit, offset int) ([]repo.Asset, error) {
	params := repo.GetAssetsByTypeParams{
		Type:   assetType,
		Limit:  int64(limit),
		Offset: int64(offset),
	}

	return s.queries.GetAssetsByType(ctx, params)
}

// GetAssetsByOwner retrieves assets by owner with pagination
func (s *assetService) GetAssetsByOwner(ctx context.Context, ownerID int, limit, offset int) ([]repo.Asset, error) {
	params := repo.GetAssetsByOwnerParams{
		OwnerID: int32PtrFromIntPtr(&ownerID),
		Limit:   int64(limit),
		Offset:  int64(offset),
	}

	return s.queries.GetAssetsByOwner(ctx, params)
}

// GetAssetsByOwnerSorted retrieves assets by owner sorted by taken_time
func (s *assetService) GetAssetsByOwnerSorted(ctx context.Context, ownerID int, sortOrder string, limit, offset int) ([]repo.Asset, error) {
	params := repo.GetAssetsByOwnerSortedParams{
		OwnerID:   int32PtrFromIntPtr(&ownerID),
		SortOrder: sortOrder,
		Limit:     int64(limit),
		Offset:    int64(offset),
	}

	return s.queries.GetAssetsByOwnerSorted(ctx, params)
}

// GetAssetsByTypesSorted retrieves assets by multiple types sorted by taken_time
func (s *assetService) GetAssetsByTypesSorted(ctx context.Context, assetTypes []string, sortOrder string, limit, offset int) ([]repo.Asset, error) {
	params := repo.GetAssetsByTypesSortedParams{
		Types:     dbtypes.StringsJSONParam(assetTypes),
		SortOrder: sortOrder,
		Limit:     int64(limit),
		Offset:    int64(offset),
	}

	return s.queries.GetAssetsByTypesSorted(ctx, params)
}

// GetAssetsByOwnerAndTypes retrieves assets by owner and multiple types sorted by taken_time
func (s *assetService) GetAssetsByOwnerAndTypes(ctx context.Context, ownerID int, assetTypes []string, sortOrder string, limit, offset int) ([]repo.Asset, error) {
	params := repo.GetAssetsByOwnerAndTypesSortedParams{
		OwnerID:   int32PtrFromIntPtr(&ownerID),
		Types:     dbtypes.StringsJSONParam(assetTypes),
		SortOrder: sortOrder,
		Limit:     int64(limit),
		Offset:    int64(offset),
	}

	return s.queries.GetAssetsByOwnerAndTypesSorted(ctx, params)
}

// DetectDuplicates finds assets with the same hash
func (s *assetService) DetectDuplicates(ctx context.Context, hash string) ([]repo.Asset, error) {
	return s.queries.GetAssetsByContentHash(ctx, hash)
}

// UpdateAssetMetadata updates the specific metadata of an asset and extracts taken_time
func (s *assetService) UpdateAssetMetadata(ctx context.Context, id uuid.UUID, metadata dbtypes.SpecificMetadata) error {
	return s.UpdateAssetMetadataWithExifRaw(ctx, id, metadata, nil)
}

func (s *assetService) UpdateAssetMetadataWithExifRaw(ctx context.Context, id uuid.UUID, metadata dbtypes.SpecificMetadata, exifRaw json.RawMessage) error {
	// Get the asset to determine its type for taken_time extraction
	asset, err := s.queries.GetAssetByID(ctx, id)
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
	var takenTimeParam any
	if takenTime != nil {
		takenTimeParam = dbtypes.NewTimestamp(*takenTime)
	}
	var captureOffset *int64
	if captureOffsetMinutes != nil {
		value := int64(*captureOffsetMinutes)
		captureOffset = &value
	}

	params := repo.UpdateAssetMetadataWithTakenTimeParams{
		AssetID:              id,
		SpecificMetadata:     metadata,
		ExifRaw:              dbtypes.JSON(exifRaw),
		TakenTime:            takenTimeParam,
		CaptureOffsetMinutes: captureOffset,
		GpsLatitude:          gpsLatitude,
		GpsLongitude:         gpsLongitude,
		GpsGeohash5:          gpsGeohash5,
		GpsGeohash7:          gpsGeohash7,
	}

	tx, err := s.pool.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin metadata update transaction: %w", err)
	}
	defer tx.Rollback()
	if err := s.queries.WithTx(tx).UpdateAssetMetadataWithTakenTime(ctx, params); err != nil {
		return err
	}
	if asset.OwnerID != nil {
		if err := event.MarkEventFactsChangedTx(ctx, tx, *asset.OwnerID, "asset_metadata_changed"); err != nil {
			return err
		}
		if err := s.enqueueEventRebuildTx(ctx, tx, *asset.OwnerID); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit metadata update transaction: %w", err)
	}
	return nil
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
	tx, err := s.pool.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin asset Trash transaction: %w", err)
	}
	defer tx.Rollback()
	queries := s.queries.WithTx(tx)
	if err := queries.DeleteAsset(ctx, id); err != nil {
		return err
	}
	// The trashed component can no longer serve as the browsing component of
	// its logical media item; re-pick the primary from what remains.
	if item, err := queries.GetMediaItemByAssetID(ctx, id); err == nil {
		if err := NormalizeMediaItemPrimaryAsset(ctx, queries, item.MediaItemID); err != nil {
			return fmt.Errorf("normalize media item after Trash: %w", err)
		}
	} else if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if err := enqueueOCRIndexOutbox(ctx, queries, id); err != nil {
		return err
	}
	if item, err := queries.GetMediaItemByAssetID(ctx, id); err == nil && item.OwnerID != nil {
		if err := event.MarkEventFactsChangedTx(ctx, tx, *item.OwnerID, "asset_trashed"); err != nil {
			return err
		}
		if err := s.enqueueEventRebuildTx(ctx, tx, *item.OwnerID); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit asset Trash transaction: %w", err)
	}
	return nil
}

// RestoreAsset restores an asset from the app Trash.
func (s *assetService) RestoreAsset(ctx context.Context, id uuid.UUID) error {
	tx, err := s.pool.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin asset restore transaction: %w", err)
	}
	defer tx.Rollback()
	queries := s.queries.WithTx(tx)
	if err := queries.RestoreAsset(ctx, id); err != nil {
		return err
	}
	// A restored component may reclaim the canonical primary slot (for example
	// a JPEG that outranks the RAW that served while it was trashed).
	if item, err := queries.GetMediaItemByAssetID(ctx, id); err == nil {
		if err := NormalizeMediaItemPrimaryAsset(ctx, queries, item.MediaItemID); err != nil {
			return fmt.Errorf("normalize media item after restore: %w", err)
		}
	} else if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if err := enqueueOCRIndexOutbox(ctx, queries, id); err != nil {
		return err
	}
	if item, err := queries.GetMediaItemByAssetID(ctx, id); err == nil && item.OwnerID != nil {
		if err := event.MarkEventFactsChangedTx(ctx, tx, *item.OwnerID, "asset_restored"); err != nil {
			return err
		}
		if err := s.enqueueEventRebuildTx(ctx, tx, *item.OwnerID); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit asset restore transaction: %w", err)
	}
	return nil
}

func (s *assetService) enqueueEventRebuildTx(ctx context.Context, tx *sql.Tx, ownerID int32) error {
	if s.eventQueue == nil {
		return nil
	}
	args := jobs.EventRebuildArgs{OwnerID: ownerID}
	opts := args.InsertOpts()
	if _, err := s.eventQueue.InsertTx(ctx, tx, args, &opts); err != nil {
		return fmt.Errorf("enqueue Event rebuild: %w", err)
	}
	return nil
}

// AddAssetToAlbum adds an asset to an album
func (s *assetService) AddAssetToAlbum(ctx context.Context, assetID uuid.UUID, albumID int) error {
	params := repo.AddAssetToAlbumParams{
		AssetID: assetID,
		AlbumID: int32(albumID),
	}

	return s.queries.AddAssetToAlbum(ctx, params)
}

// RemoveAssetFromAlbum removes an asset from an album
func (s *assetService) RemoveAssetFromAlbum(ctx context.Context, assetID uuid.UUID, albumID int) error {
	params := repo.RemoveAssetFromAlbumParams{
		AssetID: assetID,
		AlbumID: int32(albumID),
	}

	return s.queries.RemoveAssetFromAlbum(ctx, params)
}

// AddTagToAsset adds a tag to an asset
func (s *assetService) AddTagToAsset(ctx context.Context, assetID uuid.UUID, tagID int, confidence float32, source string) error {
	params := repo.AddTagToAssetParams{
		AssetID:    assetID,
		TagID:      int32(tagID),
		Confidence: float64(confidence),
		Source:     source,
	}

	return s.queries.AddTagToAsset(ctx, params)
}

// RemoveTagFromAsset removes a tag from an asset
func (s *assetService) RemoveTagFromAsset(ctx context.Context, assetID uuid.UUID, tagID int) error {
	params := repo.RemoveTagFromAssetParams{
		AssetID: assetID,
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
	row, err := s.queries.GetAssetWithTags(ctx, assetID)
	if err != nil {
		return nil, err
	}

	// The SQLite driver exposes aggregate expressions dynamically; normalize
	// the result to raw JSON bytes for the caller to unmarshal.
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
		Limit: int64(limit),
		Query: q,
	})
}

// SaveAssetIndex implements the INDEX step: verify asset exists by hash and complete indexing
func (s *assetService) SaveAssetIndex(ctx context.Context, taskID string, hash string) error {
	assets, err := s.queries.GetAssetsByContentHash(ctx, hash)
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
func (s *assetService) CreateThumbnail(ctx context.Context, assetID uuid.UUID, size string, thumbnailPath string) (*repo.Thumbnail, error) {
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
	params := repo.GetThumbnailByAssetAndSizeParams{
		AssetID: assetID,
		Size:    size,
	}

	dbThumbnail, err := s.queries.GetThumbnailByAssetAndSize(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to get thumbnail: %w", err)
	}

	return &dbThumbnail, nil
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
		IsAiGenerated: isAIGenerated,
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
	value := int64(rating)
	params := repo.UpdateAssetRatingParams{
		AssetID: id,
		Rating:  &value,
	}

	return s.queries.UpdateAssetRating(ctx, params)
}

func (s *assetService) UpdateAssetLike(ctx context.Context, id uuid.UUID, liked bool) error {
	params := repo.UpdateAssetLikeParams{
		AssetID: id,
		Liked:   liked,
	}

	return s.queries.UpdateAssetLike(ctx, params)
}

func (s *assetService) UpdateAssetRatingAndLike(ctx context.Context, id uuid.UUID, rating int, liked bool) error {
	value := int64(rating)
	params := repo.UpdateAssetRatingAndLikeParams{
		AssetID: id,
		Rating:  &value,
		Liked:   liked,
	}

	return s.queries.UpdateAssetRatingAndLike(ctx, params)
}

func (s *assetService) UpdateAssetDescription(ctx context.Context, id uuid.UUID, description string) error {
	params := repo.UpdateAssetDescriptionParams{
		AssetID:     id,
		Description: description,
	}

	return s.queries.UpdateAssetDescription(ctx, params)
}

func (s *assetService) GetAssetsByRating(ctx context.Context, rating int, ownerID *int32, limit, offset int) ([]repo.Asset, error) {
	ratingValue := int64(rating)
	params := repo.GetAssetsByRatingParams{
		Rating:  &ratingValue,
		OwnerID: ownerID,
		Limit:   int64(limit),
		Offset:  int64(offset),
	}

	return s.queries.GetAssetsByRating(ctx, params)
}

func (s *assetService) GetLikedAssets(ctx context.Context, ownerID *int32, limit, offset int) ([]repo.Asset, error) {
	params := repo.GetLikedAssetsParams{
		OwnerID: ownerID,
		Limit:   int64(limit),
		Offset:  int64(offset),
	}

	return s.queries.GetLikedAssets(ctx, params)
}

func (s *assetService) UpdateAssetDuration(ctx context.Context, id uuid.UUID, duration float64) error {
	params := repo.UpdateAssetDurationParams{
		AssetID:  id,
		Duration: &duration,
	}

	return s.queries.UpdateAssetDuration(ctx, params)
}

func (s *assetService) UpdateAssetDimensions(ctx context.Context, id uuid.UUID, width, height int32) error {
	widthValue := int64(width)
	heightValue := int64(height)
	params := repo.UpdateAssetDimensionsParams{
		AssetID: id,
		Width:   &widthValue,
		Height:  &heightValue,
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

func assetSetSourceSQLiteUUIDs(source *AssetSetSource) *string {
	if source == nil {
		return nil
	}
	ids := make(dbtypes.UUIDs, 0, len(source.AssetIDs))
	for _, id := range source.AssetIDs {
		if id == uuid.Nil {
			continue
		}
		ids = append(ids, id)
	}
	return dbtypes.UUIDsJSONParam(ids)
}

func sqliteStrings(values []string) *string {
	return dbtypes.StringsJSONParam(values)
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
	var err error
	params.QueryAssetsParams, err = s.applyEventScope(ctx, params.QueryAssetsParams)
	if err != nil {
		return SearchAssetsResult{}, err
	}

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
	var err error
	params, err = s.applyEventScope(ctx, params)
	if err != nil {
		return nil, 0, err
	}
	if params.SearchType == "semantic" && params.Query != "" {
		return s.queryAssetsAggregate(ctx, params)
	}
	return s.queryAssetsUnified(ctx, params)
}

// applyEventScope resolves the owner-authorized logical Event set once and
// turns it into the existing asset-set filter used by every browse/search
// implementation.  This prevents EventID from being silently ignored by a
// media-item query path while keeping repository_id a read-only projection.
func (s *assetService) applyEventScope(ctx context.Context, params QueryAssetsParams) (QueryAssetsParams, error) {
	if params.EventID == nil || strings.TrimSpace(*params.EventID) == "" {
		return params, nil
	}
	if params.OwnerID == nil {
		return QueryAssetsParams{}, event.ErrNotFound
	}
	resolved, _, err := event.NewResolver(s.pool).OrderedAssets(ctx, *params.OwnerID, *params.EventID, 0)
	if err != nil {
		return QueryAssetsParams{}, err
	}
	ids := make([]uuid.UUID, 0, len(resolved))
	for _, item := range resolved {
		id, err := uuid.Parse(item.AssetID)
		if err != nil {
			return QueryAssetsParams{}, fmt.Errorf("parse Event asset ID: %w", err)
		}
		ids = append(ids, id)
	}
	if params.Source != nil {
		allowed := make(map[uuid.UUID]struct{}, len(ids))
		for _, id := range ids {
			allowed[id] = struct{}{}
		}
		filtered := make([]uuid.UUID, 0, len(params.Source.AssetIDs))
		for _, id := range params.Source.AssetIDs {
			if _, ok := allowed[id]; ok {
				filtered = append(filtered, id)
			}
		}
		ids = filtered
	}
	params.Source = &AssetSetSource{Kind: AssetSetSourceEvent, AssetIDs: ids, PreserveSnapshotOrder: true}
	return params, nil
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
		Query:  params.Query,
		Filter: filter,
		Limit:  params.Limit,
		Offset: params.Offset,
		Debug:  false,
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

// unifiedQueryInputs carries QueryAssetsParams values converted to the shapes
// the unified media-item SQL queries expect (JSON-encoded lists, SQLite
// timestamps, nullable enums).
type unifiedQueryInputs struct {
	assetIDs        *string
	assetTypes      *string
	tagNames        *string
	stackKinds      *string
	repoUUID        uuid.NullUUID
	rating          *int32
	dateFrom        dbtypes.Timestamp
	dateTo          dbtypes.Timestamp
	query           *string
	sortBy          *string
	composition     *string
	stackMembership *string
	isDeleted       bool
}

func mediaCompositionParam(value MediaComposition) *string {
	if value == "" {
		return nil
	}
	s := string(value)
	return &s
}

func stackMembershipParam(value StackMembership) *string {
	if value == "" {
		return nil
	}
	s := string(value)
	return &s
}

func newUnifiedQueryInputs(params QueryAssetsParams) (unifiedQueryInputs, error) {
	in := unifiedQueryInputs{
		assetIDs:        assetSetSourceSQLiteUUIDs(params.Source),
		assetTypes:      sqliteStrings(params.AssetTypes),
		tagNames:        sqliteStrings(params.TagNames),
		stackKinds:      sqliteStrings(params.StackKinds),
		composition:     mediaCompositionParam(params.MediaComposition),
		stackMembership: stackMembershipParam(params.StackMembership),
		isDeleted:       params.IsDeleted != nil && *params.IsDeleted,
	}

	if params.RepositoryID != nil && *params.RepositoryID != "" {
		parsedUUID, err := uuid.Parse(*params.RepositoryID)
		if err != nil {
			return unifiedQueryInputs{}, fmt.Errorf("invalid repository ID: %w", err)
		}
		in.repoUUID = uuid.NullUUID{UUID: parsedUUID, Valid: true}
	}
	if params.Rating != nil {
		r := int32(*params.Rating)
		in.rating = &r
	}
	if params.DateFrom != nil {
		in.dateFrom = dbtypes.NewTimestamp(*params.DateFrom)
	}
	if params.DateTo != nil {
		in.dateTo = dbtypes.NewTimestamp(*params.DateTo)
	}
	if params.Query != "" {
		query := params.Query
		in.query = &query
	}
	switch params.SortBy {
	case "recently_added", "date_captured":
		sortBy := params.SortBy
		in.sortBy = &sortBy
	}
	return in, nil
}

func countMediaItemsUnifiedParams(params QueryAssetsParams, in unifiedQueryInputs) repo.CountMediaItemsUnifiedParams {
	return repo.CountMediaItemsUnifiedParams{
		AssetIds:         in.assetIDs,
		AssetTypes:       in.assetTypes,
		TagNames:         in.tagNames,
		StackKinds:       in.stackKinds,
		IsDeleted:        in.isDeleted,
		Query:            in.query,
		AssetType:        params.AssetType,
		OwnerID:          params.OwnerID,
		RepositoryID:     in.repoUUID,
		FolderPath:       params.FolderPath,
		FolderRecursive:  params.FolderRecursive,
		PersonID:         params.PersonID,
		AlbumID:          params.AlbumID,
		TagName:          params.TagName,
		TagSource:        params.TagSource,
		FilenameVal:      params.FilenameValue,
		FilenameOperator: params.FilenameOperator,
		DateFrom:         in.dateFrom,
		DateTo:           in.dateTo,
		Composition:      in.composition,
		StackMembership:  in.stackMembership,
		Rating:           in.rating,
		Liked:            params.Liked,
		CameraModel:      params.CameraModel,
		LensModel:        params.LensModel,
		LocationNorth:    params.LocationNorth,
		LocationSouth:    params.LocationSouth,
		LocationEast:     params.LocationEast,
		LocationWest:     params.LocationWest,
	}
}

func countMediaItemFilesUnifiedParams(params QueryAssetsParams, in unifiedQueryInputs) repo.CountMediaItemFilesUnifiedParams {
	return repo.CountMediaItemFilesUnifiedParams{
		AssetIds:         in.assetIDs,
		AssetTypes:       in.assetTypes,
		TagNames:         in.tagNames,
		StackKinds:       in.stackKinds,
		IsDeleted:        in.isDeleted,
		Query:            in.query,
		AssetType:        params.AssetType,
		OwnerID:          params.OwnerID,
		RepositoryID:     in.repoUUID,
		FolderPath:       params.FolderPath,
		FolderRecursive:  params.FolderRecursive,
		PersonID:         params.PersonID,
		AlbumID:          params.AlbumID,
		TagName:          params.TagName,
		TagSource:        params.TagSource,
		FilenameVal:      params.FilenameValue,
		FilenameOperator: params.FilenameOperator,
		DateFrom:         in.dateFrom,
		DateTo:           in.dateTo,
		Composition:      in.composition,
		StackMembership:  in.stackMembership,
		Rating:           in.rating,
		Liked:            params.Liked,
		CameraModel:      params.CameraModel,
		LensModel:        params.LensModel,
		LocationNorth:    params.LocationNorth,
		LocationSouth:    params.LocationSouth,
		LocationEast:     params.LocationEast,
		LocationWest:     params.LocationWest,
	}
}

func getMediaItemsUnifiedParams(params QueryAssetsParams, in unifiedQueryInputs) repo.GetMediaItemsUnifiedParams {
	return repo.GetMediaItemsUnifiedParams{
		AssetIds:         in.assetIDs,
		AssetTypes:       in.assetTypes,
		TagNames:         in.tagNames,
		StackKinds:       in.stackKinds,
		SortBy:           in.sortBy,
		IsDeleted:        in.isDeleted,
		Query:            in.query,
		AssetType:        params.AssetType,
		OwnerID:          params.OwnerID,
		RepositoryID:     in.repoUUID,
		FolderPath:       params.FolderPath,
		FolderRecursive:  params.FolderRecursive,
		PersonID:         params.PersonID,
		AlbumID:          params.AlbumID,
		TagName:          params.TagName,
		TagSource:        params.TagSource,
		FilenameVal:      params.FilenameValue,
		FilenameOperator: params.FilenameOperator,
		DateFrom:         in.dateFrom,
		DateTo:           in.dateTo,
		Composition:      in.composition,
		StackMembership:  in.stackMembership,
		Rating:           in.rating,
		Liked:            params.Liked,
		CameraModel:      params.CameraModel,
		LensModel:        params.LensModel,
		LocationNorth:    params.LocationNorth,
		LocationSouth:    params.LocationSouth,
		LocationEast:     params.LocationEast,
		LocationWest:     params.LocationWest,
		Offset:           int64(params.Offset),
		Limit:            int64(params.Limit),
	}
}

// queryAssetsUnified lists one primary asset per matching logical media item.
// The returned total counts media items, not component files.
func (s *assetService) queryAssetsUnified(ctx context.Context, params QueryAssetsParams) ([]repo.Asset, int64, error) {
	in, err := newUnifiedQueryInputs(params)
	if err != nil {
		return nil, 0, err
	}

	total, err := s.queries.CountMediaItemsUnified(ctx, countMediaItemsUnifiedParams(params, in))
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count media items: %w", err)
	}

	rows, err := s.queries.GetMediaItemsUnified(ctx, getMediaItemsUnifiedParams(params, in))
	if err != nil {
		return nil, 0, err
	}

	assets := make([]repo.Asset, 0, len(rows))
	for _, row := range rows {
		assets = append(assets, row.Asset)
	}
	return assets, total, nil
}

func (s *assetService) queryAssetsVector(ctx context.Context, params QueryAssetsParams) ([]repo.Asset, int64, error) {
	if s.semanticRetriever == nil {
		return nil, 0, ErrSemanticSearchUnavailable
	}
	filter, err := buildAggregateSearchFilter(params)
	if err != nil {
		return nil, 0, err
	}
	candidates, _, err := s.semanticRetriever.RetrieveSet(ctx, aggregatesearch.Request{
		Query:  params.Query,
		Filter: filter,
	}, aggregatesearch.StrictnessNormal, fusedSetCap)
	if err != nil {
		return nil, 0, err
	}
	total := int64(len(candidates))
	if params.Offset < 0 {
		params.Offset = 0
	}
	if params.Limit <= 0 {
		params.Limit = 50
	}
	if params.Offset >= len(candidates) {
		return []repo.Asset{}, total, nil
	}
	end := params.Offset + params.Limit
	if end > len(candidates) {
		end = len(candidates)
	}
	assets, err := s.hydrateAssetsInOrder(ctx, candidateIDs(candidates[params.Offset:end]), params.IsDeleted)
	return assets, total, err
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
	// Canonicalize the query vector identically to stored image vectors (see
	// SaveEmbedding) so query and index live in the same MRL-truncated,
	// unit-length space.
	embeddingResult.Vector = canonicalizeSemanticVector(embeddingResult.Vector)
	return embeddingResult, nil
}

func (s *assetService) searchAssetsInResolvedSpace(ctx context.Context, params QueryAssetsParams, model string, vector []float32, limit, offset int, includeCount bool) ([]repo.Asset, int64, error) {
	_ = model
	_ = vector
	params.Limit = limit
	params.Offset = offset
	assets, total, err := s.queryAssetsVector(ctx, params)
	if !includeCount {
		total = 0
	}
	return assets, total, err
}

func (s *assetService) QueryPhotoMapPoints(ctx context.Context, params QueryPhotoMapPointsParams) ([]PhotoMapPoint, int64, error) {
	var repoUUID uuid.NullUUID
	if params.RepositoryID != nil && *params.RepositoryID != "" {
		parsedUUID, err := uuid.Parse(*params.RepositoryID)
		if err != nil {
			return nil, 0, fmt.Errorf("invalid repository ID: %w", err)
		}
		repoUUID = uuid.NullUUID{UUID: parsedUUID, Valid: true}
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
		Limit:        int64(params.Limit),
		Offset:       int64(params.Offset),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query photo map points: %w", err)
	}

	points := make([]PhotoMapPoint, 0, len(rows))
	for _, row := range rows {
		if row.AssetID == uuid.Nil || !row.UploadTime.Valid || row.GpsLatitude == nil || row.GpsLongitude == nil {
			continue
		}

		var takenTime *time.Time
		if row.TakenTime.Valid {
			t := row.TakenTime.Time
			takenTime = &t
		}

		points = append(points, PhotoMapPoint{
			AssetID:          row.AssetID.String(),
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

// SearchAssetIDsSemanticForOwner is the Agent-only data-plane entrypoint. The
// owner filter is part of retrieval itself, so unauthorized candidates never
// enter ranking, calibration, refs, or live-pin snapshots.
func (s *assetService) SearchAssetIDsSemanticForOwner(ctx context.Context, ownerID int32, query string, strictness aggregatesearch.SetStrictness, maxResults int) ([]uuid.UUID, aggregatesearch.SetMeta, error) {
	if s.semanticRetriever == nil {
		return nil, aggregatesearch.SetMeta{}, ErrSemanticSearchUnavailable
	}
	candidates, meta, err := s.semanticRetriever.RetrieveSet(ctx, aggregatesearch.Request{
		Query:  query,
		Filter: aggregatesearch.Filter{OwnerID: &ownerID},
	}, strictness, maxResults)
	if err != nil {
		return nil, meta, err
	}
	return candidateIDs(candidates), meta, nil
}

func (s *assetService) SearchAssetIDsOCRForOwner(ctx context.Context, ownerID int32, query string, maxResults int) ([]uuid.UUID, error) {
	if s.ocrRetriever == nil {
		return nil, ErrSemanticSearchUnavailable
	}
	candidates, err := s.ocrRetriever.Retrieve(ctx, aggregatesearch.Request{
		Query:  query,
		TopK:   maxResults,
		Filter: aggregatesearch.Filter{OwnerID: &ownerID},
	})
	if err != nil {
		return nil, err
	}
	return candidateIDs(candidates), nil
}

// filenameMembershipParams mirrors the query's filter for the filename
// channel of the Results tier. Rows are (media_item_id, primary_asset_id)
// pairs at media-item granularity.
func filenameMembershipParams(params QueryAssetsParams) repo.GetMediaItemRefsUnifiedParams {
	out := repo.GetMediaItemRefsUnifiedParams{Limit: fusedSetCap}
	out.AssetIds = assetSetSourceSQLiteUUIDs(params.Source)
	if params.Query != "" {
		operator := "contains"
		filename := params.Query
		out.FilenameVal = &filename
		out.FilenameOperator = &operator
	}
	out.AssetType = params.AssetType
	out.AssetTypes = sqliteStrings(params.AssetTypes)
	out.OwnerID = params.OwnerID
	out.PersonID = params.PersonID
	out.AlbumID = params.AlbumID
	out.TagName = params.TagName
	out.TagNames = sqliteStrings(params.TagNames)
	out.TagSource = params.TagSource
	out.FolderPath = params.FolderPath
	out.FolderRecursive = params.FolderRecursive
	if params.RepositoryID != nil && *params.RepositoryID != "" {
		if parsed, err := uuid.Parse(strings.TrimSpace(*params.RepositoryID)); err == nil {
			out.RepositoryID = uuid.NullUUID{UUID: parsed, Valid: true}
		}
	}
	if params.DateFrom != nil {
		out.DateFrom = dbtypes.NewTimestamp(*params.DateFrom)
	}
	if params.DateTo != nil {
		out.DateTo = dbtypes.NewTimestamp(*params.DateTo)
	}
	out.Composition = mediaCompositionParam(params.MediaComposition)
	out.StackMembership = stackMembershipParam(params.StackMembership)
	out.StackKinds = sqliteStrings(params.StackKinds)
	out.IsDeleted = params.IsDeleted != nil && *params.IsDeleted
	if params.Rating != nil {
		rating := int32(*params.Rating)
		out.Rating = &rating
	}
	out.Liked = params.Liked
	out.CameraModel = params.CameraModel
	out.LensModel = params.LensModel
	return out
}
