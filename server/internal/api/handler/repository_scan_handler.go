package handler

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"server/internal/api"
	"server/internal/api/dto"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/storage"
	"server/internal/storage/scanner"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ListLifecycleAudit returns the durable administrator lifecycle history.
// @Summary List repository lifecycle audit events
// @Tags repositories
// @Produce json
// @Security BearerAuth
// @Param limit query int false "Maximum events (1-200)"
// @Param offset query int false "Pagination offset"
// @Success 200 {object} dto.ListLifecycleAuditEventsResponseDTO
// @Router /api/v1/repositories/lifecycle-audit [get]
func (h *RepositoryScanHandler) ListLifecycleAudit(c *gin.Context) {
	limit, _ := strconv.ParseInt(c.DefaultQuery("limit", "100"), 10, 64)
	offset, _ := strconv.ParseInt(c.DefaultQuery("offset", "0"), 10, 64)
	events, err := h.repoManager.ListLifecycleAudit(c.Request.Context(), storage.LifecycleAuditFilter{
		TargetType: c.Query("target_type"), TargetID: c.Query("target_id"), Limit: limit, Offset: offset,
	})
	if err != nil {
		api.GinBadRequest(c, err, "Lifecycle audit history is unavailable")
		return
	}
	api.JSONOK(c, dto.ListLifecycleAuditEventsResponseDTO{Events: lifecycleAuditDTOs(events, false)})
}

// GetStorageDiagnostics returns administrator-only filesystem and mount facts.
// @Summary Get Storage Location and repository diagnostics
// @Tags repositories
// @Produce json
// @Security BearerAuth
// @Success 200 {object} dto.StorageDiagnosticsResponseDTO
// @Router /api/v1/repositories/storage-diagnostics [get]
func (h *RepositoryScanHandler) GetStorageDiagnostics(c *gin.Context) {
	items, err := h.storageDiagnostics(c.Request.Context(), false)
	if err != nil {
		api.GinInternalError(c, err, "Storage diagnostics are unavailable")
		return
	}
	api.JSONOK(c, dto.StorageDiagnosticsResponseDTO{GeneratedAt: time.Now().UTC(), Items: items})
}

// DownloadStorageSupportBundle exports diagnostics with absolute paths redacted.
// @Summary Download a path-redacted storage support bundle
// @Tags repositories
// @Produce json
// @Security BearerAuth
// @Success 200 {object} dto.StorageSupportBundleDTO
// @Router /api/v1/repositories/storage-support-bundle [get]
func (h *RepositoryScanHandler) DownloadStorageSupportBundle(c *gin.Context) {
	items, err := h.storageDiagnostics(c.Request.Context(), true)
	if err != nil {
		api.GinInternalError(c, err, "Storage support bundle is unavailable")
		return
	}
	events, err := h.repoManager.ListLifecycleAudit(c.Request.Context(), storage.LifecycleAuditFilter{Limit: 200})
	if err != nil {
		api.GinInternalError(c, err, "Storage support bundle audit history is unavailable")
		return
	}
	c.Header("Content-Disposition", `attachment; filename="lumilio-storage-support.json"`)
	api.JSONOK(c, dto.StorageSupportBundleDTO{
		GeneratedAt: time.Now().UTC(), PathsRedacted: true, Diagnostics: items,
		AuditEvents: lifecycleAuditDTOs(events, true),
	})
}

func (h *RepositoryScanHandler) storageDiagnostics(ctx context.Context, redact bool) ([]dto.StorageDiagnosticDTO, error) {
	roots, err := h.repoManager.ListRepositoryRoots(ctx)
	if err != nil {
		return nil, err
	}
	repositories, err := h.repoManager.ListRepositories()
	if err != nil {
		return nil, err
	}
	items := make([]dto.StorageDiagnosticDTO, 0, len(roots)+len(repositories))
	for _, root := range roots {
		info := storage.InspectStoragePath(root.Path)
		item := storageDiagnosticDTO("storage_location", root.RootID.String(), "", root.Name, root.Path, string(root.Status), root.RootID.String(), info, redact)
		item.RegisteredMountFingerprint = root.MountFingerprint
		item.MountFingerprintChanged = root.MountFingerprint != "" && info.MountFingerprint != "" && root.MountFingerprint != info.MountFingerprint
		if item.MountFingerprintChanged {
			item.RiskWarnings = append(item.RiskWarnings, "mount_fingerprint_changed")
		}
		items = append(items, item)
	}
	for _, repository := range repositories {
		info := storage.InspectStoragePath(repository.Path)
		items = append(items, storageDiagnosticDTO("repository", repository.RepoID.String(), repository.RootID.String(), repository.Name, repository.Path, string(repository.Reachability), repository.RepoID.String(), info, redact))
	}
	return items, nil
}

