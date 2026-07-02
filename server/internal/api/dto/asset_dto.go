package dto

import (
	"encoding/json"
	"fmt"
	"time"

	"server/internal/db/dbtypes"
	"server/internal/db/repo"

	"github.com/google/uuid"
)

// UploadAssetRequestDTO represents the request structure for asset upload
type UploadAssetRequestDTO struct {
	RepositoryID string `form:"repository_id" binding:"omitempty,uuid" example:"550e8400-e29b-41d4-a716-446655440000"`
}

// BatchUploadRequestDTO represents the request structure for batch upload
type BatchUploadRequestDTO struct {
	RepositoryID string `form:"repository_id" binding:"omitempty,uuid" example:"550e8400-e29b-41d4-a716-446655440000"`
}

// ReprocessAssetRequestDTO represents the request structure for asset reprocessing
type ReprocessAssetRequestDTO struct {
	Tasks          []string `json:"tasks" binding:"omitempty" example:"thumbnail_small,thumbnail_medium,transcode_1080p"`
	ForceFullRetry bool     `json:"force_full_retry,omitempty" example:"false"`
}

// ReprocessAssetResponseDTO represents the response structure for asset reprocessing
type ReprocessAssetResponseDTO struct {
	AssetID     string   `json:"asset_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Status      string   `json:"status" example:"queued"`
	Message     string   `json:"message" example:"Reprocessing job queued successfully"`
	FailedTasks []string `json:"failed_tasks,omitempty" example:"thumbnail_small,transcode_1080p"`
	RetryTasks  []string `json:"retry_tasks,omitempty" example:"thumbnail_small,transcode_1080p"`
}

type RebuildAssetIndexesRequestDTO struct {
	RepositoryID string   `json:"repository_id,omitempty" binding:"omitempty,uuid" example:"550e8400-e29b-41d4-a716-446655440000"`
	Tasks        []string `json:"tasks,omitempty" example:"semantic,ocr"`
	Limit        int      `json:"limit,omitempty" minimum:"1" maximum:"500" example:"200"`
	MissingOnly  *bool    `json:"missing_only,omitempty" example:"true"`
}

type RebuildAssetIndexesResponseDTO struct {
	Status         string   `json:"status" example:"queued"`
	Message        string   `json:"message" example:"Index rebuild job queued successfully"`
	JobID          int64    `json:"job_id" example:"123"`
	RequestedTasks []string `json:"requested_tasks"`
	DisabledTasks  []string `json:"disabled_tasks,omitempty"`
	Limit          int      `json:"limit" example:"200"`
	MissingOnly    bool     `json:"missing_only" example:"true"`
	RepositoryID   *string  `json:"repository_id,omitempty" example:"550e8400-e29b-41d4-a716-446655440000"`
}

type IndexingRepositoryOptionDTO struct {
	ID   string `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Name string `json:"name" example:"Photos Library"`
	// Path is only populated for admin callers; repository filesystem
	// locations are never exposed to regular users.
	Path      string `json:"path,omitempty" example:"/Volumes/Media/Photos"`
	Role      string `json:"role" example:"regular"`
	IsPrimary bool   `json:"is_primary" example:"false"`
}

type IndexingRepositoryListResponseDTO struct {
	Repositories []IndexingRepositoryOptionDTO `json:"repositories"`
}

type AssetIndexingTaskStatsDTO struct {
	IndexedCount int `json:"indexed_count" example:"1200"`
	QueuedJobs   int `json:"queued_jobs" example:"12"`
	TotalCount   int `json:"total_count" example:"2400"`
}

type AssetIndexingTaskSetStatsDTO struct {
	Semantic AssetIndexingTaskStatsDTO `json:"semantic"`
	BioCLIP  AssetIndexingTaskStatsDTO `json:"bioclip"`
	OCR      AssetIndexingTaskStatsDTO `json:"ocr"`
	Face     AssetIndexingTaskStatsDTO `json:"face"`
}

type AssetIndexingStatsResponseDTO struct {
	PhotoTotal  int                          `json:"photo_total" example:"2400"`
	ReindexJobs int                          `json:"reindex_jobs" example:"1"`
	Tasks       AssetIndexingTaskSetStatsDTO `json:"tasks"`
}

// UploadResponseDTO represents the response structure for file upload
type UploadResponseDTO struct {
	TaskID      int64  `json:"task_id" example:"12345"`
	Status      string `json:"status" example:"processing"`
	FileName    string `json:"file_name" example:"photo.jpg"`
	Size        int64  `json:"size" example:"1048576"`
	ContentHash string `json:"content_hash" example:"abcd1234567890"`
	Message     string `json:"message" example:"File received and queued for processing"`
}

// BatchUploadResponseDTO represents the response structure for batch upload
type BatchUploadResponseDTO struct {
	Results []BatchUploadResultDTO `json:"results"`
}

