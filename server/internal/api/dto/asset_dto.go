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
	AssetID          string                   `json:"asset_id"`
	OwnerID          *int32                   `json:"owner_id"`
	RepositoryID     *string                  `json:"repository_id,omitempty"`
	Type             string                   `json:"type"`
	OriginalFilename string                   `json:"original_filename"`
	StoragePath      string                   `json:"storage_path"`
	MimeType         string                   `json:"mime_type"`
	FileSize         int64                    `json:"file_size"`
	Hash             *string                  `json:"hash"`
	Width            *int32                   `json:"width"`
	Height           *int32                   `json:"height"`
	Duration         *float64                 `json:"duration"`
	UploadTime       time.Time                `json:"upload_time"`
	TakenTime        *time.Time               `json:"taken_time,omitempty"`
	Rating           *int32                   `json:"rating,omitempty"`
	Liked            *bool                    `json:"liked,omitempty"`
	IsDeleted        *bool                    `json:"is_deleted"`
	DeletedAt        *time.Time               `json:"deleted_at,omitempty"`
	Metadata         dbtypes.SpecificMetadata `json:"specific_metadata" swaggertype:"object" oneOf:"dbtypes.PhotoSpecificMetadata,dbtypes.VideoSpecificMetadata,dbtypes.AudioSpecificMetadata"`
	Status           []byte                   `json:"status"`
}

// ToAssetDTO converts a repo.Asset to AssetDTO
func ToAssetDTO(a repo.Asset) AssetDTO {
	var id string
	if a.AssetID.Valid {
		id = uuid.UUID(a.AssetID.Bytes).String()
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
		AssetID:          id,
		OwnerID:          a.OwnerID,
		RepositoryID:     repositoryID,
		Type:             a.Type,
		OriginalFilename: a.OriginalFilename,
		StoragePath:      *a.StoragePath,
		MimeType:         a.MimeType,
		FileSize:         a.FileSize,
		Hash:             a.Hash,
		Width:            a.Width,
		Height:           a.Height,
		Duration:         a.Duration,
		UploadTime:       uploadTime,
		TakenTime:        takenTime,
		Rating:           a.Rating,
		Liked:            a.Liked,
		IsDeleted:        a.IsDeleted,
		DeletedAt:        deletedAt,
		Metadata:         a.SpecificMetadata,
		Status:           a.Status,
	}
}

// AssetListResponseDTO represents the response structure for asset listing
type AssetListResponseDTO struct {
	Assets []AssetDTO `json:"assets"`
	Limit  int        `json:"limit" example:"20"`
	Offset int        `json:"offset" example:"0"`
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
	Value string `json:"value" example:"IMG_"`
	Mode  string `json:"mode" example:"startswith" enums:"contains,matches,startswith,endswith"`
}

// DateRangeDTO represents a date range filter
type DateRangeDTO struct {
	From *time.Time `json:"from,omitempty"`
	To   *time.Time `json:"to,omitempty"`
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

	parse := func(val *string) (*time.Time, error) {
		if val == nil || *val == "" {
			return nil, nil
		}
		layouts := []string{
			"2006-01-02",
			time.RFC3339,
			"2006-01-02T15:04:05Z07:00",
		}
		for _, layout := range layouts {
			if t, err := time.Parse(layout, *val); err == nil {
				return &t, nil
			}
		}
		return nil, fmt.Errorf("invalid date format: %s", *val)
	}

	var err error
	if d.From, err = parse(a.From); err != nil {
		return err
	}
	if d.To, err = parse(a.To); err != nil {
		return err
	}
	return nil
}

// AssetFilterDTO represents comprehensive filtering options
type AssetFilterDTO struct {
	RepositoryID *string            `json:"repository_id,omitempty" example:"550e8400-e29b-41d4-a716-446655440000"`
	Type         *string            `json:"type,omitempty" example:"PHOTO" enums:"PHOTO,VIDEO,AUDIO"`
	OwnerID      *int32             `json:"owner_id,omitempty" example:"123"`
	RAW          *bool              `json:"raw,omitempty" example:"true"`
	Rating       *int               `json:"rating,omitempty" example:"5" minimum:"0" maximum:"5"`
	Liked        *bool              `json:"liked,omitempty" example:"true"`
	Filename     *FilenameFilterDTO `json:"filename,omitempty"`
	Date         *DateRangeDTO      `json:"date,omitempty"`
	CameraMake   *string            `json:"camera_make,omitempty" example:"Canon"`
	Lens         *string            `json:"lens,omitempty" example:"EF 50mm f/1.8"`
}

// FilterAssetsRequestDTO represents the request structure for filtering assets
type FilterAssetsRequestDTO struct {
	Filter AssetFilterDTO `json:"filter"`
	Limit  int            `json:"limit" example:"20" minimum:"1" maximum:"100"`
	Offset int            `json:"offset" example:"0" minimum:"0"`
}

// SearchAssetsRequestDTO represents the request structure for searching assets
type SearchAssetsRequestDTO struct {
	Query      string         `json:"query" binding:"required" example:"red bird on branch"`
	SearchType string         `json:"search_type" binding:"required" example:"filename" enums:"filename,semantic"`
	Filter     AssetFilterDTO `json:"filter,omitempty"`
	Limit      int            `json:"limit" example:"20" minimum:"1" maximum:"100"`
	Offset     int            `json:"offset" example:"0" minimum:"0"`
}

// OptionsResponseDTO represents the response for filter options
type OptionsResponseDTO struct {
	CameraMakes []string `json:"camera_makes"`
	Lenses      []string `json:"lenses"`
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
