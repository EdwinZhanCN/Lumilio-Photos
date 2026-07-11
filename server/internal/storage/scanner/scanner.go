package scanner

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"server/config"
	"server/internal/db/repo"
	"server/internal/queue/jobs"
	"server/internal/storage"
	"server/internal/storage/repocfg"
	"server/internal/utils/file"
	"server/internal/utils/hash"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/riverqueue/river"
	"go.uber.org/zap"
)

const (
	ScanStatusQueued    = "queued"
	ScanStatusRunning   = "running"
	ScanStatusCompleted = "completed"
	ScanStatusFailed    = "failed"
	ScanStatusCancelled = "cancelled"
)

type EnqueueResult struct {
	JobID        int64
	RepositoryID string
	Mode         string
	Status       string
}

type Scanner struct {
	queries *repo.Queries
	queue   *river.Client[pgx.Tx]
	cfg     config.RepositoryScanConfig
	logger  *zap.Logger
}

type diskEntry struct {
	StoragePath string
	Filename    string
	Size        int64
	MTime       time.Time
}

type walkResult struct {
	entries       map[string]diskEntry
	deferredPaths map[string]struct{}
	skipped       int64
	deleteSafe    bool
	partialReason string
}

type scanCounters struct {
	discovered int64
	updated    int64
	deleted    int64
	skipped    int64
}

type assetFingerprint struct {
	hash string
	size int64
}

func NewScanner(queries *repo.Queries, queue *river.Client[pgx.Tx], cfg config.RepositoryScanConfig, logger *zap.Logger) *Scanner {
	if logger == nil {
		logger = zap.NewNop()
	}
	if cfg.IntervalSeconds <= 0 {
		cfg.IntervalSeconds = 300
	}
	if cfg.SettleSeconds <= 0 {
		cfg.SettleSeconds = 5
	}
	if cfg.MaxConcurrentRepos <= 0 {
		cfg.MaxConcurrentRepos = 1
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 500
	}

	return &Scanner{
		queries: queries,
		queue:   queue,
		cfg:     cfg,
		logger:  logger.With(zap.String("component", "repository_scanner")),
	}
}

// ReclaimInterruptedRuns marks scan runs left "running" by a previous process
// (e.g. a crash or restart) as failed, so the one-running-per-repository index
// does not permanently block future scans. Call once at startup.
func (s *Scanner) ReclaimInterruptedRuns(ctx context.Context) error {
	if s == nil || s.queries == nil {
		return nil
	}
	reclaimed, err := s.queries.ReclaimInterruptedRepositoryScanRuns(ctx)
	if err != nil {
		return fmt.Errorf("reclaim interrupted scan runs: %w", err)
	}
	if reclaimed > 0 {
		s.logger.Warn("reclaimed interrupted repository scan runs",
			zap.String("operation", "repository_scan.reclaim"),
			zap.Int64("count", reclaimed),
		)
	}
	return nil
}

func (s *Scanner) EnqueueManualScan(ctx context.Context, repositoryID string, requestedBy string, force bool) (EnqueueResult, error) {
	return s.enqueueScan(ctx, repositoryID, jobs.RepositoryScanModeManual, requestedBy, force)
}

func (s *Scanner) EnqueuePeriodicScan(ctx context.Context, repositoryID string) (EnqueueResult, error) {
	return s.enqueueScan(ctx, repositoryID, jobs.RepositoryScanModePeriodic, "", false)
}

// EnqueueAllPeriodicScans lists all active repositories and enqueues a
// periodic scan job for each, respecting MaxConcurrentRepos concurrency.
func (s *Scanner) EnqueueAllPeriodicScans(ctx context.Context) {
	repositories, err := s.queries.ListActiveRepositories(ctx)
	if err != nil {
		s.logger.Warn("failed to list active repositories for scan",
			zap.String("operation", "repository_scan.enqueue_all"),
			zap.Error(err),
		)
		return
	}

	sem := make(chan struct{}, s.cfg.MaxConcurrentRepos)
	var wg sync.WaitGroup
	for _, repository := range repositories {
		repository := repository
		if !repository.RepoID.Valid {
			continue
		}
		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
			return
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			if _, err := s.EnqueuePeriodicScan(ctx, repository.RepoID.String()); err != nil {
				s.logger.Warn("failed to enqueue periodic repository scan",
					zap.String("operation", "repository_scan.enqueue"),
					zap.String("repository_id", repository.RepoID.String()),
					zap.Error(err),
				)
			}
		}()
	}
	wg.Wait()
}

