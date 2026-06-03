package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"server/config"
	"server/internal/db/repo"
	"server/internal/logging"
	"server/internal/queue/jobs"

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
	ReindexJobs int64
	Tasks       struct {
		Semantic AssetIndexingTaskStats
		BioCLIP  AssetIndexingTaskStats
		OCR      AssetIndexingTaskStats
		Face     AssetIndexingTaskStats
	}
}

type ReindexAssetsInput struct {
	RepositoryID *string
	Tasks        []AssetIndexingTask
	Limit        int
	MissingOnly  bool
}

type ReindexAssetsJobResult struct {
	JobID        int64
	Requested    []AssetIndexingTask
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
		auditProvider = logging.NewRepositoryAuditProvider(logger)
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
		}
	}

	seen := make(map[AssetIndexingTask]bool, len(tasks))
	result := make([]AssetIndexingTask, 0, len(tasks))
	for _, task := range tasks {
		switch task {
		case AssetIndexingTaskSemanticImage, AssetIndexingTaskOCR, AssetIndexingTaskFaceRecognition:
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
	stats.Tasks.Semantic.TotalCount = stats.PhotoTotal
	stats.Tasks.OCR.TotalCount = stats.PhotoTotal
	stats.Tasks.Face.TotalCount = stats.PhotoTotal

	stats.Tasks.Semantic.IndexedCount, err = s.queries.CountPhotoAssetsWithEmbeddingType(ctx, repo.CountPhotoAssetsWithEmbeddingTypeParams{
		RepositoryID:  repositoryUUID,
		EmbeddingType: string(EmbeddingTypeSemantic),
	})
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

	stats.Tasks.Semantic.QueuedJobs = s.countPendingQueueJobs(ctx, "process_semantic")
	stats.Tasks.BioCLIP.QueuedJobs = s.countPendingQueueJobs(ctx, "process_bioclip")
	stats.Tasks.OCR.QueuedJobs = s.countPendingQueueJobs(ctx, "process_ocr")
	stats.Tasks.Face.QueuedJobs = s.countPendingQueueJobs(ctx, "process_face")
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

	jobResult, err := s.queueClient.Insert(ctx, jobs.ReindexAssetsArgs{
		RepositoryID: input.RepositoryID,
		Tasks:        indexingTasksToStrings(requestedTasks),
		Limit:        input.Limit,
		MissingOnly:  input.MissingOnly,
	}, &river.InsertOpts{Queue: "reindex_assets"})
	if err != nil {
		return ReindexAssetsJobResult{}, fmt.Errorf("enqueue reindex job: %w", err)
	}

	return ReindexAssetsJobResult{
		JobID:        jobResult.Job.ID,
		Requested:    requestedTasks,
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
	return nil
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

	if !input.MissingOnly {
		assets, err := s.queries.ListPhotoAssetsForIndexingBatch(ctx, repo.ListPhotoAssetsForIndexingBatchParams{
			RepositoryID: repositoryUUID,
			Limit:        int32(input.Limit),
			Offset:       0,
		})
		if err != nil {
			return nil, fmt.Errorf("list photo assets for indexing: %w", err)
		}
		for _, asset := range assets {
			for _, task := range tasks {
				addCandidate(asset, task)
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
		return s.queries.ListPhotoAssetsMissingEmbeddingType(ctx, repo.ListPhotoAssetsMissingEmbeddingTypeParams{
			RepositoryID:  repositoryUUID,
			EmbeddingType: string(EmbeddingTypeSemantic),
			Limit:         int32(limit),
			Offset:        0,
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
		if err := s.enqueueSemanticTask(ctx, candidate.asset.AssetID); err != nil {
			return queued, err
		}
		queued++
	}
	if candidate.tasks[AssetIndexingTaskOCR] {
		if err := s.enqueueOCRTask(ctx, candidate.asset.AssetID); err != nil {
			return queued, err
		}
		queued++
	}
	if candidate.tasks[AssetIndexingTaskFaceRecognition] {
		if err := s.enqueueFaceTask(ctx, candidate.asset.AssetID); err != nil {
			return queued, err
		}
		queued++
	}

	return queued, nil
}

func (s *assetIndexingService) enqueueSemanticTask(
	ctx context.Context,
	assetID pgtype.UUID,
) error {
	_, err := s.queueClient.Insert(ctx, jobs.ProcessSemanticArgs{
		AssetID:           assetID,
		PreprocessVersion: jobs.MLPreprocessVersionV1,
	}, &river.InsertOpts{Queue: "process_semantic"})
	if err != nil {
		return fmt.Errorf("enqueue semantic job: %w", err)
	}
	return nil
}

func (s *assetIndexingService) enqueueOCRTask(
	ctx context.Context,
	assetID pgtype.UUID,
) error {
	_, err := s.queueClient.Insert(ctx, jobs.ProcessOcrArgs{
		AssetID:           assetID,
		PreprocessVersion: jobs.MLPreprocessVersionV1,
	}, &river.InsertOpts{Queue: "process_ocr"})
	if err != nil {
		return fmt.Errorf("enqueue OCR job: %w", err)
	}
	return nil
}

func (s *assetIndexingService) enqueueFaceTask(
	ctx context.Context,
	assetID pgtype.UUID,
) error {
	_, err := s.queueClient.Insert(ctx, jobs.ProcessFaceArgs{
		AssetID:           assetID,
		PreprocessVersion: jobs.MLPreprocessVersionV1,
	}, &river.InsertOpts{Queue: "process_face"})
	if err != nil {
		return fmt.Errorf("enqueue face job: %w", err)
	}
	return nil
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
		return logging.NewRepositoryAuditProvider(s.logger).ForPath(repoPath)
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

func filterEnabledIndexingTasks(tasks []AssetIndexingTask, cfg config.MLConfig) []AssetIndexingTask {
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
		}
	}

	return enabled
}

func indexingTasksToStrings(tasks []AssetIndexingTask) []string {
	result := make([]string, 0, len(tasks))
	for _, task := range tasks {
		result = append(result, string(task))
	}
	return result
}
