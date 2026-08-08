package jobs

import (
	"time"

	"github.com/google/uuid"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
)

// ProcessSemanticArgs is the River job payload for semantic embedding/classification.
// Duplicated here (instead of importing processors) to avoid import cycles.
// Keep this in sync with processors.SemanticPayload.
type ProcessSemanticArgs struct {
	AssetID           uuid.UUID `json:"assetId"`
	PreprocessVersion string    `json:"preprocessVersion,omitempty"`
}

func (ProcessSemanticArgs) Kind() string { return "process_semantic" }

func (ProcessSemanticArgs) InsertOpts() river.InsertOpts {
	return mlProcessInsertOpts()
}

func mlProcessInsertOpts() river.InsertOpts {
	return river.InsertOpts{
		MaxAttempts: MLProcessMaxAttempts,
		// Dedupe concurrent reindex/retry fan-out per asset: an equivalent job
		// still pending or running is silently skipped. Completed jobs are
		// deliberately excluded: explicit backfill, reset, and retry operations
		// must be able to process the same asset again inside the five-minute
		// window. ByArgs also keys on PreprocessVersion where present.
		UniqueOpts: river.UniqueOpts{
			ByArgs:   true,
			ByPeriod: MLProcessUniquePeriod,
			ByState: []rivertype.JobState{
				rivertype.JobStateAvailable,
				rivertype.JobStatePending,
				rivertype.JobStateRetryable,
				rivertype.JobStateRunning,
				rivertype.JobStateScheduled,
			},
		},
	}
}

const (
	MLPreprocessVersionV1 = "ml-image-v1"
	MLProcessMaxAttempts  = 8
	MLProcessUniquePeriod = 5 * time.Minute
	LocalToolMaxAttempts  = 5
)

type EventRebuildArgs struct {
	OwnerID int32 `json:"ownerId" river:"unique"`
}

func (EventRebuildArgs) Kind() string { return "rebuild_events" }

func (EventRebuildArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		Queue: "rebuild_events",
		UniqueOpts: river.UniqueOpts{
			ByArgs: true,
			ByState: []rivertype.JobState{
				rivertype.JobStateAvailable,
				rivertype.JobStatePending,
				rivertype.JobStateRetryable,
				rivertype.JobStateRunning,
				rivertype.JobStateScheduled,
			},
		},
	}
}

type ScheduleEventRebuildsArgs struct{}

func (ScheduleEventRebuildsArgs) Kind() string { return "schedule_event_rebuilds" }

func (ScheduleEventRebuildsArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: "rebuild_events", UniqueOpts: river.UniqueOpts{
		ByArgs: true,
		ByState: []rivertype.JobState{
			rivertype.JobStateAvailable,
			rivertype.JobStatePending,
			rivertype.JobStateRetryable,
			rivertype.JobStateRunning,
			rivertype.JobStateScheduled,
		},
	}}
}

// ZeroshotClassifyArgs is the River job payload for zero-shot
// classification. It scores the asset's already-stored semantic image embedding
// against classifier prototypes; it does not re-run any ML model.
type ZeroshotClassifyArgs struct {
	AssetID uuid.UUID `json:"assetId"`
}

func (ZeroshotClassifyArgs) Kind() string { return "classify_zeroshot" }

func (ZeroshotClassifyArgs) InsertOpts() river.InsertOpts {
	return mlProcessInsertOpts()
}

// ProcessBioClipArgs is the River job payload for BioCLIP classification.
// Duplicated here (instead of importing processors) to avoid import cycles.
type ProcessBioClipArgs struct {
	AssetID           uuid.UUID `json:"assetId"`
	PreprocessVersion string    `json:"preprocessVersion,omitempty"`
}

func (ProcessBioClipArgs) Kind() string { return "process_bioclip" }

func (ProcessBioClipArgs) InsertOpts() river.InsertOpts {
	return mlProcessInsertOpts()
}

// AssetRetryPayload is the River job payload for selective retry of asset processing tasks
type AssetRetryPayload struct {
	AssetID        string   `json:"assetId" river:"unique"`
	RetryTasks     []string `json:"retryTasks,omitempty"` // Empty means retry all failed tasks
	ForceFullRetry bool     `json:"forceFullRetry,omitempty"`
}

func (AssetRetryPayload) Kind() string { return "retry_asset" }

// InsertOpts collapses only overlapping retry requests for the same asset.
// A completed retry never blocks a later explicit retry.
func (AssetRetryPayload) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		UniqueOpts: river.UniqueOpts{
			ByArgs:   true,
			ByPeriod: 1 * time.Minute,
			ByState: []rivertype.JobState{
				rivertype.JobStateAvailable,
				rivertype.JobStatePending,
				rivertype.JobStateRetryable,
				rivertype.JobStateRunning,
				rivertype.JobStateScheduled,
			},
		},
	}
}

