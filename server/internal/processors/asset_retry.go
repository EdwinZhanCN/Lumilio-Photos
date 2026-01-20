package processors

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/h2non/bimg"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/riverqueue/river"

	"server/internal/db/dbtypes"
	"server/internal/db/dbtypes/status"
	"server/internal/db/repo"
	"server/internal/queue/jobs"
	"server/internal/utils/imaging"
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

	pgUUID := pgtype.UUID{Bytes: assetID, Valid: true}

	// Get asset from database
	asset, err := ap.queries.GetAssetByID(ctx, pgUUID)
	if err != nil {
		return fmt.Errorf("asset not found: %w", err)
	}

	// Parse current status
	var currentStatus status.AssetStatus
	if len(asset.Status) > 0 {
		currentStatus, err = status.FromJSONB(asset.Status)
		if err != nil {
			return fmt.Errorf("failed to parse asset status: %w", err)
		}
	}

	// Get repository information
	repository, err := ap.queries.GetRepository(ctx, asset.RepositoryID)
	if err != nil {
		return fmt.Errorf("failed to get repository: %w", err)
	}

	// Check if storage path exists
	if asset.StoragePath == nil || *asset.StoragePath == "" {
		return fmt.Errorf("asset has no storage path")
	}

	// Check if the file exists
	assetPath := filepath.Join(repository.Path, *asset.StoragePath)
	if _, err := os.Stat(assetPath); os.IsNotExist(err) {
		return fmt.Errorf("asset file not found: %s", assetPath)
	}

	// Determine which tasks to retry
	tasksToRetry := retryTasks
	if len(tasksToRetry) == 0 {
		// If no specific tasks requested, retry all failed tasks
		tasksToRetry = currentStatus.GetFailedTasks()
	}

	if len(tasksToRetry) == 0 {
		return fmt.Errorf("no failed tasks to retry")
	}

	// Update status to retrying
	retryStatus := status.NewProcessingStatus(fmt.Sprintf("Retrying tasks: %v", tasksToRetry))
	retryStatusJSON, err := retryStatus.ToJSONB()
	if err != nil {
		return fmt.Errorf("failed to marshal retry status: %w", err)
	}

	_, err = ap.queries.UpdateAssetStatus(ctx, repo.UpdateAssetStatusParams{
		AssetID: asset.AssetID,
		Status:  retryStatusJSON,
	})
	if err != nil {
		return fmt.Errorf("failed to update asset status: %w", err)
	}

	// Re-enqueue tasks based on failed task names (using queue names as canonical task names)
	assetType := dbtypes.AssetType(asset.Type)
	log.Printf("Retrying %d tasks for %s asset %s: %v", len(tasksToRetry), assetType, asset.AssetID.String(), tasksToRetry)
	return ap.enqueueRetryTasks(ctx, &asset, repository, assetType, tasksToRetry)
}

