package handler

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"server/internal/api"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/processors"
	"server/internal/queue/jobs"
	"server/internal/service"
	"server/internal/storage"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/riverqueue/river"
)

// UploadResponse represents the response structure for file upload
type UploadResponse struct {
	TaskID      int64  `json:"task_id" example:"12345"`
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
	ContentHash string  `json:"content_hash"`        // MLService-provided content hash
	TaskID      *int64  `json:"task_id,omitempty"`   // Only present for successful uploads
	Status      *string `json:"status,omitempty"`    // Only present for successful uploads
	Size        *int64  `json:"size,omitempty"`      // Only present for successful uploads
	Message     *string `json:"message,omitempty"`   // Status message
	Error       *string `json:"error,omitempty"`     // Only present for failed uploads
}

// AssetListResponse represents the response structure for asset listing
type AssetListResponse struct {
	Assets []AssetDTO `json:"assets"`
	Limit  int        `json:"limit" example:"20"`
	Offset int        `json:"offset" example:"0"`
}

// AssetDTO represents a simplified asset payload for APIs and docs
type AssetDTO struct {
	AssetID          string                   `json:"asset_id"`
	OwnerID          *int32                   `json:"owner_id"`
	Type             string                   `json:"type"`
	OriginalFilename string                   `json:"original_filename"`
	StoragePath      string                   `json:"storage_path"`
	MimeType         string                   `json:"mime_type"`
	FileSize         int64                    `json:"file_size"`
	Hash             *string                  `json:"hash"`
	Width            *int32                   `json:"width"`
	Height           *int32                   `json:"height"`
	Duration         *float64                 `json:"duration"`
	UploadTime       time.Time                `json:"upload_time"`
	IsDeleted        *bool                    `json:"is_deleted"`
	DeletedAt        *time.Time               `json:"deleted_at,omitempty"`
	Metadata         dbtypes.SpecificMetadata `json:"specific_metadata" swaggertype:"object"`
}

// toAssetDTO maps repo.Asset to AssetDTO
func toAssetDTO(a repo.Asset) AssetDTO {
	var id string
	if a.AssetID.Valid {
		id = uuid.UUID(a.AssetID.Bytes).String()
	}
	var uploadTime time.Time
	if a.UploadTime.Valid {
		uploadTime = a.UploadTime.Time
	}
	var deletedAt *time.Time
	if a.DeletedAt.Valid {
		t := a.DeletedAt.Time
		deletedAt = &t
	}
	return AssetDTO{
		AssetID:          id,
		OwnerID:          a.OwnerID,
		Type:             a.Type,
		OriginalFilename: a.OriginalFilename,
		StoragePath:      a.StoragePath,
		MimeType:         a.MimeType,
		FileSize:         a.FileSize,
		Hash:             a.Hash,
		Width:            a.Width,
		Height:           a.Height,
		Duration:         a.Duration,
		UploadTime:       uploadTime,
		IsDeleted:        a.IsDeleted,
		DeletedAt:        deletedAt,
		Metadata:         a.SpecificMetadata,
	}
}

// UpdateAssetRequest represents the request structure for updating asset metadata
type UpdateAssetRequest struct {
	Metadata dbtypes.SpecificMetadata `json:"specific_metadata" swaggertype:"object"`
}

// UpdateRatingRequest represents the request structure for updating asset rating
type UpdateRatingRequest struct {
	Rating int `json:"rating" example:"5" validate:"min=0,max=5"`
}

// UpdateLikeRequest represents the request structure for updating asset like status
type UpdateLikeRequest struct {
	Liked bool `json:"liked" example:"true"`
}

// UpdateRatingAndLikeRequest represents the request structure for updating both rating and like status
type UpdateRatingAndLikeRequest struct {
	Rating int  `json:"rating" example:"5" validate:"min=0,max=5"`
	Liked  bool `json:"liked" example:"true"`
}

// UpdateDescriptionRequest represents the request structure for updating asset description
type UpdateDescriptionRequest struct {
	Description string `json:"description" example:"A beautiful sunset photo"`
}

// MessageResponse represents a simple message response
type MessageResponse struct {
	Message string `json:"message" example:"Operation completed successfully"`
}

// AssetTypesResponse represents the response structure for asset types
type AssetTypesResponse struct {
	Types []dbtypes.AssetType `json:"types"`
}

// FilenameFilter represents filename filtering options
type FilenameFilter struct {
	Value string `json:"value" example:"IMG_"`
	Mode  string `json:"mode" example:"startswith" enums:"contains,matches,startswith,endswith"`
}

// DateRange represents a date range filter
type DateRange struct {
	From *time.Time `json:"from,omitempty"`
	To   *time.Time `json:"to,omitempty"`
}

// AssetFilter represents comprehensive filtering options
type AssetFilter struct {
	RepositoryID *string         `json:"repository_id,omitempty" example:"550e8400-e29b-41d4-a716-446655440000"`
	Type         *string         `json:"type,omitempty" example:"PHOTO" enums:"PHOTO,VIDEO,AUDIO"`
	OwnerID      *int32          `json:"owner_id,omitempty" example:"123"`
	RAW          *bool           `json:"raw,omitempty" example:"true"`
	Rating       *int            `json:"rating,omitempty" example:"5" minimum:"0" maximum:"5"`
	Liked        *bool           `json:"liked,omitempty" example:"true"`
	Filename     *FilenameFilter `json:"filename,omitempty"`
	Date         *DateRange      `json:"date,omitempty"`
	CameraMake   *string         `json:"camera_make,omitempty" example:"Canon"`
	Lens         *string         `json:"lens,omitempty" example:"EF 50mm f/1.8"`
}

// FilterAssetsRequest represents the request structure for filtering assets
type FilterAssetsRequest struct {
	Filter AssetFilter `json:"filter"`
	Limit  int         `json:"limit" example:"20" minimum:"1" maximum:"100"`
	Offset int         `json:"offset" example:"0" minimum:"0"`
}

// SearchAssetsRequest represents the request structure for searching assets
type SearchAssetsRequest struct {
	Query      string `json:"query" binding:"required" example:"red bird on branch"`
	SearchType string `json:"search_type" binding:"required" example:"filename" enums:"filename,semantic"`

	Filter AssetFilter `json:"filter,omitempty"`
	Limit  int         `json:"limit" example:"20" minimum:"1" maximum:"100"`
	Offset int         `json:"offset" example:"0" minimum:"0"`
}

