package handler

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"server/internal/api"
	"server/internal/api/dto"
	"server/internal/db/dbtypes"
	"server/internal/db/dbtypes/status"
	"server/internal/db/repo"
	"server/internal/processors"
	"server/internal/queue/jobs"
	"server/internal/service"
	"server/internal/storage"
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
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
)

// uploadStatusDuplicate marks an upload the server skipped because identical
// content already exists in the target repository.
const uploadStatusDuplicate = "duplicate"

// AssetHandler handles HTTP requests for asset management
type AssetHandler struct {
	assetService    service.AssetService
	authService     *service.AuthService
	indexingService service.AssetIndexingService
	stackService    service.StackService
	queries         *repo.Queries
	repoManager     storage.RepositoryManager
	stagingManager  storage.StagingManager
	queueClient     *river.Client[pgx.Tx]
	settingsService service.SettingsService
	runtimeChecker  service.LumenService
	memoryMonitor   *memory.MemoryMonitor
	sessionManager  *upload.SessionManager
	chunkMerger     *upload.ChunkMerger
	uploadLimiter   chan struct{}
}

// NewAssetHandler creates a new AssetHandler instance
func NewAssetHandler(
	assetService service.AssetService,
	authService *service.AuthService,
	indexingService service.AssetIndexingService,
	stackService service.StackService,
	queries *repo.Queries,
	repoManager storage.RepositoryManager,
	stagingManager storage.StagingManager,
	queueClient *river.Client[pgx.Tx],
	settingsService service.SettingsService,
	runtimeChecker service.LumenService,
) *AssetHandler {
	memoryMonitor := memory.NewMemoryMonitor()
	sessionManager := upload.NewSessionManager(30 * time.Minute) // 30 minute timeout
	chunkMerger := upload.NewChunkMerger(storage.NewDirectoryManager())
	// Increased limit to 32 to support HTTP/2 multiplexing for chunked uploads
	uploadLimiter := make(chan struct{}, 32)

	handler := &AssetHandler{
		assetService:    assetService,
		authService:     authService,
		indexingService: indexingService,
		stackService:    stackService,
		queries:         queries,
		repoManager:     repoManager,
		stagingManager:  stagingManager,
		queueClient:     queueClient,
		settingsService: settingsService,
		runtimeChecker:  runtimeChecker,
		memoryMonitor:   memoryMonitor,
		sessionManager:  sessionManager,
		chunkMerger:     chunkMerger,
		uploadLimiter:   uploadLimiter,
	}

	return handler
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
		return repository, nil
	}

	repoUUID, err := uuid.Parse(repositoryID)
	if err != nil {
		return repo.Repository{}, errInvalidRepositoryID
	}
	repository, err := h.queries.GetRepository(ctx, pgtype.UUID{Bytes: repoUUID, Valid: true})
	if err != nil {
		return repo.Repository{}, errRepositoryNotFound
	}
	return repository, nil
}

// respondRepositoryError maps a resolveUploadRepository failure onto its HTTP response.
func (h *AssetHandler) respondRepositoryError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, errInvalidRepositoryID):
		api.GinBadRequest(c, err, "Invalid repository ID")
	case errors.Is(err, errRepositoryNotFound):
		api.GinNotFound(c, err, "Repository not found")
	default:
		api.GinBadRequest(c, err, "Please specify a repository_id or create a repository first")
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
// @Param X-Content-Hash header string false "Client-calculated BLAKE3 hash of the file"
// @Success 200 {object} dto.UploadResponseDTO "Upload successful"
// @Failure 400 {object} api.ErrorResponse "Bad request - no file provided or parse error"
// @Failure 500 {object} api.ErrorResponse "Internal server error"
// @Router /api/v1/assets [post]
func (h *AssetHandler) UploadAsset(c *gin.Context) {
	h.uploadLimiter <- struct{}{}
	defer func() { <-h.uploadLimiter }()

	ctx := c.Request.Context()

	var req dto.UploadAssetRequestDTO
	if err := c.ShouldBind(&req); err != nil {
		api.GinBadRequest(c, err, "Invalid request")
		return
	}

	err := c.Request.ParseMultipartForm(32 << 20)
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

	validationResult := filevalidator.ValidateFile(header.Filename, header.Header.Get("Content-Type"))
	if !validationResult.Valid {
		api.GinBadRequest(c, fmt.Errorf("unsupported file type: %s", validationResult.ErrorReason))
		return
	}
	log.Printf("Validated file %s as %s with canonical MIME %s (RAW: %v)",
		header.Filename, validationResult.AssetType, validationResult.MimeType, validationResult.IsRAW)

	repository, err := h.resolveUploadRepository(ctx, req.RepositoryID)
	if err != nil {
		h.respondRepositoryError(c, err)
		return
	}

	clientHash := c.GetHeader("X-Content-Hash")

	// Instant upload: a client-provided fingerprint that already exists in this
	// repository means the bytes are already here, so skip staging entirely.
	if clientHash != "" {
		duplicate, err := h.findDuplicateByHash(ctx, clientHash, header.Size, repository.RepoID)
		if err != nil {
			api.GinInternalError(c, err, "Failed to check for duplicate content")
			return
		}
		if duplicate != nil {
			log.Printf("Duplicate upload skipped: %s matches asset %s (hash %s)", header.Filename, duplicate.assetID, clientHash)
			api.JSONOK(c, dto.UploadResponseDTO{
				Status:      uploadStatusDuplicate,
				FileName:    header.Filename,
				Size:        header.Size,
				ContentHash: clientHash,
				Message:     "File already exists in repository",
			})
			return
		}
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
		h.handleUploadFailureFile(repository.Path, stagingFile.Path, header.Filename, "copy upload data to staging")
		api.GinInternalError(c, err, "Upload failed")
		return
	}

	// Calculate hash if not provided by client
	if clientHash == "" {
		log.Println("No content hash provided by client, calculating hash...")
		hashResult, err := hash.CalculateFileHash(stagingFile.Path, hash.AlgorithmBLAKE3, true)
		if err != nil {
			log.Printf("Failed to calculate hash: %v", err)
			h.handleUploadFailureFile(repository.Path, stagingFile.Path, header.Filename, "calculate upload hash")
			api.GinInternalError(c, err, "Failed to calculate file hash")
			return
		}
		clientHash = hashResult.Hash
		if hashResult.IsQuick {
			log.Printf("Calculated quick hash for large file %s: %s", header.Filename, clientHash)
		} else {
			log.Printf("Calculated hash for %s: %s", header.Filename, clientHash)
		}
	} else {
		log.Printf("Trusting client-provided hash for %s: %s", header.Filename, clientHash)
	}

	// Get user ID from JWT claims
	var userID string
	if id, exists := c.Get("user_id"); exists {
		userID = fmt.Sprintf("%d", id)
	} else {
		// Fallback to anonymous user if not authenticated
		userID = "anonymous"
	}

	payload := processors.AssetPayload{
		ClientHash:   clientHash,
		StagedPath:   stagingFile.Path,
		UserID:       userID,
		Timestamp:    time.Now(),
		ContentType:  validationResult.MimeType,
		FileName:     header.Filename,
		RepositoryID: uuid.UUID(repository.RepoID.Bytes).String(),
	}

	jobInsetResult, err := h.queueClient.Insert(ctx, jobs.IngestAssetArgs{
		ClientHash:   payload.ClientHash,
		StagedPath:   payload.StagedPath,
		UserID:       payload.UserID,
		Timestamp:    payload.Timestamp,
		ContentType:  payload.ContentType,
		FileName:     payload.FileName,
		RepositoryID: payload.RepositoryID,
	}, &river.InsertOpts{Queue: "ingest_asset"})

	if err != nil {
		log.Printf("Failed to enqueue task: %v", err)
		h.handleUploadFailureFile(repository.Path, stagingFile.Path, header.Filename, "enqueue ingest task")
		api.GinInternalError(c, err, "Upload failed")
		return
	}
	if jobInsetResult == nil || jobInsetResult.Job == nil {
		log.Printf("Failed to enqueue task: empty result")
		h.handleUploadFailureFile(repository.Path, stagingFile.Path, header.Filename, "enqueue ingest task returned empty result")
		api.GinInternalError(c, fmt.Errorf("enqueue failed"), "Upload failed")
		return
	}
	jobId := jobInsetResult.Job.ID
	log.Printf("Task %d enqueued for processing file %s in repository %s", jobId, header.Filename, repository.Name)

	response := dto.UploadResponseDTO{
		TaskID:      jobId,
		Status:      "processing",
		FileName:    header.Filename,
		Size:        header.Size,
		ContentHash: clientHash,
		Message:     fmt.Sprintf("File received and queued for processing in repository '%s'", repository.Name),
	}

	// Merge structural media components and detect bursts asynchronously after upload.
	if req.RepositoryID != "" {
		go func(repoID string) {
			if _, err := h.queueClient.Insert(context.Background(), jobs.DetectStacksArgs{
				RepositoryID: repoID,
			}, &river.InsertOpts{Queue: "detect_stacks"}); err != nil {
				log.Printf("failed to enqueue detect stacks after upload: %v", err)
			}
		}(req.RepositoryID)
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
// @Failure 400 {object} api.ErrorResponse "Bad request - no files provided or parse error"
// @Failure 500 {object} api.ErrorResponse "Internal server error"
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
		api.GinBadRequest(c, err, "Failed to read multipart data")
		return
	}

	clientHash := c.GetHeader("X-Content-Hash")

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
		chunkInfos  []upload.ChunkInfo
	}

	sessions := make(map[string]*sessionState)
	buf := make([]byte, 1<<20) // 1MiB shared buffer for streaming copy

	for {
		part, perr := mr.NextPart()
		if perr == io.EOF {
			break
		}
		if perr != nil {
			api.GinBadRequest(c, perr, "Failed to read multipart data")
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
			api.GinBadRequest(c, err, "Invalid file field name")
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
			h.sessionManager.CreateSession(fileInfo.SessionID, filename, 0, fileInfo.TotalChunks, contentType, repository.Path, userID)
		}

		// Update session hash if provided
		if clientHash != "" {
			h.sessionManager.SetSessionHash(fileInfo.SessionID, clientHash)
		}

		h.sessionManager.UpdateSessionStatus(fileInfo.SessionID, "uploading")

		targetName := filename
		if fileInfo.Type == "chunk" {
			targetName = fmt.Sprintf("chunk_%s_%d", fileInfo.SessionID, fileInfo.ChunkIndex)
		}

		stagingFile, err := h.stagingManager.CreateStagingFile(repository.Path, targetName)
		if err != nil {
			part.Close()
			api.GinInternalError(c, err, "Failed to create staging file")
			return
		}

		dst, err := os.Create(stagingFile.Path)
		if err != nil {
			part.Close()
			h.handleUploadFailureFile(repository.Path, stagingFile.Path, targetName, "open batch staging file")
			api.GinInternalError(c, err, "Failed to open staging file")
			return
		}

		written, err := io.CopyBuffer(dst, part, buf)
		dst.Close()
		part.Close()
		if err != nil {
			h.handleUploadFailureFile(repository.Path, stagingFile.Path, targetName, "save batch upload data")
			api.GinInternalError(c, err, "Failed to save upload data")
			return
		}

		h.sessionManager.UpdateSessionChunk(fileInfo.SessionID, fileInfo.ChunkIndex, written)

		state.chunkInfos = append(state.chunkInfos, upload.ChunkInfo{
			SessionID:  fileInfo.SessionID,
			ChunkIndex: fileInfo.ChunkIndex,
			FilePath:   stagingFile.Path,
			Size:       written,
		})
	}

	if len(sessions) == 0 {
		api.GinBadRequest(c, errors.New("no files provided"), "No files provided")
		return
	}

	var results []dto.BatchUploadResultDTO

	for sessionID, state := range sessions {
		if state.info.Type == "single" {
			session, _ := h.sessionManager.GetSession(sessionID)
			header := &multipart.FileHeader{
				Filename: state.filename,
				Size:     state.chunkInfos[0].Size,
				Header:   map[string][]string{},
			}
			header.Header.Set("Content-Type", state.contentType)

			result, err := h.processCompletedUpload(ctx, header, session, repository, state.chunkInfos[0].FilePath)
			if err != nil {
				errMsg := err.Error()
				results = append(results, dto.BatchUploadResultDTO{
					Success:  false,
					FileName: state.filename,
					Error:    &errMsg,
				})
				continue
			}

			h.sessionManager.UpdateSessionStatus(sessionID, "completed")
			results = append(results, *result)
			continue
		}

		h.chunkMerger.AddChunks(sessionID, state.chunkInfos)

		if !h.sessionManager.IsSessionComplete(sessionID) {
			progress, _ := h.sessionManager.GetSessionProgress(sessionID)
			status := "uploading"
			message := fmt.Sprintf("Upload in progress: %.1f%% complete", progress*100)
			results = append(results, dto.BatchUploadResultDTO{
				Success:  true,
				FileName: state.filename,
				Status:   &status,
				Message:  &message,
			})
			continue
		}

		h.sessionManager.UpdateSessionStatus(sessionID, "merging")
		mergeResult, err := h.chunkMerger.MergeChunks(sessionID, state.info.TotalChunks, repository.Path)
		if err != nil {
			errMsg := err.Error()
			h.sessionManager.SetSessionError(sessionID, errMsg)
			h.chunkMerger.CleanupChunks(sessionID)
			results = append(results, dto.BatchUploadResultDTO{
				Success:  false,
				FileName: state.filename,
				Error:    &errMsg,
			})
			continue
		}

		header := &multipart.FileHeader{
			Filename: state.filename,
			Size:     mergeResult.TotalSize,
			Header:   map[string][]string{},
		}
		header.Header.Set("Content-Type", state.contentType)

		session, _ := h.sessionManager.GetSession(sessionID)
		result, err := h.processCompletedUpload(ctx, header, session, repository, mergeResult.MergedFilePath)

		h.chunkMerger.CleanupChunks(sessionID)

		if err != nil {
			if mergeResult.MergedFilePath != "" {
				h.chunkMerger.CleanupMergedFile(mergeResult.MergedFilePath)
			}
			errMsg := err.Error()
			h.sessionManager.SetSessionError(sessionID, errMsg)
			results = append(results, dto.BatchUploadResultDTO{
				Success:  false,
				FileName: state.filename,
				Error:    &errMsg,
			})
			continue
		}

		h.sessionManager.UpdateSessionStatus(sessionID, "completed")
		results = append(results, *result)
	}

	if len(sessions) > 0 {
		go h.cleanupExpiredSessions()
	}

	// Merge structural media components and detect bursts asynchronously.
	// This is best-effort: if metadata has not been extracted yet, the next
	// scan or manual trigger will complete the stacking.
	if repositoryID != "" {
		go func(repoID string) {
			if _, err := h.queueClient.Insert(context.Background(), jobs.DetectStacksArgs{
				RepositoryID: repoID,
			}, &river.InsertOpts{Queue: "detect_stacks"}); err != nil {
				log.Printf("failed to enqueue detect stacks after upload: %v", err)
			}
		}(repositoryID)
	}

	api.JSONOK(c, dto.BatchUploadResponseDTO{Results: results})
}

