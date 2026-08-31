package handler

import (
	"archive/zip"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"server/internal/api"
	"server/internal/api/dto"
	"server/internal/api/problem"
	"server/internal/db/catalogtx"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/pipeline"
	"server/internal/service"
	"server/internal/storage"
	roelocations "server/internal/storage/roe/locations"
	filevalidator "server/internal/utils/file"
	"server/internal/utils/hash"
	"server/internal/utils/imagesource"
	"server/internal/utils/imaging"
	"server/internal/utils/memory"
	"server/internal/utils/upload"
	"strconv"
	"strings"
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

var (
	errInvalidRepositoryID = errors.New("invalid repository ID")
	errRepositoryNotFound  = errors.New("repository not found")
	errNoRepository        = errors.New("no repository available")
)

// resolveUploadRepository resolves an explicit repository UUID, falling back to
// the primary repository when repositoryID is empty.
func (h *AssetHandler) resolveUploadRepository(ctx context.Context, repositoryID string) (repo.Repository, error) {
	if strings.TrimSpace(repositoryID) == "" {
		repository, err := h.queries.GetPrimaryRepository(ctx)
		if err != nil {
			return repo.Repository{}, errNoRepository
		}
		if err := rejectOfflineRepository(repository); err != nil {
			return repo.Repository{}, err
		}
		return repository, nil
	}

	repoUUID, err := uuid.Parse(repositoryID)
	if err != nil {
		return repo.Repository{}, errInvalidRepositoryID
	}
	repository, err := h.queries.GetRepository(ctx, repoUUID)
	if err != nil {
		return repo.Repository{}, errRepositoryNotFound
	}
	if err := rejectOfflineRepository(repository); err != nil {
		return repo.Repository{}, err
	}
	return repository, nil
}

func (h *AssetHandler) resolveUploadOwnerID(ctx context.Context, raw string) (int32, error) {
	if parsed, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 32); err == nil && parsed > 0 {
		return int32(parsed), nil
	}
	user, err := h.queries.GetUserByUsername(ctx, strings.TrimSpace(raw))
	if err != nil {
		return 0, fmt.Errorf("resolve upload owner: %w", err)
	}
	return user.UserID, nil
}

// enqueueStagingCommit makes the recoverable staging journal and its ID-only
// River delivery visible atomically. A failed transaction leaves ownership
// with the request handler, which may safely quarantine the unclaimed file.
func (h *AssetHandler) enqueueStagingCommit(
	ctx context.Context,
	repository repo.Repository,
	ownerID int32,
	stagingFile *storage.StagingFile,
	originalFilename string,
	mimeType string,
	hashes *hash.LayeredHashResult,
) (uuid.UUID, error) {
	if hashes == nil || stagingFile == nil || ownerID <= 0 {
		return uuid.Nil, errors.New("staging commit identity is incomplete")
	}
	tx, err := h.writer.BeginTx(ctx, catalogtx.OperationAssetStagingCommit, nil)
	if err != nil {
		return uuid.Nil, err
	}
	defer tx.Rollback()
	commitID := uuid.New()
	now := dbtypes.NewTimestamp(time.Now().UTC())
	queries := h.queries.WithTx(tx.Raw())
	if _, err := queries.CreateRepositoryStagingCommit(ctx, repo.CreateRepositoryStagingCommitParams{
		CommitID: commitID, RepositoryID: repository.RepoID, OwnerID: ownerID,
		SourceKind: "upload", StagingPath: stagingFile.PrivatePath,
		OriginalFilename: originalFilename, MimeType: mimeType,
		FullHash: strings.ToLower(hashes.ContentHash), FileSize: hashes.FileSize,
		QuickFingerprint:        hashes.QuickFingerprint,
		QuickFingerprintVersion: hashes.QuickFingerprintVersion, CreatedAt: now,
	}); err != nil {
		return uuid.Nil, err
	}
	receiptID := uuid.New()
	if err := pipeline.RequestIngestTx(ctx, tx.Raw(), commitID, receiptID, uuid.New()); err != nil {
		return uuid.Nil, err
	}
	if err := tx.Commit(); err != nil {
		return uuid.Nil, err
	}
	return receiptID, nil
}

// rejectOfflineRepository refuses ingest into a repository whose location is not
// currently reachable. Staging a file for a repository that cannot be written is
// a guaranteed failure later, with a worse error attached.
func rejectOfflineRepository(repository repo.Repository) error {
	if repository.Reachability != dbtypes.RepositoryReachabilityActive {
		return fmt.Errorf("%w: %s", storage.ErrRepositoryOffline, repository.Name)
	}
	if repository.Activity == dbtypes.RepositoryActivityPaused {
		return fmt.Errorf("%w: %s is paused", storage.ErrRepositoryBusy, repository.Name)
	}
	return nil
}

// respondRepositoryError maps a resolveUploadRepository failure onto its HTTP response.
func (h *AssetHandler) respondRepositoryError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, errInvalidRepositoryID):
		api.WriteProblem(c, api.BadRequest(err))
	case errors.Is(err, errRepositoryNotFound):
		api.WriteProblem(c, api.NotFound(err))
	case errors.Is(err, storage.ErrRepositoryOffline):
		api.WriteProblem(c, api.StatusProblem(http.StatusConflict, err))
	case errors.Is(err, storage.ErrRepositoryBusy):
		api.WriteProblem(c, api.StatusProblem(http.StatusConflict, err))
	default:
		api.WriteProblem(c, api.BadRequest(err))
	}
}

// UploadAsset handles asset upload requests
// @Summary Upload a single asset
// @Description Upload a single photo, video, audio file, or document to the system. The file is staged in a repository and queued for processing.
// @Tags assets
// @Accept multipart/form-data
// @Produce json
// @Param file formData file true "Asset file to upload"
// @Param repository_id formData string false "Repository UUID (uses default repository if not provided)" example("550e8400-e29b-41d4-a716-446655440000")
// @Success 200 {object} dto.UploadResponseDTO "Upload successful"
// @Failure 400 {object} api.ProblemResponse "Bad request - no file provided or parse error"
// @Failure 500 {object} api.ProblemResponse "Internal server error"
// @Router /api/v1/assets [post]
func (h *AssetHandler) UploadAsset(c *gin.Context) {
	h.uploadLimiter <- struct{}{}
	defer func() { <-h.uploadLimiter }()

	ctx := c.Request.Context()

	var req dto.UploadAssetRequestDTO
	if err := c.ShouldBind(&req); err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	err := c.Request.ParseMultipartForm(32 << 20)
	if err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		api.WriteProblem(c, api.BadRequest(errors.New("no file provided")))
		return
	}
	defer file.Close()

	validationResult := filevalidator.ValidateFile(header.Filename, header.Header.Get("Content-Type"))
	if !validationResult.Valid {
		api.WriteProblem(c, api.BadRequest(fmt.Errorf("unsupported file type: %s", validationResult.ErrorReason)))
		return
	}
	log.Printf("Validated file %s as %s with canonical MIME %s (RAW: %v)",
		header.Filename, validationResult.AssetType, validationResult.MimeType, validationResult.IsRAW)

	repository, err := h.resolveUploadRepository(ctx, req.RepositoryID)
	if err != nil {
		h.respondRepositoryError(c, err)
		return
	}
	if _, err := h.repoManager.CheckRepositoryWriteCapacity(ctx, repository.RepoID.String(), uint64(max(header.Size, 0))); err != nil {
		h.respondCapacityError(c, err)
		return
	}

	// Create staging file in repository
	stagingFile, stagingWriter, err := h.stagingManager.CreateStagingFile(repository, header.Filename)
	if err != nil {
		log.Printf("Failed to create staging file: %v", err)
		api.WriteProblem(c, api.Internal(err))
		return
	}

	_, err = io.Copy(stagingWriter, file)
	if err != nil {
		_ = stagingWriter.Close()
		log.Printf("Failed to copy file to staging: %v", err)
		h.handleUploadFailureFile(repository, stagingFile, "copy upload data to staging")
		api.WriteProblem(c, api.Internal(err))
		return
	}

	if err := stagingWriter.Sync(); err != nil {
		_ = stagingWriter.Close()
		h.handleUploadFailureFile(repository, stagingFile, "sync upload staging file")
		api.WriteProblem(c, api.Internal(err))
		return
	}
	stagingInfo, err := stagingWriter.Stat()
	if err != nil {
		_ = stagingWriter.Close()
		h.handleUploadFailureFile(repository, stagingFile, "stat upload staging file")
		api.WriteProblem(c, api.Internal(err))
		return
	}
	hashResult, err := hash.CalculateLayeredBLAKE3Reader(stagingWriter, stagingInfo.Size())
	closeErr := stagingWriter.Close()
	err = errors.Join(err, closeErr)
	if err != nil {
		log.Printf("Failed to calculate authoritative hash: %v", err)
		h.handleUploadFailureFile(repository, stagingFile, "calculate upload hash")
		api.WriteProblem(c, api.Internal(err))
		return
	}
	ownerID, err := currentUserIDFromContext(c)
	if err != nil || ownerID == nil {
		h.handleUploadFailureFile(repository, stagingFile, "resolve upload owner")
		api.WriteProblem(c, api.Unauthorized(err))
		return
	}
	receiptID, err := h.enqueueStagingCommit(ctx, repository, *ownerID, stagingFile,
		header.Filename, validationResult.MimeType, hashResult)
	if err != nil {
		log.Printf("Failed to enqueue task: %v", err)
		h.handleUploadFailureFile(repository, stagingFile, "enqueue ingest task")
		api.WriteProblem(c, api.Internal(err))
		return
	}
	log.Printf("Ingest receipt %s accepted for file %s in repository %s", receiptID, header.Filename, repository.Name)

	response := dto.UploadResponseDTO{
		ReceiptID:   receiptID.String(),
		Status:      "processing",
		FileName:    header.Filename,
		Size:        header.Size,
		ContentHash: hashResult.ContentHash,
		Message:     fmt.Sprintf("File received and queued for processing in repository '%s'", repository.Name),
	}

	api.JSONOK(c, response)
}

