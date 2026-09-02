package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"server/internal/db/catalogtx"
	"server/internal/db/repo"
	"server/internal/logging"
	"server/internal/pipeline"
	"server/internal/settings"
	"server/internal/storage"
	"server/internal/workqos"

	"github.com/google/uuid"
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
	ReceiptID    uuid.UUID
	Requested    []AssetIndexingTask
	Disabled     []AssetIndexingTask
	Limit        int
	MissingOnly  bool
	RepositoryID *string
}

type AssetIndexingService interface {
	GetIndexingStats(ctx context.Context, repositoryID *string) (AssetIndexingStats, error)
	EnqueueReindexAssets(ctx context.Context, input ReindexAssetsInput) (ReindexAssetsJobResult, error)
	// ProcessReindexReceipt advances one bounded page. The boolean reports that
	// another page was scheduled in the same catalog transaction, allowing the
	// queue macro to snooze without publishing a completion receipt early.
	ProcessReindexReceipt(ctx context.Context, receiptID uuid.UUID, expectedRevision uint64) (more bool, err error)
	PrepareReindexReceipt(ctx context.Context, receiptID uuid.UUID, expectedRevision uint64) (PreparedReindex, error)
	ApplyPreparedReindexTx(ctx context.Context, tx *sql.Tx, prepared PreparedReindex) error
}

var ErrReindexProjectionStale = errors.New("reindex projection source revision changed")
var ErrSemanticResetRequiresGlobalScope = errors.New("semantic reset cannot be scoped to one repository")

type PreparedReindexCandidate struct {
	AssetID   uuid.UUID
	ContentID uuid.UUID
}

// PreparedReindex is the immutable bounded page computed by a rebuild macro.
// Its catalog effects (stage requests, cursor advancement, and receipt
// completion) are applied together by the commit coordinator.
type PreparedReindex struct {
	ReceiptID         uuid.UUID
	RequestedRevision uint64
	Candidates        []PreparedReindexCandidate
	RepositoryID      *uuid.UUID
	Tasks             []string
	Limit             int
	Cursor            string
	MissingOnly       bool
	HasMore           bool
	NextCursor        string
	ResetSemantic     bool
}

type assetIndexingService struct {
	queries         *repo.Queries
	settingsService SettingsService
	runtimeChecker  LumenService
	writer          *catalogtx.Writer
	readerPool      *sql.DB
	logger          *zap.Logger
	auditProvider   logging.RepositoryAuditProvider
	files           *storage.RepositoryFSFactory
}

type reindexCandidate struct {
	asset repo.Asset
	tasks map[AssetIndexingTask]bool
}

func NewAssetIndexingService(
	queries *repo.Queries,
	settingsService SettingsService,
	runtimeChecker LumenService,
	dbpool *sql.DB,
	logger *zap.Logger,
	auditProvider logging.RepositoryAuditProvider,
	files *storage.RepositoryFSFactory,
) AssetIndexingService {
	return NewAssetIndexingServiceWithReader(
		queries,
		settingsService,
		runtimeChecker,
		catalogtx.NewWriter(dbpool, nil),
		dbpool,
		logger,
		auditProvider,
		files,
	)
}

func NewAssetIndexingServiceWithReader(
	queries *repo.Queries,
	settingsService SettingsService,
	runtimeChecker LumenService,
	writer *catalogtx.Writer,
	readerPool *sql.DB,
	logger *zap.Logger,
	auditProvider logging.RepositoryAuditProvider,
	files *storage.RepositoryFSFactory,
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
		writer:          writer,
		readerPool:      readerPool,
		logger:          logger.With(zap.String("component", "indexing")),
		auditProvider:   auditProvider,
		files:           files,
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

func expandSemanticResetTasks(tasks []AssetIndexingTask) []AssetIndexingTask {
	if !containsIndexingTask(tasks, AssetIndexingTaskSemanticImage) || containsIndexingTask(tasks, AssetIndexingTaskVideoSemantic) {
		return tasks
	}
	tasks = append(append([]AssetIndexingTask(nil), tasks...), AssetIndexingTaskVideoSemantic)
	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i] < tasks[j]
	})
	return tasks
}