// OptionsResponse represents the response for filter options
type OptionsResponse struct {
	CameraMakes []string `json:"camera_makes"`
	Lenses      []string `json:"lenses"`
}

// AssetHandler handles HTTP requests for asset management
type AssetHandler struct {
	assetService   service.AssetService
	queries        *repo.Queries
	repoManager    storage.RepositoryManager
	stagingManager storage.StagingManager
	queueClient    *river.Client[pgx.Tx]
}

// NewAssetHandler creates a new AssetHandler instance
func NewAssetHandler(
	assetService service.AssetService,
	queries *repo.Queries,
	repoManager storage.RepositoryManager,
	stagingManager storage.StagingManager,
	queueClient *river.Client[pgx.Tx],
) *AssetHandler {
	return &AssetHandler{
		assetService:   assetService,
		queries:        queries,
		repoManager:    repoManager,
		stagingManager: stagingManager,
		queueClient:    queueClient,
	}
}

// UploadAsset handles asset upload requests
// @Summary Upload a single asset
// @Description Upload a single photo, video, audio file, or document to the system. The file is staged in a repository and queued for processing.
// @Tags assets
// @Accept multipart/form-data
// @Produce json
// @Param file formData file true "Asset file to upload"
// @Param repository_id formData string false "Repository UUID (optional, uses default if not provided)"
// @Param X-Content-Hash header string false "Client-calculated BLAKE3 hash of the file"
// @Success 200 {object} api.Result{data=UploadResponse} "Upload successful"
// @Failure 400 {object} api.Result "Bad request - no file provided or parse error"
// @Failure 500 {object} api.Result "Internal server error"
// @Router /assets [post]
func (h *AssetHandler) UploadAsset(c *gin.Context) {
	ctx := c.Request.Context()

	// Parse multipart form
	err := c.Request.ParseMultipartForm(32 << 20) // 32MB max
	if err != nil {
		api.GinBadRequest(c, err, "Failed to parse form")
		return
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		api.GinBadRequest(c, errors.New("no file provided"))
		return
	}
	defer file.Close()

	// Get or use default repository
	repositoryID := c.PostForm("repository_id")
	var repository repo.Repository
	if repositoryID != "" {
		repoUUID, err := uuid.Parse(repositoryID)
		if err != nil {
			api.GinBadRequest(c, err, "Invalid repository ID")
			return
		}
		repository, err = h.queries.GetRepository(ctx, pgtype.UUID{Bytes: repoUUID, Valid: true})
		if err != nil {
			api.GinNotFound(c, err, "Repository not found")
			return
		}
	} else {
		// Use first available repository as default
		repositories, err := h.queries.ListRepositories(ctx)
		if err != nil || len(repositories) == 0 {
			api.GinBadRequest(c, errors.New("no repository available"), "Please specify a repository_id or create a repository first")
			return
		}
		repository = repositories[0]
	}

	// Get client hash or calculate it
	clientHash := c.GetHeader("X-Content-Hash")
	if clientHash == "" {
		log.Println("Warning: No content hash provided by client, will calculate during processing")
		clientHash = "" // Will be calculated by processor
	}

	// Create staging file in repository
	stagingFile, err := h.stagingManager.CreateStagingFile(repository.Path, header.Filename)
	if err != nil {
		log.Printf("Failed to create staging file: %v", err)
		api.GinInternalError(c, err, "Upload failed")
		return
	}

	// Write uploaded content to staging file
	osFile, err := os.Create(stagingFile.Path)
	if err != nil {
		log.Printf("Failed to open staging file: %v", err)
		api.GinInternalError(c, err, "Upload failed")
		return
	}

	_, err = io.Copy(osFile, file)
	osFile.Close()
	if err != nil {
		log.Printf("Failed to copy file to staging: %v", err)
		os.Remove(stagingFile.Path)
		api.GinInternalError(c, err, "Upload failed")
		return
	}

	userID := c.GetString("user_id")
	if userID == "" {
		userID = "anonymous"
	}

	payload := processors.AssetPayload{
		ClientHash:   clientHash,
		StagedPath:   stagingFile.Path,
		UserID:       userID,
		Timestamp:    time.Now(),
		ContentType:  header.Header.Get("Content-Type"),
		FileName:     header.Filename,
		RepositoryID: uuid.UUID(repository.RepoID.Bytes).String(),
	}

	jobInsetResult, err := h.queueClient.Insert(ctx, jobs.ProcessAssetArgs{
		ClientHash:   payload.ClientHash,
		StagedPath:   payload.StagedPath,
		UserID:       payload.UserID,
		Timestamp:    payload.Timestamp,
		ContentType:  payload.ContentType,
		FileName:     payload.FileName,
		RepositoryID: payload.RepositoryID,
	}, &river.InsertOpts{Queue: "process_asset"})

	if err != nil {
		log.Printf("Failed to enqueue task: %v", err)
		api.GinInternalError(c, err, "Upload failed")
		return
	}
	if jobInsetResult == nil || jobInsetResult.Job == nil {
		log.Printf("Failed to enqueue task: empty result")
		api.GinInternalError(c, fmt.Errorf("enqueue failed"), "Upload failed")
		return
	}
	jobId := jobInsetResult.Job.ID
	log.Printf("Task %d enqueued for processing file %s in repository %s", jobId, header.Filename, repository.Name)

	response := UploadResponse{
		TaskID:      jobId,
		Status:      "processing",
		FileName:    header.Filename,
		Size:        header.Size,
		ContentHash: clientHash,
		Message:     fmt.Sprintf("File received and queued for processing in repository '%s'", repository.Name),
	}
	api.GinSuccess(c, response)
}

