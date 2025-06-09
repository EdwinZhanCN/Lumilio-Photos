package controller

import (
	"context"
	"io"
	"log"
	"net/http"
	"server/cmd/web"
	"server/internal/models"
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

// UploadPhoto handles file upload requests.
func (c *PhotoController) UploadPhoto(w http.ResponseWriter, r *http.Request) {
	// 1. Parse request parameters.
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Invalid file upload", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// 2. Call service layer to upload the photo.
	photo, err := c.photoService.UploadPhoto(r.Context(), file, header.Filename, header.Size)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 3. Process the uploaded image (generate thumbnails) in a new background context.
	go func() {
		ctx := context.Background()
		photoID := photo.PhotoID.String()

		log.Printf("Processing uploaded image in the background for photo: %s", photoID)
		if err := c.imageProcessor.ProcessUploadedImage(ctx, photoID, photo.StoragePath); err != nil {
			log.Printf("Failed to process image %s: %v", photoID, err)
		}

		log.Printf("Extracting metadata for photo: %s from path: %s", photoID, photo.StoragePath)
		metadata, err := c.imageProcessor.ExtractImageMetadata(ctx, photoID, photo.StoragePath)
		if err == nil && metadata != nil {
			if err := c.photoService.UpdatePhotoMetadata(ctx, photo.PhotoID, metadata); err != nil {
				log.Printf("Failed to update metadata for photo %s: %v", photoID, err)
			} else {
				log.Printf("Extracted metadata for photo %s: Camera: %s, FNumber: %.1f, ISO: %d", photoID, metadata.CameraModel, metadata.FNumber, metadata.IsoSpeed)
			}
		} else if err != nil {
			log.Printf("Error extracting metadata for photo %s: %v", photoID, err)
		}
	}()

	// 4. Return standardized response.
	response := map[string]interface{}{
		"id":   photo.PhotoID,
		"url":  photo.StoragePath,
		"size": photo.FileSize,
	}
	web.Success(w, response)
}

// BatchUploadPhotos handles multiple file uploads in a single request
func (c *PhotoController) BatchUploadPhotos(w http.ResponseWriter, r *http.Request) {
	// 0. get the time

	// 1. Parse the multipart form with a reasonable max memory
	if err := r.ParseMultipartForm(32 << 20); err != nil { // 32MB max memory
		http.Error(w, "Invalid multipart form", http.StatusBadRequest)
		return
	}

	// 2. Get all files from the form
	formFiles := r.MultipartForm.File["files"]
	if len(formFiles) == 0 {
		http.Error(w, "No files uploaded", http.StatusBadRequest)
		return
	}

	// 3. Prepare arrays for batch upload
	files := make([]io.Reader, len(formFiles))
	filenames := make([]string, len(formFiles))
	fileSizes := make([]int64, len(formFiles))

	// 4. Open all files
	for i, fileHeader := range formFiles {
		file, err := fileHeader.Open()
		if err != nil {
			http.Error(w, "Failed to open uploaded file", http.StatusInternalServerError)
			return
		}
		defer file.Close()

		files[i] = file
		filenames[i] = fileHeader.Filename
		fileSizes[i] = fileHeader.Size
	}

	// 5. Call service layer to batch upload photos
	photos, errs := c.photoService.BatchUploadPhotos(r.Context(), files, filenames, fileSizes)

	// 6. Process uploaded images in background
	for _, photo := range photos {
		if photo == nil {
			continue
		}

		go func(p *models.Photo) {
			ctx := context.Background()
			photoID := p.PhotoID.String()
			if err := c.imageProcessor.ProcessUploadedImage(ctx, photoID, p.StoragePath); err != nil {
				log.Printf("Failed to process image %s: %v", photoID, err)
			}

			metadata, err := c.imageProcessor.ExtractImageMetadata(ctx, photoID, p.StoragePath)
			if err == nil && metadata != nil {
				if err := c.photoService.UpdatePhotoMetadata(ctx, p.PhotoID, metadata); err != nil {
					log.Printf("Failed to update metadata for photo %s: %v", photoID, err)
				}
			} else if err != nil {
				log.Printf("Error extracting metadata for photo %s: %v", photoID, err)
			}
		}(photo)
	}

	// 7. Prepare response
	responseData := make([]map[string]interface{}, len(photos))
	for i, photo := range photos {
		if photo != nil {
			responseData[i] = map[string]interface{}{
				"id":   photo.PhotoID,
				"url":  photo.StoragePath,
				"size": photo.FileSize,
			}
		} else if i < len(errs) && errs[i] != nil {
			responseData[i] = map[string]interface{}{
				"error": errs[i].Error(),
			}
		}
	}

	web.Success(w, map[string]interface{}{
		"results":    responseData,
		"total":      len(formFiles),
		"successful": len(photos),
	})
}
