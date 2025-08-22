package processors

import (
	"context"
	"fmt"
	"io"
	"os"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/queue"
	"server/internal/service"
	"server/internal/storage"
	"server/internal/utils/file"
	"time"
)

type AssetPayload struct {
	ClientHash  string    `json:"clientHash"`
	StagedPath  string    `json:"stagedPath"`
	UserID      string    `json:"userId"`
	Timestamp   time.Time `json:"timestamp"`
	ContentType string    `json:"contentType,omitempty"`
	FileName    string    `json:"fileName,omitempty"`
}

// AssetProcessor handles processing tasks for different asset types
type AssetProcessor struct {
	assetService   service.AssetService
	mlService      service.MLService
	storageService storage.Storage
	clipQueue      queue.Queue[CLIPPayload]
}

func NewAssetProcessor(assetService service.AssetService, mlService service.MLService, storageService storage.Storage, clipQueue queue.Queue[CLIPPayload]) *AssetProcessor {
	return &AssetProcessor{
		assetService:   assetService,
		mlService:      mlService,
		storageService: storageService,
		clipQueue:      clipQueue,
	}
}

func (ap *AssetProcessor) ProcessAsset(ctx context.Context, task AssetPayload) (*repo.Asset, error) {
	assetFile, err := os.Open(task.StagedPath)
	info, _ := assetFile.Stat()
	fileSize := info.Size()

	var ownerIDPtr *int32
	if task.UserID != "anonymous" {
		ownerID := int32(1)
		ownerIDPtr = &ownerID
	}
	if err != nil {
		return nil, err
	}

	contentType := file.DetermineAssetType(task.ContentType)

	storagePath, err := ap.storageService.CommitStagedFile(ctx, task.StagedPath, task.FileName, task.ClientHash)

	if _, err := assetFile.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("reset file pointer: %w", err)
	}

	if err != nil {
		return nil, err
	}

	// What we left here?
	// Width
	// Height
	// Duration
	// those field will update later
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
		return asset, ap.processPhotoAsset(ctx, asset, assetFile)
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
