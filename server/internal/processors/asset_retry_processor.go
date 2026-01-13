package processors

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"server/config"
	"server/internal/db/dbtypes"
	"server/internal/db/dbtypes/status"
	"server/internal/db/repo"
	"server/internal/service"
	"server/internal/storage"
	"server/internal/utils/errgroup"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/riverqueue/river"
)

// AssetRetryPayload represents the payload for selective asset retry
type AssetRetryPayload struct {
	AssetID        string   `json:"assetId" river:"unique"`
	RetryTasks     []string `json:"retryTasks,omitempty"` // Empty means retry all failed tasks
	ForceFullRetry bool     `json:"forceFullRetry,omitempty"`
}

func (AssetRetryPayload) Kind() string { return "retry_asset" }

// AssetRetryProcessor handles selective retry of failed asset processing tasks
type AssetRetryProcessor struct {
	assetService   service.AssetService
	queries        *repo.Queries
	repoManager    storage.RepositoryManager
	stagingManager storage.StagingManager
	queueClient    *river.Client[pgx.Tx]
	appConfig      config.AppConfig
}

func NewAssetRetryProcessor(
	assetService service.AssetService,
	queries *repo.Queries,
	repoManager storage.RepositoryManager,
	stagingManager storage.StagingManager,
	queueClient *river.Client[pgx.Tx],
	appConfig config.AppConfig,
) *AssetRetryProcessor {
	return &AssetRetryProcessor{
		assetService:   assetService,
		queries:        queries,
		repoManager:    repoManager,
		stagingManager: stagingManager,
		queueClient:    queueClient,
		appConfig:      appConfig,
	}
}

// RetryAsset handles selective retry of failed asset processing tasks
func (arp *AssetRetryProcessor) RetryAsset(ctx context.Context, task AssetRetryPayload) error {
	// Parse asset ID
	assetID, err := uuid.Parse(task.AssetID)
	if err != nil {
		return fmt.Errorf("invalid asset ID: %w", err)
	}

	// Get asset from database
	pgUUID := pgtype.UUID{}
	if err := pgUUID.Scan(assetID.String()); err != nil {
		return fmt.Errorf("invalid asset ID format: %w", err)
	}

	asset, err := arp.queries.GetAssetByID(ctx, pgUUID)
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

	// Check if asset is retryable
	if !currentStatus.IsRetryable() {
		return fmt.Errorf("asset is not in a retryable state")
	}

	// Check for fatal errors
	if currentStatus.HasFatalErrors() {
		return fmt.Errorf("asset has fatal errors that prevent reprocessing")
	}

	// Get repository information
	repository, err := arp.queries.GetRepository(ctx, asset.RepositoryID)
	if err != nil {
		return fmt.Errorf("failed to get repository: %w", err)
	}

	// Check if storage path exists
	if asset.StoragePath == nil || *asset.StoragePath == "" {
		return fmt.Errorf("asset has no storage path")
	}

	// Resolve the full path to the asset file
	assetPath := filepath.Join(repository.Path, *asset.StoragePath)

	// Check if the file exists
	if _, err := os.Stat(assetPath); os.IsNotExist(err) {
		return fmt.Errorf("asset file not found")
	}

	// Update status to processing for retry
	retryStatus := status.NewProcessingStatus("Selective retry in progress")
	retryStatusJSON, err := retryStatus.ToJSONB()
	if err != nil {
		return fmt.Errorf("failed to marshal retry status: %w", err)
	}

	_, err = arp.queries.UpdateAssetStatus(ctx, repo.UpdateAssetStatusParams{
		AssetID: asset.AssetID,
		Status:  retryStatusJSON,
	})
	if err != nil {
		return fmt.Errorf("failed to update asset status: %w", err)
	}

	// Determine which tasks to retry
	tasksToRetry := task.RetryTasks
	if len(tasksToRetry) == 0 || task.ForceFullRetry {
		// If no specific tasks requested or force full retry, retry all failed tasks
		tasksToRetry = currentStatus.GetFailedTasks()
	}

	// Process the asset with selective retry
	err = arp.processAssetWithSelectiveRetry(ctx, repository, &asset, assetPath, tasksToRetry, currentStatus)
	if err != nil {
		// Update status to failed
		failedStatus := status.NewFailedStatus("Selective retry failed", []status.ErrorDetail{
			{Task: "selective_retry", Error: err.Error()},
		})
		failedStatusJSON, _ := failedStatus.ToJSONB()
		_, _ = arp.queries.UpdateAssetStatus(ctx, repo.UpdateAssetStatusParams{
			AssetID: asset.AssetID,
			Status:  failedStatusJSON,
		})
		return err
	}

	return nil
}

