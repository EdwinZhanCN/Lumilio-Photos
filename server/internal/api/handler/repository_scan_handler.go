package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"server/internal/api"
	"server/internal/api/dto"
	"server/internal/cloud"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/storage"
	"server/internal/storage/scanner"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type RepositoryScanService interface {
	EnqueueManualScan(ctx context.Context, repositoryID string, requestedBy string, force bool) (scanner.EnqueueResult, error)
	GetLatestScanRun(ctx context.Context, repositoryID string) (repo.RepositoryScanRun, error)
	ListScanRuns(ctx context.Context, repositoryID string, limit, offset int32) ([]repo.RepositoryScanRun, error)
}

type RepositoryScanHandler struct {
	scanService  RepositoryScanService
	repoManager  storage.RepositoryManager
	storageRoot  string
	cloudService cloud.CloudSyncService
}

func NewRepositoryScanHandler(scanService RepositoryScanService, repoManager storage.RepositoryManager, storageRoot string, cloudService cloud.CloudSyncService) *RepositoryScanHandler {
	return &RepositoryScanHandler{
		scanService:  scanService,
		repoManager:  repoManager,
		storageRoot:  storageRoot,
		cloudService: cloudService,
	}
}

// CreateRepository creates or registers a repository folder below the configured storage root.
// @Summary Create repository
// @Description Create a repository folder under the server storage root. If the target folder already contains a .lumiliorepo file, it is registered instead.
// @Tags repositories
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.CreateRepositoryRequestDTO true "Repository name"
// @Success 200 {object} dto.CreateRepositoryResponseDTO "Repository created successfully"
// @Failure 400 {object} api.ErrorResponse "Invalid request"
// @Failure 401 {object} api.ErrorResponse "Unauthorized"
// @Failure 403 {object} api.ErrorResponse "Forbidden"
// @Failure 500 {object} api.ErrorResponse "Internal server error"
// @Router /api/v1/repositories [post]
func (h *RepositoryScanHandler) CreateRepository(c *gin.Context) {
	if h == nil || h.repoManager == nil {
		api.GinInternalError(c, errors.New("repository manager unavailable"), "Repository manager unavailable")
		return
	}

	var req dto.CreateRepositoryRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		api.GinBadRequest(c, err, "Invalid repository request")
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		api.GinBadRequest(c, errors.New("repository name is required"), "Repository name is required")
		return
	}

	role := repositoryRoleFromRequest(req.Role)
	if role == dbtypes.RepoRolePrimary && strings.TrimSpace(req.CloudCredentialID) != "" {
		api.GinBadRequest(c, errors.New("cloud imports are not supported for primary repository setup"), "Cloud imports are not supported for primary repository setup")
		return
	}

	ownerID := adminIDFromContext(c)
	result, err := h.repoManager.CreateRepository(c.Request.Context(), storage.CreateRepositorySpec{
		Name:              name,
		Role:              role,
		Root:              h.storageRoot,
		Path:              strings.TrimSpace(req.Path),
		OwnerID:           ownerID,
		StorageStrategy:   req.StorageStrategy,
		DuplicateHandling: req.DuplicateHandling,
	})
	if err != nil {
		var conflict *storage.RepositoryConflictError
		switch {
		case errors.Is(err, storage.ErrPrimaryRepositoryExists):
			api.GinError(c, http.StatusConflict, err, http.StatusConflict, "Primary repository already exists")
		case errors.Is(err, storage.ErrPrimaryRepositoryRequired):
			api.GinError(c, http.StatusConflict, err, http.StatusConflict, "Primary repository must be created first")
		case errors.Is(err, storage.ErrRepositoryExistsAtPath):
			api.GinBadRequest(c, err, "Repository already exists")
		case errors.Is(err, storage.ErrPathNotAllowed):
			api.GinBadRequest(c, err, "Repository path is not allowed")
		case errors.As(err, &conflict):
			// Only the user can say whether this is the same library that moved
			// or an independent copy. Hand back both paths so the client can ask.
			api.GinError(c, http.StatusConflict, err, http.StatusConflict, conflict.Error())
		default:
			api.GinBadRequest(c, err, "Failed to create repository")
		}
		return
	}
	dbRepo := result.Repository

	var cloudImportRunID *string
	var cloudImportError *string
	if strings.TrimSpace(req.CloudCredentialID) != "" {
		if h.cloudService == nil {
			errText := "cloud service unavailable"
			cloudImportError = &errText
		} else {
			credentialID, parseErr := uuid.Parse(req.CloudCredentialID)
			if parseErr != nil {
				errText := "invalid cloud_credential_id"
				cloudImportError = &errText
			} else {
				repositoryID := uuid.UUID(dbRepo.RepoID.Bytes)
				runID, bindErr := h.cloudService.BindRepositoryCredentialAndStartImport(c.Request.Context(), cloud.BindRepositoryCredentialInput{
					RepositoryID: repositoryID,
					CredentialID: credentialID,
					OwnerID:      ownerID,
				})
				if bindErr != nil {
					errText := bindErr.Error()
					cloudImportError = &errText
				} else {
					runIDText := runID.String()
					cloudImportRunID = &runIDText
				}
			}
		}
	}

	api.JSONOK(c, dto.CreateRepositoryResponseDTO{
		Repository:       toRepositoryDTO(dbRepo),
		Warnings:         result.Warnings,
		CloudImportRunID: cloudImportRunID,
		CloudImportError: cloudImportError,
	})
}