func parseRepositoryUUID(repositoryID *string) (uuid.NullUUID, error) {
	if repositoryID == nil || strings.TrimSpace(*repositoryID) == "" {
		return uuid.NullUUID{}, nil
	}

	parsed, err := uuid.Parse(strings.TrimSpace(*repositoryID))
	if err != nil {
		return uuid.NullUUID{}, fmt.Errorf("invalid repository ID: %w", err)
	}
	return uuid.NullUUID{UUID: parsed, Valid: true}, nil
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

	// QueueDB is disposable control state, so backlog reporting is derived from
	// catalog desired/applied rows rather than River job states. Enrichment is a
	// single macro boundary; the same pending asset count is exposed for each
	// optional coverage lane for backwards-compatible response shape.
	pendingAssets := s.countPendingCatalogEnrichment(ctx)
	stats.Tasks.Semantic.QueuedJobs = pendingAssets
	stats.Tasks.BioCLIP.QueuedJobs = pendingAssets
	stats.Tasks.OCR.QueuedJobs = pendingAssets
	stats.Tasks.Face.QueuedJobs = pendingAssets
	stats.Tasks.VideoSemantic.QueuedJobs = pendingAssets
	stats.ReindexJobs = s.countPendingReindexReceipts(ctx)

	return stats, nil
}

func (s *assetIndexingService) EnqueueReindexAssets(ctx context.Context, input ReindexAssetsInput) (ReindexAssetsJobResult, error) {
	if s.writer == nil {
		return ReindexAssetsJobResult{}, errors.New("catalog writer is not configured")
	}

	input = normalizeReindexAssetsInput(input)
	requestedTasks := normalizeRequestedIndexingTasks(input.Tasks)
	if input.ResetSemantic {
		if input.RepositoryID != nil {
			return ReindexAssetsJobResult{}, ErrSemanticResetRequiresGlobalScope
		}
		// search_embeddings is one authoritative photo-and-video search index.
		// A model reset must refill both semantic lanes after deleting it; rebuilding
		// only photo embeddings would leave video frame coverage permanently absent.
		requestedTasks = expandSemanticResetTasks(requestedTasks)
		input.MissingOnly = false
	}
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

	receiptID := uuid.New()
	var repositoryID *uuid.UUID
	if input.RepositoryID != nil {
		parsed, err := uuid.Parse(*input.RepositoryID)
		if err != nil {
			return ReindexAssetsJobResult{}, err
		}
		repositoryID = &parsed
	}
	err = s.writer.Transact(ctx, catalogtx.OperationAssetReindexRequest, nil, func(tx *sql.Tx) error {
		return pipeline.RequestReindexTx(ctx, tx, receiptID, repositoryID, indexingTasksToStrings(enabledTasks), input.Limit, input.MissingOnly, input.ResetSemantic)
	})
	if err != nil {
		return ReindexAssetsJobResult{}, fmt.Errorf("request reindex: %w", err)
	}

	return ReindexAssetsJobResult{
		ReceiptID:    receiptID,
		Requested:    enabledTasks,
		Disabled:     disabledTasks,
		Limit:        input.Limit,
		MissingOnly:  input.MissingOnly,
		RepositoryID: input.RepositoryID,
	}, nil
}

// offset pagination would skip or reprocess assets.
func (s *assetIndexingService) ProcessReindexReceipt(ctx context.Context, receiptID uuid.UUID, expectedRevision uint64) (bool, error) {
	prepared, err := s.PrepareReindexReceipt(ctx, receiptID, expectedRevision)
	if err != nil {
		return false, err
	}
	if prepared.ReceiptID == uuid.Nil {
		return false, nil
	}
	tx, err := s.writer.BeginTx(ctx, catalogtx.OperationAssetReindexRequest, nil)
	if err != nil {
		return false, fmt.Errorf("begin reindex commit: %w", err)
	}
	if err := s.ApplyPreparedReindexTx(ctx, tx.Raw(), prepared); err != nil {
		_ = tx.Rollback()
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit reindex: %w", err)
	}
	return prepared.HasMore, nil
}

