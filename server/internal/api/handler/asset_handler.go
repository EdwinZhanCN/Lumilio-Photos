package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"server/internal/api"
	"server/internal/api/dto"
	"server/internal/db/catalogtx"
	"server/internal/db/repo"
	"server/internal/service"
	"server/internal/storage"
	roelocations "server/internal/storage/roe/locations"
	"server/internal/utils/memory"
	"server/internal/utils/upload"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// AssetHandler handles HTTP requests for asset management
type AssetHandler struct {
	assetService     service.AssetService
	authService      *service.AuthService
	indexingService  service.AssetIndexingService
	stackService     service.StackService
	queries          *repo.Queries
	database         *sql.DB
	readerDatabase   *sql.DB
	writer           *catalogtx.Writer
	repoManager      storage.RepositoryManager
	stagingManager   storage.StagingManager
	files            *storage.RepositoryFSFactory
	locationResolver *roelocations.Resolver
	settingsService  service.SettingsService
	runtimeChecker   service.LumenService
	memoryMonitor    *memory.MemoryMonitor
	sessionManager   *upload.SessionManager
	chunkMerger      *upload.ChunkMerger
	uploadLimiter    chan struct{}
}

// NewAssetHandler creates a new AssetHandler instance
func NewAssetHandler(
	assetService service.AssetService,
	authService *service.AuthService,
	indexingService service.AssetIndexingService,
	stackService service.StackService,
	queries *repo.Queries,
	database *sql.DB,
	writer *catalogtx.Writer,
	repoManager storage.RepositoryManager,
	stagingManager storage.StagingManager,
	settingsService service.SettingsService,
	runtimeChecker service.LumenService,
	files *storage.RepositoryFSFactory,
) *AssetHandler {
	memoryMonitor := memory.NewMemoryMonitor()
	sessionManager := upload.NewSessionManager(30*time.Minute, queries, files)
	chunkMerger := upload.NewChunkMerger(stagingManager)
	// Increased limit to 32 to support HTTP/2 multiplexing for chunked uploads
	uploadLimiter := make(chan struct{}, 32)

	handler := &AssetHandler{
		assetService:    assetService,
		authService:     authService,
		indexingService: indexingService,
		stackService:    stackService,
		queries:         queries,
		database:        database,
		readerDatabase:  database,
		writer:          writer,
		repoManager:     repoManager,
		stagingManager:  stagingManager,
		files:           files,
		settingsService: settingsService,
		runtimeChecker:  runtimeChecker,
		memoryMonitor:   memoryMonitor,
		sessionManager:  sessionManager,
		chunkMerger:     chunkMerger,
		uploadLimiter:   uploadLimiter,
	}

	return handler
}

// SetReaderDatabase installs the query-only catalog connection used by
// polling/status endpoints. Tests and small tools may omit it, in which case
// the constructor's database connection remains the safe fallback.
func (h *AssetHandler) SetReaderDatabase(reader *sql.DB) {
	if h != nil && reader != nil {
		h.readerDatabase = reader
	}
}

// SetLocationResolver installs the single execution-time Asset-to-Location
// resolver shared by media serving, exports, and background processing.
func (h *AssetHandler) SetLocationResolver(resolver *roelocations.Resolver) {
	if h != nil {
		h.locationResolver = resolver
	}
}

// respondRepositoryError maps a resolveUploadRepository failure onto its HTTP response.
func (h *AssetHandler) respondRepositoryError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, errInvalidRepositoryID):
		api.WriteProblem(c, api.BadRequest(err))
	case errors.Is(err, errRepositoryNotFound):
		api.WriteProblem(c, api.NotFound(err))
	case writeUploadAdmissionError(c, err):
	default:
		api.WriteProblem(c, api.BadRequest(err))
	}
}

