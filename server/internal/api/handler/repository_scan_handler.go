package handler

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"

	"server/internal/api"
	"server/internal/api/dto"
	"server/internal/db/repo"
	"server/internal/storage"
	"server/internal/storage/repocfg"
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
	scanService RepositoryScanService
	repoManager storage.RepositoryManager
	storageRoot string
}

func NewRepositoryScanHandler(scanService RepositoryScanService, repoManager storage.RepositoryManager, storageRoot string) *RepositoryScanHandler {
	return &RepositoryScanHandler{
		scanService: scanService,
		repoManager: repoManager,
		storageRoot: storageRoot,
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
// @Success 200 {object} api.Result{data=dto.CreateRepositoryResponseDTO} "Repository created successfully"
// @Failure 400 {object} api.Result "Invalid request"
// @Failure 401 {object} api.Result "Unauthorized"
// @Failure 403 {object} api.Result "Forbidden"
// @Failure 500 {object} api.Result "Internal server error"
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

	repoPath, err := resolveRepositoryCreatePath(h.storageRoot, name)
	if err != nil {
		api.GinBadRequest(c, err, "Invalid repository name")
		return
	}

	if existing, err := h.repoManager.GetRepositoryByPath(repoPath); err == nil && existing != nil {
		api.GinBadRequest(c, fmt.Errorf("repository already exists at %s", repoPath), "Repository already exists")
		return
	}

	var dbRepo *repo.Repository
	if repocfg.IsRepositoryRoot(repoPath) {
		dbRepo, err = h.repoManager.AddRepository(repoPath)
	} else {
		cfg := repocfg.NewDefaultRepositoryConfig(name)
		dbRepo, err = h.repoManager.InitializeRepository(repoPath, *cfg)
	}
	if err != nil {
		api.GinBadRequest(c, err, "Failed to create repository")
		return
	}

	api.GinSuccess(c, dto.CreateRepositoryResponseDTO{
		Repository: toRepositoryDTO(dbRepo),
	})
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
// @Success 200 {object} api.Result{data=dto.RepositoryScanQueuedDTO} "Repository scan queued successfully"
// @Failure 400 {object} api.Result "Invalid request"
// @Failure 401 {object} api.Result "Unauthorized"
// @Failure 403 {object} api.Result "Forbidden"
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

	api.GinSuccess(c, dto.RepositoryScanQueuedDTO{
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
// @Success 200 {object} api.Result{data=dto.RepositoryScanRunDTO} "Latest repository scan retrieved successfully"
// @Failure 404 {object} api.Result "No scan run found"
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
	api.GinSuccess(c, toRepositoryScanRunDTO(scanRun))
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
// @Success 200 {object} api.Result{data=dto.RepositoryScanRunListDTO} "Repository scan runs retrieved successfully"
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
	api.GinSuccess(c, dto.RepositoryScanRunListDTO{Scans: items})
}

func resolveRepositoryCreatePath(storageRoot, name string) (string, error) {
	root := strings.TrimSpace(storageRoot)
	if root == "" {
		root = strings.TrimSpace(os.Getenv("STORAGE_PATH"))
	}
	if root == "" {
		return "", errors.New("storage root is not configured")
	}

	cleanRoot, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return "", fmt.Errorf("invalid storage root: %w", err)
	}

	folderName := repositoryFolderNameFromName(name)
	if folderName == "" {
		return "", errors.New("repository name must contain letters or numbers")
	}

	repoPath, err := filepath.Abs(filepath.Join(cleanRoot, folderName))
	if err != nil {
		return "", fmt.Errorf("invalid repository path: %w", err)
	}

	rel, err := filepath.Rel(cleanRoot, repoPath)
	if err != nil {
		return "", fmt.Errorf("invalid repository path: %w", err)
	}
	if rel == "." || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return "", errors.New("repository path must be inside storage root")
	}

	return repoPath, nil
}

func repositoryFolderNameFromName(name string) string {
	var builder strings.Builder
	lastDash := false
	for _, r := range strings.TrimSpace(name) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			builder.WriteRune(unicode.ToLower(r))
			lastDash = false
		case r == '-' || r == '_':
			if builder.Len() > 0 {
				builder.WriteRune(r)
				lastDash = r == '-'
			}
		case unicode.IsSpace(r):
			if builder.Len() > 0 && !lastDash {
				builder.WriteRune('-')
				lastDash = true
			}
		default:
			if builder.Len() > 0 && !lastDash {
				builder.WriteRune('-')
				lastDash = true
			}
		}
	}

	return strings.Trim(strings.TrimSpace(builder.String()), "-_")
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
		ID:        id,
		Name:      repository.Name,
		Path:      repository.Path,
		IsPrimary: isPrimaryRepository(repository.Name, repository.Path),
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
