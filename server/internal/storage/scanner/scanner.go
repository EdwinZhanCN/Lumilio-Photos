package scanner

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"server/config"
	"server/internal/db/repo"
	"server/internal/queue/jobs"
	"server/internal/storage"
	"server/internal/storage/repocfg"
	"server/internal/utils/file"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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

type Result struct {
	ScanRun         repo.RepositoryScanRun
	DiscoveredCount int64
	UpdatedCount    int64
	DeletedCount    int64
	SkippedCount    int64
}

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

type scanCounters struct {
	discovered int64
	updated    int64
	deleted    int64
	skipped    int64
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

func (s *Scanner) EnqueueManualScan(ctx context.Context, repositoryID string, requestedBy string, force bool) (EnqueueResult, error) {
	return s.enqueueScan(ctx, repositoryID, jobs.RepositoryScanModeManual, requestedBy, force)
}

func (s *Scanner) EnqueuePeriodicScan(ctx context.Context, repositoryID string) (EnqueueResult, error) {
	return s.enqueueScan(ctx, repositoryID, jobs.RepositoryScanModePeriodic, "", false)
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
	scanRun, err := s.queries.CreateRepositoryScanRun(ctx, repo.CreateRepositoryScanRunParams{
		ScanID:       scanID,
		RepositoryID: repository.RepoID,
		Mode:         normalizeMode(args.Mode),
		RequestedBy:  requestedByPtr,
		Status:       ScanStatusRunning,
		StartedAt:    pgtype.Timestamptz{Time: now, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("create scan run: %w", err)
	}

	running, err := s.queries.CountRunningRepositoryScanRuns(ctx, repo.CountRunningRepositoryScanRunsParams{
		RepositoryID: repository.RepoID,
		ScanID:       scanID,
	})
	if err != nil {
		_, _ = s.cancelScan(ctx, scanID, "failed to check running scans")
		return fmt.Errorf("count running scans: %w", err)
	}
	if running > 0 {
		_, err = s.cancelScan(ctx, scanID, "another scan is already running for this repository")
		return err
	}

	counters, scanErr := s.scanRepository(ctx, repository, args.Force)
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

func (s *Scanner) scanRepository(ctx context.Context, repository repo.Repository, force bool) (scanCounters, error) {
	diskEntries, skipped, err := walkRepository(repository.Path, time.Duration(s.cfg.SettleSeconds)*time.Second)
	counters := scanCounters{skipped: skipped}
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

	for storagePath, entry := range diskEntries {
		if ctx.Err() != nil {
			return counters, ctx.Err()
		}
		asset, exists := dbByPath[storagePath]
		if !exists {
			if err := s.enqueueDiscover(ctx, repository.RepoID, entry, jobs.DiscoverOperationUpsert); err != nil {
				return counters, err
			}
			counters.discovered++
			continue
		}
		delete(dbByPath, storagePath)

		if force || isSoftDeleted(asset) || asset.FileSize != entry.Size || fileMTimeIsNewerThanAsset(entry.MTime, asset) {
			if err := s.enqueueDiscover(ctx, repository.RepoID, entry, jobs.DiscoverOperationUpsert); err != nil {
				return counters, err
			}
			counters.updated++
		}
	}

	for storagePath, asset := range dbByPath {
		if ctx.Err() != nil {
			return counters, ctx.Err()
		}
		if isSoftDeleted(asset) {
			continue
		}
		entry := diskEntry{
			StoragePath: storagePath,
			Filename:    filepath.Base(storagePath),
		}
		if err := s.enqueueDiscover(ctx, repository.RepoID, entry, jobs.DiscoverOperationDelete); err != nil {
			return counters, err
		}
		counters.deleted++
	}

	return counters, nil
}

func (s *Scanner) enqueueDiscover(ctx context.Context, repositoryID pgtype.UUID, entry diskEntry, operation string) error {
	args := jobs.DiscoverAssetArgs{
		RepositoryID: repositoryID.String(),
		RelativePath: filepath.ToSlash(entry.StoragePath),
		Operation:    operation,
		FileName:     entry.Filename,
		DetectedAt:   time.Now().UTC(),
	}
	if operation == jobs.DiscoverOperationUpsert {
		args.ContentType = file.NewValidator().GetMimeTypeFromExtension(filepath.Ext(entry.Filename))
		args.FileSize = entry.Size
	}

	_, err := s.queue.Insert(ctx, args, &river.InsertOpts{Queue: "discover_asset"})
	return err
}

func (s *Scanner) cancelScan(ctx context.Context, scanID pgtype.UUID, reason string) (repo.RepositoryScanRun, error) {
	return s.queries.CancelRepositoryScanRun(ctx, repo.CancelRepositoryScanRunParams{
		ScanID:     scanID,
		FinishedAt: pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
		Error:      stringPtr(reason),
	})
}

func walkRepository(repoPath string, settle time.Duration) (map[string]diskEntry, int64, error) {
	entries := make(map[string]diskEntry)
	now := time.Now()
	var skipped int64

	err := filepath.WalkDir(repoPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			skipped++
			return nil
		}
		if path == repoPath {
			return nil
		}

		rel, err := filepath.Rel(repoPath, path)
		if err != nil {
			skipped++
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
			skipped++
			return nil
		}

		info, infoErr := d.Info()
		if infoErr != nil || info.IsDir() || !info.Mode().IsRegular() {
			skipped++
			return nil
		}
		if settle > 0 && now.Sub(info.ModTime()) < settle {
			skipped++
			return nil
		}

		entries[cleaned] = diskEntry{
			StoragePath: cleaned,
			Filename:    filepath.Base(cleaned),
			Size:        info.Size(),
			MTime:       info.ModTime().UTC(),
		}
		return nil
	})
	if err != nil {
		return nil, skipped, err
	}
	return entries, skipped, nil
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
