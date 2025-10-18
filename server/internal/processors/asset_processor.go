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
	"server/internal/utils/file"
	"server/internal/utils/raw"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/riverqueue/river"
)

type AssetPayload struct {
	ClientHash   string    `json:"clientHash" river:"unique"`
	StagedPath   string    `json:"stagedPath"`
	UserID       string    `json:"userId" river:"unique"`
	Timestamp    time.Time `json:"timestamp"`
	ContentType  string    `json:"contentType,omitempty"`
	FileName     string    `json:"fileName,omitempty"`
	RepositoryID string    `json:"repositoryId,omitempty"` // Repository UUID
}

// AssetProcessor handles processing tasks for different asset types
type AssetProcessor struct {
	assetService   service.AssetService
	queries        *repo.Queries
	repoManager    storage.RepositoryManager
	stagingManager storage.StagingManager
	queueClient    *river.Client[pgx.Tx]
	appConfig      config.AppConfig
}

func NewAssetProcessor(
	assetService service.AssetService,
	queries *repo.Queries,
	repoManager storage.RepositoryManager,
	stagingManager storage.StagingManager,
	queueClient *river.Client[pgx.Tx],
	appConfig config.AppConfig,
) *AssetProcessor {
	return &AssetProcessor{
		assetService:   assetService,
		queries:        queries,
		repoManager:    repoManager,
		stagingManager: stagingManager,
		queueClient:    queueClient,
		appConfig:      appConfig,
	}
}

func (ap *AssetProcessor) ProcessAsset(ctx context.Context, task AssetPayload) (*repo.Asset, error) {
	// Get repository information first
	var repository repo.Repository
	if task.RepositoryID != "" {
		repoUUID, err := uuid.Parse(task.RepositoryID)
		if err != nil {
			return nil, fmt.Errorf("invalid repository ID: %w", err)
		}
		repoUUIDPgtype := pgtype.UUID{Bytes: repoUUID, Valid: true}
		repository, err = ap.queries.GetRepository(ctx, repoUUIDPgtype)
		if err != nil {
			return nil, fmt.Errorf("repository not found: %w", err)
		}
	} else {
		// Fallback: get default repository or first repository for user
		repositories, err := ap.queries.ListRepositories(ctx)
		if err != nil || len(repositories) == 0 {
			return nil, fmt.Errorf("no repository available: %w", err)
		}
		repository = repositories[0]
	}

	var ownerIDPtr *int32
	if task.UserID != "anonymous" {
		ownerID := int32(1)
		ownerIDPtr = &ownerID
	}

	// Validate file first
	validationResult := file.ValidateFile(task.FileName, task.ContentType)
	if !validationResult.Valid {
		return nil, fmt.Errorf("file validation failed: %s", validationResult.ErrorReason)
	}
	contentType := validationResult.AssetType

	// Check if file exists in staging
	var (
		fileInfo    os.FileInfo
		fileSize    int64
		stagingFile *storage.StagingFile
	)

	fmt.Printf("AssetProcessor: Checking staged file at path: %s\n", task.StagedPath)
	if info, err := os.Stat(task.StagedPath); err == nil {
		fileInfo = info
		fileSize = fileInfo.Size()
		fmt.Printf("AssetProcessor: Found staged file, size: %d bytes\n", fileSize)

		// Create staging file structure
		stagingFile = &storage.StagingFile{
			ID:        filepath.Base(task.StagedPath),
			RepoPath:  repository.Path,
			Path:      task.StagedPath,
			Filename:  task.FileName,
			CreatedAt: task.Timestamp,
		}
		fmt.Printf("AssetProcessor: Created staging file structure for: %s\n", task.FileName)
	} else {
		fmt.Printf("AssetProcessor: ERROR - Staged file not found at %s: %v\n", task.StagedPath, err)
		return nil, fmt.Errorf("staged file not found: %w", err)
	}

	// Create asset record with NULL storage_path initially
	initialStatus := status.NewProcessingStatus("Asset processing started")
	statusJSON, err := initialStatus.ToJSONB()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal initial status: %w", err)
	}

	params := repo.CreateAssetParams{
		OwnerID:          ownerIDPtr,
		Type:             string(contentType),
		OriginalFilename: task.FileName,
		StoragePath:      nil, // Will be set after successful processing
		MimeType:         task.ContentType,
		FileSize:         fileSize,
		Hash:             &task.ClientHash,
		Width:            nil,
		Height:           nil,
		Duration:         nil,
		TakenTime:        pgtype.Timestamptz{Time: time.Now(), Valid: true},
		SpecificMetadata: nil,
		Rating:           func() *int32 { r := int32(0); return &r }(),
		Liked:            nil,
		RepositoryID:     pgtype.UUID{Bytes: repository.RepoID.Bytes, Valid: true},
		Status:           statusJSON,
	}

	asset, err := ap.assetService.CreateAssetRecord(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to create asset record: %w", err)
	}

	// Process the asset with comprehensive error handling
	err = ap.processAssetWithStatus(ctx, repository, asset, stagingFile)
	if err != nil {
		// If processing failed completely, move file to failed directory and update status
		if moveErr := ap.stagingManager.MoveStagingToFailed(stagingFile); moveErr != nil {
			// Log the error but don't fail the entire process
			fmt.Printf("Failed to move file to failed directory: %v\n", moveErr)
		}

		// Update status to failed
		failedStatus := status.NewFailedStatus("Asset processing failed", []status.ErrorDetail{
			{Task: "process_asset", Error: err.Error()},
		})
		statusJSON, _ := failedStatus.ToJSONB()
		_, _ = ap.queries.UpdateAssetStatus(ctx, repo.UpdateAssetStatusParams{
			AssetID: asset.AssetID,
			Status:  statusJSON,
		})

		return asset, err
	}

	return asset, nil
}