// BatchUploadAssets handles multiple asset uploads
// @Summary Batch upload assets
// @Description Batch upload multiple assets to a repository. Each file part's field name should be its BLAKE3 content hash. All files are staged and queued for processing.
// @Tags assets
// @Accept multipart/form-data
// @Produce json
// @Param repository_id formData string false "Repository UUID (optional, uses default if not provided)"
// @Success 200 {object} api.Result{data=BatchUploadResponse} "Batch upload completed"
// @Failure 400 {object} api.Result "Bad request - no files provided or parse error"
// @Failure 500 {object} api.Result "Internal server error"
// @Router /assets/batch [post]
func (h *AssetHandler) BatchUploadAssets(c *gin.Context) {
	ctx := c.Request.Context()

	err := c.Request.ParseMultipartForm(256 << 20) // 256MB max for batch
	if err != nil {
		api.GinBadRequest(c, err, "Failed to parse form")
		return
	}

	form := c.Request.MultipartForm
	if form == nil || len(form.File) == 0 {
		api.GinBadRequest(c, errors.New("no files provided"), "No files provided")
		return
	}

	// Get or use default repository
	repositoryID := c.PostForm("repository_id")
	var repository repo.Repository
	if repositoryID != "" {
		repoUUID, err := uuid.Parse(repositoryID)
		if err != nil {
			api.GinBadRequest(c, err, "Invalid repository ID")
			return
		}
		repository, err = h.queries.GetRepository(ctx, pgtype.UUID{Bytes: repoUUID, Valid: true})
		if err != nil {
			api.GinNotFound(c, err, "Repository not found")
			return
		}
	} else {
		// Use first available repository as default
		repositories, err := h.queries.ListRepositories(ctx)
		if err != nil || len(repositories) == 0 {
			api.GinBadRequest(c, errors.New("no repository available"), "Please specify a repository_id or create a repository first")
			return
		}
		repository = repositories[0]
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

		// Create staging file in repository
		stagingFile, err := h.stagingManager.CreateStagingFile(repository.Path, header.Filename)
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

		osFile, err := os.Create(stagingFile.Path)
		if err != nil {
			log.Printf("Failed to open staging file: %v", err)
			errMsg := "Failed to open staging file: " + err.Error()
			results = append(results, BatchUploadResult{
				Success:     false,
				FileName:    header.Filename,
				ContentHash: clientHash,
				Error:       &errMsg,
			})
			file.Close()
			continue
		}

		_, err = io.Copy(osFile, file)
		osFile.Close()
		file.Close()
		if err != nil {
			log.Printf("Failed to copy file to staging: %v", err)
			os.Remove(stagingFile.Path)
			errMsg := "Failed to copy file to staging: " + err.Error()
			results = append(results, BatchUploadResult{
				Success:     false,
				FileName:    header.Filename,
				ContentHash: clientHash,
				Error:       &errMsg,
			})
			continue
		}

		payload := processors.AssetPayload{
			ClientHash:   clientHash,
			StagedPath:   stagingFile.Path,
			UserID:       userID,
			Timestamp:    time.Now(),
			ContentType:  header.Header.Get("Content-Type"),
			FileName:     header.Filename,
			RepositoryID: uuid.UUID(repository.RepoID.Bytes).String(),
		}

		jobInsetResult, err := h.queueClient.Insert(ctx, jobs.ProcessAssetArgs{
			ClientHash:   payload.ClientHash,
			StagedPath:   payload.StagedPath,
			UserID:       payload.UserID,
			Timestamp:    payload.Timestamp,
			ContentType:  payload.ContentType,
			FileName:     payload.FileName,
			RepositoryID: payload.RepositoryID,
		}, &river.InsertOpts{Queue: "process_asset"})

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
		if jobInsetResult == nil || jobInsetResult.Job == nil {
			errMsg := "Failed to enqueue task: empty result"
			results = append(results, BatchUploadResult{
				Success:     false,
				FileName:    header.Filename,
				ContentHash: clientHash,
				Error:       &errMsg,
			})
			continue
		}
		jobId := jobInsetResult.Job.ID

		log.Printf("Task %d enqueued for processing file %s in repository %s", jobId, header.Filename, repository.Name)

		taskID := jobId
		status := "processing"
		size := header.Size
		message := fmt.Sprintf("File received and queued for processing in repository '%s'", repository.Name)

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

	api.GinSuccess(c, BatchUploadResponse{Results: results})
}

// GetAsset retrieves a single asset by ID
// @Summary Get asset by ID
// @Description Retrieve detailed information about a specific asset. Optionally include thumbnails, tags, albums, and species predictions.
// @Tags assets
// @Accept json
// @Produce json
// @Param id path string true "Asset ID (UUID format)" example("550e8400-e29b-41d4-a716-446655440000")
// @Param include_thumbnails query bool false "Include thumbnails" default(true)
// @Param include_tags query bool false "Include tags" default(true)
// @Param include_albums query bool false "Include albums" default(true)
// @Param include_species query bool false "Include species predictions" default(true)
// @Success 200 {object} api.Result{data=AssetDTO} "Asset details with optional relationships"
// @Failure 400 {object} api.Result "Invalid asset ID"
// @Failure 404 {object} api.Result "Asset not found"
// @Router /assets/{id} [get]
func (h *AssetHandler) GetAsset(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		api.GinBadRequest(c, err, "Invalid asset ID")
		return
	}

	// Parse include options with defaults
	includeThumbnails := c.DefaultQuery("include_thumbnails", "true") == "true"
	includeTags := c.DefaultQuery("include_tags", "true") == "true"
	includeAlbums := c.DefaultQuery("include_albums", "true") == "true"
	includeSpecies := c.DefaultQuery("include_species", "true") == "true"

	asset, err := h.assetService.GetAssetWithOptions(c.Request.Context(), id, includeThumbnails, includeTags, includeAlbums, includeSpecies)
	if err != nil {
		api.GinNotFound(c, err, "Asset not found")
		return
	}

	api.GinSuccess(c, asset)
}

