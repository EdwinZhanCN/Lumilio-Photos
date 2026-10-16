package handler

import (
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"server/internal/api"
	"server/internal/api/dto"
	"server/internal/api/problem"
	"server/internal/db/repo"
	"server/internal/service"
	"server/internal/storage/trash"
)

func (h *AssetHandler) lifecycleQueries() *repo.Queries {
	if h.readerDatabase != nil {
		return repo.New(h.readerDatabase)
	}
	return h.queries
}

func (h *AssetHandler) SetTrashRetentionDays(days int) { h.trashRetentionDays = days }

// authorizeSelection checks the whole selection before any side effect.
func (h *AssetHandler) authorizeSelection(c *gin.Context, values []string) ([]uuid.UUID, bool) {
	ids := make([]uuid.UUID, 0, len(values))
	seen := map[uuid.UUID]bool{}
	for _, value := range values {
		id, err := uuid.Parse(value)
		if err != nil {
			api.WriteProblem(c, api.BadRequest(err))
			return nil, false
		}
		if seen[id] {
			continue
		}
		if _, ok := h.getAuthorizedAssetAny(c, id, "Authentication required", "Access denied"); !ok {
			return nil, false
		}
		ids = append(ids, id)
		seen[id] = true
	}
	return ids, true
}

func (h *AssetHandler) bindSelection(c *gin.Context) ([]uuid.UUID, bool) {
	var request dto.AssetSelectionDTO
	if err := c.ShouldBindJSON(&request); err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return nil, false
	}
	return h.authorizeSelection(c, request.AssetIDs)
}

func (h *AssetHandler) bindPurge(c *gin.Context, empty bool) (trash.Request, uuid.NullUUID, bool) {
	var request dto.AssetPurgeRequestDTO
	if err := c.ShouldBindJSON(&request); err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return trash.Request{}, uuid.NullUUID{}, false
	}
	if (empty && len(request.AssetIDs) != 0) || (!empty && len(request.AssetIDs) == 0 && request.RepositoryID == nil) || (len(request.AssetIDs) > 0 && request.RepositoryID != nil) {
		api.WriteProblem(c, api.BadRequest(errors.New("choose a selection or one Repository")))
		return trash.Request{}, uuid.NullUUID{}, false
	}
	var repositoryID uuid.NullUUID
	if request.RepositoryID != nil {
		user, ok := currentUserFromContext(c)
		if !ok || !service.IsAdminRole(user.Role) {
			api.WriteProblem(c, api.Forbidden(errors.New("Repository-wide lifecycle actions require administrator access")))
			return trash.Request{}, repositoryID, false
		}
		id, err := uuid.Parse(*request.RepositoryID)
		if err != nil {
			api.WriteProblem(c, api.BadRequest(err))
			return trash.Request{}, repositoryID, false
		}
		if _, err := h.lifecycleQueries().GetRepository(c.Request.Context(), id); err != nil {
			api.WriteProblem(c, api.NotFound(err))
			return trash.Request{}, repositoryID, false
		}
		repositoryID = uuid.NullUUID{UUID: id, Valid: true}
	}
	ids, ok := h.authorizeSelection(c, request.AssetIDs)
	operation := trashRequestFrom(c, ids...)
	operation.ConfirmationType = "explicit_lifecycle_action"
	return operation, repositoryID, ok
}

func purgeDTO(result trash.PurgeResult) dto.AssetLifecycleResultDTO {
	return dto.AssetLifecycleResultDTO{Files: result.Files, Assets: result.Assets, Entries: result.Entries, Bytes: result.Bytes, Renamed: []dto.AssetRestoredPathDTO{}}
}

// GetAssetAvailability reports display state without filesystem I/O. Metadata
// remains readable in Missing and Trash; opening originals requires availability.
// @Summary Check asset availability
// @Tags assets
// @Produce json
// @Param id path string true "Asset ID"
// @Success 200 {object} dto.MessageResponseDTO
// @Failure 400 {object} api.ProblemResponse
// @Failure 403 {object} api.ProblemResponse
// @Failure 404 {object} api.ProblemResponse
// @Failure 409 {object} api.ProblemResponse "asset_missing, asset_offline, or asset_trashed"
// @Router /api/v1/assets/{id}/availability [get]
func (h *AssetHandler) GetAssetAvailability(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}
	asset, ok := h.getAuthorizedAssetForRead(c, id, "Authentication required", "Access denied")
	if !ok || !h.ensureAssetAvailable(c, asset.AssetID, asset.LifecycleState) {
		return
	}
	api.JSONOK(c, dto.MessageResponseDTO{Message: "available"})
}