// BatchUploadResultDTO represents a single result in a batch upload
type BatchUploadResultDTO struct {
	Success     bool    `json:"success"`
	FileName    string  `json:"file_name,omitempty"`
	ContentHash string  `json:"content_hash"`
	TaskID      *int64  `json:"task_id,omitempty"`
	Status      *string `json:"status,omitempty"`
	Size        *int64  `json:"size,omitempty"`
	Message     *string `json:"message,omitempty"`
	Error       *string `json:"error,omitempty"`
}

// UploadConfigResponseDTO represents the response structure for upload configuration
type UploadConfigResponseDTO struct {
	ChunkSize           int64 `json:"chunk_size"`
	MaxConcurrent       int   `json:"max_concurrent"`
	MemoryBuffer        int64 `json:"memory_buffer"`
	MergeConcurrency    int   `json:"merge_concurrency"`
	MaxInFlightRequests int   `json:"max_in_flight_requests"`
}

// SessionProgressDTO represents progress information for an upload session
type SessionProgressDTO struct {
	SessionID    string    `json:"session_id"`
	Filename     string    `json:"filename"`
	Status       string    `json:"status"`
	Progress     float64   `json:"progress"`
	Received     int       `json:"received_chunks"`
	Total        int       `json:"total_chunks"`
	BytesDone    int64     `json:"bytes_done"`
	BytesTotal   int64     `json:"bytes_total"`
	LastActivity time.Time `json:"last_activity"`
}

// ProgressSummaryDTO represents summary information for all upload sessions
type ProgressSummaryDTO struct {
	TotalSessions   int     `json:"total_sessions"`
	ActiveSessions  int     `json:"active_sessions"`
	CompletedFiles  int     `json:"completed_files"`
	FailedSessions  int     `json:"failed_sessions"`
	OverallProgress float64 `json:"overall_progress"`
}

// UploadProgressResponseDTO represents the response structure for upload progress
type UploadProgressResponseDTO struct {
	Sessions []SessionProgressDTO `json:"sessions"`
	Summary  ProgressSummaryDTO   `json:"summary"`
}

// AssetDTO represents an asset
type AssetDTO struct {
	AssetID              string                          `json:"asset_id"`
	OwnerID              *int32                          `json:"owner_id"`
	RepositoryID         *string                         `json:"repository_id,omitempty"`
	Type                 string                          `json:"type"`
	OriginalFilename     string                          `json:"original_filename"`
	StoragePath          string                          `json:"storage_path"`
	MimeType             string                          `json:"mime_type"`
	FileSize             int64                           `json:"file_size"`
	Hash                 *string                         `json:"hash"`
	Width                *int32                          `json:"width"`
	Height               *int32                          `json:"height"`
	Duration             *float64                        `json:"duration"`
	UploadTime           time.Time                       `json:"upload_time"`
	TakenTime            *time.Time                      `json:"taken_time,omitempty"`
	CaptureOffsetMinutes *int16                          `json:"capture_offset_minutes,omitempty"`
	Rating               *int32                          `json:"rating,omitempty"`
	Liked                *bool                           `json:"liked,omitempty"`
	IsDeleted            *bool                           `json:"is_deleted"`
	DeletedAt            *time.Time                      `json:"deleted_at,omitempty"`
	Metadata             dbtypes.SpecificMetadata        `json:"specific_metadata" swaggertype:"object" oneOf:"dbtypes.PhotoSpecificMetadata,dbtypes.VideoSpecificMetadata,dbtypes.AudioSpecificMetadata"`
	Status               []byte                          `json:"status"`
	SpeciesPredictions   []dbtypes.SpeciesPredictionMeta `json:"species_predictions,omitempty"`
	// Stack fields (populated when stack mode is enabled)
	Stack *StackPreviewDTO `json:"stack,omitempty"`
}

type AssetExifResponseDTO struct {
	AssetID string         `json:"asset_id"`
	ExifRaw map[string]any `json:"exif_raw" swaggertype:"object"`
}

type LumilioSidecarSourceDTO struct {
	OriginalFilename string  `json:"original_filename" example:"IMG_0001.jpg"`
	StoragePath      string  `json:"storage_path" example:"inbox/2026/05/IMG_0001.jpg"`
	MimeType         string  `json:"mime_type" example:"image/jpeg"`
	FileSize         int64   `json:"file_size" example:"1048576"`
	Hash             *string `json:"hash,omitempty" example:"abcd1234567890"`
	Width            *int32  `json:"width,omitempty" example:"6000"`
	Height           *int32  `json:"height,omitempty" example:"4000"`
}

type StudioEditCropDTO struct {
	X      float64 `json:"x" example:"0"`
	Y      float64 `json:"y" example:"0"`
	Width  float64 `json:"width" example:"1000"`
	Height float64 `json:"height" example:"800"`
}

