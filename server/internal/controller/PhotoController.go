package controller

import (
	"net/http"
	"server/cmd/web"
	"server/internal/service"
)

type PhotoController struct {
	photoService service.PhotoService
}

func NewPhotoController(s service.PhotoService) *PhotoController {
	return &PhotoController{photoService: s}
}

// UploadPhoto 处理文件上传请求
func (c *PhotoController) UploadPhoto(w http.ResponseWriter, r *http.Request) {
	// 1. Parse request parameters
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Invalid file upload", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// 2. Call Service layer
	photo, err := c.photoService.UploadPhoto(r.Context(), file, header.Filename, header.Size)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 3. Return standardized response
	response := map[string]interface{}{
		"id":   photo.PhotoID,
		"url":  photo.StoragePath,
		"size": photo.FileSize,
	}

	// Send success response
	web.Success(w, response)
}
