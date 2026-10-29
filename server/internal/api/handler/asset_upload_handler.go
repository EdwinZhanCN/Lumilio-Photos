package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"path"
	"path/filepath"
	"server/internal/api"
	"server/internal/api/dto"
	"server/internal/api/problem"
	"server/internal/db/catalogtx"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/pipeline"
	"server/internal/storage"
	filevalidator "server/internal/utils/file"
	"server/internal/utils/hash"
	"server/internal/utils/memory"
	"server/internal/utils/upload"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

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
		if err := storage.CheckUploadAdmission(repository, storage.WriteFacts{}); err != nil {
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
	if err := storage.CheckUploadAdmission(repository, storage.WriteFacts{}); err != nil {
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
	if err := pipeline.RequestIngestTx(ctx, tx.Raw(), commitID, receiptID); err != nil {
		return uuid.Nil, err
	}
	if err := tx.Commit(); err != nil {
		return uuid.Nil, err
	}
	return receiptID, nil
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
	if !h.guardUploadWriteCapacity(c, repository, uint64(max(header.Size, 0))) {
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
		FullHashes:   precheckJSONHashes(contentHashes),
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
		QuickFingerprints: precheckJSONHashes(quickFingerprints),
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
	if !h.guardUploadWriteCapacity(c, repository, uint64(req.TotalSize)) {
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

func (h *AssetHandler) handleUploadFailureFile(repository repo.Repository, stagingFile *storage.StagingFile, reason string) {
	if stagingFile == nil || strings.TrimSpace(stagingFile.PrivatePath) == "" {
		return
	}
	if err := h.stagingManager.MoveStagingToFailed(repository, stagingFile); err != nil {
		log.Printf("Failed to quarantine upload file %s (%s): %v", stagingFile.PrivatePath, reason, err)
	}
}

// Precheck collections are required membership sets, not optional filters.
func precheckJSONHashes(values []string) string {
	encoded := dbtypes.StringsJSONParam(values)
	if encoded == nil {
		return "[]"
	}
	return *encoded
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
