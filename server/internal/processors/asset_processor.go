package processors

import (
	"context"
	"fmt"
	"os"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/service"
	"server/internal/storage"
	"server/internal/utils/file"
	"time"
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
	mlService      service.MLService
	storageService storage.Storage
}

func NewAssetProcessor(assetService service.AssetService, mlService service.MLService, storageService storage.Storage) *AssetProcessor {
	return &AssetProcessor{
		assetService:   assetService,
		mlService:      mlService,
		storageService: storageService,
	}
}

func (ap *AssetProcessor) ProcessAsset(ctx context.Context, task AssetPayload) (*repo.Asset, error) {
	assetFile, err := os.Open(task.StagedPath)
	if err != nil {
		return nil, err
	}
	defer assetFile.Close() // Close the staged file

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
		SpecificMetadata: nil,
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
		//TODO: implement
		return asset, ap.processVideoAsset(asset)
	case string(dbtypes.AssetTypeAudio):
		//TODO: implement
		return asset, ap.processAudioAsset(asset)
	default:
		return asset, fmt.Errorf("unsupported asset type: %s", asset.Type)
	}
}
