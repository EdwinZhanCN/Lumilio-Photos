package processors

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/google/uuid"
	"github.com/riverqueue/river"

	"server/internal/db/dbtypes"
	"server/internal/db/dbtypes/status"
	"server/internal/db/repo"
	"server/internal/queue/jobs"
)

// ProcessRetryTask is the entry point for the retry worker. It handles AssetRetryPayload.
func (ap *AssetProcessor) ProcessRetryTask(ctx context.Context, payload jobs.AssetRetryPayload) error {
	return ap.RetryAsset(ctx, payload.AssetID, payload.RetryTasks)
}

// RetryAsset handles selective retry of failed asset processing tasks by re-enqueueing
// them to the appropriate per-task queues.
func (ap *AssetProcessor) RetryAsset(ctx context.Context, assetIDStr string, retryTasks []string) error {
	// Parse asset ID
	assetID, err := uuid.Parse(assetIDStr)
	if err != nil {
		return fmt.Errorf("invalid asset ID: %w", err)
	}

	// Get asset from database
	asset, err := ap.queries.GetAssetByID(ctx, assetID)
	if err != nil {
		return fmt.Errorf("asset not found: %w", err)
	}
	if !asset.RepositoryID.Valid || ap.repositories == nil {
		return fmt.Errorf("asset retry repository gate unavailable")
	}
	_, releaseWork, err := ap.repositories.BeginRepositoryWork(ctx, asset.RepositoryID.UUID.String(), dbtypes.RepositoryActivityProcessing)
	if err != nil {
		return err
	}
	defer releaseWork()

	// Parse current status
	var currentStatus status.AssetStatus
	if len(asset.Status) > 0 {
		currentStatus, err = status.FromJSON(asset.Status)
		if err != nil {
			return fmt.Errorf("failed to parse asset status: %w", err)
		}
	}

	source, err := ap.resolveCurrentAssetSource(ctx, assetID, "", "")
	if err != nil {
		return err
	}
	defer source.Close()

	// Determine which tasks to retry
	tasksToRetry := retryTasks
	if len(tasksToRetry) == 0 {
		// If no specific tasks requested, retry all failed tasks
		tasksToRetry = currentStatus.GetFailedTasks()
	}

	if len(tasksToRetry) == 0 {
		return fmt.Errorf("no failed tasks to retry")
	}

	tx, err := ap.database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin asset retry transaction: %w", err)
	}
	defer tx.Rollback()
	txQueries := ap.queries.WithTx(tx)
	err = txQueries.MutateAssetStatus(ctx, asset.AssetID, func(current status.AssetStatus) (status.AssetStatus, error) {
		current.EnsureTasks(tasksToRetry)
		for _, taskName := range tasksToRetry {
			current.MarkTaskPending(taskName, fmt.Sprintf("Retry queued for %s", taskName))
		}
		return current, nil
	})
	if err != nil {
		return fmt.Errorf("failed to update asset status: %w", err)
	}

	// Re-enqueue tasks based on failed task names (using queue names as canonical task names)
	assetType := dbtypes.AssetType(asset.Type)
	log.Printf("Retrying %d tasks for %s asset %s: %v", len(tasksToRetry), assetType, asset.AssetID.String(), tasksToRetry)
	if err := ap.enqueueRetryTasks(ctx, tx, &asset, assetType, tasksToRetry, source.observation.ObservationToken); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit asset retry transaction: %w", err)
	}
	return nil
}