// PrepareReindexReceipt computes one deterministic candidate page from the
// reader pool. It does not reset indexes, request stages, advance cursors, or
// complete receipts; those mutations are committed atomically by the
// coordinator through ApplyPreparedReindexTx.
func (s *assetIndexingService) PrepareReindexReceipt(ctx context.Context, receiptID uuid.UUID, expectedRevision uint64) (PreparedReindex, error) {
	if receiptID == uuid.Nil || expectedRevision == 0 {
		return PreparedReindex{}, errors.New("invalid reindex receipt generation")
	}
	var repositoryRaw, cursor sql.NullString
	var tasksJSON string
	var limit int
	var missingOnly, resetSemantic bool
	var requestedRevision, appliedRevision uint64
	if err := s.readerPool.QueryRowContext(ctx, `SELECT repository_id,tasks,page_limit,cursor,missing_only,reset_semantic,requested_revision,applied_revision FROM asset_reindex_requests WHERE receipt_id=?`, receiptID.String()).Scan(&repositoryRaw, &tasksJSON, &limit, &cursor, &missingOnly, &resetSemantic, &requestedRevision, &appliedRevision); err != nil {
		return PreparedReindex{}, fmt.Errorf("read reindex receipt: %w", err)
	}
	if requestedRevision != expectedRevision || appliedRevision >= requestedRevision {
		return PreparedReindex{}, nil
	}
	var requestedNames []string
	if err := json.Unmarshal([]byte(tasksJSON), &requestedNames); err != nil {
		return PreparedReindex{}, fmt.Errorf("decode reindex tasks: %w", err)
	}
	requestedTasks := make([]AssetIndexingTask, 0, len(requestedNames))
	for _, task := range requestedNames {
		requestedTasks = append(requestedTasks, AssetIndexingTask(task))
	}
	effectiveConfig, err := s.settingsService.GetEffectiveMLConfig(ctx)
	if err != nil {
		return PreparedReindex{}, fmt.Errorf("load ML settings: %w", err)
	}
	enabledTasks := filterEnabledIndexingTasks(requestedTasks, effectiveConfig)
	if len(enabledTasks) == 0 {
		return PreparedReindex{ReceiptID: receiptID, RequestedRevision: expectedRevision, Tasks: requestedNames, Limit: limit, Cursor: cursor.String, MissingOnly: missingOnly, ResetSemantic: resetSemantic}, nil
	}
	var repositoryID *string
	if repositoryRaw.Valid {
		repositoryID = &repositoryRaw.String
	}
	repositoryUUID, err := parseRepositoryUUID(repositoryID)
	if err != nil {
		return PreparedReindex{}, err
	}
	offset := 0
	if cursor.Valid && cursor.String != "" {
		offset, err = strconv.Atoi(cursor.String)
		if err != nil || offset < 0 {
			return PreparedReindex{}, errors.New("invalid reindex cursor")
		}
	}
	input := ReindexAssetsInput{RepositoryID: repositoryID, Tasks: enabledTasks, Limit: limit, Offset: offset, MissingOnly: missingOnly, ResetSemantic: resetSemantic}
	candidates, err := s.collectReindexCandidates(ctx, repositoryUUID, enabledTasks, input)
	if err != nil {
		return PreparedReindex{}, err
	}
	nextOffset, hasMore := nextReindexPageOffset(missingOnly, len(candidates), limit, offset)
	var requestedRepositoryID *uuid.UUID
	if repositoryRaw.Valid {
		parsed, err := uuid.Parse(repositoryRaw.String)
		if err != nil {
			return PreparedReindex{}, err
		}
		requestedRepositoryID = &parsed
	}
	prepared := PreparedReindex{ReceiptID: receiptID, RequestedRevision: expectedRevision, RepositoryID: requestedRepositoryID, Tasks: requestedNames, Limit: limit, Cursor: cursor.String, MissingOnly: missingOnly, HasMore: hasMore, ResetSemantic: resetSemantic && offset == 0 && containsIndexingTask(enabledTasks, AssetIndexingTaskSemanticImage)}
	if hasMore {
		prepared.NextCursor = strconv.Itoa(nextOffset)
	}
	prepared.Candidates = make([]PreparedReindexCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		prepared.Candidates = append(prepared.Candidates, PreparedReindexCandidate{AssetID: candidate.asset.AssetID, ContentID: candidate.asset.ContentID})
	}
	return prepared, nil
}