// ProcessOcrArgs is the River job payload for OCR text extraction.
// Duplicated here (instead of importing processors) to avoid import cycles.
type ProcessOcrArgs struct {
	AssetID           uuid.UUID `json:"assetId"`
	PreprocessVersion string    `json:"preprocessVersion,omitempty"`
}

func (ProcessOcrArgs) Kind() string { return "process_ocr" }

func (ProcessOcrArgs) InsertOpts() river.InsertOpts {
	return mlProcessInsertOpts()
}

// ProcessOCROutboxArgs is the periodic trigger for applying authoritative
// SQLite OCR mutations to the rebuildable Bleve sidecar.
type ProcessOCROutboxArgs struct{}

func (ProcessOCROutboxArgs) Kind() string { return "process_ocr_outbox" }

func (ProcessOCROutboxArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		Queue:       "ocr_index",
		MaxAttempts: 5,
		UniqueOpts: river.UniqueOpts{
			ByArgs:   true,
			ByPeriod: time.Second,
		},
	}
}

// ProcessFaceArgs is the River job payload for face detection and recognition.
// Duplicated here (instead of importing processors) to avoid import cycles.
type ProcessFaceArgs struct {
	AssetID           uuid.UUID `json:"assetId"`
	PreprocessVersion string    `json:"preprocessVersion,omitempty"`
}

func (ProcessFaceArgs) Kind() string { return "process_face" }

func (ProcessFaceArgs) InsertOpts() river.InsertOpts {
	return mlProcessInsertOpts()
}

// ProcessVideoFramesArgs is the River job payload for video frame semantic
// embedding. Frames are extracted from the transcoded web.mp4 and written as
// multi-row search_embeddings with frame_ts_ms set.
type ProcessVideoFramesArgs struct {
	AssetID           uuid.UUID `json:"assetId"`
	PreprocessVersion string    `json:"preprocessVersion,omitempty"`
}

func (ProcessVideoFramesArgs) Kind() string { return "process_video_frames" }

func (ProcessVideoFramesArgs) InsertOpts() river.InsertOpts {
	return mlProcessInsertOpts()
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
	AssetID uuid.UUID `json:"assetId" river:"unique"`
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
	AssetID uuid.UUID `json:"assetId"`
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

// DiscoverAssetArgs is a generation-bound repository observation. The worker
// reloads the file-index row and rejects stale tokens before materialization.
type DiscoverAssetArgs struct {
	RepositoryID     uuid.UUID `json:"repositoryId" river:"unique"`
	StoragePath      string    `json:"storagePath" river:"unique"`
	ScanID           uuid.UUID `json:"scanId" river:"unique"`
	ObservationToken string    `json:"observationToken" river:"unique"`
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
	AssetID             uuid.UUID `json:"assetId"`
	ObservationToken    string    `json:"observationToken"`
	ExpectedContentHash string    `json:"expectedContentHash"`
}

func (MetadataArgs) Kind() string { return "metadata_asset" }

func (MetadataArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{MaxAttempts: LocalToolMaxAttempts}
}

// ThumbnailArgs triggers thumbnail generation per asset.
type ThumbnailArgs struct {
	AssetID             uuid.UUID `json:"assetId"`
	ObservationToken    string    `json:"observationToken"`
	ExpectedContentHash string    `json:"expectedContentHash"`
}

func (ThumbnailArgs) Kind() string { return "thumbnail_asset" }

func (ThumbnailArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{MaxAttempts: LocalToolMaxAttempts}
}

// TranscodeArgs triggers audio/video transcoding per asset.
type TranscodeArgs struct {
	AssetID             uuid.UUID `json:"assetId"`
	ObservationToken    string    `json:"observationToken"`
	ExpectedContentHash string    `json:"expectedContentHash"`
}

func (TranscodeArgs) Kind() string { return "transcode_asset" }

func (TranscodeArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{MaxAttempts: LocalToolMaxAttempts}
}

// DatabaseBackupArgs is the periodic database-backup tick. The worker decides
// from runtime settings whether a dump is actually due, so ticks are cheap and
// schedule changes need no periodic-job re-registration. Force marks an admin
// "back up now" request: it bypasses both the enabled/due checks and periodic
// uniqueness so every explicit request can create a new recovery point.
type DatabaseBackupArgs struct {
	Force bool `json:"force,omitempty"`
}

func (DatabaseBackupArgs) Kind() string { return "database_backup" }

func (a DatabaseBackupArgs) InsertOpts() river.InsertOpts {
	opts := river.InsertOpts{
		Queue:       "db_backup",
		MaxAttempts: 3,
	}
	if !a.Force {
		opts.UniqueOpts = river.UniqueOpts{ByArgs: true, ByPeriod: 30 * time.Minute}
	}
	return opts
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