// PrecheckUpload reports which of the candidate files already exist in the repository.
// @Summary Precheck uploads against existing content hashes
// @Description Given client-computed BLAKE3 fingerprints, reports which files already exist in the repository so the client can skip transporting them.
// @Tags assets
// @Accept json
// @Produce json
// @Param request body dto.UploadPrecheckRequestDTO true "Candidate files"
// @Success 200 {object} dto.UploadPrecheckResponseDTO
// @Failure 400 {object} api.ErrorResponse
// @Failure 404 {object} api.ErrorResponse
// @Failure 500 {object} api.ErrorResponse
// @Router /api/v1/assets/precheck [post]
func (h *AssetHandler) PrecheckUpload(c *gin.Context) {
	ctx := c.Request.Context()

	var req dto.UploadPrecheckRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		api.GinBadRequest(c, err, "Invalid request body")
		return
	}

	repository, err := h.resolveUploadRepository(ctx, req.RepositoryID)
	if err != nil {
		h.respondRepositoryError(c, err)
		return
	}

	hashes := make([]string, 0, len(req.Files))
	for _, file := range req.Files {
		hashes = append(hashes, file.Hash)
	}

	rows, err := h.queries.GetAssetsByHashesAndRepository(ctx, repo.GetAssetsByHashesAndRepositoryParams{
		Hashes:       hashes,
		RepositoryID: repository.RepoID,
	})
	if err != nil {
		api.GinInternalError(c, err, "Failed to precheck uploads")
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
	existing := make(map[fingerprint]existingAsset, len(rows))
	for _, row := range rows {
		if row.Hash == nil {
			continue
		}
		key := fingerprint{hash: *row.Hash, size: row.FileSize}
		if _, seen := existing[key]; seen {
			continue
		}
		existing[key] = existingAsset{
			assetID:  row.AssetID.String(),
			filename: row.OriginalFilename,
		}
	}

	results := make([]dto.UploadPrecheckResultDTO, 0, len(req.Files))
	duplicateCount := 0
	for _, file := range req.Files {
		result := dto.UploadPrecheckResultDTO{Hash: file.Hash}
		if match, ok := existing[fingerprint{hash: file.Hash, size: file.Size}]; ok {
			result.Duplicate = true
			result.AssetID = &match.assetID
			result.FileName = &match.filename
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

	if sessionIDsParam != "" {
		// Get specific sessions
		sessionIDs := strings.Split(sessionIDsParam, ",")
		for _, sessionID := range sessionIDs {
			if session, exists := h.sessionManager.GetSession(sessionID); exists {
				targetSessions = append(targetSessions, session)
			}
		}
	} else {
		// Get all sessions for current user
		userID := c.GetString("user_id")
		if userID == "" {
			userID = "anonymous"
		}
		targetSessions = h.sessionManager.GetSessionsByUser(userID)
	}

	var totalBytesDone, totalBytesTotal int64
	var completedFiles int

	sessionsProgress := make([]dto.SessionProgressDTO, len(targetSessions))
	for i, session := range targetSessions {
		progress, _ := h.sessionManager.GetSessionProgress(session.SessionID)

		sessionsProgress[i] = dto.SessionProgressDTO{
			SessionID:    session.SessionID,
			Filename:     session.Filename,
			Status:       session.Status,
			Progress:     progress,
			Received:     len(session.ReceivedChunks),
			Total:        session.TotalChunks,
			BytesDone:    session.BytesReceived,
			BytesTotal:   session.TotalSize,
			LastActivity: session.LastActivity,
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

// GetUploadJobStatus returns lifecycle state for accepted ingest jobs.
// @Summary Get upload materialization status
// @Description Get backend ingest lifecycle state for upload task IDs owned by the current caller
// @Tags assets
// @Produce json
// @Param task_ids query string true "Comma-separated upload task IDs"
// @Success 200 {object} dto.UploadJobStatusResponseDTO "Upload materialization status"
// @Failure 400 {object} api.ErrorResponse "Invalid task IDs"
// @Router /api/v1/assets/batch/jobs [get]
func (h *AssetHandler) GetUploadJobStatus(c *gin.Context) {
	rawIDs := strings.Split(strings.TrimSpace(c.Query("task_ids")), ",")
	if len(rawIDs) == 0 || len(rawIDs) > 100 || (len(rawIDs) == 1 && strings.TrimSpace(rawIDs[0]) == "") {
		api.GinBadRequest(c, errors.New("task_ids must contain between 1 and 100 IDs"), "Invalid upload task IDs")
		return
	}

	ids := make([]int64, 0, len(rawIDs))
	for _, rawID := range rawIDs {
		id, err := strconv.ParseInt(strings.TrimSpace(rawID), 10, 64)
		if err != nil || id <= 0 {
			api.GinBadRequest(c, errors.New("task_ids must be positive integers"), "Invalid upload task IDs")
			return
		}
		ids = append(ids, id)
	}

	jobRows, err := h.queueClient.JobList(c.Request.Context(), river.NewJobListParams().IDs(ids...).Kinds(jobs.IngestAssetArgs{}.Kind()).First(len(ids)))
	if err != nil {
		api.GinInternalError(c, err, "Failed to load upload status")
		return
	}

	callerID := "anonymous"
	if id, exists := c.Get("user_id"); exists {
		callerID = fmt.Sprintf("%d", id)
	}
	statuses := make([]dto.UploadJobStatusDTO, 0, len(jobRows.Jobs))
	for _, row := range jobRows.Jobs {
		if status, ok := uploadJobStatusForCaller(row, callerID); ok {
			statuses = append(statuses, status)
		}
	}

	api.JSONOK(c, dto.UploadJobStatusResponseDTO{Jobs: statuses})
}

func uploadJobStatusForCaller(row *rivertype.JobRow, callerID string) (dto.UploadJobStatusDTO, bool) {
	if row == nil {
		return dto.UploadJobStatusDTO{}, false
	}
	var args jobs.IngestAssetArgs
	if err := json.Unmarshal(row.EncodedArgs, &args); err != nil || args.UserID != callerID {
		return dto.UploadJobStatusDTO{}, false
	}
	terminal := row.State == rivertype.JobStateCompleted || row.State == rivertype.JobStateCancelled || row.State == rivertype.JobStateDiscarded
	success := row.State == rivertype.JobStateCompleted
	var errorMessage *string
	if len(row.Errors) > 0 && !success {
		message := row.Errors[len(row.Errors)-1].Error
		errorMessage = &message
	}
	return dto.UploadJobStatusDTO{
		TaskID: row.ID, FileName: args.FileName, Status: string(row.State),
		Terminal: terminal, Success: success, Error: errorMessage,
	}, true
}

// GetAsset retrieves a single asset by ID
// @Summary Get asset by ID
// @Description Retrieve detailed information about a specific asset. Optionally include thumbnails, tags, albums, species predictions, OCR results, face recognition, and captions.
// @Tags assets
// @Accept json
// @Produce json
// @Param id path string true "Asset ID (UUID format)" example("550e8400-e29b-41d4-a716-446655440000")
// @Param include_thumbnails query bool false "Include thumbnails" default(true)
// @Param include_tags query bool false "Include tags" default(true)
// @Param include_albums query bool false "Include albums" default(true)
// @Param include_species query bool false "Include species predictions" default(true)
// @Param include_ocr query bool false "Include OCR results" default(false)
// @Param include_faces query bool false "Include face recognition" default(false)
// @Success 200 {object} dto.AssetDetailDTO "Asset details with optional relationships"
// @Failure 400 {object} api.ErrorResponse "Invalid asset ID"
// @Failure 404 {object} api.ErrorResponse "Asset not found"
// @Router /api/v1/assets/{id} [get]
func (h *AssetHandler) GetAsset(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		api.GinBadRequest(c, err, "Invalid asset ID")
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
		api.GinNotFound(c, err, "Asset not found")
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
// @Failure 400 {object} api.ErrorResponse "Invalid asset ID"
// @Failure 404 {object} api.ErrorResponse "Asset or EXIF not found"
// @Router /api/v1/assets/{id}/exif [get]
func (h *AssetHandler) GetAssetExif(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		api.GinBadRequest(c, err, "Invalid asset ID")
		return
	}

	if _, ok := h.getAuthorizedAssetForRead(c, id, "Authentication required to access this asset", "You don't have permission to access this asset"); !ok {
		return
	}

	exifRaw, err := h.assetService.GetAssetExifRaw(c.Request.Context(), id)
	if err != nil {
		api.GinNotFound(c, err, "Asset not found")
		return
	}
	if len(exifRaw) == 0 {
		api.GinNotFound(c, errors.New("raw EXIF has not been extracted for this asset"), "EXIF not found")
		return
	}

	var exifRawObject map[string]any
	if err := json.Unmarshal(exifRaw, &exifRawObject); err != nil {
		api.GinInternalError(c, err, "Failed to decode EXIF")
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
// @Failure 400 {object} api.ErrorResponse "Invalid asset ID"
// @Failure 404 {object} api.ErrorResponse "Asset not found"
// @Failure 500 {object} api.ErrorResponse "Internal server error"
// @Router /api/v1/assets/{id}/sidecar [get]
func (h *AssetHandler) GetAssetSidecar(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		api.GinBadRequest(c, err, "Invalid asset ID")
		return
	}

	asset, ok := h.getAuthorizedAsset(c, id, "Authentication required to access this asset", "You don't have permission to access this asset")
	if !ok {
		return
	}

	repoPath, err := h.resolveAssetRepoPath(c.Request.Context(), asset)
	if err != nil {
		api.GinInternalError(c, err, "Failed to resolve asset sidecar")
		return
	}

	dirManager := h.repoManager.GetDirectoryManager()
	sidecar := h.defaultSidecarForAsset(id, asset)
	exists := false

	content, err := dirManager.ReadSidecar(repoPath, id.String())
	if err != nil {
		api.GinInternalError(c, err, "Failed to read asset sidecar")
		return
	}
	if content != nil {
		if err := json.Unmarshal(content, &sidecar); err != nil {
			api.GinInternalError(c, err, "Failed to decode asset sidecar")
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
// @Failure 400 {object} api.ErrorResponse "Invalid asset ID or request body"
// @Failure 404 {object} api.ErrorResponse "Asset not found"
// @Failure 500 {object} api.ErrorResponse "Internal server error"
// @Router /api/v1/assets/{id}/sidecar [put]
func (h *AssetHandler) UpdateAssetSidecar(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		api.GinBadRequest(c, err, "Invalid asset ID")
		return
	}

	asset, ok := h.getAuthorizedAsset(c, id, "Authentication required to update this asset", "You don't have permission to update this asset")
	if !ok {
		return
	}

	var sidecar dto.LumilioSidecarV1DTO
	if err := c.ShouldBindJSON(&sidecar); err != nil {
		api.GinBadRequest(c, err, "Invalid request body")
		return
	}

	sidecar.Version = 1
	sidecar.AssetID = id.String()
	sidecar.Source = h.sidecarSourceForAsset(asset)
	sidecar.UpdatedAt = time.Now().UTC()

	repoPath, err := h.resolveAssetRepoPath(c.Request.Context(), asset)
	if err != nil {
		api.GinInternalError(c, err, "Failed to resolve asset sidecar")
		return
	}

	content, err := json.MarshalIndent(sidecar, "", "  ")
	if err != nil {
		api.GinInternalError(c, err, "Failed to encode asset sidecar")
		return
	}

	dirManager := h.repoManager.GetDirectoryManager()
	if err := dirManager.WriteSidecar(repoPath, id.String(), content); err != nil {
		api.GinInternalError(c, err, "Failed to save asset sidecar")
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
// @Failure 400 {object} api.ErrorResponse "Invalid asset ID or size parameter"
// @Failure 404 {object} api.ErrorResponse "Asset or thumbnail not found"
// @Failure 500 {object} api.ErrorResponse "Internal server error"
// @Router /api/v1/assets/{id}/thumbnail [get]
func (h *AssetHandler) GetAssetThumbnail(c *gin.Context) {
	// Parse asset ID from URL parameter
	idStr := c.Param("id")
	assetID, err := uuid.Parse(idStr)
	if err != nil {
		api.GinBadRequest(c, err, "Invalid asset ID")
		return
	}

	// Get size parameter from query (default to "medium")
	size := c.DefaultQuery("size", "medium")

	// Validate size parameter
	if size != "small" && size != "medium" && size != "large" {
		api.GinBadRequest(c, errors.New("invalid size parameter"), "Invalid size parameter. Must be 'small', 'medium', or 'large'")
		return
	}

	asset, ok := h.getAuthorizedAssetForMedia(c, assetID, "Authentication required to access this thumbnail", "You don't have permission to access this thumbnail")
	if !ok {
		return
	}

	// Get thumbnail from service
	thumbnail, err := h.assetService.GetThumbnailByAssetIDAndSize(c.Request.Context(), assetID, size)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			api.GinNotFound(c, err, "Thumbnail not found")
			return
		}
		log.Printf("Failed to retrieve thumbnail metadata: %v", err)
		api.GinInternalError(c, err, "Failed to retrieve thumbnail")
		return
	}

	repository, err := h.getRepositoryForAsset(c.Request.Context(), asset)
	if err != nil {
		log.Printf("Failed to resolve repository for thumbnail request: %v", err)
		api.GinInternalError(c, err, "Failed to resolve repository")
		return
	}
	fullPath := h.resolveRepositoryPath(repository.Path, thumbnail.StoragePath)

	// Get file info for proper cache control
	fileInfo, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			api.GinNotFound(c, err, "Thumbnail file not found")
			return
		}
		log.Printf("Failed to get file info for %s: %v", fullPath, err)
		api.GinInternalError(c, err, "Failed to access thumbnail file")
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
// @Failure 400 {object} api.ErrorResponse "Invalid asset ID"
// @Failure 404 {object} api.ErrorResponse "Asset not found"
// @Failure 500 {object} api.ErrorResponse "Internal server error"
// @Router /api/v1/assets/{id}/original [get]
func (h *AssetHandler) GetOriginalFile(c *gin.Context) {
	ctx := c.Request.Context()

	// Parse asset ID from URL parameter
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		api.GinBadRequest(c, err, "Invalid asset ID")
		return
	}

	asset, ok := h.getAuthorizedAssetForMedia(c, id, "Authentication required to access this file", "You don't have permission to access this file")
	if !ok {
		return
	}

	if asset.StoragePath == nil || strings.TrimSpace(*asset.StoragePath) == "" {
		api.GinNotFound(c, fmt.Errorf("asset storage path is empty"), "Original file not found")
		return
	}

	repository, err := h.getRepositoryForAsset(ctx, asset)
	if err != nil {
		log.Printf("Failed to resolve repository for original file: %v", err)
		api.GinInternalError(c, err, "Failed to access repository")
		return
	}
	fullPath := h.resolveRepositoryPath(repository.Path, *asset.StoragePath)

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
// @Failure 400 {object} api.ErrorResponse "Invalid request"
// @Failure 401 {object} api.ErrorResponse "Authentication required"
// @Failure 403 {object} api.ErrorResponse "Forbidden"
// @Failure 404 {object} api.ErrorResponse "Asset or original file not found"
// @Failure 422 {object} api.ErrorResponse "Source image could not be encoded"
// @Failure 500 {object} api.ErrorResponse "Internal server error"
// @Router /api/v1/assets/{id}/export [get]
func (h *AssetHandler) ExportAsset(c *gin.Context) {
	ctx := c.Request.Context()

	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		api.GinBadRequest(c, err, "Invalid asset ID")
		return
	}

	format := strings.ToLower(strings.TrimSpace(c.Query("format")))
	if !imaging.IsSupportedExportFormat(format) {
		api.GinBadRequest(c, fmt.Errorf("unsupported export format %q", format), "Unsupported export format")
		return
	}

	asset, ok := h.getAuthorizedAssetForMedia(c, id, "Authentication required to export this file", "You don't have permission to export this file")
	if !ok {
		return
	}

	if asset.StoragePath == nil || strings.TrimSpace(*asset.StoragePath) == "" {
		api.GinNotFound(c, fmt.Errorf("asset storage path is empty"), "Original file not found")
		return
	}

	repository, err := h.getRepositoryForAsset(ctx, asset)
	if err != nil {
		log.Printf("Failed to resolve repository for export: %v", err)
		api.GinInternalError(c, err, "Failed to access repository")
		return
	}
	fullPath := h.resolveRepositoryPath(repository.Path, *asset.StoragePath)

	if _, statErr := os.Stat(fullPath); os.IsNotExist(statErr) {
		api.GinNotFound(c, statErr, "Original file not found")
		return
	}

	// OpenPhoto yields a libvips-decodable source for any photo: RAW files are
	// resolved to their embedded preview (full render as fallback), non-RAW files
	// are opened directly. This is what lets the export endpoint handle RAW.
	reader, err := imagesource.OpenPhoto(ctx, fullPath, asset.OriginalFilename)
	if err != nil {
		log.Printf("Failed to open source for export of asset %s: %v", id, err)
		api.GinError(c, http.StatusUnprocessableEntity, err, http.StatusUnprocessableEntity,
			"Failed to decode the source image for export")
		return
	}
	defer reader.Close()

	buf, err := io.ReadAll(reader)
	if err != nil {
		log.Printf("Failed to read source for export of asset %s: %v", id, err)
		api.GinInternalError(c, err, "Failed to read source image")
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
		api.GinError(c, http.StatusUnprocessableEntity, err, http.StatusUnprocessableEntity,
			"Failed to encode export; the source image may be unsupported")
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
// @Failure 400 {object} api.ErrorResponse "Invalid request"
// @Failure 401 {object} api.ErrorResponse "Authentication required"
// @Failure 403 {object} api.ErrorResponse "Forbidden"
// @Failure 404 {object} api.ErrorResponse "Asset or original file not found"
// @Failure 500 {object} api.ErrorResponse "Internal server error"
// @Router /api/v1/assets/download [post]
func (h *AssetHandler) DownloadAssets(c *gin.Context) {
	ctx := c.Request.Context()

	var req dto.DownloadAssetsRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		api.GinBadRequest(c, err, "Invalid request body")
		return
	}

	if len(req.AssetIDs) == 0 {
		api.GinBadRequest(c, errors.New("asset_ids is required"), "asset_ids is required")
		return
	}

	files := make([]assetDownloadFile, 0, len(req.AssetIDs))
	for _, rawAssetID := range req.AssetIDs {
		assetIDText := strings.TrimSpace(rawAssetID)
		assetID, err := uuid.Parse(assetIDText)
		if err != nil {
			api.GinBadRequest(c, err, "Invalid asset ID")
			return
		}

		asset, ok := h.getAuthorizedAssetForMedia(c, assetID, "Authentication required to access this file", "You don't have permission to access this file")
		if !ok {
			return
		}

		if asset.StoragePath == nil || strings.TrimSpace(*asset.StoragePath) == "" {
			api.GinNotFound(c, fmt.Errorf("asset storage path is empty"), "Original file not found")
			return
		}

		repository, err := h.getRepositoryForAsset(ctx, asset)
		if err != nil {
			log.Printf("Failed to resolve repository for bulk download: %v", err)
			api.GinInternalError(c, err, "Failed to access repository")
			return
		}

		fullPath := h.resolveRepositoryPath(repository.Path, *asset.StoragePath)
		fileInfo, err := os.Stat(fullPath)
		if err != nil {
			if os.IsNotExist(err) {
				log.Printf("Original file not found at path: %s", fullPath)
				api.GinNotFound(c, err, "Original file not found")
				return
			}
			api.GinInternalError(c, err, "Failed to access original file")
			return
		}
		if fileInfo.IsDir() {
			api.GinNotFound(c, fmt.Errorf("original file path is a directory"), "Original file not found")
			return
		}

		files = append(files, assetDownloadFile{
			asset: *asset,
			path:  fullPath,
		})
	}

	filename := fmt.Sprintf("lumilio-assets-%s.zip", time.Now().Format("20060102-150405"))
	c.Header("Cache-Control", "no-store")
	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	c.Status(http.StatusOK)

	zipWriter := zip.NewWriter(c.Writer)
	archiveNames := make(map[string]int, len(files))
	for _, file := range files {
		if err := writeAssetToZip(zipWriter, archiveNames, file); err != nil {
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
// @Failure 400 {object} api.ErrorResponse "Invalid asset ID"
// @Failure 404 {object} api.ErrorResponse "Asset not found or not a video"
// @Failure 500 {object} api.ErrorResponse "Internal server error"
// @Router /api/v1/assets/{id}/video/web [get]
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
	asset, ok := h.getAuthorizedAssetForMedia(c, id, "Authentication required to access this video", "You don't have permission to access this video")
	if !ok {
		return
	}

	// Check if asset is a video
	if asset.Type != "VIDEO" {
		api.GinBadRequest(c, fmt.Errorf("asset is not a video"), "Asset is not a video")
		return
	}

	if asset.StoragePath == nil || strings.TrimSpace(*asset.StoragePath) == "" {
		api.GinNotFound(c, fmt.Errorf("asset storage path is empty"), "Video file not found")
		return
	}

	repository, err := h.getRepositoryForAsset(ctx, asset)
	if err != nil {
		api.GinInternalError(c, err, "Failed to access repository")
		return
	}
	repoPath := repository.Path

	// Construct web video file path in .lumilio/assets/videos/web/
	var fullPath string
	webVersionExists := false

	if asset.Hash != nil && *asset.Hash != "" {
		webVideoFilename := fmt.Sprintf("%s_web.mp4", *asset.Hash)
		webVideoPath := filepath.Join(storage.DefaultStructure.VideosDir, "web", webVideoFilename)
		fullPath = filepath.Join(repoPath, webVideoPath)

		if _, err := os.Stat(fullPath); err == nil {
			webVersionExists = true
		}
	}

	// Check if web version exists, fallback to original
	if !webVersionExists {
		// Fallback to original file
		fullPath = h.resolveRepositoryPath(repoPath, *asset.StoragePath)
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
// @Failure 400 {object} api.ErrorResponse "Invalid asset ID"
// @Failure 404 {object} api.ErrorResponse "Asset not found or not audio"
// @Failure 500 {object} api.ErrorResponse "Internal server error"
// @Router /api/v1/assets/{id}/audio/web [get]
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
	asset, ok := h.getAuthorizedAssetForMedia(c, id, "Authentication required to access this audio", "You don't have permission to access this audio")
	if !ok {
		return
	}

	// Check if asset is audio
	if asset.Type != "AUDIO" {
		api.GinBadRequest(c, fmt.Errorf("asset is not audio"), "Asset is not audio")
		return
	}

	if asset.StoragePath == nil || strings.TrimSpace(*asset.StoragePath) == "" {
		api.GinNotFound(c, fmt.Errorf("asset storage path is empty"), "Audio file not found")
		return
	}

	repository, err := h.getRepositoryForAsset(ctx, asset)
	if err != nil {
		api.GinInternalError(c, err, "Failed to access repository")
		return
	}
	repoPath := repository.Path

	// Construct web audio file path in .lumilio/assets/audios/web/
	var fullPath string
	webVersionExists := false

	if asset.Hash != nil && *asset.Hash != "" {
		webAudioFilename := fmt.Sprintf("%s_web.mp3", *asset.Hash)
		webAudioPath := filepath.Join(storage.DefaultStructure.AudiosDir, "web", webAudioFilename)
		fullPath = filepath.Join(repoPath, webAudioPath)

		if _, err := os.Stat(fullPath); err == nil {
			webVersionExists = true
		}
	}

	// Check if web version exists, fallback to original
	if !webVersionExists {
		// Fallback to original file
		fullPath = h.resolveRepositoryPath(repoPath, *asset.StoragePath)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			log.Printf("Audio file not found at path: %s", fullPath)
			api.GinNotFound(c, err, "Audio file not found")
			return
		}
	}

	// Set appropriate headers for audio streaming
	c.Header("Cache-Control", "public, max-age=86400") // Cache for 1 day
	c.Header("Content-Type", "audio/mpeg")
	c.Header("Vary", "Accept-Encoding")
	c.Header("Accept-Ranges", "bytes") // Enable range requests for audio seeking

	// Serve the file
	c.File(fullPath)
}

// UpdateAsset updates asset metadata
// @Summary Update asset metadata
// @Description Update the specific metadata of an asset (e.g., photo EXIF data, video metadata).
// @Tags assets
// @Produce json
// @Param id path string true "Asset ID (UUID format)" example("550e8400-e29b-41d4-a716-446655440000")
// @Param data body dto.UpdateAssetRequestDTO true "Asset metadata"
// @Success 200 {object} dto.MessageResponseDTO "Asset updated successfully"
// @Failure 400 {object} api.ErrorResponse "Invalid asset ID or request body"
// @Failure 500 {object} api.ErrorResponse "Internal server error"
// @Router /api/v1/assets/{id} [put]
func (h *AssetHandler) UpdateAsset(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		api.GinBadRequest(c, err, "Invalid asset ID")
		return
	}

	if _, ok := h.getAuthorizedAsset(c, id, "Authentication required to update this asset", "You don't have permission to update this asset"); !ok {
		return
	}

	var updateData dto.UpdateAssetRequestDTO
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
// @Failure 400 {object} api.ErrorResponse "Invalid asset ID format"
// @Failure 500 {object} api.ErrorResponse "Internal server error"
// @Router /api/v1/assets/{id} [delete]
func (h *AssetHandler) DeleteAsset(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		api.GinBadRequest(c, err, "Invalid asset ID")
		return
	}

	if _, ok := h.getAuthorizedAsset(c, id, "Authentication required to delete this asset", "You don't have permission to delete this asset"); !ok {
		return
	}

	err = h.assetService.DeleteAsset(c.Request.Context(), id)
	if err != nil {
		log.Printf("Failed to delete asset: %v", err)
		api.GinInternalError(c, err, "Failed to delete asset")
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
// @Failure 400 {object} api.ErrorResponse "Invalid asset ID format"
// @Failure 500 {object} api.ErrorResponse "Internal server error"
// @Router /api/v1/assets/{id}/restore [post]
func (h *AssetHandler) RestoreAsset(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		api.GinBadRequest(c, err, "Invalid asset ID")
		return
	}

	if _, ok := h.getAuthorizedAssetAny(c, id, "Authentication required to restore this asset", "You don't have permission to restore this asset"); !ok {
		return
	}

	err = h.assetService.RestoreAsset(c.Request.Context(), id)
	if err != nil {
		log.Printf("Failed to restore asset: %v", err)
		api.GinInternalError(c, err, "Failed to restore asset")
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
// @Failure 400 {object} api.ErrorResponse "Invalid asset ID or album ID"
// @Failure 500 {object} api.ErrorResponse "Internal server error"
// @Router /api/v1/assets/{id}/albums/{albumId} [post]
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

	asset, ok := h.getAuthorizedAsset(c, assetID, "Authentication required to modify this asset", "You don't have permission to modify this asset")
	if !ok {
		return
	}

	album, err := h.queries.GetAlbumByID(c.Request.Context(), int32(albumID))
	if err != nil {
		api.GinNotFound(c, err, "Album not found")
		return
	}
	if !ensureOwnerAccess(c, &album.UserID, "Authentication required to modify this album", "You don't have permission to modify this album") {
		return
	}
	if asset.OwnerID != nil && *asset.OwnerID != album.UserID && !currentUserIsAdmin(c) {
		api.GinForbidden(c, errors.New("cross-user album access denied"), "Asset and album must belong to the same user")
		return
	}

	err = h.assetService.AddAssetToAlbum(c.Request.Context(), assetID, albumID)
	if err != nil {
		log.Printf("Failed to add asset to album: %v", err)
		api.GinInternalError(c, err, "Failed to add asset to album")
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
	if err := enqueueBioClipAsset(ctx, h.queueClient, asset); err != nil {
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
			service.AssetIndexingTaskFaceRecognition:
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
			ID:        uuid.UUID(repository.RepoID.Bytes).String(),
			Name:      repository.Name,
			Role:      string(repository.Role),
			IsPrimary: repository.Role == dbtypes.RepoRolePrimary,
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

func buildQueryAssetsParams(query, searchType, sortBy, viewerTimeZone, stackMode string, filter dto.AssetFilterDTO, pagination dto.PaginationDTO) service.QueryAssetsParams {
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
		AssetType:        filter.Type,
		AssetTypes:       filter.Types,
		OwnerID:          filter.OwnerID,
		AlbumID:          albumIDPtr,
		FilenameValue:    filenameValue,
		FilenameOperator: filenameOperator,
		DateFrom:         dateFrom,
		DateTo:           dateTo,
		IsRaw:            filter.RAW,
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
	}
}

func toAssetDTOs(assets []repo.Asset) []dto.AssetDTO {
	items := make([]dto.AssetDTO, len(assets))
	for i, asset := range assets {
		items[i] = dto.ToAssetDTO(asset)
	}
	return items
}

func uuidStrings(values []uuid.UUID) []string {
	if len(values) == 0 {
		return nil
	}

	items := make([]string, 0, len(values))
	for _, value := range values {
		if value == uuid.Nil {
			continue
		}
		items = append(items, value.String())
	}
	return items
}

func toBrowseItemDTOs(items []service.BrowseItem) []dto.BrowseItemDTO {
	dtos := make([]dto.BrowseItemDTO, 0, len(items))
	for _, item := range items {
		assetDTO := dto.ToAssetDTO(item.Asset)
		if item.Type == "stack" && item.Stack != nil {
			stackSize := len(item.Stack.MemberAssetIDs)
			assetDTO.Stack = &dto.StackPreviewDTO{
				StackID:    item.Stack.StackID.String(),
				StackKind:  string(item.Stack.Kind),
				StackCover: true,
				StackSize:  &stackSize,
			}
			dtos = append(dtos, dto.BrowseItemDTO{
				Type: item.Type,
				ID:   item.ID,
				Stack: &dto.BrowseStackDTO{
					StackID:          item.Stack.StackID.String(),
					StackKind:        string(item.Stack.Kind),
					CoverAssetID:     item.Stack.CoverAssetID.String(),
					CoverAsset:       assetDTO,
					StackSize:        stackSize,
					MemberAssetIDs:   uuidStrings(item.Stack.MemberAssetIDs),
					MatchedMemberIDs: uuidStrings(item.Stack.MatchedMemberIDs),
				},
			})
			continue
		}

		dtos = append(dtos, dto.BrowseItemDTO{
			Type:  "asset",
			ID:    item.ID,
			Asset: &assetDTO,
		})
	}
	return dtos
}

func toQueryBrowseResponseDTO(result service.BrowseQueryResult, limit, offset int) dto.QueryAssetsResponseDTO {
	totalVisible := int(result.TotalVisible)
	totalAssets := int(result.TotalAssets)
	itemDTOs := toBrowseItemDTOs(result.Items)
	return dto.QueryAssetsResponseDTO{
		Items:        itemDTOs,
		TotalVisible: &totalVisible,
		TotalAssets:  &totalAssets,
		StackMode:    result.StackMode,
		Limit:        limit,
		Offset:       offset,
	}
}

func toSearchBrowseResponseDTO(result service.SearchBrowseResult, limit, offset int) dto.SearchAssetsResponseDTO {
	resultsTotalVisible := int(result.ResultsTotalVisible)
	resultsTotalAssets := int(result.ResultsTotalAssets)
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
		ResultItems:         resultItemDTOs,
		ResultsTotalVisible: &resultsTotalVisible,
		ResultsTotalAssets:  &resultsTotalAssets,
		StackMode:           result.StackMode,
		Limit:               limit,
		Offset:              offset,
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
// @Failure 400 {object} api.ErrorResponse "Invalid request parameters"
// @Failure 503 {object} api.ErrorResponse "Semantic search unavailable"
// @Failure 500 {object} api.ErrorResponse "Internal server error"
// @Router /api/v1/assets/list [post]
func (h *AssetHandler) QueryAssets(c *gin.Context) {
	var req dto.AssetQueryRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		api.GinBadRequest(c, err, "Invalid request data")
		return
	}

	normalizeAssetQueryPagination(&req.Pagination)

	if err := validateAssetQuerySearchType(req.SearchType); err != nil {
		api.GinBadRequest(c, err, "Search type must be 'filename' or 'semantic'")
		return
	}
	if err := validateAssetQuerySortBy(req.SortBy); err != nil {
		api.GinBadRequest(c, err, "sort_by must be 'recently_added' or 'date_captured'")
		return
	}
	if err := validateStackMode(req.StackMode); err != nil {
		api.GinBadRequest(c, err, "stack_mode must be 'collapsed' or 'expanded'")
		return
	}

	// Default to filename search if not specified
	if req.SearchType == "" {
		req.SearchType = "filename"
	}

	params := buildQueryAssetsParams(req.Query, req.SearchType, req.SortBy, req.ViewerTimezone, req.StackMode, req.Filter, req.Pagination)
	params = applyAssetOwnershipScope(c, params)

	browseResult, err := h.assetService.QueryBrowseItems(c.Request.Context(), params)
	if err != nil {
		// Check for semantic search unavailable error
		if errors.Is(err, service.ErrSemanticSearchUnavailable) {
			api.GinError(c, 503, err, 503, "Semantic search is currently unavailable")
			return
		}
		log.Printf("Failed to query assets: %v", err)
		api.GinInternalError(c, err, "Failed to query assets")
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
// @Description Search assets with optional top results enhancement and filename fallback.
// @Tags assets
// @Produce json
// @Param data body dto.SearchAssetsRequestDTO true "Search parameters"
// @Success 200 {object} dto.SearchAssetsResponseDTO "Assets searched successfully"
// @Failure 400 {object} api.ErrorResponse "Invalid request parameters"
// @Failure 500 {object} api.ErrorResponse "Internal server error"
// @Router /api/v1/assets/search [post]
func (h *AssetHandler) SearchAssets(c *gin.Context) {
	var req dto.SearchAssetsRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		api.GinBadRequest(c, err, "Invalid request data")
		return
	}

	normalizeAssetQueryPagination(&req.Pagination)
	if err := validateAssetQuerySortBy(req.SortBy); err != nil {
		api.GinBadRequest(c, err, "sort_by must be 'recently_added' or 'date_captured'")
		return
	}
	if err := validateStackMode(req.StackMode); err != nil {
		api.GinBadRequest(c, err, "stack_mode must be 'collapsed' or 'expanded'")
		return
	}

	if err := validateSearchEnhancementMode(req.EnhancementMode); err != nil {
		api.GinBadRequest(c, err, "Enhancement mode must be 'auto', 'off', or 'only'")
		return
	}
	if strings.TrimSpace(req.EnhancementMode) == "" {
		req.EnhancementMode = string(service.SearchEnhancementModeAuto)
	}

	params := buildQueryAssetsParams(req.Query, "filename", req.SortBy, req.ViewerTimezone, req.StackMode, req.Filter, req.Pagination)
	params = applyAssetOwnershipScope(c, params)

	result, err := h.assetService.SearchBrowseItems(c.Request.Context(), service.SearchAssetsParams{
		QueryAssetsParams: params,
		EnhancementMode:   service.SearchEnhancementMode(req.EnhancementMode),
		TopResultsLimit:   req.TopResultsLimit,
		Debug:             req.Debug,
	})
	if err != nil {
		log.Printf("Failed to search assets: %v", err)
		api.GinInternalError(c, err, "Failed to search assets")
		return
	}

	searchResponse := toSearchBrowseResponseDTO(result, req.Pagination.Limit, req.Pagination.Offset)

	api.JSONOK(c, searchResponse)
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
// @Failure 500 {object} api.ErrorResponse "Internal server error"
// @Router /api/v1/assets/indexing/repositories [get]
func (h *AssetHandler) ListIndexingRepositories(c *gin.Context) {
	repositories, err := h.repoManager.ListRepositories()
	if err != nil {
		log.Printf("Failed to list repositories for indexing: %v", err)
		api.GinInternalError(c, err, "Failed to list repositories")
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
// @Failure 400 {object} api.ErrorResponse "Invalid repository ID"
// @Failure 500 {object} api.ErrorResponse "Internal server error"
// @Router /api/v1/assets/indexing/stats [get]
func (h *AssetHandler) GetIndexingStats(c *gin.Context) {
	repositoryID := strings.TrimSpace(c.Query("repository_id"))
	var repositoryIDPtr *string
	if repositoryID != "" {
		if _, err := uuid.Parse(repositoryID); err != nil {
			api.GinBadRequest(c, err, "Invalid repository ID")
			return
		}
		repositoryIDPtr = &repositoryID
	}

	stats, err := h.indexingService.GetIndexingStats(c.Request.Context(), repositoryIDPtr)
	if err != nil {
		log.Printf("Failed to load indexing stats: %v", err)
		api.GinInternalError(c, err, "Failed to load indexing stats")
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
// @Failure 400 {object} api.ErrorResponse "Invalid request parameters"
// @Failure 500 {object} api.ErrorResponse "Internal server error"
// @Router /api/v1/assets/indexing/rebuild [post]
func (h *AssetHandler) RebuildAssetIndexes(c *gin.Context) {
	var req dto.RebuildAssetIndexesRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil && err != io.EOF {
		api.GinBadRequest(c, err, "Invalid request data")
		return
	}

	tasks, err := parseIndexingTasks(req.Tasks)
	if err != nil {
		api.GinBadRequest(c, err, "Invalid indexing task list")
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

	result, err := h.indexingService.EnqueueReindexAssets(c.Request.Context(), service.ReindexAssetsInput{
		RepositoryID: repositoryIDPtr,
		Tasks:        tasks,
		Limit:        normalizeRebuildIndexLimit(req.Limit),
		MissingOnly:  missingOnly,
	})
	if err != nil {
		log.Printf("Failed to queue reindex job: %v", err)
		api.GinInternalError(c, err, "Failed to queue reindex job")
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
	if result.JobID == 0 && len(result.Requested) == 0 {
		status = "skipped"
		message = "All requested indexing tasks are disabled in ML settings"
	}

	api.JSONOK(c, dto.RebuildAssetIndexesResponseDTO{
		Status:         status,
		Message:        message,
		JobID:          result.JobID,
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
// @Failure 400 {object} api.ErrorResponse "Invalid request parameters"
// @Failure 500 {object} api.ErrorResponse "Internal server error"
// @Router /api/v1/assets/featured [get]
func (h *AssetHandler) GetFeaturedAssets(c *gin.Context) {
	count, err := parseIntQueryWithRange(c, "count", 8, 1, 24)
	if err != nil {
		api.GinBadRequest(c, err, "Invalid count parameter")
		return
	}

	candidateLimit, err := parseIntQueryWithRange(c, "candidate_limit", 240, 16, 1000)
	if err != nil {
		api.GinBadRequest(c, err, "Invalid candidate_limit parameter")
		return
	}

	days, err := parseIntQueryWithRange(c, "days", 3650, 0, 36500)
	if err != nil {
		api.GinBadRequest(c, err, "Invalid days parameter")
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
			api.GinBadRequest(c, err, "Invalid repository_id parameter")
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
		api.GinInternalError(c, err, "Failed to build featured photos")
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
// @Failure 400 {object} api.ErrorResponse "Invalid request parameters"
// @Failure 500 {object} api.ErrorResponse "Internal server error"
// @Router /api/v1/assets/map-points [get]
func (h *AssetHandler) GetPhotoMapPoints(c *gin.Context) {
	limit, err := parseIntQueryWithRange(c, "limit", 1000, 1, 5000)
	if err != nil {
		api.GinBadRequest(c, err, "Invalid limit parameter")
		return
	}

	offset, err := parseIntQueryWithRange(c, "offset", 0, 0, 10000000)
	if err != nil {
		api.GinBadRequest(c, err, "Invalid offset parameter")
		return
	}

	var repositoryID *string
	if rawRepoID := strings.TrimSpace(c.Query("repository_id")); rawRepoID != "" {
		if _, err := uuid.Parse(rawRepoID); err != nil {
			api.GinBadRequest(c, err, "Invalid repository_id parameter")
			return
		}
		repositoryID = &rawRepoID
	}

	south, north, west, east, err := parseOptionalMapViewport(c)
	if err != nil {
		api.GinBadRequest(c, err, "Invalid viewport parameters")
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
		api.GinInternalError(c, err, "Failed to query photo map points")
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
		if !asset.AssetID.Valid {
			continue
		}
		id, err := uuid.FromBytes(asset.AssetID.Bytes[:])
		if err != nil {
			continue
		}
		seen[id.String()] = struct{}{}
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
// @Failure 500 {object} api.ErrorResponse "Internal server error"
// @Router /api/v1/assets/filter-options [get]
func (h *AssetHandler) GetFilterOptions(c *gin.Context) {
	ctx := c.Request.Context()

	cameraModels, err := h.assetService.GetDistinctCameraModels(ctx)
	if err != nil {
		log.Printf("Failed to get camera models: %v", err)
		api.GinInternalError(c, err, "Failed to get filter options")
		return
	}

	lenses, err := h.assetService.GetDistinctLenses(ctx)
	if err != nil {
		log.Printf("Failed to get lenses: %v", err)
		api.GinInternalError(c, err, "Failed to get filter options")
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
// @Failure 400 {object} api.ErrorResponse "Bad request"
// @Failure 404 {object} api.ErrorResponse "Asset not found"
// @Failure 500 {object} api.ErrorResponse "Internal server error"
// @Router /api/v1/assets/{id}/rating [put]
func (h *AssetHandler) UpdateAssetRating(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		api.GinBadRequest(c, err, "Invalid asset ID")
		return
	}

	var req dto.UpdateRatingRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		api.GinBadRequest(c, err, "Invalid request body")
		return
	}

	if req.Rating < 0 || req.Rating > 5 {
		api.GinBadRequest(c, nil, "Rating must be between 0 and 5")
		return
	}

	if _, ok := h.getAuthorizedAsset(c, id, "Authentication required to update this asset", "You don't have permission to update this asset"); !ok {
		return
	}

	err = h.assetService.UpdateAssetRating(c.Request.Context(), id, req.Rating)
	if err != nil {
		log.Printf("Failed to update asset rating: %v", err)
		api.GinInternalError(c, err, "Failed to update rating")
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
// @Failure 400 {object} api.ErrorResponse "Bad request"
// @Failure 404 {object} api.ErrorResponse "Asset not found"
// @Failure 500 {object} api.ErrorResponse "Internal server error"
// @Router /api/v1/assets/{id}/like [put]
func (h *AssetHandler) UpdateAssetLike(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		api.GinBadRequest(c, err, "Invalid asset ID")
		return
	}

	var req dto.UpdateLikeRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		api.GinBadRequest(c, err, "Invalid request body")
		return
	}

	if _, ok := h.getAuthorizedAsset(c, id, "Authentication required to update this asset", "You don't have permission to update this asset"); !ok {
		return
	}

	err = h.assetService.UpdateAssetLike(c.Request.Context(), id, req.Liked)
	if err != nil {
		log.Printf("Failed to update asset like status: %v", err)
		api.GinInternalError(c, err, "Failed to update like status")
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
// @Failure 400 {object} api.ErrorResponse "Bad request"
// @Failure 404 {object} api.ErrorResponse "Asset not found"
// @Failure 500 {object} api.ErrorResponse "Internal server error"
// @Router /api/v1/assets/{id}/rating-and-like [put]
func (h *AssetHandler) UpdateAssetRatingAndLike(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		api.GinBadRequest(c, err, "Invalid asset ID")
		return
	}

	var req dto.UpdateRatingAndLikeRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		api.GinBadRequest(c, err, "Invalid request body")
		return
	}

	if req.Rating < 0 || req.Rating > 5 {
		api.GinBadRequest(c, nil, "Rating must be between 0 and 5")
		return
	}

	if _, ok := h.getAuthorizedAsset(c, id, "Authentication required to update this asset", "You don't have permission to update this asset"); !ok {
		return
	}

	err = h.assetService.UpdateAssetRatingAndLike(c.Request.Context(), id, req.Rating, req.Liked)
	if err != nil {
		log.Printf("Failed to update asset rating and like status: %v", err)
		api.GinInternalError(c, err, "Failed to update rating and like status")
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
// @Failure 400 {object} api.ErrorResponse "Bad request"
// @Failure 404 {object} api.ErrorResponse "Asset not found"
// @Failure 500 {object} api.ErrorResponse "Internal server error"
// @Router /api/v1/assets/{id}/description [put]
func (h *AssetHandler) UpdateAssetDescription(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		api.GinBadRequest(c, err, "Invalid asset ID")
		return
	}

	var req dto.UpdateDescriptionRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		api.GinBadRequest(c, err, "Invalid request body")
		return
	}

	if _, ok := h.getAuthorizedAsset(c, id, "Authentication required to update this asset", "You don't have permission to update this asset"); !ok {
		return
	}

	err = h.assetService.UpdateAssetDescription(c.Request.Context(), id, req.Description)
	if err != nil {
		log.Printf("Failed to update asset description: %v", err)
		api.GinInternalError(c, err, "Failed to update description")
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
// @Failure 400 {object} api.ErrorResponse "Bad request"
// @Failure 500 {object} api.ErrorResponse "Internal server error"
// @Router /api/v1/assets/{id}/tags [get]
func (h *AssetHandler) GetAssetTags(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		api.GinBadRequest(c, err, "Invalid asset ID")
		return
	}

	if _, ok := h.getAuthorizedAssetForRead(c, id, "Authentication required to view this asset", "You don't have permission to view this asset"); !ok {
		return
	}

	raw, err := h.assetService.GetAssetTags(c.Request.Context(), id)
	if err != nil {
		log.Printf("Failed to get asset tags: %v", err)
		api.GinInternalError(c, err, "Failed to get tags")
		return
	}

	tags := []dto.AssetTagDTO{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &tags); err != nil {
			log.Printf("Failed to decode asset tags: %v", err)
			api.GinInternalError(c, err, "Failed to decode tags")
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
// @Failure 400 {object} api.ErrorResponse "Bad request"
// @Failure 500 {object} api.ErrorResponse "Internal server error"
// @Router /api/v1/assets/{id}/tags [post]
func (h *AssetHandler) AddAssetTag(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		api.GinBadRequest(c, err, "Invalid asset ID")
		return
	}

	var req dto.AddAssetTagRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		api.GinBadRequest(c, err, "Invalid request body")
		return
	}

	if _, ok := h.getAuthorizedAsset(c, id, "Authentication required to update this asset", "You don't have permission to update this asset"); !ok {
		return
	}

	tag, err := h.assetService.AddManualTagToAsset(c.Request.Context(), id, req.TagName)
	if err != nil {
		log.Printf("Failed to add tag to asset: %v", err)
		api.GinInternalError(c, err, "Failed to add tag")
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
// @Failure 400 {object} api.ErrorResponse "Bad request"
// @Failure 500 {object} api.ErrorResponse "Internal server error"
// @Router /api/v1/assets/{id}/tags/{tagId} [delete]
func (h *AssetHandler) RemoveAssetTag(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		api.GinBadRequest(c, err, "Invalid asset ID")
		return
	}

	tagID, err := strconv.Atoi(c.Param("tagId"))
	if err != nil {
		api.GinBadRequest(c, err, "Invalid tag ID")
		return
	}

	if _, ok := h.getAuthorizedAsset(c, id, "Authentication required to update this asset", "You don't have permission to update this asset"); !ok {
		return
	}

	if err := h.assetService.RemoveTagFromAsset(c.Request.Context(), id, tagID); err != nil {
		log.Printf("Failed to remove tag from asset: %v", err)
		api.GinInternalError(c, err, "Failed to remove tag")
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
// @Failure 500 {object} api.ErrorResponse "Internal server error"
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
		api.GinInternalError(c, err, "Failed to list tags")
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
// @Failure 400 {object} api.ErrorResponse "Invalid request parameters"
// @Failure 500 {object} api.ErrorResponse "Internal server error"
// @Router /api/v1/assets/tag-summaries [get]
func (h *AssetHandler) GetTagSummaries(c *gin.Context) {
	var repositoryID *string
	if rawRepoID := strings.TrimSpace(c.Query("repository_id")); rawRepoID != "" {
		if _, err := uuid.Parse(rawRepoID); err != nil {
			api.GinBadRequest(c, err, "Invalid repository_id parameter")
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
		api.GinBadRequest(c, err, "Invalid limit parameter")
		return
	}
	offset, err := parseIntQueryWithRange(c, "offset", 0, 0, 10000000)
	if err != nil {
		api.GinBadRequest(c, err, "Invalid offset parameter")
		return
	}

	summaries, err := h.assetService.ListTagSummaries(c.Request.Context(), ownerScopeID(c), repositoryID, source, query, limit, offset)
	if err != nil {
		log.Printf("Failed to list tag summaries: %v", err)
		api.GinInternalError(c, err, "Failed to list tag summaries")
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
// @Failure 400 {object} api.ErrorResponse "Invalid request parameters"
// @Failure 500 {object} api.ErrorResponse "Internal server error"
// @Router /api/v1/assets/folders [get]
func (h *AssetHandler) GetFolders(c *gin.Context) {
	var repositoryID *string
	if rawRepoID := strings.TrimSpace(c.Query("repository_id")); rawRepoID != "" {
		if _, err := uuid.Parse(rawRepoID); err != nil {
			api.GinBadRequest(c, err, "Invalid repository_id parameter")
			return
		}
		repositoryID = &rawRepoID
	}

	parentPath := normalizeFolderPath(c.Query("path"))

	summaries, err := h.assetService.ListFolderSummaries(c.Request.Context(), ownerScopeID(c), repositoryID, parentPath)
	if err != nil {
		log.Printf("Failed to list folder summaries: %v", err)
		api.GinInternalError(c, err, "Failed to list folder summaries")
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
// @Failure 400 {object} api.ErrorResponse "Invalid request parameters"
// @Failure 500 {object} api.ErrorResponse "Internal server error"
// @Router /api/v1/assets/folders/summary [get]
func (h *AssetHandler) GetFolderSummary(c *gin.Context) {
	repositoryID := strings.TrimSpace(c.Query("repository_id"))
	if _, err := uuid.Parse(repositoryID); err != nil {
		api.GinBadRequest(c, err, "Invalid or missing repository_id parameter")
		return
	}

	folderPath := normalizeFolderPath(c.Query("path"))

	summary, err := h.assetService.GetFolderSummary(c.Request.Context(), ownerScopeID(c), repositoryID, folderPath)
	if err != nil {
		log.Printf("Failed to get folder summary: %v", err)
		api.GinInternalError(c, err, "Failed to get folder summary")
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
// @Failure 400 {object} api.ErrorResponse "Bad request"
// @Failure 500 {object} api.ErrorResponse "Internal server error"
// @Router /api/v1/assets/rating/{rating} [get]
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
		api.GinInternalError(c, err, "Failed to retrieve assets")
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
// @Failure 500 {object} api.ErrorResponse "Internal server error"
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
		api.GinInternalError(c, err, "Failed to retrieve liked assets")
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

	// Get staging manager from repository manager
	stagingManager := h.repoManager.GetStagingManager()

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
				// Use staging manager's cleanup function with short max age (1 hour)
				err := stagingManager.CleanupStaging(repo.Path, time.Hour)
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
		for _, repoPath := range repositoryIDs {
			// Use GetRepositoryByPath since RepositoryID in session stores the path, not UUID
			repo, err := h.repoManager.GetRepositoryByPath(repoPath)
			if err != nil {
				log.Printf("❌ Failed to get repository with path %s: %v", repoPath, err)
				errorCount++
				continue
			}

			// Use staging manager's cleanup function with short max age (1 hour)
			err = stagingManager.CleanupStaging(repo.Path, time.Hour)
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

func (h *AssetHandler) getRepositoryForAsset(ctx context.Context, asset *repo.Asset) (*repo.Repository, error) {
	return getRepositoryForAsset(ctx, h.queries, asset)
}

func (h *AssetHandler) resolveAssetRepoPath(ctx context.Context, asset *repo.Asset) (string, error) {
	if asset == nil {
		return "", fmt.Errorf("asset is nil")
	}

	repository, err := h.getRepositoryForAsset(ctx, asset)
	if err != nil {
		return "", err
	}

	return repository.Path, nil
}

func (h *AssetHandler) sidecarSourceForAsset(asset *repo.Asset) dto.LumilioSidecarSourceDTO {
	source := dto.LumilioSidecarSourceDTO{}
	if asset == nil {
		return source
	}

	source.OriginalFilename = asset.OriginalFilename
	source.MimeType = asset.MimeType
	source.FileSize = asset.FileSize
	source.Hash = asset.Hash
	source.Width = asset.Width
	source.Height = asset.Height
	if asset.StoragePath != nil {
		source.StoragePath = *asset.StoragePath
	}
	return source
}

func (h *AssetHandler) defaultSidecarForAsset(assetID uuid.UUID, asset *repo.Asset) dto.LumilioSidecarV1DTO {
	return dto.LumilioSidecarV1DTO{
		Version:     1,
		AssetID:     assetID.String(),
		Source:      h.sidecarSourceForAsset(asset),
		Adjustments: dto.StudioEditAdjustmentsDTO{},
		UpdatedAt:   time.Now().UTC(),
	}
}

func (h *AssetHandler) resolveRepositoryPath(repositoryPath string, storagePath string) string {
	return resolveRepositoryPath(repositoryPath, storagePath)
}

func (h *AssetHandler) handleUploadFailureFile(repoPath, filePath, filename, reason string) {
	if strings.TrimSpace(filePath) == "" {
		return
	}

	if h.isStagingIncomingPath(repoPath, filePath) {
		stagingFile := &storage.StagingFile{
			ID:        filepath.Base(filePath),
			RepoPath:  repoPath,
			Path:      filePath,
			Filename:  filename,
			CreatedAt: time.Now(),
		}
		if err := h.stagingManager.MoveStagingToFailed(stagingFile); err != nil {
			log.Printf("Failed to move upload file to failed dir (%s): %v", reason, err)
			h.removeUploadTempFile(filePath)
		}
		return
	}

	h.removeUploadTempFile(filePath)
}

func (h *AssetHandler) removeUploadTempFile(filePath string) {
	if strings.TrimSpace(filePath) == "" {
		return
	}
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		log.Printf("Failed to remove temporary upload file %s: %v", filePath, err)
	}
}

func (h *AssetHandler) isStagingIncomingPath(repoPath string, filePath string) bool {
	repoAbs, repoErr := filepath.Abs(repoPath)
	pathAbs, pathErr := filepath.Abs(filePath)
	if repoErr != nil || pathErr != nil {
		return false
	}

	incomingDir := filepath.Join(repoAbs, storage.DefaultStructure.IncomingDir)
	rel, err := filepath.Rel(incomingDir, pathAbs)
	if err != nil || rel == "." {
		return err == nil && rel == "."
	}
	return !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != ".."
}

// stringPtr returns a pointer to a string
func stringPtr(s string) *string {
	return &s
}

// groupFilesBySession groups uploaded files by their session ID
func (h *AssetHandler) groupFilesBySession(formFiles map[string][]*multipart.FileHeader) map[string]map[string]*multipart.FileHeader {
	sessionGroups := make(map[string]map[string]*multipart.FileHeader)

	for fieldName, headers := range formFiles {
		if len(headers) == 0 {
			continue
		}

		log.Printf("Processing field: %s with %d headers", fieldName, len(headers))
		fileInfo, err := upload.ParseFileField(fieldName)
		if err != nil {
			log.Printf("Invalid field name format: %s - %v", fieldName, err)
			continue
		}
		log.Printf("Parsed field: type=%s, session=%s, chunk_index=%d, total_chunks=%d",
			fileInfo.Type, fileInfo.SessionID, fileInfo.ChunkIndex, fileInfo.TotalChunks)

		if sessionGroups[fileInfo.SessionID] == nil {
			sessionGroups[fileInfo.SessionID] = make(map[string]*multipart.FileHeader)
		}
		sessionGroups[fileInfo.SessionID][fieldName] = headers[0]
		log.Printf("Added field to session group: %s", fileInfo.SessionID)
	}

	return sessionGroups
}

// processUploadSession processes a complete upload session (single file or chunks)
func (h *AssetHandler) processUploadSession(ctx context.Context, sessionID string, files map[string]*multipart.FileHeader, repository repo.Repository, userID string) (*dto.BatchUploadResultDTO, error) {
	// Get first file to determine session type
	var firstFileInfo *upload.FileFieldInfo
	for fieldName := range files {
		log.Printf("processUploadSession: examining field %s", fieldName)
		fileInfo, err := upload.ParseFileField(fieldName)
		if err != nil {
			log.Printf("processUploadSession: failed to parse field %s: %v", fieldName, err)
			return nil, err
		}
		firstFileInfo = fileInfo
		log.Printf("processUploadSession: first file info - type=%s, session=%s, chunks=%d/%d",
			fileInfo.Type, fileInfo.SessionID, fileInfo.ChunkIndex, fileInfo.TotalChunks)
		break
	}

	if firstFileInfo == nil {
		return nil, errors.New("no valid files in session")
	}

	// Check memory availability for large files
	if firstFileInfo.Type == "chunk" {
		totalSize := int64(0)
		for _, header := range files {
			totalSize += header.Size
		}

		canAccept, reason := h.memoryMonitor.CanAcceptNewUpload(totalSize)
		if !canAccept {
			return nil, fmt.Errorf("insufficient system memory: %s", reason)
		}
	}

	// Process based on session type
	if firstFileInfo.Type == "single" {
		log.Printf("processUploadSession: processing as single file session")
		// Get the single file header
		var header *multipart.FileHeader
		for _, h := range files {
			header = h
			break
		}
		return h.processSingleFileSession(ctx, header, repository, userID)
	} else {
		log.Printf("processUploadSession: processing as chunked file session with %d total chunks", firstFileInfo.TotalChunks)
		return h.processChunkedFileSession(ctx, sessionID, files, firstFileInfo.TotalChunks, repository, userID)
	}
}

// processSingleFileSession processes a single file upload
func (h *AssetHandler) processSingleFileSession(ctx context.Context, header *multipart.FileHeader, repository repo.Repository, userID string) (*dto.BatchUploadResultDTO, error) {
	// Validate file type
	contentType := header.Header.Get("Content-Type")
	validationResult := filevalidator.ValidateFile(header.Filename, contentType)
	if !validationResult.Valid {
		return nil, fmt.Errorf("unsupported file type: %s", validationResult.ErrorReason)
	}

	// Create session for tracking
	session := h.sessionManager.CreateSession("", header.Filename, header.Size, 1, contentType, repository.Path, userID)
	h.sessionManager.UpdateSessionStatus(session.SessionID, "uploading")

	// Process the single file
	return h.processCompletedUpload(ctx, header, session, repository, "")
}

// processChunkedFileSession processes a chunked file upload session
func (h *AssetHandler) processChunkedFileSession(ctx context.Context, sessionID string, files map[string]*multipart.FileHeader, totalChunks int, repository repo.Repository, userID string) (*dto.BatchUploadResultDTO, error) {
	// Get filename from first chunk
	var filename string
	for _, header := range files {
		filename = header.Filename
		break
	}

	// Calculate total size
	totalSize := int64(0)
	for _, header := range files {
		totalSize += header.Size
	}

	// Validate file type using first chunk
	var firstHeader *multipart.FileHeader
	for _, header := range files {
		firstHeader = header
		break
	}
	contentType := firstHeader.Header.Get("Content-Type")
	validationResult := filevalidator.ValidateFile(filename, contentType)
	if !validationResult.Valid {
		return nil, fmt.Errorf("unsupported file type: %s", validationResult.ErrorReason)
	}

	session, exists := h.sessionManager.GetSession(sessionID)
	if !exists {
		log.Printf("processChunkedFileSession: creating new session %s", sessionID)
		// Pass the client-provided sessionID to create the session
		session = h.sessionManager.CreateSession(sessionID, filename, totalSize, totalChunks, contentType, repository.Path, userID)
	} else {
		log.Printf("processChunkedFileSession: using existing session %s", sessionID)
	}

	// Update session with received chunks
	log.Printf("processChunkedFileSession: updating session with %d files", len(files))
	for fieldName, header := range files {
		log.Printf("processChunkedFileSession: processing field %s, size=%d", fieldName, header.Size)
		fileInfo, err := upload.ParseFileField(fieldName)
		if err != nil {
			log.Printf("processChunkedFileSession: failed to parse field %s: %v", fieldName, err)
			continue
		}
		log.Printf("processChunkedFileSession: updating chunk %d for session %s", fileInfo.ChunkIndex, sessionID)
		success := h.sessionManager.UpdateSessionChunk(sessionID, fileInfo.ChunkIndex, header.Size)
		log.Printf("processChunkedFileSession: chunk update result=%v", success)
	}

	// Save all chunks to staging directory and add to chunk merger
	log.Printf("processChunkedFileSession: saving %d chunks to staging", len(files))
	chunkInfos := make([]upload.ChunkInfo, 0, len(files))
	for fieldName, header := range files {
		log.Printf("processChunkedFileSession: preparing chunk from field %s", fieldName)
		fileInfo, _ := upload.ParseFileField(fieldName)

		// Save chunk to temporary file
		tempFile, err := h.stagingManager.CreateStagingFile(repository.Path, fmt.Sprintf("chunk_%s_%d", sessionID, fileInfo.ChunkIndex))
		if err != nil {
			return nil, fmt.Errorf("failed to create chunk file: %w", err)
		}
		log.Printf("Chunk %d saved to: %s", fileInfo.ChunkIndex, tempFile.Path)

		file, err := header.Open()
		if err != nil {
			log.Printf("processChunkedFileSession: failed to open chunk %d: %v", fileInfo.ChunkIndex, err)
			return nil, fmt.Errorf("failed to open chunk: %w", err)
		}

		osFile, err := os.Create(tempFile.Path)
		if err != nil {
			log.Printf("processChunkedFileSession: failed to create chunk file %s: %v", tempFile.Path, err)
			file.Close()
			return nil, fmt.Errorf("failed to create chunk file: %w", err)
		}

		bytesCopied, err := io.Copy(osFile, file)
		osFile.Close()
		file.Close()
		if err != nil {
			log.Printf("processChunkedFileSession: failed to save chunk %d: %v", fileInfo.ChunkIndex, err)
			return nil, fmt.Errorf("failed to save chunk: %w", err)
		}
		log.Printf("processChunkedFileSession: saved chunk %d, copied %d bytes to %s", fileInfo.ChunkIndex, bytesCopied, tempFile.Path)

		chunkInfos = append(chunkInfos, upload.ChunkInfo{
			SessionID:  sessionID,
			ChunkIndex: fileInfo.ChunkIndex,
			FilePath:   tempFile.Path,
			Size:       header.Size,
		})
	}

	// Add chunks to chunk merger for tracking across requests
	h.chunkMerger.AddChunks(sessionID, chunkInfos)

	// Check if all chunks are received
	log.Printf("processChunkedFileSession: checking if session %s is complete", sessionID)
	isComplete := h.sessionManager.IsSessionComplete(sessionID)
	log.Printf("processChunkedFileSession: session complete status=%v", isComplete)

	if !isComplete {
		// Not all chunks received yet, return progress
		progress, exists := h.sessionManager.GetSessionProgress(sessionID)
		log.Printf("processChunkedFileSession: progress=%f, exists=%v", progress, exists)
		status := "uploading"
		message := fmt.Sprintf("Upload in progress: %.1f%% complete", progress*100)
		log.Printf("processChunkedFileSession: returning progress: %s", message)

		result := &dto.BatchUploadResultDTO{
			Success:  true,
			FileName: filename,
			Status:   &status,
			Message:  &message,
		}
		log.Printf("processChunkedFileSession: returning progress result: %+v", result)
		return result, nil
	}

	// All chunks received, merge and process
	log.Printf("processChunkedFileSession: all chunks received, starting merge")
	h.sessionManager.UpdateSessionStatus(sessionID, "merging")

	// Merge all chunks using the chunk merger's stored chunks
	mergeResult, err := h.chunkMerger.MergeChunks(sessionID, totalChunks, repository.Path)
	if err != nil {
		h.sessionManager.SetSessionError(sessionID, err.Error())
		// Cleanup chunk files
		h.chunkMerger.CleanupChunks(sessionID)
		return nil, fmt.Errorf("failed to merge chunks: %w", err)
	}
	log.Printf("Chunks merged to: %s (size: %d)", mergeResult.MergedFilePath, mergeResult.TotalSize)

	// Create a mock header for the merged file
	mergedHeader := &multipart.FileHeader{
		Filename: filename,
		Size:     mergeResult.TotalSize,
		Header:   map[string][]string{},
	}
	mergedHeader.Header.Set("Content-Type", contentType)

	// Process the merged file
	log.Printf("Starting processCompletedUpload for merged file: %s", mergeResult.MergedFilePath)
	result, processErr := h.processCompletedUpload(ctx, mergedHeader, session, repository, mergeResult.MergedFilePath)

	// Cleanup chunk files regardless of processing result
	h.chunkMerger.CleanupChunks(sessionID)

	if processErr != nil {
		// Cleanup merged file only if processing failed
		if mergeResult.MergedFilePath != "" {
			h.chunkMerger.CleanupMergedFile(mergeResult.MergedFilePath)
		}
		h.sessionManager.SetSessionError(sessionID, processErr.Error())
		return nil, processErr
	}

	h.sessionManager.UpdateSessionStatus(sessionID, "completed")
	return result, nil
}

// processCompletedUpload processes a completed upload (single file or merged chunks)
func (h *AssetHandler) processCompletedUpload(ctx context.Context, header *multipart.FileHeader, session *upload.UploadSession, repository repo.Repository, mergedFilePath string) (*dto.BatchUploadResultDTO, error) {
	var stagingFilePath string
	log.Printf("processCompletedUpload: mergedFilePath=%s, filename=%s", mergedFilePath, header.Filename)

	if mergedFilePath != "" {
		// Use the merged file path for chunked uploads
		stagingFilePath = mergedFilePath
		log.Printf("Using merged file path for chunked upload: %s", stagingFilePath)
	} else {
		// Create staging file for single file uploads
		stagingFile, err := h.stagingManager.CreateStagingFile(repository.Path, header.Filename)
		if err != nil {
			return nil, fmt.Errorf("failed to create staging file: %w", err)
		}
		stagingFilePath = stagingFile.Path
		log.Printf("Created staging file for single upload: %s", stagingFilePath)

		// Copy single file to staging
		file, err := header.Open()
		if err != nil {
			return nil, fmt.Errorf("failed to open file: %w", err)
		}
		defer file.Close()

		osFile, err := os.Create(stagingFilePath)
		if err != nil {
			return nil, fmt.Errorf("failed to open staging file: %w", err)
		}
		defer osFile.Close()

		_, err = io.Copy(osFile, file)
		if err != nil {
			h.handleUploadFailureFile(repository.Path, stagingFilePath, header.Filename, "copy completed upload to staging")
			return nil, fmt.Errorf("failed to copy file to staging: %w", err)
		}
	}

	// Calculate hash (prioritize client-provided hash from session)
	var finalHash string
	var hashMethod string

	if session != nil && session.ContentHash != "" {
		finalHash = session.ContentHash
		hashMethod = "client-provided"
		log.Printf("Trusting client-provided hash for %s: %s", header.Filename, finalHash)
	} else {
		log.Printf("Calculating hash for file: %s", stagingFilePath)
		hashResult, err := hash.CalculateFileHash(stagingFilePath, hash.AlgorithmBLAKE3, true)
		if err != nil {
			log.Printf("Failed to calculate hash for %s: %v", stagingFilePath, err)
			h.handleUploadFailureFile(repository.Path, stagingFilePath, header.Filename, "calculate completed upload hash")
			return nil, fmt.Errorf("failed to calculate file hash: %w", err)
		}

		finalHash = hashResult.Hash
		hashMethod = "quick"
		if !hashResult.IsQuick {
			hashMethod = "full"
		}
		log.Printf("Calculated %s hash for %s: %s", hashMethod, header.Filename, finalHash)
	}

	validationResult := filevalidator.ValidateFile(header.Filename, session.ContentType)
	if !validationResult.Valid {
		h.handleUploadFailureFile(repository.Path, stagingFilePath, header.Filename, "validate completed upload")
		return nil, fmt.Errorf("unsupported file type: %s", validationResult.ErrorReason)
	}
	finalContentType := validationResult.MimeType
	log.Printf("Completed upload resolved canonical MIME %s for %s", finalContentType, header.Filename)

	// Instant upload: identical content already in the repository, so drop the
	// staged bytes instead of ingesting a second copy.
	duplicate, err := h.findDuplicateByHash(ctx, finalHash, header.Size, repository.RepoID)
	if err != nil {
		h.handleUploadFailureFile(repository.Path, stagingFilePath, header.Filename, "check duplicate content before enqueue")
		return nil, fmt.Errorf("failed to check for duplicate content: %w", err)
	}

	if duplicate != nil {
		log.Printf("Duplicate upload skipped: %s matches asset %s (hash %s)", header.Filename, duplicate.assetID, finalHash)
		h.removeUploadTempFile(stagingFilePath)
		size := header.Size
		status := uploadStatusDuplicate
		message := "File already exists in repository"
		return &dto.BatchUploadResultDTO{
			Success:     true,
			FileName:    header.Filename,
			ContentHash: finalHash,
			Status:      &status,
			Size:        &size,
			Message:     &message,
		}, nil
	}

	// Enqueue for processing
	log.Printf("Enqueuing processing job for file: %s (hash: %s)", stagingFilePath, finalHash)
	jobResult, err := h.queueClient.Insert(ctx, jobs.IngestAssetArgs{
		ClientHash:   finalHash,
		StagedPath:   stagingFilePath,
		UserID:       session.UserID,
		Timestamp:    time.Now(),
		ContentType:  finalContentType,
		FileName:     session.Filename,
		RepositoryID: uuid.UUID(repository.RepoID.Bytes).String(),
	}, &river.InsertOpts{Queue: "ingest_asset"})

	if err != nil {
		h.handleUploadFailureFile(repository.Path, stagingFilePath, header.Filename, "enqueue ingest task")
		return nil, fmt.Errorf("failed to enqueue task: %w", err)
	}

	if jobResult == nil || jobResult.Job == nil {
		log.Printf("Failed to enqueue task: empty result for file: %s", stagingFilePath)
		h.handleUploadFailureFile(repository.Path, stagingFilePath, header.Filename, "enqueue ingest task returned empty result")
		return nil, errors.New("failed to enqueue task: empty result")
	}

	taskID := jobResult.Job.ID
	status := "processing"
	size := header.Size
	message := fmt.Sprintf("File uploaded with %s hash and queued for processing in repository '%s'", hashMethod, repository.Name)

	log.Printf("Task %d enqueued for processing file %s in repository %s (staged path: %s)", taskID, header.Filename, repository.Name, stagingFilePath)

	return &dto.BatchUploadResultDTO{
		Success:     true,
		FileName:    header.Filename,
		ContentHash: finalHash,
		TaskID:      &taskID,
		Status:      &status,
		Size:        &size,
		Message:     &message,
	}, nil
}

// duplicateAsset identifies the already-stored asset that a candidate upload matches.
type duplicateAsset struct {
	assetID  string
	filename string
}

// findDuplicateByHash reports the existing asset in the repository carrying the
// same content fingerprint, or nil when the content is new. Hash equality is the
// system's identity notion for asset content; size is compared alongside it
// because files over hash.QuickHashThreshold carry a quick hash that only covers
// their first and last chunk.
func (h *AssetHandler) findDuplicateByHash(ctx context.Context, contentHash string, size int64, repositoryID pgtype.UUID) (*duplicateAsset, error) {
	if contentHash == "" {
		return nil, nil
	}

	rows, err := h.queries.GetAssetsByHashesAndRepository(ctx, repo.GetAssetsByHashesAndRepositoryParams{
		Hashes:       []string{contentHash},
		RepositoryID: repositoryID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to look up existing content hash: %w", err)
	}

	for _, row := range rows {
		if row.FileSize != size {
			continue
		}
		return &duplicateAsset{
			assetID:  row.AssetID.String(),
			filename: row.OriginalFilename,
		}, nil
	}
	return nil, nil
}

// ReprocessAsset reprocesses a failed or warning asset
// @Summary Reprocess asset
// @Description Reprocess a failed or warning asset by resetting its status and re-enqueuing for processing
// @Tags assets
// @Produce json
// @Param id path string true "Asset ID"
// @Param data body dto.ReprocessAssetRequestDTO false "Reprocessing tasks (optional)"
// @Success 200 {object} dto.ReprocessAssetResponseDTO
// @Failure 400 {object} api.ErrorResponse
// @Failure 404 {object} api.ErrorResponse
// @Failure 500 {object} api.ErrorResponse
// @Router /api/v1/assets/{id}/reprocess [post]
func (h *AssetHandler) ReprocessAsset(c *gin.Context) {
	ctx := c.Request.Context()

	// Parse asset ID
	assetIDStr := c.Param("id")
	assetID, err := uuid.Parse(assetIDStr)
	if err != nil {
		api.GinBadRequest(c, err, "Invalid asset ID")
		return
	}

	// Parse request body
	var req dto.ReprocessAssetRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil && err != io.EOF {
		// Allow empty body
		req = dto.ReprocessAssetRequestDTO{}
	}

	// Validate requested tasks (using queue names as canonical task identifiers)
	if len(req.Tasks) > 0 {
		validQueues := map[string]bool{
			"metadata_asset":   true,
			"thumbnail_asset":  true,
			"transcode_asset":  true,
			"process_semantic": true,
			"process_bioclip":  true,
			"process_ocr":      true,
			"process_face":     true,
		}

		for _, task := range req.Tasks {
			if !validQueues[task] {
				api.GinBadRequest(c, fmt.Errorf("invalid queue name: %s", task), fmt.Sprintf("Invalid queue name: %s", task))
				return
			}
		}
	}

	// Get the asset to check its current status
	pgUUID := pgtype.UUID{}
	if err := pgUUID.Scan(assetID.String()); err != nil {
		api.GinBadRequest(c, err, "Invalid asset ID format")
		return
	}

	asset, err := h.queries.GetAssetByID(ctx, pgUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			api.GinNotFound(c, err, "Asset not found")
			return
		}
		api.GinInternalError(c, err, "Failed to get asset")
		return
	}

	if !ensureOwnerAccess(c, asset.OwnerID, "Authentication required to reprocess this asset", "You don't have permission to reprocess this asset") {
		return
	}

	// Parse current status
	var currentStatus status.AssetStatus
	if len(asset.Status) > 0 {
		currentStatus, err = status.FromJSONB(asset.Status)
		if err != nil {
			api.GinInternalError(c, err, "Failed to parse asset status")
			return
		}
	}

	// Check for fatal errors (skip state check to allow retry on any state)
	if currentStatus.HasFatalErrors() {
		api.GinBadRequest(c, errors.New("asset has fatal errors"), "Asset has fatal errors that prevent reprocessing")
		return
	}

	// Determine retry strategy
	if len(req.Tasks) == 0 || req.ForceFullRetry {
		// Full retry - reset status and enqueue full processing job
		updatedAsset, err := h.queries.ResetAssetStatusForRetry(ctx, pgUUID)
		if err != nil {
			api.GinInternalError(c, err, "Failed to reset asset status")
			return
		}

		// Get repository information
		repository, err := h.queries.GetRepository(ctx, updatedAsset.RepositoryID)
		if err != nil {
			api.GinInternalError(c, err, "Failed to get repository")
			return
		}

		// Check if storage path exists
		if updatedAsset.StoragePath == nil || *updatedAsset.StoragePath == "" {
			api.GinBadRequest(c, errors.New("asset has no storage path"), "Asset has no storage path")
			return
		}

		// Resolve the full path to the asset file
		assetPath := filepath.Join(repository.Path, *updatedAsset.StoragePath)

		// Check if the file exists
		if _, err := os.Stat(assetPath); os.IsNotExist(err) {
			api.GinNotFound(c, err, "Asset file not found")
			return
		}

		// Create a new processing job
		storagePath := *updatedAsset.StoragePath
		assetType := dbtypes.AssetType(updatedAsset.Type)

		metaArgs := jobs.MetadataArgs{
			AssetID:          updatedAsset.AssetID,
			RepoPath:         repository.Path,
			StoragePath:      storagePath,
			AssetType:        assetType,
			OriginalFilename: updatedAsset.OriginalFilename,
			FileSize:         updatedAsset.FileSize,
			MimeType:         updatedAsset.MimeType,
		}
		if _, err := h.queueClient.Insert(ctx, metaArgs, &river.InsertOpts{Queue: "metadata_asset"}); err != nil {
			api.GinInternalError(c, err, "Failed to enqueue metadata job")
			return
		}

		switch assetType {
		case dbtypes.AssetTypePhoto:
			if _, err := h.queueClient.Insert(ctx, jobs.ThumbnailArgs{
				AssetID:     updatedAsset.AssetID,
				RepoPath:    repository.Path,
				StoragePath: storagePath,
				AssetType:   assetType,
			}, &river.InsertOpts{Queue: "thumbnail_asset"}); err != nil {
				api.GinInternalError(c, err, "Failed to enqueue thumbnail job")
				return
			}
		case dbtypes.AssetTypeVideo:
			if _, err := h.queueClient.Insert(ctx, jobs.ThumbnailArgs{
				AssetID:     updatedAsset.AssetID,
				RepoPath:    repository.Path,
				StoragePath: storagePath,
				AssetType:   assetType,
			}, &river.InsertOpts{Queue: "thumbnail_asset"}); err != nil {
				api.GinInternalError(c, err, "Failed to enqueue thumbnail job")
				return
			}
			if _, err := h.queueClient.Insert(ctx, jobs.TranscodeArgs{
				AssetID:     updatedAsset.AssetID,
				RepoPath:    repository.Path,
				StoragePath: storagePath,
				AssetType:   assetType,
			}, &river.InsertOpts{Queue: "transcode_asset"}); err != nil {
				api.GinInternalError(c, err, "Failed to enqueue transcode job")
				return
			}
		case dbtypes.AssetTypeAudio:
			if _, err := h.queueClient.Insert(ctx, jobs.TranscodeArgs{
				AssetID:     updatedAsset.AssetID,
				RepoPath:    repository.Path,
				StoragePath: storagePath,
				AssetType:   assetType,
			}, &river.InsertOpts{Queue: "transcode_asset"}); err != nil {
				api.GinInternalError(c, err, "Failed to enqueue transcode job")
				return
			}
		}

		log.Printf("Full reprocessing jobs enqueued for asset %s", assetID.String())

		// Return success response
		response := dto.ReprocessAssetResponseDTO{
			AssetID:    assetID.String(),
			Status:     "queued",
			Message:    "Full reprocessing job queued successfully",
			RetryTasks: []string{"all_failed_tasks"}, // Indicate full retry
		}

		api.JSONOK(c, response)
		return
	} else {
		// Selective retry - enqueue selective retry job
		// Create selective retry job payload
		retryArgs := jobs.AssetRetryPayload{
			AssetID:        assetID.String(),
			RetryTasks:     req.Tasks,
			ForceFullRetry: req.ForceFullRetry,
		}

		// Enqueue the selective retry job
		jobResult, err := h.queueClient.Insert(ctx, retryArgs, &river.InsertOpts{
			Queue: "retry_asset",
		})
		if err != nil {
			api.GinInternalError(c, err, "Failed to enqueue selective retry job")
			return
		}

		log.Printf("Selective retry job %d enqueued for asset %s, tasks: %v", jobResult.Job.ID, assetID.String(), req.Tasks)

		// Return success response
		response := dto.ReprocessAssetResponseDTO{
			AssetID:    assetID.String(),
			Status:     "queued",
			Message:    "Selective retry job queued successfully",
			RetryTasks: req.Tasks,
		}

		api.JSONOK(c, response)
		return
	}
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
// @Failure 404 {object} api.ErrorResponse
// @Router /api/v1/assets/{id}/media-item [get]
// @Security BearerAuth
func (h *AssetHandler) GetAssetMediaItem(c *gin.Context) {
	assetID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		api.GinBadRequest(c, err, "Invalid asset ID")
		return
	}
	if _, ok := h.getAuthorizedAssetForRead(c, assetID, "Authentication required to access this asset", "You don't have permission to access this asset"); !ok {
		return
	}
	item, err := h.stackService.GetMediaItemByAsset(c.Request.Context(), assetID, ownerScopeID(c))
	if err != nil {
		api.GinNotFound(c, err, "Media item not found")
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
// @Failure 404 {object} api.ErrorResponse
// @Router /api/v1/assets/{id}/stack [get]
// @Security BearerAuth
func (h *AssetHandler) GetAssetStack(c *gin.Context) {
	assetIDStr := c.Param("id")
	assetID, err := uuid.Parse(assetIDStr)
	if err != nil {
		api.GinBadRequest(c, err, "Invalid asset ID")
		return
	}

	if _, ok := h.getAuthorizedAssetForRead(c, assetID, "Authentication required to access this asset", "You don't have permission to access this asset"); !ok {
		return
	}

	stackInfo, err := h.stackService.GetStackByAssetAny(c.Request.Context(), assetID, ownerScopeID(c))
	if err != nil {
		if errors.Is(err, service.ErrStackNotFound) {
			api.GinNotFound(c, err, "Asset is not in a stack")
			return
		}
		api.GinInternalError(c, err, "Failed to get stack")
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
// @Failure 400 {object} api.ErrorResponse
// @Failure 409 {object} api.ErrorResponse
// @Router /api/v1/assets/stacks [post]
// @Security BearerAuth
func (h *AssetHandler) CreateManualStack(c *gin.Context) {
	var req dto.CreateManualStackRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		api.GinBadRequest(c, err, "Invalid request body")
		return
	}

	if len(req.AssetIDs) < 2 {
		api.GinBadRequest(c, errors.New("at least 2 asset IDs are required"), "At least 2 asset IDs are required")
		return
	}

	assetIDs := make([]uuid.UUID, len(req.AssetIDs))
	for i, idStr := range req.AssetIDs {
		id, err := uuid.Parse(idStr)
		if err != nil {
			api.GinBadRequest(c, err, fmt.Sprintf("Invalid asset ID: %s", idStr))
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
			api.GinError(c, http.StatusConflict, err, http.StatusConflict, "One or more assets already belong to a stack")
			return
		}
		api.GinInternalError(c, err, "Failed to create stack")
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
		api.GinBadRequest(c, err, "Invalid asset ID")
		return
	}

	if _, ok := h.getAuthorizedAsset(c, assetID, "Authentication required to modify this asset", "You don't have permission to modify this asset"); !ok {
		return
	}

	if err := h.stackService.RemoveFromStack(c.Request.Context(), assetID); err != nil {
		api.GinInternalError(c, err, "Failed to unstack asset")
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
		api.GinBadRequest(c, err, "Invalid repository ID")
		return
	}

	count, err := h.stackService.AutoDetectStacks(c.Request.Context(), repoID)
	if err != nil {
		api.GinInternalError(c, err, "Failed to detect stacks")
		return
	}

	api.JSONOK(c, dto.AutoDetectStacksResponseDTO{
		RepositoryID:  repoID.String(),
		StacksCreated: count,
	})
}