// BatchUploadAssets handles multiple asset uploads with unified chunk support
// @Summary Batch upload assets with chunk support
// @Description Unified batch upload endpoint that supports both small files and chunked large files. Field names should follow format: single_{session_id} for single files or chunk_{session_id}_{index}_{total} for chunks.
// @Tags assets
// @Accept multipart/form-data
// @Produce json
// @Param repository_id formData string false "Repository UUID (uses default repository if not provided)" example("550e8400-e29b-41d4-a716-446655440000")
// @Param file formData file false "Single file upload - use format: single_{session_id}" example("single_123e4567-e89b-12d3-a456-426614174000")
// @Param file formData file false "Chunked file upload - use format: chunk_{session_id}_{index}_{total}" example("chunk_123e4567-e89b-12d3-a456-426614174000_1_10")
// @Success 200 {object} dto.BatchUploadResponseDTO "Batch upload completed"
// @Failure 400 {object} api.ProblemResponse "Bad request - no files provided or parse error"
// @Failure 500 {object} api.ProblemResponse "Internal server error"
// @Router /api/v1/assets/batch [post]
func (h *AssetHandler) BatchUploadAssets(c *gin.Context) {
	h.uploadLimiter <- struct{}{}
	defer func() { <-h.uploadLimiter }()

	ctx := c.Request.Context()

	repositoryID := strings.TrimSpace(c.Query("repository_id"))
	var repository repo.Repository
	repositoryResolved := false
	resolveRepository := func() bool {
		if repositoryResolved {
			return true
		}
		resolved, err := h.resolveUploadRepository(ctx, repositoryID)
		if err != nil {
			h.respondRepositoryError(c, err)
			return false
		}
		repository = resolved
		repositoryResolved = true
		return true
	}

	mr, err := c.Request.MultipartReader()
	if err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	clientFingerprint := c.GetHeader("X-Upload-Fingerprint")

	// Get user ID from JWT claims
	var userID string
	if id, exists := c.Get("user_id"); exists {
		userID = fmt.Sprintf("%d", id)
	} else {
		// Fallback to anonymous user if not authenticated
		userID = "anonymous"
	}

	type sessionState struct {
		info        *upload.FileFieldInfo
		filename    string
		contentType string
	}

	sessions := make(map[string]*sessionState)
	buf := make([]byte, 1<<20) // 1MiB shared buffer for streaming copy

	for {
		part, perr := mr.NextPart()
		if perr == io.EOF {
			break
		}
		if perr != nil {
			api.WriteProblem(c, api.BadRequest(perr))
			return
		}
		if part.FileName() == "" {
			if part.FormName() == "repository_id" {
				data, _ := io.ReadAll(part)
				repositoryID = strings.TrimSpace(string(data))
				repositoryResolved = false
			}
			part.Close()
			continue
		}

		fieldName := part.FormName()
		fileInfo, err := upload.ParseFileField(fieldName)
		if err != nil {
			part.Close()
			api.WriteProblem(c, api.BadRequest(err))
			return
		}

		filename := part.FileName()
		contentType := part.Header.Get("Content-Type")

		state := sessions[fileInfo.SessionID]
		if state == nil {
			state = &sessionState{
				info:        fileInfo,
				filename:    filename,
				contentType: contentType,
			}
			sessions[fileInfo.SessionID] = state
		}

		if !repositoryResolved {
			if !resolveRepository() {
				return
			}
		}

		if _, exists := h.sessionManager.GetSession(fileInfo.SessionID); !exists {
			if fileInfo.Type == "chunk" {
				part.Close()
				api.WriteProblem(c, api.BadRequest(errors.New("upload session must be created first")))
				return
			}
			h.sessionManager.CreateSession(fileInfo.SessionID, filename, 0, fileInfo.TotalChunks, contentType, repository.RepoID.String(), userID)
		}
		session, _ := h.sessionManager.GetSession(fileInfo.SessionID)
		if session.UserID != userID || session.RepositoryID != repository.RepoID.String() || session.TotalChunks != fileInfo.TotalChunks || session.Filename != path.Base(strings.ReplaceAll(filename, `\`, "/")) {
			part.Close()
			api.WriteProblem(c, api.BadRequest(errors.New("upload session metadata mismatch")))
			return
		}
		alreadyReceived := false
		for _, index := range session.ReceivedChunks {
			if index == fileInfo.ChunkIndex {
				alreadyReceived = true
				break
			}
		}
		if alreadyReceived {
			_, _ = io.Copy(io.Discard, part)
			part.Close()
			continue
		}

		// Update session hash if provided
		if clientFingerprint != "" {
			h.sessionManager.SetSessionFingerprint(fileInfo.SessionID, clientFingerprint)
		}

		h.sessionManager.UpdateSessionStatus(fileInfo.SessionID, "uploading")

		targetName := filename
		if fileInfo.Type == "chunk" {
			targetName = fmt.Sprintf("chunk_%s_%d", fileInfo.SessionID, fileInfo.ChunkIndex)
		}

		stagingFile, dst, err := h.stagingManager.CreateStagingFile(repository, targetName)
		if err != nil {
			part.Close()
			api.WriteProblem(c, api.Internal(err))
			return
		}

		written, err := io.CopyBuffer(dst, part, buf)
		closeErr := dst.Close()
		err = errors.Join(err, closeErr)
		part.Close()
		if err != nil {
			h.handleUploadFailureFile(repository, stagingFile, "save batch upload data")
			api.WriteProblem(c, api.Internal(err))
			return
		}

		if !h.sessionManager.UpdateSessionChunk(fileInfo.SessionID, fileInfo.ChunkIndex, written, stagingFile.PrivatePath) {
			api.WriteProblem(c, api.Internal(errors.New("failed to persist upload session")))
			return
		}

	}

	if len(sessions) == 0 {
		api.WriteProblem(c, api.BadRequest(errors.New("no files provided")))
		return
	}

	var results []dto.BatchUploadResultDTO

	for sessionID, state := range sessions {
		session, ok := h.sessionManager.GetSession(sessionID)
		if !ok {
			continue
		}
		allChunks := make([]upload.ChunkInfo, 0, len(session.ReceivedChunks))
		for _, index := range session.ReceivedChunks {
			allChunks = append(allChunks, upload.ChunkInfo{
				SessionID: sessionID, ChunkIndex: index, PrivatePath: session.ChunkFiles[index], Size: session.ChunkSizes[index],
			})
		}
		if state.info.Type == "single" {
			if len(allChunks) != 1 {
				continue
			}
			header := &multipart.FileHeader{
				Filename: state.filename,
				Size:     allChunks[0].Size,
				Header:   map[string][]string{},
			}
			header.Header.Set("Content-Type", state.contentType)

			chunk := allChunks[0]
			result, err := h.processCompletedUpload(ctx, header, session, repository, &storage.StagingFile{
				ID: sessionID, RepositoryID: repository.RepoID, PrivatePath: chunk.PrivatePath, Filename: state.filename,
			})
			if err != nil {
				results = append(results, dto.BatchUploadResultDTO{
					Success:   false,
					SessionID: sessionID,
					FileName:  state.filename,
					Problem:   newUploadProblem(false),
				})
				continue
			}

			h.sessionManager.UpdateSessionStatus(sessionID, "completed")
			result.SessionID = sessionID
			results = append(results, *result)
			continue
		}

		h.chunkMerger.AddChunks(sessionID, allChunks)

		if !h.sessionManager.IsSessionComplete(sessionID) {
			progress, _ := h.sessionManager.GetSessionProgress(sessionID)
			status := "uploading"
			message := fmt.Sprintf("Upload in progress: %.1f%% complete", progress*100)
			results = append(results, dto.BatchUploadResultDTO{
				Success:   true,
				SessionID: sessionID,
				FileName:  state.filename,
				Status:    &status,
				Message:   &message,
			})
			continue
		}

		h.sessionManager.UpdateSessionStatus(sessionID, "merging")
		mergeResult, err := h.chunkMerger.MergeChunks(repository, sessionID, state.info.TotalChunks, state.filename)
		if err != nil {
			h.sessionManager.SetSessionError(sessionID, err.Error())
			h.chunkMerger.CleanupChunks(repository, sessionID)
			results = append(results, dto.BatchUploadResultDTO{
				Success:   false,
				SessionID: sessionID,
				FileName:  state.filename,
				Problem:   newUploadProblem(false),
			})
			continue
		}

		header := &multipart.FileHeader{
			Filename: state.filename,
			Size:     mergeResult.TotalSize,
			Header:   map[string][]string{},
		}
		header.Header.Set("Content-Type", state.contentType)

		result, err := h.processCompletedUpload(ctx, header, session, repository, mergeResult.StagingFile)

		h.chunkMerger.CleanupChunks(repository, sessionID)

		if err != nil {
			_ = h.stagingManager.RemoveStagingFile(repository, mergeResult.StagingFile)
			h.sessionManager.SetSessionError(sessionID, err.Error())
			results = append(results, dto.BatchUploadResultDTO{
				Success:   false,
				SessionID: sessionID,
				FileName:  state.filename,
				Problem:   newUploadProblem(false),
			})
			continue
		}

		h.sessionManager.UpdateSessionStatus(sessionID, "completed")
		if result.ReceiptID != nil {
			h.sessionManager.SetSessionReceiptID(sessionID, *result.ReceiptID)
		}
		result.SessionID = sessionID
		results = append(results, *result)
	}

	if len(sessions) > 0 {
		go h.cleanupExpiredSessions()
	}

	api.JSONOK(c, dto.BatchUploadResponseDTO{Results: results})
}

// PrecheckUpload reports possible matches for client-provided fingerprints.
// @Summary Precheck uploads against existing content fingerprints
// @Description Given client-computed BLAKE3 fingerprints, reports advisory candidates. Candidates must still be uploaded for server-side full-file verification.
// @Tags assets
// @Accept json
// @Produce json
// @Param request body dto.UploadPrecheckRequestDTO true "Candidate files"
// @Success 200 {object} dto.UploadPrecheckResponseDTO
// @Failure 400 {object} api.ProblemResponse
// @Failure 404 {object} api.ProblemResponse
// @Failure 500 {object} api.ProblemResponse
// @Router /api/v1/assets/precheck [post]
func (h *AssetHandler) PrecheckUpload(c *gin.Context) {
	ctx := c.Request.Context()

	var req dto.UploadPrecheckRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	repository, err := h.resolveUploadRepository(ctx, req.RepositoryID)
	if err != nil {
		h.respondRepositoryError(c, err)
		return
	}

	contentHashes := make([]string, 0, len(req.Files))
	quickFingerprints := make([]string, 0, len(req.Files))
	for _, file := range req.Files {
		if file.IsQuick {
			if file.FingerprintVersion == nil || *file.FingerprintVersion != hash.QuickFingerprintVersion {
				continue
			}
			quickFingerprints = append(quickFingerprints, file.Hash)
		} else {
			contentHashes = append(contentHashes, file.Hash)
		}
	}

	contentRows, err := h.queries.ListAssetFullHashPrecheckMatches(ctx, repo.ListAssetFullHashPrecheckMatchesParams{
		FullHashes:   valueOrEmpty(dbtypes.StringsJSONParam(contentHashes)),
		RepositoryID: repository.RepoID,
	})
	if err != nil {
		api.WriteProblem(c, api.Internal(err))
		return
	}

	// Keyed by hash and size together: a quick hash only covers the first and
	// last 1 MiB, so size is part of the identity we match on.
	type fingerprint struct {
		hash string
		size int64
	}
	type existingAsset struct {
		assetID  string
		filename string
	}
	existing := make(map[fingerprint]existingAsset, len(contentRows))
	for _, row := range contentRows {
		key := fingerprint{hash: row.FullHash, size: row.FileSize}
		if _, seen := existing[key]; seen {
			continue
		}
		existing[key] = existingAsset{
			assetID:  row.AssetID.String(),
			filename: row.OriginalFilename,
		}
	}
	quickRows, err := h.queries.ListAssetQuickFingerprintPrecheckMatches(ctx, repo.ListAssetQuickFingerprintPrecheckMatchesParams{
		QuickFingerprints: valueOrEmpty(dbtypes.StringsJSONParam(quickFingerprints)),
		RepositoryID:      repository.RepoID,
	})
	if err != nil {
		api.WriteProblem(c, api.Internal(err))
		return
	}
	quickCandidates := make(map[fingerprint]existingAsset, len(quickRows))
	for _, row := range quickRows {
		if row.QuickFingerprint == nil {
			continue
		}
		key := fingerprint{hash: *row.QuickFingerprint, size: row.FileSize}
		quickCandidates[key] = existingAsset{assetID: row.AssetID.String(), filename: row.OriginalFilename}
	}

	results := make([]dto.UploadPrecheckResultDTO, 0, len(req.Files))
	duplicateCount := 0
	for _, file := range req.Files {
		result := dto.UploadPrecheckResultDTO{Hash: file.Hash}
		key := fingerprint{hash: file.Hash, size: file.Size}
		if file.IsQuick {
			if file.FingerprintVersion == nil || *file.FingerprintVersion != hash.QuickFingerprintVersion {
				results = append(results, result)
				continue
			}
			if match, ok := quickCandidates[key]; ok {
				result.Candidate = true
				result.AssetID = &match.assetID
				result.FileName = &match.filename
			}
		} else if match, ok := existing[key]; ok {
			result.Candidate = true
			result.AssetID = &match.assetID
			result.FileName = &match.filename
		}
		if result.Candidate {
			duplicateCount++
		}
		results = append(results, result)
	}

	api.JSONOK(c, dto.UploadPrecheckResponseDTO{
		Results:        results,
		DuplicateCount: duplicateCount,
	})
}

// GetUploadConfig returns current upload configuration
// @Summary Get upload configuration
// @Description Get current upload configuration including chunk size and concurrency limits based on system memory
// @Tags assets
// @Accept json
// @Produce json
// @Success 200 {object} dto.UploadConfigResponseDTO "Upload configuration"
// @Router /api/v1/assets/batch/config [get]
func (h *AssetHandler) GetUploadConfig(c *gin.Context) {
	config, err := h.memoryMonitor.GetOptimalChunkConfig()
	if err != nil {
		// Fallback to default config
		config = &memory.ChunkConfig{
			ChunkSize:           5 * 1024 * 1024,
			MaxConcurrent:       3,
			MemoryBuffer:        100 * 1024 * 1024,
			UpdateInterval:      30,
			MergeConcurrency:    2,
			MaxInFlightRequests: 3,
		}
	}

	response := dto.UploadConfigResponseDTO{
		ChunkSize:           config.ChunkSize,
		MaxConcurrent:       config.MaxConcurrent,
		MemoryBuffer:        config.MemoryBuffer,
		MergeConcurrency:    config.MergeConcurrency,
		MaxInFlightRequests: config.MaxInFlightRequests,
	}

	api.JSONOK(c, response)
}

// CreateUploadSession creates or resumes a durable chunk upload session.
// @Summary Create or resume an upload session
// @Tags assets
// @Accept json
// @Produce json
// @Param request body dto.CreateUploadSessionRequestDTO true "Upload metadata"
// @Success 200 {object} dto.UploadSessionResponseDTO
// @Router /api/v1/assets/batch/sessions [post]
func (h *AssetHandler) CreateUploadSession(c *gin.Context) {
	var req dto.CreateUploadSessionRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}
	repository, err := h.resolveUploadRepository(c.Request.Context(), req.RepositoryID)
	if err != nil {
		h.respondRepositoryError(c, err)
		return
	}
	if _, err := h.repoManager.CheckRepositoryWriteCapacity(c.Request.Context(), repository.RepoID.String(), uint64(req.TotalSize)); err != nil {
		h.respondCapacityError(c, err)
		return
	}
	userID := "anonymous"
	if id, ok := c.Get("user_id"); ok {
		userID = fmt.Sprintf("%d", id)
	}
	session := h.sessionManager.CreateSession(req.SessionID, filepath.Base(req.Filename), req.TotalSize, req.TotalChunks, req.ContentType, repository.RepoID.String(), userID)
	if req.ClientFingerprint != "" {
		h.sessionManager.SetSessionFingerprint(session.SessionID, req.ClientFingerprint)
	}
	chunks := make([]upload.ChunkInfo, 0, len(session.ReceivedChunks))
	for _, index := range session.ReceivedChunks {
		chunks = append(chunks, upload.ChunkInfo{SessionID: session.SessionID, ChunkIndex: index, PrivatePath: session.ChunkFiles[index], Size: session.ChunkSizes[index]})
	}
	h.chunkMerger.AddChunks(session.SessionID, chunks)
	api.JSONOK(c, dto.UploadSessionResponseDTO{SessionID: session.SessionID, Status: session.Status, TotalChunks: session.TotalChunks, ReceivedChunks: session.ReceivedChunks, BytesReceived: session.BytesReceived, ReceiptID: session.ReceiptID})
}

func (h *AssetHandler) respondCapacityError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, storage.ErrRepositoryReadOnly):
		api.WriteProblem(c, api.StatusProblem(http.StatusConflict, err))
	case errors.Is(err, storage.ErrInsufficientSpace):
		api.WriteProblem(c, api.StatusProblem(http.StatusInsufficientStorage, err))
	default:
		api.WriteProblem(c, api.Internal(err))
	}
}

// GetUploadProgress returns upload progress for sessions
// @Summary Get upload progress
// @Description Get detailed progress information for upload sessions
// @Tags assets
// @Accept json
// @Produce json
// @Param session_ids query string false "Comma-separated session IDs (optional)"
// @Success 200 {object} dto.UploadProgressResponseDTO "Upload progress details"
// @Router /api/v1/assets/batch/progress [get]
func (h *AssetHandler) GetUploadProgress(c *gin.Context) {
	sessionIDsParam := c.Query("session_ids")
	var targetSessions []*upload.UploadSession
	callerID := "anonymous"
	if id, exists := c.Get("user_id"); exists {
		callerID = fmt.Sprintf("%d", id)
	}

	if sessionIDsParam != "" {
		// Get specific sessions
		sessionIDs := strings.Split(sessionIDsParam, ",")
		for _, sessionID := range sessionIDs {
			if session, exists := h.sessionManager.GetSession(sessionID); exists && session.UserID == callerID {
				targetSessions = append(targetSessions, session)
			}
		}
	} else {
		// Get all sessions for current user
		targetSessions = h.sessionManager.GetSessionsByUser(callerID)
	}

	var totalBytesDone, totalBytesTotal int64
	var completedFiles int

	sessionsProgress := make([]dto.SessionProgressDTO, len(targetSessions))
	for i, session := range targetSessions {
		progress, _ := h.sessionManager.GetSessionProgress(session.SessionID)

		sessionsProgress[i] = dto.SessionProgressDTO{
			SessionID:       session.SessionID,
			Filename:        session.Filename,
			Status:          session.Status,
			Progress:        progress,
			Received:        len(session.ReceivedChunks),
			Total:           session.TotalChunks,
			BytesDone:       session.BytesReceived,
			BytesTotal:      session.TotalSize,
			LastActivity:    session.LastActivity,
			CompletedChunks: append([]int(nil), session.ReceivedChunks...),
		}

		totalBytesDone += session.BytesReceived
		totalBytesTotal += session.TotalSize

		if session.Status == "completed" {
			completedFiles++
		}
	}

	overallProgress := 0.0
	if totalBytesTotal > 0 {
		overallProgress = float64(totalBytesDone) / float64(totalBytesTotal)
	}

	summary := dto.ProgressSummaryDTO{
		TotalSessions:   len(targetSessions),
		ActiveSessions:  h.sessionManager.GetActiveSessionCount(),
		CompletedFiles:  completedFiles,
		FailedSessions:  0, // Would need to track failures separately
		OverallProgress: overallProgress,
	}

	response := dto.UploadProgressResponseDTO{
		Sessions: sessionsProgress,
		Summary:  summary,
	}

	api.JSONOK(c, response)
}

// GetUploadOperationStatus returns catalog-owned lifecycle state for accepted ingests.
// @Summary Get upload materialization status
// @Description Get ingest receipt state owned by the current caller
// @Tags assets
// @Produce json
// @Param receipt_ids query string true "Comma-separated catalog receipt IDs"
// @Success 200 {object} dto.UploadOperationStatusResponseDTO "Upload materialization status"
// @Failure 400 {object} api.ProblemResponse "Invalid receipt IDs"
// @Router /api/v1/assets/batch/operations [get]
func (h *AssetHandler) GetUploadOperationStatus(c *gin.Context) {
	statuses, err := h.loadUploadOperationStatuses(c, c.Query("receipt_ids"))
	if err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}
	api.JSONOK(c, dto.UploadOperationStatusResponseDTO{Operations: statuses})
}

// StreamUploadOperationStatus streams catalog ingest receipt updates until terminal.
// @Summary Stream upload materialization status
// @Tags assets
// @Produce text/event-stream
// @Param receipt_ids query string true "Comma-separated catalog receipt IDs"
// @Success 200 {string} string "SSE stream"
// @Router /api/v1/assets/batch/operations/stream [get]
func (h *AssetHandler) StreamUploadOperationStatus(c *gin.Context) {
	requestedIDs, err := parseUploadReceiptIDs(c.Query("receipt_ids"))
	if err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		api.WriteProblem(c, api.Internal(errors.New("streaming unsupported")))
		return
	}
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	ticker := time.NewTicker(500 * time.Millisecond)
	heartbeat := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	defer heartbeat.Stop()
	send := func(event string, value any) bool {
		data, err := json.Marshal(value)
		if err != nil {
			return false
		}
		_, err = fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", event, data)
		if err == nil {
			flusher.Flush()
		}
		return err == nil
	}
	for {
		statuses, err := h.loadUploadOperationStatuses(c, c.Query("receipt_ids"))
		if err != nil {
			send("error", problem.NewReference(problem.UploadProcessingFailed, true))
			return
		}
		if !send("operations", dto.UploadOperationStatusResponseDTO{Operations: statuses}) {
			return
		}
		if allRequestedUploadOperationsTerminal(requestedIDs, statuses) {
			send("done", dto.UploadOperationStatusResponseDTO{Operations: statuses})
			return
		}
		select {
		case <-c.Request.Context().Done():
			return
		case <-ticker.C:
		case <-heartbeat.C:
			if !send("heartbeat", map[string]int64{"timestamp": time.Now().Unix()}) {
				return
			}
		}
	}
}

func parseUploadReceiptIDs(raw string) ([]uuid.UUID, error) {
	rawIDs := strings.Split(strings.TrimSpace(raw), ",")
	if len(rawIDs) == 0 || len(rawIDs) > 100 || (len(rawIDs) == 1 && strings.TrimSpace(rawIDs[0]) == "") {
		return nil, errors.New("receipt_ids must contain between 1 and 100 IDs")
	}
	ids := make([]uuid.UUID, 0, len(rawIDs))
	for _, rawID := range rawIDs {
		id, err := uuid.Parse(strings.TrimSpace(rawID))
		if err != nil {
			return nil, errors.New("receipt_ids must be UUIDs")
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func (h *AssetHandler) loadUploadOperationStatuses(c *gin.Context, raw string) ([]dto.UploadOperationStatusDTO, error) {
	ids, err := parseUploadReceiptIDs(raw)
	if err != nil {
		return nil, err
	}
	callerID, err := currentUserIDFromContext(c)
	if err != nil || callerID == nil {
		return nil, errors.New("upload owner is unavailable")
	}
	statuses := make([]dto.UploadOperationStatusDTO, 0, len(ids))
	for _, id := range ids {
		var status dto.UploadOperationStatusDTO
		var terminalError sql.NullString
		reader := h.readerDatabase
		if reader == nil {
			reader = h.database
		}
		err := reader.QueryRowContext(c, `SELECT receipt.receipt_id, staging_commit.original_filename, receipt.state, receipt.terminal_error FROM catalog_operation_receipts receipt JOIN repository_staging_commits staging_commit ON staging_commit.commit_id = receipt.subject_id WHERE receipt.receipt_id = ? AND receipt.kind = 'ingest' AND staging_commit.owner_id = ?`, id.String(), *callerID).Scan(&status.ReceiptID, &status.FileName, &status.Status, &terminalError)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return nil, err
		}
		status.Terminal = status.Status == "completed" || status.Status == "failed"
		status.Success = status.Status == "completed"
		if terminalError.Valid {
			value := problem.ReferenceFor(problem.UploadProcessingFailed, "receipt:"+status.ReceiptID, true)
			status.Problem = &value
		}
		statuses = append(statuses, status)
	}

	return statuses, nil
}

// allRequestedUploadJobsTerminal is true only when every requested task ID is
// present in statuses and marked terminal. A partial/ownership-filtered set must
// not end the SSE stream early.
func allRequestedUploadOperationsTerminal(requested []uuid.UUID, statuses []dto.UploadOperationStatusDTO) bool {
	if len(requested) == 0 {
		return false
	}
	byID := make(map[string]dto.UploadOperationStatusDTO, len(statuses))
	for _, status := range statuses {
		byID[status.ReceiptID] = status
	}
	for _, id := range requested {
		status, ok := byID[id.String()]
		if !ok || !status.Terminal {
			return false
		}
	}
	return true
}

func newUploadProblem(retryable bool) *problem.Reference {
	value := problem.NewReference(problem.UploadProcessingFailed, retryable)
	return &value
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

// GetAssetSidecar retrieves the Lumilio edit sidecar for an asset.
// @Summary Get asset edit sidecar
// @Description Retrieve the non-destructive Studio edit sidecar stored under the asset repository .lumilio directory.
// @Tags assets
// @Accept json
// @Produce json
// @Param id path string true "Asset ID (UUID format)" example("550e8400-e29b-41d4-a716-446655440000")
// @Success 200 {object} dto.AssetSidecarResponseDTO "Asset sidecar"
// @Failure 400 {object} api.ProblemResponse "Invalid asset ID"
// @Failure 404 {object} api.ProblemResponse "Asset not found"
// @Failure 500 {object} api.ProblemResponse "Internal server error"
// @Router /api/v1/assets/{id}/sidecar [get]
func (h *AssetHandler) GetAssetSidecar(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	asset, ok := h.getAuthorizedAsset(c, id, "Authentication required to access this asset", "You don't have permission to access this asset")
	if !ok {
		return
	}

	opened, err := h.locationResolver.OpenAsset(c.Request.Context(), id)
	if err != nil {
		respondRepositoryResolveError(c, err, "Failed to resolve asset location")
		return
	}
	repositoryID := opened.Catalog.RepoID.String()
	projectedPath := opened.Path.String()
	_ = opened.Close()
	source, err := h.sidecarSourceForAsset(c.Request.Context(), asset, projectedPath)
	if err != nil {
		api.WriteProblem(c, api.Internal(err))
		return
	}
	sidecar := h.defaultSidecarForAsset(id, source)
	exists := false

	content, err := h.repoManager.ReadRepositorySidecar(c.Request.Context(), repositoryID, id.String())
	if err != nil {
		api.WriteProblem(c, api.Internal(err))
		return
	}
	if content != nil {
		if err := json.Unmarshal(content, &sidecar); err != nil {
			api.WriteProblem(c, api.Internal(err))
			return
		}
		exists = true
	}

	if sidecar.Version == 0 {
		sidecar.Version = 1
	}
	if sidecar.AssetID == "" {
		sidecar.AssetID = id.String()
	}

	api.JSONOK(c, dto.AssetSidecarResponseDTO{
		AssetID: id.String(),
		Exists:  exists,
		Sidecar: sidecar,
	})
}

// UpdateAssetSidecar stores the Lumilio edit sidecar for an asset.
// @Summary Update asset edit sidecar
// @Description Store non-destructive Studio edit data under the asset repository .lumilio directory.
// @Tags assets
// @Produce json
// @Param id path string true "Asset ID (UUID format)" example("550e8400-e29b-41d4-a716-446655440000")
// @Param data body dto.LumilioSidecarV1DTO true "Sidecar payload"
// @Success 200 {object} dto.AssetSidecarResponseDTO "Asset sidecar saved"
// @Failure 400 {object} api.ProblemResponse "Invalid asset ID or request body"
// @Failure 404 {object} api.ProblemResponse "Asset not found"
// @Failure 500 {object} api.ProblemResponse "Internal server error"
// @Router /api/v1/assets/{id}/sidecar [put]
func (h *AssetHandler) UpdateAssetSidecar(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	asset, ok := h.getAuthorizedAsset(c, id, "Authentication required to update this asset", "You don't have permission to update this asset")
	if !ok {
		return
	}

	var sidecar dto.LumilioSidecarV1DTO
	if err := c.ShouldBindJSON(&sidecar); err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	sidecar.Version = 1
	sidecar.AssetID = id.String()
	opened, err := h.locationResolver.OpenAsset(c.Request.Context(), id)
	if err != nil {
		respondRepositoryResolveError(c, err, "Failed to resolve asset location")
		return
	}
	repositoryID := opened.Catalog.RepoID.String()
	projectedPath := opened.Path.String()
	_ = opened.Close()
	sidecar.Source, err = h.sidecarSourceForAsset(c.Request.Context(), asset, projectedPath)
	if err != nil {
		api.WriteProblem(c, api.Internal(err))
		return
	}
	sidecar.UpdatedAt = time.Now().UTC()

	content, err := json.MarshalIndent(sidecar, "", "  ")
	if err != nil {
		api.WriteProblem(c, api.Internal(err))
		return
	}

	if err := h.repoManager.WriteRepositorySidecar(c.Request.Context(), repositoryID, id.String(), content); err != nil {
		api.WriteProblem(c, api.Internal(err))
		return
	}

	api.JSONOK(c, dto.AssetSidecarResponseDTO{
		AssetID: id.String(),
		Exists:  true,
		Sidecar: sidecar,
	})
}

// GetAssetThumbnail retrieves a thumbnail for a specific asset by asset ID and size
// @Summary Get asset thumbnail
// @Description Retrieve a specific thumbnail image for an asset by asset ID and size parameter. Returns the image file directly.
// @Tags assets
// @Produce image/jpeg
// @Param id path string true "Asset ID (UUID format)" example("550e8400-e29b-41d4-a716-446655440000")
// @Param size query string false "Thumbnail size" default(medium) Enums(small,medium,large)
// @Success 200 {file} string "Thumbnail image file"
// @Failure 400 {object} api.ProblemResponse "Invalid asset ID or size parameter"
// @Failure 404 {object} api.ProblemResponse "Asset or thumbnail not found"
// @Failure 500 {object} api.ProblemResponse "Internal server error"
// @Router /api/v1/assets/{id}/thumbnail [get]
func (h *AssetHandler) GetAssetThumbnail(c *gin.Context) {
	// Parse asset ID from URL parameter
	idStr := c.Param("id")
	assetID, err := uuid.Parse(idStr)
	if err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	// Get size parameter from query (default to "medium")
	size := c.DefaultQuery("size", "medium")

	// Validate size parameter
	if size != "small" && size != "medium" && size != "large" {
		api.WriteProblem(c, api.BadRequest(errors.New("invalid size parameter")))
		return
	}

	_, ok := h.getAuthorizedAssetForMedia(c, assetID, "Authentication required to access this thumbnail", "You don't have permission to access this thumbnail")
	if !ok {
		return
	}

	// Get thumbnail from service
	thumbnail, err := h.assetService.GetThumbnailByAssetIDAndSize(c.Request.Context(), assetID, size)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			api.WriteProblem(c, api.NotFound(err))
			return
		}
		log.Printf("Failed to retrieve thumbnail metadata: %v", err)
		api.WriteProblem(c, api.Internal(err))
		return
	}

	repository, err := h.queries.GetRepository(c.Request.Context(), thumbnail.RepositoryID)
	if err != nil {
		log.Printf("Failed to resolve repository for thumbnail request: %v", err)
		respondRepositoryResolveError(c, err, "Failed to resolve repository")
		return
	}
	repositoryFS, file, err := openRepositoryPrivate(h.files, repository, thumbnail.StoragePath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			api.WriteProblem(c, api.NotFound(err))
			return
		}
		log.Printf("Failed to open thumbnail: %v", err)
		api.WriteProblem(c, api.Internal(err))
		return
	}
	fileInfo, err := file.Stat()
	if err != nil {
		_ = file.Close()
		_ = repositoryFS.Close()
		api.WriteProblem(c, api.Internal(err))
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
		_ = file.Close()
		_ = repositoryFS.Close()
		log.Printf("Request for asset %s thumbnail (%s) - 304 Not Modified (ETag: %s)", assetID.String(), size, etag)
		c.Status(http.StatusNotModified)
		return
	}

	serveRepositoryFile(c, repositoryFS, file, thumbnail.StoragePath)
}

// GetOriginalFile serves the original file content by asset ID
// @Summary Get original file
// @Description Serve the original file content for an asset by asset ID. Returns the file as an octet-stream.
// @Tags assets
// @Produce application/octet-stream
// @Param id path string true "Asset ID (UUID format)" example("550e8400-e29b-41d4-a716-446655440000")
// @Success 200 {file} file "Original file content"
// @Failure 400 {object} api.ProblemResponse "Invalid asset ID"
// @Failure 404 {object} api.ProblemResponse "Asset not found"
// @Failure 500 {object} api.ProblemResponse "Internal server error"
// @Router /api/v1/assets/{id}/original [get]
func (h *AssetHandler) GetOriginalFile(c *gin.Context) {
	ctx := c.Request.Context()

	// Parse asset ID from URL parameter
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	asset, ok := h.getAuthorizedAssetForMedia(c, id, "Authentication required to access this file", "You don't have permission to access this file")
	if !ok {
		return
	}

	opened, err := h.locationResolver.OpenAsset(ctx, asset.AssetID)
	if err != nil {
		log.Printf("Failed to resolve active location for original file: %v", err)
		respondRepositoryResolveError(c, err, "Failed to access repository")
		return
	}

	// Set appropriate headers
	c.Header("Cache-Control", "public, max-age=86400") // Cache for 1 day
	c.Header("Content-Type", asset.MimeType)
	c.Header("Content-Disposition", fmt.Sprintf("inline; filename=\"%s\"", asset.OriginalFilename))

	serveRepositoryFile(c, opened.Repository, opened.File, asset.OriginalFilename)
}

// clampedIntQuery parses an integer query parameter, returning def when absent
// or invalid, and clamping the result to [min, max].
func clampedIntQuery(c *gin.Context, key string, def, min, max int) int {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return def
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// ExportAsset re-encodes an asset's original file to a requested format and size.
// @Summary Export asset
// @Description Re-encode an asset's original file to JPEG, PNG, WebP, or AVIF with optional max dimensions and quality, and stream it back as a download.
// @Tags assets
// @Produce image/jpeg,image/png,image/webp,image/avif
// @Param id path string true "Asset ID"
// @Param format query string true "Output format (jpeg, png, webp, avif)"
// @Param quality query int false "Quality 1-100 for lossy formats"
// @Param max_width query int false "Maximum output width in pixels"
// @Param max_height query int false "Maximum output height in pixels"
// @Param filename query string false "Base download filename (without extension)"
// @Success 200 {file} file "Encoded image"
// @Failure 400 {object} api.ProblemResponse "Invalid request"
// @Failure 401 {object} api.ProblemResponse "Authentication required"
// @Failure 403 {object} api.ProblemResponse "Forbidden"
// @Failure 404 {object} api.ProblemResponse "Asset or original file not found"
// @Failure 422 {object} api.ProblemResponse "Source image could not be encoded"
// @Failure 500 {object} api.ProblemResponse "Internal server error"
// @Router /api/v1/assets/{id}/export [get]
func (h *AssetHandler) ExportAsset(c *gin.Context) {
	ctx := c.Request.Context()

	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	format := strings.ToLower(strings.TrimSpace(c.Query("format")))
	if !imaging.IsSupportedExportFormat(format) {
		api.WriteProblem(c, api.BadRequest(fmt.Errorf("unsupported export format %q", format)))
		return
	}

	asset, ok := h.getAuthorizedAssetForMedia(c, id, "Authentication required to export this file", "You don't have permission to export this file")
	if !ok {
		return
	}

	opened, fullPath, err := h.locationResolver.LocalAssetPath(ctx, asset.AssetID)
	if err != nil {
		log.Printf("Failed to resolve active location for export: %v", err)
		respondRepositoryResolveError(c, err, "Failed to access repository")
		return
	}
	defer opened.Close()
	_ = opened.File.Close()
	opened.File = nil

	// OpenPhoto yields a libvips-decodable source for any photo: RAW files are
	// resolved to their embedded preview (full render as fallback), non-RAW files
	// are opened directly. This is what lets the export endpoint handle RAW.
	reader, err := imagesource.OpenPhoto(ctx, fullPath, asset.OriginalFilename)
	if err != nil {
		log.Printf("Failed to open source for export of asset %s: %v", id, err)
		api.WriteProblem(c, api.StatusProblem(http.StatusUnprocessableEntity, err))
		return
	}
	defer reader.Close()

	buf, err := io.ReadAll(reader)
	if err != nil {
		log.Printf("Failed to read source for export of asset %s: %v", id, err)
		api.WriteProblem(c, api.Internal(err))
		return
	}

	out, mime, ext, err := imaging.ExportImageBytes(buf, imaging.ExportParams{
		Format:    format,
		Quality:   clampedIntQuery(c, "quality", 0, 1, 100),
		MaxWidth:  clampedIntQuery(c, "max_width", 0, 0, 60000),
		MaxHeight: clampedIntQuery(c, "max_height", 0, 0, 60000),
	})
	if err != nil {
		log.Printf("Failed to export asset %s as %s: %v", id, format, err)
		api.WriteProblem(c, api.StatusProblem(http.StatusUnprocessableEntity, err))
		return
	}

	base := strings.TrimSuffix(asset.OriginalFilename, filepath.Ext(asset.OriginalFilename))
	if q := strings.TrimSpace(c.Query("filename")); q != "" {
		base = q
	}
	if strings.TrimSpace(base) == "" {
		base = "export"
	}

	c.Header("Cache-Control", "private, max-age=0")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", base+"."+ext))
	c.Data(http.StatusOK, mime, out)
}

// DownloadAssets serves multiple original files as a zip archive.
// @Summary Download assets
// @Description Serve original files for the requested asset IDs as a zip archive.
// @Tags assets
// @Produce application/zip
// @Param data body dto.DownloadAssetsRequestDTO true "Asset IDs to download"
// @Success 200 {file} file "Zip archive"
// @Failure 400 {object} api.ProblemResponse "Invalid request"
// @Failure 401 {object} api.ProblemResponse "Authentication required"
// @Failure 403 {object} api.ProblemResponse "Forbidden"
// @Failure 404 {object} api.ProblemResponse "Asset or original file not found"
// @Failure 500 {object} api.ProblemResponse "Internal server error"
// @Router /api/v1/assets/download [post]
func (h *AssetHandler) DownloadAssets(c *gin.Context) {
	ctx := c.Request.Context()

	var req dto.DownloadAssetsRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	if len(req.AssetIDs) == 0 {
		api.WriteProblem(c, api.BadRequest(errors.New("asset_ids is required")))
		return
	}

	files := make([]assetDownloadFile, 0, len(req.AssetIDs))
	for _, rawAssetID := range req.AssetIDs {
		assetIDText := strings.TrimSpace(rawAssetID)
		assetID, err := uuid.Parse(assetIDText)
		if err != nil {
			api.WriteProblem(c, api.BadRequest(err))
			return
		}

		asset, ok := h.getAuthorizedAssetForMedia(c, assetID, "Authentication required to access this file", "You don't have permission to access this file")
		if !ok {
			return
		}

		files = append(files, assetDownloadFile{asset: *asset})
	}

	filename := fmt.Sprintf("lumilio-assets-%s.zip", time.Now().Format("20060102-150405"))
	c.Header("Cache-Control", "no-store")
	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	c.Status(http.StatusOK)

	zipWriter := zip.NewWriter(c.Writer)
	archiveNames := make(map[string]int, len(files))
	for _, file := range files {
		if err := writeAssetToZip(ctx, h.locationResolver, zipWriter, archiveNames, file); err != nil {
			log.Printf("Failed to write asset to zip: %v", err)
			_ = zipWriter.Close()
			return
		}
	}

	if err := zipWriter.Close(); err != nil {
		log.Printf("Failed to finalize asset download zip: %v", err)
	}
}

// GetWebVideo serves the web-optimized video version by asset ID
// @Summary Get web-optimized video
// @Description Serve the web-optimized MP4 video version for an asset by asset ID.
// @Tags assets
// @Produce video/mp4
// @Param id path string true "Asset ID (UUID format)" example("550e8400-e29b-41d4-a716-446655440000")
// @Success 200 {file} file "Web-optimized video file"
// @Failure 400 {object} api.ProblemResponse "Invalid asset ID"
// @Failure 404 {object} api.ProblemResponse "Asset not found or not a video"
// @Failure 500 {object} api.ProblemResponse "Internal server error"
// @Router /api/v1/assets/{id}/video/web [get]
func (h *AssetHandler) GetWebVideo(c *gin.Context) {
	ctx := c.Request.Context()

	// Parse asset ID from URL parameter
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	// Get asset metadata from service
	asset, ok := h.getAuthorizedAssetForMedia(c, id, "Authentication required to access this video", "You don't have permission to access this video")
	if !ok {
		return
	}

	// Check if asset is a video
	if asset.Type != "VIDEO" {
		api.WriteProblem(c, api.BadRequest(fmt.Errorf("asset is not a video")))
		return
	}

	repositoryFS, file, err := openWebOrOriginal(ctx, h.locationResolver, asset, "videos", "_web.mp4")
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			api.WriteProblem(c, api.NotFound(err))
		} else {
			api.WriteProblem(c, api.Internal(err))
		}
		return
	}

	// Set appropriate headers for video streaming
	c.Header("Cache-Control", "public, max-age=86400") // Cache for 1 day
	c.Header("Content-Type", "video/mp4")
	c.Header("Accept-Ranges", "bytes") // Enable range requests for video seeking

	serveRepositoryFile(c, repositoryFS, file, asset.OriginalFilename)
}

// GetWebAudio serves the web-optimized audio version by asset ID
// @Summary Get web-optimized audio
// @Description Serve the web-optimized MP3 audio version for an asset by asset ID.
// @Tags assets
// @Produce audio/mpeg
// @Param id path string true "Asset ID (UUID format)" example("550e8400-e29b-41d4-a716-446655440000")
// @Success 200 {file} file "Web-optimized audio file"
// @Failure 400 {object} api.ProblemResponse "Invalid asset ID"
// @Failure 404 {object} api.ProblemResponse "Asset not found or not audio"
// @Failure 500 {object} api.ProblemResponse "Internal server error"
// @Router /api/v1/assets/{id}/audio/web [get]
func (h *AssetHandler) GetWebAudio(c *gin.Context) {
	ctx := c.Request.Context()

	// Parse asset ID from URL parameter
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	// Get asset metadata from service
	asset, ok := h.getAuthorizedAssetForMedia(c, id, "Authentication required to access this audio", "You don't have permission to access this audio")
	if !ok {
		return
	}

	// Check if asset is audio
	if asset.Type != "AUDIO" {
		api.WriteProblem(c, api.BadRequest(fmt.Errorf("asset is not audio")))
		return
	}

	repositoryFS, file, err := openWebOrOriginal(ctx, h.locationResolver, asset, "audios", "_web.mp3")
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			api.WriteProblem(c, api.NotFound(err))
		} else {
			api.WriteProblem(c, api.Internal(err))
		}
		return
	}

	// Set appropriate headers for audio streaming
	c.Header("Cache-Control", "public, max-age=86400") // Cache for 1 day
	c.Header("Content-Type", "audio/mpeg")
	c.Header("Vary", "Accept-Encoding")
	c.Header("Accept-Ranges", "bytes") // Enable range requests for audio seeking

	serveRepositoryFile(c, repositoryFS, file, asset.OriginalFilename)
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

// GetAssetTypes returns available asset types
// @Summary Get supported asset types
// @Description Retrieve a list of all supported asset types in the system.
// @Tags assets
// @Accept json
// @Produce json
// @Success 200 {object} dto.AssetTypesResponseDTO "Asset types retrieved successfully"
// @Router /api/v1/assets/types [get]
func (h *AssetHandler) GetAssetTypes(c *gin.Context) {
	types := []dbtypes.AssetType{
		dbtypes.AssetTypePhoto,
		dbtypes.AssetTypeVideo,
		dbtypes.AssetTypeAudio,
	}

	api.JSONOK(c, dto.AssetTypesResponseDTO{Types: types})
}

func normalizeAssetQueryPagination(pagination *dto.PaginationDTO) {
	if pagination.Limit <= 0 || pagination.Limit > 100 {
		pagination.Limit = 20
	}
	if pagination.Offset < 0 {
		pagination.Offset = 0
	}
}

func validateAssetQuerySearchType(searchType string) error {
	if searchType == "" || searchType == "filename" || searchType == "semantic" {
		return nil
	}
	return errors.New("invalid search type")
}

func validateAssetQuerySortBy(sortBy string) error {
	switch strings.ToLower(strings.TrimSpace(sortBy)) {
	case "", "recently_added", "date_captured":
		return nil
	default:
		return errors.New("invalid sort_by")
	}
}

func validateSearchEnhancementMode(mode string) error {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", "auto", "off", "only":
		return nil
	default:
		return errors.New("invalid enhancement mode")
	}
}

func validateStackMode(mode string) error {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", service.StackModeCollapsed, service.StackModeExpanded:
		return nil
	default:
		return errors.New("invalid stack mode")
	}
}

// rejectSearchStackMode enforces that search requests carry no stack_mode:
// search results are always flat by media item (relevance order must not be
// reordered by stack collapse).
func rejectSearchStackMode(mode string) error {
	if strings.TrimSpace(mode) != "" {
		return errors.New("stack_mode is not supported for search")
	}
	return nil
}

func normalizeRebuildIndexLimit(limit int) int {
	switch {
	case limit <= 0:
		return 200
	case limit > 500:
		return 500
	default:
		return limit
	}
}

func parseIndexingTasks(tasks []string) ([]service.AssetIndexingTask, error) {
	if len(tasks) == 0 {
		return nil, nil
	}

	result := make([]service.AssetIndexingTask, 0, len(tasks))
	for _, rawTask := range tasks {
		task := service.AssetIndexingTask(strings.ToLower(strings.TrimSpace(rawTask)))
		switch task {
		case service.AssetIndexingTaskSemanticImage,
			service.AssetIndexingTaskOCR,
			service.AssetIndexingTaskFaceRecognition,
			service.AssetIndexingTaskVideoSemantic:
			result = append(result, task)
		case service.AssetIndexingTaskBioCLIP:
			return nil, fmt.Errorf("bioclip indexing is album-scoped")
		default:
			return nil, fmt.Errorf("invalid indexing task: %s", rawTask)
		}
	}
	return result, nil
}

func toIndexingStatsResponseDTO(stats service.AssetIndexingStats) dto.AssetIndexingStatsResponseDTO {
	return dto.AssetIndexingStatsResponseDTO{
		PhotoTotal:  int(stats.PhotoTotal),
		VideoTotal:  int(stats.VideoTotal),
		ReindexJobs: int(stats.ReindexJobs),
		Tasks: dto.AssetIndexingTaskSetStatsDTO{
			Semantic: dto.AssetIndexingTaskStatsDTO{
				IndexedCount: int(stats.Tasks.Semantic.IndexedCount),
				QueuedJobs:   int(stats.Tasks.Semantic.QueuedJobs),
				TotalCount:   int(stats.Tasks.Semantic.TotalCount),
			},
			BioCLIP: dto.AssetIndexingTaskStatsDTO{
				IndexedCount: int(stats.Tasks.BioCLIP.IndexedCount),
				QueuedJobs:   int(stats.Tasks.BioCLIP.QueuedJobs),
				TotalCount:   int(stats.Tasks.BioCLIP.TotalCount),
			},
			OCR: dto.AssetIndexingTaskStatsDTO{
				IndexedCount: int(stats.Tasks.OCR.IndexedCount),
				QueuedJobs:   int(stats.Tasks.OCR.QueuedJobs),
				TotalCount:   int(stats.Tasks.OCR.TotalCount),
			},
			Face: dto.AssetIndexingTaskStatsDTO{
				IndexedCount: int(stats.Tasks.Face.IndexedCount),
				QueuedJobs:   int(stats.Tasks.Face.QueuedJobs),
				TotalCount:   int(stats.Tasks.Face.TotalCount),
			},
			VideoSemantic: dto.AssetIndexingTaskStatsDTO{
				IndexedCount: int(stats.Tasks.VideoSemantic.IndexedCount),
				QueuedJobs:   int(stats.Tasks.VideoSemantic.QueuedJobs),
				TotalCount:   int(stats.Tasks.VideoSemantic.TotalCount),
			},
		},
	}
}

func toIndexingRepositoryListResponseDTO(repositories []*repo.Repository, includePath bool) dto.IndexingRepositoryListResponseDTO {
	items := make([]dto.IndexingRepositoryOptionDTO, 0, len(repositories))
	for _, repository := range repositories {
		if repository == nil {
			continue
		}
		item := dto.IndexingRepositoryOptionDTO{
			ID:           repository.RepoID.String(),
			Name:         repository.Name,
			Role:         string(repository.Role),
			RootID:       repository.RootID.String(),
			Reachability: string(repository.Reachability),
			Activity:     string(repository.Activity),
			PauseReason:  repository.PauseReason,
			IsPrimary:    repository.Role == dbtypes.RepoRolePrimary,
		}
		if includePath {
			item.Path = repository.Path
		}
		items = append(items, item)
	}

	return dto.IndexingRepositoryListResponseDTO{
		Repositories: items,
	}
}

func normalizeAssetQuerySortBy(sortBy string) string {
	switch strings.ToLower(strings.TrimSpace(sortBy)) {
	case "recently_added":
		return "recently_added"
	case "date_captured":
		return "date_captured"
	default:
		return "date_captured"
	}
}

func normalizeFilenameOperator(operator string) string {
	switch strings.ToLower(strings.TrimSpace(operator)) {
	case "matches":
		return "matches"
	case "starts_with", "startswith":
		return "starts_with"
	case "ends_with", "endswith":
		return "ends_with"
	default:
		return "contains"
	}
}

// normalizeFolderPath normalizes a repository-relative folder path filter:
// it converts platform separators to '/', collapses repeated separators via
// path.Clean, and trims leading/trailing slashes so SQL prefix matching
// against storage_path is consistent regardless of client input.
func normalizeFolderPath(folderPath string) string {
	cleaned := strings.ReplaceAll(folderPath, "\\", "/")
	cleaned = path.Clean(cleaned)
	cleaned = strings.Trim(cleaned, "/")
	if cleaned == "." {
		return ""
	}
	return cleaned
}

func assetQueryDateLocation(viewerTimeZone string) *time.Location {
	if strings.TrimSpace(viewerTimeZone) == "" {
		return time.UTC
	}
	location, err := time.LoadLocation(strings.TrimSpace(viewerTimeZone))
	if err != nil {
		return time.UTC
	}
	return location
}

// browseFilterFromDTO validates and normalizes the media-item/stack filter
// blocks: unknown enum values are rejected, empty kinds arrays normalize to
// unset, and unstacked+kinds is contradictory (mirrors the service check so
// the request fails fast with a 400).
func browseFilterFromDTO(filter dto.AssetFilterDTO) (service.MediaComposition, service.StackMembership, []string, error) {
	var composition service.MediaComposition
	if filter.MediaItem != nil && filter.MediaItem.Composition != nil {
		switch service.MediaComposition(*filter.MediaItem.Composition) {
		case service.MediaCompositionContainsRAW, service.MediaCompositionJPEGRAW,
			service.MediaCompositionRAWUnpaired, service.MediaCompositionNoRAW,
			service.MediaCompositionLivePhoto:
			composition = service.MediaComposition(*filter.MediaItem.Composition)
		default:
			return "", "", nil, fmt.Errorf("unknown media_item.composition: %q", string(*filter.MediaItem.Composition))
		}
	}

	var membership service.StackMembership
	var kinds []string
	if filter.Stack != nil {
		if filter.Stack.Membership != nil {
			switch service.StackMembership(*filter.Stack.Membership) {
			case service.StackMembershipStacked, service.StackMembershipUnstacked:
				membership = service.StackMembership(*filter.Stack.Membership)
			default:
				return "", "", nil, fmt.Errorf("unknown stack.membership: %q", string(*filter.Stack.Membership))
			}
		}
		for _, kind := range filter.Stack.Kinds {
			kind = strings.ToLower(strings.TrimSpace(kind))
			if kind == "" {
				continue
			}
			if !dbtypes.StackKind(kind).Valid() {
				return "", "", nil, fmt.Errorf("unknown stack.kinds value: %q", kind)
			}
			kinds = append(kinds, kind)
		}
		if membership == service.StackMembershipUnstacked && len(kinds) > 0 {
			return "", "", nil, fmt.Errorf("stack.membership=unstacked excludes stack.kinds")
		}
	}
	return composition, membership, kinds, nil
}

func buildQueryAssetsParams(query, searchType, sortBy, viewerTimeZone, stackMode string, filter dto.AssetFilterDTO, pagination dto.PaginationDTO) (service.QueryAssetsParams, error) {
	mediaComposition, stackMembership, stackKinds, err := browseFilterFromDTO(filter)
	if err != nil {
		return service.QueryAssetsParams{}, err
	}

	var dateFrom, dateTo *time.Time
	if filter.Date != nil {
		dateFrom = filter.Date.From
		dateTo = filter.Date.To

		// Normalize date-only inputs in the viewer's timezone. Exact timestamps
		// remain exact.
		location := assetQueryDateLocation(viewerTimeZone)
		if dateFrom != nil && filter.Date.FromDateOnly {
			start := time.Date(dateFrom.Year(), dateFrom.Month(), dateFrom.Day(), 0, 0, 0, 0, location)
			dateFrom = &start
		}
		if dateFrom != nil && dateTo == nil && filter.Date.FromDateOnly {
			end := time.Date(dateFrom.Year(), dateFrom.Month(), dateFrom.Day(), 23, 59, 59, int(time.Second-time.Nanosecond), location)
			dateTo = &end
		} else if dateTo != nil && filter.Date.ToDateOnly {
			end := time.Date(dateTo.Year(), dateTo.Month(), dateTo.Day(), 23, 59, 59, int(time.Second-time.Nanosecond), location)
			dateTo = &end
		}
	}

	var albumIDPtr *int32
	if filter.AlbumID != nil {
		id := int32(*filter.AlbumID)
		albumIDPtr = &id
	}

	var filenameValue, filenameOperator *string
	if filter.Filename != nil && strings.TrimSpace(filter.Filename.Value) != "" {
		value := strings.TrimSpace(filter.Filename.Value)
		operator := normalizeFilenameOperator(filter.Filename.Operator)
		filenameValue = &value
		filenameOperator = &operator
	}

	var locationNorth, locationSouth, locationEast, locationWest *float64
	if filter.Location != nil {
		locationNorth = &filter.Location.North
		locationSouth = &filter.Location.South
		locationEast = &filter.Location.East
		locationWest = &filter.Location.West
	}

	var folderPath *string
	if filter.FolderPath != nil {
		normalized := normalizeFolderPath(*filter.FolderPath)
		folderPath = &normalized
	}

	return service.QueryAssetsParams{
		Query:            query,
		SearchType:       searchType,
		ViewerTimeZone:   viewerTimeZone,
		RepositoryID:     filter.RepositoryID,
		EventID:          filter.EventID,
		AssetType:        filter.Type,
		AssetTypes:       filter.Types,
		OwnerID:          filter.OwnerID,
		AlbumID:          albumIDPtr,
		FilenameValue:    filenameValue,
		FilenameOperator: filenameOperator,
		DateFrom:         dateFrom,
		DateTo:           dateTo,
		MediaComposition: mediaComposition,
		StackMembership:  stackMembership,
		StackKinds:       stackKinds,
		IsDeleted:        filter.IsDeleted,
		Rating:           filter.Rating,
		Liked:            filter.Liked,
		CameraModel:      filter.CameraModel,
		LensModel:        filter.Lens,
		TagName:          filter.TagName,
		TagSource:        filter.TagSource,
		TagNames:         filter.TagNames,
		PersonID:         filter.PersonID,
		FolderPath:       folderPath,
		FolderRecursive:  filter.FolderRecursive,
		LocationNorth:    locationNorth,
		LocationSouth:    locationSouth,
		LocationEast:     locationEast,
		LocationWest:     locationWest,
		SortBy:           normalizeAssetQuerySortBy(sortBy),
		StackMode:        strings.ToLower(strings.TrimSpace(stackMode)),
		Limit:            pagination.Limit,
		Offset:           pagination.Offset,
	}, nil
}

func toAssetDTOs(assets []repo.Asset) []dto.AssetDTO {
	items := make([]dto.AssetDTO, len(assets))
	for i, asset := range assets {
		items[i] = dto.ToAssetDTO(asset)
	}
	return items
}

func toBrowseMediaItemDTO(media service.BrowseMediaItem) dto.BrowseMediaItemDTO {
	item := dto.BrowseMediaItemDTO{
		MediaItemID:  media.MediaItemID.String(),
		MediaKind:    media.MediaKind,
		PrimaryAsset: dto.ToAssetDTO(media.PrimaryAsset),
		Composition: dto.MediaCompositionDTO{
			ComponentCount: media.ComponentCount,
			HasRAW:         media.HasRAW,
			HasJPEG:        media.HasJPEG,
			HasEdited:      media.HasEdited,
			HasLiveMotion:  media.HasLiveMotion,
		},
	}
	if media.StackID != uuid.Nil {
		item.Stack = &dto.StackPreviewDTO{
			StackID:   media.StackID.String(),
			StackKind: string(media.StackKind),
		}
	}
	return item
}

func toBrowseStackMemberDTOs(members []service.BrowseStackMember) []dto.BrowseStackMemberDTO {
	items := make([]dto.BrowseStackMemberDTO, 0, len(members))
	for _, member := range members {
		items = append(items, dto.BrowseStackMemberDTO{
			MediaItemID:    member.MediaItemID.String(),
			PrimaryAssetID: member.PrimaryAssetID.String(),
		})
	}
	return items
}

func toBrowseItemDTOs(items []service.BrowseItem) []dto.BrowseItemDTO {
	dtos := make([]dto.BrowseItemDTO, 0, len(items))
	for _, item := range items {
		if item.Type == service.BrowseItemTypeStack && item.Stack != nil {
			cover := toBrowseMediaItemDTO(item.Stack.Cover)
			stackSize := len(item.Stack.Members)
			cover.Stack = &dto.StackPreviewDTO{
				StackID:    item.Stack.StackID.String(),
				StackKind:  string(item.Stack.Kind),
				StackCover: true,
				StackSize:  &stackSize,
			}
			dtos = append(dtos, dto.BrowseItemDTO{
				Type:     service.BrowseItemTypeStack,
				ID:       item.ID,
				BestTsMs: item.BestTsMs,
				Stack: &dto.BrowseStackDTO{
					StackID:        item.Stack.StackID.String(),
					StackKind:      string(item.Stack.Kind),
					Cover:          cover,
					Members:        toBrowseStackMemberDTOs(item.Stack.Members),
					MatchedMembers: toBrowseStackMemberDTOs(item.Stack.MatchedMembers),
				},
			})
			continue
		}

		if item.MediaItem == nil {
			continue
		}
		media := toBrowseMediaItemDTO(*item.MediaItem)
		dtos = append(dtos, dto.BrowseItemDTO{
			Type:      service.BrowseItemTypeMediaItem,
			ID:        item.ID,
			MediaItem: &media,
			BestTsMs:  item.BestTsMs,
		})
	}
	return dtos
}

func toQueryBrowseResponseDTO(result service.BrowseQueryResult, limit, offset int) dto.QueryAssetsResponseDTO {
	totalVisible := int(result.TotalVisible)
	totalMediaItems := int(result.TotalMediaItems)
	totalFiles := int(result.TotalFiles)
	itemDTOs := toBrowseItemDTOs(result.Items)
	return dto.QueryAssetsResponseDTO{
		Items:           itemDTOs,
		TotalVisible:    &totalVisible,
		TotalMediaItems: &totalMediaItems,
		TotalFiles:      &totalFiles,
		StackMode:       result.StackMode,
		Limit:           limit,
		Offset:          offset,
	}
}

func toSearchBrowseResponseDTO(result service.SearchBrowseResult, limit, offset int) dto.SearchAssetsResponseDTO {
	resultsTotalVisible := int(result.ResultsTotalVisible)
	resultsTotalMediaItems := int(result.ResultsTotalMediaItems)
	topItemDTOs := toBrowseItemDTOs(result.TopResults)
	resultItemDTOs := toBrowseItemDTOs(result.Results)
	return dto.SearchAssetsResponseDTO{
		TopItems: topItemDTOs,
		TopResultsMeta: dto.SearchTopResultsMetaDTO{
			Enabled:           result.TopResultsMeta.Enabled,
			Degraded:          result.TopResultsMeta.Degraded,
			Reason:            result.TopResultsMeta.Reason,
			SourceTypes:       append([]string{}, result.TopResultsMeta.SourceTypes...),
			CandidateCount:    result.TopResultsMeta.CandidateCount,
			CandidatePoolSize: result.TopResultsMeta.CandidatePoolSize,
			Sources:           toSearchSourceMetaDTOs(result.TopResultsMeta.Sources),
			Debug:             toSearchDebugItemDTOs(result.TopResultsMeta.Debug),
		},
		ResultItems:            resultItemDTOs,
		ResultsTotalVisible:    &resultsTotalVisible,
		ResultsTotalMediaItems: &resultsTotalMediaItems,
		Limit:                  limit,
		Offset:                 offset,
	}
}

func toSearchSourceMetaDTOs(sources []service.SearchSourceMeta) []dto.SearchSourceMetaDTO {
	items := make([]dto.SearchSourceMetaDTO, 0, len(sources))
	for _, source := range sources {
		items = append(items, dto.SearchSourceMetaDTO{
			Type:           source.Type,
			Weight:         source.Weight,
			CandidateCount: source.CandidateCount,
			DurationMs:     source.DurationMs,
			Error:          source.Error,
		})
	}
	return items
}

func toSearchDebugItemDTOs(debug []service.SearchDebugItem) []dto.SearchDebugItemDTO {
	items := make([]dto.SearchDebugItemDTO, 0, len(debug))
	for _, item := range debug {
		contributions := make(map[string]dto.SearchDebugContributionDTO, len(item.Contributions))
		for source, contribution := range item.Contributions {
			contributions[source] = dto.SearchDebugContributionDTO{
				Rank:     contribution.Rank,
				Weight:   contribution.Weight,
				RRFScore: contribution.RRFScore,
				RawScore: contribution.RawScore,
			}
		}
		items = append(items, dto.SearchDebugItemDTO{
			AssetID:       item.AssetID,
			Score:         item.Score,
			Contributions: contributions,
		})
	}
	return items
}

// QueryAssets handles unified asset listing, filtering, and searching
// @Summary Query assets (unified endpoint)
// @Description Unified endpoint for listing, filtering, and searching assets. Replaces separate /filter and /search endpoints.
// @Tags assets
// @Produce json
// @Param data body dto.AssetQueryRequestDTO true "Query parameters"
// @Success 200 {object} dto.QueryAssetsResponseDTO "Assets queried successfully"
// @Failure 400 {object} api.ProblemResponse "Invalid request parameters"
// @Failure 503 {object} api.ProblemResponse "Image Semantic Analysis unavailable"
// @Failure 500 {object} api.ProblemResponse "Internal server error"
// @Router /api/v1/assets/list [post]
func (h *AssetHandler) QueryAssets(c *gin.Context) {
	var req dto.AssetQueryRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	normalizeAssetQueryPagination(&req.Pagination)

	if err := validateAssetQuerySearchType(req.SearchType); err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}
	if err := validateAssetQuerySortBy(req.SortBy); err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}
	if err := validateStackMode(req.StackMode); err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	// Default to filename search if not specified
	if req.SearchType == "" {
		req.SearchType = "filename"
	}

	params, err := buildQueryAssetsParams(req.Query, req.SearchType, req.SortBy, req.ViewerTimezone, req.StackMode, req.Filter, req.Pagination)
	if err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}
	params = applyAssetOwnershipScope(c, params)

	browseResult, err := h.assetService.QueryBrowseItems(c.Request.Context(), params)
	if err != nil {
		if errors.Is(err, service.ErrInvalidBrowseFilter) {
			api.WriteProblem(c, api.BadRequest(err))
			return
		}
		// Check for semantic search unavailable error
		if errors.Is(err, service.ErrSemanticSearchUnavailable) {
			api.WriteProblem(c, api.StatusProblem(503, err))
			return
		}
		log.Printf("Failed to query assets: %v", err)
		api.WriteProblem(c, api.Internal(err))
		return
	}

	response := toQueryBrowseResponseDTO(
		browseResult,
		req.Pagination.Limit,
		req.Pagination.Offset,
	)
	api.JSONOK(c, response)
}

// SearchAssets handles sectioned asset search with best-effort top results.
// @Summary Search assets
// @Description Search assets with optional top results enhancement, filename fallback, or visual similarity to a catalog asset.
// @Tags assets
// @Produce json
// @Param data body dto.SearchAssetsRequestDTO true "Search parameters"
// @Success 200 {object} dto.SearchAssetsResponseDTO "Assets searched successfully"
// @Failure 400 {object} api.ProblemResponse "Invalid request parameters"
// @Failure 404 {object} api.ProblemResponse "Query asset not found"
// @Failure 409 {object} api.ProblemResponse "Query asset has no Image Semantic Analysis embedding"
// @Failure 503 {object} api.ProblemResponse "Image Semantic Analysis unavailable"
// @Failure 500 {object} api.ProblemResponse "Internal server error"
// @Router /api/v1/assets/search [post]
func (h *AssetHandler) SearchAssets(c *gin.Context) {
	var req dto.SearchAssetsRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	normalizeAssetQueryPagination(&req.Pagination)
	if err := validateAssetQuerySortBy(req.SortBy); err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}
	if err := rejectSearchStackMode(req.StackMode); err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	if err := validateSearchEnhancementMode(req.EnhancementMode); err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}
	if strings.TrimSpace(req.EnhancementMode) == "" {
		req.EnhancementMode = string(service.SearchEnhancementModeAuto)
	}

	similarID, err := parseSimilarToAssetID(req.SimilarToAssetID)
	if err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}
	query := strings.TrimSpace(req.Query)
	if similarID != nil && query != "" {
		api.WriteProblem(c, api.BadRequest(errors.New("query and similar_to_asset_id are mutually exclusive")))
		return
	}
	if similarID != nil {
		if _, ok := h.loadVisibleSearchQueryAsset(c, *similarID); !ok {
			return
		}
	}

	params, err := buildQueryAssetsParams(query, "filename", req.SortBy, req.ViewerTimezone, "", req.Filter, req.Pagination)
	if err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}
	params = applyAssetOwnershipScope(c, params)

	result, err := h.assetService.SearchBrowseItems(c.Request.Context(), service.SearchAssetsParams{
		QueryAssetsParams: params,
		EnhancementMode:   service.SearchEnhancementMode(req.EnhancementMode),
		TopResultsLimit:   req.TopResultsLimit,
		Debug:             req.Debug,
		SimilarToAssetID:  similarID,
	})
	if err != nil {
		if !h.respondVisualSearchError(c, err) {
			return
		}
		if errors.Is(err, service.ErrInvalidBrowseFilter) {
			api.WriteProblem(c, api.BadRequest(err))
			return
		}
		log.Printf("Failed to search assets: %v", err)
		api.WriteProblem(c, api.Internal(err))
		return
	}

	searchResponse := toSearchBrowseResponseDTO(result, req.Pagination.Limit, req.Pagination.Offset)

	api.JSONOK(c, searchResponse)
}

const (
	maxImageSearchUploadBytes         = 256 << 20
	imageSearchMultipartMemoryBytes   = 32 << 20
	imageSearchMultipartOverheadBytes = 8 << 20
)

// SearchAssetsByImage searches the catalog by a live-embedded local image file.
// @Summary Search assets by image
// @Description Embed an uploaded image with Image Semantic Analysis and return visually similar catalog media. The original is reduced to an in-memory medium thumbnail, then discarded; it is not stored. RAW uses the same OpenPhoto path as ingest. Maximum upload size is 256 MiB.
// @Tags assets
// @Accept multipart/form-data
// @Produce json
// @Param file formData file true "Query image"
// @Param filter formData string false "JSON AssetFilterDTO"
// @Param limit formData int false "Page size"
// @Param offset formData int false "Page offset"
// @Param top_results_limit formData int false "KNN cap, maximum 200"
// @Param viewer_timezone formData string false "Viewer timezone"
// @Success 200 {object} dto.SearchAssetsResponseDTO "Assets searched successfully"
// @Failure 400 {object} api.ProblemResponse "Invalid request parameters"
// @Failure 503 {object} api.ProblemResponse "Image Semantic Analysis unavailable"
// @Failure 500 {object} api.ProblemResponse "Internal server error"
// @Router /api/v1/assets/search/by-image [post]
func (h *AssetHandler) SearchAssetsByImage(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxImageSearchUploadBytes+imageSearchMultipartOverheadBytes)
	if err := c.Request.ParseMultipartForm(imageSearchMultipartMemoryBytes); err != nil {
		if isImageSearchTooLarge(err) {
			api.WriteProblem(c, api.BadRequest(service.ErrAssetFileTooLarge))
			return
		}
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}
	defer file.Close()
	if header.Size > maxImageSearchUploadBytes {
		api.WriteProblem(c, api.BadRequest(service.ErrAssetFileTooLarge))
		return
	}

	filename := filepath.Base(filepath.ToSlash(header.Filename))
	if filename == "." || filename == string(filepath.Separator) {
		filename = ""
	}
	queryPath, queryBytes, err := readImageSearchUpload(file, header)
	if err != nil {
		if isImageSearchTooLarge(err) {
			api.WriteProblem(c, api.BadRequest(service.ErrAssetFileTooLarge))
			return
		}
		api.WriteProblem(c, api.BadRequest(err))
		return
	}
	if queryPath == "" && len(queryBytes) == 0 {
		api.WriteProblem(c, api.BadRequest(errors.New("empty image file")))
		return
	}

	filter, err := parseImageSearchFilter(c.PostForm("filter"))
	if err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}
	pagination := dto.PaginationDTO{
		Limit:  formIntDefault(c.PostForm("limit"), 20),
		Offset: formIntDefault(c.PostForm("offset"), 0),
	}
	normalizeAssetQueryPagination(&pagination)
	topResultsLimit := formIntDefault(c.PostForm("top_results_limit"), 0)

	params, err := buildQueryAssetsParams("", "filename", "", c.PostForm("viewer_timezone"), "", filter, pagination)
	if err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}
	params = applyAssetOwnershipScope(c, params)

	result, err := h.assetService.SearchBrowseItems(c.Request.Context(), service.SearchAssetsParams{
		QueryAssetsParams:  params,
		EnhancementMode:    service.SearchEnhancementModeOnly,
		TopResultsLimit:    topResultsLimit,
		QueryImage:         queryBytes,
		QueryImagePath:     queryPath,
		QueryImageFilename: filename,
	})
	if err != nil {
		if !h.respondVisualSearchError(c, err) {
			return
		}
		if errors.Is(err, service.ErrInvalidBrowseFilter) {
			api.WriteProblem(c, api.BadRequest(err))
			return
		}
		log.Printf("Failed to search assets by image: %v", err)
		api.WriteProblem(c, api.Internal(err))
		return
	}

	api.JSONOK(c, toSearchBrowseResponseDTO(result, pagination.Limit, pagination.Offset))
}

func readImageSearchUpload(file multipart.File, header *multipart.FileHeader) (string, []byte, error) {
	if osFile, ok := file.(*os.File); ok {
		if header.Size == 0 {
			info, err := osFile.Stat()
			if err != nil {
				return "", nil, err
			}
			if info.Size() == 0 {
				return "", nil, nil
			}
			if info.Size() > maxImageSearchUploadBytes {
				return "", nil, &http.MaxBytesError{Limit: maxImageSearchUploadBytes}
			}
		}
		return osFile.Name(), nil, nil
	}

	imageBytes, err := io.ReadAll(io.LimitReader(file, maxImageSearchUploadBytes+1))
	if err != nil {
		return "", nil, err
	}
	if int64(len(imageBytes)) > maxImageSearchUploadBytes {
		return "", nil, &http.MaxBytesError{Limit: maxImageSearchUploadBytes}
	}
	return "", imageBytes, nil
}

func isImageSearchTooLarge(err error) bool {
	var maxBytes *http.MaxBytesError
	return errors.As(err, &maxBytes)
}

func parseSimilarToAssetID(raw *string) (*uuid.UUID, error) {
	if raw == nil {
		return nil, nil
	}
	value := strings.TrimSpace(*raw)
	if value == "" {
		return nil, nil
	}
	id, err := uuid.Parse(value)
	if err != nil {
		return nil, err
	}
	return &id, nil
}

func (h *AssetHandler) loadVisibleSearchQueryAsset(c *gin.Context, assetID uuid.UUID) (*repo.Asset, bool) {
	asset, err := h.assetService.GetAssetAny(c.Request.Context(), assetID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			api.WriteProblem(c, api.NotFound(err))
			return nil, false
		}
		api.WriteProblem(c, api.Internal(err))
		return nil, false
	}

	user, ok := currentUserFromContext(c)
	if !ok {
		if asset.OwnerID != nil {
			api.WriteProblem(c, api.NotFound(errors.New("asset not found")))
			return nil, false
		}
		return asset, true
	}
	if service.IsAdminRole(user.Role) || asset.OwnerID == nil || int32(user.UserID) == *asset.OwnerID {
		return asset, true
	}
	api.WriteProblem(c, api.NotFound(errors.New("asset not found")))
	return nil, false
}

func (h *AssetHandler) respondVisualSearchError(c *gin.Context, err error) bool {
	switch {
	case errors.Is(err, service.ErrEmbeddingMissing):
		api.WriteProblem(c, api.KnownProblem(problem.ImageEmbeddingMissing, err))
		return false
	case errors.Is(err, service.ErrSemanticSearchUnavailable):
		api.WriteProblem(c, api.KnownProblem(problem.SemanticAnalysisUnavailable, err))
		return false
	case errors.Is(err, service.ErrInvalidImageQuery):
		api.WriteProblem(c, api.BadRequest(err))
		return false
	default:
		return true
	}
}

func parseImageSearchFilter(raw string) (dto.AssetFilterDTO, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return dto.AssetFilterDTO{}, nil
	}
	var filter dto.AssetFilterDTO
	if err := json.Unmarshal([]byte(raw), &filter); err != nil {
		return dto.AssetFilterDTO{}, err
	}
	return filter, nil
}

func formIntDefault(raw string, fallback int) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}

// ListIndexingRepositories returns repository options for scope selectors
// (browse scope, upload target) and indexing filters. All authenticated users
// may read the shared registry; filesystem paths are admin-only.
// @Summary List repositories for scope selection
// @Description Return the shared repository registry for browse-scope/upload selectors and indexing filters. Paths are only included for admins.
// @Tags assets
// @Accept json
// @Produce json
// @Success 200 {object} dto.IndexingRepositoryListResponseDTO "Repository options retrieved successfully"
// @Failure 500 {object} api.ProblemResponse "Internal server error"
// @Router /api/v1/assets/indexing/repositories [get]
func (h *AssetHandler) ListIndexingRepositories(c *gin.Context) {
	repositories, err := h.repoManager.ListRepositories()
	if err != nil {
		log.Printf("Failed to list repositories for indexing: %v", err)
		api.WriteProblem(c, api.Internal(err))
		return
	}

	isAdmin := ownerScopeID(c) == nil
	api.JSONOK(c, toIndexingRepositoryListResponseDTO(repositories, isAdmin))
}

// GetIndexingStats returns indexing coverage and queue status for photo AI tasks.
// @Summary Get asset indexing stats
// @Description Return indexing coverage and queued job counts for photo AI tasks.
// @Tags assets
// @Accept json
// @Produce json
// @Param repository_id query string false "Optional repository UUID filter"
// @Success 200 {object} dto.AssetIndexingStatsResponseDTO "Indexing stats retrieved successfully"
// @Failure 400 {object} api.ProblemResponse "Invalid repository ID"
// @Failure 500 {object} api.ProblemResponse "Internal server error"
// @Router /api/v1/assets/indexing/stats [get]
func (h *AssetHandler) GetIndexingStats(c *gin.Context) {
	repositoryID := strings.TrimSpace(c.Query("repository_id"))
	var repositoryIDPtr *string
	if repositoryID != "" {
		if _, err := uuid.Parse(repositoryID); err != nil {
			api.WriteProblem(c, api.BadRequest(err))
			return
		}
		repositoryIDPtr = &repositoryID
	}

	stats, err := h.indexingService.GetIndexingStats(c.Request.Context(), repositoryIDPtr)
	if err != nil {
		log.Printf("Failed to load indexing stats: %v", err)
		api.WriteProblem(c, api.Internal(err))
		return
	}

	api.JSONOK(c, toIndexingStatsResponseDTO(stats))
}

// RebuildAssetIndexes queues a background indexing backfill batch for existing photos.
// @Summary Queue asset index rebuild
// @Description Queue a background batch that backfills AI indexing for existing photos.
// @Tags assets
// @Produce json
// @Param data body dto.RebuildAssetIndexesRequestDTO false "Reindex request"
// @Success 200 {object} dto.RebuildAssetIndexesResponseDTO "Reindex job queued successfully"
// @Failure 400 {object} api.ProblemResponse "Invalid request parameters"
// @Failure 500 {object} api.ProblemResponse "Internal server error"
// @Router /api/v1/assets/indexing/rebuild [post]
func (h *AssetHandler) RebuildAssetIndexes(c *gin.Context) {
	var req dto.RebuildAssetIndexesRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil && err != io.EOF {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	tasks, err := parseIndexingTasks(req.Tasks)
	if err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	var repositoryIDPtr *string
	if trimmedRepositoryID := strings.TrimSpace(req.RepositoryID); trimmedRepositoryID != "" {
		repositoryIDPtr = &trimmedRepositoryID
	}

	missingOnly := true
	if req.MissingOnly != nil {
		missingOnly = *req.MissingOnly
	}

	resetSemantic := false
	if req.ResetSemantic != nil {
		resetSemantic = *req.ResetSemantic
	}

	result, err := h.indexingService.EnqueueReindexAssets(c.Request.Context(), service.ReindexAssetsInput{
		RepositoryID:  repositoryIDPtr,
		Tasks:         tasks,
		Limit:         normalizeRebuildIndexLimit(req.Limit),
		MissingOnly:   missingOnly,
		ResetSemantic: resetSemantic,
	})
	if err != nil {
		log.Printf("Failed to queue reindex job: %v", err)
		if errors.Is(err, service.ErrSemanticResetRequiresGlobalScope) {
			api.WriteProblem(c, api.BadRequest(err))
			return
		}
		api.WriteProblem(c, api.Internal(err))
		return
	}

	requestedTasks := make([]string, 0, len(result.Requested))
	for _, task := range result.Requested {
		requestedTasks = append(requestedTasks, string(task))
	}

	disabledTasks := make([]string, 0, len(result.Disabled))
	for _, task := range result.Disabled {
		disabledTasks = append(disabledTasks, string(task))
	}

	status := "queued"
	message := "Index rebuild job queued successfully"
	if result.ReceiptID == uuid.Nil && len(result.Requested) == 0 {
		status = "skipped"
		message = "All requested indexing tasks are disabled in ML settings"
	}
	receiptID := ""
	if result.ReceiptID != uuid.Nil {
		receiptID = result.ReceiptID.String()
	}

	api.JSONOK(c, dto.RebuildAssetIndexesResponseDTO{
		Status:         status,
		Message:        message,
		ReceiptID:      receiptID,
		RequestedTasks: requestedTasks,
		DisabledTasks:  disabledTasks,
		Limit:          result.Limit,
		MissingOnly:    result.MissingOnly,
		RepositoryID:   result.RepositoryID,
	})
}

// GetFeaturedAssets returns deterministic curated featured photos.
// @Summary Get featured photos
// @Description Select a small set of featured photos using deterministic weighted sampling (A-ES) with diversity constraints.
// @Tags assets
// @Accept json
// @Produce json
// @Param count query int false "Number of featured photos to return" default(8)
// @Param candidate_limit query int false "Max candidate photos considered before selection" default(240)
// @Param days query int false "Only consider photos from the last N days (0 disables date cutoff)" default(3650)
// @Param seed query string false "Deterministic seed (default: current UTC date YYYY-MM-DD)"
// @Param repository_id query string false "Optional repository UUID filter"
// @Success 200 {object} dto.FeaturedAssetsResponseDTO "Featured photos selected successfully"
// @Failure 400 {object} api.ProblemResponse "Invalid request parameters"
// @Failure 500 {object} api.ProblemResponse "Internal server error"
// @Router /api/v1/assets/featured [get]
func (h *AssetHandler) GetFeaturedAssets(c *gin.Context) {
	count, err := parseIntQueryWithRange(c, "count", 8, 1, 24)
	if err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	candidateLimit, err := parseIntQueryWithRange(c, "candidate_limit", 240, 16, 1000)
	if err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	days, err := parseIntQueryWithRange(c, "days", 3650, 0, 36500)
	if err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	seed := strings.TrimSpace(c.Query("seed"))
	now := time.Now().UTC()
	if seed == "" {
		seed = now.Format("2006-01-02")
	}

	var repositoryID *string
	if rawRepoID := strings.TrimSpace(c.Query("repository_id")); rawRepoID != "" {
		if _, err := uuid.Parse(rawRepoID); err != nil {
			api.WriteProblem(c, api.BadRequest(err))
			return
		}
		repositoryID = &rawRepoID
	}

	var dateFrom *time.Time
	if days > 0 {
		from := now.AddDate(0, 0, -days)
		dateFrom = &from
	}

	photoType := service.AssetTypePhoto
	params := service.QueryAssetsParams{
		SearchType:   "filename",
		RepositoryID: repositoryID,
		AssetType:    &photoType,
		DateFrom:     dateFrom,
		Limit:        candidateLimit,
		Offset:       0,
	}
	params = applyAssetOwnershipScope(c, params)

	assets, _, err := h.assetService.QueryAssets(c.Request.Context(), params)
	if err != nil {
		log.Printf("Failed to query featured candidate assets: %v", err)
		api.WriteProblem(c, api.Internal(err))
		return
	}

	selected := service.SelectFeaturedPhotos(assets, service.FeaturedSelectionOptions{
		Count: count,
		Seed:  seed,
		Now:   now,
	})

	uniqueCandidates := countUniqueAssets(assets)

	dtos := make([]dto.AssetDTO, len(selected))
	for i, a := range selected {
		dtos[i] = dto.ToAssetDTO(a)
	}

	response := dto.FeaturedAssetsResponseDTO{
		Assets:          dtos,
		Count:           len(dtos),
		CandidateCount:  uniqueCandidates,
		Seed:            seed,
		Strategy:        "weighted_aes_v1",
		GeneratedAtTime: now,
	}
	api.JSONOK(c, response)
}

// GetPhotoMapPoints returns lightweight photo map points with valid GPS coordinates.
// @Summary Get photo map points
// @Description Return lightweight paginated photo records containing only map-related fields (asset ID, filename, times, GPS lat/lon).
// @Tags assets
// @Accept json
// @Produce json
// @Param limit query int false "Page size (1-5000)" default(1000)
// @Param offset query int false "Page offset" default(0)
// @Param repository_id query string false "Optional repository UUID filter"
// @Param south query number false "Viewport south latitude (-90 to 90)"
// @Param north query number false "Viewport north latitude (-90 to 90)"
// @Param west query number false "Viewport west longitude (-180 to 180)"
// @Param east query number false "Viewport east longitude (-180 to 180)"
// @Success 200 {object} dto.AssetMapPointListResponseDTO "Map points retrieved successfully"
// @Failure 400 {object} api.ProblemResponse "Invalid request parameters"
// @Failure 500 {object} api.ProblemResponse "Internal server error"
// @Router /api/v1/assets/map-points [get]
func (h *AssetHandler) GetPhotoMapPoints(c *gin.Context) {
	limit, err := parseIntQueryWithRange(c, "limit", 1000, 1, 5000)
	if err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	offset, err := parseIntQueryWithRange(c, "offset", 0, 0, 10000000)
	if err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	var repositoryID *string
	if rawRepoID := strings.TrimSpace(c.Query("repository_id")); rawRepoID != "" {
		if _, err := uuid.Parse(rawRepoID); err != nil {
			api.WriteProblem(c, api.BadRequest(err))
			return
		}
		repositoryID = &rawRepoID
	}

	south, north, west, east, err := parseOptionalMapViewport(c)
	if err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	points, total, err := h.assetService.QueryPhotoMapPoints(c.Request.Context(), applyMapPointOwnershipScope(c, service.QueryPhotoMapPointsParams{
		RepositoryID: repositoryID,
		South:        south,
		North:        north,
		West:         west,
		East:         east,
		Limit:        limit,
		Offset:       offset,
	}))
	if err != nil {
		log.Printf("Failed to query photo map points: %v", err)
		api.WriteProblem(c, api.Internal(err))
		return
	}

	pointDTOs := make([]dto.AssetMapPointDTO, len(points))
	for i, point := range points {
		pointDTOs[i] = dto.AssetMapPointDTO{
			AssetID:          point.AssetID,
			OriginalFilename: point.OriginalFilename,
			UploadTime:       point.UploadTime,
			TakenTime:        point.TakenTime,
			GPSLatitude:      point.GPSLatitude,
			GPSLongitude:     point.GPSLongitude,
		}
	}

	totalInt := int(total)
	response := dto.AssetMapPointListResponseDTO{
		Points: pointDTOs,
		Total:  &totalInt,
		Limit:  limit,
		Offset: offset,
	}
	api.JSONOK(c, response)
}

func parseOptionalMapViewport(c *gin.Context) (*float64, *float64, *float64, *float64, error) {
	names := []string{"south", "north", "west", "east"}
	values := make([]*float64, len(names))
	present := 0
	for index, name := range names {
		raw, exists := c.GetQuery(name)
		if !exists || strings.TrimSpace(raw) == "" {
			continue
		}
		value, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return nil, nil, nil, nil, fmt.Errorf("parse %s: %w", name, err)
		}
		values[index] = &value
		present++
	}
	if present == 0 {
		return nil, nil, nil, nil, nil
	}
	if present != len(names) {
		return nil, nil, nil, nil, errors.New("south, north, west, and east must be provided together")
	}
	south, north, west, east := *values[0], *values[1], *values[2], *values[3]
	if south < -90 || south > 90 || north < -90 || north > 90 || south > north {
		return nil, nil, nil, nil, errors.New("latitude bounds must satisfy -90 <= south <= north <= 90")
	}
	if west < -180 || west > 180 || east < -180 || east > 180 {
		return nil, nil, nil, nil, errors.New("longitude bounds must be between -180 and 180")
	}
	return values[0], values[1], values[2], values[3], nil
}

func parseIntQueryWithRange(
	c *gin.Context,
	name string,
	defaultValue int,
	minValue int,
	maxValue int,
) (int, error) {
	raw := strings.TrimSpace(c.Query(name))
	if raw == "" {
		return defaultValue, nil
	}

	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, err
	}
	if value < minValue || value > maxValue {
		return 0, fmt.Errorf("%s must be between %d and %d", name, minValue, maxValue)
	}
	return value, nil
}

func countUniqueAssets(assets []repo.Asset) int {
	seen := make(map[string]struct{}, len(assets))
	for _, asset := range assets {
		if asset.AssetID == uuid.Nil {
			continue
		}
		seen[asset.AssetID.String()] = struct{}{}
	}
	return len(seen)
}

// GetFilterOptions returns available options for filters
// @Summary Get filter options
// @Description Get available camera models and lenses for filter dropdowns
// @Tags assets
// @Accept json
// @Produce json
// @Success 200 {object} dto.OptionsResponseDTO "Filter options retrieved successfully"
// @Failure 500 {object} api.ProblemResponse "Internal server error"
// @Router /api/v1/assets/filter-options [get]
func (h *AssetHandler) GetFilterOptions(c *gin.Context) {
	ctx := c.Request.Context()

	cameraModels, err := h.assetService.GetDistinctCameraModels(ctx)
	if err != nil {
		log.Printf("Failed to get camera models: %v", err)
		api.WriteProblem(c, api.Internal(err))
		return
	}

	lenses, err := h.assetService.GetDistinctLenses(ctx)
	if err != nil {
		log.Printf("Failed to get lenses: %v", err)
		api.WriteProblem(c, api.Internal(err))
		return
	}

	response := dto.OptionsResponseDTO{
		CameraModels: cameraModels,
		Lenses:       lenses,
	}
	api.JSONOK(c, response)
}

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

// GetAssetsByRating gets assets filtered by rating
// @Summary Get assets by rating
// @Description Get assets with a specific rating (0-5)
// @Tags assets
// @Accept json
// @Produce json
// @Param rating path int true "Rating (0-5)"
// @Param limit query int false "Number of assets to return" default(20)
// @Param offset query int false "Number of assets to skip" default(0)
// @Success 200 {object} dto.AssetListResponseDTO "Assets retrieved successfully"
// @Failure 400 {object} api.ProblemResponse "Bad request"
// @Failure 500 {object} api.ProblemResponse "Internal server error"
// @Router /api/v1/assets/rating/{rating} [get]
func (h *AssetHandler) GetAssetsByRating(c *gin.Context) {
	ratingStr := c.Param("rating")
	rating, err := strconv.Atoi(ratingStr)
	if err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	if rating < 0 || rating > 5 {
		api.WriteProblem(c, api.BadRequest(nil))
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

	user, ok := requireCurrentUser(c)
	if !ok {
		return
	}
	var ownerID *int32
	if !service.IsAdminRole(user.Role) {
		id := int32(user.UserID)
		ownerID = &id
	}

	assets, err := h.assetService.GetAssetsByRating(c.Request.Context(), rating, ownerID, limit, offset)
	if err != nil {
		log.Printf("Failed to get assets by rating: %v", err)
		api.WriteProblem(c, api.Internal(err))
		return
	}

	assetDTOs := make([]dto.AssetDTO, len(assets))
	for i, asset := range assets {
		assetDTOs[i] = dto.ToAssetDTO(asset)
	}

	response := dto.AssetListResponseDTO{
		Assets: assetDTOs,
		Limit:  limit,
		Offset: offset,
	}

	api.JSONOK(c, response)
}

// GetLikedAssets gets all liked/favorited assets
// @Summary Get liked assets
// @Description Get all assets that have been liked/favorited
// @Tags assets
// @Accept json
// @Produce json
// @Param limit query int false "Number of assets to return" default(20)
// @Param offset query int false "Number of assets to skip" default(0)
// @Success 200 {object} dto.AssetListResponseDTO "Liked assets retrieved successfully"
// @Failure 500 {object} api.ProblemResponse "Internal server error"
// @Router /api/v1/assets/liked [get]
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

	user, ok := requireCurrentUser(c)
	if !ok {
		return
	}
	var ownerID *int32
	if !service.IsAdminRole(user.Role) {
		id := int32(user.UserID)
		ownerID = &id
	}

	assets, err := h.assetService.GetLikedAssets(ctx, ownerID, limit, offset)
	if err != nil {
		log.Printf("Failed to get liked assets: %v", err)
		api.WriteProblem(c, api.Internal(err))
		return
	}

	assetDTOs := make([]dto.AssetDTO, len(assets))
	for i, asset := range assets {
		assetDTOs[i] = dto.ToAssetDTO(asset)
	}

	response := dto.AssetListResponseDTO{
		Assets: assetDTOs,
		Limit:  limit,
		Offset: offset,
	}

	api.JSONOK(c, response)
}

// Helper methods for unified chunk upload

// cleanupExpiredSessions periodically cleans up expired upload sessions
func (h *AssetHandler) cleanupExpiredSessions() {
	expiredCount := h.sessionManager.CleanupExpiredSessions()
	if expiredCount > 0 {
		log.Printf("Cleaned up %d expired upload sessions", expiredCount)
	}
}

// StartCleanupTasks starts background cleanup goroutines that respect ctx
// cancellation for graceful shutdown. Call from app.go after construction.
func (h *AssetHandler) StartCleanupTasks(ctx context.Context) {
	h.cleanupExpiredSessions()
	h.cleanupOrphanedChunks()

	go func() {
		sessionTicker := time.NewTicker(5 * time.Minute)
		defer sessionTicker.Stop()
		orphanedChunkTicker := time.NewTicker(30 * time.Minute)
		defer orphanedChunkTicker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-sessionTicker.C:
				h.cleanupExpiredSessions()
			case <-orphanedChunkTicker.C:
				h.cleanupOrphanedChunks()
			}
		}
	}()
}

// cleanupOrphanedChunks removes orphaned chunk files that aren't associated with any active session
func (h *AssetHandler) cleanupOrphanedChunks() {
	log.Println("🔍 Starting orphaned chunk cleanup...")

	// Get all active session IDs
	activeSessions := h.sessionManager.GetAllSessions()
	activeSessionIDs := make(map[string]bool)
	for _, session := range activeSessions {
		activeSessionIDs[session.SessionID] = true
	}

	// Track stats
	errorCount := 0

	// Get all repository IDs that have active or recent upload activity
	repoIDs := make(map[string]bool)
	for _, session := range activeSessions {
		if session.RepositoryID != "" {
			repoIDs[session.RepositoryID] = true
		}
	}

	// Convert map to slice
	var repositoryIDs []string
	for id := range repoIDs {
		repositoryIDs = append(repositoryIDs, id)
	}

	// If there are no active repositories with sessions, we'll do a general cleanup
	// of all known repositories
	if len(repositoryIDs) == 0 {
		// Get all repositories using ListRepositories
		repositories, err := h.repoManager.ListRepositories()
		if err != nil {
			log.Printf("❌ Failed to list repositories for orphaned chunk cleanup: %v", err)
		} else {
			for _, repo := range repositories {
				// An offline repository has no reachable staging directory;
				// walking it only produces I/O errors and false failure counts.
				if repo.Reachability != dbtypes.RepositoryReachabilityActive {
					continue
				}
				// Use staging manager's cleanup function with short max age (1 hour)
				err := h.stagingManager.CleanupStaging(*repo, time.Hour)
				if err != nil {
					log.Printf("❌ Failed to cleanup staging for repository %s: %v", repo.Name, err)
					errorCount++
				} else {
					log.Printf("✅ Cleaned up staging for repository %s", repo.Name)
				}
			}
		}
	} else {
		// Cleanup for specific repositories with active sessions
		for _, repositoryID := range repositoryIDs {
			repo, err := h.repoManager.GetRepository(repositoryID)
			if err != nil {
				log.Printf("❌ Failed to get repository %s: %v", repositoryID, err)
				errorCount++
				continue
			}

			// Use staging manager's cleanup function with short max age (1 hour)
			err = h.stagingManager.CleanupStaging(*repo, time.Hour)
			if err != nil {
				log.Printf("❌ Failed to cleanup staging for repository %s: %v", repo.Name, err)
				errorCount++
			} else {
				log.Printf("✅ Cleaned up staging for repository %s", repo.Name)
			}
		}
	}

	log.Printf("✅ Orphaned chunk cleanup completed: %d errors", errorCount)
}

func (h *AssetHandler) sidecarSourceForAsset(ctx context.Context, asset *repo.Asset, projectedPath string) (dto.LumilioSidecarSourceDTO, error) {
	source := dto.LumilioSidecarSourceDTO{}
	if asset == nil {
		return source, fmt.Errorf("asset is nil")
	}
	content, err := h.queries.GetContentObjectByID(ctx, asset.ContentID)
	if err != nil {
		return source, err
	}
	source.OriginalFilename = asset.OriginalFilename
	source.MimeType = asset.MimeType
	source.FileSize = content.FileSize
	source.Hash = stringPtr(content.FullHash)
	if asset.Width != nil {
		width := int32(*asset.Width)
		source.Width = &width
	}
	if asset.Height != nil {
		height := int32(*asset.Height)
		source.Height = &height
	}
	source.StoragePath = projectedPath
	return source, nil
}

func (h *AssetHandler) defaultSidecarForAsset(assetID uuid.UUID, source dto.LumilioSidecarSourceDTO) dto.LumilioSidecarV1DTO {
	return dto.LumilioSidecarV1DTO{
		Version:     1,
		AssetID:     assetID.String(),
		Source:      source,
		Adjustments: dto.StudioEditAdjustmentsDTO{},
		UpdatedAt:   time.Now().UTC(),
	}
}

func (h *AssetHandler) handleUploadFailureFile(repository repo.Repository, stagingFile *storage.StagingFile, reason string) {
	if stagingFile == nil || strings.TrimSpace(stagingFile.PrivatePath) == "" {
		return
	}
	if err := h.stagingManager.MoveStagingToFailed(repository, stagingFile); err != nil {
		log.Printf("Failed to quarantine upload file %s (%s): %v", stagingFile.PrivatePath, reason, err)
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

// processCompletedUpload processes a completed upload (single file or merged chunks)
func (h *AssetHandler) processCompletedUpload(ctx context.Context, header *multipart.FileHeader, session *upload.UploadSession, repository repo.Repository, stagingFile *storage.StagingFile) (*dto.BatchUploadResultDTO, error) {
	if stagingFile == nil {
		return nil, errors.New("completed upload has no staging file")
	}
	opened, err := h.stagingManager.OpenStagingFile(repository, stagingFile)
	if err != nil {
		return nil, err
	}
	info, err := opened.Stat()
	if err != nil {
		_ = opened.Close()
		return nil, err
	}
	hashResult, err := hash.CalculateLayeredBLAKE3Reader(opened, info.Size())
	err = errors.Join(err, opened.Close())
	if err != nil {
		h.handleUploadFailureFile(repository, stagingFile, "calculate completed upload hash")
		return nil, fmt.Errorf("failed to calculate file hash: %w", err)
	}
	finalHash := hashResult.ContentHash

	validationResult := filevalidator.ValidateFile(header.Filename, session.ContentType)
	if !validationResult.Valid {
		h.handleUploadFailureFile(repository, stagingFile, "validate completed upload")
		return nil, fmt.Errorf("unsupported file type: %s", validationResult.ErrorReason)
	}
	finalContentType := validationResult.MimeType

	ownerID, err := h.resolveUploadOwnerID(ctx, session.UserID)
	if err != nil {
		h.handleUploadFailureFile(repository, stagingFile, "resolve completed upload owner")
		return nil, err
	}
	receiptID, err := h.enqueueStagingCommit(ctx, repository, ownerID, stagingFile,
		session.Filename, finalContentType, hashResult)
	if err != nil {
		h.handleUploadFailureFile(repository, stagingFile, "enqueue ingest task")
		return nil, fmt.Errorf("failed to enqueue task: %w", err)
	}

	status := "processing"
	size := hashResult.FileSize
	message := fmt.Sprintf("File uploaded with verified content hash and queued for processing in repository '%s'", repository.Name)

	return &dto.BatchUploadResultDTO{
		Success:     true,
		SessionID:   session.SessionID,
		FileName:    header.Filename,
		ContentHash: finalHash,
		ReceiptID:   stringPtr(receiptID.String()),
		Status:      &status,
		Size:        &size,
		Message:     &message,
	}, nil
}

// ReprocessAsset requests a new fenced asset-pipeline generation.
// @Summary Reprocess asset
// @Description Request catalog-owned analysis, derivative, transcode, and enrichment stages for an asset. Progress is reported from the receipt and desired/applied catalog state.
// @Tags assets
// @Produce json
// @Param id path string true "Asset ID"
// @Param data body dto.ReprocessAssetRequestDTO false "Reprocessing tasks (optional)"
// @Success 200 {object} dto.ReprocessAssetResponseDTO
// @Failure 400 {object} api.ProblemResponse
// @Failure 404 {object} api.ProblemResponse
// @Failure 500 {object} api.ProblemResponse
// @Router /api/v1/assets/{id}/reprocess [post]
func (h *AssetHandler) ReprocessAsset(c *gin.Context) {
	ctx := c.Request.Context()

	// Parse asset ID
	assetIDStr := c.Param("id")
	assetID, err := uuid.Parse(assetIDStr)
	if err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	// Parse request body
	var req dto.ReprocessAssetRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil && err != io.EOF {
		// Allow empty body
		req = dto.ReprocessAssetRequestDTO{}
	}

	// Validate requested product stages. Queue names are not API contracts.
	if len(req.Tasks) > 0 {
		for _, task := range req.Tasks {
			if !isValidReprocessStage(task) {
				api.WriteProblem(c, api.BadRequest(fmt.Errorf("invalid pipeline stage: %s", task)))
				return
			}
		}
	}

	// Read the immutable asset identity once. Reprocessing is a desired-state
	// mutation; product status is derived from the pipeline state and receipt,
	// not from a task map embedded in assets.status.
	asset, err := h.queries.GetAssetByID(ctx, assetID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			api.WriteProblem(c, api.NotFound(err))
			return
		}
		api.WriteProblem(c, api.Internal(err))
		return
	}

	if !ensureOwnerAccess(c, asset.OwnerID, "Authentication required to reprocess this asset", "You don't have permission to reprocess this asset") {
		return
	}

	opened, err := h.locationResolver.OpenAsset(ctx, asset.AssetID)
	if err != nil {
		respondRepositoryResolveError(c, err, "Asset has no available location")
		return
	}
	repositoryID := opened.Catalog.RepoID
	if err := opened.Close(); err != nil {
		api.WriteProblem(c, api.Internal(err))
		return
	}

	_, releaseWork, err := h.repoManager.BeginRepositoryWork(ctx, repositoryID.String(), dbtypes.RepositoryActivityProcessing)
	if err != nil {
		api.WriteProblem(c, api.StatusProblem(http.StatusConflict, err))
		return
	}
	released := false
	defer func() {
		if !released {
			_ = releaseWork()
		}
	}()
	tx, err := h.writer.BeginTx(ctx, catalogtx.OperationAssetReprocess, nil)
	if err != nil {
		api.WriteProblem(c, api.Internal(err))
		return
	}
	defer tx.Rollback()
	stages := requestedAssetStages(req.Tasks, dbtypes.AssetType(asset.Type), len(req.Tasks) == 0 || req.ForceFullRetry)
	receiptID := uuid.New()
	now := time.Now().UTC().UnixMicro()
	if _, err := tx.Raw().ExecContext(ctx, `INSERT INTO catalog_operation_receipts (receipt_id,kind,subject_id,desired_version,state,created_at,updated_at) VALUES (?,?,?,1,'pending',?,?)`, receiptID.String(), "reprocess", assetID.String(), now, now); err != nil {
		api.WriteProblem(c, api.Internal(err))
		return
	}
	if err := pipeline.RequestAssetStagesTx(ctx, tx.Raw(), asset.AssetID, asset.ContentID, stages, pipeline.AssetPipelineVersion, pipeline.AdmissionInteractive, receiptID); err != nil {
		api.WriteProblem(c, api.Internal(err))
		return
	}
	if err := tx.Commit(); err != nil {
		api.WriteProblem(c, api.Internal(err))
		return
	}
	if err := releaseWork(); err != nil {
		api.WriteProblem(c, api.Internal(err))
		return
	}
	released = true
	api.JSONOK(c, dto.ReprocessAssetResponseDTO{AssetID: assetID.String(), ReceiptID: receiptID.String(), Status: "queued", Message: "Reprocessing request accepted"})
}

func isValidReprocessStage(stage string) bool {
	switch pipeline.Stage(stage) {
	case pipeline.StageAnalyze, pipeline.StageDerivatives, pipeline.StageTranscode, pipeline.StageEnrich:
		return true
	default:
		return false
	}
}

func requestedAssetStages(requested []string, assetType dbtypes.AssetType, full bool) []pipeline.Stage {
	if !full {
		result := make([]pipeline.Stage, 0, len(requested))
		for _, stage := range requested {
			result = append(result, pipeline.Stage(stage))
		}
		return result
	}
	result := []pipeline.Stage{pipeline.StageAnalyze, pipeline.StageEnrich}
	if assetType == dbtypes.AssetTypePhoto {
		return append(result, pipeline.StageDerivatives)
	}
	if assetType == dbtypes.AssetTypeVideo {
		return append(result, pipeline.StageDerivatives, pipeline.StageTranscode)
	}
	if assetType == dbtypes.AssetTypeAudio {
		return append(result, pipeline.StageTranscode)
	}
	return result
}

// ============================================================================
// Stack operations
// ============================================================================

// GetAssetMediaItem returns the logical media item containing an asset.
// @Summary Get logical media item
// @Description Returns the logical media item and its RAW/JPEG, Live Photo, or edited components
// @Tags assets
// @Produce json
// @Param id path string true "Asset ID"
// @Success 200 {object} dto.MediaItemByAssetResponseDTO
// @Failure 404 {object} api.ProblemResponse
// @Router /api/v1/assets/{id}/media-item [get]
// @Security BearerAuth
func (h *AssetHandler) GetAssetMediaItem(c *gin.Context) {
	assetID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}
	if _, ok := h.getAuthorizedAssetForRead(c, assetID, "Authentication required to access this asset", "You don't have permission to access this asset"); !ok {
		return
	}
	item, err := h.stackService.GetMediaItemByAsset(c.Request.Context(), assetID, ownerScopeID(c))
	if err != nil {
		api.WriteProblem(c, api.NotFound(err))
		return
	}
	components := make([]dto.MediaItemComponentDTO, 0, len(item.Components))
	for _, component := range item.Components {
		components = append(components, dto.MediaItemComponentDTO{
			AssetID: component.AssetID.String(), Relation: string(component.Relation), Position: component.Position,
		})
	}
	api.JSONOK(c, dto.MediaItemByAssetResponseDTO{
		AssetID: assetID.String(),
		MediaItem: dto.MediaItemDTO{
			MediaItemID: item.MediaItemID.String(), MediaKind: item.Kind,
			PrimaryAssetID: item.PrimaryAssetID.String(), Components: components,
		},
	})
}

// GetAssetStack returns the stack that contains the given asset.
// @Summary Get asset stack
// @Description Returns the stack (group) that contains the specified asset
// @Tags assets
// @Produce json
// @Param id path string true "Asset ID"
// @Success 200 {object} dto.StackByAssetResponseDTO
// @Failure 404 {object} api.ProblemResponse
// @Router /api/v1/assets/{id}/stack [get]
// @Security BearerAuth
func (h *AssetHandler) GetAssetStack(c *gin.Context) {
	assetIDStr := c.Param("id")
	assetID, err := uuid.Parse(assetIDStr)
	if err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	if _, ok := h.getAuthorizedAssetForRead(c, assetID, "Authentication required to access this asset", "You don't have permission to access this asset"); !ok {
		return
	}

	stackInfo, err := h.stackService.GetStackByAssetAny(c.Request.Context(), assetID, ownerScopeID(c))
	if err != nil {
		if errors.Is(err, service.ErrStackNotFound) {
			api.WriteProblem(c, api.NotFound(err))
			return
		}
		api.WriteProblem(c, api.Internal(err))
		return
	}

	// Convert to DTO
	members := make([]dto.StackMemberDTO, len(stackInfo.Members))
	for i, m := range stackInfo.Members {
		members[i] = dto.StackMemberDTO{
			MediaItemID:    m.MediaItemID.String(),
			PrimaryAssetID: m.AssetID.String(),
			Position:       m.Position,
		}
	}

	response := dto.StackByAssetResponseDTO{
		AssetID: assetID.String(),
		Stack: dto.StackDTO{
			StackID:     stackInfo.StackID.String(),
			StackKind:   string(stackInfo.Kind),
			MemberCount: stackInfo.MemberCount,
			Members:     members,
		},
	}

	api.JSONOK(c, response)
}

// CreateManualStack manually groups assets into a stack.
// @Summary Create manual stack
// @Description Manually groups the specified assets into a new stack
// @Tags assets
// @Produce json
// @Param data body dto.CreateManualStackRequestDTO true "Asset IDs to stack"
// @Success 201 {object} dto.StackDTO
// @Failure 400 {object} api.ProblemResponse
// @Failure 409 {object} api.ProblemResponse
// @Router /api/v1/assets/stacks [post]
// @Security BearerAuth
func (h *AssetHandler) CreateManualStack(c *gin.Context) {
	var req dto.CreateManualStackRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	if len(req.AssetIDs) < 2 {
		api.WriteProblem(c, api.BadRequest(errors.New("at least 2 asset IDs are required")))
		return
	}

	assetIDs := make([]uuid.UUID, len(req.AssetIDs))
	for i, idStr := range req.AssetIDs {
		id, err := uuid.Parse(idStr)
		if err != nil {
			api.WriteProblem(c, api.BadRequest(err))
			return
		}
		assetIDs[i] = id
	}

	// Every asset in the stack must belong to the caller (or caller is admin).
	for _, id := range assetIDs {
		if _, ok := h.getAuthorizedAsset(c, id, "Authentication required to stack these assets", "You don't have permission to stack one or more of these assets"); !ok {
			return
		}
	}

	stackInfo, err := h.stackService.CreateManualStack(c.Request.Context(), assetIDs)
	if err != nil {
		if errors.Is(err, service.ErrAssetAlreadyStacked) {
			api.WriteProblem(c, api.StatusProblem(http.StatusConflict, err))
			return
		}
		api.WriteProblem(c, api.Internal(err))
		return
	}

	members := make([]dto.StackMemberDTO, len(stackInfo.Members))
	for i, m := range stackInfo.Members {
		members[i] = dto.StackMemberDTO{
			MediaItemID:    m.MediaItemID.String(),
			PrimaryAssetID: m.AssetID.String(),
			Position:       m.Position,
		}
	}

	response := dto.StackDTO{
		StackID:     stackInfo.StackID.String(),
		StackKind:   string(stackInfo.Kind),
		MemberCount: stackInfo.MemberCount,
		Members:     members,
	}

	c.JSON(http.StatusCreated, response)
}

// UnstackAsset removes an asset from its stack.
// @Summary Remove asset from stack
// @Description Removes an asset from its stack, making it standalone
// @Tags assets
// @Produce json
// @Param id path string true "Asset ID"
// @Success 200 {object} api.SuccessResponse
// @Router /api/v1/assets/{id}/stack [delete]
// @Security BearerAuth
func (h *AssetHandler) UnstackAsset(c *gin.Context) {
	assetIDStr := c.Param("id")
	assetID, err := uuid.Parse(assetIDStr)
	if err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	if _, ok := h.getAuthorizedAsset(c, assetID, "Authentication required to modify this asset", "You don't have permission to modify this asset"); !ok {
		return
	}

	if err := h.stackService.RemoveFromStack(c.Request.Context(), assetID); err != nil {
		api.WriteProblem(c, api.Internal(err))
		return
	}

	api.JSONOK(c, api.SuccessResponse{Message: "Asset removed from stack"})
}

// AutoDetectStacks merges structural media components and detects burst stacks for a repository.
// @Summary Auto-detect stacks
// @Description Merges RAW/JPEG and Live Photo components into logical media items, then detects burst presentation stacks
// @Tags repositories
// @Produce json
// @Param id path string true "Repository ID"
// @Success 200 {object} dto.AutoDetectStacksResponseDTO
// @Router /api/v1/repositories/{id}/stacks/detect [post]
// @Security BearerAuth
func (h *AssetHandler) AutoDetectStacks(c *gin.Context) {
	repoIDStr := c.Param("id")
	repoID, err := uuid.Parse(repoIDStr)
	if err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	count, err := h.stackService.AutoDetectStacks(c.Request.Context(), repoID)
	if err != nil {
		api.WriteProblem(c, api.Internal(err))
		return
	}

	api.JSONOK(c, dto.AutoDetectStacksResponseDTO{
		RepositoryID:  repoID.String(),
		StacksCreated: count,
	})
}