// ListAssets retrieves assets with optional filtering
// @Summary List assets
// @Description Retrieve a paginated list of assets. Filter by type(s) or owner. Assets are sorted by taken_time (photo capture time or video record time). At least one filter parameter is required.
// @Tags assets
// @Accept json
// @Produce json
// @Param type query string false "Single asset type filter" Enums(PHOTO,VIDEO,AUDIO,DOCUMENT) example("PHOTO")
// @Param types query string false "Multiple asset types filter (comma-separated)" example("PHOTO,VIDEO")
// @Param owner_id query int false "Filter by owner ID" example(123)
// @Param limit query int false "Maximum number of results (max 100)" default(20) example(20)
// @Param offset query int false "Number of results to skip for pagination" default(0) example(0)
// @Param sort_order query string false "Sort order by taken_time" Enums(asc,desc) default("desc") example("desc")
// @Success 200 {object} api.Result{data=AssetListResponse} "Assets retrieved successfully"
// @Failure 400 {object} api.Result "Invalid parameters"
// @Failure 500 {object} api.Result "Internal server error"
// @Router /assets [get]
func (h *AssetHandler) ListAssets(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "20")
	offsetStr := c.DefaultQuery("offset", "0")
	typeStr := c.Query("type")
	typesStr := c.Query("types")
	ownerIDStr := c.Query("owner_id")
	sortOrder := c.DefaultQuery("sort_order", "desc")

	limit, _ := strconv.Atoi(limitStr)
	offset, _ := strconv.Atoi(offsetStr)

	if limit > 100 {
		limit = 100
	}

	// Validate sort parameter
	if sortOrder != "asc" && sortOrder != "desc" {
		api.GinBadRequest(c, errors.New("invalid sort_order"), "sort_order must be 'asc' or 'desc'")
		return
	}

	ctx := c.Request.Context()
	var assets []repo.Asset
	var err error

	// Determine asset types to filter by
	var assetTypes []string
	if typesStr != "" {
		// Handle comma-separated types parameter
		typeList := strings.Split(typesStr, ",")
		for _, t := range typeList {
			t = strings.TrimSpace(t)
			assetType := dbtypes.AssetType(t)
			if !assetType.Valid() {
				api.GinBadRequest(c, errors.New("invalid asset type in types"), fmt.Sprintf("Invalid asset type: %s", t))
				return
			}
			assetTypes = append(assetTypes, *assetType.String())
		}
	} else if typeStr != "" {
		// Handle single type parameter for backward compatibility
		assetType := dbtypes.AssetType(typeStr)
		if !assetType.Valid() {
			api.GinBadRequest(c, errors.New("invalid asset type"), "Invalid asset type")
			return
		}
		assetTypes = append(assetTypes, *assetType.String())
	}

	switch {
	case ownerIDStr != "":
		ownerID, parseErr := strconv.Atoi(ownerIDStr)
		if parseErr != nil {
			api.GinBadRequest(c, parseErr, "Invalid owner_id")
			return
		}
		if len(assetTypes) > 0 {
			assets, err = h.assetService.GetAssetsByOwnerAndTypes(ctx, ownerID, assetTypes, sortOrder, limit, offset)
		} else {
			assets, err = h.assetService.GetAssetsByOwnerSorted(ctx, ownerID, sortOrder, limit, offset)
		}

	case len(assetTypes) > 0:
		assets, err = h.assetService.GetAssetsByTypesSorted(ctx, assetTypes, sortOrder, limit, offset)

	default:
		api.GinBadRequest(c, errors.New("missing query parameters"), "Please specify type, types, or owner_id")
		return
	}

	if err != nil {
		log.Printf("Failed to retrieve assets: %v", err)
		api.GinInternalError(c, err, "Failed to retrieve assets")
		return
	}

	dtos := make([]AssetDTO, len(assets))
	for i, a := range assets {
		dtos[i] = toAssetDTO(a)
	}
	response := AssetListResponse{
		Assets: dtos,
		Limit:  limit,
		Offset: offset,
	}
	api.GinSuccess(c, response)
}

// GetAssetThumbnail retrieves a thumbnail for a specific asset by asset ID and size
// @Summary Get asset thumbnail
// @Description Retrieve a specific thumbnail image for an asset by asset ID and size parameter. Returns the image file directly.
// @Tags assets
// @Produce image/jpeg
// @Param id path string true "Asset ID (UUID format)" example("550e8400-e29b-41d4-a716-446655440000")
// @Param size query string false "Thumbnail size" default(medium) Enums(small,medium,large)
// @Success 200 {file} string "Thumbnail image file"
// @Failure 400 {object} api.Result "Invalid asset ID or size parameter"
// @Failure 404 {object} api.Result "Asset or thumbnail not found"
// @Failure 500 {object} api.Result "Internal server error"
// @Router /assets/{id}/thumbnail [get]
func (h *AssetHandler) GetAssetThumbnail(c *gin.Context) {
	// Parse asset ID from URL parameter
	idStr := c.Param("id")
	assetID, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid asset ID"})
		return
	}

	// Get size parameter from query (default to "medium")
	size := c.DefaultQuery("size", "medium")

	// Validate size parameter
	if size != "small" && size != "medium" && size != "large" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid size parameter. Must be 'small', 'medium', or 'large'"})
		return
	}

	// First verify asset exists without loading full data
	_, err = h.assetService.GetAssetWithOptions(c.Request.Context(), assetID, false, false, false, false)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Asset not found"})
			return
		}
		log.Printf("Failed to verify asset existence: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to access asset"})
		return
	}

	// Get thumbnail from service
	thumbnail, err := h.assetService.GetThumbnailByAssetIDAndSize(c.Request.Context(), assetID, size)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Thumbnail not found"})
			return
		}
		log.Printf("Failed to retrieve thumbnail metadata: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve thumbnail"})
		return
	}

	// Thumbnail paths are repository-relative, stored in .lumilio/assets/thumbnails/
	// For now, we need to find which repository this asset belongs to
	// Since we don't track asset->repository mapping yet, we'll try to construct the path
	// This is a temporary solution until we implement proper file_records lookup
	fullPath := thumbnail.StoragePath
	if !filepath.IsAbs(fullPath) {
		// Try to find the repository by listing all repositories
		repositories, err := h.queries.ListRepositories(c.Request.Context())
		if err == nil && len(repositories) > 0 {
			// Use first repository for now - proper implementation needs file_records
			fullPath = filepath.Join(repositories[0].Path, thumbnail.StoragePath)
		}
	}

	// Get file info for proper cache control
	fileInfo, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Thumbnail file not found"})
			return
		}
		log.Printf("Failed to get file info for %s: %v", fullPath, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to access thumbnail file"})
		return
	}

	// Content-based ETag for cache consistency
	etag := fmt.Sprintf(`"%s-%s-%d"`,
		thumbnail.AssetID.String()[:8], // Short asset ID for uniqueness
		thumbnail.Size,
		fileInfo.ModTime().Unix())

	// Production-ready cache headers
	c.Header("ETag", etag)
	c.Header("Cache-Control", "public, max-age=86400, must-revalidate") // 24h cache with validation
	c.Header("Vary", "Accept-Encoding")

	// Check conditional request
	if match := c.GetHeader("If-None-Match"); match == etag {
		log.Printf("Request for asset %s thumbnail (%s) - 304 Not Modified (ETag: %s)", assetID.String(), size, etag)
		c.Status(http.StatusNotModified)
		return
	}

	log.Printf("Request for asset %s thumbnail (%s), serving file: %s (ETag: %s)", assetID.String(), size, fullPath, etag)

	c.File(fullPath)
}