func (s *Scanner) enqueueScan(ctx context.Context, repositoryID string, mode string, requestedBy string, force bool) (EnqueueResult, error) {
	if s == nil || s.queue == nil {
		return EnqueueResult{}, fmt.Errorf("repository scanner queue unavailable")
	}
	repoID, err := parseRepositoryID(repositoryID)
	if err != nil {
		return EnqueueResult{}, err
	}
	if _, err := s.queries.GetRepository(ctx, repoID); err != nil {
		return EnqueueResult{}, fmt.Errorf("get repository: %w", err)
	}
	mode = normalizeMode(mode)
	job, err := s.queue.Insert(ctx, jobs.ScanRepositoryArgs{
		RepositoryID: repositoryID,
		Mode:         mode,
		RequestedBy:  requestedBy,
		Force:        force,
	}, &river.InsertOpts{Queue: "scan_repository"})
	if err != nil {
		return EnqueueResult{}, fmt.Errorf("enqueue repository scan: %w", err)
	}
	return EnqueueResult{
		JobID:        job.Job.ID,
		RepositoryID: repositoryID,
		Mode:         mode,
		Status:       ScanStatusQueued,
	}, nil
}

func (s *Scanner) ProcessScanRepository(ctx context.Context, args jobs.ScanRepositoryArgs) error {
	if s == nil || s.queries == nil || s.queue == nil {
		return fmt.Errorf("repository scanner unavailable")
	}
	repoID, err := parseRepositoryID(args.RepositoryID)
	if err != nil {
		return err
	}
	repository, err := s.queries.GetRepository(ctx, repoID)
	if err != nil {
		return fmt.Errorf("get repository: %w", err)
	}
	if !isScannableRepositoryRoot(repository.Path) {
		return fmt.Errorf("repository path is not a scannable repository root: %s", repository.Path)
	}

	scanID := pgtype.UUID{Bytes: uuid.New(), Valid: true}
	now := time.Now().UTC()
	requestedBy := strings.TrimSpace(args.RequestedBy)
	var requestedByPtr *string
	if requestedBy != "" {
		requestedByPtr = &requestedBy
	}
	// The repository_scan_runs_one_running partial unique index guarantees at most
	// one running scan per repository: a concurrent attempt fails here with a
	// unique violation and is skipped (no row created), replacing the previous
	// racy create-then-count-then-cancel guard.
	scanRun, err := s.queries.CreateRepositoryScanRun(ctx, repo.CreateRepositoryScanRunParams{
		ScanID:       scanID,
		RepositoryID: repository.RepoID,
		Mode:         normalizeMode(args.Mode),
		RequestedBy:  requestedByPtr,
		Status:       ScanStatusRunning,
		StartedAt:    pgtype.Timestamptz{Time: now, Valid: true},
	})
	if err != nil {
		if isUniqueConstraintViolation(err) {
			s.logger.Info("repository scan skipped: another scan is already running",
				zap.String("operation", "repository_scan.skip"),
				zap.String("repository_id", args.RepositoryID),
			)
			return nil
		}
		return fmt.Errorf("create scan run: %w", err)
	}

	counters, scanErr := s.scanRepository(ctx, repository, normalizeMode(args.Mode), args.Force)
	finishedAt := pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true}
	if scanErr != nil {
		_, failErr := s.queries.FailRepositoryScanRun(ctx, repo.FailRepositoryScanRunParams{
			ScanID:          scanID,
			FinishedAt:      finishedAt,
			DiscoveredCount: counters.discovered,
			UpdatedCount:    counters.updated,
			DeletedCount:    counters.deleted,
			SkippedCount:    counters.skipped,
			Error:           stringPtr(scanErr.Error()),
		})
		if failErr != nil {
			return fmt.Errorf("scan failed: %w; additionally failed to mark scan failed: %v", scanErr, failErr)
		}
		return scanErr
	}

	completed, err := s.queries.CompleteRepositoryScanRun(ctx, repo.CompleteRepositoryScanRunParams{
		ScanID:          scanID,
		FinishedAt:      finishedAt,
		DiscoveredCount: counters.discovered,
		UpdatedCount:    counters.updated,
		DeletedCount:    counters.deleted,
		SkippedCount:    counters.skipped,
	})
	if err != nil {
		return fmt.Errorf("complete scan run: %w", err)
	}
	scanRun = completed

	if _, err := s.queries.UpdateRepositoryLastSync(ctx, repo.UpdateRepositoryLastSyncParams{
		RepoID:    repository.RepoID,
		LastSync:  finishedAt,
		UpdatedAt: finishedAt,
	}); err != nil {
		s.logger.Warn("failed to update repository last sync",
			zap.String("repository_id", args.RepositoryID),
			zap.Error(err),
		)
	}

	s.logger.Info("repository scan completed",
		zap.String("repository_id", args.RepositoryID),
		zap.String("scan_id", scanRun.ScanID.String()),
		zap.Int64("discovered", counters.discovered),
		zap.Int64("updated", counters.updated),
		zap.Int64("deleted", counters.deleted),
		zap.Int64("skipped", counters.skipped),
	)

	// Merge structural media components and detect bursts after scan completion.
	if _, insertErr := s.queue.Insert(ctx, jobs.DetectStacksArgs{
		RepositoryID: args.RepositoryID,
	}, &river.InsertOpts{Queue: "detect_stacks"}); insertErr != nil {
		s.logger.Warn("failed to enqueue detect stacks job",
			zap.String("repository_id", args.RepositoryID),
			zap.Error(insertErr),
		)
	}

	return nil
}

