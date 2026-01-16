package jobs

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/riverqueue/river"

	"server/internal/db/dbtypes"
)

// ProcessClipArgs is the River job payload for CLIP embedding/classification.
// Duplicated here (instead of importing processors) to avoid import cycles.
// Keep this in sync with processors.CLIPPayload.
type ProcessClipArgs struct {
	AssetID   pgtype.UUID `json:"assetId"`
	ImageData []byte      `json:"imageData"`
}

func (ProcessClipArgs) Kind() string { return "process_clip" }

// AssetRetryPayload is the River job payload for selective retry of asset processing tasks
type AssetRetryPayload struct {
	AssetID        string   `json:"assetId" river:"unique"`
	RetryTasks     []string `json:"retryTasks,omitempty"` // Empty means retry all failed tasks
	ForceFullRetry bool     `json:"forceFullRetry,omitempty"`
}

func (AssetRetryPayload) Kind() string { return "retry_asset" }

// InsertOpts implements JobArgsWithInsertOpts. Uniqueness is disabled to allow
// multiple retry jobs per asset; processors must handle any dedupe logic.
func (AssetRetryPayload) InsertOpts() river.InsertOpts {
	// Uniqueness biased by the time period, to avoid duplicate retry jobs in quick succession.
	return river.InsertOpts{UniqueOpts: river.UniqueOpts{
		ByPeriod: 1 * time.Minute,
	}}
}

// ProcessOcrArgs is the River job payload for OCR text extraction.
// Duplicated here (instead of importing processors) to avoid import cycles.
type ProcessOcrArgs struct {
	AssetID   pgtype.UUID `json:"assetId"`
	ImageData []byte      `json:"imageData"`
}

func (ProcessOcrArgs) Kind() string { return "process_ocr" }

// ProcessCaptionArgs is the River job payload for AI image captioning.
// Duplicated here (instead of importing processors) to avoid import cycles.
type ProcessCaptionArgs struct {
	AssetID      pgtype.UUID `json:"assetId"`
	ImageData    []byte      `json:"imageData"`
	CustomPrompt string      `json:"customPrompt,omitempty"`
}

func (ProcessCaptionArgs) Kind() string { return "process_caption" }

// ProcessFaceArgs is the River job payload for face detection and recognition.
// Duplicated here (instead of importing processors) to avoid import cycles.
type ProcessFaceArgs struct {
	AssetID   pgtype.UUID `json:"assetId"`
	ImageData []byte      `json:"imageData"`
}

func (ProcessFaceArgs) Kind() string { return "process_face" }

// IngestAssetArgs handles initial staging ingestion and asset creation.
type IngestAssetArgs struct {
	ClientHash   string    `json:"clientHash" river:"unique"`
	StagedPath   string    `json:"stagedPath"`
	UserID       string    `json:"userId" river:"unique"`
	Timestamp    time.Time `json:"timestamp"`
	ContentType  string    `json:"contentType,omitempty"`
	FileName     string    `json:"fileName,omitempty"`
	RepositoryID string    `json:"repositoryId,omitempty"`
}

func (IngestAssetArgs) Kind() string { return "ingest_asset" }

// MetadataArgs triggers EXIF/ffprobe metadata extraction per asset.
type MetadataArgs struct {
	AssetID          pgtype.UUID       `json:"assetId"`
	RepoPath         string            `json:"repoPath"`
	StoragePath      string            `json:"storagePath"`
	AssetType        dbtypes.AssetType `json:"assetType"`
	OriginalFilename string            `json:"originalFilename,omitempty"`
	FileSize         int64             `json:"fileSize,omitempty"`
	MimeType         string            `json:"mimeType,omitempty"`
}

func (MetadataArgs) Kind() string { return "metadata_asset" }

// ThumbnailArgs triggers thumbnail generation per asset.
type ThumbnailArgs struct {
	AssetID     pgtype.UUID       `json:"assetId"`
	RepoPath    string            `json:"repoPath"`
	StoragePath string            `json:"storagePath"`
	AssetType   dbtypes.AssetType `json:"assetType"`
}

func (ThumbnailArgs) Kind() string { return "thumbnail_asset" }

// TranscodeArgs triggers audio/video transcoding per asset.
type TranscodeArgs struct {
	AssetID     pgtype.UUID       `json:"assetId"`
	RepoPath    string            `json:"repoPath"`
	StoragePath string            `json:"storagePath"`
	AssetType   dbtypes.AssetType `json:"assetType"`
}

func (TranscodeArgs) Kind() string { return "transcode_asset" }
