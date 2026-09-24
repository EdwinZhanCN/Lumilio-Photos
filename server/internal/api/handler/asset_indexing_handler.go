package handler

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"server/internal/api"
	"server/internal/api/dto"
	"server/internal/db/catalogtx"
	"server/internal/db/dbtypes"
	"server/internal/pipeline"
	"server/internal/service"
	"server/internal/workqos"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

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

// GetAssetIndexRebuild reports the state of one queued index rebuild.
// @Summary Get asset index rebuild status
// @Description Report whether a rebuild receipt is pending, completed, or failed. A receipt completes only after every page was requested and every enrichment stage it requested was applied.
// @Tags assets
// @Produce json
// @Param receipt_id path string true "Rebuild receipt ID"
// @Success 200 {object} dto.AssetIndexingRebuildStatusDTO
// @Failure 400 {object} api.ProblemResponse "Invalid receipt ID"
// @Failure 404 {object} api.ProblemResponse "Rebuild receipt not found"
// @Failure 500 {object} api.ProblemResponse "Internal server error"
// @Router /api/v1/assets/indexing/rebuild/{receipt_id} [get]
func (h *AssetHandler) GetAssetIndexRebuild(c *gin.Context) {
	receiptID, err := uuid.Parse(c.Param("receipt_id"))
	if err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}
	status, err := h.indexingService.GetReindexReceipt(c.Request.Context(), receiptID)
	if errors.Is(err, service.ErrReindexReceiptNotFound) {
		api.WriteProblem(c, api.NotFound(err))
		return
	}
	if err != nil {
		api.WriteProblem(c, api.Internal(err))
		return
	}
	api.JSONOK(c, dto.AssetIndexingRebuildStatusDTO{
		ReceiptID:     status.ReceiptID.String(),
		State:         status.State,
		TerminalError: status.TerminalError,
	})
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
	if err := pipeline.RequestAssetStagesTx(ctx, tx.Raw(), asset.AssetID, asset.ContentID, stages, pipeline.AssetPipelineVersion, workqos.Interactive, receiptID); err != nil {
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