func storageDiagnosticDTO(targetType, targetID, parentTargetID, name, path, reachability, markerUUID string, info storage.StoragePathInfo, redact bool) dto.StorageDiagnosticDTO {
	lockInfo, _ := storage.InspectRepositoryLock(path, targetType)
	var lastCoordination *time.Time
	if !lockInfo.AcquiredAt.IsZero() {
		value := lockInfo.AcquiredAt
		lastCoordination = &value
	}
	canonical := info.CanonicalPath
	if redact {
		path = redactSupportPath(path)
		canonical = redactSupportPath(canonical)
	}
	return dto.StorageDiagnosticDTO{
		TargetType: targetType, TargetID: targetID, ParentTargetID: parentTargetID, Name: name, Path: path, CanonicalPath: canonical,
		Reachability: reachability, Writable: info.Writable, CapacityKnown: info.CapacityKnown,
		TotalBytes: info.TotalBytes, AvailableBytes: info.AvailableBytes, Filesystem: info.Filesystem,
		MountID: info.MountID, MountSource: func() string {
			if redact {
				return redactSupportPath(info.MountSource)
			}
			return info.MountSource
		}(),
		Device: info.Device, Inode: info.Inode, EffectiveUID: info.EffectiveUID, EffectiveGID: info.EffectiveGID,
		CaseBehaviorKnown: info.CaseBehaviorKnown, CaseSensitive: info.CaseSensitive,
		LockHolder: lockInfo.Holder, LastCoordination: lastCoordination,
		MarkerUUID: markerUUID, MountFingerprint: info.MountFingerprint,
		NetworkFilesystem: info.NetworkFilesystem, RemovableLikely: info.RemovableLikely,
		CloudSyncProvider: info.CloudSyncProvider, RiskWarnings: info.RiskWarnings,
	}
}

func lifecycleAuditDTOs(events []storage.LifecycleAuditEvent, redact bool) []dto.LifecycleAuditEventDTO {
	items := make([]dto.LifecycleAuditEventDTO, 0, len(events))
	for _, event := range events {
		oldPath, newPath := event.OldPath, event.NewPath
		if redact {
			oldPath, newPath = redactSupportPath(oldPath), redactSupportPath(newPath)
		}
		items = append(items, dto.LifecycleAuditEventDTO{
			EventID: event.EventID, OccurredAt: event.OccurredAt, Actor: event.Actor, ActorUserID: event.ActorUserID,
			HostInstanceID: event.HostInstanceID, RequestID: event.RequestID, OperationID: event.OperationID,
			Action: event.Action, TargetType: event.TargetType, TargetID: event.TargetID, Source: event.Source,
			ConfirmationType: event.ConfirmationType, OldPath: oldPath, NewPath: newPath,
			Result: event.Result, FailureStage: event.FailureStage, Details: redactSupportDetails(event.Details, redact),
		})
	}
	return items
}

func redactSupportDetails(details []byte, redact bool) json.RawMessage {
	if !redact || len(details) == 0 {
		return details
	}
	var value any
	if err := json.Unmarshal(details, &value); err != nil {
		return json.RawMessage(`{"redacted":true}`)
	}
	value = redactSupportValue(value)
	encoded, err := json.Marshal(value)
	if err != nil {
		return json.RawMessage(`{"redacted":true}`)
	}
	return encoded
}

func redactSupportValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			typed[key] = redactSupportValue(child)
		}
		return typed
	case []any:
		for index, child := range typed {
			typed[index] = redactSupportValue(child)
		}
		return typed
	case string:
		if containsSupportPath(typed) {
			return redactSupportPath(typed)
		}
		return typed
	default:
		return value
	}
}

func containsSupportPath(value string) bool {
	trimmed := strings.TrimSpace(value)
	if strings.HasPrefix(trimmed, "/") || strings.HasPrefix(trimmed, `\\`) {
		return true
	}
	if len(trimmed) >= 3 && trimmed[1] == ':' && (trimmed[2] == '\\' || trimmed[2] == '/') {
		return true
	}
	return supportPathPattern.MatchString(value)
}