// enqueueRetryTasks re-enqueues specific tasks to their respective queues.
// tasksToRetry uses queue names directly (bijection: queue name = task identifier).
func (ap *AssetProcessor) enqueueRetryTasks(
	ctx context.Context,
	asset *repo.Asset,
	repository repo.Repository,
	assetType dbtypes.AssetType,
	tasksToRetry []string,
) error {
	// Build queue set from task/queue names (they are the same in our bijection)
	queueSet := make(map[string]bool)
	for _, queueName := range tasksToRetry {
		queueSet[queueName] = true
	}

	// Prepare common job arguments
	commonMeta := jobs.MetadataArgs{
		AssetID:          asset.AssetID,
		RepoPath:         repository.Path,
		StoragePath:      *asset.StoragePath,
		AssetType:        assetType,
		OriginalFilename: asset.OriginalFilename,
		FileSize:         asset.FileSize,
		MimeType:         asset.MimeType,
	}
	commonThumb := jobs.ThumbnailArgs{
		AssetID:     asset.AssetID,
		RepoPath:    repository.Path,
		StoragePath: *asset.StoragePath,
		AssetType:   assetType,
	}
	commonTranscode := jobs.TranscodeArgs{
		AssetID:     asset.AssetID,
		RepoPath:    repository.Path,
		StoragePath: *asset.StoragePath,
		AssetType:   assetType,
	}

	// Enqueue tasks based on queue names (bijection: queue name = task name)
	// Available queues: metadata_asset, thumbnail_asset, transcode_asset, process_clip, process_ocr, process_caption, process_face

	// Enqueue metadata_asset if requested (all asset types support metadata)
	if queueSet["metadata_asset"] {
		_, err := ap.queueClient.Insert(ctx, commonMeta, &river.InsertOpts{Queue: "metadata_asset"})
		if err != nil {
			return fmt.Errorf("enqueue metadata_asset retry: %w", err)
		}
		log.Printf("Enqueued metadata task for %s asset %s", assetType, asset.AssetID.String())
	}

	// Enqueue thumbnail_asset if requested AND asset type supports it
	if queueSet["thumbnail_asset"] {
		switch assetType {
		case dbtypes.AssetTypePhoto:
			_, err := ap.queueClient.Insert(ctx, commonThumb, &river.InsertOpts{Queue: "thumbnail_asset"})
			if err != nil {
				return fmt.Errorf("enqueue thumbnail_asset retry for photo: %w", err)
			}
			log.Printf("Enqueued thumbnail task for photo asset %s", asset.AssetID.String())
		case dbtypes.AssetTypeVideo:
			_, err := ap.queueClient.Insert(ctx, commonThumb, &river.InsertOpts{Queue: "thumbnail_asset"})
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
			_, err := ap.queueClient.Insert(ctx, commonTranscode, &river.InsertOpts{Queue: "transcode_asset"})
			if err != nil {
				return fmt.Errorf("enqueue transcode_asset retry for video: %w", err)
			}
			log.Printf("Enqueued transcode task for video asset %s", asset.AssetID.String())
		case dbtypes.AssetTypeAudio:
			_, err := ap.queueClient.Insert(ctx, commonTranscode, &river.InsertOpts{Queue: "transcode_asset"})
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
	// ML tasks are only applicable to photos
	if assetType == dbtypes.AssetTypePhoto {
		fullPath := filepath.Join(repository.Path, *asset.StoragePath)

		// Check each ML task queue name
		if queueSet["process_clip"] || queueSet["process_ocr"] || queueSet["process_caption"] || queueSet["process_face"] {
			// Re-enqueue ML jobs based on which ones failed
			// We need to pass which specific ML tasks to retry
			err := ap.retryMLJobs(ctx, asset, fullPath, queueSet)
			if err != nil {
				return fmt.Errorf("enqueue ML retry: %w", err)
			}
		}
	}

	log.Printf("Completed retry task enqueueing for asset %s", asset.AssetID.String())
	return nil
}

// retryMLJobs re-enqueues specific ML jobs that failed
func (ap *AssetProcessor) retryMLJobs(ctx context.Context, asset *repo.Asset, fullPath string, taskSet map[string]bool) error {
	mlConfig := ap.appConfig.MLConfig

	// Extract RAW preview once (shared by all ML jobs if needed)
	previewData, err := ap.extractRAWPreview(ctx, fullPath, asset.OriginalFilename)
	if err != nil {
		return fmt.Errorf("extract RAW preview for ML retry: %w", err)
	}

	// Prepare image input reader
	var imageInput io.Reader
	var closeFunc func() error

	if previewData != nil {
		imageInput = bytes.NewReader(previewData)
		closeFunc = func() error { return nil }
	} else {
		f, err := os.Open(fullPath)
		if err != nil {
			return fmt.Errorf("open photo for ML retry: %w", err)
		}
		imageInput = f
		closeFunc = f.Close
	}
	defer closeFunc()

	// CLIP: process_clip queue
	if taskSet["process_clip"] && mlConfig.CLIPEnabled {
		clipData, err := imaging.ProcessImageStream(imageInput, bimg.Options{
			Width:     224,
			Height:    224,
			Crop:      true,
			Gravity:   bimg.GravitySmart,
			Quality:   90,
			Type:      bimg.WEBP,
			NoProfile: true,
		})
		if err != nil {
			return fmt.Errorf("CLIP preprocessing for retry: %w", err)
		}

		_, err = ap.queueClient.Insert(ctx, jobs.ProcessClipArgs{
			AssetID:   asset.AssetID,
			ImageData: clipData,
		}, &river.InsertOpts{Queue: "process_clip"})
		if err != nil {
			return fmt.Errorf("enqueue process_clip retry: %w", err)
		}
	}

	// OCR: process_ocr queue
	if taskSet["process_ocr"] && mlConfig.OCREnabled {
		// Reset reader
		if previewData != nil {
			imageInput = bytes.NewReader(previewData)
		} else {
			f, err := os.Open(fullPath)
			if err != nil {
				return fmt.Errorf("open photo for OCR retry: %w", err)
			}
			defer f.Close()
			imageInput = f
		}

		ocrData, err := imaging.ProcessImageStream(imageInput, bimg.Options{
			Width:     1920,
			Height:    1920,
			Quality:   90,
			Type:      bimg.JPEG,
			NoProfile: true,
		})
		if err != nil {
			return fmt.Errorf("OCR preprocessing for retry: %w", err)
		}

		_, err = ap.queueClient.Insert(ctx, jobs.ProcessOcrArgs{
			AssetID:   asset.AssetID,
			ImageData: ocrData,
		}, &river.InsertOpts{Queue: "process_ocr"})
		if err != nil {
			return fmt.Errorf("enqueue process_ocr retry: %w", err)
		}
	}

	// Caption: process_caption queue
	if taskSet["process_caption"] && mlConfig.CaptionEnabled {
		// Reset reader
		if previewData != nil {
			imageInput = bytes.NewReader(previewData)
		} else {
			f, err := os.Open(fullPath)
			if err != nil {
				return fmt.Errorf("open photo for caption retry: %w", err)
			}
			defer f.Close()
			imageInput = f
		}

		captionData, err := imaging.ProcessImageStream(imageInput, bimg.Options{
			Width:     1024,
			Height:    1024,
			Quality:   85,
			Type:      bimg.JPEG,
			NoProfile: true,
		})
		if err != nil {
			return fmt.Errorf("caption preprocessing for retry: %w", err)
		}

		_, err = ap.queueClient.Insert(ctx, jobs.ProcessCaptionArgs{
			AssetID:   asset.AssetID,
			ImageData: captionData,
		}, &river.InsertOpts{Queue: "process_caption"})
		if err != nil {
			return fmt.Errorf("enqueue process_caption retry: %w", err)
		}
	}

	// Face: process_face queue
	if taskSet["process_face"] && mlConfig.FaceEnabled {
		// Reset reader
		if previewData != nil {
			imageInput = bytes.NewReader(previewData)
		} else {
			f, err := os.Open(fullPath)
			if err != nil {
				return fmt.Errorf("open photo for face retry: %w", err)
			}
			defer f.Close()
			imageInput = f
		}

		faceData, err := imaging.ProcessImageStream(imageInput, bimg.Options{
			Width:     1920,
			Height:    1920,
			Quality:   90,
			Type:      bimg.JPEG,
			NoProfile: true,
		})
		if err != nil {
			return fmt.Errorf("face preprocessing for retry: %w", err)
		}

		_, err = ap.queueClient.Insert(ctx, jobs.ProcessFaceArgs{
			AssetID:   asset.AssetID,
			ImageData: faceData,
		}, &river.InsertOpts{Queue: "process_face"})
		if err != nil {
			return fmt.Errorf("enqueue process_face retry: %w", err)
		}
	}

	return nil
}

// RetryAssetTask retries a specific task for an asset
func (ap *AssetProcessor) RetryAssetTask(ctx context.Context, assetIDStr string, taskName string) error {
	return ap.RetryAsset(ctx, assetIDStr, []string{taskName})
}