func (s *Scanner) GetLatestScanRun(ctx context.Context, repositoryID string) (repo.RepositoryScanRun, error) {
	repoID, err := parseRepositoryID(repositoryID)
	if err != nil {
		return repo.RepositoryScanRun{}, err
	}
	return s.queries.GetLatestRepositoryScanRun(ctx, repoID)
}

func (s *Scanner) ListScanRuns(ctx context.Context, repositoryID string, limit, offset int32) ([]repo.RepositoryScanRun, error) {
	repoID, err := parseRepositoryID(repositoryID)
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	return s.queries.ListRepositoryScanRuns(ctx, repo.ListRepositoryScanRunsParams{
		RepositoryID: repoID,
		Limit:        limit,
		Offset:       offset,
	})
}

func (s *Scanner) scanRepository(ctx context.Context, repository repo.Repository, mode string, force bool) (scanCounters, error) {
	settle := time.Duration(s.cfg.SettleSeconds) * time.Second
	if force || normalizeMode(mode) == jobs.RepositoryScanModeManual {
		settle = 0
	}

	walk, err := walkRepository(repository.Path, settle)
	counters := scanCounters{skipped: walk.skipped}
	if err != nil {
		return counters, err
	}

	dbAssets, err := s.queries.ListAssetsByRepositoryAny(ctx, repository.RepoID)
	if err != nil {
		return counters, fmt.Errorf("list repository assets: %w", err)
	}
	dbByPath := make(map[string]repo.Asset, len(dbAssets))
	for _, asset := range dbAssets {
		if asset.StoragePath == nil {
			continue
		}
		cleaned, ok := CleanWorkspacePath(*asset.StoragePath)
		if !ok || IsExcludedWorkspacePath(cleaned) {
			continue
		}
		dbByPath[cleaned] = asset
	}

	batch := s.newDiscoverBatcher(ctx)

	newEntries := make(map[string]diskEntry)
	for storagePath, entry := range walk.entries {
		if ctx.Err() != nil {
			return counters, ctx.Err()
		}
		asset, exists := dbByPath[storagePath]
		if !exists {
			newEntries[storagePath] = entry
			continue
		}
		delete(dbByPath, storagePath)

		if force || isSoftDeleted(asset) || asset.FileSize != entry.Size || fileMTimeIsNewerThanAsset(entry.MTime, asset) {
			if err := batch.add(repository.RepoID, entry, jobs.DiscoverOperationUpsert); err != nil {
				return counters, err
			}
			counters.updated++
		}
	}

	moved, err := s.reconcileMovedEntries(ctx, repository, newEntries, dbByPath)
	if err != nil {
		return counters, err
	}
	counters.updated += moved

	for _, entry := range newEntries {
		if ctx.Err() != nil {
			return counters, ctx.Err()
		}
		if err := batch.add(repository.RepoID, entry, jobs.DiscoverOperationUpsert); err != nil {
			return counters, err
		}
		counters.discovered++
	}

	if !walk.deleteSafe {
		s.logger.Warn("repository scan skipped delete reconciliation after partial walk",
			zap.String("repository_id", repository.RepoID.String()),
			zap.String("repository_path", repository.Path),
			zap.String("reason", walk.partialReason),
		)
		if err := batch.flush(); err != nil {
			return counters, err
		}
		return counters, nil
	}

	for storagePath, asset := range dbByPath {
		if ctx.Err() != nil {
			return counters, ctx.Err()
		}
		if _, deferred := walk.deferredPaths[storagePath]; deferred {
			continue
		}
		if isSoftDeleted(asset) {
			continue
		}
		entry := diskEntry{
			StoragePath: storagePath,
			Filename:    filepath.Base(storagePath),
		}
		if err := batch.add(repository.RepoID, entry, jobs.DiscoverOperationDelete); err != nil {
			return counters, err
		}
		counters.deleted++
	}

	if err := batch.flush(); err != nil {
		return counters, err
	}
	return counters, nil
}