// RelocateRepository points an existing repository at a new on-disk location.
// @Summary Relocate repository
// @Description Update a repository's recorded location after its folder moved. The .lumiliorepo at the new path must carry the same repository ID. Assets are not moved: their storage paths are repository-relative.
// @Tags repositories
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Repository UUID"
// @Param request body dto.RelocateRepositoryRequestDTO true "New repository path"
// @Success 200 {object} dto.RepositoryDTO "Repository relocated successfully"
// @Failure 400 {object} api.ErrorResponse "Invalid request"
// @Failure 401 {object} api.ErrorResponse "Unauthorized"
// @Failure 403 {object} api.ErrorResponse "Forbidden"
// @Failure 404 {object} api.ErrorResponse "Repository not found"
// @Failure 409 {object} api.ErrorResponse "Another repository already occupies the path"
// @Router /api/v1/repositories/{id}/relocate [post]
func (h *RepositoryScanHandler) RelocateRepository(c *gin.Context) {
	if h == nil || h.repoManager == nil {
		api.GinInternalError(c, errors.New("repository manager unavailable"), "Repository manager unavailable")
		return
	}

	id := strings.TrimSpace(c.Param("id"))
	if _, err := uuid.Parse(id); err != nil {
		api.GinBadRequest(c, err, "Invalid repository ID")
		return
	}

	var req dto.RelocateRepositoryRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		api.GinBadRequest(c, err, "Invalid relocate request")
		return
	}

	updated, err := h.repoManager.RelocateRepository(c.Request.Context(), id, strings.TrimSpace(req.Path))
	if err != nil {
		switch {
		case errors.Is(err, storage.ErrRepositoryExistsAtPath):
			api.GinError(c, http.StatusConflict, err, http.StatusConflict, "Another repository is already registered at that path")
		case errors.Is(err, storage.ErrRepositoryIDMismatch):
			api.GinBadRequest(c, err, "That folder holds a different repository")
		case errors.Is(err, pgx.ErrNoRows):
			api.GinNotFound(c, err, "Repository not found")
		default:
			api.GinBadRequest(c, err, "Failed to relocate repository")
		}
		return
	}

	api.JSONOK(c, toRepositoryDTO(updated))
}

