package processors

import (
	"context"
	"fmt"
	"io"
	"os"
	"server/internal/models"
	"server/internal/service"
	"server/internal/utils/file"
	"time"

	"github.com/google/uuid"
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
	assetService service.AssetService
	mlService    service.MLService
}

func NewAssetProcessor(assetService service.AssetService, mlService service.MLService) *AssetProcessor {
	return &AssetProcessor{
		assetService: assetService,
		mlService:    mlService,
	}
}

type Job struct {
	AssetID   uuid.UUID
	AssetType models.AssetType

	State   JobState
	Payload io.Reader

	ErrorHistory []error
	CurrentStep  string
}

type JobState string

const (
	StatePendingExif          JobState = "pending_exif"
	StateProcessingExif       JobState = "processing_exif"
	StatePendingThumb         JobState = "pending_thumbnail"
	StateProcessingThumb      JobState = "processing_thumbnail"
	StatePendingML            JobState = "pending_ml"
	StateProcessingML         JobState = "processing_ml"
	StatePendingStorage       JobState = "pending_storage"
	StateProcessingStorage    JobState = "processing_storage"
	StatePendingDB            JobState = "pending_db"
	StateProcessingDBJobState JobState = "processing_db"
	StateCompleted            JobState = "completed"
	StateFailed               JobState = "failed"
)

// ProcessAsset processes an asset based on its type
func (ap *AssetProcessor) ProcessAsset(ctx context.Context, task AssetPayload) (*models.Asset, error) {
	assetFile, err := os.Open(task.StagedPath)
	info, _ := assetFile.Stat()
	fileSize := info.Size()

	var ownerIDPtr *int
	if task.UserID != "anonymous" {
		ownerID := 1
		ownerIDPtr = &ownerID
	}
	if err != nil {
		return nil, err
	}

	contentType := file.DetermineAssetType(task.ContentType)

	storagePath, err := ap.assetService.SaveNewAsset(ctx, assetFile, task.FileName, task.ContentType)

	// 重置文件指针到开头
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
	newAsset := &models.Asset{
		AssetID:          uuid.New(),
		OwnerID:          ownerIDPtr,
		Type:             contentType,
		OriginalFilename: task.FileName,
		StoragePath:      storagePath,
		MimeType:         task.ContentType,
		FileSize:         fileSize,
		Hash:             task.ClientHash,
		UploadTime:       task.Timestamp,
	}

	asset, err := ap.assetService.CreateAssetRecord(ctx, newAsset)

	if err != nil {
		return nil, err
	}

	switch asset.Type {
	case models.AssetTypePhoto:
		// TODO: Remove Staged file
		return asset, ap.processPhotoAsset(ctx, asset, assetFile)
	case models.AssetTypeVideo:
		//TODO: implement
		return asset, ap.processVideoAsset(asset)
	case models.AssetTypeAudio:
		//TODO: implement
		return asset, ap.processAudioAsset(asset)
	default:
		return asset, fmt.Errorf("unsupported asset type: %s", asset.Type)
	}
}