func (s *Scanner) reconcileMovedEntries(
	ctx context.Context,
	repository repo.Repository,
	newEntries map[string]diskEntry,
	missingAssets map[string]repo.Asset,
) (int64, error) {
	if len(newEntries) == 0 || len(missingAssets) == 0 {
		return 0, nil
	}

	candidates := make(map[assetFingerprint][]string)
	candidateSizes := make(map[int64]struct{})
	for storagePath, asset := range missingAssets {
		if isSoftDeleted(asset) || asset.Hash == nil || strings.TrimSpace(*asset.Hash) == "" {
			continue
		}
		key := assetFingerprint{hash: *asset.Hash, size: asset.FileSize}
		candidates[key] = append(candidates[key], storagePath)
		candidateSizes[asset.FileSize] = struct{}{}
	}
	if len(candidates) == 0 {
		return 0, nil
	}

	var moved int64
	for storagePath, entry := range newEntries {
		if ctx.Err() != nil {
			return moved, ctx.Err()
		}
		if _, ok := candidateSizes[entry.Size]; !ok {
			continue
		}

		fullPath := filepath.Join(repository.Path, filepath.FromSlash(entry.StoragePath))
		hashResult, err := hash.CalculateFileHash(fullPath, hash.AlgorithmBLAKE3, true)
		if err != nil {
			s.logger.Warn("failed to hash potential moved asset",
				zap.String("repository_id", repository.RepoID.String()),
				zap.String("storage_path", entry.StoragePath),
				zap.Error(err),
			)
			continue
		}

		key := assetFingerprint{hash: hashResult.Hash, size: entry.Size}
		paths := candidates[key]
		if len(paths) != 1 {
			continue
		}

		oldPath := paths[0]
		asset, ok := missingAssets[oldPath]
		if !ok {
			continue
		}

		if _, err := s.queries.MoveAssetWithinRepository(ctx, repo.MoveAssetWithinRepositoryParams{
			AssetID:          asset.AssetID,
			RepositoryID:     repository.RepoID,
			StoragePath:      &entry.StoragePath,
			OriginalFilename: entry.Filename,
		}); err != nil {
			return moved, fmt.Errorf("move discovered asset %s to %s: %w", oldPath, storagePath, err)
		}

		delete(newEntries, storagePath)
		delete(missingAssets, oldPath)
		delete(candidates, key)
		moved++

		s.logger.Debug("repository scan reconciled moved asset",
			zap.String("repository_id", repository.RepoID.String()),
			zap.String("asset_id", asset.AssetID.String()),
			zap.String("old_storage_path", oldPath),
			zap.String("new_storage_path", storagePath),
		)
	}

	return moved, nil
}

// discoverBatcher accumulates discover_asset jobs and inserts them in batches of
// cfg.BatchSize via River's InsertMany, instead of one insert per file.
type discoverBatcher struct {
	queue     *river.Client[pgx.Tx]
	ctx       context.Context
	batchSize int
	pending   []river.InsertManyParams
}

func (s *Scanner) newDiscoverBatcher(ctx context.Context) *discoverBatcher {
	return &discoverBatcher{queue: s.queue, ctx: ctx, batchSize: s.cfg.BatchSize}
}

func (b *discoverBatcher) add(repositoryID pgtype.UUID, entry diskEntry, operation string) error {
	args := jobs.DiscoverAssetArgs{
		RepositoryID: repositoryID.String(),
		RelativePath: filepath.ToSlash(entry.StoragePath),
		Operation:    operation,
		FileName:     entry.Filename,
		DetectedAt:   time.Now().UTC(),
	}
	if operation == jobs.DiscoverOperationUpsert {
		if mediaInfo, err := file.ResolveMedia(entry.Filename); err == nil {
			args.ContentType = mediaInfo.MimeType
		}
		args.FileSize = entry.Size
	}

	b.pending = append(b.pending, river.InsertManyParams{
		Args:       args,
		InsertOpts: &river.InsertOpts{Queue: "discover_asset"},
	})
	if len(b.pending) >= b.batchSize {
		return b.flush()
	}
	return nil
}

