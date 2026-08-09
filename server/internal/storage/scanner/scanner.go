package scanner

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	"server/config"
	"server/internal/db"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/queue/jobs"
	"server/internal/storage"
	hashutil "server/internal/utils/hash"

	"github.com/google/uuid"
	"github.com/mattn/go-sqlite3"
	"github.com/riverqueue/river"
	"go.uber.org/zap"
)

const (
	ScanStatusQueued    = "queued"
	ScanStatusRunning   = "running"
	ScanStatusCompleted = "completed"
	ScanStatusFailed    = "failed"
	ScanStatusCancelled = "cancelled"

	indexStatePresent   = "present"
	indexStateMissing   = "missing"
	indexStateAmbiguous = "ambiguous"
	indexStateDeferred  = "deferred"
)

type EnqueueResult struct {
	JobID        int64
	RepositoryID string
	Mode         string
	Status       string
}

type Scanner struct {
	database         *db.DB
	queries          *repo.Queries
	queue            *river.Client[*sql.Tx]
	files            *storage.RepositoryFSFactory
	repositories     storage.RepositoryManager
	cfg              config.RepositoryScanConfig
	logger           *zap.Logger
	beforeScanInsert func()
}

type scanCounters struct {
	discovered    int64
	updated       int64
	moved         int64
	deleted       int64
	skipped       int64
	deferred      int64
	ambiguous     int64
	authoritative bool
	partialReason string
}

type observedCandidate struct {
	observation storage.FileObservation
	index       *repo.RepositoryFileIndex
	asset       *repo.Asset
}

type missingCandidate struct {
	storagePath string
	path        storage.RepositoryPath
	index       *repo.RepositoryFileIndex
	asset       *repo.Asset
}

type moveDecision struct {
	oldPath string
	newPath string
	asset   *repo.Asset
	newFile storage.FileObservation
}

type indexUpsert struct {
	observation          storage.FileObservation
	assetID              uuid.NullUUID
	state                string
	firstSeenScanID      uuid.UUID
	missingSinceScanID   uuid.NullUUID
	missingConfirmations int64
	ambiguityGroup       *string
	reason               *string
	inspectionError      *string
}

type indexStateUpdate struct {
	state                string
	missingSinceScanID   uuid.NullUUID
	missingConfirmations int64
	ambiguityGroup       *string
	reason               *string
	inspectionError      *string
}

func NewScanner(
	database *db.DB,
	queue *river.Client[*sql.Tx],
	files *storage.RepositoryFSFactory,
	repositories storage.RepositoryManager,
	cfg config.RepositoryScanConfig,
	logger *zap.Logger,
) *Scanner {
	if logger == nil {
		logger = zap.NewNop()
	}
	var queries *repo.Queries
	if database != nil {
		queries = database.Queries
	}
	return &Scanner{
		database:     database,
		queries:      queries,
		queue:        queue,
		files:        files,
		repositories: repositories,
		cfg:          cfg,
		logger:       logger.With(zap.String("component", "repository_scanner")),
	}
}

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
	if _, err := s.queries.ResetRepositoriesByActivity(ctx, repo.ResetRepositoriesByActivityParams{
		Activity:  dbtypes.RepositoryActivityScanning,
		UpdatedAt: dbtypes.NewTimestamp(time.Now().UTC()),
	}); err != nil {
		return fmt.Errorf("reset interrupted repository activity: %w", err)
	}
	return nil
}

func (s *Scanner) EnqueueManualScan(ctx context.Context, repositoryID string, requestedBy string, force bool) (EnqueueResult, error) {
	return s.enqueueScan(ctx, repositoryID, jobs.RepositoryScanModeManual, requestedBy, force)
}

func (s *Scanner) EnqueuePeriodicScan(ctx context.Context, repositoryID string) (EnqueueResult, error) {
	return s.enqueueScan(ctx, repositoryID, jobs.RepositoryScanModePeriodic, "", false)
}

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
	if s == nil || s.queue == nil || s.queries == nil || s.repositories == nil {
		return EnqueueResult{}, fmt.Errorf("repository scanner queue unavailable")
	}
	repositoryUUID, err := parseRepositoryID(repositoryID)
	if err != nil {
		return EnqueueResult{}, err
	}
	_, releaseWork, err := s.repositories.BeginRepositoryWork(ctx, repositoryUUID.String(), dbtypes.RepositoryActivityScanning)
	if err != nil {
		return EnqueueResult{}, err
	}
	defer releaseWork()
	mode = normalizeMode(mode)
	args := jobs.ScanRepositoryArgs{
		RepositoryID: repositoryID,
		Mode:         mode,
		RequestedBy:  requestedBy,
		Force:        force,
	}
	opts := args.InsertOpts()
	opts.Queue = "scan_repository"
	if s.beforeScanInsert != nil {
		s.beforeScanInsert()
	}
	job, err := s.queue.Insert(ctx, args, &opts)
	if err != nil {
		return EnqueueResult{}, fmt.Errorf("enqueue repository scan: %w", err)
	}
	if err := releaseWork(); err != nil {
		return EnqueueResult{}, fmt.Errorf("finish repository scan enqueue: %w", err)
	}
	return EnqueueResult{JobID: job.Job.ID, RepositoryID: repositoryID, Mode: mode, Status: ScanStatusQueued}, nil
}

