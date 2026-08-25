package jobs

import (
	"sort"

	"github.com/google/uuid"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
)

// Queue names are a closed runtime contract. Every job's InsertOpts names one
// of these queues, queue setup provisions exactly this set, and pressure
// qualification requires claim/completion evidence for the same set. River's
// implicit "default" queue is deliberately outside the contract: a missing
// Queue value must fail tests instead of leaving durable work unconsumed.
const (
	QueueIngestAsset           = "ingest_asset"
	QueueMetadataAsset         = "metadata_asset"
	QueueThumbnailAsset        = "thumbnail_asset"
	QueueTranscodeAsset        = "transcode_asset"
	QueueRetryAsset            = "retry_asset"
	QueueReindexAssets         = "reindex_assets"
	QueueRebuildLocations      = "rebuild_location_clusters"
	QueueRebuildEvents         = "rebuild_events"
	QueueEventScheduler        = "event_scheduler"
	QueueObserveRepository     = "observe_repository"
	QueueHashRepositoryNode    = "hash_repository_node"
	QueueRepositoryOutbox      = "repository_outbox"
	QueueDatabaseBackup        = "db_backup"
	QueueDetectStacks          = "detect_stacks"
	QueueMatchLivePhoto        = "match_live_photo"
	QueueProcessSemantic       = "process_semantic"
	QueueProcessBioClip        = "process_bioclip"
	QueueProcessOCR            = "process_ocr"
	QueueOCRIndex              = "ocr_index"
	QueueProcessFace           = "process_face"
	QueueProcessVideoFrames    = "process_video_frames"
	QueueClassifyZeroShot      = "classify_zeroshot"
	QueueProcessPerceptualHash = "process_phash"
)

// activeUniqueStates is the convergence boundary for repeatable work. A job
// already queued or running blocks an equivalent follower, while completion
// immediately permits a later factual change or explicit retry. Do not combine
// this with ByPeriod: a period is part of River's uniqueness key, so a
// long-running job can acquire a follower when the clock crosses a new window.
func activeUniqueStates() []rivertype.JobState {
	return []rivertype.JobState{
		rivertype.JobStateAvailable,
		rivertype.JobStatePending,
		rivertype.JobStateRetryable,
		rivertype.JobStateRunning,
		rivertype.JobStateScheduled,
	}
}

// ProcessSemanticArgs is the River job payload for semantic embedding/classification.
// Duplicated here (instead of importing processors) to avoid import cycles.
// Keep this in sync with processors.SemanticPayload.
type ProcessSemanticArgs struct {
	AssetID           uuid.UUID `json:"assetId"`
	ExpectedContentID uuid.UUID `json:"expectedContentId,omitempty"`
	PreprocessVersion string    `json:"preprocessVersion,omitempty"`
}

func (ProcessSemanticArgs) Kind() string { return "process_semantic" }

func (ProcessSemanticArgs) InsertOpts() river.InsertOpts {
	return mlProcessInsertOpts(QueueProcessSemantic)
}

func mlProcessInsertOpts(queue string) river.InsertOpts {
	return river.InsertOpts{
		Queue:       queue,
		MaxAttempts: MLProcessMaxAttempts,
		// Dedupe concurrent reindex/retry fan-out per asset: an equivalent job
		// still pending or running is silently skipped. Completed jobs are
		// deliberately excluded: explicit backfill, reset, and retry operations
		// must be able to process the same asset again inside the five-minute
		// window. ByArgs also keys on PreprocessVersion where present.
		UniqueOpts: river.UniqueOpts{
			ByArgs:  true,
			ByState: activeUniqueStates(),
		},
	}
}

const (
	MLPreprocessVersionV1 = "ml-image-v1"
	MLProcessMaxAttempts  = 8
	LocalToolMaxAttempts  = 5
)

type EventRebuildArgs struct {
	OwnerID int32 `json:"ownerId" river:"unique"`
	Force   bool  `json:"force,omitempty"`
}

func (EventRebuildArgs) Kind() string { return "rebuild_events" }