// GetOriginalFile serves the original file content by asset ID
// @Summary Get original file
// @Description Serve the original file content for an asset by asset ID. Returns the file as an octet-stream.
// @Tags assets
// @Produce application/octet-stream
// @Param id path string true "Asset ID (UUID format)" example("550e8400-e29b-41d4-a716-446655440000")
// @Success 200 {file} file "Original file content"
// @Failure 400 {object} api.Result "Invalid asset ID"
// @Failure 404 {object} api.Result "Asset not found"
// @Failure 500 {object} api.Result "Internal server error"
// @Router /assets/{id}/original [get]
func (h *AssetHandler) GetOriginalFile(c *gin.Context) {
	ctx := c.Request.Context()

	// Parse asset ID from URL parameter
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		api.GinBadRequest(c, err, "Invalid asset ID")
		return
	}

	// Get asset metadata from service
	asset, err := h.assetService.GetAsset(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			api.GinNotFound(c, err, "Asset not found")
			return
		}
		log.Printf("Failed to retrieve asset metadata: %v", err)
		api.GinInternalError(c, err, "Failed to retrieve asset")
		return
	}

	// Construct full file path from repository-relative storage path
	// Asset paths are stored relative to repository root (e.g., "inbox/2024/01/photo.jpg")
	fullPath := asset.StoragePath
	if !filepath.IsAbs(fullPath) {
		// Try to find the repository by listing all repositories
		repositories, err := h.queries.ListRepositories(ctx)
		if err == nil && len(repositories) > 0 {
			// Use first repository for now - proper implementation needs file_records
			fullPath = filepath.Join(repositories[0].Path, asset.StoragePath)
		}
	}

	// Check if file exists
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		log.Printf("Original file not found at path: %s", fullPath)
		api.GinNotFound(c, err, "Original file not found")
		return
	}

	// Set appropriate headers
	c.Header("Cache-Control", "public, max-age=86400") // Cache for 1 day
	c.Header("Content-Type", asset.MimeType)
	c.Header("Content-Disposition", fmt.Sprintf("inline; filename=\"%s\"", asset.OriginalFilename))

	// Serve the file
	c.File(fullPath)
}

// GetWebVideo serves the web-optimized video version by asset ID
// @Summary Get web-optimized video
// @Description Serve the web-optimized MP4 video version for an asset by asset ID.
// @Tags assets
// @Produce video/mp4
// @Param id path string true "Asset ID (UUID format)" example("550e8400-e29b-41d4-a716-446655440000")
// @Success 200 {file} file "Web-optimized video file"
// @Failure 400 {object} api.Result "Invalid asset ID"
// @Failure 404 {object} api.Result "Asset not found or not a video"
// @Failure 500 {object} api.Result "Internal server error"
// @Router /assets/{id}/video/web [get]
func (h *AssetHandler) GetWebVideo(c *gin.Context) {
	ctx := c.Request.Context()

	// Parse asset ID from URL parameter
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		api.GinBadRequest(c, err, "Invalid asset ID")
		return
	}

	// Get asset metadata from service
	asset, err := h.assetService.GetAsset(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			api.GinNotFound(c, err, "Asset not found")
			return
		}
		log.Printf("Failed to retrieve asset metadata: %v", err)
		api.GinInternalError(c, err, "Failed to retrieve asset")
		return
	}

	// Check if asset is a video
	if asset.Type != "VIDEO" {
		api.GinBadRequest(c, fmt.Errorf("asset is not a video"), "Asset is not a video")
		return
	}

	// Get repository path for this asset
	repositories, err := h.queries.ListRepositories(ctx)
	if err != nil || len(repositories) == 0 {
		api.GinInternalError(c, err, "Failed to access repository")
		return
	}
	repoPath := repositories[0].Path

	// Construct web video file path in .lumilio/assets/videos/web/
	ext := filepath.Ext(asset.OriginalFilename)
	nameWithoutExt := strings.TrimSuffix(asset.OriginalFilename, ext)
	webVideoFilename := fmt.Sprintf("%s_web.mp4", nameWithoutExt)
	webVideoPath := filepath.Join(storage.DefaultStructure.VideosDir, "web", webVideoFilename)
	fullPath := filepath.Join(repoPath, webVideoPath)

	// Check if web version exists, fallback to original
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		// Fallback to original file
		fullPath = asset.StoragePath
		if !filepath.IsAbs(fullPath) {
			fullPath = filepath.Join(repoPath, asset.StoragePath)
		}
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			log.Printf("Video file not found at path: %s", fullPath)
			api.GinNotFound(c, err, "Video file not found")
			return
		}
	}

	// Set appropriate headers for video streaming
	c.Header("Cache-Control", "public, max-age=86400") // Cache for 1 day
	c.Header("Content-Type", "video/mp4")
	c.Header("Accept-Ranges", "bytes") // Enable range requests for video seeking

	// Serve the file
	c.File(fullPath)
}

// GetWebAudio serves the web-optimized audio version by asset ID
// @Summary Get web-optimized audio
// @Description Serve the web-optimized MP3 audio version for an asset by asset ID.
// @Tags assets
// @Produce audio/mpeg
// @Param id path string true "Asset ID (UUID format)" example("550e8400-e29b-41d4-a716-446655440000")
// @Success 200 {file} file "Web-optimized audio file"
// @Failure 400 {object} api.Result "Invalid asset ID"
// @Failure 404 {object} api.Result "Asset not found or not audio"
// @Failure 500 {object} api.Result "Internal server error"
// @Router /assets/{id}/audio/web [get]
func (h *AssetHandler) GetWebAudio(c *gin.Context) {
	ctx := c.Request.Context()

	// Parse asset ID from URL parameter
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		api.GinBadRequest(c, err, "Invalid asset ID")
		return
	}

	// Get asset metadata from service
	asset, err := h.assetService.GetAsset(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			api.GinNotFound(c, err, "Asset not found")
			return
		}
		log.Printf("Failed to retrieve asset metadata: %v", err)
		api.GinInternalError(c, err, "Failed to retrieve asset")
		return
	}

	// Check if asset is audio
	if asset.Type != "AUDIO" {
		api.GinBadRequest(c, fmt.Errorf("asset is not audio"), "Asset is not audio")
		return
	}

	// Get repository path for this asset
	repositories, err := h.queries.ListRepositories(ctx)
	if err != nil || len(repositories) == 0 {
		api.GinInternalError(c, err, "Failed to access repository")
		return
	}
	repoPath := repositories[0].Path

	// Construct web audio file path in .lumilio/assets/audios/web/
	ext := filepath.Ext(asset.OriginalFilename)
	nameWithoutExt := strings.TrimSuffix(asset.OriginalFilename, ext)
	webAudioFilename := fmt.Sprintf("%s_web.mp3", nameWithoutExt)
	webAudioPath := filepath.Join(storage.DefaultStructure.AudiosDir, "web", webAudioFilename)
	fullPath := filepath.Join(repoPath, webAudioPath)

	// Check if web version exists, fallback to original
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		// Fallback to original file
		fullPath = asset.StoragePath
		if !filepath.IsAbs(fullPath) {
			fullPath = filepath.Join(repoPath, asset.StoragePath)
		}
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			log.Printf("Audio file not found at path: %s", fullPath)
			api.GinNotFound(c, err, "Audio file not found")
			return
		}
	}

	// Set appropriate headers for audio streaming
	c.Header("Cache-Control", "public, max-age=86400") // Cache for 1 day
	c.Header("Content-Type", "audio/mpeg")
	c.Header("Accept-Ranges", "bytes") // Enable range requests for audio seeking

	// Serve the file
	c.File(fullPath)
}