func (s *Scanner) ProcessScanRepository(ctx context.Context, args jobs.ScanRepositoryArgs) error {
	if s == nil || s.database == nil || s.queries == nil || s.queue == nil || s.files == nil {
		return fmt.Errorf("repository scanner unavailable")
	}
	repositoryID, err := parseRepositoryID(args.RepositoryID)
	if err != nil {
		return err
	}
	repository, err := s.queries.GetRepository(ctx, repositoryID)
	if err != nil {
		return fmt.Errorf("get repository: %w", err)
	}
	if _, err := s.queries.BeginRepositoryActivity(ctx, repo.BeginRepositoryActivityParams{
		RepoID: repositoryID, Activity: dbtypes.RepositoryActivityScanning,
		UpdatedAt: dbtypes.NewTimestamp(time.Now().UTC()),
	}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			s.logger.Info("repository scan deferred: repository is unavailable or busy",
				zap.String("operation", "repository_scan.defer"),
				zap.String("repository_id", args.RepositoryID),
			)
			return fmt.Errorf("%w: repository scan will retry after maintenance", storage.ErrRepositoryBusy)
		}
		return fmt.Errorf("enter repository scanning activity: %w", err)
	}
	defer func() {
		if _, finishErr := s.queries.FinishRepositoryActivity(context.Background(), repo.FinishRepositoryActivityParams{
			RepoID: repositoryID, Activity: dbtypes.RepositoryActivityScanning,
			UpdatedAt: dbtypes.NewTimestamp(time.Now().UTC()),
		}); finishErr != nil {
			s.logger.Error("failed to clear repository scanning activity",
				zap.String("repository_id", args.RepositoryID), zap.Error(finishErr))
		}
	}()

	scanID := uuid.New()
	startedAt := time.Now().UTC()
	requestedBy := strings.TrimSpace(args.RequestedBy)
	var requestedByPtr *string
	if requestedBy != "" {
		requestedByPtr = &requestedBy
	}
	if _, err := s.queries.CreateRepositoryScanRun(ctx, repo.CreateRepositoryScanRunParams{
		ScanID:       scanID,
		RepositoryID: repository.RepoID,
		Mode:         normalizeMode(args.Mode),
		RequestedBy:  requestedByPtr,
		Status:       ScanStatusRunning,
		StartedAt:    dbtypes.NewTimestamp(startedAt),
	}); err != nil {
		if isUniqueConstraintViolation(err) {
			s.logger.Info("repository scan skipped: another scan is already running",
				zap.String("operation", "repository_scan.skip"),
				zap.String("repository_id", args.RepositoryID),
			)
			return nil
		}
		return fmt.Errorf("create scan run: %w", err)
	}

	counters, scanRun, scanErr := s.scanRepository(ctx, repository, scanID, startedAt, args.Force)
	if scanErr != nil {
		finishedAt := dbtypes.NewTimestamp(time.Now().UTC())
		_, failErr := s.queries.FailRepositoryScanRun(ctx, repo.FailRepositoryScanRunParams{
			ScanID:          scanID,
			FinishedAt:      finishedAt,
			DiscoveredCount: counters.discovered,
			UpdatedCount:    counters.updated,
			DeletedCount:    counters.deleted,
			SkippedCount:    counters.skipped,
			MovedCount:      counters.moved,
			DeferredCount:   counters.deferred,
			AmbiguousCount:  counters.ambiguous,
			Authoritative:   counters.authoritative,
			PartialReason:   optionalString(counters.partialReason),
			Error:           stringPtr(scanErr.Error()),
		})
		if failErr != nil {
			return fmt.Errorf("scan failed: %w; additionally failed to mark scan failed: %v", scanErr, failErr)
		}
		return scanErr
	}

	if _, err := s.queries.UpdateRepositoryLastSync(ctx, repo.UpdateRepositoryLastSyncParams{
		RepoID:    repository.RepoID,
		LastSync:  scanRun.FinishedAt,
		UpdatedAt: scanRun.FinishedAt,
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
		zap.Int64("moved", counters.moved),
		zap.Int64("deleted", counters.deleted),
		zap.Int64("deferred", counters.deferred),
		zap.Int64("ambiguous", counters.ambiguous),
		zap.Bool("authoritative", counters.authoritative),
	)
	return nil
}

