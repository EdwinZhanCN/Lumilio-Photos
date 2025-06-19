package utils

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"log"
	"math"
	"os"
	"path/filepath"
	"server/internal/models"
	"server/internal/service"
	"server/internal/storage"
	"strconv"
	"strings"
	"time"

	pb "server/proto"

	"github.com/google/uuid"
	"github.com/nfnt/resize"

	// Extended image format support
	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
)

// AssetProcessor handles processing tasks for different asset types
type AssetProcessor struct {
	// Dependencies for processing different asset types
	assetService service.AssetService
	storage      storage.Storage
	storagePath  string
	mlService    service.MLService
	// Image preprocessing configuration
	mlImageMaxWidth  uint
	mlImageMaxHeight uint
	mlImageQuality   int
}

// ThumbnailSize defines the name and max dimension for a thumbnail
type ThumbnailSize struct {
	Name      string
	Dimension uint
}

// Defines the standard sizes for thumbnail generation.
var thumbnailSizes = []ThumbnailSize{
	{Name: "small", Dimension: 400},
	{Name: "medium", Dimension: 800},
	{Name: "large", Dimension: 1920},
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

// ProcessNewAsset processes a newly uploaded asset file from staging area
func (ap *AssetProcessor) ProcessNewAsset(stagedPath string, userID string, fileName string) (*models.Asset, error) {
	ctx := context.Background()

	// Open the staged file
	file, err := os.Open(stagedPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open staged file: %w", err)
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	// Parse owner ID if possible
	var ownerIDPtr *int
	if userID != "anonymous" {
		ownerID := 1 // Default owner ID, in real app convert userID string to int
		ownerIDPtr = &ownerID
	}

	// Reset file position for reading
	if _, err := file.Seek(0, 0); err != nil {
		return nil, fmt.Errorf("failed to reset file position: %w", err)
	}

	// Use AssetService.UploadAsset to handle the upload process
	// This will calculate hash, check duplicates, upload to storage, and create DB record
	asset, err := ap.assetService.UploadAsset(ctx, file, fileName, fileInfo.Size(), ownerIDPtr)
	if err != nil {
		return nil, fmt.Errorf("failed to upload asset: %w", err)
	}

	// Now process the asset for metadata extraction and AI tagging
	if err := ap.ProcessAsset(ctx, asset); err != nil {
		// Log error but don't fail the entire operation
		fmt.Printf("Warning: failed to process asset metadata: %v\n", err)
	}

	// Clean up the staged file since it's now in permanent storage
	if err := os.Remove(stagedPath); err != nil {
		fmt.Printf("Warning: failed to remove staged file %s: %v\n", stagedPath, err)
	}

	return asset, nil
}

// ProcessExistingAsset processes an existing file that's already in the storage system
func (ap *AssetProcessor) ProcessExistingAsset(filePath string, userID string, fileName string) (*models.Asset, error) {
	ctx := context.Background()

	// Calculate hash for the existing file
	hash, err := CalculateFileHash(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate file hash: %w", err)
	}

	// Check if this asset already exists in the database
	existingAssets, err := ap.assetService.DetectDuplicates(ctx, hash)
	if err == nil && len(existingAssets) > 0 {
		// Asset already exists, just process it for any missing metadata
		asset := existingAssets[0]
		if err := ap.ProcessAsset(ctx, asset); err != nil {
			fmt.Printf("Warning: failed to process existing asset metadata: %v\n", err)
		}
		return asset, nil
	}

	// Asset doesn't exist in DB, create a new record
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	// Parse owner ID
	var ownerIDPtr *int
	if userID != "anonymous" {
		ownerID := 1
		ownerIDPtr = &ownerID
	}

	// Create asset record directly (file is already in storage)
	asset := &models.Asset{
		AssetID:          uuid.New(),
		OwnerID:          ownerIDPtr,
		Type:             determineAssetType(fileName),
		OriginalFilename: fileName,
		StoragePath:      filePath,
		MimeType:         getMimeTypeFromFileName(fileName),
		FileSize:         fileInfo.Size(),
		Hash:             hash,
		UploadTime:       time.Now(),
		IsDeleted:        false,
		SpecificMetadata: make(models.SpecificMetadata),
	}

	// Save to database
	if err := ap.assetService.CreateAssetRecord(ctx, asset); err != nil {
		return nil, fmt.Errorf("failed to create asset record: %w", err)
	}

	// Process for metadata and AI tagging
	if err := ap.ProcessAsset(ctx, asset); err != nil {
		fmt.Printf("Warning: failed to process asset metadata: %v\n", err)
	}

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
	// 1. Extract Metadata
	metadata, err := ap.ExtractAssetMetadata(ctx, asset.AssetID.String(), asset.StoragePath)
	if err != nil {
		return fmt.Errorf("failed to extract asset metadata: %w", err)
	}
	if err := asset.SetPhotoMetadata(&metadata); err != nil {
		return fmt.Errorf("failed to set photo metadata: %w", err)
	}

	// 2. Save Metadata to Database
	if err := ap.assetService.UpdateAssetMetadata(ctx, asset.AssetID, asset.SpecificMetadata); err != nil {
		return fmt.Errorf("failed to save asset metadata: %w", err)
	}

	// 3. Get AI generated tags
	// Construct full path for reading the image file
	rootStoragePath := os.Getenv("STORAGE_PATH")
	if rootStoragePath == "" {
		rootStoragePath = ap.storagePath
	}
	fullImagePath := filepath.Join(rootStoragePath, asset.StoragePath)

	// Check if file exists before trying to read
	if _, err := os.Stat(fullImagePath); os.IsNotExist(err) {
		return fmt.Errorf("image file not found at path: %s", fullImagePath)
	}

	// Resize image for ML processing to reduce payload size
	resizedImageBytes, err := ap.resizeImageForML(fullImagePath, ap.mlImageMaxWidth, ap.mlImageMaxHeight)
	if err != nil {
		return fmt.Errorf("failed to resize image for ML: %w", err)
	}

	imageProcessRequest := pb.ImageProcessRequest{
		ImageId:   asset.AssetID.String(),
		ImageData: resizedImageBytes,
	}

	imageProcessReponse, err := ap.mlService.ProcessImageForCLIP(&imageProcessRequest)
	if err != nil {
		return fmt.Errorf("failed to process image for CLIP: %w", err)
	}

	if err := ap.saveCLIPTagsToAsset(ctx, asset, imageProcessReponse); err != nil {
		return fmt.Errorf("failed to save CLIP tags: %w", err)
	}

	// 4. Generate and save thumbnails
	if err := ap.generateAndSaveThumbnails(ctx, asset); err != nil {
		// Log as a warning because the main asset processing succeeded
		log.Printf("Warning: failed to generate thumbnails for asset %s: %v", asset.AssetID, err)
	}

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

// NewAssetProcessor creates a new asset processor with configurable ML image preprocessing
func NewAssetProcessor(assetService service.AssetService, storage storage.Storage, storagePath string, mlService service.MLService) *AssetProcessor {
	// Load ML image preprocessing configuration from environment variables
	maxWidth := uint(1024)  // Default
	maxHeight := uint(1024) // Default
	quality := 85           // Default

	if envWidth := os.Getenv("ML_IMAGE_MAX_WIDTH"); envWidth != "" {
		if width, err := strconv.ParseUint(envWidth, 10, 32); err == nil {
			maxWidth = uint(width)
		}
	}

	if envHeight := os.Getenv("ML_IMAGE_MAX_HEIGHT"); envHeight != "" {
		if height, err := strconv.ParseUint(envHeight, 10, 32); err == nil {
			maxHeight = uint(height)
		}
	}

	if envQuality := os.Getenv("ML_IMAGE_QUALITY"); envQuality != "" {
		if q, err := strconv.Atoi(envQuality); err == nil && q >= 1 && q <= 100 {
			quality = q
		}
	}

	fmt.Printf("ML Image Preprocessing Config: %dx%d max size, %d%% JPEG quality\n",
		maxWidth, maxHeight, quality)

	return &AssetProcessor{
		assetService:     assetService,
		storage:          storage,
		storagePath:      storagePath,
		mlService:        mlService,
		mlImageMaxWidth:  maxWidth,
		mlImageMaxHeight: maxHeight,
		mlImageQuality:   quality,
	}
}

// resizeImageForML resizes an image to fit within the specified dimensions while maintaining aspect ratio
func (ap *AssetProcessor) resizeImageForML(imagePath string, maxWidth, maxHeight uint) ([]byte, error) {
	// Check file extension first for debugging
	ext := strings.ToLower(filepath.Ext(imagePath))
	fmt.Printf("Processing image: %s (extension: %s)\n", imagePath, ext)

	// Open the image file
	file, err := os.Open(imagePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open image %s: %w", imagePath, err)
	}
	defer file.Close()

	// Read first few bytes to check file signature
	fileHeader := make([]byte, 512)
	n, _ := file.Read(fileHeader)
	if n > 0 {
		fmt.Printf("File header (first %d bytes): %x\n", min(n, 16), fileHeader[:min(n, 16)])
	}

	// Reset file position for decoding
	file.Seek(0, 0)

	// Decode the image with better error handling for different formats
	img, format, err := image.Decode(file)
	if err != nil {
		return nil, fmt.Errorf("failed to decode image %s (detected format: %s, file extension: %s): %w", imagePath, format, ext, err)
	}

	fmt.Printf("Successfully decoded image: format=%s, extension=%s\n", format, ext)

	// Get original dimensions
	bounds := img.Bounds()
	width := uint(bounds.Dx())
	height := uint(bounds.Dy())

	// Validate dimensions
	if width == 0 || height == 0 {
		return nil, fmt.Errorf("invalid image dimensions: %dx%d", width, height)
	}

	// Get original file size for comparison
	originalFileInfo, err := os.Stat(imagePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get original file info: %w", err)
	}
	originalFileSize := originalFileInfo.Size()

	// Skip processing if image is already small enough and file size is reasonable
	if width <= maxWidth && height <= maxHeight && originalFileSize < 2*1024*1024 { // < 2MB
		// Return original bytes for small images
		originalBytes, err := os.ReadFile(imagePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read original image: %w", err)
		}
		fmt.Printf("Image already optimal for ML: %dx%d, %d KB\n", width, height, originalFileSize/1024)
		return originalBytes, nil
	}

	// Calculate new dimensions while maintaining aspect ratio
	widthRatio := float64(maxWidth) / float64(width)
	heightRatio := float64(maxHeight) / float64(height)
	ratio := math.Min(widthRatio, heightRatio)

	newWidth := uint(float64(width) * ratio)
	newHeight := uint(float64(height) * ratio)

	// Ensure minimum dimensions
	if newWidth < 224 && newHeight < 224 {
		// Maintain minimum size for ML models (224x224 is common)
		if width > height {
			newWidth = 224
			newHeight = uint(float64(newWidth) * float64(height) / float64(width))
		} else {
			newHeight = 224
			newWidth = uint(float64(newHeight) * float64(width) / float64(height))
		}
	}

	// Resize the image using high-quality Lanczos resampling
	resizedImg := resize.Resize(newWidth, newHeight, img, resize.Lanczos3)

	// Always encode as JPEG for ML processing (optimal size/quality balance)
	var buf bytes.Buffer
	quality := ap.mlImageQuality

	// Adjust quality based on image size for better compression
	if originalFileSize > 10*1024*1024 { // > 10MB
		quality = int(float64(quality) * 0.8) // Reduce quality for very large images
	}

	err = jpeg.Encode(&buf, resizedImg, &jpeg.Options{Quality: quality})
	if err != nil {
		return nil, fmt.Errorf("failed to encode resized image: %w", err)
	}

	// Calculate compression stats
	originalPixels := width * height
	newPixels := newWidth * newHeight
	pixelReduction := float64(originalPixels) / float64(newPixels)
	newFileSize := int64(buf.Len())
	sizeReduction := float64(originalFileSize) / float64(newFileSize)

	fmt.Printf("Resized image for ML: %dx%d → %dx%d (%.1fx pixel reduction, %.1fx size reduction: %d KB → %d KB, quality: %d%%)\n",
		width, height, newWidth, newHeight, pixelReduction, sizeReduction,
		originalFileSize/1024, newFileSize/1024, quality)

	return buf.Bytes(), nil
}

// saveCLIPTagsToAsset saves tags from the ML service response to the asset.
func (ap *AssetProcessor) saveCLIPTagsToAsset(ctx context.Context, asset *models.Asset, resp *pb.ImageProcessResponse) error {
	fmt.Printf("DEBUG: Starting saveCLIPTagsToAsset for asset ID: %s\n", asset.AssetID)
	predictedScores := resp.GetPredictedScores()
	fmt.Printf("DEBUG: Received %d predicted scores\n", len(predictedScores))

	topN := 3
	if len(predictedScores) > topN {
		predictedScores = predictedScores[:topN]
		fmt.Printf("DEBUG: Limiting to top %d scores\n", topN)
	}

	for i, prediction := range predictedScores {
		label := prediction.GetLabel()
		score := prediction.GetSimilarityScore()

		fmt.Printf("DEBUG: Processing prediction %d: label='%s', score=%f\n", i+1, label, score)

		tag, err := ap.assetService.GetOrCreateTagByName(ctx, label, "CLIP", true)
		if err != nil {
			fmt.Printf("ERROR: Failed to get or create tag for label '%s': %v\n", label, err)
			return fmt.Errorf("failed to get or create tag: %w", err)
		}
		fmt.Printf("DEBUG: Successfully got or created tag: ID=%d, Name=%s\n", tag.TagID, tag.TagName)

		err = ap.assetService.AddTagToAsset(ctx, asset.AssetID, tag.TagID, float32(score), "ai")
		if err != nil {
			fmt.Printf("ERROR: Failed to add tag to asset: AssetID=%s, TagID=%d, Error=%v\n", asset.AssetID, tag.TagID, err)
			return fmt.Errorf("failed to add tag to asset: %w", err)
		}
		fmt.Printf("DEBUG: Successfully added tag to asset: AssetID=%s, TagID=%d\n", asset.AssetID, tag.TagID)
	}
	fmt.Printf("DEBUG: Finished saveCLIPTagsToAsset for asset ID: %s\n", asset.AssetID)
	return nil
}

// generateAndSaveThumbnails creates, uploads, and records thumbnails for a given asset.
func (ap *AssetProcessor) generateAndSaveThumbnails(ctx context.Context, asset *models.Asset) error {
	log.Printf("Generating thumbnails for asset %s", asset.AssetID)

	// Defensive check to prevent panic if storage is not initialized
	if ap.storage == nil {
		return fmt.Errorf("storage service is not initialized in AssetProcessor")
	}

	// Get full path to original image
	rootStoragePath := os.Getenv("STORAGE_PATH")
	if rootStoragePath == "" {
		rootStoragePath = ap.storagePath
	}
	fullImagePath := filepath.Join(rootStoragePath, asset.StoragePath)

	// Open and decode the original image
	file, err := os.Open(fullImagePath)
	if err != nil {
		return fmt.Errorf("failed to open original image for thumbnailing: %w", err)
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		return fmt.Errorf("failed to decode original image for thumbnailing: %w", err)
	}

	// Check for valid image dimensions
	bounds := img.Bounds()
	if bounds.Dx() == 0 || bounds.Dy() == 0 {
		return fmt.Errorf("invalid image dimensions (0) for thumbnailing asset %s", asset.AssetID)
	}

	// Generate a thumbnail for each defined size
	for _, size := range thumbnailSizes {
		// Use resize.Thumbnail which maintains aspect ratio and is efficient
		resizedImg := resize.Thumbnail(size.Dimension, size.Dimension, img, resize.Lanczos3)

		// Encode as JPEG
		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, resizedImg, &jpeg.Options{Quality: 85}); err != nil {
			log.Printf("Failed to encode thumbnail %s for asset %s: %v", size.Name, asset.AssetID, err)
			continue // Don't stop for one failed thumbnail
		}

		// Determine thumbnail storage path and filename
		ext := filepath.Ext(asset.OriginalFilename)
		baseFilename := strings.TrimSuffix(asset.OriginalFilename, ext)
		thumbFilename := fmt.Sprintf("%s_thumb_%s.jpg", baseFilename, size.Name)

		// Upload thumbnail to storage
		storagePath, err := ap.storage.UploadWithMetadata(ctx, &buf, thumbFilename, "image/jpeg")
		if err != nil {
			log.Printf("Failed to upload thumbnail %s for asset %s: %v", size.Name, asset.AssetID, err)
			continue
		}

		// Save thumbnail record to database
		if _, err := ap.assetService.CreateThumbnail(ctx, asset.AssetID, size.Name, storagePath); err != nil {
			log.Printf("Failed to save thumbnail record %s for asset %s: %v", size.Name, asset.AssetID, err)
			// Attempt to clean up orphaned thumbnail file
			_ = ap.storage.Delete(context.Background(), storagePath)
			continue
		}
		log.Printf("Successfully created and saved thumbnail '%s' for asset %s at %s", size.Name, asset.AssetID, storagePath)
	}

	return nil
}
