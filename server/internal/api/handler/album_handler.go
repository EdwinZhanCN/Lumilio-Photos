package handler

import (
	"errors"
	"log"
	"strconv"
	"time"

	"server/internal/api"
	"server/internal/db/repo"
	"server/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type AlbumHandler struct {
	albumService *service.AlbumService
	queries      *repo.Queries
}

// NewAlbumHandler creates a new album handler
func NewAlbumHandler(albumService *service.AlbumService, queries *repo.Queries) *AlbumHandler {
	return &AlbumHandler{
		albumService: albumService,
		queries:      queries,
	}
}

// Request/Response types
type CreateAlbumRequest struct {
	AlbumName    string  `json:"album_name" binding:"required"`
	Description  *string `json:"description"`
	CoverAssetID string  `json:"cover_asset_id" binding:"required,uuid4"`
}

type UpdateAlbumRequest struct {
	AlbumName    *string `json:"album_name"`
	Description  *string `json:"description"`
	CoverAssetID *string `json:"cover_asset_id" binding:"omitempty,uuid4"`
}

type AlbumDTO struct {
	AlbumID      int32     `json:"album_id"`
	UserID       int32     `json:"user_id"`
	AlbumName    string    `json:"album_name"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	Description  *string   `json:"description"`
	CoverAssetID *string   `json:"cover_asset_id"`
}

type GetAlbumResponse struct {
	AlbumDTO
	AssetCount int64 `json:"asset_count"`
}

type ListAlbumsResponse struct {
	Albums []GetAlbumResponse `json:"albums"`
	Total  int                `json:"total"`
	Limit  int                `json:"limit"`
	Offset int                `json:"offset"`
}

type AddAssetToAlbumRequest struct {
	Position *int32 `json:"position"`
}

type UpdateAssetPositionRequest struct {
	Position *int32 `json:"position" binding:"required"`
}

func toAlbumDTO(a repo.Album) AlbumDTO {
	var createdAt time.Time
	if a.CreatedAt.Valid {
		createdAt = a.CreatedAt.Time
	}
	var updatedAt time.Time
	if a.UpdatedAt.Valid {
		updatedAt = a.UpdatedAt.Time
	}
	var coverID *string
	if a.CoverAssetID.Valid {
		s := uuid.UUID(a.CoverAssetID.Bytes).String()
		coverID = &s
	}

	return AlbumDTO{
		AlbumID:      a.AlbumID,
		UserID:       a.UserID,
		AlbumName:    a.AlbumName,
		CreatedAt:    createdAt,
		UpdatedAt:    updatedAt,
		Description:  a.Description,
		CoverAssetID: coverID,
	}
}

// NewAlbum creates a new album
// @Summary Create a new album
// @Description Create a new album for the authenticated user
// @Tags albums
// @Accept json
// @Produce json
// @Param request body CreateAlbumRequest true "Album creation data"
// @Success 200 {object} api.Result{data=GetAlbumResponse} "Album created successfully"
// @Failure 400 {object} api.Result "Invalid request data"
// @Failure 401 {object} api.Result "Unauthorized"
// @Failure 500 {object} api.Result "Failed to create album"
// @Router /albums [post]
// @Security BearerAuth
func (h *AlbumHandler) NewAlbum(c *gin.Context) {
	// Get user ID from JWT claims (set by auth middleware)
	userID, exists := c.Get("user_id")
	if !exists {
		api.GinUnauthorized(c, errors.New("user ID not found in token"), "Unauthorized")
		return
	}

	var req CreateAlbumRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.GinBadRequest(c, err, "Invalid request data")
		return
	}

	// Parse and validate cover asset ID
	coverAssetUUID, err := uuid.Parse(req.CoverAssetID)
	if err != nil {
		api.GinBadRequest(c, err, "Invalid cover asset ID")
		return
	}

	// Create album parameters
	params := repo.CreateAlbumParams{
		UserID:       userID.(int32),
		AlbumName:    req.AlbumName,
		Description:  req.Description,
		CoverAssetID: pgtype.UUID{Bytes: coverAssetUUID, Valid: true},
	}

	album, err := (*h.albumService).CreateNewAlbum(c.Request.Context(), params)
	if err != nil {
		log.Printf("Failed to create album: %v", err)
		api.GinInternalError(c, err, "Failed to create album")
		return
	}

	// Get asset count for the new album (should be 0 for new album)
	count, _ := h.queries.GetAlbumAssetCount(c.Request.Context(), album.AlbumID)

	response := GetAlbumResponse{
		AlbumDTO:   toAlbumDTO(album),
		AssetCount: count,
	}

	api.GinSuccess(c, response)
}

// GetAlbum retrieves a specific album by ID
// @Summary Get album by ID
// @Description Retrieve a specific album by its ID
// @Tags albums
// @Accept json
// @Produce json
// @Param id path int true "Album ID"
// @Success 200 {object} api.Result{data=GetAlbumResponse} "Album retrieved successfully"
// @Failure 400 {object} api.Result "Invalid album ID"
// @Failure 404 {object} api.Result "Album not found"
// @Router /albums/{id} [get]
// @Security BearerAuth
func (h *AlbumHandler) GetAlbum(c *gin.Context) {
	albumIDStr := c.Param("id")
	albumID, err := strconv.ParseInt(albumIDStr, 10, 32)
	if err != nil {
		api.GinBadRequest(c, err, "Invalid album ID")
		return
	}

	album, err := h.queries.GetAlbumByID(c.Request.Context(), int32(albumID))
	if err != nil {
		api.GinNotFound(c, err, "Album not found")
		return
	}

	// Get asset count for the album
	count, err := h.queries.GetAlbumAssetCount(c.Request.Context(), album.AlbumID)
	if err != nil {
		log.Printf("Failed to get asset count for album %d: %v", album.AlbumID, err)
		count = 0 // Default to 0 if count fails
	}

	response := GetAlbumResponse{
		AlbumDTO:   toAlbumDTO(album),
		AssetCount: count,
	}

	api.GinSuccess(c, response)
}

// ListAlbums retrieves albums for the authenticated user
// @Summary List albums
// @Description Retrieve a paginated list of albums for the authenticated user
// @Tags albums
// @Accept json
// @Produce json
// @Param limit query int false "Maximum number of results (max 100)" default(20)
// @Param offset query int false "Number of results to skip for pagination" default(0)
// @Success 200 {object} api.Result{data=ListAlbumsResponse} "Albums retrieved successfully"
// @Failure 400 {object} api.Result "Invalid parameters"
// @Failure 401 {object} api.Result "Unauthorized"
// @Failure 500 {object} api.Result "Failed to retrieve albums"
// @Router /albums [get]
// @Security BearerAuth
func (h *AlbumHandler) ListAlbums(c *gin.Context) {
	// Get user ID from JWT claims
	userID, exists := c.Get("user_id")
	if !exists {
		api.GinUnauthorized(c, errors.New("user ID not found in token"), "Unauthorized")
		return
	}

	// Parse pagination parameters
	limitStr := c.DefaultQuery("limit", "20")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, err := strconv.ParseInt(limitStr, 10, 32)
	if err != nil || limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100 // Cap at 100 albums per request
	}

	offset, err := strconv.ParseInt(offsetStr, 10, 32)
	if err != nil || offset < 0 {
		offset = 0
	}

	params := repo.GetAlbumsByUserParams{
		UserID: userID.(int32),
		Limit:  int32(limit),
		Offset: int32(offset),
	}

	albums, err := h.queries.GetAlbumsByUser(c.Request.Context(), params)
	if err != nil {
		log.Printf("Failed to retrieve albums for user %d: %v", userID.(int32), err)
		api.GinInternalError(c, err, "Failed to retrieve albums")
		return
	}

	// Get asset counts for each album
	albumResponses := make([]GetAlbumResponse, len(albums))
	for i, album := range albums {
		count, err := h.queries.GetAlbumAssetCount(c.Request.Context(), album.AlbumID)
		if err != nil {
			log.Printf("Failed to get asset count for album %d: %v", album.AlbumID, err)
			count = 0
		}

		albumResponses[i] = GetAlbumResponse{
			AlbumDTO:   toAlbumDTO(album),
			AssetCount: count,
		}
	}

	response := ListAlbumsResponse{
		Albums: albumResponses,
		Total:  len(albumResponses),
		Limit:  int(limit),
		Offset: int(offset),
	}

	api.GinSuccess(c, response)
}

// UpdateAlbum updates an existing album
// @Summary Update album
// @Description Update an existing album's information
// @Tags albums
// @Accept json
// @Produce json
// @Param id path int true "Album ID"
// @Param request body UpdateAlbumRequest true "Album update data"
// @Success 200 {object} api.Result{data=GetAlbumResponse} "Album updated successfully"
// @Failure 400 {object} api.Result "Invalid album ID or request data"
// @Failure 401 {object} api.Result "Unauthorized"
// @Failure 403 {object} api.Result "Forbidden"
// @Failure 404 {object} api.Result "Album not found"
// @Failure 500 {object} api.Result "Failed to update album"
// @Router /albums/{id} [put]
// @Security BearerAuth
func (h *AlbumHandler) UpdateAlbum(c *gin.Context) {
	albumIDStr := c.Param("id")
	albumID, err := strconv.ParseInt(albumIDStr, 10, 32)
	if err != nil {
		api.GinBadRequest(c, err, "Invalid album ID")
		return
	}

	var req UpdateAlbumRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.GinBadRequest(c, err, "Invalid request data")
		return
	}

	// Get the existing album to verify ownership and get current values
	existingAlbum, err := h.queries.GetAlbumByID(c.Request.Context(), int32(albumID))
	if err != nil {
		api.GinNotFound(c, err, "Album not found")
		return
	}

	// Verify ownership (if user context is available)
	userID, exists := c.Get("user_id")
	if exists && existingAlbum.UserID != userID.(int32) {
		api.GinForbidden(c, errors.New("access denied"), "You don't have permission to update this album")
		return
	}

	// Prepare update parameters with existing values as defaults
	updateParams := repo.UpdateAlbumParams{
		AlbumID:      int32(albumID),
		AlbumName:    existingAlbum.AlbumName,
		Description:  existingAlbum.Description,
		CoverAssetID: existingAlbum.CoverAssetID,
	}

	// Update fields if provided
	if req.AlbumName != nil {
		updateParams.AlbumName = *req.AlbumName
	}
	if req.Description != nil {
		updateParams.Description = req.Description
	}
	if req.CoverAssetID != nil {
		coverAssetUUID, err := uuid.Parse(*req.CoverAssetID)
		if err != nil {
			api.GinBadRequest(c, err, "Invalid cover asset ID")
			return
		}
		updateParams.CoverAssetID = pgtype.UUID{Bytes: coverAssetUUID, Valid: true}
	}

	updatedAlbum, err := h.queries.UpdateAlbum(c.Request.Context(), updateParams)
	if err != nil {
		log.Printf("Failed to update album %d: %v", albumID, err)
		api.GinInternalError(c, err, "Failed to update album")
		return
	}

	// Get asset count for the updated album
	count, err := h.queries.GetAlbumAssetCount(c.Request.Context(), updatedAlbum.AlbumID)
	if err != nil {
		log.Printf("Failed to get asset count for album %d: %v", updatedAlbum.AlbumID, err)
		count = 0
	}

	response := GetAlbumResponse{
		AlbumDTO:   toAlbumDTO(updatedAlbum),
		AssetCount: count,
	}

	api.GinSuccess(c, response)
}

// DeleteAlbum deletes an album
// @Summary Delete album
// @Description Delete an album by its ID
// @Tags albums
// @Accept json
// @Produce json
// @Param id path int true "Album ID"
// @Success 200 {object} api.Result "Album deleted successfully"
// @Failure 400 {object} api.Result "Invalid album ID"
// @Failure 401 {object} api.Result "Unauthorized"
// @Failure 403 {object} api.Result "Forbidden"
// @Failure 404 {object} api.Result "Album not found"
// @Failure 500 {object} api.Result "Failed to delete album"
// @Router /albums/{id} [delete]
// @Security BearerAuth
func (h *AlbumHandler) DeleteAlbum(c *gin.Context) {
	albumIDStr := c.Param("id")
	albumID, err := strconv.ParseInt(albumIDStr, 10, 32)
	if err != nil {
		api.GinBadRequest(c, err, "Invalid album ID")
		return
	}

	// Get the existing album to verify ownership
	existingAlbum, err := h.queries.GetAlbumByID(c.Request.Context(), int32(albumID))
	if err != nil {
		api.GinNotFound(c, err, "Album not found")
		return
	}

	// Verify ownership (if user context is available)
	userID, exists := c.Get("user_id")
	if exists && existingAlbum.UserID != userID.(int32) {
		api.GinForbidden(c, errors.New("access denied"), "You don't have permission to delete this album")
		return
	}

	err = (*h.albumService).DeleteAlbum(c.Request.Context(), int32(albumID))
	if err != nil {
		log.Printf("Failed to delete album %d: %v", albumID, err)
		api.GinInternalError(c, err, "Failed to delete album")
		return
	}

	api.GinSuccess(c, gin.H{"message": "Album deleted successfully"})
}

// GetAlbumAssets retrieves all assets in an album
// @Summary Get assets in album
// @Description Retrieve all assets in a specific album
// @Tags albums
// @Accept json
// @Produce json
// @Param id path int true "Album ID"
// @Success 200 {object} api.Result "Assets retrieved successfully"
// @Failure 400 {object} api.Result "Invalid album ID"
// @Failure 404 {object} api.Result "Album not found"
// @Failure 500 {object} api.Result "Failed to retrieve album assets"
// @Router /albums/{id}/assets [get]
// @Security BearerAuth
func (h *AlbumHandler) GetAlbumAssets(c *gin.Context) {
	albumIDStr := c.Param("id")
	albumID, err := strconv.ParseInt(albumIDStr, 10, 32)
	if err != nil {
		api.GinBadRequest(c, err, "Invalid album ID")
		return
	}

	// Verify album exists
	_, err = h.queries.GetAlbumByID(c.Request.Context(), int32(albumID))
	if err != nil {
		api.GinNotFound(c, err, "Album not found")
		return
	}

	assets, err := h.queries.GetAlbumAssets(c.Request.Context(), int32(albumID))
	if err != nil {
		log.Printf("Failed to retrieve assets for album %d: %v", albumID, err)
		api.GinInternalError(c, err, "Failed to retrieve album assets")
		return
	}

	api.GinSuccess(c, gin.H{
		"album_id": albumID,
		"assets":   assets,
		"count":    len(assets),
	})
}

// AddAssetToAlbum adds an asset to an album
// @Summary Add asset to album
// @Description Add an asset to a specific album
// @Tags albums
// @Accept json
// @Produce json
// @Param id path int true "Album ID"
// @Param assetId path string true "Asset ID (UUID format)"
// @Param request body AddAssetToAlbumRequest false "Asset position in album"
// @Success 200 {object} api.Result "Asset added to album successfully"
// @Failure 400 {object} api.Result "Invalid album ID or asset ID"
// @Failure 404 {object} api.Result "Album not found"
// @Failure 500 {object} api.Result "Failed to add asset to album"
// @Router /albums/{id}/assets/{assetId} [post]
// @Security BearerAuth
func (h *AlbumHandler) AddAssetToAlbum(c *gin.Context) {
	albumIDStr := c.Param("id")
	albumID, err := strconv.ParseInt(albumIDStr, 10, 32)
	if err != nil {
		api.GinBadRequest(c, err, "Invalid album ID")
		return
	}

	assetIDStr := c.Param("assetId")
	assetID, err := uuid.Parse(assetIDStr)
	if err != nil {
		api.GinBadRequest(c, err, "Invalid asset ID")
		return
	}

	var req AddAssetToAlbumRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.GinBadRequest(c, err, "Invalid request data")
		return
	}

	// Verify album exists
	_, err = h.queries.GetAlbumByID(c.Request.Context(), int32(albumID))
	if err != nil {
		api.GinNotFound(c, err, "Album not found")
		return
	}

	params := repo.AddAssetToAlbumParams{
		AssetID:  pgtype.UUID{Bytes: assetID, Valid: true},
		AlbumID:  int32(albumID),
		Position: req.Position,
	}

	err = h.queries.AddAssetToAlbum(c.Request.Context(), params)
	if err != nil {
		log.Printf("Failed to add asset %s to album %d: %v", assetID, albumID, err)
		api.GinInternalError(c, err, "Failed to add asset to album")
		return
	}

	api.GinSuccess(c, gin.H{"message": "Asset added to album successfully"})
}

// RemoveAssetFromAlbum removes an asset from an album
// @Summary Remove asset from album
// @Description Remove an asset from a specific album
// @Tags albums
// @Accept json
// @Produce json
// @Param id path int true "Album ID"
// @Param assetId path string true "Asset ID (UUID format)"
// @Success 200 {object} api.Result "Asset removed from album successfully"
// @Failure 400 {object} api.Result "Invalid album ID or asset ID"
// @Failure 500 {object} api.Result "Failed to remove asset from album"
// @Router /albums/{id}/assets/{assetId} [delete]
// @Security BearerAuth
func (h *AlbumHandler) RemoveAssetFromAlbum(c *gin.Context) {
	albumIDStr := c.Param("id")
	albumID, err := strconv.ParseInt(albumIDStr, 10, 32)
	if err != nil {
		api.GinBadRequest(c, err, "Invalid album ID")
		return
	}

	assetIDStr := c.Param("assetId")
	assetID, err := uuid.Parse(assetIDStr)
	if err != nil {
		api.GinBadRequest(c, err, "Invalid asset ID")
		return
	}

	params := repo.RemoveAssetFromAlbumParams{
		AssetID: pgtype.UUID{Bytes: assetID, Valid: true},
		AlbumID: int32(albumID),
	}

	err = h.queries.RemoveAssetFromAlbum(c.Request.Context(), params)
	if err != nil {
		log.Printf("Failed to remove asset %s from album %d: %v", assetID, albumID, err)
		api.GinInternalError(c, err, "Failed to remove asset from album")
		return
	}

	api.GinSuccess(c, gin.H{"message": "Asset removed from album successfully"})
}

// UpdateAssetPositionInAlbum updates the position of an asset within an album
// @Summary Update asset position in album
// @Description Update the position of an asset within a specific album
// @Tags albums
// @Accept json
// @Produce json
// @Param id path int true "Album ID"
// @Param assetId path string true "Asset ID (UUID format)"
// @Param request body UpdateAssetPositionRequest true "New position for the asset"
// @Success 200 {object} api.Result "Asset position updated successfully"
// @Failure 400 {object} api.Result "Invalid album ID or asset ID"
// @Failure 500 {object} api.Result "Failed to update asset position"
// @Router /albums/{id}/assets/{assetId}/position [put]
// @Security BearerAuth
func (h *AlbumHandler) UpdateAssetPositionInAlbum(c *gin.Context) {
	albumIDStr := c.Param("id")
	albumID, err := strconv.ParseInt(albumIDStr, 10, 32)
	if err != nil {
		api.GinBadRequest(c, err, "Invalid album ID")
		return
	}

	assetIDStr := c.Param("assetId")
	assetID, err := uuid.Parse(assetIDStr)
	if err != nil {
		api.GinBadRequest(c, err, "Invalid asset ID")
		return
	}

	var req UpdateAssetPositionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.GinBadRequest(c, err, "Invalid request data")
		return
	}

	params := repo.UpdateAssetPositionInAlbumParams{
		AlbumID:  int32(albumID),
		AssetID:  pgtype.UUID{Bytes: assetID, Valid: true},
		Position: req.Position,
	}

	err = h.queries.UpdateAssetPositionInAlbum(c.Request.Context(), params)
	if err != nil {
		log.Printf("Failed to update asset position in album: %v", err)
		api.GinInternalError(c, err, "Failed to update asset position")
		return
	}

	api.GinSuccess(c, gin.H{"message": "Asset position updated successfully"})
}

// GetAssetAlbums retrieves all albums that contain a specific asset
// @Summary Get albums containing asset
// @Description Retrieve all albums that contain a specific asset
// @Tags albums
// @Accept json
// @Produce json
// @Param id path string true "Asset ID (UUID format)"
// @Success 200 {object} api.Result "Albums retrieved successfully"
// @Failure 400 {object} api.Result "Invalid asset ID"
// @Failure 500 {object} api.Result "Failed to retrieve asset albums"
// @Router /assets/{id}/albums [get]
// @Security BearerAuth
func (h *AlbumHandler) GetAssetAlbums(c *gin.Context) {
	assetIDStr := c.Param("id")
	assetID, err := uuid.Parse(assetIDStr)
	if err != nil {
		api.GinBadRequest(c, err, "Invalid asset ID")
		return
	}

	albums, err := h.queries.GetAssetAlbums(c.Request.Context(), pgtype.UUID{Bytes: assetID, Valid: true})
	if err != nil {
		log.Printf("Failed to retrieve albums for asset %s: %v", assetID, err)
		api.GinInternalError(c, err, "Failed to retrieve asset albums")
		return
	}

	api.GinSuccess(c, gin.H{
		"asset_id": assetID,
		"albums":   albums,
		"count":    len(albums),
	})
}