func (s *Scanner) GetLatestScanRun(ctx context.Context, repositoryID string) (repo.RepositoryScanRun, error) {
	repositoryUUID, err := parseRepositoryID(repositoryID)
	if err != nil {
		return repo.RepositoryScanRun{}, err
	}
	return s.queries.GetLatestRepositoryScanRun(ctx, repositoryUUID)
}

func (s *Scanner) ListScanRuns(ctx context.Context, repositoryID string, limit, offset int32) ([]repo.RepositoryScanRun, error) {
	repositoryUUID, err := parseRepositoryID(repositoryID)
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
		RepositoryID: repositoryUUID,
		Limit:        int64(limit),
		Offset:       int64(offset),
	})
}

func (s *Scanner) scanRepository(
	ctx context.Context,
	repository repo.Repository,
	scanID uuid.UUID,
	startedAt time.Time,
	force bool,
) (scanCounters, repo.RepositoryScanRun, error) {
	counters := scanCounters{authoritative: true}
	repositoryFS, err := s.files.Open(repository)
	if err != nil {
		counters.authoritative = false
		counters.partialReason = err.Error()
		return counters, repo.RepositoryScanRun{}, err
	}
	defer repositoryFS.Close()

	settle := time.Duration(s.cfg.SettleSeconds) * time.Second
	walk, err := repositoryFS.WalkUserMedia(ctx, storage.WalkOptions{
		ScanID: scanID,
		Settle: settle,
		Now:    startedAt,
	})
	counters.skipped = walk.Skipped
	counters.authoritative = walk.Authoritative && len(walk.DeferredPaths) == 0
	counters.partialReason = walk.PartialReason
	if len(walk.DeferredPaths) > 0 {
		counters.partialReason = firstReason(counters.partialReason, "one or more supported media files are still settling")
	}
	if err != nil {
		return counters, repo.RepositoryScanRun{}, err
	}

	indexRows, err := s.queries.ListRepositoryFileIndex(ctx, repository.RepoID)
	if err != nil {
		return counters, repo.RepositoryScanRun{}, fmt.Errorf("list repository file index: %w", err)
	}
	assets, err := s.queries.ListAssetsByRepositoryAny(ctx, uuid.NullUUID{UUID: repository.RepoID, Valid: true})
	if err != nil {
		return counters, repo.RepositoryScanRun{}, fmt.Errorf("list repository assets: %w", err)
	}
	claims, err := s.queries.ListRecoverableIngestClaims(ctx, uuid.NullUUID{UUID: repository.RepoID, Valid: true})
	if err != nil {
		return counters, repo.RepositoryScanRun{}, fmt.Errorf("list recoverable ingest claims: %w", err)
	}

	indexByPath := make(map[string]*repo.RepositoryFileIndex, len(indexRows))
	for i := range indexRows {
		indexByPath[indexRows[i].StoragePath] = &indexRows[i]
	}
	assetByPath := make(map[string]*repo.Asset, len(assets))
	for i := range assets {
		if assets[i].StoragePath != nil {
			assetByPath[*assets[i].StoragePath] = &assets[i]
		}
	}
	claimedPaths := make(map[string]struct{}, len(claims)*2)
	for _, claim := range claims {
		if claim.StoragePath != nil {
			claimedPaths[*claim.StoragePath] = struct{}{}
		}
		if claim.StagingPath != "" {
			claimedPaths[claim.StagingPath] = struct{}{}
		}
	}
	deferredPaths := make(map[string]struct{}, len(walk.DeferredPaths))
	for _, deferredPath := range walk.DeferredPaths {
		deferredPaths[deferredPath.String()] = struct{}{}
	}

	upserts := make(map[string]indexUpsert, len(walk.Observations))
	stateUpdates := make(map[string]indexStateUpdate)
	newCandidates := make(map[string]observedCandidate)
	discoveryJobs := make(map[string]jobs.DiscoverAssetArgs)
	observedPaths := make(map[string]struct{}, len(walk.Observations))

	for _, observation := range walk.Observations {
		storagePath := observation.Path.String()
		observedPaths[storagePath] = struct{}{}
		indexed := indexByPath[storagePath]
		asset := assetByPath[storagePath]
		if _, claimed := claimedPaths[storagePath]; claimed {
			counters.deferred++
			if indexed != nil {
				reason := "recoverable ingest owns this path"
				stateUpdates[storagePath] = deferredState(reason, "")
			}
			continue
		}

		fastUnchanged := !force && indexed != nil && asset != nil && !asset.IsDeleted &&
			indexed.AssetID.Valid && indexed.AssetID.UUID == asset.AssetID &&
			indexed.State == indexStatePresent && indexed.ObservationToken == observation.ObservationToken
		if fastUnchanged {
			upserts[storagePath] = observationUpsert(scanID, observation, indexed, asset.AssetID, indexStatePresent)
			continue
		}

		hashed, inspectErr := inspectWithContent(ctx, repositoryFS, observation.Path, observation.Size)
		if inspectErr != nil {
			counters.authoritative = false
			counters.partialReason = firstReason(counters.partialReason, inspectErr.Error())
			counters.deferred++
			reason := "stable content inspection failed"
			detail := inspectErr.Error()
			mutation := observationUpsert(scanID, observation, indexed, assetID(asset), indexStateDeferred)
			mutation.reason = &reason
			mutation.inspectionError = &detail
			upserts[storagePath] = mutation
			continue
		}
		hashed.ScanID = scanID
		if err := repositoryFS.Revalidate(ctx, hashed); err != nil {
			counters.authoritative = false
			counters.partialReason = firstReason(counters.partialReason, err.Error())
			counters.deferred++
			reason := "file changed after content inspection"
			detail := err.Error()
			mutation := observationUpsert(scanID, observation, indexed, assetID(asset), indexStateDeferred)
			mutation.reason = &reason
			mutation.inspectionError = &detail
			upserts[storagePath] = mutation
			continue
		}

		if asset != nil {
			upserts[storagePath] = observationUpsert(scanID, hashed, indexed, asset.AssetID, indexStatePresent)
			contentChanged := hashed.ContentHash == nil || asset.ContentHash != *hashed.ContentHash || asset.FileSize != hashed.Size
			if force || asset.IsDeleted || contentChanged {
				discoveryJobs[storagePath] = discoverArgs(repository.RepoID, scanID, hashed)
				counters.updated++
			}
			continue
		}

		upserts[storagePath] = observationUpsert(scanID, hashed, indexed, uuid.Nil, indexStatePresent)
		newCandidates[storagePath] = observedCandidate{observation: hashed, index: indexed}
	}

	for storagePath := range deferredPaths {
		if indexed := indexByPath[storagePath]; indexed != nil {
			reason := "file is still settling"
			stateUpdates[storagePath] = deferredState(reason, "")
			counters.deferred++
		}
	}
	for storagePath := range claimedPaths {
		if _, observed := observedPaths[storagePath]; observed {
			continue
		}
		if indexed := indexByPath[storagePath]; indexed != nil {
			reason := "recoverable ingest owns this path"
			stateUpdates[storagePath] = deferredState(reason, "")
		}
	}

	missing := make(map[string]missingCandidate)
	for i := range assets {
		asset := &assets[i]
		if asset.IsDeleted || asset.StoragePath == nil || strings.TrimSpace(*asset.StoragePath) == "" {
			continue
		}
		storagePath := *asset.StoragePath
		if _, present := observedPaths[storagePath]; present {
			continue
		}
		if _, protected := deferredPaths[storagePath]; protected {
			continue
		}
		if _, claimed := claimedPaths[storagePath]; claimed {
			continue
		}
		parsed, parseErr := storage.ParseUserMediaPath(storagePath)
		if parseErr != nil {
			counters.deferred++
			counters.authoritative = false
			counters.partialReason = firstReason(counters.partialReason, fmt.Sprintf("asset %s has invalid storage path", asset.AssetID))
			continue
		}
		missing[storagePath] = missingCandidate{
			storagePath: storagePath,
			path:        parsed,
			index:       indexByPath[storagePath],
			asset:       asset,
		}
	}

	// A destructive or identity-changing decision must prove that every old
	// path is still absent after traversal.
	for storagePath, candidate := range missing {
		// On a case-insensitive filesystem the old spelling still opens after a
		// case-only rename. The newly walked path is the absence proof in that
		// one special case; move matching below still requires full BLAKE3.
		if hasCaseOnlyMoveCandidate(candidate, newCandidates) {
			continue
		}
		_, inspectErr := repositoryFS.InspectMedia(ctx, candidate.path, storage.HashNone)
		switch {
		case inspectErr == nil:
			delete(missing, storagePath)
			counters.authoritative = false
			counters.partialReason = firstReason(counters.partialReason, "a previously walked path changed before reconciliation")
			if candidate.index != nil {
				stateUpdates[storagePath] = deferredState("path reappeared during reconciliation", "")
			}
		case errors.Is(inspectErr, fs.ErrNotExist):
			// A missing child path is the expected result. Repository identity is
			// verified again immediately before commit below.
		default:
			delete(missing, storagePath)
			counters.authoritative = false
			counters.partialReason = firstReason(counters.partialReason, inspectErr.Error())
			if candidate.index != nil {
				stateUpdates[storagePath] = deferredState("old path could not be revalidated", inspectErr.Error())
			}
		}
	}

	moves, ambiguousOld, ambiguousNew := matchMoves(missing, newCandidates)
	for _, move := range moves {
		delete(discoveryJobs, move.newPath)
		mutation := upserts[move.newPath]
		mutation.assetID = uuid.NullUUID{UUID: move.asset.AssetID, Valid: true}
		mutation.state = indexStatePresent
		mutation.reason = nil
		upserts[move.newPath] = mutation
		delete(stateUpdates, move.oldPath)
		counters.moved++
	}
	for oldPath, group := range ambiguousOld {
		candidate := missing[oldPath]
		reason := "multiple full-content matches; no identity guess was made"
		if candidate.index == nil {
			upserts[oldPath] = bootstrapAssetUpsert(scanID, candidate, indexStateAmbiguous, group, reason)
		} else {
			stateUpdates[oldPath] = indexStateUpdate{
				state: indexStateAmbiguous, ambiguityGroup: &group, reason: &reason,
			}
		}
		counters.ambiguous++
	}
	for newPath, group := range ambiguousNew {
		delete(discoveryJobs, newPath)
		mutation := upserts[newPath]
		reason := "multiple full-content matches; no identity guess was made"
		mutation.state = indexStateAmbiguous
		mutation.ambiguityGroup = &group
		mutation.reason = &reason
		upserts[newPath] = mutation
		counters.ambiguous++
	}

	for storagePath, candidate := range newCandidates {
		if _, moved := moveDestination(moves, storagePath); moved {
			continue
		}
		if _, ambiguous := ambiguousNew[storagePath]; ambiguous {
			continue
		}
		discoveryJobs[storagePath] = discoverArgs(repository.RepoID, scanID, candidate.observation)
		counters.discovered++
	}

	moveOldPaths := make(map[string]struct{}, len(moves))
	for _, move := range moves {
		moveOldPaths[move.oldPath] = struct{}{}
	}
	deleteAssets := make(map[string]uuid.UUID)
	for storagePath, candidate := range missing {
		if _, moved := moveOldPaths[storagePath]; moved {
			continue
		}
		if _, ambiguous := ambiguousOld[storagePath]; ambiguous {
			continue
		}
		if !counters.authoritative {
			if candidate.index != nil {
				stateUpdates[storagePath] = deferredState("scan was not authoritative for absence", counters.partialReason)
			}
			continue
		}
		confirmed, confirmErr := s.isSecondConsecutiveAbsence(ctx, candidate.index, startedAt, settle)
		if confirmErr != nil {
			return counters, repo.RepositoryScanRun{}, confirmErr
		}
		if confirmed {
			deleteAssets[storagePath] = candidate.asset.AssetID
			stateUpdates[storagePath] = indexStateUpdate{
				state:                indexStateMissing,
				missingSinceScanID:   candidate.index.MissingSinceScanID,
				missingConfirmations: 2,
			}
			counters.deleted++
			continue
		}
		reason := "first authoritative absence"
		if candidate.index == nil {
			upserts[storagePath] = bootstrapAssetUpsert(scanID, candidate, indexStateMissing, "", reason)
		} else {
			stateUpdates[storagePath] = indexStateUpdate{
				state:                indexStateMissing,
				missingSinceScanID:   uuid.NullUUID{UUID: scanID, Valid: true},
				missingConfirmations: 1,
				reason:               &reason,
			}
		}
	}

	if err := repositoryFS.VerifyIdentity(); err != nil {
		counters.authoritative = false
		counters.partialReason = firstReason(counters.partialReason, err.Error())
		return counters, repo.RepositoryScanRun{}, err
	}
	for _, move := range moves {
		if err := repositoryFS.Revalidate(ctx, move.newFile); err != nil {
			counters.authoritative = false
			counters.partialReason = firstReason(counters.partialReason, err.Error())
			return counters, repo.RepositoryScanRun{}, err
		}
	}

	var completed repo.RepositoryScanRun
	finishedAt := dbtypes.NewTimestamp(time.Now().UTC())
	err = s.database.WithTx(ctx, func(tx *sql.Tx, queries *repo.Queries) error {
		for _, storagePath := range sortedKeys(upserts) {
			if _, err := queries.UpsertRepositoryFileObservation(ctx, upsertParams(repository.RepoID, upserts[storagePath], finishedAt)); err != nil {
				return fmt.Errorf("upsert file index %s: %w", storagePath, err)
			}
		}
		for _, storagePath := range sortedKeys(stateUpdates) {
			state := stateUpdates[storagePath]
			if _, err := queries.UpdateRepositoryFileIndexState(ctx, repo.UpdateRepositoryFileIndexStateParams{
				State:                state.state,
				MissingSinceScanID:   state.missingSinceScanID,
				MissingConfirmations: state.missingConfirmations,
				AmbiguityGroup:       state.ambiguityGroup,
				ReconciliationReason: state.reason,
				LastInspectionError:  state.inspectionError,
				UpdatedAt:            finishedAt,
				RepositoryID:         repository.RepoID,
				StoragePath:          storagePath,
			}); err != nil {
				return fmt.Errorf("update file index state %s: %w", storagePath, err)
			}
		}
		sort.Slice(moves, func(i, j int) bool {
			if moves[i].oldPath == moves[j].oldPath {
				return moves[i].newPath < moves[j].newPath
			}
			return moves[i].oldPath < moves[j].oldPath
		})
		for _, move := range moves {
			newPath := move.newPath
			if _, err := queries.MoveAssetWithinRepository(ctx, repo.MoveAssetWithinRepositoryParams{
				StoragePath:      &newPath,
				OriginalFilename: path.Base(newPath),
				AssetID:          move.asset.AssetID,
				RepositoryID:     uuid.NullUUID{UUID: repository.RepoID, Valid: true},
			}); err != nil {
				return fmt.Errorf("move asset %s from %s to %s: %w", move.asset.AssetID, move.oldPath, move.newPath, err)
			}
			if err := queries.DeleteRepositoryFileIndexEntry(ctx, repo.DeleteRepositoryFileIndexEntryParams{
				RepositoryID: repository.RepoID,
				StoragePath:  move.oldPath,
			}); err != nil {
				return fmt.Errorf("remove moved file index source %s: %w", move.oldPath, err)
			}
		}
		for _, storagePath := range sortedKeys(deleteAssets) {
			if err := queries.DeleteAsset(ctx, deleteAssets[storagePath]); err != nil {
				return fmt.Errorf("soft-delete asset at %s: %w", storagePath, err)
			}
		}
		for _, storagePath := range sortedKeys(discoveryJobs) {
			args := discoveryJobs[storagePath]
			opts := args.InsertOpts()
			opts.Queue = "discover_asset"
			if _, err := s.queue.InsertTx(ctx, tx, args, &opts); err != nil {
				return fmt.Errorf("enqueue discovery for %s: %w", storagePath, err)
			}
		}
		var completeErr error
		completed, completeErr = queries.CompleteRepositoryScanRun(ctx, repo.CompleteRepositoryScanRunParams{
			ScanID:          scanID,
			FinishedAt:      finishedAt,
			DiscoveredCount: counters.discovered,
			UpdatedCount:    counters.updated,
			DeletedCount:    counters.deleted,
			SkippedCount:    counters.skipped,
			MovedCount:      counters.moved,
			DeferredCount:   counters.deferred,
			AmbiguousCount:  counters.ambiguous,
			Authoritative:   counters.authoritative,
			PartialReason:   optionalString(counters.partialReason),
		})
		return completeErr
	})
	if err != nil {
		return counters, repo.RepositoryScanRun{}, fmt.Errorf("commit repository reconciliation: %w", err)
	}
	return counters, completed, nil
}

