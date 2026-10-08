package handler

import (
	"encoding/json"
	"log"
	"server/internal/api"
	"server/internal/api/dto"
	"server/internal/service"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Rating Management Handlers

// UpdateAssetRating updates the rating of an asset
// @Summary Update asset rating
// @Description Update the rating (0-5) of a specific asset
// @Tags assets
// @Produce json
// @Param id path string true "Asset ID"
// @Param rating body dto.UpdateRatingRequestDTO true "Rating data"
// @Success 200 {object} dto.MessageResponseDTO "Rating updated successfully"
// @Failure 400 {object} api.ProblemResponse "Bad request"
// @Failure 404 {object} api.ProblemResponse "Asset not found"
// @Failure 500 {object} api.ProblemResponse "Internal server error"
// @Router /api/v1/assets/{id}/rating [put]
func (h *AssetHandler) UpdateAssetRating(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	var req dto.UpdateRatingRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	if req.Rating < 0 || req.Rating > 5 {
		api.WriteProblem(c, api.BadRequest(nil))
		return
	}

	if _, ok := h.getAuthorizedAsset(c, id, "Authentication required to update this asset", "You don't have permission to update this asset"); !ok {
		return
	}

	err = h.assetService.UpdateAssetRating(c.Request.Context(), id, req.Rating)
	if err != nil {
		log.Printf("Failed to update asset rating: %v", err)
		api.WriteProblem(c, api.Internal(err))
		return
	}

	api.JSONOK(c, dto.MessageResponseDTO{Message: "Rating updated successfully"})
}

// UpdateAssetLike updates the like status of an asset
// @Summary Update asset like status
// @Description Update the like/favorite status of a specific asset
// @Tags assets
// @Produce json
// @Param id path string true "Asset ID"
// @Param like body dto.UpdateLikeRequestDTO true "Like data"
// @Success 200 {object} dto.MessageResponseDTO "Like status updated successfully"
// @Failure 400 {object} api.ProblemResponse "Bad request"
// @Failure 404 {object} api.ProblemResponse "Asset not found"
// @Failure 500 {object} api.ProblemResponse "Internal server error"
// @Router /api/v1/assets/{id}/like [put]
func (h *AssetHandler) UpdateAssetLike(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	var req dto.UpdateLikeRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	if _, ok := h.getAuthorizedAsset(c, id, "Authentication required to update this asset", "You don't have permission to update this asset"); !ok {
		return
	}

	err = h.assetService.UpdateAssetLike(c.Request.Context(), id, req.Liked)
	if err != nil {
		log.Printf("Failed to update asset like status: %v", err)
		api.WriteProblem(c, api.Internal(err))
		return
	}

	api.JSONOK(c, dto.MessageResponseDTO{Message: "Like status updated successfully"})
}

// UpdateAssetRatingAndLike updates both rating and like status of an asset
// @Summary Update asset rating and like status
// @Description Update both the rating (0-5) and like/favorite status of a specific asset
// @Tags assets
// @Produce json
// @Param id path string true "Asset ID"
// @Param data body dto.UpdateRatingAndLikeRequestDTO true "Rating and like data"
// @Success 200 {object} dto.MessageResponseDTO "Rating and like status updated successfully"
// @Failure 400 {object} api.ProblemResponse "Bad request"
// @Failure 404 {object} api.ProblemResponse "Asset not found"
// @Failure 500 {object} api.ProblemResponse "Internal server error"
// @Router /api/v1/assets/{id}/rating-and-like [put]
func (h *AssetHandler) UpdateAssetRatingAndLike(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	var req dto.UpdateRatingAndLikeRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	if req.Rating < 0 || req.Rating > 5 {
		api.WriteProblem(c, api.BadRequest(nil))
		return
	}

	if _, ok := h.getAuthorizedAsset(c, id, "Authentication required to update this asset", "You don't have permission to update this asset"); !ok {
		return
	}

	err = h.assetService.UpdateAssetRatingAndLike(c.Request.Context(), id, req.Rating, req.Liked)
	if err != nil {
		log.Printf("Failed to update asset rating and like status: %v", err)
		api.WriteProblem(c, api.Internal(err))
		return
	}

	api.JSONOK(c, dto.MessageResponseDTO{Message: "Rating and like status updated successfully"})
}

// UpdateAssetDescription updates the description of an asset
// @Summary Update asset description
// @Description Update the description metadata of an asset
// @Tags assets
// @Produce json
// @Param id path string true "Asset ID"
// @Param description body dto.UpdateDescriptionRequestDTO true "Description data"
// @Success 200 {object} dto.MessageResponseDTO "Description updated successfully"
// @Failure 400 {object} api.ProblemResponse "Bad request"
// @Failure 404 {object} api.ProblemResponse "Asset not found"
// @Failure 500 {object} api.ProblemResponse "Internal server error"
// @Router /api/v1/assets/{id}/description [put]
func (h *AssetHandler) UpdateAssetDescription(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	var req dto.UpdateDescriptionRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	if _, ok := h.getAuthorizedAsset(c, id, "Authentication required to update this asset", "You don't have permission to update this asset"); !ok {
		return
	}

	err = h.assetService.UpdateAssetDescription(c.Request.Context(), id, req.Description)
	if err != nil {
		log.Printf("Failed to update asset description: %v", err)
		api.WriteProblem(c, api.Internal(err))
		return
	}

	api.JSONOK(c, dto.MessageResponseDTO{Message: "Description updated successfully"})
}

// GetAssetTags lists the tags attached to an asset
// @Summary Get asset tags
// @Description Get all tags (manual and AI-generated) attached to an asset
// @Tags assets
// @Produce json
// @Param id path string true "Asset ID"
// @Success 200 {object} dto.AssetTagsResponseDTO "Tags retrieved successfully"
// @Failure 400 {object} api.ProblemResponse "Bad request"
// @Failure 500 {object} api.ProblemResponse "Internal server error"
// @Router /api/v1/assets/{id}/tags [get]
func (h *AssetHandler) GetAssetTags(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	if _, ok := h.getAuthorizedAssetForRead(c, id, "Authentication required to view this asset", "You don't have permission to view this asset"); !ok {
		return
	}

	raw, err := h.assetService.GetAssetTags(c.Request.Context(), id)
	if err != nil {
		log.Printf("Failed to get asset tags: %v", err)
		api.WriteProblem(c, api.Internal(err))
		return
	}

	tags := []dto.AssetTagDTO{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &tags); err != nil {
			log.Printf("Failed to decode asset tags: %v", err)
			api.WriteProblem(c, api.Internal(err))
			return
		}
	}

	api.JSONOK(c, dto.AssetTagsResponseDTO{Tags: tags})
}

