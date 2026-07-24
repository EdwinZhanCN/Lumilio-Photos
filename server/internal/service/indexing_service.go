package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"server/internal/db/repo"
	"server/internal/logging"
	"server/internal/queue/jobs"
	"server/internal/settings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"go.uber.org/zap"
)

type AssetIndexingTask string

const (
	AssetIndexingTaskSemanticImage   AssetIndexingTask = "semantic"
	AssetIndexingTaskBioCLIP         AssetIndexingTask = "bioclip"
	AssetIndexingTaskOCR             AssetIndexingTask = "ocr"
	AssetIndexingTaskFaceRecognition AssetIndexingTask = "face"
	AssetIndexingTaskVideoSemantic   AssetIndexingTask = "video_semantic"
)

const defaultIndexingBatchSize = 200
const maxIndexingBatchSize = 500

type AssetIndexingTaskStats struct {
	IndexedCount int64
	QueuedJobs   int64
	TotalCount   int64
}

type AssetIndexingStats struct {
	PhotoTotal  int64
	VideoTotal  int64
	ReindexJobs int64
	Tasks       struct {
		Semantic      AssetIndexingTaskStats
		BioCLIP       AssetIndexingTaskStats
		OCR           AssetIndexingTaskStats
		Face          AssetIndexingTaskStats
		VideoSemantic AssetIndexingTaskStats
	}
}

type ReindexAssetsInput struct {
	RepositoryID *string
	Tasks        []AssetIndexingTask
	Limit        int
	Offset       int
	MissingOnly  bool
	// ResetSemantic wipes all semantic vectors and demotes the default embedding
	// space before rebuilding. Used for a model swap (drop+refill). Honored only
	// on the first page (Offset == 0) when the semantic task is enabled.
	ResetSemantic bool
}

func containsIndexingTask(tasks []AssetIndexingTask, target AssetIndexingTask) bool {
	for _, t := range tasks {
		if t == target {
			return true
		}
	}
	return false
}

type ReindexAssetsJobResult struct {
	JobID        int64
	Requested    []AssetIndexingTask
	Disabled     []AssetIndexingTask
	Limit        int
	MissingOnly  bool
	RepositoryID *string
}

type AssetIndexingService interface {
	GetIndexingStats(ctx context.Context, repositoryID *string) (AssetIndexingStats, error)
	EnqueueReindexAssets(ctx context.Context, input ReindexAssetsInput) (ReindexAssetsJobResult, error)
	ProcessReindexAssets(ctx context.Context, input ReindexAssetsInput) error
}

type assetIndexingService struct {
	queries         *repo.Queries
	settingsService SettingsService
	runtimeChecker  LumenService
	queueClient     *river.Client[pgx.Tx]
	dbpool          *pgxpool.Pool
	logger          *zap.Logger
	auditProvider   logging.RepositoryAuditProvider
}

type reindexCandidate struct {
	asset repo.Asset
	tasks map[AssetIndexingTask]bool
}

func NewAssetIndexingService(
	queries *repo.Queries,
	settingsService SettingsService,
	runtimeChecker LumenService,
	queueClient *river.Client[pgx.Tx],
	dbpool *pgxpool.Pool,
	logger *zap.Logger,
	auditProvider logging.RepositoryAuditProvider,
) AssetIndexingService {
	if logger == nil {
		logger = zap.NewNop()
	}
	if auditProvider == nil {
		auditProvider = logging.NewRepositoryAuditProvider(logger, false)
	}
	return &assetIndexingService{
		queries:         queries,
		settingsService: settingsService,
		runtimeChecker:  runtimeChecker,
		queueClient:     queueClient,
		dbpool:          dbpool,
		logger:          logger.With(zap.String("component", "indexing")),
		auditProvider:   auditProvider,
	}
}

func normalizeReindexAssetsInput(input ReindexAssetsInput) ReindexAssetsInput {
	if input.Limit <= 0 {
		input.Limit = defaultIndexingBatchSize
	}
	if input.Limit > maxIndexingBatchSize {
		input.Limit = maxIndexingBatchSize
	}
	if input.Offset < 0 {
		input.Offset = 0
	}
	if input.Tasks == nil {
		input.Tasks = []AssetIndexingTask{}
	}
	if !input.MissingOnly {
		input.MissingOnly = false
	}
	return input
}