func hasCaseOnlyMoveCandidate(missing missingCandidate, newFiles map[string]observedCandidate) bool {
	matches := 0
	for newPath, candidate := range newFiles {
		if newPath == missing.storagePath || !strings.EqualFold(newPath, missing.storagePath) ||
			candidate.observation.ContentHash == nil || candidate.observation.Size != missing.asset.FileSize ||
			!strings.EqualFold(*candidate.observation.ContentHash, missing.asset.ContentHash) {
			continue
		}
		matches++
	}
	return matches == 1
}

func inspectWithContent(ctx context.Context, repositoryFS *storage.RepositoryFS, repositoryPath storage.RepositoryPath, size int64) (storage.FileObservation, error) {
	mode := storage.HashFull
	if size > hashutil.QuickHashThreshold {
		mode = storage.HashQuickAndFull
	}
	return repositoryFS.InspectMedia(ctx, repositoryPath, mode)
}

func (s *Scanner) isSecondConsecutiveAbsence(ctx context.Context, indexed *repo.RepositoryFileIndex, now time.Time, settle time.Duration) (bool, error) {
	if indexed == nil || indexed.State != indexStateMissing || indexed.MissingConfirmations != 1 || !indexed.MissingSinceScanID.Valid {
		return false, nil
	}
	run, err := s.queries.GetRepositoryScanRun(ctx, indexed.MissingSinceScanID.UUID)
	if err != nil {
		return false, fmt.Errorf("load first absence scan: %w", err)
	}
	return now.Sub(run.StartedAt.Time) >= settle, nil
}