// ApplyPreparedReindexTx applies a prepared candidate page to a caller-owned
// transaction. A stale page is a successful no-op at the commit boundary.
func (s *assetIndexingService) ApplyPreparedReindexTx(ctx context.Context, tx *sql.Tx, prepared PreparedReindex) error {
	if s == nil || tx == nil || prepared.ReceiptID == uuid.Nil || prepared.RequestedRevision == 0 {
		return errors.New("reindex commit is incomplete")
	}
	var requested, applied uint64
	if err := tx.QueryRowContext(ctx, `SELECT requested_revision,applied_revision FROM asset_reindex_requests WHERE receipt_id=?`, prepared.ReceiptID.String()).Scan(&requested, &applied); err != nil {
		return fmt.Errorf("validate reindex receipt: %w", err)
	}
	if requested != prepared.RequestedRevision || applied >= requested {
		return ErrReindexProjectionStale
	}
	qtx := s.queries.WithTx(tx)
	if prepared.ResetSemantic {
		if err := qtx.DeleteAllSearchEmbeddings(ctx); err != nil {
			return fmt.Errorf("reset semantic index: delete embeddings: %w", err)
		}
		if err := qtx.ClearDefaultSearchSpaceByType(ctx, string(EmbeddingTypeSemantic)); err != nil {
			return fmt.Errorf("reset semantic index: clear default space: %w", err)
		}
	}
	for _, candidate := range prepared.Candidates {
		if candidate.AssetID == uuid.Nil || candidate.ContentID == uuid.Nil {
			return errors.New("invalid reindex candidate")
		}
		if err := pipeline.RequestAssetStagesTx(ctx, tx, candidate.AssetID, candidate.ContentID, []pipeline.Stage{pipeline.StageEnrich}, pipeline.AssetPipelineVersion, workqos.Maintenance, prepared.ReceiptID); err != nil {
			return err
		}
	}
	if prepared.HasMore {
		return pipeline.AdvanceReindexTx(ctx, tx, prepared.ReceiptID, prepared.RequestedRevision, prepared.NextCursor)
	}
	return pipeline.FinishReindexTx(ctx, tx, prepared.ReceiptID, prepared.RequestedRevision)
}

func nextReindexPageOffset(missingOnly bool, candidateCount, limit, currentOffset int) (nextOffset int, hasMore bool) {
	if missingOnly || limit <= 0 || candidateCount < limit {
		return 0, false
	}
	return currentOffset + limit, true
}

func (s *assetIndexingService) collectReindexCandidates(
	ctx context.Context,
	repositoryUUID uuid.NullUUID,
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
				Limit:        int64(input.Limit),
				Offset:       int64(input.Offset),
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
				Limit:        int64(input.Limit),
				Offset:       int64(input.Offset),
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
	repositoryUUID uuid.NullUUID,
	task AssetIndexingTask,
	limit int,
) ([]repo.Asset, error) {
	switch task {
	case AssetIndexingTaskSemanticImage:
		return s.queries.ListPhotoAssetsMissingSemanticEmbedding(ctx, repo.ListPhotoAssetsMissingSemanticEmbeddingParams{
			RepositoryID: repositoryUUID,
			Limit:        int64(limit),
			Offset:       0,
		})
	case AssetIndexingTaskOCR:
		return s.queries.ListPhotoAssetsMissingOCRResults(ctx, repo.ListPhotoAssetsMissingOCRResultsParams{
			RepositoryID: repositoryUUID,
			Limit:        int64(limit),
			Offset:       0,
		})
	case AssetIndexingTaskFaceRecognition:
		return s.queries.ListPhotoAssetsMissingFaceResults(ctx, repo.ListPhotoAssetsMissingFaceResultsParams{
			RepositoryID: repositoryUUID,
			Limit:        int64(limit),
			Offset:       0,
		})
	case AssetIndexingTaskVideoSemantic:
		return s.queries.ListVideoAssetsMissingSemanticFrames(ctx, repo.ListVideoAssetsMissingSemanticFramesParams{
			RepositoryID: repositoryUUID,
			Limit:        int64(limit),
			Offset:       0,
		})
	default:
		return nil, fmt.Errorf("unsupported indexing task: %s", task)
	}
}

func (s *assetIndexingService) countPendingCatalogEnrichment(ctx context.Context) int64 {
	if s.readerPool == nil {
		return 0
	}

	var count int64
	if err := s.readerPool.QueryRowContext(ctx, `SELECT count(*) FROM asset_pipeline_state WHERE desired_version>applied_version AND stage='enrich'`).Scan(&count); err != nil {
		s.logger.Warn("indexing stats catalog backlog count failed",
			zap.String("operation", "indexing.stats"),
			zap.String("scope", "asset_pipeline.enrich"),
			zap.Error(err),
		)
		return 0
	}
	return count
}

func (s *assetIndexingService) countPendingReindexReceipts(ctx context.Context) int64 {
	if s.readerPool == nil {
		return 0
	}
	var count int64
	if err := s.readerPool.QueryRowContext(ctx, `SELECT count(*) FROM asset_reindex_requests WHERE requested_revision>applied_revision`).Scan(&count); err != nil {
		s.logger.Warn("indexing stats receipt count failed", zap.String("operation", "indexing.stats"), zap.String("scope", "asset_reindex"), zap.Error(err))
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
	if !repositoryUUID.Valid {
		return s.auditProvider.ForPath(repoPath)
	}
	repository, err := s.queries.GetRepository(context.Background(), repositoryUUID.UUID)
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