type StudioEditAdjustmentsDTO struct {
	Exposure       float64            `json:"exposure" example:"0"`
	Contrast       float64            `json:"contrast" example:"0"`
	Highlights     float64            `json:"highlights" example:"0"`
	Shadows        float64            `json:"shadows" example:"0"`
	Whites         float64            `json:"whites" example:"0"`
	Blacks         float64            `json:"blacks" example:"0"`
	Temperature    float64            `json:"temperature" example:"0"`
	Tint           float64            `json:"tint" example:"0"`
	Vibrance       float64            `json:"vibrance" example:"0"`
	Saturation     float64            `json:"saturation" example:"0"`
	Clarity        float64            `json:"clarity" example:"0"`
	Sharpness      float64            `json:"sharpness" example:"0"`
	NoiseReduction float64            `json:"noiseReduction" example:"0"`
	Rotation       float64            `json:"rotation" example:"0"`
	FlipHorizontal bool               `json:"flipHorizontal" example:"false"`
	FlipVertical   bool               `json:"flipVertical" example:"false"`
	Crop           *StudioEditCropDTO `json:"crop,omitempty"`
}

type LumilioSidecarV1DTO struct {
	Version     int                      `json:"version" example:"1"`
	AssetID     string                   `json:"asset_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Source      LumilioSidecarSourceDTO  `json:"source"`
	Adjustments StudioEditAdjustmentsDTO `json:"adjustments"`
	UpdatedAt   time.Time                `json:"updated_at" example:"2026-05-26T10:00:00Z"`
}

type AssetSidecarResponseDTO struct {
	AssetID string              `json:"asset_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Exists  bool                `json:"exists" example:"true"`
	Sidecar LumilioSidecarV1DTO `json:"sidecar"`
}

type AssetGroupDTO struct {
	Key    string     `json:"key" example:"date:today"`
	Assets []AssetDTO `json:"assets"`
}

// ToAssetDTO converts a repo.Asset to AssetDTO
func ToAssetDTO(a repo.Asset) AssetDTO {
	var id string
	if a.AssetID.Valid {
		id = uuid.UUID(a.AssetID.Bytes).String()
	}
	var storagePath string
	if a.StoragePath != nil {
		storagePath = *a.StoragePath
	}
	var uploadTime time.Time
	if a.UploadTime.Valid {
		uploadTime = a.UploadTime.Time
	}
	var deletedAt *time.Time
	if a.DeletedAt.Valid {
		t := a.DeletedAt.Time
		deletedAt = &t
	}
	var repositoryID *string
	if a.RepositoryID.Valid {
		repoUUID := uuid.UUID(a.RepositoryID.Bytes).String()
		repositoryID = &repoUUID
	}
	var takenTime *time.Time
	if a.TakenTime.Valid {
		t := a.TakenTime.Time
		takenTime = &t
	}
	return AssetDTO{
		AssetID:              id,
		OwnerID:              a.OwnerID,
		RepositoryID:         repositoryID,
		Type:                 a.Type,
		OriginalFilename:     a.OriginalFilename,
		StoragePath:          storagePath,
		MimeType:             a.MimeType,
		FileSize:             a.FileSize,
		Hash:                 a.Hash,
		Width:                a.Width,
		Height:               a.Height,
		Duration:             a.Duration,
		UploadTime:           uploadTime,
		TakenTime:            takenTime,
		CaptureOffsetMinutes: a.CaptureOffsetMinutes,
		Rating:               a.Rating,
		Liked:                a.Liked,
		IsDeleted:            a.IsDeleted,
		DeletedAt:            deletedAt,
		Metadata:             a.SpecificMetadata,
		Status:               a.Status,
	}
}

// AssetThumbnailDTO mirrors one entry of the `thumbnails` aggregate built by
// GetAssetWithRelations.
type AssetThumbnailDTO struct {
	ThumbnailID string `json:"thumbnail_id"`
	Size        string `json:"size"`
	StoragePath string `json:"storage_path"`
	MimeType    string `json:"mime_type"`
}

// AssetTagDTO mirrors one entry of the `tags` aggregate built by
// GetAssetWithRelations.
type AssetTagDTO struct {
	TagID      int32    `json:"tag_id"`
	TagName    string   `json:"tag_name"`
	Confidence *float64 `json:"confidence,omitempty"`
	// Source identifies who created the tag link, e.g. "manual" for
	// user-added tags or "zeroshot" for AI-generated ones. Manual tags are
	// the only ones the UI lets the user remove.
	Source *string `json:"source,omitempty"`
}

// AddAssetTagRequestDTO is the body for adding a manual tag to an asset.
type AddAssetTagRequestDTO struct {
	TagName string `json:"tag_name" binding:"required" example:"vacation"`
}

// AssetTagsResponseDTO is the list of tags attached to an asset.
type AssetTagsResponseDTO struct {
	Tags []AssetTagDTO `json:"tags"`
}

// TagDTO is a tag definition used for autocomplete/suggestions.
type TagDTO struct {
	TagID    int32  `json:"tag_id"`
	TagName  string `json:"tag_name"`
	Category string `json:"category,omitempty"`
}

// TagListResponseDTO is a flat list of tag definitions.
type TagListResponseDTO struct {
	Tags []TagDTO `json:"tags"`
}