// UpdateAsset updates asset metadata
// @Summary Update asset metadata
// @Description Update the specific metadata of an asset (e.g., photo EXIF data, video metadata).
// @Tags assets
// @Accept json
// @Produce json
// @Param id path string true "Asset ID (UUID format)" example("550e8400-e29b-41d4-a716-446655440000")
// @Param metadata body UpdateAssetRequest true "Updated metadata"
// @Success 200 {object} api.Result{data=MessageResponse} "Asset updated successfully"
// @Failure 400 {object} api.Result "Invalid asset ID or request body"
// @Failure 500 {object} api.Result "Internal server error"
// @Router /assets/{id} [put]
func (h *AssetHandler) UpdateAsset(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		api.GinBadRequest(c, err, "Invalid asset ID")
		return
	}

	var updateData UpdateAssetRequest
	if err := c.ShouldBindJSON(&updateData); err != nil {
		api.GinBadRequest(c, err, "Invalid request body")
		return
	}

	err = h.assetService.UpdateAssetMetadata(c.Request.Context(), id, updateData.Metadata)
	if err != nil {
		log.Printf("Failed to update asset metadata: %v", err)
		api.GinInternalError(c, err, "Failed to update asset")
		return
	}

	api.GinSuccess(c, MessageResponse{Message: "Asset updated successfully"})
}

// DeleteAsset deletes an asset
// @Summary Delete asset
// @Description Soft delete an asset by marking it as deleted. The physical file is not removed.
// @Tags assets
// @Accept json
// @Produce json
// @Param id path string true "Asset ID (UUID format)" example("550e8400-e29b-41d4-a716-446655440000")
// @Success 200 {object} api.Result{data=MessageResponse} "Asset deleted successfully"
// @Failure 400 {object} api.Result "Invalid asset ID format"
// @Failure 500 {object} api.Result "Internal server error"
// @Router /assets/{id} [delete]
func (h *AssetHandler) DeleteAsset(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		api.GinBadRequest(c, err, "Invalid asset ID")
		return
	}

	err = h.assetService.DeleteAsset(c.Request.Context(), id)
	if err != nil {
		log.Printf("Failed to delete asset: %v", err)
		api.GinInternalError(c, err, "Failed to delete asset")
		return
	}

	api.GinSuccess(c, MessageResponse{Message: "Asset deleted successfully"})
}

// AddAssetToAlbum adds an asset to an album
// @Summary Add asset to album
// @Description Associate an asset with a specific album by asset ID and album ID.
// @Tags assets
// @Accept json
// @Produce json
// @Param id path string true "Asset ID (UUID format)" example("550e8400-e29b-41d4-a716-446655440000")
// @Param albumId path int true "Album ID" example(123)
// @Success 200 {object} api.Result{data=MessageResponse} "Asset added to album successfully"
// @Failure 400 {object} api.Result "Invalid asset ID or album ID"
// @Failure 500 {object} api.Result "Internal server error"
// @Router /assets/{id}/albums/{albumId} [post]
func (h *AssetHandler) AddAssetToAlbum(c *gin.Context) {
	assetID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		api.GinBadRequest(c, err, "Invalid asset ID")
		return
	}

	albumID, err := strconv.Atoi(c.Param("albumId"))
	if err != nil {
		api.GinBadRequest(c, err, "Invalid album ID")
		return
	}

	err = h.assetService.AddAssetToAlbum(c.Request.Context(), assetID, albumID)
	if err != nil {
		log.Printf("Failed to add asset to album: %v", err)
		api.GinInternalError(c, err, "Failed to add asset to album")
		return
	}

	api.GinSuccess(c, MessageResponse{Message: "Asset added to album successfully"})
}

// GetAssetTypes returns available asset types
// @Summary Get supported asset types
// @Description Retrieve a list of all supported asset types in the system.
// @Tags assets
// @Accept json
// @Produce json
// @Success 200 {object} api.Result{data=AssetTypesResponse} "Asset types retrieved successfully"
// @Router /assets/types [get]
func (h *AssetHandler) GetAssetTypes(c *gin.Context) {
	types := []dbtypes.AssetType{
		dbtypes.AssetTypePhoto,
		dbtypes.AssetTypeVideo,
		dbtypes.AssetTypeAudio,
	}

	api.GinSuccess(c, AssetTypesResponse{Types: types})
}

// FilterAssets handles asset filtering with complex filters
// @Summary Filter assets
// @Description Filter assets using comprehensive filtering options including repository selection, RAW, rating, liked status, filename patterns, date ranges, camera make, and lens
// @Tags assets
// @Accept json
// @Produce json
// @Param request body FilterAssetsRequest true "Filter criteria"
// @Success 200 {object} api.Result{data=AssetListResponse} "Assets filtered successfully"
// @Failure 400 {object} api.Result "Invalid request parameters"
// @Failure 500 {object} api.Result "Internal server error"
// @Router /assets/filter [post]
func (h *AssetHandler) FilterAssets(c *gin.Context) {
	var req FilterAssetsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.GinBadRequest(c, err, "Invalid request data")
		return
	}

	// Validate and set defaults
	if req.Limit <= 0 || req.Limit > 100 {
		req.Limit = 20
	}
	if req.Offset < 0 {
		req.Offset = 0
	}

	ctx := c.Request.Context()
	filter := req.Filter

	// Convert filter parameters for SQL query
	var typePtr *string
	if filter.Type != nil {
		typePtr = filter.Type
	}

	var filenameVal, filenameMode *string
	if filter.Filename != nil {
		filenameVal = &filter.Filename.Value
		filenameMode = &filter.Filename.Mode
	}

	var dateFrom, dateTo *time.Time
	if filter.Date != nil {
		dateFrom = filter.Date.From
		dateTo = filter.Date.To
	}

	assets, err := h.assetService.FilterAssets(ctx,
		filter.RepositoryID, typePtr, filter.OwnerID, filenameVal, filenameMode,
		dateFrom, dateTo, filter.RAW, filter.Rating, filter.Liked,
		filter.CameraMake, filter.Lens, req.Limit, req.Offset)

	if err != nil {
		log.Printf("Failed to filter assets: %v", err)
		api.GinInternalError(c, err, "Failed to filter assets")
		return
	}

	dtos := make([]AssetDTO, len(assets))
	for i, a := range assets {
		dtos[i] = toAssetDTO(a)
	}

	response := AssetListResponse{
		Assets: dtos,
		Limit:  req.Limit,
		Offset: req.Offset,
	}
	api.GinSuccess(c, response)
}