// RegisterRepositoryCopy registers a duplicated repository folder as a new,
// independent repository.
// @Summary Register repository copy
// @Description Register a folder whose .lumiliorepo carries an already-registered repository ID as an independent repository. A fresh repository ID is written into the folder, the way cloning a git repository produces a distinct working copy.
// @Tags repositories
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.RelocateRepositoryRequestDTO true "Repository path"
// @Success 200 {object} dto.RepositoryDTO "Repository registered successfully"
// @Failure 400 {object} api.ErrorResponse "Invalid request"
// @Failure 401 {object} api.ErrorResponse "Unauthorized"
// @Failure 403 {object} api.ErrorResponse "Forbidden"
// @Router /api/v1/repositories/copies [post]
func (h *RepositoryScanHandler) RegisterRepositoryCopy(c *gin.Context) {
	if h == nil || h.repoManager == nil {
		api.GinInternalError(c, errors.New("repository manager unavailable"), "Repository manager unavailable")
		return
	}

	var req dto.RelocateRepositoryRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		api.GinBadRequest(c, err, "Invalid request")
		return
	}

	created, err := h.repoManager.RegisterRepositoryCopy(
		c.Request.Context(),
		strings.TrimSpace(req.Path),
		adminIDFromContext(c),
		dbtypes.RepoRoleRegular,
	)
	if err != nil {
		api.GinBadRequest(c, err, "Failed to register repository copy")
		return
	}

	api.JSONOK(c, toRepositoryDTO(created))
}

// QueueRepositoryScan queues a manual repository scan.
// @Summary Queue repository scan
// @Description Queue a manual scan for a repository free workspace.
// @Tags repositories
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Repository UUID"
// @Param request body dto.RepositoryScanRequestDTO false "Scan request"
// @Success 200 {object} dto.RepositoryScanQueuedDTO "Repository scan queued successfully"
// @Failure 400 {object} api.ErrorResponse "Invalid request"
// @Failure 401 {object} api.ErrorResponse "Unauthorized"
// @Failure 403 {object} api.ErrorResponse "Forbidden"
// @Router /api/v1/repositories/{id}/scan [post]
func (h *RepositoryScanHandler) QueueRepositoryScan(c *gin.Context) {
	if h == nil || h.scanService == nil {
		api.GinInternalError(c, errors.New("repository scan service unavailable"), "Repository scan service unavailable")
		return
	}

	var req dto.RepositoryScanRequestDTO
	if c.Request.Body != nil && c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			api.GinBadRequest(c, err, "Invalid scan request")
			return
		}
	}

	user, ok := requireCurrentUser(c)
	if !ok {
		return
	}
	requestedBy := strings.TrimSpace(user.Username)
	if requestedBy == "" {
		requestedBy = strconv.Itoa(user.UserID)
	}

	result, err := h.scanService.EnqueueManualScan(c.Request.Context(), strings.TrimSpace(c.Param("id")), requestedBy, req.Force)
	if err != nil {
		api.GinBadRequest(c, err, "Failed to queue repository scan")
		return
	}

	api.JSONOK(c, dto.RepositoryScanQueuedDTO{
		JobID:        result.JobID,
		RepositoryID: result.RepositoryID,
		Mode:         result.Mode,
		Status:       result.Status,
	})
}

// GetLatestRepositoryScan returns the latest scan run for a repository.
// @Summary Get latest repository scan
// @Description Return the latest scan run for a repository.
// @Tags repositories
// @Produce json
// @Security BearerAuth
// @Param id path string true "Repository UUID"
// @Success 200 {object} dto.RepositoryScanRunDTO "Latest repository scan retrieved successfully"
// @Failure 404 {object} api.ErrorResponse "No scan run found"
// @Router /api/v1/repositories/{id}/scans/latest [get]
func (h *RepositoryScanHandler) GetLatestRepositoryScan(c *gin.Context) {
	scanRun, err := h.scanService.GetLatestScanRun(c.Request.Context(), strings.TrimSpace(c.Param("id")))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			api.GinNotFound(c, err, "No repository scan run found")
			return
		}
		api.GinBadRequest(c, err, "Failed to load latest repository scan")
		return
	}
	api.JSONOK(c, toRepositoryScanRunDTO(scanRun))
}