// TagSummaryDTO summarizes one tag's usage across the caller's accessible
// asset set. Distinct (tag_id, source) pairs are reported separately since a
// tag name can carry both manual and AI/system assignments.
type TagSummaryDTO struct {
	TagID        int32      `json:"tag_id" example:"42"`
	TagName      string     `json:"tag_name" example:"document"`
	Source       string     `json:"source,omitempty" example:"manual"`
	AssetCount   int64      `json:"asset_count" example:"37"`
	CoverAssetID *string    `json:"cover_asset_id,omitempty" example:"550e8400-e29b-41d4-a716-446655440000"`
	LastUsedAt   *time.Time `json:"last_used_at,omitempty"`
}

// TagSummaryListResponseDTO is a browsable, count/cover-enriched tag list,
// distinct from the autocomplete-oriented TagListResponseDTO.
type TagSummaryListResponseDTO struct {
	Tags []TagSummaryDTO `json:"tags"`
}

// FolderSummaryDTO summarizes assets grouped under one immediate child
// folder of the requested parent path. FolderPath and DisplayName are
// repository-relative; absolute host paths are never exposed here.
type FolderSummaryDTO struct {
	RepositoryID   string     `json:"repository_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	RepositoryName string     `json:"repository_name" example:"Primary Library"`
	FolderPath     string     `json:"folder_path" example:"inbox/2026/05"`
	DisplayName    string     `json:"display_name" example:"05"`
	Depth          int        `json:"depth" example:"3"`
	AssetCount     int64      `json:"asset_count" example:"128"`
	PhotoCount     int64      `json:"photo_count" example:"110"`
	VideoCount     int64      `json:"video_count" example:"18"`
	AudioCount     int64      `json:"audio_count" example:"0"`
	DateStart      *time.Time `json:"date_start,omitempty"`
	DateEnd        *time.Time `json:"date_end,omitempty"`
	CoverAssetID   *string    `json:"cover_asset_id,omitempty" example:"550e8400-e29b-41d4-a716-446655440000"`
}

// FolderListResponseDTO is a browsable folder listing scoped to one parent path.
type FolderListResponseDTO struct {
	Folders    []FolderSummaryDTO `json:"folders"`
	ParentPath string             `json:"parent_path"`
}

// AssetAlbumRefDTO mirrors one entry of the `albums` aggregate built by
// GetAssetWithRelations.
type AssetAlbumRefDTO struct {
	AlbumID   int32      `json:"album_id"`
	AlbumName string     `json:"album_name"`
	Position  *int32     `json:"position,omitempty"`
	AddedTime *time.Time `json:"added_time,omitempty"`
}

// AssetOCRTextItemDTO mirrors one OCR text item produced by GetAssetWithRelations.
// BoundingBox is freeform jsonb geometry and is left untyped.
type AssetOCRTextItemDTO struct {
	ID          int64           `json:"id"`
	TextContent string          `json:"text_content"`
	Confidence  *float64        `json:"confidence,omitempty"`
	BoundingBox json.RawMessage `json:"bounding_box,omitempty" swaggertype:"object"`
	TextLength  *int32          `json:"text_length,omitempty"`
	AreaPixels  *float64        `json:"area_pixels,omitempty"`
}

// AssetOCRResultDTO mirrors the `ocr_result` object produced by
// GetAssetWithRelations when include_ocr is requested.
type AssetOCRResultDTO struct {
	ModelID          string                `json:"model_id"`
	TotalCount       *int32                `json:"total_count,omitempty"`
	ProcessingTimeMs *int32                `json:"processing_time_ms,omitempty"`
	CreatedAt        *time.Time            `json:"created_at,omitempty"`
	UpdatedAt        *time.Time            `json:"updated_at,omitempty"`
	TextItems        []AssetOCRTextItemDTO `json:"text_items"`
}

// AssetFaceItemDTO mirrors one detected face produced by GetAssetWithRelations.
// BoundingBox and Expression are freeform jsonb and are left untyped.
type AssetFaceItemDTO struct {
	ID          int64           `json:"id"`
	FaceID      *string         `json:"face_id,omitempty"`
	BoundingBox json.RawMessage `json:"bounding_box,omitempty" swaggertype:"object"`
	Confidence  *float64        `json:"confidence,omitempty"`
	AgeGroup    *string         `json:"age_group,omitempty"`
	Gender      *string         `json:"gender,omitempty"`
	Ethnicity   *string         `json:"ethnicity,omitempty"`
	Expression  *string         `json:"expression,omitempty"`
	IsPrimary   *bool           `json:"is_primary,omitempty"`
	ClusterID   *int32          `json:"cluster_id,omitempty"`
	ClusterName *string         `json:"cluster_name,omitempty"`
}

// AssetFaceResultDTO mirrors the `face_result` object produced by
// GetAssetWithRelations when include_faces is requested.
type AssetFaceResultDTO struct {
	ModelID          string             `json:"model_id"`
	TotalFaces       *int32             `json:"total_faces,omitempty"`
	ProcessingTimeMs *int32             `json:"processing_time_ms,omitempty"`
	CreatedAt        *time.Time         `json:"created_at,omitempty"`
	UpdatedAt        *time.Time         `json:"updated_at,omitempty"`
	Faces            []AssetFaceItemDTO `json:"faces"`
}

// AssetDetailDTO is the typed response for GET /assets/{id}. It embeds the base
// AssetDTO and exposes the optional relations populated by the include_* query
// flags. Replaces the previous untyped map[string]interface{} response so the
// contract is honest and frontend access is type-safe.
type AssetDetailDTO struct {
	AssetDTO
	Thumbnails []AssetThumbnailDTO `json:"thumbnails,omitempty"`
	Tags       []AssetTagDTO       `json:"tags,omitempty"`
	Albums     []AssetAlbumRefDTO  `json:"albums,omitempty"`
	OcrResult  *AssetOCRResultDTO  `json:"ocr_result,omitempty"`
	FaceResult *AssetFaceResultDTO `json:"face_result,omitempty"`
}

// AssetDetailIncludes controls which optional relations ToAssetDetailDTO emits.
type AssetDetailIncludes struct {
	Thumbnails bool
	Tags       bool
	Albums     bool
	Species    bool
	OCR        bool
	Faces      bool
}

// ToAssetDetailDTO converts a GetAssetWithRelations row into a typed
// AssetDetailDTO, honoring the requested includes. Aggregate columns arrive as
// raw JSON ([]byte); malformed or empty blobs degrade to nil rather than erroring.
func ToAssetDetailDTO(r repo.GetAssetWithRelationsRow, inc AssetDetailIncludes) AssetDetailDTO {
	var id string
	if r.AssetID.Valid {
		id = uuid.UUID(r.AssetID.Bytes).String()
	}
	var storagePath string
	if r.StoragePath != nil {
		storagePath = *r.StoragePath
	}
	var uploadTime time.Time
	if r.UploadTime.Valid {
		uploadTime = r.UploadTime.Time
	}
	var takenTime *time.Time
	if r.TakenTime.Valid {
		t := r.TakenTime.Time
		takenTime = &t
	}
	var deletedAt *time.Time
	if r.DeletedAt.Valid {
		t := r.DeletedAt.Time
		deletedAt = &t
	}
	var repositoryID *string
	if r.RepositoryID.Valid {
		repoUUID := uuid.UUID(r.RepositoryID.Bytes).String()
		repositoryID = &repoUUID
	}

	base := AssetDTO{
		AssetID:              id,
		OwnerID:              r.OwnerID,
		RepositoryID:         repositoryID,
		Type:                 r.Type,
		OriginalFilename:     r.OriginalFilename,
		StoragePath:          storagePath,
		MimeType:             r.MimeType,
		FileSize:             r.FileSize,
		Hash:                 r.Hash,
		Width:                r.Width,
		Height:               r.Height,
		Duration:             r.Duration,
		UploadTime:           uploadTime,
		TakenTime:            takenTime,
		CaptureOffsetMinutes: r.CaptureOffsetMinutes,
		Rating:               r.Rating,
		Liked:                r.Liked,
		IsDeleted:            r.IsDeleted,
		DeletedAt:            deletedAt,
		Metadata:             r.SpecificMetadata,
		Status:               r.Status,
	}

	if inc.Species && len(r.SpeciesPredictions) > 0 {
		var preds []dbtypes.SpeciesPredictionMeta
		if err := json.Unmarshal(r.SpeciesPredictions, &preds); err == nil {
			base.SpeciesPredictions = preds
		}
	}

	detail := AssetDetailDTO{AssetDTO: base}

	if inc.Thumbnails && len(r.Thumbnails) > 0 {
		var thumbs []AssetThumbnailDTO
		if err := json.Unmarshal(r.Thumbnails, &thumbs); err == nil {
			detail.Thumbnails = thumbs
		}
	}
	if inc.Tags && len(r.Tags) > 0 {
		var tags []AssetTagDTO
		if err := json.Unmarshal(r.Tags, &tags); err == nil {
			detail.Tags = tags
		}
	}
	if inc.Albums && len(r.Albums) > 0 {
		var albums []AssetAlbumRefDTO
		if err := json.Unmarshal(r.Albums, &albums); err == nil {
			detail.Albums = albums
		}
	}
	if inc.OCR && len(r.OcrResult) > 0 {
		var ocr AssetOCRResultDTO
		if err := json.Unmarshal(r.OcrResult, &ocr); err == nil {
			detail.OcrResult = &ocr
		}
	}
	if inc.Faces && len(r.FaceResult) > 0 {
		var face AssetFaceResultDTO
		if err := json.Unmarshal(r.FaceResult, &face); err == nil {
			detail.FaceResult = &face
		}
	}

	return detail
}

// AssetListResponseDTO represents the response structure for asset listing
type AssetListResponseDTO struct {
	Assets []AssetDTO `json:"assets"`
	Total  *int       `json:"total,omitempty" example:"150"`
	Limit  int        `json:"limit" example:"20"`
	Offset int        `json:"offset" example:"0"`
}

type QueryAssetsResponseDTO struct {
	Items        []BrowseItemDTO `json:"items,omitempty"`
	TotalVisible *int            `json:"total_visible,omitempty" example:"120"`
	TotalAssets  *int            `json:"total_assets,omitempty" example:"150"`
	StackMode    string          `json:"stack_mode,omitempty" example:"collapsed" enums:"collapsed,expanded"`
	Limit        int             `json:"limit" example:"20"`
	Offset       int             `json:"offset" example:"0"`
}

// SearchAssetsResponseDTO represents the response structure for searching assets
type SearchAssetsResponseDTO struct {
	TopItems            []BrowseItemDTO         `json:"top_items,omitempty"`
	TopResultsMeta      SearchTopResultsMetaDTO `json:"top_results_meta"`
	ResultItems         []BrowseItemDTO         `json:"result_items,omitempty"`
	ResultsTotalVisible *int                    `json:"results_total_visible,omitempty" example:"120"`
	ResultsTotalAssets  *int                    `json:"results_total_assets,omitempty" example:"150"`
	StackMode           string                  `json:"stack_mode,omitempty" example:"collapsed" enums:"collapsed,expanded"`
	Limit               int                     `json:"limit" example:"20"`
	Offset              int                     `json:"offset" example:"0"`
}

// DownloadAssetsRequestDTO represents a bulk original-file download request.
type DownloadAssetsRequestDTO struct {
	AssetIDs []string `json:"asset_ids" binding:"required" example:"550e8400-e29b-41d4-a716-446655440000,550e8400-e29b-41d4-a716-446655440001"`
}

// FeaturedAssetsResponseDTO represents curated featured photos for home/gallery use.
type FeaturedAssetsResponseDTO struct {
	Assets          []AssetDTO `json:"assets"`
	Count           int        `json:"count" example:"8"`
	CandidateCount  int        `json:"candidate_count" example:"240"`
	Seed            string     `json:"seed" example:"2026-02-10"`
	Strategy        string     `json:"strategy" example:"weighted_aes_v1"`
	GeneratedAtTime time.Time  `json:"generated_at_time" example:"2026-02-10T12:00:00Z"`
}

// AssetMapPointDTO represents a lightweight map point for photo location rendering.
type AssetMapPointDTO struct {
	AssetID          string     `json:"asset_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	OriginalFilename string     `json:"original_filename" example:"IMG_1234.HEIC"`
	UploadTime       time.Time  `json:"upload_time" example:"2026-02-10T12:00:00Z"`
	TakenTime        *time.Time `json:"taken_time,omitempty" example:"2026-02-09T08:30:00Z"`
	GPSLatitude      float64    `json:"gps_latitude" example:"37.7749"`
	GPSLongitude     float64    `json:"gps_longitude" example:"-122.4194"`
}