// AddAssetTag adds a manual tag to an asset
// @Summary Add a manual tag to an asset
// @Description Resolve (creating if needed) a tag by name and link it to the asset with the manual source
// @Tags assets
// @Accept json
// @Produce json
// @Param id path string true "Asset ID"
// @Param request body dto.AddAssetTagRequestDTO true "Tag to add"
// @Success 200 {object} dto.AssetTagDTO "Tag added successfully"
// @Failure 400 {object} api.ProblemResponse "Bad request"
// @Failure 500 {object} api.ProblemResponse "Internal server error"
// @Router /api/v1/assets/{id}/tags [post]
func (h *AssetHandler) AddAssetTag(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	var req dto.AddAssetTagRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	if _, ok := h.getAuthorizedAsset(c, id, "Authentication required to update this asset", "You don't have permission to update this asset"); !ok {
		return
	}

	tag, err := h.assetService.AddManualTagToAsset(c.Request.Context(), id, req.TagName)
	if err != nil {
		log.Printf("Failed to add tag to asset: %v", err)
		api.WriteProblem(c, api.Internal(err))
		return
	}

	source := service.AssetTagSourceUser
	resp := dto.AssetTagDTO{
		TagID:   tag.TagID,
		TagName: tag.TagName,
		Source:  &source,
	}
	api.JSONOK(c, resp)
}

