package jobs

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/riverqueue/river"

	"server/internal/db/dbtypes"
)

// ProcessSemanticArgs is the River job payload for semantic embedding/classification.
// Duplicated here (instead of importing processors) to avoid import cycles.
// Keep this in sync with processors.SemanticPayload.
type ProcessSemanticArgs struct {
	AssetID           pgtype.UUID `json:"assetId"`
	PreprocessVersion string      `json:"preprocessVersion,omitempty"`
}

func (ProcessSemanticArgs) Kind() string { return "process_semantic" }

func (ProcessSemanticArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		MaxAttempts: MLProcessMaxAttempts,
		// Dedupe concurrent reindex/retry fan-out per asset: an equivalent job
		// still available/running/completed in the table is silently skipped
		// (Insert returns UniqueSkippedAsDuplicate=true, nil error). Default
		// ByState includes completed, so overlapping full-rebuild chains collapse
		// to one job per asset instead of racing the non-transactional OCR/face
		// save paths. ByArgs also keys on PreprocessVersion, so bumping the
		// version re-allows a re-run.
		UniqueOpts: river.UniqueOpts{
			ByArgs:   true,
			ByPeriod: MLProcessUniquePeriod,
		},
	}
}

const (
	MLPreprocessVersionV1 = "ml-image-v1"
	MLProcessMaxAttempts  = 8
	MLProcessUniquePeriod = 5 * time.Minute
	LocalToolMaxAttempts  = 5
)

// ZeroshotClassifyArgs is the River job payload for zero-shot
// classification. It scores the asset's already-stored semantic image embedding
// against classifier prototypes; it does not re-run any ML model.
type ZeroshotClassifyArgs struct {
	AssetID pgtype.UUID `json:"assetId"`
}

func (ZeroshotClassifyArgs) Kind() string { return "classify_zeroshot" }

func (ZeroshotClassifyArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		MaxAttempts: MLProcessMaxAttempts,
		// Dedupe concurrent reindex/retry fan-out per asset: an equivalent job
		// still available/running/completed in the table is silently skipped
		// (Insert returns UniqueSkippedAsDuplicate=true, nil error). Default
		// ByState includes completed, so overlapping full-rebuild chains collapse
		// to one job per asset instead of racing the non-transactional OCR/face
		// save paths. ByArgs also keys on PreprocessVersion, so bumping the
		// version re-allows a re-run.
		UniqueOpts: river.UniqueOpts{
			ByArgs:   true,
			ByPeriod: MLProcessUniquePeriod,
		},
	}
}

// ProcessBioClipArgs is the River job payload for BioCLIP classification.
// Duplicated here (instead of importing processors) to avoid import cycles.
type ProcessBioClipArgs struct {
	AssetID           pgtype.UUID `json:"assetId"`
	PreprocessVersion string      `json:"preprocessVersion,omitempty"`
}

func (ProcessBioClipArgs) Kind() string { return "process_bioclip" }

func (ProcessBioClipArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		MaxAttempts: MLProcessMaxAttempts,
		// Dedupe concurrent reindex/retry fan-out per asset: an equivalent job
		// still available/running/completed in the table is silently skipped
		// (Insert returns UniqueSkippedAsDuplicate=true, nil error). Default
		// ByState includes completed, so overlapping full-rebuild chains collapse
		// to one job per asset instead of racing the non-transactional OCR/face
		// save paths. ByArgs also keys on PreprocessVersion, so bumping the
		// version re-allows a re-run.
		UniqueOpts: river.UniqueOpts{
			ByArgs:   true,
			ByPeriod: MLProcessUniquePeriod,
		},
	}
}

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
	AssetID           pgtype.UUID `json:"assetId"`
	PreprocessVersion string      `json:"preprocessVersion,omitempty"`
}

func (ProcessOcrArgs) Kind() string { return "process_ocr" }

func (ProcessOcrArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		MaxAttempts: MLProcessMaxAttempts,
		// Dedupe concurrent reindex/retry fan-out per asset: an equivalent job
		// still available/running/completed in the table is silently skipped
		// (Insert returns UniqueSkippedAsDuplicate=true, nil error). Default
		// ByState includes completed, so overlapping full-rebuild chains collapse
		// to one job per asset instead of racing the non-transactional OCR/face
		// save paths. ByArgs also keys on PreprocessVersion, so bumping the
		// version re-allows a re-run.
		UniqueOpts: river.UniqueOpts{
			ByArgs:   true,
			ByPeriod: MLProcessUniquePeriod,
		},
	}
}

