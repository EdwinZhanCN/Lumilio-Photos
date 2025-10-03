package processors

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"server/config"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/service"
	"server/internal/storage"
	"server/internal/utils/file"
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
	// Verify staged file exists
	info, err := os.Stat(task.StagedPath)
	if err != nil {
		return nil, fmt.Errorf("staged file not found: %w", err)
	}
	fileSize := info.Size()

	// Get repository information
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

	contentType := file.DetermineAssetType(task.ContentType)

	// Commit staged file to repository inbox using StagingManager
	// Convert task.StagedPath to StagingFile structure
	stagingFile := &storage.StagingFile{
		ID:        filepath.Base(task.StagedPath),
		RepoPath:  repository.Path,
		Path:      task.StagedPath,
		Filename:  task.FileName,
		CreatedAt: task.Timestamp,
	}

	// Commit to inbox based on repository configuration
	err = ap.stagingManager.CommitStagingFileToInbox(stagingFile, task.ClientHash)
	if err != nil {
		return nil, fmt.Errorf("failed to commit staged file to inbox: %w", err)
	}

	// Resolve the final storage path (relative to repository root)
	inboxPath, err := ap.stagingManager.ResolveInboxPath(repository.Path, task.FileName, task.ClientHash)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve inbox path: %w", err)
	}

	// Create asset record with repository-relative path
	// Note: repository_id will be added after migration is run
	params := repo.CreateAssetParams{
		OwnerID:          ownerIDPtr,
		Type:             string(contentType),
		OriginalFilename: task.FileName,
		StoragePath:      inboxPath, // Store relative path within repository
		MimeType:         task.ContentType,
		FileSize:         fileSize,
		Hash:             &task.ClientHash,
		Width:            nil,
		Height:           nil,
		Duration:         nil,
		TakenTime:        pgtype.Timestamptz{Time: time.Now(), Valid: true}, // Fallback to current time, will be updated when EXIF is processed
		SpecificMetadata: nil,
		Rating:           func() *int32 { r := int32(0); return &r }(),
		Liked:            nil,
	}

	asset, err := ap.assetService.CreateAssetRecord(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to create asset record: %w", err)
	}

	// Open the committed file for processing
	committedFilePath := filepath.Join(repository.Path, inboxPath)
	assetFile, err := os.Open(committedFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open committed file: %w", err)
	}
	defer assetFile.Close()

	// Process based on asset type
	switch asset.Type {
	case string(dbtypes.AssetTypePhoto):
		err := ap.processPhotoAsset(ctx, repository, asset, assetFile)
		return asset, err
	case string(dbtypes.AssetTypeVideo):
		err := ap.processVideoAsset(ctx, repository, asset, assetFile)
		return asset, err
	case string(dbtypes.AssetTypeAudio):
		err := ap.processAudioAsset(ctx, repository, asset, assetFile)
		return asset, err
	default:
		return asset, fmt.Errorf("unsupported asset type: %s", asset.Type)
	}
}