// AssetMapPointListResponseDTO represents paginated lightweight photo map points.
type AssetMapPointListResponseDTO struct {
	Points []AssetMapPointDTO `json:"points"`
	Total  *int               `json:"total,omitempty" example:"1500"`
	Limit  int                `json:"limit" example:"1000"`
	Offset int                `json:"offset" example:"0"`
}

// UpdateAssetRequestDTO represents the request structure for updating asset metadata
type UpdateAssetRequestDTO struct {
	Metadata dbtypes.SpecificMetadata `json:"specific_metadata" swaggertype:"object" oneOf:"dbtypes.PhotoSpecificMetadata,dbtypes.VideoSpecificMetadata,dbtypes.AudioSpecificMetadata"`
}

// UpdateRatingRequestDTO represents the request structure for updating asset rating
type UpdateRatingRequestDTO struct {
	Rating int `json:"rating" example:"5" validate:"min=0,max=5"`
}

// UpdateLikeRequestDTO represents the request structure for updating asset like status
type UpdateLikeRequestDTO struct {
	Liked bool `json:"liked" example:"true"`
}

// UpdateRatingAndLikeRequestDTO represents the request structure for updating both rating and like status
type UpdateRatingAndLikeRequestDTO struct {
	Rating int  `json:"rating" example:"5" validate:"min=0,max=5"`
	Liked  bool `json:"liked" example:"true"`
}

