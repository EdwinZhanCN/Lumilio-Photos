package processors

import (
	"context"
	"fmt"
	"os"
	"server/config"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/service"
	"server/internal/storage"
	"server/internal/utils/file"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/riverqueue/river"
)

type AssetPayload struct {
	ClientHash  string    `json:"clientHash" river:"unique"`
	StagedPath  string    `json:"stagedPath"`
	UserID      string    `json:"userId" river:"unique"`
	Timestamp   time.Time `json:"timestamp"`
	ContentType string    `json:"contentType,omitempty"`
	FileName    string    `json:"fileName,omitempty"`
}

// AssetProcessor handles processing tasks for different asset types
type AssetProcessor struct {
	assetService   service.AssetService
	storageService storage.Storage
	queueClient    *river.Client[pgx.Tx]
	appConfig      config.AppConfig
}

func NewAssetProcessor(assetService service.AssetService, storageService storage.Storage, queueClient *river.Client[pgx.Tx], appConfig config.AppConfig) *AssetProcessor {
	return &AssetProcessor{
		assetService:   assetService,
		storageService: storageService,
		queueClient:    queueClient,
		appConfig:      appConfig,
	}
}

func (ap *AssetProcessor) ProcessAsset(ctx context.Context, task AssetPayload) (*repo.Asset, error) {
	assetFile, err := os.Open(task.StagedPath)
	if err != nil {
		return nil, err
	}
	defer func(assetFile *os.File) {
		err := assetFile.Close()
		if err != nil {
			fmt.Printf("failed to close staged file: %v\n", err)
		}
	}(assetFile) // Close the staged file

	info, err := assetFile.Stat()
	if err != nil {
		return nil, err
	}
	fileSize := info.Size()

	var ownerIDPtr *int32
	if task.UserID != "anonymous" {
		ownerID := int32(1)
		ownerIDPtr = &ownerID
	}

	contentType := file.DetermineAssetType(task.ContentType)

	// Commit the staged file FIRST
	storagePath, err := ap.storageService.CommitStagedFile(ctx, task.StagedPath, task.FileName, task.ClientHash)
	if err != nil {
		return nil, err
	}

	// Create asset record
	params := repo.CreateAssetParams{
		OwnerID:          ownerIDPtr,
		Type:             string(contentType),
		OriginalFilename: task.FileName,
		StoragePath:      storagePath,
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
		return nil, err
	}

	switch asset.Type {
	case string(dbtypes.AssetTypePhoto):
		err := ap.processPhotoAsset(ctx, asset, assetFile)
		return asset, err
	case string(dbtypes.AssetTypeVideo):
		err := ap.processVideoAsset(ctx, asset, assetFile)
		return asset, err
	case string(dbtypes.AssetTypeAudio):
		err := ap.processAudioAsset(ctx, asset, assetFile)
		return asset, err
	default:
		return asset, fmt.Errorf("unsupported asset type: %s", asset.Type)
	}
}