func (b *discoverBatcher) flush() error {
	if len(b.pending) == 0 {
		return nil
	}
	if _, err := b.queue.InsertMany(b.ctx, b.pending); err != nil {
		return fmt.Errorf("enqueue discover batch: %w", err)
	}
	b.pending = b.pending[:0]
	return nil
}

func isUniqueConstraintViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}

func walkRepository(repoPath string, settle time.Duration) (walkResult, error) {
	result := walkResult{
		entries:       make(map[string]diskEntry),
		deferredPaths: make(map[string]struct{}),
		deleteSafe:    true,
	}
	now := time.Now()

	err := filepath.WalkDir(repoPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			result.skipped++
			result.deleteSafe = false
			if result.partialReason == "" {
				result.partialReason = err.Error()
			}
			return nil
		}
		if path == repoPath {
			return nil
		}

		rel, err := filepath.Rel(repoPath, path)
		if err != nil {
			result.skipped++
			result.deleteSafe = false
			if result.partialReason == "" {
				result.partialReason = err.Error()
			}
			return nil
		}
		rel = filepath.ToSlash(rel)

		if d.IsDir() {
			if IsExcludedWorkspacePath(rel) {
				return filepath.SkipDir
			}
			return nil
		}

		cleaned, ok := ShouldScanPath(rel)
		if !ok {
			result.skipped++
			return nil
		}

		info, infoErr := d.Info()
		if infoErr != nil || info.IsDir() || !info.Mode().IsRegular() {
			result.skipped++
			if infoErr != nil {
				result.deleteSafe = false
				if result.partialReason == "" {
					result.partialReason = infoErr.Error()
				}
			}
			return nil
		}
		if settle > 0 && now.Sub(info.ModTime()) < settle {
			result.skipped++
			result.deferredPaths[cleaned] = struct{}{}
			return nil
		}

		result.entries[cleaned] = diskEntry{
			StoragePath: cleaned,
			Filename:    filepath.Base(cleaned),
			Size:        info.Size(),
			MTime:       info.ModTime().UTC(),
		}
		return nil
	})
	if err != nil {
		return result, err
	}
	return result, nil
}

func ShouldScanPath(path string) (string, bool) {
	cleaned, ok := CleanWorkspacePath(path)
	if !ok || IsExcludedWorkspacePath(cleaned) {
		return "", false
	}
	if !file.IsSupportedExtension(filepath.Ext(cleaned)) {
		return "", false
	}
	return cleaned, true
}

func CleanWorkspacePath(path string) (string, bool) {
	if strings.TrimSpace(path) == "" {
		return "", false
	}
	clean := filepath.Clean(filepath.FromSlash(path))
	if clean == "." || filepath.IsAbs(clean) {
		return "", false
	}
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", false
	}
	return filepath.ToSlash(clean), true
}

func IsExcludedWorkspacePath(path string) bool {
	normalized := filepath.ToSlash(strings.TrimSpace(path))
	if normalized == "" {
		return true
	}
	if normalized == storage.DefaultStructure.SystemDir || strings.HasPrefix(normalized, storage.DefaultStructure.SystemDir+"/") {
		return true
	}
	if normalized == storage.DefaultStructure.InboxDir || strings.HasPrefix(normalized, storage.DefaultStructure.InboxDir+"/") {
		return true
	}
	return false
}

func fileMTimeIsNewerThanAsset(mtime time.Time, asset repo.Asset) bool {
	if !asset.UpdatedAt.Valid {
		return true
	}
	return mtime.After(asset.UpdatedAt.Time.Add(time.Second))
}

func isSoftDeleted(asset repo.Asset) bool {
	return asset.IsDeleted != nil && *asset.IsDeleted
}

func isScannableRepositoryRoot(repoPath string) bool {
	cleaned := strings.TrimSpace(repoPath)
	if cleaned == "" {
		return false
	}
	info, err := os.Stat(cleaned)
	if err != nil || !info.IsDir() {
		return false
	}
	return repocfg.IsRepositoryRoot(cleaned)
}

func parseRepositoryID(repositoryID string) (pgtype.UUID, error) {
	parsed, err := uuid.Parse(strings.TrimSpace(repositoryID))
	if err != nil {
		return pgtype.UUID{}, fmt.Errorf("invalid repository id: %w", err)
	}
	return pgtype.UUID{Bytes: parsed, Valid: true}, nil
}

func normalizeMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case jobs.RepositoryScanModeManual:
		return jobs.RepositoryScanModeManual
	default:
		return jobs.RepositoryScanModePeriodic
	}
}

func stringPtr(value string) *string {
	return &value
}