// Support data is deliberately conservative: if a free-form string appears to
// contain an absolute path, redact the complete string. This prevents path
// fragments from surviving punctuation, URI, UNC, or Windows-drive forms where
// extracting only the path would be ambiguous and easy to get wrong.
var supportPathPattern = regexp.MustCompile(`(?i)(?:[a-z][a-z0-9+.-]*:/+|\\\\|(?:^|[[:space:]\(\[\{"'=,:;])[a-z]:[\\/]|(?:^|[[:space:]\(\[\{"'=,:;])/[^/])`)

func redactSupportPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	digest := sha256.Sum256([]byte(path))
	return "<redacted:" + hex.EncodeToString(digest[:6]) + ">"
}

type RepositoryScanService interface {
	EnqueueManualScan(ctx context.Context, repositoryID string, requestedBy string, force bool) (scanner.EnqueueResult, error)
	GetLatestScanRun(ctx context.Context, repositoryID string) (repo.RepositoryScanRun, error)
	ListScanRuns(ctx context.Context, repositoryID string, limit, offset int32) ([]repo.RepositoryScanRun, error)
}

type RepositoryScanHandler struct {
	scanService RepositoryScanService
	repoManager storage.RepositoryManager
}

func NewRepositoryScanHandler(scanService RepositoryScanService, repoManager storage.RepositoryManager) *RepositoryScanHandler {
	return &RepositoryScanHandler{
		scanService: scanService,
		repoManager: repoManager,
	}
}