// GetAsset retrieves a single asset by ID
// @Summary Get asset by ID
// @Description Retrieve detailed information about a specific asset. Optionally include thumbnails, tags, albums, BioCLIP Species Recognition predictions, OCR Text Recognition results, Person Recognition results, and captions.
// @Tags assets
// @Accept json
// @Produce json
// @Param id path string true "Asset ID (UUID format)" example("550e8400-e29b-41d4-a716-446655440000")
// @Param include_thumbnails query bool false "Include thumbnails" default(true)
// @Param include_tags query bool false "Include tags" default(true)
// @Param include_albums query bool false "Include albums" default(true)
// @Param include_species query bool false "Include species predictions" default(true)
// @Param include_ocr query bool false "Include OCR Text Recognition results" default(false)
// @Param include_faces query bool false "Include Person Recognition results" default(false)
// @Success 200 {object} dto.AssetDetailDTO "Asset details with optional relationships"
// @Failure 400 {object} api.ProblemResponse "Invalid asset ID"
// @Failure 404 {object} api.ProblemResponse "Asset not found"
// @Router /api/v1/assets/{id} [get]
func (h *AssetHandler) GetAsset(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	if _, ok := h.getAuthorizedAssetForRead(c, id, "Authentication required to access this asset", "You don't have permission to access this asset"); !ok {
		return
	}

	// Parse include options. Thumbnails/tags/albums/species default on; the
	// heavier AI relations (OCR, faces) default off to avoid extra payload.
	includes := dto.AssetDetailIncludes{
		Thumbnails: c.DefaultQuery("include_thumbnails", "true") == "true",
		Tags:       c.DefaultQuery("include_tags", "true") == "true",
		Albums:     c.DefaultQuery("include_albums", "true") == "true",
		Species:    c.DefaultQuery("include_species", "true") == "true",
		OCR:        c.DefaultQuery("include_ocr", "false") == "true",
		Faces:      c.DefaultQuery("include_faces", "false") == "true",
	}

	row, err := h.assetService.GetAssetRelations(c.Request.Context(), id)
	if err != nil {
		api.WriteProblem(c, api.NotFound(err))
		return
	}

	api.JSONOK(c, dto.ToAssetDetailDTO(row, includes))
}

// GetAssetExif retrieves the raw EXIF JSON captured during metadata processing.
// @Summary Get raw asset EXIF
// @Description Retrieve the full exiftool JSON object stored for an asset during metadata processing.
// @Tags assets
// @Accept json
// @Produce json
// @Param id path string true "Asset ID (UUID format)" example("550e8400-e29b-41d4-a716-446655440000")
// @Success 200 {object} dto.AssetExifResponseDTO "Raw EXIF JSON"
// @Failure 400 {object} api.ProblemResponse "Invalid asset ID"
// @Failure 404 {object} api.ProblemResponse "Asset or EXIF not found"
// @Router /api/v1/assets/{id}/exif [get]
func (h *AssetHandler) GetAssetExif(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	if _, ok := h.getAuthorizedAssetForRead(c, id, "Authentication required to access this asset", "You don't have permission to access this asset"); !ok {
		return
	}

	exifRaw, err := h.assetService.GetAssetExifRaw(c.Request.Context(), id)
	if err != nil {
		api.WriteProblem(c, api.NotFound(err))
		return
	}
	if len(exifRaw) == 0 {
		api.WriteProblem(c, api.NotFound(errors.New("raw EXIF has not been extracted for this asset")))
		return
	}

	var exifRawObject map[string]any
	if err := json.Unmarshal(exifRaw, &exifRawObject); err != nil {
		api.WriteProblem(c, api.Internal(err))
		return
	}

	api.JSONOK(c, dto.AssetExifResponseDTO{
		AssetID: id.String(),
		ExifRaw: exifRawObject,
	})
}

// UpdateAsset updates asset metadata
// @Summary Update asset metadata
// @Description Update the specific metadata of an asset (e.g., photo EXIF data, video metadata).
// @Tags assets
// @Produce json
// @Param id path string true "Asset ID (UUID format)" example("550e8400-e29b-41d4-a716-446655440000")
// @Param data body dto.UpdateAssetRequestDTO true "Asset metadata"
// @Success 200 {object} dto.MessageResponseDTO "Asset updated successfully"
// @Failure 400 {object} api.ProblemResponse "Invalid asset ID or request body"
// @Failure 500 {object} api.ProblemResponse "Internal server error"
// @Router /api/v1/assets/{id} [put]
func (h *AssetHandler) UpdateAsset(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	if _, ok := h.getAuthorizedAsset(c, id, "Authentication required to update this asset", "You don't have permission to update this asset"); !ok {
		return
	}

	var updateData dto.UpdateAssetRequestDTO
	if err := c.ShouldBindJSON(&updateData); err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	err = h.assetService.UpdateAssetMetadata(c.Request.Context(), id, updateData.Metadata)
	if err != nil {
		log.Printf("Failed to update asset metadata: %v", err)
		api.WriteProblem(c, api.Internal(err))
		return
	}

	api.JSONOK(c, dto.MessageResponseDTO{Message: "Asset updated successfully"})
}

