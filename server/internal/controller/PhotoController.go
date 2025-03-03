package controller

import (
	"net/http"
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
	// 1. 解析请求参数
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Invalid file upload", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// 2. 调用 Service 层
	photo, err := c.photoService.UploadPhoto(r.Context(), file, header.Filename)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 3. 返回标准化响应
	response := map[string]interface{}{
		"id":   photo.ID,
		"url":  photo.StoragePath,
		"size": photo.FileSize,
	}
	jsonResponse(w, response, http.StatusCreated)
}