// processAssetWithStatus handles the main processing logic with status tracking
func (ap *AssetProcessor) processAssetWithStatus(ctx context.Context, repository repo.Repository, asset *repo.Asset, stagingFile *storage.StagingFile) error {
	fmt.Printf("processAssetWithStatus: Opening staging file: %s\n", stagingFile.Path)
	// Open the staging file for processing
	assetFile, err := os.Open(stagingFile.Path)
	if err != nil {
		fmt.Printf("processAssetWithStatus: ERROR - Failed to open staging file %s: %v\n", stagingFile.Path, err)
		return fmt.Errorf("failed to open staging file: %w", err)
	}
	defer assetFile.Close()

	// Process based on asset type and collect errors
	var processingErrors []status.ErrorDetail

	switch asset.Type {
	case string(dbtypes.AssetTypePhoto):
		photoErrors := ap.processPhotoAssetWithErrors(ctx, repository, asset, assetFile)
		processingErrors = append(processingErrors, photoErrors...)
	case string(dbtypes.AssetTypeVideo):
		videoErrors := ap.processVideoAssetWithErrors(ctx, repository, asset, assetFile)
		processingErrors = append(processingErrors, videoErrors...)
	case string(dbtypes.AssetTypeAudio):
		audioErrors := ap.processAudioAssetWithErrors(ctx, repository, asset, assetFile)
		processingErrors = append(processingErrors, audioErrors...)
	default:
		return fmt.Errorf("unsupported asset type: %s", asset.Type)
	}

	// Determine final status based on errors
	var finalStatus status.AssetStatus
	var finalStoragePath string

	if len(processingErrors) == 0 {
		// All processing succeeded - commit to inbox
		fmt.Printf("processAssetWithStatus: Committing file to inbox: %s\n", stagingFile.Path)
		finalRelPath, err := ap.stagingManager.CommitStagingFileToInbox(stagingFile, "")
		if err != nil {
			fmt.Printf("processAssetWithStatus: ERROR - Failed to commit file to inbox: %v\n", err)
			return fmt.Errorf("failed to commit file to inbox: %w", err)
		}
		fmt.Printf("processAssetWithStatus: Successfully committed file to: %s\n", finalRelPath)
		finalStoragePath = finalRelPath
		finalStatus = status.NewCompleteStatus()
	} else {
		// Some processing failed - commit to inbox but mark as warning
		fmt.Printf("processAssetWithStatus: Committing file to inbox with warnings: %s\n", stagingFile.Path)
		finalRelPath, err := ap.stagingManager.CommitStagingFileToInbox(stagingFile, "")
		if err != nil {
			fmt.Printf("processAssetWithStatus: ERROR - Failed to commit file to inbox: %v\n", err)
			return fmt.Errorf("failed to commit file to inbox: %w", err)
		}
		fmt.Printf("processAssetWithStatus: Successfully committed file to: %s (with warnings)\n", finalRelPath)
		finalStoragePath = finalRelPath
		finalStatus = status.NewWarningStatus("Asset processed with warnings", processingErrors)
	}

	// Update asset with final storage path and status
	statusJSON, err := finalStatus.ToJSONB()
	if err != nil {
		return fmt.Errorf("failed to marshal final status: %w", err)
	}

	_, err = ap.queries.UpdateAssetStoragePathAndStatus(ctx, repo.UpdateAssetStoragePathAndStatusParams{
		AssetID:     asset.AssetID,
		StoragePath: &finalStoragePath,
		Status:      statusJSON,
	})
	if err != nil {
		return fmt.Errorf("failed to update asset storage path and status: %w", err)
	}

	return nil
}

// processPhotoAssetWithErrors processes photo assets and returns detailed error information
func (ap *AssetProcessor) processPhotoAssetWithErrors(ctx context.Context, repository repo.Repository, asset *repo.Asset, fileReader io.Reader) []status.ErrorDetail {
	var errors []status.ErrorDetail

	// First check if this is a RAW file
	isRAWFile := raw.IsRAWFile(asset.OriginalFilename)

	if isRAWFile {
		if err := ap.processRAWAsset(ctx, repository, asset, fileReader); err != nil {
			errors = append(errors, status.ErrorDetail{
				Task:  "raw_processing",
				Error: err.Error(),
			})
		}
	} else {
		if err := ap.processStandardPhotoAsset(ctx, repository, asset, fileReader); err != nil {
			errors = append(errors, status.ErrorDetail{
				Task:  "photo_processing",
				Error: err.Error(),
			})
		}
	}

	return errors
}

// processVideoAssetWithErrors processes video assets and returns detailed error information
func (ap *AssetProcessor) processVideoAssetWithErrors(ctx context.Context, repository repo.Repository, asset *repo.Asset, fileReader io.Reader) []status.ErrorDetail {
	var errors []status.ErrorDetail

	if err := ap.processVideoAsset(ctx, repository, asset, fileReader); err != nil {
		errors = append(errors, status.ErrorDetail{
			Task:  "video_processing",
			Error: err.Error(),
		})
	}

	return errors
}

// processAudioAssetWithErrors processes audio assets and returns detailed error information
func (ap *AssetProcessor) processAudioAssetWithErrors(ctx context.Context, repository repo.Repository, asset *repo.Asset, fileReader io.Reader) []status.ErrorDetail {
	var errors []status.ErrorDetail

	if err := ap.processAudioAsset(ctx, repository, asset, fileReader); err != nil {
		errors = append(errors, status.ErrorDetail{
			Task:  "audio_processing",
			Error: err.Error(),
		})
	}

	return errors
}