func normalizeRequestedIndexingTasks(tasks []AssetIndexingTask) []AssetIndexingTask {
	if len(tasks) == 0 {
		return []AssetIndexingTask{
			AssetIndexingTaskSemanticImage,
			AssetIndexingTaskOCR,
			AssetIndexingTaskFaceRecognition,
			AssetIndexingTaskVideoSemantic,
		}
	}

	seen := make(map[AssetIndexingTask]bool, len(tasks))
	result := make([]AssetIndexingTask, 0, len(tasks))
	for _, task := range tasks {
		switch task {
		case AssetIndexingTaskSemanticImage, AssetIndexingTaskOCR, AssetIndexingTaskFaceRecognition, AssetIndexingTaskVideoSemantic:
			if seen[task] {
				continue
			}
			seen[task] = true
			result = append(result, task)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i] < result[j]
	})
	return result
}

func parseRepositoryUUID(repositoryID *string) (pgtype.UUID, error) {
	if repositoryID == nil || strings.TrimSpace(*repositoryID) == "" {
		return pgtype.UUID{}, nil
	}

	var pgUUID pgtype.UUID
	if err := pgUUID.Scan(strings.TrimSpace(*repositoryID)); err != nil {
		return pgtype.UUID{}, fmt.Errorf("invalid repository ID: %w", err)
	}
	return pgUUID, nil
}

func (s *assetIndexingService) GetIndexingStats(ctx context.Context, repositoryID *string) (AssetIndexingStats, error) {
	repositoryUUID, err := parseRepositoryUUID(repositoryID)
	if err != nil {
		return AssetIndexingStats{}, err
	}

	stats := AssetIndexingStats{}

	stats.PhotoTotal, err = s.queries.CountPhotoAssetsForIndexing(ctx, repositoryUUID)
	if err != nil {
		return AssetIndexingStats{}, fmt.Errorf("count photo assets: %w", err)
	}
	stats.VideoTotal, err = s.queries.CountVideoAssetsForIndexing(ctx, repositoryUUID)
	if err != nil {
		return AssetIndexingStats{}, fmt.Errorf("count video assets: %w", err)
	}
	stats.Tasks.Semantic.TotalCount = stats.PhotoTotal
	stats.Tasks.OCR.TotalCount = stats.PhotoTotal
	stats.Tasks.Face.TotalCount = stats.PhotoTotal
	stats.Tasks.VideoSemantic.TotalCount = stats.VideoTotal

	stats.Tasks.Semantic.IndexedCount, err = s.queries.CountPhotoAssetsWithSemanticEmbedding(ctx, repositoryUUID)
	if err != nil {
		return AssetIndexingStats{}, fmt.Errorf("count semantic coverage: %w", err)
	}

	stats.Tasks.BioCLIP.TotalCount, err = s.queries.CountBioAlbumPhotoAssets(ctx, repositoryUUID)
	if err != nil {
		return AssetIndexingStats{}, fmt.Errorf("count bio album photos: %w", err)
	}

	stats.Tasks.BioCLIP.IndexedCount, err = s.queries.CountBioAlbumPhotoAssetsWithSpeciesPredictions(ctx, repositoryUUID)
	if err != nil {
		return AssetIndexingStats{}, fmt.Errorf("count bioclip coverage: %w", err)
	}

	stats.Tasks.OCR.IndexedCount, err = s.queries.CountPhotoAssetsWithOCRResults(ctx, repositoryUUID)
	if err != nil {
		return AssetIndexingStats{}, fmt.Errorf("count ocr coverage: %w", err)
	}

	stats.Tasks.Face.IndexedCount, err = s.queries.CountPhotoAssetsWithFaceResults(ctx, repositoryUUID)
	if err != nil {
		return AssetIndexingStats{}, fmt.Errorf("count face coverage: %w", err)
	}

	stats.Tasks.VideoSemantic.IndexedCount, err = s.queries.CountVideoAssetsWithSemanticFrames(ctx, repositoryUUID)
	if err != nil {
		return AssetIndexingStats{}, fmt.Errorf("count video semantic coverage: %w", err)
	}

	stats.Tasks.Semantic.QueuedJobs = s.countPendingQueueJobs(ctx, "process_semantic")
	stats.Tasks.BioCLIP.QueuedJobs = s.countPendingQueueJobs(ctx, "process_bioclip")
	stats.Tasks.OCR.QueuedJobs = s.countPendingQueueJobs(ctx, "process_ocr")
	stats.Tasks.Face.QueuedJobs = s.countPendingQueueJobs(ctx, "process_face")
	stats.Tasks.VideoSemantic.QueuedJobs = s.countPendingQueueJobs(ctx, "process_video_frames")
	stats.ReindexJobs = s.countPendingQueueJobs(ctx, "reindex_assets")

	return stats, nil
}