// ListRepositoryScans lists recent scan runs for a repository.
// @Summary List repository scans
// @Description List recent scan runs for a repository.
// @Tags repositories
// @Produce json
// @Security BearerAuth
// @Param id path string true "Repository UUID"
// @Param limit query int false "Limit" default(20)
// @Param offset query int false "Offset" default(0)
// @Success 200 {object} dto.RepositoryScanRunListDTO "Repository scan runs retrieved successfully"
// @Router /api/v1/repositories/{id}/scans [get]
func (h *RepositoryScanHandler) ListRepositoryScans(c *gin.Context) {
	limit := parseInt32Query(c, "limit", 20)
	offset := parseInt32Query(c, "offset", 0)
	scans, err := h.scanService.ListScanRuns(c.Request.Context(), strings.TrimSpace(c.Param("id")), limit, offset)
	if err != nil {
		api.GinBadRequest(c, err, "Failed to list repository scans")
		return
	}
	items := make([]dto.RepositoryScanRunDTO, 0, len(scans))
	for _, scanRun := range scans {
		items = append(items, toRepositoryScanRunDTO(scanRun))
	}
	api.JSONOK(c, dto.RepositoryScanRunListDTO{Scans: items})
}

// ListRepositories returns all registered repositories.
// @Summary List repositories
// @Description Return all registered repositories.
// @Tags repositories
// @Produce json
// @Security BearerAuth
// @Success 200 {object} dto.ListRepositoriesResponseDTO "Repositories retrieved successfully"
// @Router /api/v1/repositories [get]
func (h *RepositoryScanHandler) ListRepositories(c *gin.Context) {
	repos, err := h.repoManager.ListRepositories()
	if err != nil {
		api.GinInternalError(c, err, "Failed to list repositories")
		return
	}

	items := make([]dto.RepositoryDTO, 0, len(repos))
	for _, r := range repos {
		items = append(items, toRepositoryDTO(r))
	}
	api.JSONOK(c, dto.ListRepositoriesResponseDTO{Repositories: items})
}

// GetRepository returns a single repository by ID.
// @Summary Get repository
// @Description Return a single repository.
// @Tags repositories
// @Produce json
// @Security BearerAuth
// @Param id path string true "Repository UUID"
// @Success 200 {object} dto.RepositoryDTO "Repository retrieved successfully"
// @Failure 404 {object} api.ErrorResponse "Repository not found"
// @Router /api/v1/repositories/{id} [get]
func (h *RepositoryScanHandler) GetRepository(c *gin.Context) {
	repo, err := h.repoManager.GetRepository(strings.TrimSpace(c.Param("id")))
	if err != nil {
		api.GinNotFound(c, err, "Repository not found")
		return
	}
	api.JSONOK(c, toRepositoryDTO(repo))
}

// UpdateRepository updates mutable fields of a repository.
// @Summary Update repository
// @Description Update mutable repository fields (name, storage_strategy, local_settings, default_owner_id).
// @Tags repositories
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Repository UUID"
// @Param request body dto.UpdateRepositoryRequestDTO true "Fields to update"
// @Success 200 {object} dto.RepositoryDTO "Repository updated successfully"
// @Failure 400 {object} api.ErrorResponse "Invalid request"
// @Failure 404 {object} api.ErrorResponse "Repository not found"
// @Router /api/v1/repositories/{id} [patch]
func (h *RepositoryScanHandler) UpdateRepository(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))

	existing, err := h.repoManager.GetRepository(id)
	if err != nil {
		api.GinNotFound(c, err, "Repository not found")
		return
	}

	var req dto.UpdateRepositoryRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		api.GinBadRequest(c, err, "Invalid request")
		return
	}

	// Merge patch into existing config
	cfg := existing.Config
	if req.Name != nil {
		cfg.Name = *req.Name
	}
	if req.StorageStrategy != nil {
		cfg.StorageStrategy = *req.StorageStrategy
	}
	if req.LocalSettings != nil {
		cfg.LocalSettings.HandleDuplicateFilenames = req.LocalSettings.HandleDuplicateFilenames
	}

	defaultOwnerID := existing.DefaultOwnerID
	if req.DefaultOwnerID != nil {
		defaultOwnerID = req.DefaultOwnerID
	}

	updated, err := h.repoManager.UpdateRepository(id, cfg, defaultOwnerID)
	if err != nil {
		api.GinBadRequest(c, err, "Failed to update repository")
		return
	}

	api.JSONOK(c, toRepositoryDTO(updated))
}