// processAssetWithSelectiveRetry handles selective retry of specific tasks
func (arp *AssetRetryProcessor) processAssetWithSelectiveRetry(
	ctx context.Context,
	repository repo.Repository,
	asset *repo.Asset,
	assetPath string,
	tasksToRetry []string,
	originalStatus status.AssetStatus,
) error {
	// Open the asset file for processing
	assetFile, err := os.Open(assetPath)
	if err != nil {
		return fmt.Errorf("failed to open asset file: %w", err)
	}
	defer assetFile.Close()

	// Process based on asset type and retry only specified tasks
	var processingErrors []status.ErrorDetail

	switch asset.Type {
	case string(dbtypes.AssetTypePhoto):
		err := arp.retryPhotoTasks(ctx, repository, asset, assetFile, tasksToRetry)
		if err != nil {
			processingErrors = append(processingErrors, status.ErrorDetail{
				Task:  "photo_retry",
				Error: err.Error(),
			})
		}
	case string(dbtypes.AssetTypeVideo):
		err := arp.retryVideoTasks(ctx, repository, asset, assetFile, tasksToRetry)
		if err != nil {
			processingErrors = append(processingErrors, status.ErrorDetail{
				Task:  "video_retry",
				Error: err.Error(),
			})
		}
	case string(dbtypes.AssetTypeAudio):
		err := arp.retryAudioTasks(ctx, repository, asset, assetFile, tasksToRetry)
		if err != nil {
			processingErrors = append(processingErrors, status.ErrorDetail{
				Task:  "audio_retry",
				Error: err.Error(),
			})
		}
	default:
		return fmt.Errorf("unsupported asset type for retry: %s", asset.Type)
	}

	// Combine original errors (excluding retried tasks) with new errors
	finalErrors := arp.mergeErrors(originalStatus.Errors, processingErrors, tasksToRetry)

	// Determine final status
	var finalStatus status.AssetStatus
	if len(finalErrors) == 0 {
		finalStatus = status.NewCompleteStatus()
	} else {
		finalStatus = status.NewWarningStatus("Asset retry completed with some errors", finalErrors)
	}

	// Update asset status
	statusJSON, err := finalStatus.ToJSONB()
	if err != nil {
		return fmt.Errorf("failed to marshal final status: %w", err)
	}

	_, err = arp.queries.UpdateAssetStatus(ctx, repo.UpdateAssetStatusParams{
		AssetID: asset.AssetID,
		Status:  statusJSON,
	})
	if err != nil {
		return fmt.Errorf("failed to update asset status: %w", err)
	}

	return nil
}

// retryPhotoTasks selectively retries photo processing tasks
func (arp *AssetRetryProcessor) retryPhotoTasks(
	ctx context.Context,
	repository repo.Repository,
	asset *repo.Asset,
	fileReader io.Reader,
	tasksToRetry []string,
) error {
	// Check if we need to retry metadata extraction
	shouldRetryMetadata := arp.shouldRetryTask("extract_exif", tasksToRetry) ||
		arp.shouldRetryTask("extract_metadata", tasksToRetry)

	// Check if we need to retry thumbnail generation
	shouldRetryThumbnails := arp.shouldRetryTask("generate_thumbnails", tasksToRetry) ||
		arp.shouldRetryTask("save_thumbnails", tasksToRetry)

	// Check if we need to retry CLIP processing
	shouldRetryCLIP := arp.shouldRetryTask("clip_processing", tasksToRetry)

	g := errgroup.NewFaultTolerant()

	// Retry metadata extraction if needed
	if shouldRetryMetadata {
		g.Go(func() error {
			return arp.retryPhotoMetadata(ctx, asset, fileReader)
		})
	}

	// Retry thumbnail generation if needed
	if shouldRetryThumbnails {
		g.Go(func() error {
			return arp.retryPhotoThumbnails(ctx, repository, asset, fileReader)
		})
	}

	// Retry CLIP processing if needed and enabled
	if shouldRetryCLIP && arp.appConfig.MLConfig.CLIPEnabled {
		g.Go(func() error {
			return arp.retryPhotoCLIP(ctx, asset, fileReader)
		})
	}

	// Wait for all retry tasks to complete
	errors := g.Wait()
	if len(errors) > 0 {
		// Return the first error, individual errors are collected by the caller
		if len(errors) > 0 {
			return errors[0]
		}
	}

	return nil
}