func matchMoves(missing map[string]missingCandidate, newFiles map[string]observedCandidate) ([]moveDecision, map[string]string, map[string]string) {
	type contentKey struct {
		size int64
		hash string
	}
	oldGroups := make(map[contentKey][]missingCandidate)
	newGroups := make(map[contentKey][]observedCandidate)
	for _, candidate := range missing {
		contentHash := strings.ToLower(strings.TrimSpace(candidate.asset.ContentHash))
		if contentHash == "" {
			continue
		}
		oldGroups[contentKey{size: candidate.asset.FileSize, hash: contentHash}] = append(oldGroups[contentKey{size: candidate.asset.FileSize, hash: contentHash}], candidate)
	}
	for _, candidate := range newFiles {
		if candidate.observation.ContentHash == nil {
			continue
		}
		key := contentKey{size: candidate.observation.Size, hash: strings.ToLower(*candidate.observation.ContentHash)}
		newGroups[key] = append(newGroups[key], candidate)
	}

	var moves []moveDecision
	ambiguousOld := make(map[string]string)
	ambiguousNew := make(map[string]string)
	for key, oldCandidates := range oldGroups {
		newCandidates := newGroups[key]
		if len(newCandidates) == 0 {
			continue
		}
		if len(oldCandidates) == 1 && len(newCandidates) == 1 {
			moves = append(moves, moveDecision{
				oldPath: oldCandidates[0].storagePath,
				newPath: newCandidates[0].observation.Path.String(),
				asset:   oldCandidates[0].asset,
				newFile: newCandidates[0].observation,
			})
			continue
		}
		oldPaths := make([]string, 0, len(oldCandidates))
		newPaths := make([]string, 0, len(newCandidates))
		for _, candidate := range oldCandidates {
			oldPaths = append(oldPaths, candidate.storagePath)
		}
		for _, candidate := range newCandidates {
			newPaths = append(newPaths, candidate.observation.Path.String())
		}
		group := ambiguityGroupID(key.hash, oldPaths, newPaths)
		for _, oldPath := range oldPaths {
			ambiguousOld[oldPath] = group
		}
		for _, newPath := range newPaths {
			ambiguousNew[newPath] = group
		}
	}
	return moves, ambiguousOld, ambiguousNew
}