// CreateRepository creates a repository below an authorized Storage Location.
// @Summary Create repository
// @Description Create a repository in an explicit direct-child storage folder below a registered Storage Location. Empty root_id selects the configured default. Existing .lumiliorepo targets are returned as structured recovery facts and are never opened implicitly.
// @Tags repositories
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.CreateRepositoryRequestDTO true "Repository name"
// @Success 200 {object} dto.CreateRepositoryResponseDTO "Repository created successfully"
// @Failure 400 {object} api.ErrorResponse "Invalid request"
// @Failure 401 {object} api.ErrorResponse "Unauthorized"
// @Failure 403 {object} api.ErrorResponse "Forbidden"
// @Failure 409 {object} dto.RepositoryConflictDTO "Repository identity conflict"
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

	name := req.Name
	if err := storage.ValidateRepositoryName(name); err != nil {
		api.GinBadRequest(c, err, "Invalid repository name")
		return
	}

	role := repositoryRoleFromRequest(req.Role)
	directoryName := req.DirectoryName
	if role != dbtypes.RepoRolePrimary {
		if err := storage.ValidateRepositoryDirectoryName(directoryName); err != nil {
			api.GinBadRequest(c, err, "Invalid repository storage folder")
			return
		}
	}
	actorOwnerID := adminIDFromContext(c)
	hostOwnerID, err := h.repoManager.HostOwnerID(c.Request.Context())
	if err != nil {
		api.GinInternalError(c, err, "Failed to resolve host owner")
		return
	}
	// Authenticated first-run setup always has an admin, but retain this
	// bootstrap fallback for a repository created before the primary pins the
	// Host Owner identity.
	if hostOwnerID == nil {
		hostOwnerID = actorOwnerID
	}
	requestID := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
	if requestID == "" {
		requestID = uuid.NewString()
	}
	c.Header("Idempotency-Key", requestID)
	actor := "web:admin"
	if actorOwnerID != nil {
		actor = fmt.Sprintf("web:user:%d", *actorOwnerID)
	}
	result, err := h.repoManager.CreateRepository(c.Request.Context(), storage.CreateRepositorySpec{
		RequestID:        requestID,
		Actor:            actor,
		ActorUserID:      actorOwnerID,
		HostInstanceID:   lifecycleHostInstanceID(),
		Name:             name,
		DirectoryName:    directoryName,
		Role:             role,
		RootID:           strings.TrimSpace(req.RootID),
		OwnerID:          hostOwnerID,
		StorageStrategy:  req.StorageStrategy,
		RiskConfirmation: req.RiskConfirmation,
	})
	if err != nil {
		var conflict *storage.RepositoryConflictError
		var existing *storage.ExistingRepositoryFoundError
		var invalidMarker *storage.RepositoryMarkerInvalidError
		switch {
		case errors.Is(err, storage.ErrPrimaryRepositoryExists):
			writeRepositoryConflict(c, "primary_exists", "Primary repository already exists")
		case errors.Is(err, storage.ErrPrimaryRepositoryRequired):
			writeRepositoryConflict(c, "primary_required", "Primary repository must be created first")
		case errors.Is(err, storage.ErrRepositoryRootOffline):
			writeRepositoryConflict(c, "storage_location_offline", "Storage Location is offline")
		case errors.Is(err, storage.ErrRepositoryRootInvalid):
			writeRepositoryConflict(c, "storage_location_invalid", "Storage Location needs attention")
		case errors.Is(err, storage.ErrRepositoryExistsAtPath):
			api.GinBadRequest(c, err, "Repository already exists")
		case errors.Is(err, storage.ErrInvalidRepositoryName):
			api.GinBadRequest(c, err, "Invalid repository name")
		case errors.Is(err, storage.ErrInvalidRepositoryDirectory):
			api.GinBadRequest(c, err, "Invalid repository storage folder")
		case errors.Is(err, storage.ErrRepositoryDirectoryConflict):
			api.GinError(c, http.StatusConflict, err, http.StatusConflict, "Repository storage folder conflicts with an existing directory")
		case errors.Is(err, storage.ErrRepositoryTargetNotEmpty):
			api.GinError(c, http.StatusConflict, err, http.StatusConflict, "Repository target directory is not empty")
		case errors.Is(err, storage.ErrRepositoryStorageNotWritable):
			api.GinBadRequest(c, err, "Repository storage is not writable")
		case errors.Is(err, storage.ErrRepositoryExistingTargetNotMountPoint):
			api.GinError(c, http.StatusConflict, err, http.StatusConflict, "An existing empty Linux target must be a verified mount point")
		case errors.Is(err, storage.ErrPathNotAllowed):
			api.GinBadRequest(c, err, "Repository path is not allowed")
		case errors.Is(err, storage.ErrRepositoryRiskConfirmationRequired):
			api.GinError(c, http.StatusConflict, err, http.StatusConflict, "Storage placement risks require administrator confirmation")
		case errors.As(err, &conflict):
			// Only the user can say whether this is the same Repository that moved
			// or an independent copy. Hand back both paths so the client can ask.
			c.JSON(http.StatusConflict, dto.RepositoryConflictDTO{
				Code:           http.StatusConflict,
				Message:        "Repository identity is already registered",
				ConflictType:   "repository_identity",
				RepositoryID:   conflict.RepositoryID,
				RegisteredPath: conflict.RegisteredPath,
				RequestedPath:  conflict.RequestedPath,
				Actions:        conflict.Actions,
			})
		case errors.As(err, &existing):
			c.JSON(http.StatusConflict, dto.RepositoryConflictDTO{
				Code:          http.StatusConflict,
				Message:       "An existing repository was found in the selected storage folder",
				ConflictType:  "existing_repository_found",
				RepositoryID:  existing.RepositoryID,
				RequestedPath: existing.RequestedPath,
				Actions:       []string{"open"},
			})
		case errors.As(err, &invalidMarker):
			c.JSON(http.StatusConflict, dto.RepositoryConflictDTO{
				Code:          http.StatusConflict,
				Message:       "The selected storage folder contains an invalid repository marker",
				ConflictType:  "repository_marker_invalid",
				RequestedPath: invalidMarker.RequestedPath,
				Actions:       []string{"diagnose"},
			})
		default:
			api.GinBadRequest(c, err, "Failed to create repository")
		}
		return
	}
	dbRepo := result.Repository

	api.JSONOK(c, dto.CreateRepositoryResponseDTO{
		Repository: toRepositoryDTO(dbRepo),
		Warnings:   result.Warnings,
	})
}