// DeleteAsset deletes an asset
// @Summary Delete asset
// @Description Soft delete an asset by marking it as deleted. The physical file is not removed.
// @Tags assets
// @Accept json
// @Produce json
// @Param id path string true "Asset ID (UUID format)" example("550e8400-e29b-41d4-a716-446655440000")
// @Success 200 {object} dto.MessageResponseDTO "Asset deleted successfully"
// @Failure 400 {object} api.ProblemResponse "Invalid asset ID format"
// @Failure 500 {object} api.ProblemResponse "Internal server error"
// @Router /api/v1/assets/{id} [delete]
func (h *AssetHandler) DeleteAsset(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	if _, ok := h.getAuthorizedAsset(c, id, "Authentication required to delete this asset", "You don't have permission to delete this asset"); !ok {
		return
	}

	err = h.assetService.DeleteAsset(c.Request.Context(), id)
	if err != nil {
		log.Printf("Failed to delete asset: %v", err)
		api.WriteProblem(c, api.Internal(err))
		return
	}

	api.JSONOK(c, dto.MessageResponseDTO{Message: "Asset deleted successfully"})
}

// RestoreAsset restores an asset from Trash
// @Summary Restore asset
// @Description Restore a soft-deleted asset from Trash. The original file is not moved.
// @Tags assets
// @Accept json
// @Produce json
// @Param id path string true "Asset ID (UUID format)" example("550e8400-e29b-41d4-a716-446655440000")
// @Success 200 {object} dto.MessageResponseDTO "Asset restored successfully"
// @Failure 400 {object} api.ProblemResponse "Invalid asset ID format"
// @Failure 500 {object} api.ProblemResponse "Internal server error"
// @Router /api/v1/assets/{id}/restore [post]
func (h *AssetHandler) RestoreAsset(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	if _, ok := h.getAuthorizedAssetAny(c, id, "Authentication required to restore this asset", "You don't have permission to restore this asset"); !ok {
		return
	}

	err = h.assetService.RestoreAsset(c.Request.Context(), id)
	if err != nil {
		log.Printf("Failed to restore asset: %v", err)
		api.WriteProblem(c, api.Internal(err))
		return
	}

	api.JSONOK(c, dto.MessageResponseDTO{Message: "Asset restored successfully"})
}

// AddAssetToAlbum adds an asset to an album
// @Summary Add asset to album
// @Description Associate an asset with a specific album by asset ID and album ID.
// @Tags assets
// @Accept json
// @Produce json
// @Param id path string true "Asset ID (UUID format)" example("550e8400-e29b-41d4-a716-446655440000")
// @Param albumId path int true "Album ID" example(123)
// @Success 200 {object} dto.MessageResponseDTO "Asset added to album successfully"
// @Failure 400 {object} api.ProblemResponse "Invalid asset ID or album ID"
// @Failure 500 {object} api.ProblemResponse "Internal server error"
// @Router /api/v1/assets/{id}/albums/{albumId} [post]
func (h *AssetHandler) AddAssetToAlbum(c *gin.Context) {
	assetID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	albumID, err := strconv.Atoi(c.Param("albumId"))
	if err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	asset, ok := h.getAuthorizedAsset(c, assetID, "Authentication required to modify this asset", "You don't have permission to modify this asset")
	if !ok {
		return
	}

	album, err := h.queries.GetAlbumByID(c.Request.Context(), int32(albumID))
	if err != nil {
		api.WriteProblem(c, api.NotFound(err))
		return
	}
	if !ensureOwnerAccess(c, &album.UserID, "Authentication required to modify this album", "You don't have permission to modify this album") {
		return
	}
	if asset.OwnerID != nil && *asset.OwnerID != album.UserID && !currentUserIsAdmin(c) {
		api.WriteProblem(c, api.Forbidden(errors.New("cross-user album access denied")))
		return
	}

	err = h.assetService.AddAssetToAlbum(c.Request.Context(), assetID, albumID)
	if err != nil {
		log.Printf("Failed to add asset to album: %v", err)
		api.WriteProblem(c, api.Internal(err))
		return
	}
	h.enqueueBioClipForAddedAsset(c.Request.Context(), album, *asset)

	api.JSONOK(c, dto.MessageResponseDTO{Message: "Asset added to album successfully"})
}

func (h *AssetHandler) enqueueBioClipForAddedAsset(ctx context.Context, album repo.Album, asset repo.Asset) {
	if !shouldQueueBioClipForAlbumAsset(album, asset) {
		return
	}
	available, err := bioClipRuntimeAvailable(ctx, h.settingsService, h.runtimeChecker)
	if err != nil {
		log.Printf("Failed to check BioCLIP availability for album %d asset %s: %v", album.AlbumID, asset.AssetID.String(), err)
		return
	}
	if !available {
		return
	}
	if err := requestBioClipAsset(ctx, h.writer, asset); err != nil {
		log.Printf("Failed to queue BioCLIP for album %d asset %s: %v", album.AlbumID, asset.AssetID.String(), err)
	}
}

// stringPtr returns a pointer to a string
func stringPtr(s string) *string {
	return &s
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