// SearchAssets handles both filename and semantic search with optional filtering
// @Summary Search assets
// @Description Search assets using either filename matching or semantic vector search. Can be combined with comprehensive filters including repository selection.
// @Tags assets
// @Accept json
// @Produce json
// @Param request body SearchAssetsRequest true "Search criteria"
// @Success 200 {object} api.Result{data=AssetListResponse} "Assets found successfully"
// @Failure 400 {object} api.Result "Invalid request parameters"
// @Failure 500 {object} api.Result "Internal server error"
// @Router /assets/search [post]
func (h *AssetHandler) SearchAssets(c *gin.Context) {
	var req SearchAssetsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.GinBadRequest(c, err, "Invalid request data")
		return
	}

	// Validate search type
	if req.SearchType != "filename" && req.SearchType != "semantic" {
		api.GinBadRequest(c, errors.New("invalid search type"), "Search type must be 'filename' or 'semantic'")
		return
	}

	// Validate and set defaults
	if req.Limit <= 0 || req.Limit > 100 {
		req.Limit = 20
	}
	if req.Offset < 0 {
		req.Offset = 0
	}

	ctx := c.Request.Context()
	filter := req.Filter

	// Convert filter parameters for SQL query
	var typePtr *string
	if filter.Type != nil {
		typePtr = filter.Type
	}

	var filenameVal, filenameMode *string
	if filter.Filename != nil {
		filenameVal = &filter.Filename.Value
		filenameMode = &filter.Filename.Mode
	}

	var dateFrom, dateTo *time.Time
	if filter.Date != nil {
		dateFrom = filter.Date.From
		dateTo = filter.Date.To
	}

	var assets []repo.Asset
	var err error

	if req.SearchType == "filename" {
		assets, err = h.assetService.SearchAssetsFilename(ctx, req.Query,
			filter.RepositoryID, typePtr, filter.OwnerID, filenameVal, filenameMode,
			dateFrom, dateTo, filter.RAW, filter.Rating, filter.Liked,
			filter.CameraMake, filter.Lens, req.Limit, req.Offset)
	} else {
		assets, err = h.assetService.SearchAssetsVector(ctx, req.Query,
			filter.RepositoryID, typePtr, filter.OwnerID, filenameVal, filenameMode,
			dateFrom, dateTo, filter.RAW, filter.Rating, filter.Liked,
			filter.CameraMake, filter.Lens, req.Limit, req.Offset)
	}

	if err != nil {
		log.Printf("Failed to search assets: %v", err)
		api.GinInternalError(c, err, "Failed to search assets")
		return
	}

	dtos := make([]AssetDTO, len(assets))
	for i, a := range assets {
		dtos[i] = toAssetDTO(a)
	}

	response := AssetListResponse{
		Assets: dtos,
		Limit:  req.Limit,
		Offset: req.Offset,
	}
	api.GinSuccess(c, response)
}

// GetFilterOptions returns available options for filters
// @Summary Get filter options
// @Description Get available camera makes and lenses for filter dropdowns
// @Tags assets
// @Accept json
// @Produce json
// @Success 200 {object} api.Result{data=OptionsResponse} "Filter options retrieved successfully"
// @Failure 500 {object} api.Result "Internal server error"
// @Router /assets/filter-options [get]
func (h *AssetHandler) GetFilterOptions(c *gin.Context) {
	ctx := c.Request.Context()

	cameraMakes, err := h.assetService.GetDistinctCameraMakes(ctx)
	if err != nil {
		log.Printf("Failed to get camera makes: %v", err)
		api.GinInternalError(c, err, "Failed to get filter options")
		return
	}

	lenses, err := h.assetService.GetDistinctLenses(ctx)
	if err != nil {
		log.Printf("Failed to get lenses: %v", err)
		api.GinInternalError(c, err, "Failed to get filter options")
		return
	}

	response := OptionsResponse{
		CameraMakes: cameraMakes,
		Lenses:      lenses,
	}
	api.GinSuccess(c, response)
}

// Rating Management Handlers

// UpdateAssetRating updates the rating of an asset
// @Summary Update asset rating
// @Description Update the rating (0-5) of a specific asset
// @Tags assets
// @Accept json
// @Produce json
// @Param id path string true "Asset ID"
// @Param rating body UpdateRatingRequest true "Rating data"
// @Success 200 {object} api.Result{data=MessageResponse} "Rating updated successfully"
// @Failure 400 {object} api.Result "Bad request"
// @Failure 404 {object} api.Result "Asset not found"
// @Failure 500 {object} api.Result "Internal server error"
// @Router /assets/{id}/rating [put]
func (h *AssetHandler) UpdateAssetRating(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		api.GinBadRequest(c, err, "Invalid asset ID")
		return
	}

	var req UpdateRatingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.GinBadRequest(c, err, "Invalid request body")
		return
	}

	if req.Rating < 0 || req.Rating > 5 {
		api.GinBadRequest(c, nil, "Rating must be between 0 and 5")
		return
	}

	err = h.assetService.UpdateAssetRating(c.Request.Context(), id, req.Rating)
	if err != nil {
		log.Printf("Failed to update asset rating: %v", err)
		api.GinInternalError(c, err, "Failed to update rating")
		return
	}

	api.GinSuccess(c, MessageResponse{Message: "Rating updated successfully"})
}

