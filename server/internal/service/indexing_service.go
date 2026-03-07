package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"server/config"
	"server/internal/db/repo"
	"server/internal/queue/jobs"
	"server/internal/utils/imaging"
	"server/internal/utils/raw"

	"github.com/h2non/bimg"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
)

type AssetIndexingTask string

const (
	AssetIndexingTaskClip    AssetIndexingTask = "clip"
	AssetIndexingTaskOCR     AssetIndexingTask = "ocr"
	AssetIndexingTaskCaption AssetIndexingTask = "caption"
	AssetIndexingTaskFace    AssetIndexingTask = "face"
)

const defaultIndexingBatchSize = 200
const maxIndexingBatchSize = 500

type AssetIndexingTaskStats struct {
	IndexedCount int64
	QueuedJobs   int64
}

type AssetIndexingStats struct {
	PhotoTotal  int64
	ReindexJobs int64
	Tasks       struct {
		Clip    AssetIndexingTaskStats
		OCR     AssetIndexingTaskStats
		Caption AssetIndexingTaskStats
		Face    AssetIndexingTaskStats
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
	queueClient     *river.Client[pgx.Tx]
	dbpool          *pgxpool.Pool
}

type reindexCandidate struct {
	asset repo.Asset
	tasks map[AssetIndexingTask]bool
}

func NewAssetIndexingService(
	queries *repo.Queries,
	settingsService SettingsService,
	queueClient *river.Client[pgx.Tx],
	dbpool *pgxpool.Pool,
) AssetIndexingService {
	return &assetIndexingService{
		queries:         queries,
		settingsService: settingsService,
		queueClient:     queueClient,
		dbpool:          dbpool,
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
			AssetIndexingTaskClip,
			AssetIndexingTaskOCR,
			AssetIndexingTaskCaption,
			AssetIndexingTaskFace,
		}
	}

	seen := make(map[AssetIndexingTask]bool, len(tasks))
	result := make([]AssetIndexingTask, 0, len(tasks))
	for _, task := range tasks {
		switch task {
		case AssetIndexingTaskClip, AssetIndexingTaskOCR, AssetIndexingTaskCaption, AssetIndexingTaskFace:
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

	stats.Tasks.Clip.IndexedCount, err = s.queries.CountPhotoAssetsWithEmbeddingType(ctx, repo.CountPhotoAssetsWithEmbeddingTypeParams{
		RepositoryID:  repositoryUUID,
		EmbeddingType: string(EmbeddingTypeCLIP),
	})
	if err != nil {
		return AssetIndexingStats{}, fmt.Errorf("count clip coverage: %w", err)
	}

	stats.Tasks.OCR.IndexedCount, err = s.queries.CountPhotoAssetsWithOCRResults(ctx, repositoryUUID)
	if err != nil {
		return AssetIndexingStats{}, fmt.Errorf("count ocr coverage: %w", err)
	}

	stats.Tasks.Caption.IndexedCount, err = s.queries.CountPhotoAssetsWithCaptions(ctx, repositoryUUID)
	if err != nil {
		return AssetIndexingStats{}, fmt.Errorf("count caption coverage: %w", err)
	}

	stats.Tasks.Face.IndexedCount, err = s.queries.CountPhotoAssetsWithFaceResults(ctx, repositoryUUID)
	if err != nil {
		return AssetIndexingStats{}, fmt.Errorf("count face coverage: %w", err)
	}

	stats.Tasks.Clip.QueuedJobs = s.countPendingQueueJobs(ctx, "process_clip")
	stats.Tasks.OCR.QueuedJobs = s.countPendingQueueJobs(ctx, "process_ocr")
	stats.Tasks.Caption.QueuedJobs = s.countPendingQueueJobs(ctx, "process_caption")
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
		log.Printf("reindex: no enabled tasks for request %v", requestedTasks)
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
		return nil
	}

	repositoryCache := make(map[string]repo.Repository)
	queuedJobs := 0
	failedAssets := 0

	for _, candidate := range candidates {
		queued, enqueueErr := s.enqueueAssetIndexingTasks(ctx, candidate, repositoryCache)
		if enqueueErr != nil {
			failedAssets++
			log.Printf("reindex: failed to queue asset %s: %v", candidate.asset.AssetID.String(), enqueueErr)
			continue
		}
		queuedJobs += queued
	}

	if queuedJobs == 0 && failedAssets > 0 {
		return fmt.Errorf("failed to queue %d assets for reindex", failedAssets)
	}

	log.Printf("reindex: queued %d jobs across %d assets (failed assets: %d)", queuedJobs, len(candidates), failedAssets)
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
	case AssetIndexingTaskClip:
		return s.queries.ListPhotoAssetsMissingEmbeddingType(ctx, repo.ListPhotoAssetsMissingEmbeddingTypeParams{
			RepositoryID:  repositoryUUID,
			EmbeddingType: string(EmbeddingTypeCLIP),
			Limit:         int32(limit),
			Offset:        0,
		})
	case AssetIndexingTaskOCR:
		return s.queries.ListPhotoAssetsMissingOCRResults(ctx, repo.ListPhotoAssetsMissingOCRResultsParams{
			RepositoryID: repositoryUUID,
			Limit:        int32(limit),
			Offset:       0,
		})
	case AssetIndexingTaskCaption:
		return s.queries.ListPhotoAssetsMissingCaptions(ctx, repo.ListPhotoAssetsMissingCaptionsParams{
			RepositoryID: repositoryUUID,
			Limit:        int32(limit),
			Offset:       0,
		})
	case AssetIndexingTaskFace:
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

	previewData, err := extractRAWPreviewForIndexing(ctx, fullPath, candidate.asset.OriginalFilename)
	if err != nil {
		return 0, fmt.Errorf("extract RAW preview: %w", err)
	}

	readSource := func() (io.ReadCloser, error) {
		if previewData != nil {
			return io.NopCloser(bytes.NewReader(previewData)), nil
		}
		f, err := os.Open(fullPath)
		if err != nil {
			return nil, err
		}
		return f, nil
	}

	queued := 0
	if candidate.tasks[AssetIndexingTaskClip] {
		if err := s.enqueueClipTask(ctx, candidate.asset.AssetID, readSource); err != nil {
			return queued, err
		}
		queued++
	}
	if candidate.tasks[AssetIndexingTaskOCR] {
		if err := s.enqueueOCRTask(ctx, candidate.asset.AssetID, readSource); err != nil {
			return queued, err
		}
		queued++
	}
	if candidate.tasks[AssetIndexingTaskCaption] {
		if err := s.enqueueCaptionTask(ctx, candidate.asset.AssetID, readSource); err != nil {
			return queued, err
		}
		queued++
	}
	if candidate.tasks[AssetIndexingTaskFace] {
		if err := s.enqueueFaceTask(ctx, candidate.asset.AssetID, readSource); err != nil {
			return queued, err
		}
		queued++
	}

	return queued, nil
}

func (s *assetIndexingService) enqueueClipTask(
	ctx context.Context,
	assetID pgtype.UUID,
	readSource func() (io.ReadCloser, error),
) error {
	reader, err := readSource()
	if err != nil {
		return fmt.Errorf("open asset for CLIP: %w", err)
	}
	defer reader.Close()

	imageData, err := imaging.ProcessImageStream(reader, bimg.Options{
		Width:     224,
		Height:    224,
		Crop:      true,
		Gravity:   bimg.GravitySmart,
		Quality:   90,
		Type:      bimg.WEBP,
		NoProfile: true,
	})
	if err != nil {
		return fmt.Errorf("CLIP preprocessing: %w", err)
	}

	_, err = s.queueClient.Insert(ctx, jobs.ProcessClipArgs{
		AssetID:   assetID,
		ImageData: imageData,
	}, &river.InsertOpts{Queue: "process_clip"})
	if err != nil {
		return fmt.Errorf("enqueue CLIP job: %w", err)
	}
	return nil
}

func (s *assetIndexingService) enqueueOCRTask(
	ctx context.Context,
	assetID pgtype.UUID,
	readSource func() (io.ReadCloser, error),
) error {
	reader, err := readSource()
	if err != nil {
		return fmt.Errorf("open asset for OCR: %w", err)
	}
	defer reader.Close()

	imageData, err := imaging.ProcessImageStream(reader, bimg.Options{
		Width:     1920,
		Height:    1920,
		Quality:   90,
		Type:      bimg.JPEG,
		NoProfile: true,
	})
	if err != nil {
		return fmt.Errorf("OCR preprocessing: %w", err)
	}

	_, err = s.queueClient.Insert(ctx, jobs.ProcessOcrArgs{
		AssetID:   assetID,
		ImageData: imageData,
	}, &river.InsertOpts{Queue: "process_ocr"})
	if err != nil {
		return fmt.Errorf("enqueue OCR job: %w", err)
	}
	return nil
}

func (s *assetIndexingService) enqueueCaptionTask(
	ctx context.Context,
	assetID pgtype.UUID,
	readSource func() (io.ReadCloser, error),
) error {
	reader, err := readSource()
	if err != nil {
		return fmt.Errorf("open asset for caption: %w", err)
	}
	defer reader.Close()

	imageData, err := imaging.ProcessImageStream(reader, bimg.Options{
		Width:     1024,
		Height:    1024,
		Quality:   85,
		Type:      bimg.JPEG,
		NoProfile: true,
	})
	if err != nil {
		return fmt.Errorf("caption preprocessing: %w", err)
	}

	_, err = s.queueClient.Insert(ctx, jobs.ProcessCaptionArgs{
		AssetID:   assetID,
		ImageData: imageData,
	}, &river.InsertOpts{Queue: "process_caption"})
	if err != nil {
		return fmt.Errorf("enqueue caption job: %w", err)
	}
	return nil
}

func (s *assetIndexingService) enqueueFaceTask(
	ctx context.Context,
	assetID pgtype.UUID,
	readSource func() (io.ReadCloser, error),
) error {
	reader, err := readSource()
	if err != nil {
		return fmt.Errorf("open asset for face: %w", err)
	}
	defer reader.Close()

	imageData, err := imaging.ProcessImageStream(reader, bimg.Options{
		Width:     1920,
		Height:    1920,
		Quality:   90,
		Type:      bimg.JPEG,
		NoProfile: true,
	})
	if err != nil {
		return fmt.Errorf("face preprocessing: %w", err)
	}

	_, err = s.queueClient.Insert(ctx, jobs.ProcessFaceArgs{
		AssetID:   assetID,
		ImageData: imageData,
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
		log.Printf("indexing stats: count queue %s failed: %v", queueName, err)
		return 0
	}
	return count
}

func filterEnabledIndexingTasks(tasks []AssetIndexingTask, cfg config.MLConfig) []AssetIndexingTask {
	effective := cfg.EffectiveRuntimeConfig()
	enabled := make([]AssetIndexingTask, 0, len(tasks))

	for _, task := range tasks {
		switch task {
		case AssetIndexingTaskClip:
			if effective.CLIPEnabled {
				enabled = append(enabled, task)
			}
		case AssetIndexingTaskOCR:
			if effective.OCREnabled {
				enabled = append(enabled, task)
			}
		case AssetIndexingTaskCaption:
			if effective.CaptionEnabled {
				enabled = append(enabled, task)
			}
		case AssetIndexingTaskFace:
			if effective.FaceEnabled {
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

func extractRAWPreviewForIndexing(ctx context.Context, fullPath string, originalFilename string) ([]byte, error) {
	if !raw.IsRAWFile(originalFilename) {
		return nil, nil
	}

	f, err := os.Open(fullPath)
	if err != nil {
		return nil, fmt.Errorf("open RAW file: %w", err)
	}
	defer f.Close()

	opts := raw.DefaultProcessingOptions()
	opts.FullRenderTimeout = 30 * time.Second
	opts.PreferEmbedded = true
	opts.Quality = 90

	processor := raw.NewProcessor(opts)
	result, err := processor.ProcessRAW(ctx, f, originalFilename)
	if err != nil {
		return nil, fmt.Errorf("process RAW: %w", err)
	}
	if !result.IsRAW || len(result.PreviewData) == 0 {
		return nil, nil
	}
	return result.PreviewData, nil
}