func ambiguityGroupID(contentHash string, oldPaths, newPaths []string) string {
	sort.Strings(oldPaths)
	sort.Strings(newPaths)
	value := strings.Join([]string{contentHash, strings.Join(oldPaths, "\x00"), strings.Join(newPaths, "\x00")}, "\x01")
	digest := sha256.Sum256([]byte(value))
	return fmt.Sprintf("amb-v1:%x", digest[:])
}

func observationUpsert(scanID uuid.UUID, observation storage.FileObservation, indexed *repo.RepositoryFileIndex, assetUUID uuid.UUID, state string) indexUpsert {
	firstSeen := scanID
	if indexed != nil && indexed.FirstSeenScanID.Valid {
		firstSeen = indexed.FirstSeenScanID.UUID
	}
	return indexUpsert{
		observation:     observation,
		assetID:         nullableUUID(assetUUID),
		state:           state,
		firstSeenScanID: firstSeen,
	}
}

func bootstrapAssetUpsert(scanID uuid.UUID, candidate missingCandidate, state, ambiguityGroup, reason string) indexUpsert {
	contentHash := strings.ToLower(strings.TrimSpace(candidate.asset.ContentHash))
	observation := storage.FileObservation{
		RepositoryID:     candidate.asset.RepositoryID.UUID,
		Path:             candidate.path,
		EntryKind:        storage.EntryKindRegular,
		Size:             candidate.asset.FileSize,
		ObservationToken: "asset-bootstrap-v1:" + candidate.asset.AssetID.String(),
		ContentHash:      optionalString(contentHash),
		ScanID:           scanID,
	}
	mutation := indexUpsert{
		observation:     observation,
		assetID:         uuid.NullUUID{UUID: candidate.asset.AssetID, Valid: true},
		state:           state,
		firstSeenScanID: scanID,
		reason:          &reason,
	}
	if state == indexStateMissing {
		mutation.missingSinceScanID = uuid.NullUUID{UUID: scanID, Valid: true}
		mutation.missingConfirmations = 1
	}
	if ambiguityGroup != "" {
		mutation.ambiguityGroup = &ambiguityGroup
	}
	return mutation
}