// RemoveAssetTag removes a tag from an asset
// @Summary Remove a tag from an asset
// @Description Unlink a tag from an asset by tag ID
// @Tags assets
// @Produce json
// @Param id path string true "Asset ID"
// @Param tagId path int true "Tag ID"
// @Success 200 {object} dto.MessageResponseDTO "Tag removed successfully"
// @Failure 400 {object} api.ProblemResponse "Bad request"
// @Failure 500 {object} api.ProblemResponse "Internal server error"
// @Router /api/v1/assets/{id}/tags/{tagId} [delete]
func (h *AssetHandler) RemoveAssetTag(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	tagID, err := strconv.Atoi(c.Param("tagId"))
	if err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	if _, ok := h.getAuthorizedAsset(c, id, "Authentication required to update this asset", "You don't have permission to update this asset"); !ok {
		return
	}

	if err := h.assetService.RemoveTagFromAsset(c.Request.Context(), id, tagID); err != nil {
		log.Printf("Failed to remove tag from asset: %v", err)
		api.WriteProblem(c, api.Internal(err))
		return
	}

	api.JSONOK(c, dto.MessageResponseDTO{Message: "Tag removed successfully"})
}

// ListTags returns tag definitions for autocomplete
// @Summary List/search tags
// @Description List all tags or search by name for autocomplete suggestions
// @Tags assets
// @Produce json
// @Param q query string false "Search query (substring match)"
// @Param limit query int false "Max results" default(20)
// @Success 200 {object} dto.TagListResponseDTO "Tags retrieved successfully"
// @Failure 500 {object} api.ProblemResponse "Internal server error"
// @Router /api/v1/assets/tags [get]
func (h *AssetHandler) ListTags(c *gin.Context) {
	query := c.Query("q")
	limit := 20
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	tags, err := h.assetService.SearchTags(c.Request.Context(), query, limit)
	if err != nil {
		log.Printf("Failed to list tags: %v", err)
		api.WriteProblem(c, api.Internal(err))
		return
	}

	items := make([]dto.TagDTO, 0, len(tags))
	for _, tag := range tags {
		item := dto.TagDTO{TagID: tag.TagID, TagName: tag.TagName}
		if tag.Category != nil {
			item.Category = *tag.Category
		}
		items = append(items, item)
	}

	api.JSONOK(c, dto.TagListResponseDTO{Tags: items})
}

// GetTagSummaries returns a browsable, count/cover-enriched tag vocabulary
// @Summary List tag summaries
// @Description List manual and AI/system tags with usage counts and covers, for the Tags collection view
// @Tags assets
// @Produce json
// @Param repository_id query string false "Optional repository UUID filter"
// @Param source query string false "Optional tag source filter (e.g. manual, zeroshot)"
// @Param q query string false "Search query (substring match on tag name)"
// @Param limit query int false "Max results" default(50)
// @Param offset query int false "Result offset" default(0)
// @Success 200 {object} dto.TagSummaryListResponseDTO "Tag summaries retrieved successfully"
// @Failure 400 {object} api.ProblemResponse "Invalid request parameters"
// @Failure 500 {object} api.ProblemResponse "Internal server error"
// @Router /api/v1/assets/tag-summaries [get]
func (h *AssetHandler) GetTagSummaries(c *gin.Context) {
	var repositoryID *string
	if rawRepoID := strings.TrimSpace(c.Query("repository_id")); rawRepoID != "" {
		if _, err := uuid.Parse(rawRepoID); err != nil {
			api.WriteProblem(c, api.BadRequest(err))
			return
		}
		repositoryID = &rawRepoID
	}

	var source *string
	if rawSource := strings.TrimSpace(c.Query("source")); rawSource != "" {
		source = &rawSource
	}

	var query *string
	if rawQuery := strings.TrimSpace(c.Query("q")); rawQuery != "" {
		query = &rawQuery
	}

	limit, err := parseIntQueryWithRange(c, "limit", 50, 1, 500)
	if err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}
	offset, err := parseIntQueryWithRange(c, "offset", 0, 0, 10000000)
	if err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	summaries, err := h.assetService.ListTagSummaries(c.Request.Context(), ownerScopeID(c), repositoryID, source, query, limit, offset)
	if err != nil {
		log.Printf("Failed to list tag summaries: %v", err)
		api.WriteProblem(c, api.Internal(err))
		return
	}

	items := make([]dto.TagSummaryDTO, len(summaries))
	for i, summary := range summaries {
		items[i] = dto.TagSummaryDTO{
			TagID:        summary.TagID,
			TagName:      summary.TagName,
			Source:       summary.Source,
			AssetCount:   summary.AssetCount,
			CoverAssetID: summary.CoverAssetID,
			LastUsedAt:   summary.LastUsedAt,
		}
	}
	api.JSONOK(c, dto.TagSummaryListResponseDTO{Tags: items})
}