// DeleteRepository removes a repository registration.
// @Summary Delete repository
// @Description Remove a repository from the registry. Does not delete files on disk.
// @Tags repositories
// @Produce json
// @Security BearerAuth
// @Param id path string true "Repository UUID"
// @Success 200 {object} api.SuccessResponse "Repository deleted successfully"
// @Failure 404 {object} api.ErrorResponse "Repository not found"
// @Router /api/v1/repositories/{id} [delete]
func (h *RepositoryScanHandler) DeleteRepository(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))

	existing, err := h.repoManager.GetRepository(id)
	if err != nil {
		api.GinNotFound(c, err, "Repository not found")
		return
	}
	if existing.Role == dbtypes.RepoRolePrimary {
		api.GinError(c, http.StatusConflict, errors.New("primary repository cannot be deleted"), http.StatusConflict, "Primary repository cannot be deleted")
		return
	}

	if err := h.repoManager.RemoveRepository(id); err != nil {
		api.GinInternalError(c, err, "Failed to delete repository")
		return
	}

	api.JSONOK(c, api.SuccessResponse{Message: "Repository deleted successfully"})
}

func repositoryRoleFromRequest(raw string) dbtypes.RepoRole {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case string(dbtypes.RepoRolePrimary):
		return dbtypes.RepoRolePrimary
	default:
		return dbtypes.RepoRoleRegular
	}
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func toRepositoryDTO(repository *repo.Repository) dto.RepositoryDTO {
	if repository == nil {
		return dto.RepositoryDTO{}
	}

	id := ""
	if repository.RepoID.Valid {
		id = uuid.UUID(repository.RepoID.Bytes).String()
	}

	return dto.RepositoryDTO{
		ID:              id,
		Name:            repository.Name,
		Path:            repository.Path,
		Role:            string(repository.Role),
		IsPrimary:       repository.Role == dbtypes.RepoRolePrimary,
		DefaultOwnerID:  repository.DefaultOwnerID,
		StorageStrategy: repository.Config.StorageStrategy,
		LocalSettings: dto.RepositoryLocalSettings{
			HandleDuplicateFilenames: repository.Config.LocalSettings.HandleDuplicateFilenames,
		},
	}
}

func parseInt32Query(c *gin.Context, key string, fallback int32) int32 {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return int32(value)
}

func toRepositoryScanRunDTO(scanRun repo.RepositoryScanRun) dto.RepositoryScanRunDTO {
	startedAt := scanRun.StartedAt.Time
	var finishedAt *time.Time
	if scanRun.FinishedAt.Valid {
		t := scanRun.FinishedAt.Time
		finishedAt = &t
	}
	return dto.RepositoryScanRunDTO{
		ScanID:          scanRun.ScanID.String(),
		RepositoryID:    scanRun.RepositoryID.String(),
		Mode:            scanRun.Mode,
		RequestedBy:     scanRun.RequestedBy,
		Status:          scanRun.Status,
		StartedAt:       startedAt,
		FinishedAt:      finishedAt,
		DiscoveredCount: scanRun.DiscoveredCount,
		UpdatedCount:    scanRun.UpdatedCount,
		DeletedCount:    scanRun.DeletedCount,
		SkippedCount:    scanRun.SkippedCount,
		Error:           scanRun.Error,
	}
}