// retryVideoTasks selectively retries video processing tasks
func (arp *AssetRetryProcessor) retryVideoTasks(
	ctx context.Context,
	repository repo.Repository,
	asset *repo.Asset,
	fileReader io.Reader,
	tasksToRetry []string,
) error {
	// Implementation similar to retryPhotoTasks but for video
	// This would retry metadata extraction, transcoding, and thumbnail generation
	// based on the tasksToRetry list

	// For now, return a placeholder implementation
	return fmt.Errorf("video task retry not yet implemented")
}

// retryAudioTasks selectively retries audio processing tasks
func (arp *AssetRetryProcessor) retryAudioTasks(
	ctx context.Context,
	repository repo.Repository,
	asset *repo.Asset,
	fileReader io.Reader,
	tasksToRetry []string,
) error {
	// Implementation similar to retryPhotoTasks but for audio
	// This would retry metadata extraction and transcoding
	// based on the tasksToRetry list

	// For now, return a placeholder implementation
	return fmt.Errorf("audio task retry not yet implemented")
}

// retryPhotoMetadata retries photo metadata extraction
func (arp *AssetRetryProcessor) retryPhotoMetadata(ctx context.Context, asset *repo.Asset, fileReader io.Reader) error {
	// Implementation would use the existing photo processor's metadata extraction logic
	// but only for the retry case
	return fmt.Errorf("photo metadata retry not yet implemented")
}

// retryPhotoThumbnails retries photo thumbnail generation
func (arp *AssetRetryProcessor) retryPhotoThumbnails(ctx context.Context, repository repo.Repository, asset *repo.Asset, fileReader io.Reader) error {
	// Implementation would use the existing photo processor's thumbnail generation logic
	// but only for the retry case
	return fmt.Errorf("photo thumbnail retry not yet implemented")
}

// retryPhotoCLIP retries CLIP processing for photos
func (arp *AssetRetryProcessor) retryPhotoCLIP(ctx context.Context, asset *repo.Asset, fileReader io.Reader) error {
	// Implementation would use the existing CLIP processing logic
	// but only for the retry case
	return fmt.Errorf("photo CLIP retry not yet implemented")
}

// shouldRetryTask checks if a task should be retried based on the task list
func (arp *AssetRetryProcessor) shouldRetryTask(taskName string, tasksToRetry []string) bool {
	if len(tasksToRetry) == 0 {
		return true // Retry all if no specific tasks specified
	}

	for _, task := range tasksToRetry {
		if task == taskName {
			return true
		}
	}
	return false
}

// mergeErrors combines original errors with new errors, excluding retried tasks
func (arp *AssetRetryProcessor) mergeErrors(originalErrors, newErrors []status.ErrorDetail, retriedTasks []string) []status.ErrorDetail {
	// Create a set of retried task names for efficient lookup
	retriedTaskSet := make(map[string]bool)
	for _, task := range retriedTasks {
		retriedTaskSet[task] = true
	}

	// Filter original errors: keep only those that were not retried
	finalErrors := make([]status.ErrorDetail, 0)
	for _, err := range originalErrors {
		if !retriedTaskSet[err.Task] {
			finalErrors = append(finalErrors, err)
		}
	}

	// Add new errors from the retry
	finalErrors = append(finalErrors, newErrors...)

	return finalErrors
}