// UpdateDescriptionRequestDTO represents the request structure for updating asset description
type UpdateDescriptionRequestDTO struct {
	Description string `json:"description" example:"A beautiful sunset photo"`
}

// MessageResponseDTO represents a simple message response
type MessageResponseDTO struct {
	Message string `json:"message" example:"Operation completed successfully"`
}

// AssetTypesResponseDTO represents the response structure for asset types
type AssetTypesResponseDTO struct {
	Types []dbtypes.AssetType `json:"types"`
}

// FilenameFilterDTO represents filename filtering options
type FilenameFilterDTO struct {
	Value    string `json:"value" example:"IMG_"`
	Operator string `json:"operator" example:"starts_with" enums:"contains,matches,starts_with,ends_with"`
}

// UnmarshalJSON accepts the current operator field and the legacy mode field.
func (f *FilenameFilterDTO) UnmarshalJSON(data []byte) error {
	type alias struct {
		Value    string `json:"value"`
		Operator string `json:"operator"`
		Mode     string `json:"mode"`
	}

	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}

	f.Value = a.Value
	f.Operator = a.Operator
	if f.Operator == "" {
		f.Operator = a.Mode
	}
	return nil
}

// DateRangeDTO represents a date range filter
type DateRangeDTO struct {
	From         *time.Time `json:"from,omitempty"`
	To           *time.Time `json:"to,omitempty"`
	FromDateOnly bool       `json:"-"`
	ToDateOnly   bool       `json:"-"`
}