// ListRepositoryCandidates classifies direct children of the configured default Storage Location.
// @Summary List repository candidates
// @Description Returns bounded direct-child facts for standalone and Docker workflows without accepting arbitrary filesystem paths.
// @Tags repositories
// @Produce json
// @Security BearerAuth
// @Success 200 {object} dto.ListRepositoryCandidatesResponseDTO
// @Router /api/v1/repository-candidates [get]
func (h *RepositoryScanHandler) ListRepositoryCandidates(c *gin.Context) {
	candidates, err := h.repoManager.ListDefaultRepositoryCandidates(c.Request.Context())
	if err != nil {
		api.GinBadRequest(c, err, "Repository candidates are unavailable")
		return
	}
	items := make([]dto.RepositoryCandidateDTO, 0, len(candidates))
	for _, candidate := range candidates {
		items = append(items, dto.RepositoryCandidateDTO{
			DirectoryName: candidate.DirectoryName, Classification: candidate.Classification,
			RepositoryID: candidate.RepositoryID, Name: candidate.Name,
			Writable: candidate.Writable, MountPoint: candidate.MountPoint,
			CanCreate: candidate.CanCreate, CanOpen: candidate.CanOpen,
			AllowedResolutions: hostActionResolutions(candidate.Actions),
			CapacityKnown:      candidate.CapacityKnown, TotalBytes: candidate.TotalBytes,
			AvailableBytes: candidate.AvailableBytes, Filesystem: candidate.Filesystem,
			RiskWarnings: candidate.RiskWarnings,
		})
	}
	api.JSONOK(c, dto.ListRepositoryCandidatesResponseDTO{Candidates: items})
}

// ResolveRepositoryCandidate applies an explicit moved-original or independent-copy decision to one bounded candidate.
// @Summary Resolve repository candidate identity
// @Description Resolves a same-identity direct child using user-facing decisions; no arbitrary filesystem path is accepted.
// @Tags repositories
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param Idempotency-Key header string false "Stable request identifier"
// @Param request body dto.ResolveRepositoryCandidateRequestDTO true "Repository candidate decision"
// @Success 200 {object} dto.RepositoryDTO
// @Failure 400 {object} api.ErrorResponse
// @Failure 409 {object} api.ErrorResponse
// @Router /api/v1/repository-candidates/resolve [post]
func (h *RepositoryScanHandler) ResolveRepositoryCandidate(c *gin.Context) {
	var req dto.ResolveRepositoryCandidateRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		api.GinBadRequest(c, err, "Invalid repository candidate decision")
		return
	}
	ownerID, err := h.repoManager.HostOwnerID(c.Request.Context())
	if err != nil {
		api.GinBadRequest(c, err, "Host Owner is unavailable")
		return
	}
	if ownerID == nil {
		api.GinBadRequest(c, errors.New("Host Owner is unavailable"), "Host Owner is unavailable")
		return
	}
	requestID := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
	if requestID == "" {
		requestID = uuid.NewString()
	}
	c.Header("Idempotency-Key", requestID)
	actor := "web:admin"
	if actorID := adminIDFromContext(c); actorID != nil {
		actor = fmt.Sprintf("web:user:%d", *actorID)
	}
	repository, err := h.repoManager.ResolveDefaultRepositoryCandidate(
		c.Request.Context(), req.DirectoryName, req.Resolution, ownerID,
		storage.LifecycleRequest{RequestID: requestID, Actor: actor, ActorUserID: adminIDFromContext(c), HostInstanceID: lifecycleHostInstanceID(), RiskConfirmation: req.RiskConfirmation},
	)
	if err != nil {
		if errors.Is(err, storage.ErrRepositoryOriginalOnline) || errors.Is(err, storage.ErrRepositoryBusy) {
			api.GinError(c, http.StatusConflict, err, http.StatusConflict, "Repository candidate cannot use that decision")
			return
		}
		api.GinBadRequest(c, err, "Repository candidate decision failed")
		return
	}
	api.JSONOK(c, toRepositoryDTO(repository))
}