func upsertParams(repositoryID uuid.UUID, mutation indexUpsert, updatedAt dbtypes.Timestamp) repo.UpsertRepositoryFileObservationParams {
	observation := mutation.observation
	return repo.UpsertRepositoryFileObservationParams{
		RepositoryID:            repositoryID,
		StoragePath:             observation.Path.String(),
		AssetID:                 mutation.assetID,
		EntryKind:               string(observation.EntryKind),
		FileSize:                observation.Size,
		ModifiedAtNs:            observation.ModTimeNS,
		ChangedAtNs:             observation.ChangeTimeNS,
		FileIdentityKind:        observation.FileIdentityKind,
		FileIdentityValue:       observation.FileIdentity,
		ObservationToken:        observation.ObservationToken,
		QuickFingerprint:        observation.QuickFingerprint,
		QuickFingerprintVersion: observation.QuickFingerprintVer,
		ContentHash:             observation.ContentHash,
		State:                   mutation.state,
		FirstSeenScanID:         uuid.NullUUID{UUID: mutation.firstSeenScanID, Valid: true},
		LastSeenScanID:          uuid.NullUUID{UUID: observation.ScanID, Valid: true},
		MissingSinceScanID:      mutation.missingSinceScanID,
		MissingConfirmations:    mutation.missingConfirmations,
		AmbiguityGroup:          mutation.ambiguityGroup,
		ReconciliationReason:    mutation.reason,
		LastInspectionError:     mutation.inspectionError,
		UpdatedAt:               updatedAt,
	}
}