// enqueueRetryTasks re-enqueues specific tasks to their respective queues.
// tasksToRetry uses queue names directly (bijection: queue name = task identifier).
func (ap *AssetProcessor) enqueueRetryTasks(
	ctx context.Context,
	tx *sql.Tx,
	asset *repo.Asset,
	assetType dbtypes.AssetType,
	tasksToRetry []string,
	observationToken string,
) error {
	// Build queue set from task/queue names (they are the same in our bijection)
	queueSet := make(map[string]bool)
	for _, queueName := range tasksToRetry {
		queueSet[queueName] = true
	}

	// Prepare common job arguments
	commonMeta := jobs.MetadataArgs{AssetID: asset.AssetID, ObservationToken: observationToken, ExpectedContentHash: asset.ContentHash}
	commonThumb := jobs.ThumbnailArgs{
		AssetID: asset.AssetID, ObservationToken: observationToken, ExpectedContentHash: asset.ContentHash,
	}
	commonTranscode := jobs.TranscodeArgs{
		AssetID: asset.AssetID, ObservationToken: observationToken, ExpectedContentHash: asset.ContentHash,
	}

	// Enqueue tasks based on queue names (bijection: queue name = task name)
	// Available queues: metadata_asset, thumbnail_asset, transcode_asset,
	// process_semantic, process_bioclip, process_ocr, process_face,
	// process_video_frames

	// Enqueue metadata_asset if requested (all asset types support metadata)
	if queueSet["metadata_asset"] {
		err := ap.insertRetryJob(ctx, tx, commonMeta, "metadata_asset")
		if err != nil {
			return fmt.Errorf("enqueue metadata_asset retry: %w", err)
		}
		log.Printf("Enqueued metadata task for %s asset %s", assetType, asset.AssetID.String())
	}

	// Enqueue thumbnail_asset if requested AND asset type supports it
	if queueSet["thumbnail_asset"] {
		switch assetType {
		case dbtypes.AssetTypePhoto:
			err := ap.insertRetryJob(ctx, tx, commonThumb, "thumbnail_asset")
			if err != nil {
				return fmt.Errorf("enqueue thumbnail_asset retry for photo: %w", err)
			}
			log.Printf("Enqueued thumbnail task for photo asset %s", asset.AssetID.String())
		case dbtypes.AssetTypeVideo:
			err := ap.insertRetryJob(ctx, tx, commonThumb, "thumbnail_asset")
			if err != nil {
				return fmt.Errorf("enqueue thumbnail_asset retry for video: %w", err)
			}
			log.Printf("Enqueued thumbnail task for video asset %s", asset.AssetID.String())
		case dbtypes.AssetTypeAudio:
			// Skip thumbnail for audio - they use waveform instead
			log.Printf("Skipped thumbnail task for audio asset %s (audio uses waveform instead)", asset.AssetID.String())
		default:
			return fmt.Errorf("unsupported asset type for thumbnail: %s", assetType)
		}
	}

	// Enqueue transcode_asset if requested AND asset type supports it
	if queueSet["transcode_asset"] {
		switch assetType {
		case dbtypes.AssetTypeVideo:
			err := ap.insertRetryJob(ctx, tx, commonTranscode, "transcode_asset")
			if err != nil {
				return fmt.Errorf("enqueue transcode_asset retry for video: %w", err)
			}
			log.Printf("Enqueued transcode task for video asset %s", asset.AssetID.String())
		case dbtypes.AssetTypeAudio:
			err := ap.insertRetryJob(ctx, tx, commonTranscode, "transcode_asset")
			if err != nil {
				return fmt.Errorf("enqueue transcode_asset retry for audio: %w", err)
			}
			log.Printf("Enqueued transcode task for audio asset %s", asset.AssetID.String())
		case dbtypes.AssetTypePhoto:
			// Skip transcode for photos - they don't need transcoding
			// This prevents photos from being incorrectly added to the transcode queue
			log.Printf("Skipped transcode task for photo asset %s (photos don't need transcoding)", asset.AssetID.String())
		default:
			return fmt.Errorf("unsupported asset type for transcode: %s", assetType)
		}
	}

	// Enqueue ML tasks directly if requested (now decoupled from metadata)
	if assetType == dbtypes.AssetTypePhoto {
		// Check each ML task queue name
		if queueSet["process_semantic"] || queueSet["process_bioclip"] || queueSet["process_ocr"] || queueSet["process_face"] {
			err := ap.retryMLJobs(ctx, tx, asset, queueSet)
			if err != nil {
				return fmt.Errorf("enqueue ML retry: %w", err)
			}
		}
	}
	if assetType == dbtypes.AssetTypeVideo && queueSet["process_video_frames"] {
		mlConfig, err := ap.settingsService.GetEffectiveMLConfig(ctx)
		if err != nil {
			return fmt.Errorf("load ML settings: %w", err)
		}
		if mlConfig.SemanticEnabled && mlConfig.VideoSemanticEnabled {
			err = ap.insertRetryJob(ctx, tx, jobs.ProcessVideoFramesArgs{
				AssetID: asset.AssetID, PreprocessVersion: jobs.MLPreprocessVersionV1,
			}, "process_video_frames")
		}
		if err != nil {
			return fmt.Errorf("enqueue process_video_frames retry: %w", err)
		}
	}

	log.Printf("Completed retry task enqueueing for asset %s", asset.AssetID.String())
	return nil
}

