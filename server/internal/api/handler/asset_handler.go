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

// UploadAsset handles asset upload requests.
// POST /api/v1/assets
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

	// Get client-provided hash from header (if available)
	clientHash := c.GetHeader("X-Content-Hash")
	if clientHash == "" {
		log.Println("Warning: No content hash provided by client")
		// We could calculate it server-side, but for now we'll just generate a random ID
		// In a real implementation, we should calculate the hash here
		clientHash = uuid.New().String()
	}

	// Create a unique filename in staging area
	stagingFileName := uuid.New().String()
	fileExt := filepath.Ext(header.Filename)
	stagingFilePath := filepath.Join(h.stagingPath, stagingFileName+fileExt)

	// Ensure staging directory exists
	if err := os.MkdirAll(h.stagingPath, 0755); err != nil {
		log.Printf("Failed to create staging directory: %v", err)
		api.Error(c.Writer, http.StatusInternalServerError, err, http.StatusInternalServerError, "Upload failed")
		return
	}

	// Create destination file
	stagingFile, err := os.Create(stagingFilePath)
	if err != nil {
		log.Printf("Failed to create staging file: %v", err)
		api.Error(c.Writer, http.StatusInternalServerError, err, http.StatusInternalServerError, "Upload failed")
		return
	}
	defer stagingFile.Close()

	// Copy uploaded file to staging area
	_, err = io.Copy(stagingFile, file)
	if err != nil {
		log.Printf("Failed to copy file to staging: %v", err)
		api.Error(c.Writer, http.StatusInternalServerError, err, http.StatusInternalServerError, "Upload failed")
		return
	}

	// Get user ID (in a real app, get this from authentication)
	userID := c.GetString("user_id")
	if userID == "" {
		userID = "anonymous" // Default user ID if not authenticated
	}

	// Create task for processing
	task := queue.Task{
		TaskID:      uuid.New().String(),
		ClientHash:  clientHash,
		StagedPath:  stagingFilePath,
		UserID:      userID,
		Timestamp:   time.Now(),
		ContentType: header.Header.Get("Content-Type"),
		FileName:    header.Filename,
	}

	// Enqueue task
	err = h.taskQueue.EnqueueTask(task)
	if err != nil {
		log.Printf("Failed to enqueue task: %v", err)
		api.Error(c.Writer, http.StatusInternalServerError, err, http.StatusInternalServerError, "Upload failed")
		return
	}

	log.Printf("Task %s enqueued for processing file %s", task.TaskID, header.Filename)

	// Return response indicating the task was accepted
	response := map[string]interface{}{
		"task_id":      task.TaskID,
		"status":       "processing",
		"file_name":    header.Filename,
		"size":         header.Size,
		"content_hash": clientHash,
		"message":      "File received and queued for processing",
	}
	api.Success(c.Writer, response)
}

// GetAsset retrieves a single asset by ID
// GET /api/v1/assets/:id
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
// GET /api/v1/assets?type=PHOTO&owner_id=123&limit=20&offset=0&q=search
func (h *AssetHandler) ListAssets(c *gin.Context) {
	// Parse query parameters
	limitStr := c.DefaultQuery("limit", "20")
	offsetStr := c.DefaultQuery("offset", "0")
	typeStr := c.Query("type")
	ownerIDStr := c.Query("owner_id")
	searchQuery := c.Query("q")

	limit, _ := strconv.Atoi(limitStr)
	offset, _ := strconv.Atoi(offsetStr)

	// Validate limit
	if limit > 100 {
		limit = 100
	}

	ctx := c.Request.Context()
	var assets []*models.Asset
	var err error

	// Handle different query scenarios
	switch {
	case searchQuery != "":
		// Search assets
		var assetType *models.AssetType
		if typeStr != "" {
			at := models.AssetType(typeStr)
			if at.Valid() {
				assetType = &at
			}
		}
		assets, err = h.assetService.SearchAssets(ctx, searchQuery, assetType, limit, offset)

	case ownerIDStr != "":
		// Get assets by owner
		ownerID, parseErr := strconv.Atoi(ownerIDStr)
		if parseErr != nil {
			api.Error(c.Writer, http.StatusBadRequest, parseErr, http.StatusBadRequest, "Invalid owner_id")
			return
		}
		assets, err = h.assetService.GetAssetsByOwner(ctx, ownerID, limit, offset)

	case typeStr != "":
		// Get assets by type
		assetType := models.AssetType(typeStr)
		if !assetType.Valid() {
			api.Error(c.Writer, http.StatusBadRequest, errors.New("invalid asset type"), http.StatusBadRequest, "Invalid asset type")
			return
		}
		assets, err = h.assetService.GetAssetsByType(ctx, assetType, limit, offset)

	default:
		// This would require a new method in service to get all assets
		api.Error(c.Writer, http.StatusBadRequest, errors.New("missing query parameters"), http.StatusBadRequest, "Please specify type, owner_id, or search query")
		return
	}

	if err != nil {
		log.Printf("Failed to retrieve assets: %v", err)
		api.Error(c.Writer, http.StatusInternalServerError, err, http.StatusInternalServerError, "Failed to retrieve assets")
		return
	}

	response := map[string]interface{}{
		"assets": assets,
		"limit":  limit,
		"offset": offset,
	}
	api.Success(c.Writer, response)
}