func deferredState(reason, detail string) indexStateUpdate {
	return indexStateUpdate{
		state:           indexStateDeferred,
		reason:          optionalString(reason),
		inspectionError: optionalString(detail),
	}
}

func discoverArgs(repositoryID, scanID uuid.UUID, observation storage.FileObservation) jobs.DiscoverAssetArgs {
	return jobs.DiscoverAssetArgs{
		RepositoryID:     repositoryID,
		StoragePath:      observation.Path.String(),
		ScanID:           scanID,
		ObservationToken: observation.ObservationToken,
	}
}

func moveDestination(moves []moveDecision, storagePath string) (moveDecision, bool) {
	for _, move := range moves {
		if move.newPath == storagePath {
			return move, true
		}
	}
	return moveDecision{}, false
}

func assetID(asset *repo.Asset) uuid.UUID {
	if asset == nil {
		return uuid.Nil
	}
	return asset.AssetID
}

func nullableUUID(value uuid.UUID) uuid.NullUUID {
	return uuid.NullUUID{UUID: value, Valid: value != uuid.Nil}
}

func sortedKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func optionalString(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	copy := value
	return &copy
}

func firstReason(current, next string) string {
	if current != "" {
		return current
	}
	return next
}

func isUniqueConstraintViolation(err error) bool {
	var sqliteErr sqlite3.Error
	if errors.As(err, &sqliteErr) {
		return sqliteErr.ExtendedCode == sqlite3.ErrConstraintUnique ||
			sqliteErr.ExtendedCode == sqlite3.ErrConstraintPrimaryKey
	}
	return false
}

func parseRepositoryID(id string) (uuid.UUID, error) {
	parsed, err := uuid.Parse(strings.TrimSpace(id))
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid repository ID: %w", err)
	}
	return parsed, nil
}

func normalizeMode(mode string) string {
	if strings.EqualFold(strings.TrimSpace(mode), jobs.RepositoryScanModePeriodic) {
		return jobs.RepositoryScanModePeriodic
	}
	return jobs.RepositoryScanModeManual
}

func stringPtr(value string) *string { return &value }