// ProcessFaceArgs is the River job payload for face detection and recognition.
// Duplicated here (instead of importing processors) to avoid import cycles.
type ProcessFaceArgs struct {
	AssetID           pgtype.UUID `json:"assetId"`
	PreprocessVersion string      `json:"preprocessVersion,omitempty"`
}

func (ProcessFaceArgs) Kind() string { return "process_face" }

func (ProcessFaceArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		MaxAttempts: MLProcessMaxAttempts,
		// Dedupe concurrent reindex/retry fan-out per asset: an equivalent job
		// still available/running/completed in the table is silently skipped
		// (Insert returns UniqueSkippedAsDuplicate=true, nil error). Default
		// ByState includes completed, so overlapping full-rebuild chains collapse
		// to one job per asset instead of racing the non-transactional OCR/face
		// save paths. ByArgs also keys on PreprocessVersion, so bumping the
		// version re-allows a re-run.
		UniqueOpts: river.UniqueOpts{
			ByArgs:   true,
			ByPeriod: MLProcessUniquePeriod,
		},
	}
}

// ReindexAssetsArgs queues a batch backfill for existing photo indexing tasks.
// Offset advances across self-chained full-rebuild pages (MissingOnly=false);
// it is ignored for missing-only backfills.
type ReindexAssetsArgs struct {
	RepositoryID  *string  `json:"repositoryId,omitempty"`
	Tasks         []string `json:"tasks,omitempty"`
	Limit         int      `json:"limit,omitempty"`
	Offset        int      `json:"offset,omitempty"`
	MissingOnly   bool     `json:"missingOnly,omitempty"`
	ResetSemantic bool     `json:"resetSemantic,omitempty"`
}

func (ReindexAssetsArgs) Kind() string { return "reindex_assets" }

// RebuildLocationClustersArgs rebuilds persisted geohash location clusters.
type RebuildLocationClustersArgs struct {
	RepositoryID *string `json:"repositoryId,omitempty" river:"unique"`
	OwnerID      *int32  `json:"ownerId,omitempty" river:"unique"`
}

func (RebuildLocationClustersArgs) Kind() string { return "rebuild_location_clusters" }

func (RebuildLocationClustersArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{UniqueOpts: river.UniqueOpts{
		ByArgs:   true,
		ByPeriod: 1 * time.Minute,
	}}
}

const (
	RepositoryScanModePeriodic = "periodic"
	RepositoryScanModeManual   = "manual"
)

// ScanRepositoryArgs queues a repository free-workspace scan.
type ScanRepositoryArgs struct {
	RepositoryID string `json:"repositoryId" river:"unique"`
	Mode         string `json:"mode,omitempty" river:"unique"`
	RequestedBy  string `json:"requestedBy,omitempty"`
	Force        bool   `json:"force,omitempty"`
}

func (ScanRepositoryArgs) Kind() string { return "scan_repository" }

func (ScanRepositoryArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{UniqueOpts: river.UniqueOpts{
		ByArgs:   true,
		ByPeriod: 1 * time.Minute,
	}}
}

// DetectStacksArgs triggers logical-media merging and burst detection for a repository.
type DetectStacksArgs struct {
	RepositoryID string `json:"repositoryId" river:"unique"`
}

func (DetectStacksArgs) Kind() string { return "detect_stacks" }

func (DetectStacksArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{UniqueOpts: river.UniqueOpts{
		ByArgs:   true,
		ByPeriod: 1 * time.Minute,
	}}
}

// LivePhotoMatchArgs triggers exact Apple Live Photo matching for a single asset.
type LivePhotoMatchArgs struct {
	AssetID pgtype.UUID `json:"assetId" river:"unique"`
}

func (LivePhotoMatchArgs) Kind() string { return "match_live_photo" }

func (LivePhotoMatchArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{UniqueOpts: river.UniqueOpts{
		ByArgs:   true,
		ByPeriod: 1 * time.Minute,
	}}
}

// ProcessPHashArgs triggers perceptual hash computation for duplicate detection.
type ProcessPHashArgs struct {
	AssetID pgtype.UUID `json:"assetId"`
}

func (ProcessPHashArgs) Kind() string { return "process_phash" }

