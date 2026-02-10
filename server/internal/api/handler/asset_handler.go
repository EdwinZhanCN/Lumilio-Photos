package handler

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
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
	"server/internal/utils/memory"
	"server/internal/utils/upload"
	"strconv"
	"strings"
	"time"

	"github.com/gabriel-vasile/mimetype"
	"github.com/google/uuid"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/riverqueue/river"
)

// AssetHandler handles HTTP requests for asset management
type AssetHandler struct {
	assetService   service.AssetService
	queries        *repo.Queries
	repoManager    storage.RepositoryManager
	stagingManager storage.StagingManager
	queueClient    *river.Client[pgx.Tx]
	memoryMonitor  *memory.MemoryMonitor
	sessionManager *upload.SessionManager
	chunkMerger    *upload.ChunkMerger
	uploadLimiter  chan struct{}
}

// NewAssetHandler creates a new AssetHandler instance
func NewAssetHandler(
	assetService service.AssetService,
	queries *repo.Queries,
	repoManager storage.RepositoryManager,
	stagingManager storage.StagingManager,
	queueClient *river.Client[pgx.Tx],
) *AssetHandler {
	memoryMonitor := memory.NewMemoryMonitor()
	sessionManager := upload.NewSessionManager(30 * time.Minute) // 30 minute timeout
	chunkMerger := upload.NewChunkMerger(storage.NewDirectoryManager())
	// Increased limit to 32 to support HTTP/2 multiplexing for chunked uploads
	uploadLimiter := make(chan struct{}, 32)

	handler := &AssetHandler{
		assetService:   assetService,
		queries:        queries,
		repoManager:    repoManager,
		stagingManager: stagingManager,
		queueClient:    queueClient,
		memoryMonitor:  memoryMonitor,
		sessionManager: sessionManager,
		chunkMerger:    chunkMerger,
		uploadLimiter:  uploadLimiter,
	}

	// Start background cleanup tasks
	go handler.startBackgroundCleanupTasks()

	return handler
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
// @Success 200 {object} api.Result{data=dto.UploadResponseDTO} "Upload successful"
// @Failure 400 {object} api.Result "Bad request - no file provided or parse error"
// @Failure 500 {object} api.Result "Internal server error"
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

	// Create a buffer to store file header for MIME type detection
	// Using 1024 bytes for better format detection compatibility
	headerBuf := bytes.NewBuffer(make([]byte, 0, 1024))

	// Create a TeeReader that reads from file and writes to buffer
	// This allows us to detect MIME type while preserving the file stream
	teeReader := io.TeeReader(file, headerBuf)

	// Read just the first 1024 bytes to limit buffer size
	limitedReader := io.LimitReader(teeReader, 1024)
	_, err = io.Copy(io.Discard, limitedReader)
	if err != nil {
		api.GinBadRequest(c, fmt.Errorf("failed to read file header: %w", err))
		return
	}

	// Detect MIME type from the header
	detectedMIME := mimetype.Detect(headerBuf.Bytes())
	detectedType := detectedMIME.String()

	// Use detected MIME type if the provided one is generic octet-stream
	finalContentType := header.Header.Get("Content-Type")
	if finalContentType == "application/octet-stream" || finalContentType == "" {
		finalContentType = detectedType
	}

	log.Printf("Original Content-Type: %s, Detected MIME: %s, Final Content-Type: %s",
		header.Header.Get("Content-Type"), detectedType, finalContentType)

	// Validate file type using the corrected content type
	validationResult := filevalidator.ValidateFile(header.Filename, finalContentType)
	if !validationResult.Valid {
		api.GinBadRequest(c, fmt.Errorf("unsupported file type: %s", validationResult.ErrorReason))
		return
	}
	log.Printf("Validated file %s as %s (RAW: %v)", header.Filename, validationResult.AssetType, validationResult.IsRAW)

	repositoryID := req.RepositoryID
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

	clientHash := c.GetHeader("X-Content-Hash")

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

	// Calculate hash if not provided by client
	if clientHash == "" {
		log.Println("No content hash provided by client, calculating hash...")
		hashResult, err := hash.CalculateFileHash(stagingFile.Path, hash.AlgorithmBLAKE3, true)
		if err != nil {
			log.Printf("Failed to calculate hash: %v", err)
			os.Remove(stagingFile.Path)
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
		ContentType:  finalContentType,
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

	response := dto.UploadResponseDTO{
		TaskID:      jobId,
		Status:      "processing",
		FileName:    header.Filename,
		Size:        header.Size,
		ContentHash: clientHash,
		Message:     fmt.Sprintf("File received and queued for processing in repository '%s'", repository.Name),
	}
	api.GinSuccess(c, response)
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
// @Success 200 {object} api.Result{data=dto.BatchUploadResponseDTO} "Batch upload completed"
// @Failure 400 {object} api.Result "Bad request - no files provided or parse error"
// @Failure 500 {object} api.Result "Internal server error"
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
		if repositoryID != "" {
			repoUUID, err := uuid.Parse(repositoryID)
			if err != nil {
				api.GinBadRequest(c, err, "Invalid repository ID")
				return false
			}
			repository, err = h.queries.GetRepository(ctx, pgtype.UUID{Bytes: repoUUID, Valid: true})
			if err != nil {
				api.GinNotFound(c, err, "Repository not found")
				return false
			}
		} else {
			// Use first available repository as default
			repositories, err := h.queries.ListRepositories(ctx)
			if err != nil || len(repositories) == 0 {
				api.GinBadRequest(c, errors.New("no repository available"), "Please specify a repository_id or create a repository first")
				return false
			}
			repository = repositories[0]
		}
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
			api.GinInternalError(c, err, "Failed to open staging file")
			return
		}

		written, err := io.CopyBuffer(dst, part, buf)
		dst.Close()
		part.Close()
		if err != nil {
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

	api.GinSuccess(c, dto.BatchUploadResponseDTO{Results: results})
}

// GetUploadConfig returns current upload configuration
// @Summary Get upload configuration
// @Description Get current upload configuration including chunk size and concurrency limits based on system memory
// @Tags assets
// @Accept json
// @Produce json
// @Success 200 {object} api.Result{data=dto.UploadConfigResponseDTO} "Upload configuration"
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

	api.GinSuccess(c, response)
}

// GetUploadProgress returns upload progress for sessions
// @Summary Get upload progress
// @Description Get detailed progress information for upload sessions
// @Tags assets
// @Accept json
// @Produce json
// @Param session_ids query string false "Comma-separated session IDs (optional)"
// @Success 200 {object} api.Result{data=dto.UploadProgressResponseDTO} "Upload progress details"
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

	api.GinSuccess(c, response)
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
// @Param include_captions query bool false "Include captions" default(false)
// @Success 200 {object} api.Result{data=dto.AssetDTO} "Asset details with optional relationships"
// @Failure 400 {object} api.Result "Invalid asset ID"
// @Failure 404 {object} api.Result "Asset not found"
// @Router /api/v1/assets/{id} [get]
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

	// New AI includes - default to false to avoid performance impact
	includeOCR := c.DefaultQuery("include_ocr", "false") == "true"
	includeFaces := c.DefaultQuery("include_faces", "false") == "true"
	includeCaptions := c.DefaultQuery("include_captions", "false") == "true"

	asset, err := h.assetService.GetAssetWithOptions(
		c.Request.Context(),
		id,
		includeThumbnails,
		includeTags,
		includeAlbums,
		includeSpecies,
		includeOCR,
		includeFaces,
		includeCaptions,
	)
	if err != nil {
		api.GinNotFound(c, err, "Asset not found")
		return
	}

	api.GinSuccess(c, asset)
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
// @Router /api/v1/assets/{id}/thumbnail [get]
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
	_, err = h.assetService.GetAssetWithOptions(c.Request.Context(), assetID, false, false, false, false, false, false, false)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
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
		if errors.Is(err, pgx.ErrNoRows) {
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

	// Get asset metadata from service
	asset, err := h.assetService.GetAsset(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			api.GinNotFound(c, err, "Asset not found")
			return
		}
		log.Printf("Failed to retrieve asset metadata: %v", err)
		api.GinInternalError(c, err, "Failed to retrieve asset")
		return
	}

	// Construct full file path from repository-relative storage path
	// Asset paths are stored relative to repository root (e.g., "inbox/2024/01/photo.jpg")
	fullPathPtr := asset.StoragePath
	fullPath := *fullPathPtr
	if !filepath.IsAbs(fullPath) {
		// Try to find the repository by listing all repositories
		repositories, err := h.queries.ListRepositories(ctx)
		if err == nil && len(repositories) > 0 {
			// Use first repository for now - proper implementation needs file_records
			fullPath = filepath.Join(repositories[0].Path, *asset.StoragePath)
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
	asset, err := h.assetService.GetAsset(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
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
		fullPath = *asset.StoragePath
		if !filepath.IsAbs(fullPath) {
			fullPath = filepath.Join(repoPath, *asset.StoragePath)
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
	asset, err := h.assetService.GetAsset(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
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
		fullPath = *asset.StoragePath
		if !filepath.IsAbs(fullPath) {
			fullPath = filepath.Join(repoPath, *asset.StoragePath)
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
	c.Header("Vary", "Accept-Encoding")
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
// @Param request body dto.UpdateAssetRequestDTO true "Asset metadata"
// @Success 200 {object} api.Result{data=dto.MessageResponseDTO} "Asset updated successfully"
// @Failure 400 {object} api.Result "Invalid asset ID or request body"
// @Failure 500 {object} api.Result "Internal server error"
// @Router /api/v1/assets/{id} [put]
func (h *AssetHandler) UpdateAsset(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		api.GinBadRequest(c, err, "Invalid asset ID")
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

	api.GinSuccess(c, dto.MessageResponseDTO{Message: "Asset updated successfully"})
}

// DeleteAsset deletes an asset
// @Summary Delete asset
// @Description Soft delete an asset by marking it as deleted. The physical file is not removed.
// @Tags assets
// @Accept json
// @Produce json
// @Param id path string true "Asset ID (UUID format)" example("550e8400-e29b-41d4-a716-446655440000")
// @Success 200 {object} api.Result{data=dto.MessageResponseDTO} "Asset deleted successfully"
// @Failure 400 {object} api.Result "Invalid asset ID format"
// @Failure 500 {object} api.Result "Internal server error"
// @Router /api/v1/assets/{id} [delete]
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

	api.GinSuccess(c, dto.MessageResponseDTO{Message: "Asset deleted successfully"})
}

// AddAssetToAlbum adds an asset to an album
// @Summary Add asset to album
// @Description Associate an asset with a specific album by asset ID and album ID.
// @Tags assets
// @Accept json
// @Produce json
// @Param id path string true "Asset ID (UUID format)" example("550e8400-e29b-41d4-a716-446655440000")
// @Param albumId path int true "Album ID" example(123)
// @Success 200 {object} api.Result{data=dto.MessageResponseDTO} "Asset added to album successfully"
// @Failure 400 {object} api.Result "Invalid asset ID or album ID"
// @Failure 500 {object} api.Result "Internal server error"
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

	err = h.assetService.AddAssetToAlbum(c.Request.Context(), assetID, albumID)
	if err != nil {
		log.Printf("Failed to add asset to album: %v", err)
		api.GinInternalError(c, err, "Failed to add asset to album")
		return
	}

	api.GinSuccess(c, dto.MessageResponseDTO{Message: "Asset added to album successfully"})
}

// GetAssetTypes returns available asset types
// @Summary Get supported asset types
// @Description Retrieve a list of all supported asset types in the system.
// @Tags assets
// @Accept json
// @Produce json
// @Success 200 {object} api.Result{data=dto.AssetTypesResponseDTO} "Asset types retrieved successfully"
// @Router /api/v1/assets/types [get]
func (h *AssetHandler) GetAssetTypes(c *gin.Context) {
	types := []dbtypes.AssetType{
		dbtypes.AssetTypePhoto,
		dbtypes.AssetTypeVideo,
		dbtypes.AssetTypeAudio,
	}

	api.GinSuccess(c, dto.AssetTypesResponseDTO{Types: types})
}

// QueryAssets handles unified asset listing, filtering, and searching
// @Summary Query assets (unified endpoint)
// @Description Unified endpoint for listing, filtering, and searching assets. Replaces separate /filter and /search endpoints.
// @Tags assets
// @Accept json
// @Produce json
// @Param request body dto.AssetQueryRequestDTO true "Query parameters"
// @Success 200 {object} api.Result{data=dto.AssetListResponseDTO} "Assets queried successfully"
// @Failure 400 {object} api.Result "Invalid request parameters"
// @Failure 503 {object} api.Result "Semantic search unavailable"
// @Failure 500 {object} api.Result "Internal server error"
// @Router /api/v1/assets/list [post]
func (h *AssetHandler) QueryAssets(c *gin.Context) {
	var req dto.AssetQueryRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		api.GinBadRequest(c, err, "Invalid request data")
		return
	}

	// Validate and set defaults for pagination
	if req.Pagination.Limit <= 0 || req.Pagination.Limit > 100 {
		req.Pagination.Limit = 20
	}
	if req.Pagination.Offset < 0 {
		req.Pagination.Offset = 0
	}

	// Validate search type if provided
	if req.SearchType != "" && req.SearchType != "filename" && req.SearchType != "semantic" {
		api.GinBadRequest(c, errors.New("invalid search type"), "Search type must be 'filename' or 'semantic'")
		return
	}

	// Default to filename search if not specified
	if req.SearchType == "" {
		req.SearchType = "filename"
	}

	ctx := c.Request.Context()
	filter := req.Filter

	// Convert filter parameters
	var dateFrom, dateTo *time.Time
	if filter.Date != nil {
		dateFrom = filter.Date.From
		dateTo = filter.Date.To

		// Normalize date-only inputs to inclusive end-of-day
		if dateFrom != nil && dateTo == nil {
			end := time.Date(dateFrom.Year(), dateFrom.Month(), dateFrom.Day(), 23, 59, 59, int(time.Second-time.Nanosecond), dateFrom.Location())
			dateTo = &end
		} else if dateTo != nil {
			end := time.Date(dateTo.Year(), dateTo.Month(), dateTo.Day(), 23, 59, 59, int(time.Second-time.Nanosecond), dateTo.Location())
			dateTo = &end
		}
	}

	// Get album ID from filter if provided
	var albumIDPtr *int32
	if filter.AlbumID != nil {
		id := int32(*filter.AlbumID)
		albumIDPtr = &id
	}

	// Build unified query params
	params := service.QueryAssetsParams{
		Query:        req.Query,
		SearchType:   req.SearchType,
		RepositoryID: filter.RepositoryID,
		AssetType:    filter.Type,
		AssetTypes:   filter.Types,
		OwnerID:      filter.OwnerID,
		AlbumID:      albumIDPtr,
		DateFrom:     dateFrom,
		DateTo:       dateTo,
		IsRaw:        filter.RAW,
		Rating:       filter.Rating,
		Liked:        filter.Liked,
		CameraModel:  filter.CameraMake,
		LensModel:    filter.Lens,
		GroupBy:      req.GroupBy,
		Limit:        req.Pagination.Limit,
		Offset:       req.Pagination.Offset,
	}

	assets, total, err := h.assetService.QueryAssets(ctx, params)
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

	dtos := make([]dto.AssetDTO, len(assets))
	for i, a := range assets {
		dtos[i] = dto.ToAssetDTO(a)
	}

	totalInt := int(total)
	response := dto.AssetListResponseDTO{
		Assets: dtos,
		Total:  &totalInt,
		Limit:  req.Pagination.Limit,
		Offset: req.Pagination.Offset,
	}
	api.GinSuccess(c, response)
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
// @Success 200 {object} api.Result{data=dto.FeaturedAssetsResponseDTO} "Featured photos selected successfully"
// @Failure 400 {object} api.Result "Invalid request parameters"
// @Failure 500 {object} api.Result "Internal server error"
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
	api.GinSuccess(c, response)
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
// @Description Get available camera makes and lenses for filter dropdowns
// @Tags assets
// @Accept json
// @Produce json
// @Success 200 {object} api.Result{data=dto.OptionsResponseDTO} "Filter options retrieved successfully"
// @Failure 500 {object} api.Result "Internal server error"
// @Router /api/v1/assets/filter-options [get]
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

	response := dto.OptionsResponseDTO{
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
// @Param rating body dto.UpdateRatingRequestDTO true "Rating data"
// @Success 200 {object} api.Result{data=dto.MessageResponseDTO} "Rating updated successfully"
// @Failure 400 {object} api.Result "Bad request"
// @Failure 404 {object} api.Result "Asset not found"
// @Failure 500 {object} api.Result "Internal server error"
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

	err = h.assetService.UpdateAssetRating(c.Request.Context(), id, req.Rating)
	if err != nil {
		log.Printf("Failed to update asset rating: %v", err)
		api.GinInternalError(c, err, "Failed to update rating")
		return
	}

	api.GinSuccess(c, dto.MessageResponseDTO{Message: "Rating updated successfully"})
}

// UpdateAssetLike updates the like status of an asset
// @Summary Update asset like status
// @Description Update the like/favorite status of a specific asset
// @Tags assets
// @Accept json
// @Produce json
// @Param id path string true "Asset ID"
// @Param like body dto.UpdateLikeRequestDTO true "Like data"
// @Success 200 {object} api.Result{data=dto.MessageResponseDTO} "Like status updated successfully"
// @Failure 400 {object} api.Result "Bad request"
// @Failure 404 {object} api.Result "Asset not found"
// @Failure 500 {object} api.Result "Internal server error"
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

	err = h.assetService.UpdateAssetLike(c.Request.Context(), id, req.Liked)
	if err != nil {
		log.Printf("Failed to update asset like status: %v", err)
		api.GinInternalError(c, err, "Failed to update like status")
		return
	}

	api.GinSuccess(c, dto.MessageResponseDTO{Message: "Like status updated successfully"})
}

// UpdateAssetRatingAndLike updates both rating and like status of an asset
// @Summary Update asset rating and like status
// @Description Update both the rating (0-5) and like/favorite status of a specific asset
// @Tags assets
// @Accept json
// @Produce json
// @Param id path string true "Asset ID"
// @Param data body dto.UpdateRatingAndLikeRequestDTO true "Rating and like data"
// @Success 200 {object} api.Result{data=dto.MessageResponseDTO} "Rating and like status updated successfully"
// @Failure 400 {object} api.Result "Bad request"
// @Failure 404 {object} api.Result "Asset not found"
// @Failure 500 {object} api.Result "Internal server error"
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

	err = h.assetService.UpdateAssetRatingAndLike(c.Request.Context(), id, req.Rating, req.Liked)
	if err != nil {
		log.Printf("Failed to update asset rating and like status: %v", err)
		api.GinInternalError(c, err, "Failed to update rating and like status")
		return
	}

	api.GinSuccess(c, dto.MessageResponseDTO{Message: "Rating and like status updated successfully"})
}

// UpdateAssetDescription updates the description of an asset
// @Summary Update asset description
// @Description Update the description metadata of an asset
// @Tags assets
// @Accept json
// @Produce json
// @Param id path string true "Asset ID"
// @Param description body dto.UpdateDescriptionRequestDTO true "Description data"
// @Success 200 {object} api.Result{data=dto.MessageResponseDTO} "Description updated successfully"
// @Failure 400 {object} api.Result "Bad request"
// @Failure 404 {object} api.Result "Asset not found"
// @Failure 500 {object} api.Result "Internal server error"
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

	err = h.assetService.UpdateAssetDescription(c.Request.Context(), id, req.Description)
	if err != nil {
		log.Printf("Failed to update asset description: %v", err)
		api.GinInternalError(c, err, "Failed to update description")
		return
	}

	api.GinSuccess(c, dto.MessageResponseDTO{Message: "Description updated successfully"})
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
// @Success 200 {object} api.Result{data=dto.AssetListResponseDTO} "Assets retrieved successfully"
// @Failure 400 {object} api.Result "Bad request"
// @Failure 500 {object} api.Result "Internal server error"
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

	assets, err := h.assetService.GetAssetsByRating(c.Request.Context(), rating, limit, offset)
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
// @Success 200 {object} api.Result{data=dto.AssetListResponseDTO} "Liked assets retrieved successfully"
// @Failure 500 {object} api.Result "Internal server error"
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

	assets, err := h.assetService.GetLikedAssets(ctx, limit, offset)
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

	api.GinSuccess(c, response)
}

// Helper methods for unified chunk upload

// cleanupExpiredSessions periodically cleans up expired upload sessions
func (h *AssetHandler) cleanupExpiredSessions() {
	expiredCount := h.sessionManager.CleanupExpiredSessions()
	if expiredCount > 0 {
		log.Printf("Cleaned up %d expired upload sessions", expiredCount)
	}
}

// startBackgroundCleanupTasks starts all background cleanup tasks
func (h *AssetHandler) startBackgroundCleanupTasks() {
	// Run session cleanup every 5 minutes
	sessionTicker := time.NewTicker(5 * time.Minute)
	defer sessionTicker.Stop()

	// Run orphaned chunk cleanup every 30 minutes
	orphanedChunkTicker := time.NewTicker(30 * time.Minute)
	defer orphanedChunkTicker.Stop()

	log.Println("Starting background cleanup tasks")

	// First cleanup run immediately for both tasks
	h.cleanupExpiredSessions()
	h.cleanupOrphanedChunks()

	// Main loop for cleanup tasks
	for {
		select {
		case <-sessionTicker.C:
			h.cleanupExpiredSessions()
		case <-orphanedChunkTicker.C:
			h.cleanupOrphanedChunks()
		}
	}
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
			os.Remove(stagingFilePath)
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
			os.Remove(stagingFilePath)
			return nil, fmt.Errorf("failed to calculate file hash: %w", err)
		}

		finalHash = hashResult.Hash
		hashMethod = "quick"
		if !hashResult.IsQuick {
			hashMethod = "full"
		}
		log.Printf("Calculated %s hash for %s: %s", hashMethod, header.Filename, finalHash)
	}

	// Detect actual MIME type from file content for chunked uploads
	// Read only the first 512 bytes for efficient detection
	file, err := os.Open(stagingFilePath)
	if err != nil {
		os.Remove(stagingFilePath)
		return nil, fmt.Errorf("failed to open file for MIME detection: %w", err)
	}
	defer file.Close()

	// Read only the first 1024 bytes for efficient MIME type detection
	headerBuf := make([]byte, 1024)
	n, err := file.Read(headerBuf)
	if err != nil && err != io.EOF {
		os.Remove(stagingFilePath)
		return nil, fmt.Errorf("failed to read file header for MIME detection: %w", err)
	}
	headerBuf = headerBuf[:n]

	// Detect MIME type from file header
	detectedMIME := mimetype.Detect(headerBuf)
	detectedType := detectedMIME.String()

	// Use detected MIME type if original one is generic octet-stream
	finalContentType := session.ContentType
	if session.ContentType == "application/octet-stream" || session.ContentType == "" {
		finalContentType = detectedType
	}

	log.Printf("Chunked upload - Original Content-Type: %s, Detected MIME: %s, Final Content-Type: %s",
		session.ContentType, detectedType, finalContentType)

	// Check for hash collision before enqueueing
	collision, err := h.checkHashCollisionBeforeEnqueue(ctx, finalHash, header.Filename, uuid.UUID(repository.RepoID.Bytes).String())
	if err != nil {
		os.Remove(stagingFilePath)
		return nil, fmt.Errorf("failed to check hash collision: %w", err)
	}

	if collision {
		os.Remove(stagingFilePath)
		return &dto.BatchUploadResultDTO{
			Success:     false,
			FileName:    header.Filename,
			ContentHash: finalHash,
			Error:       stringPtr("File with same content already exists in repository"),
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
		os.Remove(stagingFilePath)
		return nil, fmt.Errorf("failed to enqueue task: %w", err)
	}

	if jobResult == nil || jobResult.Job == nil {
		log.Printf("Failed to enqueue task: empty result for file: %s", stagingFilePath)
		os.Remove(stagingFilePath)
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

// checkHashCollisionBeforeEnqueue checks if a file with the same hash already exists
func (h *AssetHandler) checkHashCollisionBeforeEnqueue(ctx context.Context, hash string, filename string, repositoryID string) (bool, error) {
	repoUUID, err := uuid.Parse(repositoryID)
	if err != nil {
		return false, fmt.Errorf("invalid repository ID: %w", err)
	}

	// Check if asset with same hash exists in the repository using the new query
	existing, err := h.queries.GetAssetByHashAndRepository(ctx, repo.GetAssetByHashAndRepositoryParams{
		Hash:         &hash,
		RepositoryID: pgtype.UUID{Bytes: repoUUID, Valid: true},
	})

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// No existing asset found, no collision
			return false, nil
		}
		return false, fmt.Errorf("failed to check hash collision: %w", err)
	}

	// Found existing asset with same hash
	if existing.OriginalFilename != filename {
		log.Printf("Hash collision detected in repository %s: %s (new) vs %s (existing)", repositoryID, filename, existing.OriginalFilename)
		return true, nil
	}
	// Same filename: likely a duplicate upload
	log.Printf("Duplicate upload detected: %s with hash %s", filename, hash)

	return false, nil
}

// ReprocessAsset reprocesses a failed or warning asset
// @Summary Reprocess asset
// @Description Reprocess a failed or warning asset by resetting its status and re-enqueuing for processing
// @Tags assets
// @Accept json
// @Produce json
// @Param id path string true "Asset ID"
// @Param request body dto.ReprocessAssetRequestDTO false "Reprocessing tasks (optional)"
// @Success 200 {object} dto.ReprocessAssetResponseDTO
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v1/assets/{id}/reprocess [post]
func (h *AssetHandler) ReprocessAsset(c *gin.Context) {
	ctx := c.Request.Context()

	// Parse asset ID
	assetIDStr := c.Param("id")
	assetID, err := uuid.Parse(assetIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid asset ID"})
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
			"metadata_asset":  true,
			"thumbnail_asset": true,
			"transcode_asset": true,
			"process_clip":    true,
			"process_ocr":     true,
			"process_caption": true,
			"process_face":    true,
		}

		for _, task := range req.Tasks {
			if !validQueues[task] {
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid queue name: %s", task)})
				return
			}
		}
	}

	// Get the asset to check its current status
	pgUUID := pgtype.UUID{}
	if err := pgUUID.Scan(assetID.String()); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid asset ID format"})
		return
	}

	asset, err := h.queries.GetAssetByID(ctx, pgUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Asset not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get asset"})
		return
	}

	// Parse current status
	var currentStatus status.AssetStatus
	if len(asset.Status) > 0 {
		currentStatus, err = status.FromJSONB(asset.Status)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse asset status"})
			return
		}
	}

	// Check for fatal errors (skip state check to allow retry on any state)
	if currentStatus.HasFatalErrors() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Asset has fatal errors that prevent reprocessing"})
		return
	}

	// Determine retry strategy
	if len(req.Tasks) == 0 || req.ForceFullRetry {
		// Full retry - reset status and enqueue full processing job
		updatedAsset, err := h.queries.ResetAssetStatusForRetry(ctx, pgUUID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reset asset status"})
			return
		}

		// Get repository information
		repository, err := h.queries.GetRepository(ctx, updatedAsset.RepositoryID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get repository"})
			return
		}

		// Check if storage path exists
		if updatedAsset.StoragePath == nil || *updatedAsset.StoragePath == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Asset has no storage path"})
			return
		}

		// Resolve the full path to the asset file
		assetPath := filepath.Join(repository.Path, *updatedAsset.StoragePath)

		// Check if the file exists
		if _, err := os.Stat(assetPath); os.IsNotExist(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Asset file not found"})
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
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to enqueue metadata job"})
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
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to enqueue thumbnail job"})
				return
			}
		case dbtypes.AssetTypeVideo:
			if _, err := h.queueClient.Insert(ctx, jobs.ThumbnailArgs{
				AssetID:     updatedAsset.AssetID,
				RepoPath:    repository.Path,
				StoragePath: storagePath,
				AssetType:   assetType,
			}, &river.InsertOpts{Queue: "thumbnail_asset"}); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to enqueue thumbnail job"})
				return
			}
			if _, err := h.queueClient.Insert(ctx, jobs.TranscodeArgs{
				AssetID:     updatedAsset.AssetID,
				RepoPath:    repository.Path,
				StoragePath: storagePath,
				AssetType:   assetType,
			}, &river.InsertOpts{Queue: "transcode_asset"}); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to enqueue transcode job"})
				return
			}
		case dbtypes.AssetTypeAudio:
			if _, err := h.queueClient.Insert(ctx, jobs.TranscodeArgs{
				AssetID:     updatedAsset.AssetID,
				RepoPath:    repository.Path,
				StoragePath: storagePath,
				AssetType:   assetType,
			}, &river.InsertOpts{Queue: "transcode_asset"}); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to enqueue transcode job"})
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

		c.JSON(http.StatusOK, response)
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
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to enqueue selective retry job"})
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

		c.JSON(http.StatusOK, response)
		return
	}
}