func (s *assetIndexingService) EnqueueReindexAssets(ctx context.Context, input ReindexAssetsInput) (ReindexAssetsJobResult, error) {
	if s.queueClient == nil {
		return ReindexAssetsJobResult{}, errors.New("queue client is not configured")
	}

	input = normalizeReindexAssetsInput(input)
	requestedTasks := normalizeRequestedIndexingTasks(input.Tasks)
	if len(requestedTasks) == 0 {
		return ReindexAssetsJobResult{}, errors.New("no valid indexing tasks requested")
	}

	effectiveConfig, err := s.settingsService.GetEffectiveMLConfig(ctx)
	if err != nil {
		return ReindexAssetsJobResult{}, fmt.Errorf("load ML settings: %w", err)
	}

	enabledTasks := filterEnabledIndexingTasks(requestedTasks, effectiveConfig)
	disabledTasks := computeDisabledIndexingTasks(requestedTasks, enabledTasks)

	if len(enabledTasks) == 0 {
		return ReindexAssetsJobResult{
			Disabled:     disabledTasks,
			Limit:        input.Limit,
			MissingOnly:  input.MissingOnly,
			RepositoryID: input.RepositoryID,
		}, nil
	}

	jobResult, err := s.queueClient.Insert(ctx, jobs.ReindexAssetsArgs{
		RepositoryID:  input.RepositoryID,
		Tasks:         indexingTasksToStrings(enabledTasks),
		Limit:         input.Limit,
		MissingOnly:   input.MissingOnly,
		ResetSemantic: input.ResetSemantic,
	}, &river.InsertOpts{Queue: "reindex_assets"})
	if err != nil {
		return ReindexAssetsJobResult{}, fmt.Errorf("enqueue reindex job: %w", err)
	}

	return ReindexAssetsJobResult{
		JobID:        jobResult.Job.ID,
		Requested:    enabledTasks,
		Disabled:     disabledTasks,
		Limit:        input.Limit,
		MissingOnly:  input.MissingOnly,
		RepositoryID: input.RepositoryID,
	}, nil
}