// GetFolders lists immediate child folders under a repository-relative parent path
// @Summary List folder summaries
// @Description List immediate child folders of a repository-relative path, with recursive asset counts and covers, for the Folders collection view
// @Tags assets
// @Produce json
// @Param repository_id query string false "Optional repository UUID filter"
// @Param path query string false "Repository-relative parent folder path (empty for root)"
// @Success 200 {object} dto.FolderListResponseDTO "Folder summaries retrieved successfully"
// @Failure 400 {object} api.ProblemResponse "Invalid request parameters"
// @Failure 500 {object} api.ProblemResponse "Internal server error"
// @Router /api/v1/assets/folders [get]
func (h *AssetHandler) GetFolders(c *gin.Context) {
	var repositoryID *string
	if rawRepoID := strings.TrimSpace(c.Query("repository_id")); rawRepoID != "" {
		if _, err := uuid.Parse(rawRepoID); err != nil {
			api.WriteProblem(c, api.BadRequest(err))
			return
		}
		repositoryID = &rawRepoID
	}

	parentPath := normalizeFolderPath(c.Query("path"))

	summaries, err := h.assetService.ListFolderSummaries(c.Request.Context(), ownerScopeID(c), repositoryID, parentPath)
	if err != nil {
		log.Printf("Failed to list folder summaries: %v", err)
		api.WriteProblem(c, api.Internal(err))
		return
	}

	items := make([]dto.FolderSummaryDTO, len(summaries))
	for i, summary := range summaries {
		items[i] = folderSummaryToDTO(summary)
	}
	api.JSONOK(c, dto.FolderListResponseDTO{Folders: items, ParentPath: parentPath})
}

// GetFolderSummary returns aggregate stats for one repository-relative folder
// @Summary Get one folder summary
// @Description Get recursive asset counts, date range, and cover for one repository-relative folder path, for the Folder detail header
// @Tags assets
// @Produce json
// @Param repository_id query string true "Repository UUID"
// @Param path query string false "Repository-relative folder path (empty for root)"
// @Success 200 {object} dto.FolderSummaryDTO "Folder summary retrieved successfully"
// @Failure 400 {object} api.ProblemResponse "Invalid request parameters"
// @Failure 500 {object} api.ProblemResponse "Internal server error"
// @Router /api/v1/assets/folders/summary [get]
func (h *AssetHandler) GetFolderSummary(c *gin.Context) {
	repositoryID := strings.TrimSpace(c.Query("repository_id"))
	if _, err := uuid.Parse(repositoryID); err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	folderPath := normalizeFolderPath(c.Query("path"))

	summary, err := h.assetService.GetFolderSummary(c.Request.Context(), ownerScopeID(c), repositoryID, folderPath)
	if err != nil {
		log.Printf("Failed to get folder summary: %v", err)
		api.WriteProblem(c, api.Internal(err))
		return
	}

	api.JSONOK(c, folderSummaryToDTO(summary))
}

func folderSummaryToDTO(summary service.FolderSummary) dto.FolderSummaryDTO {
	return dto.FolderSummaryDTO{
		RepositoryID:   summary.RepositoryID,
		RepositoryName: summary.RepositoryName,
		FolderPath:     summary.FolderPath,
		DisplayName:    summary.DisplayName,
		Depth:          summary.Depth,
		AssetCount:     summary.AssetCount,
		PhotoCount:     summary.PhotoCount,
		VideoCount:     summary.VideoCount,
		AudioCount:     summary.AudioCount,
		DateStart:      summary.DateStart,
		DateEnd:        summary.DateEnd,
		CoverAssetID:   summary.CoverAssetID,
	}
}
