package processors

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/riverqueue/river"

	"server/internal/db/dbtypes"
	"server/internal/db/dbtypes/status"
	"server/internal/db/repo"
	"server/internal/queue/jobs"
	"server/internal/storage"
	"server/internal/utils/file"
)

// IngestAsset performs initial staging validation, asset record creation, commits the
// file to repository inbox, and enqueues downstream tasks.
func (ap *AssetProcessor) IngestAsset(ctx context.Context, task AssetPayload) (*repo.Asset, error) {
	// Resolve repository
	repository, err := ap.resolveRepository(ctx, task.RepositoryID)
	if err != nil {
		return nil, err
	}

	// Validate file
	validation := file.ValidateFile(task.FileName, task.ContentType)
	if !validation.Valid {
		return nil, fmt.Errorf("file validation failed: %s", validation.ErrorReason)
	}
	contentType := validation.AssetType

	// Staging file check
	info, err := os.Stat(task.StagedPath)
	if err != nil {
		return nil, fmt.Errorf("staged file not found: %w", err)
	}
	fileSize := info.Size()

	// Prepare staging file struct
	stagingFile := &storage.StagingFile{
		ID:        filepath.Base(task.StagedPath),
		RepoPath:  repository.Path,
		Path:      task.StagedPath,
		Filename:  task.FileName,
		CreatedAt: task.Timestamp,
	}

	// Initial status
	initialStatus := status.NewProcessingStatus("Asset ingestion started")
	statusJSON, err := initialStatus.ToJSONB()
	if err != nil {
		return nil, fmt.Errorf("marshal status: %w", err)
	}

	// Owner handling (anonymous → nil)
	var ownerIDPtr *int32
	if task.UserID != "anonymous" {
		ownerID := int32(1)
		ownerIDPtr = &ownerID
	}

	// Create asset record with empty storage path
	asset, err := ap.assetService.CreateAssetRecord(ctx, repo.CreateAssetParams{
		OwnerID:          ownerIDPtr,
		Type:             string(contentType),
		OriginalFilename: task.FileName,
		StoragePath:      nil,
		MimeType:         task.ContentType,
		FileSize:         fileSize,
		Hash:             &task.ClientHash,
		TakenTime:        pgtype.Timestamptz{Time: time.Now(), Valid: true},
		Rating:           func() *int32 { r := int32(0); return &r }(),
		RepositoryID:     repository.RepoID,
		Status:           statusJSON,
	})
	if err != nil {
		return nil, fmt.Errorf("create asset: %w", err)
	}

	// Commit file to inbox now so downstream workers can access it
	storageRelPath, err := ap.stagingManager.CommitStagingFileToInbox(stagingFile, "")
	if err != nil {
		return nil, fmt.Errorf("commit staging: %w", err)
	}

	// Update storage path + keep status processing
	_, err = ap.queries.UpdateAssetStoragePathAndStatus(ctx, repo.UpdateAssetStoragePathAndStatusParams{
		AssetID:     asset.AssetID,
		StoragePath: &storageRelPath,
		Status:      statusJSON,
	})
	if err != nil {
		return nil, fmt.Errorf("update asset storage path: %w", err)
	}

	// Enqueue downstream tasks
	pgID := asset.AssetID
	assetType := dbtypes.AssetType(asset.Type)
	commonMeta := jobs.MetadataArgs{
		AssetID:          pgID,
		RepoPath:         repository.Path,
		StoragePath:      storageRelPath,
		AssetType:        assetType,
		OriginalFilename: asset.OriginalFilename,
		FileSize:         asset.FileSize,
		MimeType:         asset.MimeType,
	}
	commonThumb := jobs.ThumbnailArgs{
		AssetID:     pgID,
		RepoPath:    repository.Path,
		StoragePath: storageRelPath,
		AssetType:   assetType,
	}
	commonTranscode := jobs.TranscodeArgs{
		AssetID:     pgID,
		RepoPath:    repository.Path,
		StoragePath: storageRelPath,
		AssetType:   assetType,
	}

	// Always enqueue metadata first
	_, err = ap.queueClient.Insert(ctx, commonMeta, &river.InsertOpts{Queue: "metadata_asset"})
	if err != nil {
		return nil, fmt.Errorf("enqueue metadata: %w", err)
	}

	switch assetType {
	case dbtypes.AssetTypePhoto:
		_, err = ap.queueClient.Insert(ctx, commonThumb, &river.InsertOpts{Queue: "thumbnail_asset"})
		if err != nil {
			return nil, fmt.Errorf("enqueue thumbnails: %w", err)
		}

		// Enqueue ML jobs directly for photos (decoupled from metadata)
		fullPath := filepath.Join(repository.Path, storageRelPath)
		err = ap.enqueueMLJobs(ctx, asset, fullPath)
		if err != nil {
			return nil, fmt.Errorf("enqueue ML jobs: %w", err)
		}

	case dbtypes.AssetTypeVideo:
		_, err = ap.queueClient.Insert(ctx, commonThumb, &river.InsertOpts{Queue: "thumbnail_asset"})
		if err != nil {
			return nil, fmt.Errorf("enqueue thumbnails: %w", err)
		}
		_, err = ap.queueClient.Insert(ctx, commonTranscode, &river.InsertOpts{Queue: "transcode_asset"})
		if err != nil {
			return nil, fmt.Errorf("enqueue transcode: %w", err)
		}
	case dbtypes.AssetTypeAudio:
		_, err = ap.queueClient.Insert(ctx, commonTranscode, &river.InsertOpts{Queue: "transcode_asset"})
		if err != nil {
			return nil, fmt.Errorf("enqueue transcode: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported asset type: %s", assetType)
	}

	return asset, nil
}

// resolveRepository resolves repository by ID or fallback to first available.
func (ap *AssetProcessor) resolveRepository(ctx context.Context, repositoryID string) (repo.Repository, error) {
	if repositoryID != "" {
		repoUUID, err := uuid.Parse(repositoryID)
		if err != nil {
			return repo.Repository{}, fmt.Errorf("invalid repository ID: %w", err)
		}
		repoUUIDPg := pgtype.UUID{Bytes: repoUUID, Valid: true}
		return ap.queries.GetRepository(ctx, repoUUIDPg)
	}

	repositories, err := ap.queries.ListRepositories(ctx)
	if err != nil || len(repositories) == 0 {
		return repo.Repository{}, fmt.Errorf("no repository available: %w", err)
	}
	return repositories[0], nil
}