// UnmarshalJSON supports parsing both date-only (YYYY-MM-DD) and RFC3339 timestamps.
func (d *DateRangeDTO) UnmarshalJSON(data []byte) error {
	type alias struct {
		From *string `json:"from"`
		To   *string `json:"to"`
	}

	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}

	parse := func(val *string) (*time.Time, bool, error) {
		if val == nil || *val == "" {
			return nil, false, nil
		}
		layouts := []string{
			"2006-01-02",
			time.RFC3339,
			"2006-01-02T15:04:05Z07:00",
		}
		for _, layout := range layouts {
			if t, err := time.Parse(layout, *val); err == nil {
				return &t, layout == "2006-01-02", nil
			}
		}
		return nil, false, fmt.Errorf("invalid date format: %s", *val)
	}

	var err error
	if d.From, d.FromDateOnly, err = parse(a.From); err != nil {
		return err
	}
	if d.To, d.ToDateOnly, err = parse(a.To); err != nil {
		return err
	}
	return nil
}

// LocationBBoxDTO represents a GPS bounding-box filter.
type LocationBBoxDTO struct {
	North float64 `json:"north" example:"37.9"`
	South float64 `json:"south" example:"37.7"`
	East  float64 `json:"east" example:"-122.3"`
	West  float64 `json:"west" example:"-122.5"`
}

// AssetFilterDTO represents comprehensive filtering options
type AssetFilterDTO struct {
	RepositoryID *string            `json:"repository_id,omitempty" example:"550e8400-e29b-41d4-a716-446655440000"`
	AlbumID      *int               `json:"album_id,omitempty" example:"123"`
	Type         *string            `json:"type,omitempty" example:"PHOTO" enums:"PHOTO,VIDEO,AUDIO"`
	Types        []string           `json:"types,omitempty" example:"PHOTO,VIDEO"` // Multiple asset types
	OwnerID      *int32             `json:"owner_id,omitempty" example:"123"`
	RAW          *bool              `json:"raw,omitempty" example:"true"`
	Rating       *int               `json:"rating,omitempty" example:"5" minimum:"0" maximum:"5"`
	Liked        *bool              `json:"liked,omitempty" example:"true"`
	Filename     *FilenameFilterDTO `json:"filename,omitempty"`
	Date         *DateRangeDTO      `json:"date,omitempty"`
	IsDeleted    *bool              `json:"is_deleted,omitempty" example:"false"`
	CameraModel  *string            `json:"camera_model,omitempty" example:"Canon EOS R5"`
	Lens         *string            `json:"lens,omitempty" example:"EF 50mm f/1.8"`
	Location     *LocationBBoxDTO   `json:"location,omitempty"`
	TagName      *string            `json:"tag_name,omitempty" example:"document"`
	TagSource    *string            `json:"tag_source,omitempty" example:"zeroshot"`
	TagNames     []string           `json:"tag_names,omitempty"`
	PersonID     *int32             `json:"person_id,omitempty" example:"42"`
	FolderPath   *string            `json:"folder_path,omitempty" example:"inbox/2026/05"`
	// FolderRecursive controls whether FolderPath matches descendants (default true) or direct contents only.
	FolderRecursive *bool `json:"folder_recursive,omitempty" example:"true"`
}

// FilterAssetsRequestDTO represents the request structure for filtering assets
type FilterAssetsRequestDTO struct {
	Filter AssetFilterDTO `json:"filter"`
	Limit  int            `json:"limit" example:"20" minimum:"1" maximum:"100"`
	Offset int            `json:"offset" example:"0" minimum:"0"`
}

// SearchAssetsRequestDTO represents the request structure for searching assets
type SearchAssetsRequestDTO struct {
	Query           string         `json:"query,omitempty" example:"red bird on branch"`
	Filter          AssetFilterDTO `json:"filter,omitempty"`
	SortBy          string         `json:"sort_by,omitempty" example:"date_captured" enums:"recently_added,date_captured"`
	ViewerTimezone  string         `json:"viewer_timezone,omitempty" example:"America/New_York"`
	Pagination      PaginationDTO  `json:"pagination"`
	EnhancementMode string         `json:"enhancement_mode,omitempty" example:"auto" enums:"auto,off,only"`
	TopResultsLimit int            `json:"top_results_limit,omitempty" example:"200" minimum:"1" maximum:"200"`
	StackMode       string         `json:"stack_mode,omitempty" example:"collapsed" enums:"collapsed,expanded"`
	Debug           bool           `json:"debug,omitempty"`
}