func (EventRebuildArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		Queue: QueueRebuildEvents,
		UniqueOpts: river.UniqueOpts{
			ByArgs:  true,
			ByState: activeUniqueStates(),
		},
	}
}

type ScheduleEventRebuildsArgs struct{}

func (ScheduleEventRebuildsArgs) Kind() string { return "schedule_event_rebuilds" }

func (ScheduleEventRebuildsArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: QueueEventScheduler, UniqueOpts: river.UniqueOpts{
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
	AssetID           uuid.UUID `json:"assetId"`
	ExpectedContentID uuid.UUID `json:"expectedContentId,omitempty"`
}

func (ZeroshotClassifyArgs) Kind() string { return "classify_zeroshot" }

func (ZeroshotClassifyArgs) InsertOpts() river.InsertOpts {
	return mlProcessInsertOpts(QueueClassifyZeroShot)
}

// ProcessBioClipArgs is the River job payload for BioCLIP classification.
// Duplicated here (instead of importing processors) to avoid import cycles.
type ProcessBioClipArgs struct {
	AssetID           uuid.UUID `json:"assetId"`
	ExpectedContentID uuid.UUID `json:"expectedContentId,omitempty"`
	PreprocessVersion string    `json:"preprocessVersion,omitempty"`
}

func (ProcessBioClipArgs) Kind() string { return "process_bioclip" }

func (ProcessBioClipArgs) InsertOpts() river.InsertOpts {
	return mlProcessInsertOpts(QueueProcessBioClip)
}

// AssetRetryPayload is the River job payload for selective retry of asset processing tasks
type AssetRetryPayload struct {
	AssetID        string   `json:"assetId" river:"unique"`
	RetryTasks     []string `json:"retryTasks,omitempty"` // Empty means retry all failed tasks
	ForceFullRetry bool     `json:"forceFullRetry,omitempty"`
	// EffectID is excluded from River's unique subset so overlapping explicit
	// requests still collapse by AssetID, but it remains stable when River
	// replays the accepted request after a crash.
	EffectID uuid.UUID `json:"effectId"`
}

func (AssetRetryPayload) Kind() string { return "retry_asset" }

// InsertOpts collapses only overlapping retry requests for the same asset.
// A completed retry never blocks a later explicit retry.
func (AssetRetryPayload) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		Queue: QueueRetryAsset,
		UniqueOpts: river.UniqueOpts{
			ByArgs:  true,
			ByState: activeUniqueStates(),
		},
	}
}

// ProcessOcrArgs is the River job payload for OCR text extraction.
// Duplicated here (instead of importing processors) to avoid import cycles.
type ProcessOcrArgs struct {
	AssetID           uuid.UUID `json:"assetId"`
	ExpectedContentID uuid.UUID `json:"expectedContentId,omitempty"`
	PreprocessVersion string    `json:"preprocessVersion,omitempty"`
}

func (ProcessOcrArgs) Kind() string { return "process_ocr" }

func (ProcessOcrArgs) InsertOpts() river.InsertOpts {
	return mlProcessInsertOpts(QueueProcessOCR)
}

// ProcessOCROutboxArgs applies authoritative SQLite OCR mutations to the
// rebuildable Bleve sidecar. Mutation wakeups are coalesced before insertion,
// and active-state uniqueness prevents a periodic follower from queuing behind
// a slow index flush.
type ProcessOCROutboxArgs struct{}

func (ProcessOCROutboxArgs) Kind() string { return "process_ocr_outbox" }

func (ProcessOCROutboxArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		Queue:       QueueOCRIndex,
		MaxAttempts: 5,
		UniqueOpts: river.UniqueOpts{
			ByArgs:  true,
			ByState: activeUniqueStates(),
		},
	}
}

// ProcessFaceArgs is the River job payload for face detection and recognition.
// Duplicated here (instead of importing processors) to avoid import cycles.
type ProcessFaceArgs struct {
	AssetID           uuid.UUID `json:"assetId"`
	ExpectedContentID uuid.UUID `json:"expectedContentId,omitempty"`
	PreprocessVersion string    `json:"preprocessVersion,omitempty"`
}