func (h *AssetHandler) ensureAssetAvailable(c *gin.Context, id uuid.UUID, state string) bool {
	entries, err := h.lifecycleQueries().ListRepositoryEntriesForAssets(c.Request.Context(), []uuid.NullUUID{{UUID: id, Valid: true}})
	if err != nil {
		api.WriteProblem(c, api.Internal(err))
		return false
	}
	allOffline := len(entries) > 0
	for _, entry := range entries {
		repository, err := h.lifecycleQueries().GetRepository(c.Request.Context(), entry.RepositoryID)
		if err != nil {
			api.WriteProblem(c, api.Internal(err))
			return false
		}
		if repository.Reachability == "active" {
			allOffline = false
		}
	}
	var descriptor problem.Descriptor
	switch {
	case allOffline:
		descriptor = problem.AssetOffline
	case state == "trashed":
		descriptor = problem.AssetTrashed
	case state == "missing":
		descriptor = problem.AssetMissing
	default:
		return true
	}
	api.WriteProblem(c, problem.New(descriptor, errors.New("asset unavailable")))
	return false
}

// PreviewAssetDelete reports catalog facts for a delete confirmation.
// @Summary Preview asset deletion
// @Tags assets
// @Accept json
// @Produce json
// @Param request body dto.AssetSelectionDTO true "Selected Assets"
// @Success 200 {object} dto.AssetDeleteImpactDTO
// @Failure 400 {object} api.ProblemResponse
// @Failure 403 {object} api.ProblemResponse
// @Failure 404 {object} api.ProblemResponse
// @Failure 500 {object} api.ProblemResponse
// @Router /api/v1/assets/delete-impact [post]
func (h *AssetHandler) PreviewAssetDelete(c *gin.Context) {
	ids, ok := h.bindSelection(c)
	if !ok {
		return
	}
	nullable := make([]uuid.NullUUID, 0, len(ids))
	for _, id := range ids {
		nullable = append(nullable, uuid.NullUUID{UUID: id, Valid: true})
	}
	entries, err := h.lifecycleQueries().ListRepositoryEntriesForAssets(c.Request.Context(), nullable)
	if err != nil {
		api.WriteProblem(c, api.Internal(err))
		return
	}
	result := dto.AssetDeleteImpactDTO{Assets: len(ids), RetentionDays: h.trashRetentionDays, Repositories: []dto.AssetImpactRepositoryDTO{}}
	seen := map[uuid.UUID]bool{}
	for _, entry := range entries {
		if entry.State != "present" {
			continue
		}
		result.Files++
		result.Bytes += entry.Size
		if !seen[entry.RepositoryID] {
			repository, err := h.lifecycleQueries().GetRepository(c.Request.Context(), entry.RepositoryID)
			if err != nil {
				api.WriteProblem(c, api.Internal(err))
				return
			}
			result.Repositories = append(result.Repositories, dto.AssetImpactRepositoryDTO{ID: entry.RepositoryID.String(), Name: repository.Name})
			seen[entry.RepositoryID] = true
		}
	}
	api.JSONOK(c, result)
}

// TrashAssets implements the explicit lifecycle action.
// @Summary TrashAssets
// @Description Move all present files of the selection into their Repository trash. Preflight applies to the whole selection.
// @Tags assets
// @Accept json
// @Produce json
// @Param request body dto.AssetSelectionDTO true "Lifecycle request"
// @Success 200 {object} dto.AssetLifecycleResultDTO
// @Failure 400 {object} api.ProblemResponse
// @Failure 401 {object} api.ProblemResponse
// @Failure 403 {object} api.ProblemResponse
// @Failure 404 {object} api.ProblemResponse
// @Failure 409 {object} api.ProblemResponse
// @Failure 500 {object} api.ProblemResponse
// @Router /api/v1/assets/trash [post]
func (h *AssetHandler) TrashAssets(c *gin.Context) {
	ids, ok := h.bindSelection(c)
	if !ok {
		return
	}
	result, err := h.assetService.DeleteAssets(c.Request.Context(), trashRequestFrom(c, ids...))
	if err != nil {
		writeTrashProblem(c, err)
		return
	}
	api.JSONOK(c, dto.AssetLifecycleResultDTO{Assets: result.Assets, Files: result.Files, Bytes: result.Bytes, Renamed: []dto.AssetRestoredPathDTO{}})
}

// RestoreAssets implements the explicit lifecycle action.
// @Summary RestoreAssets
// @Description Restore selected trashed files without overwriting. Reports each renamed destination.
// @Tags assets
// @Accept json
// @Produce json
// @Param request body dto.AssetSelectionDTO true "Lifecycle request"
// @Success 200 {object} dto.AssetLifecycleResultDTO
// @Failure 400 {object} api.ProblemResponse
// @Failure 401 {object} api.ProblemResponse
// @Failure 403 {object} api.ProblemResponse
// @Failure 404 {object} api.ProblemResponse
// @Failure 409 {object} api.ProblemResponse
// @Failure 500 {object} api.ProblemResponse
// @Router /api/v1/assets/restore [post]
func (h *AssetHandler) RestoreAssets(c *gin.Context) {
	ids, ok := h.bindSelection(c)
	if !ok {
		return
	}
	result, err := h.assetService.RestoreAssets(c.Request.Context(), trashRequestFrom(c, ids...))
	if err != nil {
		writeTrashProblem(c, err)
		return
	}
	api.JSONOK(c, restoreDTO(result))
}

