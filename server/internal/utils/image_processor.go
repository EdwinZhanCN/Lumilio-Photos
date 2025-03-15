package utils

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"server/internal/models"
	"server/internal/service"
	"strings"

	"github.com/disintegration/imaging"
)

// ThumbnailSize defines the available thumbnail sizes
type ThumbnailSize struct {
	Name   string
	Width  int
	Height int
}

// Available thumbnail sizes
var (
	SmallThumbnail  = ThumbnailSize{Name: "small", Width: 200, Height: 200}
	MediumThumbnail = ThumbnailSize{Name: "medium", Width: 500, Height: 500}
	LargeThumbnail  = ThumbnailSize{Name: "large", Width: 1024, Height: 1024}
)

// ImageProcessor handles image processing operations
type ImageProcessor struct {
	photoService service.PhotoService
	storage      service.CloudStorage
	basePath     string
}

// NewImageProcessor creates a new image processor
func NewImageProcessor(photoService service.PhotoService, storage service.CloudStorage, basePath string) *ImageProcessor {
	return &ImageProcessor{
		photoService: photoService,
		storage:      storage,
		basePath:     basePath,
	}
}

// ProcessUploadedImage processes an uploaded image, extracts metadata, and generates thumbnails
func (p *ImageProcessor) ProcessUploadedImage(ctx context.Context, photoID string, storagePath string) error {
	// 1. Get the original image
	file, err := p.storage.Get(ctx, storagePath)
	if err != nil {
		return fmt.Errorf("failed to get original image: %w", err)
	}
	defer file.Close()

	// 2. Read the image data
	imgData, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("failed to read image data: %w", err)
	}

	// 3. Decode the image
	img, format, err := image.Decode(bytes.NewReader(imgData))
	if err != nil {
		return fmt.Errorf("failed to decode image: %w", err)
	}

	// 4. Generate thumbnails
	photoUUID, err := models.ParseUUID(photoID)
	if err != nil {
		return fmt.Errorf("invalid photo ID: %w", err)
	}

	// Generate thumbnails for different sizes
	thumbnailSizes := []ThumbnailSize{SmallThumbnail, MediumThumbnail, LargeThumbnail}
	for _, size := range thumbnailSizes {
		if err := p.generateThumbnail(ctx, img, format, photoUUID, size); err != nil {
			return fmt.Errorf("failed to generate %s thumbnail: %w", size.Name, err)
		}
	}

	return nil
}

// generateThumbnail creates a thumbnail of the specified size
func (p *ImageProcessor) generateThumbnail(ctx context.Context, img image.Image, format string, photoID models.UUID, size ThumbnailSize) error {
	// 1. Resize the image
	thumbnail := imaging.Fit(img, size.Width, size.Height, imaging.Lanczos)

	// 2. Create a temporary file for the thumbnail
	tempDir := os.TempDir()
	thumbnailPrefix := "thumbnail-*" // Fixed pattern without path separator
	tempFile, err := os.CreateTemp(tempDir, thumbnailPrefix)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	// 3. Encode the thumbnail based on the original format
	switch strings.ToLower(format) {
	case "jpeg", "jpg":
		err = jpeg.Encode(tempFile, thumbnail, &jpeg.Options{Quality: 85})
	case "png":
		err = png.Encode(tempFile, thumbnail)
	default:
		// Default to JPEG for other formats
		err = jpeg.Encode(tempFile, thumbnail, &jpeg.Options{Quality: 85})
	}

	if err != nil {
		return fmt.Errorf("failed to encode thumbnail: %w", err)
	}

	// 4. Reset file pointer to beginning
	if _, err := tempFile.Seek(0, 0); err != nil {
		return fmt.Errorf("failed to reset file pointer: %w", err)
	}

	// 5. Upload the thumbnail to storage
	thumbnailPath, err := p.storage.Upload(ctx, tempFile)
	if err != nil {
		return fmt.Errorf("failed to upload thumbnail: %w", err)
	}

	// 6. Create thumbnail record in database
	_, err = p.photoService.CreateThumbnail(ctx, photoID, size.Name, thumbnailPath)
	if err != nil {
		// Try to clean up the uploaded thumbnail if record creation fails
		p.storage.Delete(ctx, thumbnailPath)
		return fmt.Errorf("failed to create thumbnail record: %w", err)
	}

	return nil
}

// ExtractImageMetadata is now implemented in exif_extractor.go