func (ProcessFaceArgs) Kind() string { return "process_face" }

func (ProcessFaceArgs) InsertOpts() river.InsertOpts {
	return mlProcessInsertOpts(QueueProcessFace)
}

// ProcessVideoFramesArgs is the River job payload for video frame semantic
// embedding. Frames are extracted from the transcoded web.mp4 and written as
// multi-row search_embeddings with frame_ts_ms set.
type ProcessVideoFramesArgs struct {
	AssetID           uuid.UUID `json:"assetId"`
	ExpectedContentID uuid.UUID `json:"expectedContentId,omitempty"`
	PreprocessVersion string    `json:"preprocessVersion,omitempty"`
}

func (ProcessVideoFramesArgs) Kind() string { return "process_video_frames" }

func (ProcessVideoFramesArgs) InsertOpts() river.InsertOpts {
	return mlProcessInsertOpts(QueueProcessVideoFrames)
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

func (ReindexAssetsArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: QueueReindexAssets}
}

// RebuildLocationClustersArgs rebuilds persisted geohash location clusters.
type RebuildLocationClustersArgs struct {
	RepositoryID *string `json:"repositoryId,omitempty" river:"unique"`
	OwnerID      *int32  `json:"ownerId,omitempty" river:"unique"`
}

func (RebuildLocationClustersArgs) Kind() string { return "rebuild_location_clusters" }

func (RebuildLocationClustersArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: QueueRebuildLocations, UniqueOpts: river.UniqueOpts{
		ByArgs:  true,
		ByState: activeUniqueStates(),
	}}
}

// ScheduleLocationRebuildsArgs closes the insert-versus-running-completion
// race by re-enqueueing every durable scope whose source revision is ahead of
// its published projection. The scheduler itself coalesces across active
// states and performs no catalog mutation.
type ScheduleLocationRebuildsArgs struct{}

func (ScheduleLocationRebuildsArgs) Kind() string { return "schedule_location_rebuilds" }

func (ScheduleLocationRebuildsArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: QueueRebuildLocations, UniqueOpts: river.UniqueOpts{
		ByArgs:  true,
		ByState: activeUniqueStates(),
	}}
}

// ResolveLocationClustersArgs drains the durable reverse-geocoding projection
// for one settings revision. A newer revision may coexist briefly so the old
// worker can observe the revision guard and exit without publishing anything.
type ResolveLocationClustersArgs struct {
	GeocodingRevision int64 `json:"geocodingRevision" river:"unique"`
}

func (ResolveLocationClustersArgs) Kind() string { return "resolve_location_clusters" }

func (ResolveLocationClustersArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		Queue: QueueRebuildLocations,
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

const (
	RepositoryScanModePeriodic = "periodic"
	RepositoryScanModeManual   = "manual"
)

// ObserveRepositoryArgs advances one bounded, revision-fenced controller turn.
// River snoozes the same durable job between turns, so running work participates
// in uniqueness and an outbox replay cannot create parallel controllers.
type ObserveRepositoryArgs struct {
	RepositoryID  string `json:"repositoryId" river:"unique"`
	OperationID   string `json:"operationId" river:"unique"`
	ExpectedEpoch int64  `json:"expectedEpoch" river:"unique"`
}

func (ObserveRepositoryArgs) Kind() string { return "observe_repository" }

func (ObserveRepositoryArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		Queue: QueueObserveRepository, MaxAttempts: 20,
		UniqueOpts: river.UniqueOpts{ByArgs: true, ByState: []rivertype.JobState{
			rivertype.JobStateAvailable,
			rivertype.JobStatePending,
			rivertype.JobStateRetryable,
			rivertype.JobStateRunning,
			rivertype.JobStateScheduled,
		}},
	}
}

type HashRepositoryNodeArgs struct {
	NodeID           string `json:"nodeId" river:"unique"`
	ExpectedRevision int64  `json:"expectedRevision" river:"unique"`
}