// UpdateAssetLike updates the like status of an asset
// @Summary Update asset like status
// @Description Update the like/favorite status of a specific asset
// @Tags assets
// @Accept json
// @Produce json
// @Param id path string true "Asset ID"
// @Param like body UpdateLikeRequest true "Like data"
// @Success 200 {object} api.Result{data=MessageResponse} "Like status updated successfully"
// @Failure 400 {object} api.Result "Bad request"
// @Failure 404 {object} api.Result "Asset not found"
// @Failure 500 {object} api.Result "Internal server error"
// @Router /assets/{id}/like [put]
func (h *AssetHandler) UpdateAssetLike(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		api.GinBadRequest(c, err, "Invalid asset ID")
		return
	}

	var req UpdateLikeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.GinBadRequest(c, err, "Invalid request body")
		return
	}

	err = h.assetService.UpdateAssetLike(c.Request.Context(), id, req.Liked)
	if err != nil {
		log.Printf("Failed to update asset like status: %v", err)
		api.GinInternalError(c, err, "Failed to update like status")
		return
	}

	api.GinSuccess(c, MessageResponse{Message: "Like status updated successfully"})
}

// UpdateAssetRatingAndLike updates both rating and like status of an asset
// @Summary Update asset rating and like status
// @Description Update both the rating (0-5) and like/favorite status of a specific asset
// @Tags assets
// @Accept json
// @Produce json
// @Param id path string true "Asset ID"
// @Param data body UpdateRatingAndLikeRequest true "Rating and like data"
// @Success 200 {object} api.Result{data=MessageResponse} "Rating and like status updated successfully"
// @Failure 400 {object} api.Result "Bad request"
// @Failure 404 {object} api.Result "Asset not found"
// @Failure 500 {object} api.Result "Internal server error"
// @Router /assets/{id}/rating-and-like [put]
func (h *AssetHandler) UpdateAssetRatingAndLike(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		api.GinBadRequest(c, err, "Invalid asset ID")
		return
	}

	var req UpdateRatingAndLikeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.GinBadRequest(c, err, "Invalid request body")
		return
	}

	if req.Rating < 0 || req.Rating > 5 {
		api.GinBadRequest(c, nil, "Rating must be between 0 and 5")
		return
	}

	err = h.assetService.UpdateAssetRatingAndLike(c.Request.Context(), id, req.Rating, req.Liked)
	if err != nil {
		log.Printf("Failed to update asset rating and like status: %v", err)
		api.GinInternalError(c, err, "Failed to update rating and like status")
		return
	}

	api.GinSuccess(c, MessageResponse{Message: "Rating and like status updated successfully"})
}

// UpdateAssetDescription updates the description of an asset
// @Summary Update asset description
// @Description Update the description/comment of a specific asset
// @Tags assets
// @Accept json
// @Produce json
// @Param id path string true "Asset ID"
// @Param description body UpdateDescriptionRequest true "Description data"
// @Success 200 {object} api.Result{data=MessageResponse} "Description updated successfully"
// @Failure 400 {object} api.Result "Bad request"
// @Failure 404 {object} api.Result "Asset not found"
// @Failure 500 {object} api.Result "Internal server error"
// @Router /assets/{id}/description [put]
func (h *AssetHandler) UpdateAssetDescription(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		api.GinBadRequest(c, err, "Invalid asset ID")
		return
	}

	var req UpdateDescriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.GinBadRequest(c, err, "Invalid request body")
		return
	}

	err = h.assetService.UpdateAssetDescription(c.Request.Context(), id, req.Description)
	if err != nil {
		log.Printf("Failed to update asset description: %v", err)
		api.GinInternalError(c, err, "Failed to update description")
		return
	}

	api.GinSuccess(c, MessageResponse{Message: "Description updated successfully"})
}

// GetAssetsByRating gets assets filtered by rating
// @Summary Get assets by rating
// @Description Get assets with a specific rating (0-5)
// @Tags assets
// @Accept json
// @Produce json
// @Param rating path int true "Rating (0-5)"
// @Param limit query int false "Number of assets to return" default(20)
// @Param offset query int false "Number of assets to skip" default(0)
// @Success 200 {object} api.Result{data=AssetListResponse} "Assets retrieved successfully"
// @Failure 400 {object} api.Result "Bad request"
// @Failure 500 {object} api.Result "Internal server error"
// @Router /assets/rating/{rating} [get]
func (h *AssetHandler) GetAssetsByRating(c *gin.Context) {
	ratingStr := c.Param("rating")
	rating, err := strconv.Atoi(ratingStr)
	if err != nil {
		api.GinBadRequest(c, err, "Invalid rating parameter")
		return
	}

	if rating < 0 || rating > 5 {
		api.GinBadRequest(c, nil, "Rating must be between 0 and 5")
		return
	}

	limit := 20
	offset := 0

	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	if offsetStr := c.Query("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	assets, err := h.assetService.GetAssetsByRating(c.Request.Context(), rating, limit, offset)
	if err != nil {
		log.Printf("Failed to get assets by rating: %v", err)
		api.GinInternalError(c, err, "Failed to retrieve assets")
		return
	}

	assetDTOs := make([]AssetDTO, len(assets))
	for i, asset := range assets {
		assetDTOs[i] = toAssetDTO(asset)
	}

	response := AssetListResponse{
		Assets: assetDTOs,
		Limit:  limit,
		Offset: offset,
	}

	api.GinSuccess(c, response)
}

// GetLikedAssets gets all liked/favorited assets
// @Summary Get liked assets
// @Description Get all assets that have been liked/favorited
// @Tags assets
// @Accept json
// @Produce json
// @Param limit query int false "Number of assets to return" default(20)
// @Param offset query int false "Number of assets to skip" default(0)
// @Success 200 {object} api.Result{data=AssetListResponse} "Liked assets retrieved successfully"
// @Failure 500 {object} api.Result "Internal server error"
// @Router /assets/liked [get]
func (h *AssetHandler) GetLikedAssets(c *gin.Context) {
	ctx := c.Request.Context()
	limit := 20
	offset := 0

	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	if offsetStr := c.Query("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	assets, err := h.assetService.GetLikedAssets(ctx, limit, offset)
	if err != nil {
		log.Printf("Failed to get liked assets: %v", err)
		api.GinInternalError(c, err, "Failed to retrieve liked assets")
		return
	}

	assetDTOs := make([]AssetDTO, len(assets))
	for i, asset := range assets {
		assetDTOs[i] = toAssetDTO(asset)
	}

	response := AssetListResponse{
		Assets: assetDTOs,
		Limit:  limit,
		Offset: offset,
	}

	api.GinSuccess(c, response)
}
