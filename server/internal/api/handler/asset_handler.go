package handler

import (
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"server/internal/api"
	"server/internal/models"
	"server/internal/queue"
	"server/internal/service"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// UploadResponse represents the response structure for file upload
type UploadResponse struct {
	TaskID      string `json:"task_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Status      string `json:"status" example:"processing"`
	FileName    string `json:"file_name" example:"photo.jpg"`
	Size        int64  `json:"size" example:"1048576"`
	ContentHash string `json:"content_hash" example:"abcd1234567890"`
	Message     string `json:"message" example:"File received and queued for processing"`
}

// BatchUploadResponse represents the response structure for batch upload
type BatchUploadResponse struct {
	Results []BatchUploadResult `json:"results"`
}

type BatchUploadResult struct {
	Success     bool    `json:"success"`             // Whether the file was successfully queued
	FileName    string  `json:"file_name,omitempty"` // Original filename
	ContentHash string  `json:"content_hash"`        // Client-provided content hash
	TaskID      *string `json:"task_id,omitempty"`   // Only present for successful uploads
	Status      *string `json:"status,omitempty"`    // Only present for successful uploads
	Size        *int64  `json:"size,omitempty"`      // Only present for successful uploads
	Message     *string `json:"message,omitempty"`   // Status message
	Error       *string `json:"error,omitempty"`     // Only present for failed uploads
}

// AssetListResponse represents the response structure for asset listing
type AssetListResponse struct {
	Assets []*models.Asset `json:"assets"`
	Limit  int             `json:"limit" example:"20"`
	Offset int             `json:"offset" example:"0"`
}

// UpdateAssetRequest represents the request structure for updating asset metadata
type UpdateAssetRequest struct {
	Metadata models.SpecificMetadata `json:"metadata"`
}

// MessageResponse represents a simple message response
type MessageResponse struct {
	Message string `json:"message" example:"Operation completed successfully"`
}

// AssetTypesResponse represents the response structure for asset types
type AssetTypesResponse struct {
	Types []models.AssetType `json:"types"`
}

// AssetHandler handles HTTP requests for asset management
type AssetHandler struct {
	assetService service.AssetService
	stagingPath  string
	taskQueue    *queue.TaskQueue
}

// NewAssetHandler creates a new AssetHandler instance
func NewAssetHandler(assetService service.AssetService, stagingPath string, taskQueue *queue.TaskQueue) *AssetHandler {
	return &AssetHandler{
		assetService: assetService,
		stagingPath:  stagingPath,
		taskQueue:    taskQueue,
	}
}

// UploadAsset handles asset upload requests
// @Summary Upload a single asset
// @Description Upload a single photo, video, audio file or document to the system
// @Tags assets
// @Accept multipart/form-data
// @Produce json
// @Param file formData file true "Asset file to upload"
// @Param X-Content-Hash header string false "Client-calculated BLAKE3 hash of the file"
// @Success 200 {object} api.Result{data=UploadResponse} "Upload successful"
// @Failure 400 {object} api.Result "Bad request - no file provided or parse error"
// @Failure 500 {object} api.Result "Internal server error"
// @Router /api/v1/assets [post]
func (h *AssetHandler) UploadAsset(c *gin.Context) {
	// Parse multipart form
	err := c.Request.ParseMultipartForm(32 << 20) // 32MB max
	if err != nil {
		api.Error(c.Writer, http.StatusBadRequest, err, http.StatusBadRequest, "Failed to parse form")
		return
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		api.Error(c.Writer, http.StatusBadRequest, errors.New("no file provided"), http.StatusBadRequest)
		return
	}
	defer file.Close()

	clientHash := c.GetHeader("X-Content-Hash")
	if clientHash == "" {
		log.Println("Warning: No content hash provided by client")
		clientHash = uuid.New().String()
	}

	stagingFileName := uuid.New().String()
	fileExt := filepath.Ext(header.Filename)
	stagingFilePath := filepath.Join(h.stagingPath, stagingFileName+fileExt)

	if err := os.MkdirAll(h.stagingPath, 0755); err != nil {
		log.Printf("Failed to create staging directory: %v", err)
		api.Error(c.Writer, http.StatusInternalServerError, err, http.StatusInternalServerError, "Upload failed")
		return
	}

	stagingFile, err := os.Create(stagingFilePath)
	if err != nil {
		log.Printf("Failed to create staging file: %v", err)
		api.Error(c.Writer, http.StatusInternalServerError, err, http.StatusInternalServerError, "Upload failed")
		return
	}
	defer stagingFile.Close()

	_, err = io.Copy(stagingFile, file)
	if err != nil {
		log.Printf("Failed to copy file to staging: %v", err)
		api.Error(c.Writer, http.StatusInternalServerError, err, http.StatusInternalServerError, "Upload failed")
		return
	}

	userID := c.GetString("user_id")
	if userID == "" {
		userID = "anonymous"
	}

	task := queue.Task{
		TaskID:      uuid.New().String(),
		Type:        string(queue.TaskTypeUpload),
		ClientHash:  clientHash,
		StagedPath:  stagingFilePath,
		UserID:      userID,
		Timestamp:   time.Now(),
		ContentType: header.Header.Get("Content-Type"),
		FileName:    header.Filename,
	}

	err = h.taskQueue.EnqueueTask(task)
	if err != nil {
		log.Printf("Failed to enqueue task: %v", err)
		api.Error(c.Writer, http.StatusInternalServerError, err, http.StatusInternalServerError, "Upload failed")
		return
	}

	log.Printf("Task %s enqueued for processing file %s", task.TaskID, header.Filename)

	response := UploadResponse{
		TaskID:      task.TaskID,
		Status:      "processing",
		FileName:    header.Filename,
		Size:        header.Size,
		ContentHash: clientHash,
		Message:     "File received and queued for processing",
	}
	api.Success(c.Writer, response)
}

// GetAsset retrieves a single asset by ID
// @Summary Get asset by ID
// @Description Retrieve detailed information about a specific asset including metadata, thumbnails, tags and albums
// @Tags assets
// @Accept json
// @Produce json
// @Param id path string true "Asset ID (UUID format)" example("550e8400-e29b-41d4-a716-446655440000")
// @Success 200 {object} api.Result{data=models.Asset} "Asset found"
// @Failure 400 {object} api.Result "Invalid asset ID format"
// @Failure 404 {object} api.Result "Asset not found"
// @Router /api/v1/assets/{id} [get]
func (h *AssetHandler) GetAsset(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		api.Error(c.Writer, http.StatusBadRequest, err, http.StatusBadRequest, "Invalid asset ID")
		return
	}

	asset, err := h.assetService.GetAsset(c.Request.Context(), id)
	if err != nil {
		api.Error(c.Writer, http.StatusNotFound, err, http.StatusNotFound, "Asset not found")
		return
	}

	api.Success(c.Writer, asset)
}

// ListAssets retrieves assets with optional filtering
// @Summary List assets with filtering
// @Description Retrieve a paginated list of assets with optional filtering by type, owner, or search query
// @Tags assets
// @Accept json
// @Produce json
// @Param type query string false "Asset type filter" Enums(PHOTO, VIDEO, AUDIO, DOCUMENT) example("PHOTO")
// @Param owner_id query int false "Filter by owner ID" example(123)
// @Param q query string false "Search query for filename" example("vacation")
// @Param limit query int false "Maximum number of results (max 100)" default(20) example(20)
// @Param offset query int false "Number of results to skip for pagination" default(0) example(0)
// @Success 200 {object} api.Result{data=AssetListResponse} "Assets retrieved successfully"
// @Failure 400 {object} api.Result "Invalid parameters"
// @Failure 500 {object} api.Result "Internal server error"
// @Router /api/v1/assets [get]
func (h *AssetHandler) ListAssets(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "20")
	offsetStr := c.DefaultQuery("offset", "0")
	typeStr := c.Query("type")
	ownerIDStr := c.Query("owner_id")
	searchQuery := c.Query("q")

	limit, _ := strconv.Atoi(limitStr)
	offset, _ := strconv.Atoi(offsetStr)

	if limit > 100 {
		limit = 100
	}

	ctx := c.Request.Context()
	var assets []*models.Asset
	var err error

	switch {
	case searchQuery != "":
		var assetType *models.AssetType
		if typeStr != "" {
			at := models.AssetType(typeStr)
			if at.Valid() {
				assetType = &at
			}
		}
		assets, err = h.assetService.SearchAssets(ctx, searchQuery, assetType, limit, offset)

	case ownerIDStr != "":
		ownerID, parseErr := strconv.Atoi(ownerIDStr)
		if parseErr != nil {
			api.Error(c.Writer, http.StatusBadRequest, parseErr, http.StatusBadRequest, "Invalid owner_id")
			return
		}
		assets, err = h.assetService.GetAssetsByOwner(ctx, ownerID, limit, offset)

	case typeStr != "":
		assetType := models.AssetType(typeStr)
		if !assetType.Valid() {
			api.Error(c.Writer, http.StatusBadRequest, errors.New("invalid asset type"), http.StatusBadRequest, "Invalid asset type")
			return
		}
		assets, err = h.assetService.GetAssetsByType(ctx, assetType, limit, offset)

	default:
		api.Error(c.Writer, http.StatusBadRequest, errors.New("missing query parameters"), http.StatusBadRequest, "Please specify type, owner_id, or search query")
		return
	}

	if err != nil {
		log.Printf("Failed to retrieve assets: %v", err)
		api.Error(c.Writer, http.StatusInternalServerError, err, http.StatusInternalServerError, "Failed to retrieve assets")
		return
	}

	response := AssetListResponse{
		Assets: assets,
		Limit:  limit,
		Offset: offset,
	}
	api.Success(c.Writer, response)
}

// UpdateAsset updates asset metadata
// @Summary Update asset metadata
// @Description Update the specific metadata of an asset (e.g., photo EXIF data, video metadata)
// @Tags assets
// @Accept json
// @Produce json
// @Param id path string true "Asset ID (UUID format)" example("550e8400-e29b-41d4-a716-446655440000")
// @Param metadata body UpdateAssetRequest true "Updated metadata"
// @Success 200 {object} api.Result{data=MessageResponse} "Asset updated successfully"
// @Failure 400 {object} api.Result "Invalid asset ID or request body"
// @Failure 500 {object} api.Result "Internal server error"
// @Router /api/v1/assets/{id} [put]
func (h *AssetHandler) UpdateAsset(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		api.Error(c.Writer, http.StatusBadRequest, err, http.StatusBadRequest, "Invalid asset ID")
		return
	}

	var updateData UpdateAssetRequest
	if err := c.ShouldBindJSON(&updateData); err != nil {
		api.Error(c.Writer, http.StatusBadRequest, err, http.StatusBadRequest, "Invalid request body")
		return
	}

	err = h.assetService.UpdateAssetMetadata(c.Request.Context(), id, updateData.Metadata)
	if err != nil {
		log.Printf("Failed to update asset metadata: %v", err)
		api.Error(c.Writer, http.StatusInternalServerError, err, http.StatusInternalServerError, "Failed to update asset")
		return
	}

	api.Success(c.Writer, MessageResponse{Message: "Asset updated successfully"})
}

// DeleteAsset deletes an asset
// @Summary Delete an asset
// @Description Soft delete an asset by marking it as deleted (does not remove the physical file)
// @Tags assets
// @Accept json
// @Produce json
// @Param id path string true "Asset ID (UUID format)" example("550e8400-e29b-41d4-a716-446655440000")
// @Success 200 {object} api.Result{data=MessageResponse} "Asset deleted successfully"
// @Failure 400 {object} api.Result "Invalid asset ID format"
// @Failure 500 {object} api.Result "Internal server error"
// @Router /api/v1/assets/{id} [delete]
func (h *AssetHandler) DeleteAsset(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		api.Error(c.Writer, http.StatusBadRequest, err, http.StatusBadRequest, "Invalid asset ID")
		return
	}

	err = h.assetService.DeleteAsset(c.Request.Context(), id)
	if err != nil {
		log.Printf("Failed to delete asset: %v", err)
		api.Error(c.Writer, http.StatusInternalServerError, err, http.StatusInternalServerError, "Failed to delete asset")
		return
	}

	api.Success(c.Writer, MessageResponse{Message: "Asset deleted successfully"})
}

// BatchUploadAssets handles multiple asset uploads
// @Summary Batch upload assets
// @Description Batch uploads multiple assets using a multipart/form-data request. The field name for each file part must be its BLAKE3 content hash.
// @Tags assets
// @Accept multipart/form-data
// @Produce json
// @Success 200 {object} api.Result{data=BatchUploadResponse} "Batch upload completed"
// @Failure 400 {object} api.Result "Bad request - no files provided or parse error"
// @Failure 500 {object} api.Result "Internal server error"
// @Router /api/v1/assets/batch [post]
func (h *AssetHandler) BatchUploadAssets(c *gin.Context) {
	err := c.Request.ParseMultipartForm(128 << 20) // 128MB max for batch
	if err != nil {
		api.Error(c.Writer, http.StatusBadRequest, err, http.StatusBadRequest, "Failed to parse form")
		return
	}

	form := c.Request.MultipartForm
	if form == nil || len(form.File) == 0 {
		api.Error(c.Writer, http.StatusBadRequest, errors.New("no files provided"), http.StatusBadRequest, "No files provided")
		return
	}

	userID := c.GetString("user_id")
	if userID == "" {
		userID = "anonymous"
	}

	var results []BatchUploadResult

	for clientHash, headers := range form.File {
		if len(headers) == 0 {
			continue
		}
		header := headers[0]

		file, err := header.Open()
		if err != nil {
			errMsg := "Failed to open file: " + err.Error()
			results = append(results, BatchUploadResult{
				Success:     false,
				FileName:    header.Filename,
				ContentHash: clientHash,
				Error:       &errMsg,
			})
			continue
		}

		stagingFileName := uuid.New().String()
		fileExt := filepath.Ext(header.Filename)
		stagingFilePath := filepath.Join(h.stagingPath, stagingFileName+fileExt)

		if err := os.MkdirAll(h.stagingPath, 0755); err != nil {
			log.Printf("Failed to create staging directory: %v", err)
			errMsg := "Failed to create staging directory: " + err.Error()
			results = append(results, BatchUploadResult{
				Success:     false,
				FileName:    header.Filename,
				ContentHash: clientHash,
				Error:       &errMsg,
			})
			file.Close()
			continue
		}

		stagingFile, err := os.Create(stagingFilePath)
		if err != nil {
			log.Printf("Failed to create staging file: %v", err)
			errMsg := "Failed to create staging file: " + err.Error()
			results = append(results, BatchUploadResult{
				Success:     false,
				FileName:    header.Filename,
				ContentHash: clientHash,
				Error:       &errMsg,
			})
			file.Close()
			continue
		}

		_, err = io.Copy(stagingFile, file)
		stagingFile.Close()
		file.Close()
		if err != nil {
			log.Printf("Failed to copy file to staging: %v", err)
			errMsg := "Failed to copy file to staging: " + err.Error()
			results = append(results, BatchUploadResult{
				Success:     false,
				FileName:    header.Filename,
				ContentHash: clientHash,
				Error:       &errMsg,
			})
			continue
		}

		task := queue.Task{
			TaskID:      uuid.New().String(),
			Type:        string(queue.TaskTypeUpload),
			ClientHash:  clientHash,
			StagedPath:  stagingFilePath,
			UserID:      userID,
			Timestamp:   time.Now(),
			ContentType: header.Header.Get("Content-Type"),
			FileName:    header.Filename,
		}

		err = h.taskQueue.EnqueueTask(task)
		if err != nil {
			log.Printf("Failed to enqueue task: %v", err)
			errMsg := "Failed to enqueue task: " + err.Error()
			results = append(results, BatchUploadResult{
				Success:     false,
				FileName:    header.Filename,
				ContentHash: clientHash,
				Error:       &errMsg,
			})
			continue
		}

		log.Printf("Task %s enqueued for processing file %s", task.TaskID, header.Filename)

		taskID := task.TaskID
		status := "processing"
		size := header.Size
		message := "File received and queued for processing"

		results = append(results, BatchUploadResult{
			Success:     true,
			FileName:    header.Filename,
			ContentHash: clientHash,
			TaskID:      &taskID,
			Status:      &status,
			Size:        &size,
			Message:     &message,
		})
	}

	api.Success(c.Writer, BatchUploadResponse{Results: results})
}

// AddAssetToAlbum adds an asset to an album
// @Summary Add asset to album
// @Description Associate an asset with a specific album
// @Tags assets
// @Accept json
// @Produce json
// @Param id path string true "Asset ID (UUID format)" example("550e8400-e29b-41d4-a716-446655440000")
// @Param albumId path int true "Album ID" example(123)
// @Success 200 {object} api.Result{data=MessageResponse} "Asset added to album successfully"
// @Failure 400 {object} api.Result "Invalid asset ID or album ID"
// @Failure 500 {object} api.Result "Internal server error"
// @Router /api/v1/assets/{id}/albums/{albumId} [post]
func (h *AssetHandler) AddAssetToAlbum(c *gin.Context) {
	assetID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		api.Error(c.Writer, http.StatusBadRequest, err, http.StatusBadRequest, "Invalid asset ID")
		return
	}

	albumID, err := strconv.Atoi(c.Param("albumId"))
	if err != nil {
		api.Error(c.Writer, http.StatusBadRequest, err, http.StatusBadRequest, "Invalid album ID")
		return
	}

	err = h.assetService.AddAssetToAlbum(c.Request.Context(), assetID, albumID)
	if err != nil {
		log.Printf("Failed to add asset to album: %v", err)
		api.Error(c.Writer, http.StatusInternalServerError, err, http.StatusInternalServerError, "Failed to add asset to album")
		return
	}

	api.Success(c.Writer, MessageResponse{Message: "Asset added to album successfully"})
}

// GetAssetTypes returns available asset types
// @Summary Get supported asset types
// @Description Retrieve a list of all supported asset types in the system
// @Tags assets
// @Accept json
// @Produce json
// @Success 200 {object} api.Result{data=AssetTypesResponse} "Asset types retrieved successfully"
// @Router /api/v1/assets/types [get]
func (h *AssetHandler) GetAssetTypes(c *gin.Context) {
	types := []models.AssetType{
		models.AssetTypePhoto,
		models.AssetTypeVideo,
		models.AssetTypeAudio,
		models.AssetTypeDocument,
	}

	api.Success(c.Writer, AssetTypesResponse{Types: types})
}