func (HashRepositoryNodeArgs) Kind() string { return "hash_repository_node" }

func (HashRepositoryNodeArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		Queue: QueueHashRepositoryNode, MaxAttempts: 12,
		UniqueOpts: river.UniqueOpts{ByArgs: true, ByState: []rivertype.JobState{
			rivertype.JobStateAvailable,
			rivertype.JobStatePending,
			rivertype.JobStateRetryable,
			rivertype.JobStateRunning,
			rivertype.JobStateScheduled,
		}},
	}
}

type DrainRepositoryOutboxArgs struct {
	EffectKind string `json:"effectKind" river:"unique"`
}

func (DrainRepositoryOutboxArgs) Kind() string { return "drain_repository_outbox" }

func (DrainRepositoryOutboxArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		Queue: QueueRepositoryOutbox, MaxAttempts: 20,
		UniqueOpts: river.UniqueOpts{ByArgs: true, ByState: []rivertype.JobState{
			rivertype.JobStateAvailable,
			rivertype.JobStatePending,
			rivertype.JobStateRetryable,
			rivertype.JobStateRunning,
			rivertype.JobStateScheduled,
		}},
	}
}

// DetectStacksArgs triggers logical-media merging and burst detection for a repository.
type DetectStacksArgs struct {
	RepositoryID string `json:"repositoryId" river:"unique"`
}

func (DetectStacksArgs) Kind() string { return "detect_stacks" }

func (DetectStacksArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: QueueDetectStacks, UniqueOpts: river.UniqueOpts{
		ByArgs:  true,
		ByState: activeUniqueStates(),
	}}
}

// LivePhotoMatchArgs triggers exact Apple Live Photo matching for a single asset.
type LivePhotoMatchArgs struct {
	AssetID uuid.UUID `json:"assetId" river:"unique"`
}

func (LivePhotoMatchArgs) Kind() string { return "match_live_photo" }

func (LivePhotoMatchArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: QueueMatchLivePhoto, UniqueOpts: river.UniqueOpts{
		ByArgs:  true,
		ByState: activeUniqueStates(),
	}}
}

// ProcessPHashArgs triggers perceptual hash computation for duplicate detection.
type ProcessPHashArgs struct {
	AssetID           uuid.UUID `json:"assetId"`
	ExpectedContentID uuid.UUID `json:"expectedContentId,omitempty"`
}

func (ProcessPHashArgs) Kind() string { return "process_phash" }

func (args ProcessPHashArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: QueueProcessPerceptualHash, UniqueOpts: river.UniqueOpts{
		ByArgs:  true,
		ByState: activeUniqueStates(),
	}}
}

// IngestAssetArgs resumes one durable private-staging commit. Repository,
// owner, hash, filename, and paths stay in SQLite rather than River payloads.
type IngestAssetArgs struct {
	CommitID uuid.UUID `json:"commitId" river:"unique"`
}

func (IngestAssetArgs) Kind() string { return "ingest_asset" }

func (IngestAssetArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: QueueIngestAsset, MaxAttempts: LocalToolMaxAttempts,
		UniqueOpts: river.UniqueOpts{ByArgs: true}}
}

// MetadataArgs triggers EXIF/ffprobe metadata extraction per asset.
type MetadataArgs struct {
	AssetID           uuid.UUID `json:"assetId"`
	ExpectedContentID uuid.UUID `json:"expectedContentId"`
	EffectID          uuid.UUID `json:"effectId"`
}

func (MetadataArgs) Kind() string { return "metadata_asset" }

func (MetadataArgs) InsertOpts() river.InsertOpts {
	return localProcessingInsertOpts(QueueMetadataAsset)
}

// ThumbnailArgs triggers thumbnail generation per asset.
type ThumbnailArgs struct {
	AssetID           uuid.UUID `json:"assetId"`
	ExpectedContentID uuid.UUID `json:"expectedContentId"`
	EffectID          uuid.UUID `json:"effectId"`
}