func (s *assetIndexingService) ProcessReindexAssets(ctx context.Context, input ReindexAssetsInput) error {
	input = normalizeReindexAssetsInput(input)
	requestedTasks := normalizeRequestedIndexingTasks(input.Tasks)
	if len(requestedTasks) == 0 {
		return nil
	}

	effectiveConfig, err := s.settingsService.GetEffectiveMLConfig(ctx)
	if err != nil {
		return fmt.Errorf("load ML settings: %w", err)
	}

	enabledTasks := filterEnabledIndexingTasks(requestedTasks, effectiveConfig)
	if len(enabledTasks) == 0 {
		s.logger.Info("reindex skipped: no enabled tasks",
			zap.String("operation", "reindex.process"),
			zap.Any("requested_tasks", requestedTasks),
		)
		return nil
	}

	repositoryUUID, err := parseRepositoryUUID(input.RepositoryID)
	if err != nil {
		return err
	}

	// Model-swap reset: on the first page of a semantic rebuild, drop every
	// semantic vector and demote the default space so the next re-embed promotes
	// the newly served model. This is a clean drop+refill — search returns fewer
	// results until the rebuild completes, but never mixes vectors from two
	// models. Guarded to Offset == 0 so paginated continuations do not re-wipe.
	if input.ResetSemantic && input.Offset == 0 && containsIndexingTask(enabledTasks, AssetIndexingTaskSemanticImage) {
		if err := s.queries.DeleteAllSearchEmbeddings(ctx); err != nil {
			return fmt.Errorf("reset semantic index: delete search embeddings: %w", err)
		}
		if err := s.queries.ClearDefaultSearchSpaceByType(ctx, string(EmbeddingTypeSemantic)); err != nil {
			return fmt.Errorf("reset semantic index: clear default space: %w", err)
		}
		// Video frame rows were wiped with the table; ensure videos are
		// re-queued when video semantic indexing is enabled.
		if effectiveConfig.SemanticEnabled && effectiveConfig.VideoSemanticEnabled &&
			!containsIndexingTask(enabledTasks, AssetIndexingTaskVideoSemantic) {
			enabledTasks = append(enabledTasks, AssetIndexingTaskVideoSemantic)
		}
		s.logger.Info("semantic index reset for model swap",
			zap.String("operation", "reindex.process"),
		)
	}

	candidates, err := s.collectReindexCandidates(ctx, repositoryUUID, enabledTasks, input)
	if err != nil {
		return err
	}
	if len(candidates) == 0 {
		s.audit(input.RepositoryID, "").Operation("asset.reindex",
			zap.Any("tasks", enabledTasks),
			zap.Int("limit", input.Limit),
			zap.Bool("missing_only", input.MissingOnly),
			zap.String("result", "no_candidates"),
		)
		return nil
	}

	repositoryCache := make(map[string]repo.Repository)
	queuedJobs := 0
	failedAssets := 0

	for _, candidate := range candidates {
		queued, enqueueErr := s.enqueueAssetIndexingTasks(ctx, candidate, repositoryCache)
		if enqueueErr != nil {
			failedAssets++
			s.logger.Warn("reindex failed to queue asset",
				zap.String("operation", "reindex.process"),
				zap.String("asset_id", candidate.asset.AssetID.String()),
				zap.Error(enqueueErr),
			)
			s.audit(input.RepositoryID, "").Error("asset.reindex", enqueueErr,
				zap.String("asset_id", candidate.asset.AssetID.String()),
			)
			continue
		}
		queuedJobs += queued
	}

	if queuedJobs == 0 && failedAssets > 0 {
		return fmt.Errorf("failed to queue %d assets for reindex", failedAssets)
	}

	s.logger.Info("reindex queued jobs",
		zap.String("operation", "reindex.process"),
		zap.Int("queued_jobs", queuedJobs),
		zap.Int("candidate_assets", len(candidates)),
		zap.Int("failed_assets", failedAssets),
	)
	s.audit(input.RepositoryID, "").Operation("asset.reindex",
		zap.Int("queued_jobs", queuedJobs),
		zap.Int("candidate_assets", len(candidates)),
		zap.Int("failed_assets", failedAssets),
		zap.Any("tasks", enabledTasks),
	)

	// Full rebuilds page through the entire library: a full batch means more
	// assets likely remain, so enqueue the next page (offset += limit). The
	// single-worker reindex_assets queue processes pages serially. Missing-only
	// backfills are intentionally one-shot because their result set shrinks as
	// downstream ML jobs complete, which would make offset pagination skip or
	// reprocess assets; callers re-trigger to make further progress.
	//
	// Mixed photo+video pages may return up to 2*Limit candidates; treat a
	// page as complete when fewer than Limit candidates were collected.
	pageFilled := len(candidates) >= input.Limit
	if !input.MissingOnly && pageFilled {
		if _, err := s.queueClient.Insert(ctx, jobs.ReindexAssetsArgs{
			RepositoryID: input.RepositoryID,
			Tasks:        indexingTasksToStrings(enabledTasks),
			Limit:        input.Limit,
			Offset:       input.Offset + input.Limit,
			MissingOnly:  false,
		}, &river.InsertOpts{Queue: "reindex_assets"}); err != nil {
			s.logger.Warn("reindex failed to enqueue next page",
				zap.String("operation", "reindex.process"),
				zap.Int("next_offset", input.Offset+input.Limit),
				zap.Int("limit", input.Limit),
				zap.Error(err),
			)
			return fmt.Errorf("enqueue reindex next page: %w", err)
		}
		s.logger.Info("reindex enqueued next page",
			zap.String("operation", "reindex.process"),
			zap.Int("next_offset", input.Offset+input.Limit),
			zap.Int("limit", input.Limit),
		)
	}
	return nil
}

// nextReindexPageOffset computes the offset for a chained full-rebuild page.
// It returns hasMore=false when the batch did not fill (last page reached) or
// when the run is missing-only. Missing-only backfills stay one-shot because
// their candidate set shrinks asynchronously as downstream ML jobs finish, so
// offset pagination would skip or reprocess assets.
func nextReindexPageOffset(missingOnly bool, candidateCount, limit, currentOffset int) (nextOffset int, hasMore bool) {
	if missingOnly || limit <= 0 || candidateCount < limit {
		return 0, false
	}
	return currentOffset + limit, true
}

