package controller

import (
	"context"
	"log"
	"net/http"
	"server/cmd/web"
	"server/internal/service"
	"server/internal/utils"
)

type PhotoController struct {
	photoService   service.PhotoService
	imageProcessor *utils.ImageProcessor
}

func NewPhotoController(s service.PhotoService, p *utils.ImageProcessor) *PhotoController {
	return &PhotoController{photoService: s, imageProcessor: p}
}

// UploadPhoto handles file upload requests
func (c *PhotoController) UploadPhoto(w http.ResponseWriter, r *http.Request) {
	// 1. Parse request parameters
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Invalid file upload", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// 2. Call Service layer to upload the photo
	photo, err := c.photoService.UploadPhoto(r.Context(), file, header.Filename, header.Size)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 3. Process the uploaded image (generate thumbnails)
	go func() {
		// Create a new background context instead of using the request context
		// This prevents context cancellation when the HTTP request completes
		ctx := context.Background()
		photoID := photo.PhotoID.String()

		log.Printf("Processing uploaded image in the background")
		// Process the image to generate thumbnails
		if err := c.imageProcessor.ProcessUploadedImage(ctx, photoID, photo.StoragePath); err != nil {
			// Log the error but don't fail the request
			log.Printf("Failed to process image %s: %v", photoID, err)
		}

		// Extract and save metadata
		log.Printf("Extracting metadata for photo: %s from path: %s", photoID, photo.StoragePath)
		metadata, err := c.imageProcessor.ExtractImageMetadata(ctx, photoID, photo.StoragePath)
		if err == nil && metadata != nil {
			// Update the photo with extracted metadata
			if err := c.photoService.UpdatePhotoMetadata(ctx, photo.PhotoID, metadata); err != nil {
				log.Printf("Failed to update metadata for photo %s: %v", photoID, err)
			} else {
				log.Printf("Extracted metadata: Camera: %s, FNumber: %.1f, ISO: %d",
					metadata.CameraModel, metadata.FNumber, metadata.IsoSpeed)
			}
		}
	}()

	// 4. Return standardized response
	response := map[string]interface{}{
		"id":   photo.PhotoID,
		"url":  photo.StoragePath,
		"size": photo.FileSize,
	}

	// Send success response
	web.Success(w, response)
}