// retryMLJobs re-enqueues specific ML pointer jobs that failed.
func (ap *AssetProcessor) retryMLJobs(ctx context.Context, tx *sql.Tx, asset *repo.Asset, taskSet map[string]bool) error {
	mlConfig, err := ap.settingsService.GetEffectiveMLConfig(ctx)
	if err != nil {
		return fmt.Errorf("load ML settings: %w", err)
	}

	if taskSet["process_semantic"] && mlConfig.SemanticEnabled {
		err = ap.insertRetryJob(ctx, tx, jobs.ProcessSemanticArgs{
			AssetID:           asset.AssetID,
			PreprocessVersion: jobs.MLPreprocessVersionV1,
		}, "process_semantic")
		if err != nil {
			return fmt.Errorf("enqueue process_semantic retry: %w", err)
		}
	}

	if taskSet["process_bioclip"] && mlConfig.BioCLIPEnabled {
		err = ap.insertRetryJob(ctx, tx, jobs.ProcessBioClipArgs{
			AssetID:           asset.AssetID,
			PreprocessVersion: jobs.MLPreprocessVersionV1,
		}, "process_bioclip")
		if err != nil {
			return fmt.Errorf("enqueue process_bioclip retry: %w", err)
		}
	}

	if taskSet["process_ocr"] && mlConfig.OCREnabled {
		err = ap.insertRetryJob(ctx, tx, jobs.ProcessOcrArgs{
			AssetID:           asset.AssetID,
			PreprocessVersion: jobs.MLPreprocessVersionV1,
		}, "process_ocr")
		if err != nil {
			return fmt.Errorf("enqueue process_ocr retry: %w", err)
		}
	}

	if taskSet["process_face"] && mlConfig.FaceEnabled {
		err = ap.insertRetryJob(ctx, tx, jobs.ProcessFaceArgs{
			AssetID:           asset.AssetID,
			PreprocessVersion: jobs.MLPreprocessVersionV1,
		}, "process_face")
		if err != nil {
			return fmt.Errorf("enqueue process_face retry: %w", err)
		}
	}

	return nil
}

func (ap *AssetProcessor) insertRetryJob(ctx context.Context, tx *sql.Tx, args river.JobArgs, queue string) error {
	if ap.beforeRetryInsert != nil {
		if err := ap.beforeRetryInsert(queue); err != nil {
			return err
		}
	}
	opts := river.InsertOpts{Queue: queue}
	if provider, ok := args.(interface{ InsertOpts() river.InsertOpts }); ok {
		opts = provider.InsertOpts()
		opts.Queue = queue
	}
	_, err := ap.queueClient.InsertTx(ctx, tx, args, &opts)
	return err
}

// RetryAssetTask retries a specific task for an asset
func (ap *AssetProcessor) RetryAssetTask(ctx context.Context, assetIDStr string, taskName string) error {
	return ap.RetryAsset(ctx, assetIDStr, []string{taskName})
}