// DeleteAssetsPermanently implements the explicit lifecycle action.
// @Summary DeleteAssetsPermanently
// @Description Permanently unlink selected trashed files and purge their entries. Requires explicit confirmation.
// @Tags assets
// @Accept json
// @Produce json
// @Param request body dto.AssetPurgeRequestDTO true "Lifecycle request"
// @Success 200 {object} dto.AssetLifecycleResultDTO
// @Failure 400 {object} api.ProblemResponse
// @Failure 401 {object} api.ProblemResponse
// @Failure 403 {object} api.ProblemResponse
// @Failure 404 {object} api.ProblemResponse
// @Failure 409 {object} api.ProblemResponse
// @Failure 500 {object} api.ProblemResponse
// @Router /api/v1/assets/delete-permanently [post]
func (h *AssetHandler) DeleteAssetsPermanently(c *gin.Context) {
	request, repositoryID, ok := h.bindPurge(c, false)
	if !ok {
		return
	}
	if repositoryID.Valid {
		api.WriteProblem(c, api.BadRequest(errors.New("permanent delete requires a selection")))
		return
	}
	result, err := h.assetService.DeleteAssetsPermanently(c.Request.Context(), request)
	if err != nil {
		writeTrashProblem(c, err)
		return
	}
	api.JSONOK(c, purgeDTO(result))
}

// RemoveMissingAssets implements the explicit lifecycle action.
// @Summary RemoveMissingAssets
// @Description Remove missing entries of a selection or one Repository. No files are touched; metadata is lost if no entry remains.
// @Tags assets
// @Accept json
// @Produce json
// @Param request body dto.AssetPurgeRequestDTO true "Lifecycle request"
// @Success 200 {object} dto.AssetLifecycleResultDTO
// @Failure 400 {object} api.ProblemResponse
// @Failure 401 {object} api.ProblemResponse
// @Failure 403 {object} api.ProblemResponse
// @Failure 404 {object} api.ProblemResponse
// @Failure 409 {object} api.ProblemResponse
// @Failure 500 {object} api.ProblemResponse
// @Router /api/v1/assets/remove-missing [post]
func (h *AssetHandler) RemoveMissingAssets(c *gin.Context) {
	request, repositoryID, ok := h.bindPurge(c, false)
	if !ok {
		return
	}
	result, err := h.assetService.RemoveMissingAssets(c.Request.Context(), trash.RemoveMissingRequest{Request: request, RepositoryID: repositoryID.UUID})
	if err != nil {
		writeTrashProblem(c, err)
		return
	}
	api.JSONOK(c, purgeDTO(result))
}

// EmptyAssetTrash implements the explicit lifecycle action.
// @Summary EmptyAssetTrash
// @Description Empty the authenticated owner scope or one administrator-selected Repository. Preserves copies outside the scope.
// @Tags assets
// @Accept json
// @Produce json
// @Param request body dto.AssetPurgeRequestDTO true "Lifecycle request"
// @Success 200 {object} dto.AssetLifecycleResultDTO
// @Failure 400 {object} api.ProblemResponse
// @Failure 401 {object} api.ProblemResponse
// @Failure 403 {object} api.ProblemResponse
// @Failure 404 {object} api.ProblemResponse
// @Failure 409 {object} api.ProblemResponse
// @Failure 500 {object} api.ProblemResponse
// @Router /api/v1/assets/empty-trash [post]
func (h *AssetHandler) EmptyAssetTrash(c *gin.Context) {
	request, repositoryID, ok := h.bindPurge(c, true)
	if !ok {
		return
	}
	result, err := h.assetService.EmptyAssetTrash(c.Request.Context(), request, repositoryID, ownerScopeID(c))
	if err != nil {
		writeTrashProblem(c, err)
		return
	}
	api.JSONOK(c, purgeDTO(result))
}

func restoreDTO(result trash.RestoreResult) dto.AssetLifecycleResultDTO {
	renamed := make([]dto.AssetRestoredPathDTO, 0, len(result.Renamed))
	for _, file := range result.Renamed {
		renamed = append(renamed, dto.AssetRestoredPathDTO{AssetID: file.AssetID.String(), OriginalPath: file.OriginalPath, RestoredPath: file.RestoredPath})
	}
	return dto.AssetLifecycleResultDTO{Files: result.Files, Renamed: renamed}
}