// UpdateAsset updates asset metadata
// PUT /api/v1/assets/:id
func (h *AssetHandler) UpdateAsset(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		api.Error(c.Writer, http.StatusBadRequest, err, http.StatusBadRequest, "Invalid asset ID")
		return
	}

	var updateData struct {
		Metadata models.SpecificMetadata `json:"metadata"`
	}

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

	api.Success(c.Writer, map[string]string{"message": "Asset updated successfully"})
}

// DeleteAsset deletes an asset
// DELETE /api/v1/assets/:id
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

	api.Success(c.Writer, map[string]string{"message": "Asset deleted successfully"})
}

// BatchUploadAssets handles multiple asset uploads
// POST /api/v1/assets/batch
func (h *AssetHandler) BatchUploadAssets(c *gin.Context) {
	err := c.Request.ParseMultipartForm(128 << 20) // 128MB max for batch
	if err != nil {
		api.Error(c.Writer, http.StatusBadRequest, err, http.StatusBadRequest, "Failed to parse form")
		return
	}

	files := c.Request.MultipartForm.File["files"]
	if len(files) == 0 {
		api.Error(c.Writer, http.StatusBadRequest, errors.New("no files provided"), http.StatusBadRequest, "No files provided")
		return
	}

	// Get user ID (in a real app, get this from authentication)
	userID := c.GetString("user_id")
	if userID == "" {
		userID = "anonymous"
	}

	results := make([]map[string]interface{}, len(files))

	for i, header := range files {
		file, err := header.Open()
		if err != nil {
			results[i] = map[string]interface{}{
				"filename": header.Filename,
				"error":    "Failed to open file: " + err.Error(),
				"success":  false,
			}
			continue
		}

		// Get client-provided hash from header (if available)
		clientHash := c.GetHeader("X-Content-Hash")
		if clientHash == "" {
			log.Println("Warning: No content hash provided by client for file", header.Filename)
			clientHash = uuid.New().String()
		}

		// Create a unique filename in staging area
		stagingFileName := uuid.New().String()
		fileExt := filepath.Ext(header.Filename)
		stagingFilePath := filepath.Join(h.stagingPath, stagingFileName+fileExt)

		// Ensure staging directory exists
		if err := os.MkdirAll(h.stagingPath, 0755); err != nil {
			log.Printf("Failed to create staging directory: %v", err)
			results[i] = map[string]interface{}{
				"filename": header.Filename,
				"error":    "Failed to create staging directory: " + err.Error(),
				"success":  false,
			}
			file.Close()
			continue
		}

		// Create destination file
		stagingFile, err := os.Create(stagingFilePath)
		if err != nil {
			log.Printf("Failed to create staging file: %v", err)
			results[i] = map[string]interface{}{
				"filename": header.Filename,
				"error":    "Failed to create staging file: " + err.Error(),
				"success":  false,
			}
			file.Close()
			continue
		}

		// Copy uploaded file to staging area
		_, err = io.Copy(stagingFile, file)
		stagingFile.Close()
		file.Close()
		if err != nil {
			log.Printf("Failed to copy file to staging: %v", err)
			results[i] = map[string]interface{}{
				"filename": header.Filename,
				"error":    "Failed to copy file to staging: " + err.Error(),
				"success":  false,
			}
			continue
		}

		// Create task for processing
		task := queue.Task{
			TaskID:      uuid.New().String(),
			ClientHash:  clientHash,
			StagedPath:  stagingFilePath,
			UserID:      userID,
			Timestamp:   time.Now(),
			ContentType: header.Header.Get("Content-Type"),
			FileName:    header.Filename,
		}

		// Enqueue task
		err = h.taskQueue.EnqueueTask(task)
		if err != nil {
			log.Printf("Failed to enqueue task: %v", err)
			results[i] = map[string]interface{}{
				"filename": header.Filename,
				"error":    "Failed to enqueue task: " + err.Error(),
				"success":  false,
			}
			continue
		}

		log.Printf("Task %s enqueued for processing file %s", task.TaskID, header.Filename)

		results[i] = map[string]interface{}{
			"task_id":      task.TaskID,
			"status":       "processing",
			"file_name":    header.Filename,
			"size":         header.Size,
			"content_hash": clientHash,
			"success":      true,
			"message":      "File received and queued for processing",
		}
	}

	api.Success(c.Writer, map[string]interface{}{
		"results": results,
	})
}

// AddAssetToAlbum adds an asset to an album
// POST /api/v1/assets/:id/albums/:albumId
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

	api.Success(c.Writer, map[string]string{"message": "Asset added to album successfully"})
}

// GetAssetTypes returns available asset types
// GET /api/v1/assets/types
func (h *AssetHandler) GetAssetTypes(c *gin.Context) {
	types := []models.AssetType{
		models.AssetTypePhoto,
		models.AssetTypeVideo,
		models.AssetTypeAudio,
		models.AssetTypeDocument,
	}

	api.Success(c.Writer, map[string]interface{}{
		"types": types,
	})
}
