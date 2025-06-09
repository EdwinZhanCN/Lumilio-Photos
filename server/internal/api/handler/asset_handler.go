package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"server/internal/api"
	"server/internal/models"
	"server/internal/service"
	"server/internal/utils"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// AssetHandler handles HTTP requests for asset operations
type AssetHandler struct {
	assetService   service.AssetService
	imageProcessor *utils.ImageProcessor // For backward compatibility with photo processing
}

// NewAssetHandler creates a new AssetHandler instance
func NewAssetHandler(s service.AssetService, p *utils.ImageProcessor) *AssetHandler {
	return &AssetHandler{
		assetService:   s,
		imageProcessor: p,
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

	// Optional: Parse owner ID if provided
	var ownerID *int
	if ownerIDStr := c.Request.FormValue("owner_id"); ownerIDStr != "" {
		if id, err := strconv.Atoi(ownerIDStr); err == nil {
			ownerID = &id
		}
	}

	// Upload the asset
	asset, err := h.assetService.UploadAsset(
		c.Request.Context(),
		file,
		header.Filename,
		header.Size,
		ownerID,
	)

	if err != nil {
		log.Printf("Asset upload failed: %v", err)
		api.Error(c.Writer, http.StatusInternalServerError, err, http.StatusInternalServerError, "Upload failed")
		return
	}

	// Queue background processing for photos
	if asset.IsPhoto() && h.imageProcessor != nil {
		go func() {
			assetID := asset.AssetID.String()

			// Process thumbnails
			if err := h.imageProcessor.ProcessUploadedAsset(context.Background(), assetID, asset.StoragePath); err != nil {
				log.Printf("Error processing asset %s: %v", assetID, err)
			}

			// Extract metadata for photos
			if photoMetadata, err := h.imageProcessor.ExtractAssetMetadata(context.Background(), assetID, asset.StoragePath); err == nil {
				// Convert PhotoSpecificMetadata to SpecificMetadata map
				metadataMap := make(models.SpecificMetadata)
				if data, err := json.Marshal(photoMetadata); err == nil {
					if err := json.Unmarshal(data, &metadataMap); err == nil {
						// Update asset with extracted metadata
						if err := h.assetService.UpdateAssetMetadata(context.Background(), asset.AssetID, metadataMap); err != nil {
							log.Printf("Error updating metadata for asset %s: %v", assetID, err)
						} else {
							log.Printf("Extracted metadata for asset %s: Camera: %s, FNumber: %.1f, ISO: %d",
								assetID, photoMetadata.CameraModel, photoMetadata.FNumber, photoMetadata.IsoSpeed)
						}
					} else {
						log.Printf("Error converting metadata to map for asset %s: %v", assetID, err)
					}
				} else {
					log.Printf("Error marshaling metadata for asset %s: %v", assetID, err)
				}
			} else {
				log.Printf("Error extracting metadata for asset %s: %v", assetID, err)
			}
		}()
	}

	// Return response
	response := map[string]interface{}{
		"id":        asset.AssetID,
		"type":      asset.Type,
		"url":       asset.StoragePath,
		"size":      asset.FileSize,
		"mime_type": asset.MimeType,
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

	// Optional: Parse owner ID if provided
	var ownerID *int
	if ownerIDStr := c.Request.FormValue("owner_id"); ownerIDStr != "" {
		if id, err := strconv.Atoi(ownerIDStr); err == nil {
			ownerID = &id
		}
	}

	// Prepare file readers
	fileReaders := make([]io.Reader, len(files))
	filenames := make([]string, len(files))
	fileSizes := make([]int64, len(files))

	for i, header := range files {
		file, err := header.Open()
		if err != nil {
			api.Error(c.Writer, http.StatusInternalServerError, err, http.StatusInternalServerError, "Failed to open file: "+header.Filename)
			return
		}
		defer file.Close()

		fileReaders[i] = file
		filenames[i] = header.Filename
		fileSizes[i] = header.Size
	}

	// Batch upload
	assets, errors := h.assetService.BatchUploadAssets(
		c.Request.Context(),
		fileReaders,
		filenames,
		fileSizes,
		ownerID,
	)

	// Prepare response
	results := make([]map[string]interface{}, len(assets))
	for i := range assets {
		if errors[i] != nil {
			results[i] = map[string]interface{}{
				"filename": filenames[i],
				"error":    errors[i].Error(),
				"success":  false,
			}
		} else {
			results[i] = map[string]interface{}{
				"filename": filenames[i],
				"asset":    assets[i],
				"success":  true,
			}
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