func (args ProcessPHashArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{UniqueOpts: river.UniqueOpts{
		ByArgs:   true,
		ByPeriod: 10 * time.Minute,
	}}
}

// IngestAssetArgs handles initial staging ingestion and asset creation.
type IngestAssetArgs struct {
	ContentHash      string    `json:"contentHash" river:"unique"`
	QuickFingerprint string    `json:"quickFingerprint,omitempty"`
	StagedPath       string    `json:"stagedPath"`
	UserID           string    `json:"userId" river:"unique"`
	Timestamp        time.Time `json:"timestamp"`
	ContentType      string    `json:"contentType,omitempty"`
	FileName         string    `json:"fileName,omitempty"`
	RepositoryID     string    `json:"repositoryId,omitempty"`
}

func (IngestAssetArgs) Kind() string { return "ingest_asset" }

func (IngestAssetArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{MaxAttempts: LocalToolMaxAttempts}
}

const (
	DiscoverOperationUpsert = "upsert"
	DiscoverOperationDelete = "delete"
)

// DiscoverAssetArgs handles repository file-tree discovery ingestion.
type DiscoverAssetArgs struct {
	RepositoryID string    `json:"repositoryId" river:"unique"`
	RelativePath string    `json:"relativePath" river:"unique"`        // repository-relative user workspace path, e.g. albums/2026/02/a.jpg
	Operation    string    `json:"operation,omitempty" river:"unique"` // upsert (default) or delete
	FileName     string    `json:"fileName"`
	ContentType  string    `json:"contentType,omitempty"`
	FileSize     int64     `json:"fileSize,omitempty"`
	DetectedAt   time.Time `json:"detectedAt"`
}

func (DiscoverAssetArgs) Kind() string { return "discover_asset" }

// InsertOpts reduces burst duplicates from file change storms.
func (DiscoverAssetArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		MaxAttempts: LocalToolMaxAttempts,
		UniqueOpts: river.UniqueOpts{
			ByArgs:   true,
			ByPeriod: 1 * time.Minute,
		},
	}
}

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

func (MetadataArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{MaxAttempts: LocalToolMaxAttempts}
}

// ThumbnailArgs triggers thumbnail generation per asset.
type ThumbnailArgs struct {
	AssetID     pgtype.UUID       `json:"assetId"`
	RepoPath    string            `json:"repoPath"`
	StoragePath string            `json:"storagePath"`
	AssetType   dbtypes.AssetType `json:"assetType"`
}

func (ThumbnailArgs) Kind() string { return "thumbnail_asset" }

func (ThumbnailArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{MaxAttempts: LocalToolMaxAttempts}
}

// TranscodeArgs triggers audio/video transcoding per asset.
type TranscodeArgs struct {
	AssetID     pgtype.UUID       `json:"assetId"`
	RepoPath    string            `json:"repoPath"`
	StoragePath string            `json:"storagePath"`
	AssetType   dbtypes.AssetType `json:"assetType"`
}

func (TranscodeArgs) Kind() string { return "transcode_asset" }

func (TranscodeArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{MaxAttempts: LocalToolMaxAttempts}
}

// DatabaseBackupArgs is the periodic database-backup tick. The worker decides
// from runtime settings whether a dump is actually due, so ticks are cheap and
// schedule changes need no periodic-job re-registration. Force marks an admin
// "back up now" request: it bypasses the enabled/due checks, and ByArgs keeps
// it from being deduped against a recent periodic tick.
type DatabaseBackupArgs struct {
	Force bool `json:"force,omitempty"`
}

func (DatabaseBackupArgs) Kind() string { return "database_backup" }

func (DatabaseBackupArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		Queue:       "db_backup",
		MaxAttempts: 3,
		UniqueOpts:  river.UniqueOpts{ByArgs: true, ByPeriod: 30 * time.Minute},
	}
}

// ScheduleRepositoryScansArgs is a periodic trigger that lists all active
// repositories and enqueues a ScanRepositoryArgs job for each one.
type ScheduleRepositoryScansArgs struct{}

func (ScheduleRepositoryScansArgs) Kind() string { return "schedule_repository_scans" }

func (ScheduleRepositoryScansArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		Queue:      "scan_repository",
		UniqueOpts: river.UniqueOpts{ByPeriod: 1 * time.Minute},
	}
}