// OpenRepositoryCandidate opens one validated direct child of the configured default Storage Location.
// @Summary Open repository candidate
// @Description Opens a valid .lumiliorepo by portable directory name. Prior repository-private state is isolated before an authoritative initial scan.
// @Tags repositories
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param Idempotency-Key header string false "Stable request identifier"
// @Param request body dto.OpenRepositoryCandidateRequestDTO true "Repository candidate"
// @Success 200 {object} dto.RepositoryDTO
// @Failure 400 {object} api.ErrorResponse
// @Failure 409 {object} dto.RepositoryConflictDTO
// @Router /api/v1/repository-candidates/open [post]
func (h *RepositoryScanHandler) OpenRepositoryCandidate(c *gin.Context) {
	var req dto.OpenRepositoryCandidateRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		api.GinBadRequest(c, err, "Invalid repository candidate")
		return
	}
	if err := storage.ValidateRepositoryDirectoryName(req.DirectoryName); err != nil {
		api.GinBadRequest(c, err, "Invalid repository storage folder")
		return
	}
	ownerID, err := h.repoManager.HostOwnerID(c.Request.Context())
	if err != nil {
		api.GinBadRequest(c, err, "Host Owner is unavailable")
		return
	}
	if ownerID == nil {
		api.GinBadRequest(c, errors.New("Host Owner is unavailable"), "Host Owner is unavailable")
		return
	}
	requestID := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
	if requestID == "" {
		requestID = uuid.NewString()
	}
	c.Header("Idempotency-Key", requestID)
	actor := "web:admin"
	if actorID := adminIDFromContext(c); actorID != nil {
		actor = fmt.Sprintf("web:user:%d", *actorID)
	}
	repository, err := h.repoManager.OpenDefaultRepositoryCandidate(
		c.Request.Context(), req.DirectoryName, ownerID,
		storage.LifecycleRequest{RequestID: requestID, Actor: actor, ActorUserID: adminIDFromContext(c), HostInstanceID: lifecycleHostInstanceID(), RiskConfirmation: req.RiskConfirmation},
	)
	if err != nil {
		var conflict *storage.RepositoryConflictError
		if errors.As(err, &conflict) {
			c.JSON(http.StatusConflict, dto.RepositoryConflictDTO{
				Code: http.StatusConflict, Message: "Repository identity is already registered",
				ConflictType: "repository_identity", RepositoryID: conflict.RepositoryID,
				RegisteredPath: conflict.RegisteredPath, RequestedPath: conflict.RequestedPath,
				Actions: conflict.Actions,
			})
			return
		}
		api.GinBadRequest(c, err, "Repository candidate could not be opened")
		return
	}
	api.JSONOK(c, toRepositoryDTO(repository))
}

