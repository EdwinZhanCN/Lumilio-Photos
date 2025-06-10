package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"server/internal/models"
	"server/internal/service"
	"server/internal/storage"
	"strings"
	"time"

	"github.com/google/uuid"
)

// AssetProcessor handles processing tasks for different asset types
type AssetProcessor struct {
	// Dependencies for processing different asset types
	assetService service.AssetService
	storage      storage.Storage
	storagePath  string
}

// ProcessAsset processes an asset based on its type
func (ap *AssetProcessor) ProcessAsset(ctx context.Context, asset *models.Asset) error {
	switch asset.Type {
	case models.AssetTypePhoto:
		return ap.processPhoto(ctx, asset)
	case models.AssetTypeVideo:
		return ap.processVideo(ctx, asset)
	case models.AssetTypeAudio:
		return ap.processAudio(ctx, asset)
	case models.AssetTypeDocument:
		return ap.processDocument(ctx, asset)
	default:
		return fmt.Errorf("unsupported asset type: %s", asset.Type)
	}
}

// ProcessNewAsset processes a newly uploaded asset file
func (ap *AssetProcessor) ProcessNewAsset(stagedPath string, userID string, fileName string) (*models.Asset, error) {
	// Create a context
	ctx := context.Background()

	// Calculate hash for the file (content-addressable storage)
	hash, err := CalculateFileHash(stagedPath)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate file hash: %w", err)
	}

	// Determine asset type from filename
	assetType := determineAssetType(fileName)

	// Open the file
	file, err := os.Open(stagedPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open staged file: %w", err)
	}
	defer file.Close()

	// Upload to storage
	fileInfo, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	// Rewind file after getting stats
	if _, err := file.Seek(0, 0); err != nil {
		return nil, fmt.Errorf("failed to reset file position: %w", err)
	}

	// Parse owner ID if possible
	var ownerIDPtr *int
	if userID != "anonymous" {
		// In a real app, you'd convert the userID string to an int
		// Here we're just creating a placeholder
		ownerID := 1 // Default owner ID
		ownerIDPtr = &ownerID
	}

	// Create a new asset record
	asset := &models.Asset{
		AssetID:          uuid.New(),
		OwnerID:          ownerIDPtr,
		Type:             assetType,
		OriginalFilename: fileName,
		StoragePath:      stagedPath, // This will be updated when the file is moved to final storage
		MimeType:         getMimeTypeFromFileName(fileName),
		FileSize:         fileInfo.Size(),
		Hash:             hash,
		UploadTime:       time.Now(),
		IsDeleted:        false,
		SpecificMetadata: make(models.SpecificMetadata),
	}

	// Extract metadata if possible based on asset type
	if assetType == models.AssetTypePhoto {
		photoMetadata, err := ap.ExtractAssetMetadata(ctx, asset.AssetID.String(), stagedPath)
		if err == nil {
			// Convert photo metadata to generic specific metadata
			metadataJSON, _ := json.Marshal(photoMetadata)
			json.Unmarshal(metadataJSON, &asset.SpecificMetadata)
		}
	}

	// Save the asset to the database
	uploadedAsset, err := ap.assetService.UploadAsset(ctx, file, fileName, fileInfo.Size(), ownerIDPtr)
	if err != nil {
		return nil, fmt.Errorf("failed to save asset to database: %w", err)
	}

	// Use the uploaded asset instead of our local one
	asset = uploadedAsset

	// Return the asset
	return asset, nil
}

// GetPathForHash returns the directory path for a given hash
func (ap *AssetProcessor) GetPathForHash(hash string) string {
	// Use first 2/2/2 characters of the hash as directory structure
	if len(hash) < 6 {
		return ap.storagePath
	}

	return filepath.Join(ap.storagePath, hash[0:2], hash[2:4], hash[4:6])
}

// GetFullPathForHash returns the full file path for a given hash
func (ap *AssetProcessor) GetFullPathForHash(hash string) string {
	return filepath.Join(ap.GetPathForHash(hash), hash)
}

// determineAssetType determines the asset type from the filename extension
func determineAssetType(fileName string) models.AssetType {
	ext := filepath.Ext(fileName)
	ext = strings.ToLower(ext)

	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".webp", ".tiff", ".tif", ".heic":
		return models.AssetTypePhoto
	case ".mp4", ".mov", ".avi", ".wmv", ".webm", ".flv", ".mkv":
		return models.AssetTypeVideo
	case ".mp3", ".wav", ".ogg", ".flac", ".aac", ".m4a":
		return models.AssetTypeAudio
	case ".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx", ".txt", ".csv":
		return models.AssetTypeDocument
	default:
		return models.AssetTypePhoto // Default to photo if unknown
	}
}

// processPhoto handles photo-specific processing
func (ap *AssetProcessor) processPhoto(ctx context.Context, asset *models.Asset) error {
	// Extract photo metadata, generate thumbnails, etc.
	// This would integrate with existing photo processing logic
	return nil
}

// processVideo handles video-specific processing
func (ap *AssetProcessor) processVideo(ctx context.Context, asset *models.Asset) error {
	// Extract video metadata, generate thumbnails/previews, etc.
	return nil
}

// processAudio handles audio-specific processing
func (ap *AssetProcessor) processAudio(ctx context.Context, asset *models.Asset) error {
	// Extract audio metadata, generate waveforms, etc.
	return nil
}

// processDocument handles document-specific processing
func (ap *AssetProcessor) processDocument(ctx context.Context, asset *models.Asset) error {
	// Extract document metadata, generate previews, etc.
	return nil
}

// getMimeTypeFromFileName determines the MIME type from the filename extension
func getMimeTypeFromFileName(fileName string) string {
	ext := filepath.Ext(fileName)
	ext = strings.ToLower(ext)

	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".bmp":
		return "image/bmp"
	case ".webp":
		return "image/webp"
	case ".tiff", ".tif":
		return "image/tiff"
	case ".heic":
		return "image/heic"
	case ".mp4":
		return "video/mp4"
	case ".mov":
		return "video/quicktime"
	case ".avi":
		return "video/x-msvideo"
	case ".wmv":
		return "video/x-ms-wmv"
	case ".webm":
		return "video/webm"
	case ".flv":
		return "video/x-flv"
	case ".mkv":
		return "video/x-matroska"
	case ".mp3":
		return "audio/mpeg"
	case ".wav":
		return "audio/wav"
	case ".ogg":
		return "audio/ogg"
	case ".flac":
		return "audio/flac"
	case ".aac":
		return "audio/aac"
	case ".m4a":
		return "audio/mp4"
	case ".pdf":
		return "application/pdf"
	case ".doc", ".docx":
		return "application/msword"
	case ".xls", ".xlsx":
		return "application/vnd.ms-excel"
	case ".ppt", ".pptx":
		return "application/vnd.ms-powerpoint"
	case ".txt":
		return "text/plain"
	case ".csv":
		return "text/csv"
	default:
		return "application/octet-stream"
	}
}

// NewAssetProcessor creates a new asset processor
func NewAssetProcessor(assetService service.AssetService, storage storage.Storage, storagePath string) *AssetProcessor {
	return &AssetProcessor{
		assetService: assetService,
		storage:      storage,
		storagePath:  storagePath,
	}
}