func (s *assetIndexingService) collectReindexCandidates(
	ctx context.Context,
	repositoryUUID pgtype.UUID,
	tasks []AssetIndexingTask,
	input ReindexAssetsInput,
) ([]reindexCandidate, error) {
	candidateMap := make(map[string]*reindexCandidate)
	orderedIDs := make([]string, 0, input.Limit)

	addCandidate := func(asset repo.Asset, task AssetIndexingTask) {
		assetID := asset.AssetID.String()
		candidate, exists := candidateMap[assetID]
		if !exists {
			if len(orderedIDs) >= input.Limit {
				return
			}
			candidate = &reindexCandidate{
				asset: asset,
				tasks: map[AssetIndexingTask]bool{},
			}
			candidateMap[assetID] = candidate
			orderedIDs = append(orderedIDs, assetID)
		}
		candidate.tasks[task] = true
	}
	addCandidateUnbounded := func(asset repo.Asset, task AssetIndexingTask) {
		assetID := asset.AssetID.String()
		candidate, exists := candidateMap[assetID]
		if !exists {
			candidate = &reindexCandidate{
				asset: asset,
				tasks: map[AssetIndexingTask]bool{},
			}
			candidateMap[assetID] = candidate
			orderedIDs = append(orderedIDs, assetID)
		}
		candidate.tasks[task] = true
	}

	if !input.MissingOnly {
		photoTasks := photoIndexingTasks(tasks)
		videoTasks := videoIndexingTasks(tasks)

		// Photo and video lists each use the page window independently so a
		// mixed rebuild does not starve one asset type under a shared Limit.
		if len(photoTasks) > 0 {
			assets, err := s.queries.ListPhotoAssetsForIndexingBatch(ctx, repo.ListPhotoAssetsForIndexingBatchParams{
				RepositoryID: repositoryUUID,
				Limit:        int32(input.Limit),
				Offset:       int32(input.Offset),
			})
			if err != nil {
				return nil, fmt.Errorf("list photo assets for indexing: %w", err)
			}
			for _, asset := range assets {
				for _, task := range photoTasks {
					addCandidateUnbounded(asset, task)
				}
			}
		}

		if len(videoTasks) > 0 {
			assets, err := s.queries.ListVideoAssetsForIndexingBatch(ctx, repo.ListVideoAssetsForIndexingBatchParams{
				RepositoryID: repositoryUUID,
				Limit:        int32(input.Limit),
				Offset:       int32(input.Offset),
			})
			if err != nil {
				return nil, fmt.Errorf("list video assets for indexing: %w", err)
			}
			for _, asset := range assets {
				for _, task := range videoTasks {
					addCandidateUnbounded(asset, task)
				}
			}
		}
	} else {
		for _, task := range tasks {
			assets, err := s.listMissingAssetsForTask(ctx, repositoryUUID, task, input.Limit)
			if err != nil {
				return nil, err
			}
			for _, asset := range assets {
				addCandidate(asset, task)
			}
		}
	}

	result := make([]reindexCandidate, 0, len(orderedIDs))
	for _, assetID := range orderedIDs {
		candidate := candidateMap[assetID]
		if candidate == nil || len(candidate.tasks) == 0 {
			continue
		}
		result = append(result, *candidate)
	}
	return result, nil
}

func (s *assetIndexingService) listMissingAssetsForTask(
	ctx context.Context,
	repositoryUUID pgtype.UUID,
	task AssetIndexingTask,
	limit int,
) ([]repo.Asset, error) {
	switch task {
	case AssetIndexingTaskSemanticImage:
		return s.queries.ListPhotoAssetsMissingSemanticEmbedding(ctx, repo.ListPhotoAssetsMissingSemanticEmbeddingParams{
			RepositoryID: repositoryUUID,
			Limit:        int32(limit),
			Offset:       0,
		})
	case AssetIndexingTaskOCR:
		return s.queries.ListPhotoAssetsMissingOCRResults(ctx, repo.ListPhotoAssetsMissingOCRResultsParams{
			RepositoryID: repositoryUUID,
			Limit:        int32(limit),
			Offset:       0,
		})
	case AssetIndexingTaskFaceRecognition:
		return s.queries.ListPhotoAssetsMissingFaceResults(ctx, repo.ListPhotoAssetsMissingFaceResultsParams{
			RepositoryID: repositoryUUID,
			Limit:        int32(limit),
			Offset:       0,
		})
	case AssetIndexingTaskVideoSemantic:
		return s.queries.ListVideoAssetsMissingSemanticFrames(ctx, repo.ListVideoAssetsMissingSemanticFramesParams{
			RepositoryID: repositoryUUID,
			Limit:        int32(limit),
			Offset:       0,
		})
	default:
		return nil, fmt.Errorf("unsupported indexing task: %s", task)
	}
}