type SearchTopResultsMetaDTO struct {
	Enabled           bool                  `json:"enabled"`
	Degraded          bool                  `json:"degraded"`
	Reason            string                `json:"reason,omitempty" example:"runtime_unavailable"`
	SourceTypes       []string              `json:"source_types" example:"embedding,ocr,place"`
	CandidateCount    int                   `json:"candidate_count,omitempty"`
	CandidatePoolSize int                   `json:"candidate_pool_size,omitempty"`
	Sources           []SearchSourceMetaDTO `json:"sources,omitempty"`
	Debug             []SearchDebugItemDTO  `json:"debug,omitempty"`
}

type SearchSourceMetaDTO struct {
	Type           string  `json:"type"`
	Weight         float64 `json:"weight"`
	CandidateCount int     `json:"candidate_count"`
	DurationMs     int64   `json:"duration_ms"`
	Error          string  `json:"error,omitempty"`
}

type SearchDebugContributionDTO struct {
	Rank     int     `json:"rank"`
	Weight   float64 `json:"weight"`
	RRFScore float64 `json:"rrf_score"`
	RawScore float64 `json:"raw_score"`
}

type SearchDebugItemDTO struct {
	AssetID       string                                `json:"asset_id"`
	Score         float64                               `json:"score"`
	Contributions map[string]SearchDebugContributionDTO `json:"contributions"`
}

// OptionsResponseDTO represents the response for filter options
type OptionsResponseDTO struct {
	CameraModels []string `json:"camera_models"`
	Lenses       []string `json:"lenses"`
}

// BulkLikeUpdateDTO represents the result of a bulk like/unlike operation
// This DTO is returned by the bulk_like_assets tool and contains the summary
// of the operation, including success/failure counts and affected asset IDs.
type BulkLikeUpdateDTO struct {
	// Total number of assets in the batch
	Total int `json:"total" example:"100" minimum:"0"`

	// Number of successfully updated assets
	Success int `json:"success" example:"98" minimum:"0"`

	// Number of failed updates
	Failed int `json:"failed" example:"2" minimum:"0"`

	// List of asset IDs that failed to update (only present when Failed > 0)
	FailedAssetIDs []string `json:"failed_asset_ids,omitempty" example:"550e8400-e29b-41d4-a716-446655440000,660e8400-e29b-41d4-a716-446655440001"`

	// The like status that was applied (true = liked, false = unliked)
	Liked bool `json:"liked" example:"true"`

	// Action performed: "like" or "unlike"
	Action string `json:"action" example:"like" enums:"like,unlike"`

	// Reference ID for this result (can be used in subsequent tool calls)
	RefID string `json:"ref_id,omitempty" example:"ref_1234567890"`

	// Human-readable description
	Description string `json:"description" example:"Bulk like: 98/100 successful"`

	// Timestamp when the operation completed
	Timestamp string `json:"timestamp" example:"2026-01-18T19:34:00Z"`
}

// PaginationDTO represents pagination options for list queries
type PaginationDTO struct {
	Limit  int `json:"limit" example:"20" minimum:"1" maximum:"100"`
	Offset int `json:"offset" example:"0" minimum:"0"`
}

// AssetQueryRequestDTO is the unified request for listing/searching/filtering assets
// This replaces the separate ListAssets, FilterAssets, and SearchAssets endpoints
type AssetQueryRequestDTO struct {
	Query          string         `json:"query,omitempty" example:"sunset photo"`                             // Search keyword (optional)
	SearchType     string         `json:"search_type,omitempty" example:"filename" enums:"filename,semantic"` // "filename" (default) | "semantic"
	Filter         AssetFilterDTO `json:"filter,omitempty"`                                                   // Unified filter options
	SortBy         string         `json:"sort_by,omitempty" example:"date_captured" enums:"recently_added,date_captured"`
	ViewerTimezone string         `json:"viewer_timezone,omitempty" example:"America/New_York"`
	StackMode      string         `json:"stack_mode,omitempty" example:"collapsed" enums:"collapsed,expanded"`
	Pagination     PaginationDTO  `json:"pagination"` // limit, offset
}

type BrowseStackDTO struct {
	StackID          string   `json:"stack_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	StackKind        string   `json:"stack_kind,omitempty" example:"live_photo" enums:"raw_jpeg,live_photo,manual"`
	CoverAssetID     string   `json:"cover_asset_id" example:"550e8400-e29b-41d4-a716-446655440001"`
	CoverAsset       AssetDTO `json:"cover_asset"`
	StackSize        int      `json:"stack_size" example:"3"`
	MemberAssetIDs   []string `json:"member_asset_ids"`
	MatchedMemberIDs []string `json:"matched_member_ids,omitempty"`
}

type BrowseItemDTO struct {
	Type  string          `json:"type" example:"stack" enums:"asset,stack"`
	ID    string          `json:"id" example:"stack:550e8400-e29b-41d4-a716-446655440000"`
	Asset *AssetDTO       `json:"asset,omitempty"`
	Stack *BrowseStackDTO `json:"stack,omitempty"`
}