func writeRepositoryConflict(c *gin.Context, conflictType, message string) {
	c.JSON(http.StatusConflict, dto.RepositoryConflictDTO{
		Code: http.StatusConflict, Message: message, ConflictType: conflictType,
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
		if errors.Is(err, sql.ErrNoRows) {
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
	if err := h.repoManager.ReconcileAll(c.Request.Context()); err != nil {
		api.GinInternalError(c, err, "Failed to refresh repository reachability")
		return
	}
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

// ListRepositoryRoots returns the Storage Locations authorized by the host.
// @Summary List Storage Locations
// @Description Return registered repository roots with their current reachability. Filesystem paths are admin-only through this route.
// @Tags repositories
// @Produce json
// @Security BearerAuth
// @Success 200 {object} dto.ListRepositoryRootsResponseDTO "Storage Locations retrieved successfully"
// @Router /api/v1/repository-roots [get]
func (h *RepositoryScanHandler) ListRepositoryRoots(c *gin.Context) {
	roots, err := h.repoManager.ListRepositoryRoots(c.Request.Context())
	if err != nil {
		api.GinInternalError(c, err, "Failed to list Storage Locations")
		return
	}
	items := make([]dto.RepositoryRootDTO, 0, len(roots))
	for _, root := range roots {
		pathInfo := storage.InspectStoragePath(root.Path)
		fingerprintChanged := root.MountFingerprint != "" && pathInfo.MountFingerprint != "" && root.MountFingerprint != pathInfo.MountFingerprint
		risks := append([]string{}, pathInfo.RiskWarnings...)
		if fingerprintChanged {
			risks = append(risks, "mount_fingerprint_changed")
		}
		impact, impactErr := h.repoManager.PreviewRepositoryRootRemoval(c.Request.Context(), root.RootID.String())
		if impactErr != nil {
			api.GinInternalError(c, impactErr, "Failed to inspect Storage Location")
			return
		}
		items = append(items, dto.RepositoryRootDTO{
			ID: root.RootID.String(), Name: root.Name, Path: root.Path,
			Kind: string(root.Kind), Status: string(root.Status), Writable: pathInfo.Writable,
			CapacityKnown: pathInfo.CapacityKnown, TotalBytes: pathInfo.TotalBytes,
			AvailableBytes: pathInfo.AvailableBytes, Filesystem: pathInfo.Filesystem,
			RepositoryCount: impact.RepositoryCount, ActiveOperationCount: impact.ActiveOperationCount,
			CanRemove: impact.CanRemove, RemovalBlockedBy: impact.BlockingReason,
			FilesPreserved: impact.FilesPreserved,
			RiskWarnings:   risks, MountFingerprint: pathInfo.MountFingerprint,
			RegisteredMountFingerprint: root.MountFingerprint, MountFingerprintChanged: fingerprintChanged,
		})
	}
	api.JSONOK(c, dto.ListRepositoryRootsResponseDTO{Roots: items})
}

// DeleteRepositoryRoot removes an eligible external Storage Location registration.
// @Summary Remove Storage Location registration
// @Description Remove an empty, idle external Storage Location from Lumilio. The directory, .lumilioroot marker, and every disk file are preserved.
// @Tags repositories
// @Produce json
// @Security BearerAuth
// @Param id path string true "Storage Location UUID"
// @Success 200 {object} api.SuccessResponse "Storage Location registration removed successfully"
// @Failure 404 {object} api.ErrorResponse "Storage Location not found"
// @Failure 409 {object} api.ErrorResponse "Default, non-empty, or busy Storage Location"
// @Router /api/v1/repository-roots/{id} [delete]
func (h *RepositoryScanHandler) DeleteRepositoryRoot(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if _, err := h.repoManager.GetRepositoryRoot(c.Request.Context(), id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			api.GinNotFound(c, err, "Storage Location not found")
		} else {
			api.GinBadRequest(c, err, "Invalid Storage Location")
		}
		return
	}
	requestID, actor := lifecycleRequestFromWeb(c)
	if err := h.repoManager.DeleteRepositoryRoot(c.Request.Context(), id, storage.LifecycleRequest{RequestID: requestID, Actor: actor, ActorUserID: adminIDFromContext(c), HostInstanceID: lifecycleHostInstanceID()}); err != nil {
		if errors.Is(err, storage.ErrRepositoryRootNotRemovable) ||
			errors.Is(err, storage.ErrRepositoryRootInUse) || errors.Is(err, storage.ErrRepositoryBusy) {
			api.GinError(c, http.StatusConflict, err, http.StatusConflict, "Storage Location cannot be removed")
			return
		}
		api.GinInternalError(c, err, "Failed to remove Storage Location")
		return
	}
	api.JSONOK(c, api.SuccessResponse{Message: "Storage Location registration removed; files were preserved"})
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

// GetRepositoryRemovalImpact previews catalog data that will be removed while
// confirming that repository files remain untouched.
// @Summary Preview repository removal
// @Description Return the catalog, album, queued-work, and private-state impact of removing a non-primary repository registration. Files on disk are always preserved.
// @Tags repositories
// @Produce json
// @Security BearerAuth
// @Param id path string true "Repository UUID"
// @Success 200 {object} dto.RepositoryRemovalImpactDTO "Repository removal impact"
// @Failure 404 {object} api.ErrorResponse "Repository not found"
// @Router /api/v1/repositories/{id}/removal-impact [get]
func (h *RepositoryScanHandler) GetRepositoryRemovalImpact(c *gin.Context) {
	impact, err := h.repoManager.PreviewRepositoryRemoval(c.Request.Context(), strings.TrimSpace(c.Param("id")))
	if err != nil {
		api.GinNotFound(c, err, "Repository not found")
		return
	}
	api.JSONOK(c, dto.RepositoryRemovalImpactDTO{
		RepositoryID: impact.RepositoryID, RepositoryName: impact.RepositoryName,
		AssetCount: impact.AssetCount, CatalogMediaBytes: impact.CatalogMediaBytes,
		AlbumCount: impact.AlbumCount, ActiveTaskCount: impact.ActiveTaskCount,
		CloudImportCount:  impact.CloudImportCount,
		PrivateStateBytes: impact.PrivateStateBytes, PrivateStateFound: impact.PrivateStateFound,
		FilesPreserved: true,
	})
}

// RenameRepository changes only a repository's mutable display name.
// @Summary Rename repository
// @Description Change the display name without changing identity, path, storage strategy, duplicate handling, owner, role, or root.
// @Tags repositories
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Repository UUID"
// @Param request body dto.RenameRepositoryRequestDTO true "New display name"
// @Success 200 {object} dto.RepositoryDTO
// @Failure 400 {object} api.ErrorResponse
// @Failure 404 {object} api.ErrorResponse
// @Router /api/v1/repositories/{id}/rename [post]
func (h *RepositoryScanHandler) RenameRepository(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	var request dto.RenameRepositoryRequestDTO
	if err := c.ShouldBindJSON(&request); err != nil {
		api.GinBadRequest(c, err, "Invalid repository name")
		return
	}
	requestID, actor := lifecycleRequestFromWeb(c)
	updated, err := h.repoManager.RenameRepository(c.Request.Context(), id, request.Name, storage.LifecycleRequest{
		RequestID: requestID, Actor: actor, ActorUserID: adminIDFromContext(c), HostInstanceID: lifecycleHostInstanceID(),
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			api.GinNotFound(c, err, "Repository not found")
			return
		}
		api.GinBadRequest(c, err, "Repository could not be renamed")
		return
	}
	api.JSONOK(c, toRepositoryDTO(updated))
}

// DeleteRepository removes a non-primary repository registration.
// @Summary Remove repository registration
// @Description Remove a non-primary repository and its catalog/index/task state after an exact repository-name confirmation. Original media, marker, and private files remain on disk.
// @Tags repositories
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Repository UUID"
// @Param request body dto.RemoveRepositoryRequestDTO true "Exact repository-name confirmation"
// @Success 200 {object} api.SuccessResponse "Repository registration removed successfully"
// @Failure 400 {object} api.ErrorResponse "Invalid confirmation"
// @Failure 409 {object} api.ErrorResponse "Primary or busy repository"
// @Failure 404 {object} api.ErrorResponse "Repository not found"
// @Router /api/v1/repositories/{id} [delete]
func (h *RepositoryScanHandler) DeleteRepository(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))

	existing, err := h.repoManager.GetRepository(id)
	if err != nil {
		api.GinNotFound(c, err, "Repository not found")
		return
	}
	var req dto.RemoveRepositoryRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		api.GinBadRequest(c, err, "Repository name confirmation is required")
		return
	}
	if req.ConfirmationName != existing.Name {
		api.GinBadRequest(c, errors.New("repository name confirmation does not match"), "Repository name confirmation does not match")
		return
	}
	if existing.Role == dbtypes.RepoRolePrimary {
		api.GinError(c, http.StatusConflict, storage.ErrPrimaryRepositoryNotRemovable, http.StatusConflict, "Primary repository cannot be removed")
		return
	}

	requestID, actor := lifecycleRequestFromWeb(c)
	if err := h.repoManager.RemoveRepository(c.Request.Context(), id, storage.LifecycleRequest{RequestID: requestID, Actor: actor, ActorUserID: adminIDFromContext(c), HostInstanceID: lifecycleHostInstanceID()}); err != nil {
		if errors.Is(err, storage.ErrPrimaryRepositoryNotRemovable) || errors.Is(err, storage.ErrRepositoryBusy) {
			api.GinError(c, http.StatusConflict, err, http.StatusConflict, "Repository cannot be removed while busy")
			return
		}
		api.GinInternalError(c, err, "Failed to remove repository")
		return
	}

	api.JSONOK(c, api.SuccessResponse{Message: "Repository registration removed; files were preserved"})
}

func lifecycleRequestFromWeb(c *gin.Context) (string, string) {
	requestID := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
	if requestID == "" {
		requestID = uuid.NewString()
	}
	c.Header("Idempotency-Key", requestID)
	actor := "web:admin"
	if id := adminIDFromContext(c); id != nil {
		actor = fmt.Sprintf("web:user:%d", *id)
	}
	return requestID, actor
}

func lifecycleHostInstanceID() string {
	host, err := os.Hostname()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(host)
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

	return dto.RepositoryDTO{
		ID:              repository.RepoID.String(),
		Name:            repository.Name,
		Path:            repository.Path,
		Role:            string(repository.Role),
		IsPrimary:       repository.Role == dbtypes.RepoRolePrimary,
		RootID:          repository.RootID.String(),
		Reachability:    string(repository.Reachability),
		Activity:        string(repository.Activity),
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
		MovedCount:      scanRun.MovedCount,
		DeletedCount:    scanRun.DeletedCount,
		SkippedCount:    scanRun.SkippedCount,
		DeferredCount:   scanRun.DeferredCount,
		AmbiguousCount:  scanRun.AmbiguousCount,
		Authoritative:   scanRun.Authoritative,
		PartialReason:   scanRun.PartialReason,
		Error:           scanRun.Error,
	}
}