func (s *assetIndexingService) enqueueAssetIndexingTasks(
	ctx context.Context,
	candidate reindexCandidate,
	repositoryCache map[string]repo.Repository,
) (int, error) {
	if candidate.asset.RepositoryID.Valid == false {
		return 0, errors.New("asset repository is missing")
	}
	if candidate.asset.StoragePath == nil || *candidate.asset.StoragePath == "" {
		return 0, errors.New("asset storage path is missing")
	}

	repositoryID := candidate.asset.RepositoryID.String()
	repository, ok := repositoryCache[repositoryID]
	if !ok {
		var err error
		repository, err = s.queries.GetRepository(ctx, candidate.asset.RepositoryID)
		if err != nil {
			return 0, fmt.Errorf("get repository: %w", err)
		}
		repositoryCache[repositoryID] = repository
	}

	fullPath := filepath.Join(repository.Path, *candidate.asset.StoragePath)
	if _, err := os.Stat(fullPath); err != nil {
		return 0, fmt.Errorf("stat asset file: %w", err)
	}

	queued := 0
	if candidate.tasks[AssetIndexingTaskSemanticImage] {
		inserted, err := s.enqueueSemanticTask(ctx, candidate.asset.AssetID)
		if err != nil {
			return queued, err
		}
		if inserted {
			queued++
		}
	}
	if candidate.tasks[AssetIndexingTaskOCR] {
		inserted, err := s.enqueueOCRTask(ctx, candidate.asset.AssetID)
		if err != nil {
			return queued, err
		}
		if inserted {
			queued++
		}
	}
	if candidate.tasks[AssetIndexingTaskFaceRecognition] {
		inserted, err := s.enqueueFaceTask(ctx, candidate.asset.AssetID)
		if err != nil {
			return queued, err
		}
		if inserted {
			queued++
		}
	}
	if candidate.tasks[AssetIndexingTaskVideoSemantic] {
		inserted, err := s.enqueueVideoFramesTask(ctx, candidate.asset.AssetID)
		if err != nil {
			return queued, err
		}
		if inserted {
			queued++
		}
	}

	return queued, nil
}

func (s *assetIndexingService) enqueueSemanticTask(
	ctx context.Context,
	assetID pgtype.UUID,
) (bool, error) {
	res, err := s.queueClient.Insert(ctx, jobs.ProcessSemanticArgs{
		AssetID:           assetID,
		PreprocessVersion: jobs.MLPreprocessVersionV1,
	}, &river.InsertOpts{Queue: "process_semantic"})
	if err != nil {
		return false, fmt.Errorf("enqueue semantic job: %w", err)
	}
	return !res.UniqueSkippedAsDuplicate, nil
}

func (s *assetIndexingService) enqueueOCRTask(
	ctx context.Context,
	assetID pgtype.UUID,
) (bool, error) {
	res, err := s.queueClient.Insert(ctx, jobs.ProcessOcrArgs{
		AssetID:           assetID,
		PreprocessVersion: jobs.MLPreprocessVersionV1,
	}, &river.InsertOpts{Queue: "process_ocr"})
	if err != nil {
		return false, fmt.Errorf("enqueue OCR job: %w", err)
	}
	return !res.UniqueSkippedAsDuplicate, nil
}

func (s *assetIndexingService) enqueueFaceTask(
	ctx context.Context,
	assetID pgtype.UUID,
) (bool, error) {
	res, err := s.queueClient.Insert(ctx, jobs.ProcessFaceArgs{
		AssetID:           assetID,
		PreprocessVersion: jobs.MLPreprocessVersionV1,
	}, &river.InsertOpts{Queue: "process_face"})
	if err != nil {
		return false, fmt.Errorf("enqueue face job: %w", err)
	}
	return !res.UniqueSkippedAsDuplicate, nil
}