func (ThumbnailArgs) Kind() string { return "thumbnail_asset" }

func (ThumbnailArgs) InsertOpts() river.InsertOpts {
	return localProcessingInsertOpts(QueueThumbnailAsset)
}

// TranscodeArgs triggers audio/video transcoding per asset.
type TranscodeArgs struct {
	AssetID           uuid.UUID `json:"assetId"`
	ExpectedContentID uuid.UUID `json:"expectedContentId"`
	EffectID          uuid.UUID `json:"effectId"`
}

func (TranscodeArgs) Kind() string { return "transcode_asset" }

func (TranscodeArgs) InsertOpts() river.InsertOpts {
	return localProcessingInsertOpts(QueueTranscodeAsset)
}

func localProcessingInsertOpts(queue string) river.InsertOpts {
	return river.InsertOpts{
		Queue:       queue,
		MaxAttempts: LocalToolMaxAttempts,
		UniqueOpts: river.UniqueOpts{
			ByArgs: true,
			ByState: []rivertype.JobState{
				rivertype.JobStateAvailable,
				rivertype.JobStatePending,
				rivertype.JobStateRetryable,
				rivertype.JobStateRunning,
				rivertype.JobStateScheduled,
				rivertype.JobStateCompleted,
			},
		},
	}
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
		Queue:       QueueDatabaseBackup,
		MaxAttempts: 3,
	}
	if !a.Force {
		opts.UniqueOpts = river.UniqueOpts{ByArgs: true, ByState: activeUniqueStates()}
	}
	return opts
}

// ScheduleRepositoryScansArgs is a periodic trigger that lists all active
// repositories and requests a bounded observation turn for each one.
type ScheduleRepositoryScansArgs struct{}

func (ScheduleRepositoryScansArgs) Kind() string { return "schedule_repository_scans" }

func (ScheduleRepositoryScansArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		Queue:      QueueObserveRepository,
		UniqueOpts: river.UniqueOpts{ByArgs: true, ByState: activeUniqueStates()},
	}
}

// RuntimeJob is the minimum contract shared by every registered River job.
// Keeping the closed catalog here makes missing queue routing observable before
// a job can be persisted to River's unconsumed implicit default queue.
type RuntimeJob interface {
	Kind() string
	InsertOpts() river.InsertOpts
}

// RuntimeJobCatalog returns one zero-value representative of every job type
// registered by app.Run. Tests compare this list with the source-declared Kind
// methods and with the queue setup map, so additions cannot silently drift.
func RuntimeJobCatalog() []RuntimeJob {
	return []RuntimeJob{
		ProcessSemanticArgs{}, EventRebuildArgs{}, ScheduleEventRebuildsArgs{},
		ZeroshotClassifyArgs{}, ProcessBioClipArgs{}, AssetRetryPayload{},
		ProcessOcrArgs{}, ProcessOCROutboxArgs{}, ProcessFaceArgs{},
		ProcessVideoFramesArgs{}, ReindexAssetsArgs{}, RebuildLocationClustersArgs{}, ScheduleLocationRebuildsArgs{},
		ResolveLocationClustersArgs{}, ObserveRepositoryArgs{}, HashRepositoryNodeArgs{},
		DrainRepositoryOutboxArgs{}, DetectStacksArgs{}, LivePhotoMatchArgs{},
		ProcessPHashArgs{}, IngestAssetArgs{}, MetadataArgs{}, ThumbnailArgs{},
		TranscodeArgs{}, DatabaseBackupArgs{}, ScheduleRepositoryScansArgs{},
	}
}

// RuntimeQueueNames returns the canonical sorted queue set used by runtime
// setup, diagnostics, and qualification tooling.
func RuntimeQueueNames() []string {
	seen := make(map[string]struct{})
	for _, job := range RuntimeJobCatalog() {
		seen[job.InsertOpts().Queue] = struct{}{}
	}
	queues := make([]string, 0, len(seen))
	for queue := range seen {
		queues = append(queues, queue)
	}
	sort.Strings(queues)
	return queues
}