func (s *assetIndexingService) enqueueVideoFramesTask(
	ctx context.Context,
	assetID pgtype.UUID,
) (bool, error) {
	res, err := s.queueClient.Insert(ctx, jobs.ProcessVideoFramesArgs{
		AssetID:           assetID,
		PreprocessVersion: jobs.MLPreprocessVersionV1,
	}, &river.InsertOpts{Queue: "process_video_frames"})
	if err != nil {
		return false, fmt.Errorf("enqueue video frames job: %w", err)
	}
	return !res.UniqueSkippedAsDuplicate, nil
}

func (s *assetIndexingService) countPendingQueueJobs(ctx context.Context, queueName string) int64 {
	if s.dbpool == nil {
		return 0
	}

	const query = `
SELECT COUNT(*)
FROM river_job
WHERE queue = $1
  AND state IN ('available', 'scheduled', 'running', 'retryable')
`

	var count int64
	if err := s.dbpool.QueryRow(ctx, query, queueName).Scan(&count); err != nil {
		s.logger.Warn("indexing stats queue count failed",
			zap.String("operation", "indexing.stats"),
			zap.String("queue", queueName),
			zap.Error(err),
		)
		return 0
	}
	return count
}

func (s *assetIndexingService) audit(repositoryID *string, repoPath string) logging.RepositoryAuditLogger {
	if s.auditProvider == nil {
		return logging.NewRepositoryAuditProvider(s.logger, false).ForPath(repoPath)
	}
	if strings.TrimSpace(repoPath) != "" {
		return s.auditProvider.ForPath(repoPath)
	}
	if repositoryID == nil || strings.TrimSpace(*repositoryID) == "" || s.queries == nil {
		return s.auditProvider.ForPath(repoPath)
	}
	repositoryUUID, err := parseRepositoryUUID(repositoryID)
	if err != nil {
		return s.auditProvider.ForPath(repoPath)
	}
	repository, err := s.queries.GetRepository(context.Background(), repositoryUUID)
	if err != nil {
		return s.auditProvider.ForPath(repoPath)
	}
	return s.auditProvider.ForPath(repository.Path)
}

func computeDisabledIndexingTasks(requested, enabled []AssetIndexingTask) []AssetIndexingTask {
	enabledSet := make(map[AssetIndexingTask]bool, len(enabled))
	for _, t := range enabled {
		enabledSet[t] = true
	}
	disabled := make([]AssetIndexingTask, 0, len(requested))
	for _, t := range requested {
		if !enabledSet[t] {
			disabled = append(disabled, t)
		}
	}
	return disabled
}

func filterEnabledIndexingTasks(tasks []AssetIndexingTask, cfg settings.ML) []AssetIndexingTask {
	enabled := make([]AssetIndexingTask, 0, len(tasks))

	for _, task := range tasks {
		switch task {
		case AssetIndexingTaskSemanticImage:
			if cfg.SemanticEnabled {
				enabled = append(enabled, task)
			}
		case AssetIndexingTaskBioCLIP:
			if cfg.BioCLIPEnabled {
				enabled = append(enabled, task)
			}
		case AssetIndexingTaskOCR:
			if cfg.OCREnabled {
				enabled = append(enabled, task)
			}
		case AssetIndexingTaskFaceRecognition:
			if cfg.FaceEnabled {
				enabled = append(enabled, task)
			}
		case AssetIndexingTaskVideoSemantic:
			if cfg.SemanticEnabled && cfg.VideoSemanticEnabled {
				enabled = append(enabled, task)
			}
		}
	}

	return enabled
}

func photoIndexingTasks(tasks []AssetIndexingTask) []AssetIndexingTask {
	out := make([]AssetIndexingTask, 0, len(tasks))
	for _, task := range tasks {
		switch task {
		case AssetIndexingTaskSemanticImage, AssetIndexingTaskOCR, AssetIndexingTaskFaceRecognition, AssetIndexingTaskBioCLIP:
			out = append(out, task)
		}
	}
	return out
}

func videoIndexingTasks(tasks []AssetIndexingTask) []AssetIndexingTask {
	out := make([]AssetIndexingTask, 0, len(tasks))
	for _, task := range tasks {
		if task == AssetIndexingTaskVideoSemantic {
			out = append(out, task)
		}
	}
	return out
}

func indexingTasksToStrings(tasks []AssetIndexingTask) []string {
	result := make([]string, 0, len(tasks))
	for _, task := range tasks {
		result = append(result, string(task))
	}
	return result
}
