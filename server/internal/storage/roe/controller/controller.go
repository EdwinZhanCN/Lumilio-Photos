// Package controller persists and advances bounded Repository Observation
// Engine turns. Filesystem enumeration happens before the catalog transaction;
// each turn commits at most one bounded directory page and then yields.
package controller

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"server/internal/db"
	"server/internal/db/catalogtx"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/storage"
	"server/internal/storage/roe/changefeed"
	"server/internal/storage/roe/nodegraph"
	"server/internal/storage/roe/pathsemantics"
)

const (
	StatusQueued     = "queued"
	StatusCrawling   = "crawling"
	StatusCatchingUp = "catching_up"
	StatusFinalizing = "finalizing"
	StatusCompleted  = "completed"
	StatusPartial    = "partial"
	StatusFailed     = "failed"
	StatusCancelled  = "cancelled"

	defaultBatchSize       = 32
	maximumBatchSize       = 48
	// A native change can dirty both the old and new parent and may require
	// several revision-fenced writes per prefix. Keep the writer quantum at one
	// event so a burst cannot turn one catch-up turn into an unbounded hold.
	maximumChangeBatchSize = 1
	// Each absence child performs several revision-fenced catalog writes.
	// Keep the count ceiling aligned with the time budget so a turn yields early.
	maximumAbsenceBatchSize    = 4
	defaultTransactionBudget   = 4 * time.Millisecond
	maximumTransactionBudget   = 25 * time.Millisecond
	defaultControllerLease     = 30 * time.Second
	defaultOutboxHighWaterMark = 4096
)

type Config struct {
	BatchSize            int
	TransactionBudget    time.Duration
	ControllerLease      time.Duration
	OutboxHighWaterMark  int64
	Settle               time.Duration
	DefaultPathSemantics pathsemantics.Semantics
	ChangeFeed           changefeed.Feed
	directorySource      directorySource
}

func (cfg Config) normalized() Config {
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = defaultBatchSize
	}
	if cfg.BatchSize > maximumBatchSize {
		cfg.BatchSize = maximumBatchSize
	}
	if cfg.TransactionBudget <= 0 {
		cfg.TransactionBudget = defaultTransactionBudget
	} else if cfg.TransactionBudget > maximumTransactionBudget {
		cfg.TransactionBudget = maximumTransactionBudget
	}
	if cfg.ControllerLease <= 0 {
		cfg.ControllerLease = defaultControllerLease
	}
	if cfg.OutboxHighWaterMark <= 0 {
		cfg.OutboxHighWaterMark = defaultOutboxHighWaterMark
	}
	if cfg.DefaultPathSemantics.Case == "" {
		cfg.DefaultPathSemantics = hostDefaultPathSemantics()
	}
	return cfg
}

func hostDefaultPathSemantics() pathsemantics.Semantics {
	switch runtime.GOOS {
	case "windows", "darwin":
		return pathsemantics.Semantics{
			Case: pathsemantics.CaseInsensitive, Normalization: pathsemantics.NormalizationNFC,
		}
	default:
		return pathsemantics.Semantics{
			Case: pathsemantics.CaseSensitive, Normalization: pathsemantics.NormalizationNFC,
		}
	}
}

type Receipt struct {
	OperationID    uuid.UUID
	RepositoryID   uuid.UUID
	RequestedEpoch int64
	Mode           string
	Status         string
	Inserted       bool
	Coalesced      bool
}

type TurnResult struct {
	OperationID         uuid.UUID
	Status              string
	HasMore             bool
	Backpressure        bool
	RowsApplied         int
	BytesQueued         int64
	TransactionDuration time.Duration
	CommitDuration      time.Duration
}

type Controller struct {
	database    *db.DB
	files       *storage.RepositoryFSFactory
	directories directorySource
	cfg         Config
	logger      *zap.Logger
	now         func() time.Time
	feed        changefeed.Feed
}

func New(database *db.DB, files *storage.RepositoryFSFactory, cfg Config, logger *zap.Logger) *Controller {
	if logger == nil {
		logger = zap.NewNop()
	}
	feed := cfg.ChangeFeed
	if feed == nil {
		feed = changefeed.NewNative()
	}
	directories := cfg.directorySource
	if directories == nil && files != nil {
		directories = repositoryDirectorySource{files: files}
	}
	return &Controller{
		database: database, files: files, directories: directories, cfg: cfg.normalized(),
		logger: logger.With(zap.String("component", "repository_observation_controller")),
		now:    func() time.Time { return time.Now().UTC() },
		feed:   feed,
	}
}

// directorySource is the filesystem-free seam used by deterministic scale
// profiles. Production always uses repositoryDirectorySource, which preserves
// the RepositoryFS lifecycle and performs enumeration outside transactions.
type directorySource interface {
	ReadDirectory(
		context.Context,
		repo.Repository,
		storage.DirectoryReadOptions,
	) (storage.DirectoryReadBatch, error)
}

type repositoryDirectorySource struct {
	files *storage.RepositoryFSFactory
}

func (source repositoryDirectorySource) ReadDirectory(
	ctx context.Context,
	repository repo.Repository,
	options storage.DirectoryReadOptions,
) (storage.DirectoryReadBatch, error) {
	repositoryFS, err := source.files.OpenContext(ctx, repository)
	if err != nil {
		return storage.DirectoryReadBatch{}, err
	}
	batch, readErr := repositoryFS.ReadUserMediaDirectory(ctx, options)
	closeErr := repositoryFS.Close()
	return batch, errors.Join(readErr, closeErr)
}

func (c *Controller) Notifications() <-chan uuid.UUID {
	if c == nil {
		return nil
	}
	if source, ok := c.feed.(changefeed.NotificationSource); ok {
		return source.Notifications()
	}
	return nil
}

func (c *Controller) Close() error {
	if c == nil {
		return nil
	}
	if closer, ok := c.feed.(changefeed.Closer); ok {
		return closer.Close()
	}
	return nil
}

// Request merges a durable desired epoch and operation receipt in the same
// transaction as its controller wakeup outbox effect.
func (c *Controller) Request(
	ctx context.Context,
	repositoryID uuid.UUID,
	mode string,
	requestedBy string,
	forceFullVerification bool,
) (Receipt, error) {
	if c == nil || c.database == nil {
		return Receipt{}, errors.New("repository observation controller unavailable")
	}
	if repositoryID == uuid.Nil {
		return Receipt{}, errors.New("repository ID is required")
	}
	mode = normalizeMode(mode)
	requestedBy = strings.TrimSpace(requestedBy)
	var requestedByValue *string
	if requestedBy != "" {
		requestedByValue = &requestedBy
	}
	now := dbtypes.NewTimestamp(c.now())
	receipt := Receipt{RepositoryID: repositoryID, Mode: mode, Status: StatusQueued}
	err := c.database.WithTx(ctx, catalogtx.OperationRepositoryObservationRequest, func(_ *sql.Tx, queries *repo.Queries) error {
		if _, err := queries.GetRepository(ctx, repositoryID); err != nil {
			return fmt.Errorf("load repository: %w", err)
		}
		semantics := c.cfg.DefaultPathSemantics
		if _, err := queries.EnsureRepositoryObservationState(ctx, repo.EnsureRepositoryObservationStateParams{
			RepositoryID: repositoryID, AdapterKind: "periodic", VolumeKind: "unknown",
			PathCaseMode: string(semantics.Case), PathNormalization: string(semantics.Normalization),
			CursorHealth: "unavailable", FullVerificationRequired: 1, UpdatedAt: now,
		}); err != nil {
			return fmt.Errorf("ensure repository observation state: %w", err)
		}
		state, err := queries.RequestRepositoryObservationEpoch(ctx, repo.RequestRepositoryObservationEpochParams{
			RepositoryID:             repositoryID,
			FullVerificationRequired: boolInt(forceFullVerification), UpdatedAt: now,
		})
		if err != nil {
			return fmt.Errorf("request repository observation epoch: %w", err)
		}
		active, err := queries.GetActiveRepositoryScanRun(ctx, repositoryID)
		switch {
		case err == nil:
			active, err = queries.CoalesceRepositoryScanRun(ctx, repo.CoalesceRepositoryScanRunParams{
				RunID: active.RunID, RequestedEpoch: state.DesiredEpoch,
				ForceFullVerification: boolInt(forceFullVerification), UpdatedAt: now,
			})
			if err != nil {
				return fmt.Errorf("coalesce repository observation run: %w", err)
			}
			// The previous controller wakeup is fenced by its expected epoch. If
			// this request lands before that wakeup is delivered, coalescing makes
			// the old job stale; publish the current epoch in the same transaction
			// so the durable operation cannot remain queued indefinitely.
			effectKey := fmt.Sprintf("controller:%s:%d", repositoryID, state.DesiredEpoch)
			if _, err := queries.InsertRepositoryOutboxEffect(ctx, repo.InsertRepositoryOutboxEffectParams{
				OutboxID: uuid.New(), RepositoryID: repositoryID, EffectKey: effectKey,
				EffectKind: "controller", EntityID: active.RunID.String(),
				ExpectedRevision: state.DesiredEpoch, Payload: `{}`, CreatedAt: now,
			}); err != nil {
				return fmt.Errorf("publish coalesced repository controller wakeup: %w", err)
			}
			receipt.OperationID = active.RunID
			receipt.RequestedEpoch = active.RequestedEpoch
			receipt.Mode = active.Mode
			receipt.Status = active.Status
			receipt.Coalesced = true
			return nil
		case !errors.Is(err, sql.ErrNoRows):
			return fmt.Errorf("load active repository observation run: %w", err)
		}
		runID := uuid.New()
		if _, err := queries.CreateRepositoryScanRun(ctx, repo.CreateRepositoryScanRunParams{
			RunID: runID, RepositoryID: repositoryID, RequestedEpoch: state.DesiredEpoch,
			Mode: mode, RequestedBy: requestedByValue,
			ForceFullVerification: boolInt(forceFullVerification), CreatedAt: now,
		}); err != nil {
			return fmt.Errorf("create repository observation run: %w", err)
		}
		if _, err := queries.SetActiveRepositoryObservationRun(ctx, repo.SetActiveRepositoryObservationRunParams{
			RepositoryID: repositoryID,
			ActiveRunID:  uuid.NullUUID{UUID: runID, Valid: true}, UpdatedAt: now,
		}); err != nil {
			return fmt.Errorf("activate repository observation run: %w", err)
		}
		effectKey := fmt.Sprintf("controller:%s:%d", repositoryID, state.DesiredEpoch)
		if _, err := queries.InsertRepositoryOutboxEffect(ctx, repo.InsertRepositoryOutboxEffectParams{
			OutboxID: uuid.New(), RepositoryID: repositoryID, EffectKey: effectKey,
			EffectKind: "controller", EntityID: runID.String(),
			ExpectedRevision: state.DesiredEpoch, Payload: `{}`, CreatedAt: now,
		}); err != nil {
			return fmt.Errorf("publish repository controller wakeup: %w", err)
		}
		receipt.OperationID = runID
		receipt.RequestedEpoch = state.DesiredEpoch
		receipt.Inserted = true
		return nil
	})
	if err != nil {
		return Receipt{}, err
	}
	return receipt, nil
}

func (c *Controller) EnqueueManualScan(ctx context.Context, repositoryID, requestedBy string, force bool) (Receipt, error) {
	parsed, err := uuid.Parse(strings.TrimSpace(repositoryID))
	if err != nil {
		return Receipt{}, fmt.Errorf("invalid repository ID: %w", err)
	}
	return c.Request(ctx, parsed, "manual", requestedBy, force)
}

func (c *Controller) EnqueuePeriodicScan(ctx context.Context, repositoryID string) (Receipt, error) {
	parsed, err := uuid.Parse(strings.TrimSpace(repositoryID))
	if err != nil {
		return Receipt{}, fmt.Errorf("invalid repository ID: %w", err)
	}
	return c.Request(ctx, parsed, "periodic", "", true)
}

func (c *Controller) EnqueueAllPeriodicScans(ctx context.Context) {
	if c == nil || c.database == nil {
		return
	}
	repositories, err := c.database.ReaderQueries.ListActiveRepositories(ctx)
	if err != nil {
		c.logger.Warn("list repositories for periodic observation", zap.Error(err))
		return
	}
	for _, repository := range repositories {
		if _, err := c.Request(ctx, repository.RepoID, "periodic", "", true); err != nil {
			c.logger.Warn("request periodic repository observation",
				zap.String("repository_id", repository.RepoID.String()), zap.Error(err))
		}
	}
}

func (c *Controller) GetScanRun(ctx context.Context, repositoryID, operationID string) (repo.RepositoryScanRun, error) {
	repositoryUUID, err := uuid.Parse(strings.TrimSpace(repositoryID))
	if err != nil {
		return repo.RepositoryScanRun{}, fmt.Errorf("invalid repository ID: %w", err)
	}
	operationUUID, err := uuid.Parse(strings.TrimSpace(operationID))
	if err != nil {
		return repo.RepositoryScanRun{}, fmt.Errorf("invalid operation ID: %w", err)
	}
	return c.database.ReaderQueries.GetRepositoryScanRun(ctx, repo.GetRepositoryScanRunParams{
		RepositoryID: repositoryUUID, RunID: operationUUID,
	})
}

func (c *Controller) GetLatestScanRun(ctx context.Context, repositoryID string) (repo.RepositoryScanRun, error) {
	repositoryUUID, err := uuid.Parse(strings.TrimSpace(repositoryID))
	if err != nil {
		return repo.RepositoryScanRun{}, fmt.Errorf("invalid repository ID: %w", err)
	}
	return c.database.ReaderQueries.GetLatestRepositoryScanRun(ctx, repositoryUUID)
}

func (c *Controller) ListScanRuns(ctx context.Context, repositoryID string, limit, offset int32) ([]repo.RepositoryScanRun, error) {
	repositoryUUID, err := uuid.Parse(strings.TrimSpace(repositoryID))
	if err != nil {
		return nil, fmt.Errorf("invalid repository ID: %w", err)
	}
	return c.database.ReaderQueries.ListRepositoryScanRuns(ctx, repo.ListRepositoryScanRunsParams{
		RepositoryID: repositoryUUID, Limit: int64(limit), Offset: int64(offset),
	})
}

// CancelScanRun requests cancellation of one exact, repository-scoped
// operation. The bounded controller observes the flag before its next turn.
func (c *Controller) CancelScanRun(ctx context.Context, repositoryID, operationID string) (repo.RepositoryScanRun, error) {
	repositoryUUID, err := uuid.Parse(strings.TrimSpace(repositoryID))
	if err != nil {
		return repo.RepositoryScanRun{}, fmt.Errorf("invalid repository ID: %w", err)
	}
	operationUUID, err := uuid.Parse(strings.TrimSpace(operationID))
	if err != nil {
		return repo.RepositoryScanRun{}, fmt.Errorf("invalid operation ID: %w", err)
	}
	var cancelled repo.RepositoryScanRun
	err = c.database.WithTx(ctx, catalogtx.OperationRepositoryObservationCancelRequest, func(_ *sql.Tx, queries *repo.Queries) error {
		current, err := queries.GetRepositoryScanRun(ctx, repo.GetRepositoryScanRunParams{
			RepositoryID: repositoryUUID, RunID: operationUUID,
		})
		if err != nil {
			return err
		}
		if isTerminal(current.Status) {
			cancelled = current
			return nil
		}
		cancelled, err = queries.RequestRepositoryScanRunCancellation(ctx, repo.RequestRepositoryScanRunCancellationParams{
			RunID: operationUUID, UpdatedAt: dbtypes.NewTimestamp(c.now()),
		})
		return err
	})
	return cancelled, err
}

// RunTurn advances at most one bounded controller unit. Callers enqueue a
// follower when HasMore is true; an expired controller/frontier lease makes a
// crash replay the same durable work.
func (c *Controller) RunTurn(ctx context.Context, repositoryID, operationID uuid.UUID) (TurnResult, error) {
	result := TurnResult{OperationID: operationID, HasMore: true}
	if c == nil || c.database == nil || c.directories == nil {
		return result, errors.New("repository observation controller unavailable")
	}
	nowTime := c.now()
	now := dbtypes.NewTimestamp(nowTime)
	leaseValue := uuid.NewString()
	leaseExpiry := nowTime.Add(c.cfg.ControllerLease).UnixMicro()
	_, err := c.database.Queries.ClaimRepositoryObservationController(ctx, repo.ClaimRepositoryObservationControllerParams{
		RepositoryID: repositoryID, ControllerLeaseID: &leaseValue,
		ControllerLeaseExpiresAt: &leaseExpiry, UpdatedAt: now,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return result, nil
	}
	if err != nil {
		return result, fmt.Errorf("claim repository controller: %w", err)
	}
	defer c.releaseController(repositoryID, leaseValue)

	run, err := c.database.ReaderQueries.GetRepositoryScanRun(ctx, repo.GetRepositoryScanRunParams{
		RepositoryID: repositoryID, RunID: operationID,
	})
	if err != nil {
		return result, fmt.Errorf("load repository observation run: %w", err)
	}
	result.Status = run.Status
	if isTerminal(run.Status) {
		result.HasMore = false
		return result, nil
	}
	if run.CancellationRequested != 0 {
		if err := c.cancelRun(ctx, run); err != nil {
			return result, err
		}
		result.Status = StatusCancelled
		result.HasMore = false
		return result, nil
	}

	switch run.Status {
	case StatusQueued:
		status, err := c.startRun(ctx, run)
		if err != nil {
			return result, err
		}
		result.Status = status
		return result, nil
	case StatusCrawling:
		return c.crawlTurn(ctx, run, leaseValue)
	case StatusCatchingUp:
		cursor := run.CursorEnd
		if len(cursor) == 0 {
			cursor = run.CursorStart
		}
		// Drain the immutable C1 boundary before enumerating any dirty
		// frontier. This coalesces any number of hints for one directory into
		// one durable verification instead of repeatedly walking it once per
		// adapter page.
		if len(run.CursorTarget) == 0 || !bytes.Equal(cursor, run.CursorTarget) {
			return c.catchUpTurn(ctx, run)
		}
		open, err := c.database.ReaderQueries.CountOpenRepositoryScanFrontier(ctx, run.RunID)
		if err != nil {
			return result, fmt.Errorf("count repository verification frontier: %w", err)
		}
		if open > 0 {
			return c.crawlTurn(ctx, run, leaseValue)
		}
		return c.catchUpTurn(ctx, run)
	case StatusFinalizing:
		return c.finalizeTurn(ctx, run, leaseValue)
	default:
		return result, fmt.Errorf("unsupported repository observation status %q", run.Status)
	}
}

func (c *Controller) startRun(ctx context.Context, run repo.RepositoryScanRun) (string, error) {
	repository, err := c.database.ReaderQueries.GetRepository(ctx, run.RepositoryID)
	if err != nil {
		return "", fmt.Errorf("load repository before change capture: %w", err)
	}
	state, err := c.database.ReaderQueries.GetRepositoryObservationState(ctx, run.RepositoryID)
	if err != nil {
		return "", fmt.Errorf("load repository observation state: %w", err)
	}
	checkpoint, captureErr := c.feed.Snapshot(ctx, repository)
	if captureErr != nil {
		c.logger.Warn("repository change capture unavailable",
			zap.String("repository_id", run.RepositoryID.String()), zap.Error(captureErr))
		checkpoint = changefeed.Checkpoint{AdapterKind: "periodic", Health: changefeed.HealthUnavailable}
	}
	checkpoint = normalizedCheckpoint(checkpoint)
	fullVerification := run.ForceFullVerification != 0 || state.FullVerificationRequired != 0 || !checkpoint.Valid()
	start := checkpoint
	if checkpoint.Valid() {
		persisted, cursorErr := c.database.ReaderQueries.GetRepositoryChangeCursor(ctx, repo.GetRepositoryChangeCursorParams{
			RepositoryID: run.RepositoryID, AdapterKind: checkpoint.AdapterKind,
		})
		switch {
		case cursorErr == nil:
			prior := checkpointFromCatalog(persisted, checkpoint.VolumeKind)
			if prior.Valid() && prior.VolumeIdentity != checkpoint.VolumeIdentity {
				// A different mounted volume is not an ordinary journal gap. Keep
				// the crawl positive-only and require the moved/copied repository
				// control flow before any absence can be authoritative.
				checkpoint.Health = changefeed.HealthGap
				fullVerification = true
			} else if !prior.Valid() || !prior.SameIdentity(checkpoint) {
				fullVerification = true
			} else if !fullVerification {
				start = prior
			}
		case !errors.Is(cursorErr, sql.ErrNoRows):
			return "", fmt.Errorf("load repository change cursor: %w", cursorErr)
		default:
			fullVerification = true
		}
	}
	volumeKind := normalizeVolumeKind(checkpoint.VolumeKind)
	_, err = c.database.Queries.UpdateRepositoryObservationAdapter(ctx, repo.UpdateRepositoryObservationAdapterParams{
		RepositoryID: run.RepositoryID, AdapterKind: checkpoint.AdapterKind,
		AdapterIdentity: optionalString(checkpoint.JournalIdentity),
		VolumeIdentity:  optionalString(checkpoint.VolumeIdentity), VolumeKind: volumeKind,
		CursorHealth: string(checkpoint.Health), FullVerificationRequired: boolInt(fullVerification),
		UpdatedAt: dbtypes.NewTimestamp(c.now()),
	})
	if err != nil {
		return "", fmt.Errorf("persist repository change adapter: %w", err)
	}

	initialStatus := StatusCatchingUp
	cursorTarget := []byte{}
	if fullVerification {
		initialStatus = StatusCrawling
		start = checkpoint
	} else {
		cursorTarget = append([]byte(nil), checkpoint.Cursor...)
	}
	now := dbtypes.NewTimestamp(c.now())
	err = c.database.WithTx(ctx, catalogtx.OperationRepositoryObservationStart, func(_ *sql.Tx, queries *repo.Queries) error {
		if fullVerification {
			rootNode, rootErr := queries.GetRepositoryRootNode(ctx, run.RepositoryID)
			if errors.Is(rootErr, sql.ErrNoRows) {
				revision, revisionErr := queries.AllocateRepositoryObservationRevision(ctx, repo.AllocateRepositoryObservationRevisionParams{
					RepositoryID: run.RepositoryID, UpdatedAt: now,
				})
				if revisionErr != nil {
					return revisionErr
				}
				rootNode, rootErr = queries.InsertRepositoryRootNode(ctx, repo.InsertRepositoryRootNodeParams{
					NodeID: uuid.New(), RepositoryID: run.RepositoryID,
					ObservationRevision: revision, CreatedAt: now,
				})
			}
			if rootErr != nil {
				return fmt.Errorf("ensure repository root node: %w", rootErr)
			}
			if _, enqueueErr := queries.EnqueueRepositoryScanFrontier(ctx, repo.EnqueueRepositoryScanFrontierParams{
				RunID: run.RunID, DirectoryNodeID: rootNode.NodeID.String(), Purpose: "crawl", CreatedAt: now,
			}); enqueueErr != nil {
				return fmt.Errorf("enqueue repository root frontier: %w", enqueueErr)
			}
		}
		if _, startErr := queries.StartRepositoryScanRun(ctx, repo.StartRepositoryScanRunParams{
			RunID: run.RunID, Status: initialStatus, UpdatedAt: now,
			CursorStart: start.Cursor, CursorTarget: cursorTarget,
			VolumeIdentity: optionalString(checkpoint.VolumeIdentity),
		}); startErr != nil {
			return fmt.Errorf("start repository observation run: %w", startErr)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return initialStatus, nil
}

func (c *Controller) crawlTurn(
	ctx context.Context,
	run repo.RepositoryScanRun,
	controllerLease string,
) (TurnResult, error) {
	result := TurnResult{OperationID: run.RunID, Status: StatusCrawling, HasMore: true}
	outboxDepth, err := c.database.ReaderQueries.CountPendingRepositoryOutbox(ctx, run.RepositoryID)
	if err != nil {
		return result, fmt.Errorf("count repository outbox: %w", err)
	}
	if outboxDepth >= c.cfg.OutboxHighWaterMark {
		result.Backpressure = true
		return result, nil
	}

	nowTime := c.now()
	now := dbtypes.NewTimestamp(nowTime)
	frontierLease := controllerLease + ":frontier"
	frontierExpiry := nowTime.Add(c.cfg.ControllerLease).UnixMicro()
	frontier, err := c.database.Queries.ClaimRepositoryScanFrontier(ctx, repo.ClaimRepositoryScanFrontierParams{
		RunID: run.RunID, LeaseID: &frontierLease,
		LeaseExpiresAt: &frontierExpiry, UpdatedAt: now,
	})
	if errors.Is(err, sql.ErrNoRows) {
		if run.Status == StatusCrawling {
			if err := c.transitionRun(ctx, run.RunID, StatusCrawling, StatusCatchingUp, run.PartialCoverage, nil); err != nil {
				return result, err
			}
			result.Status = StatusCatchingUp
		}
		return result, nil
	}
	if err != nil {
		return result, fmt.Errorf("claim repository scan frontier: %w", err)
	}
	directoryNodeID, err := uuid.Parse(frontier.DirectoryNodeID)
	if err != nil {
		return result, fmt.Errorf("invalid frontier node ID: %w", err)
	}
	directoryPath, err := nodegraph.ProjectPath(ctx, c.database.ReaderQueries, run.RepositoryID, directoryNodeID)
	if err != nil {
		return result, c.failFrontier(ctx, run, frontier, frontierLease, "node_path_unavailable", err)
	}
	repository, err := c.database.ReaderQueries.GetRepository(ctx, run.RepositoryID)
	if err != nil {
		return result, fmt.Errorf("load repository for crawl: %w", err)
	}
	if watcher, ok := c.feed.(changefeed.DirectoryWatcher); ok {
		if err := watcher.WatchDirectory(ctx, repository, directoryPath); err != nil {
			return result, c.failFrontier(ctx, run, frontier, frontierLease, "change_capture_failed", err)
		}
	}
	batch, enumerateErr := c.directories.ReadDirectory(ctx, repository, storage.DirectoryReadOptions{
		Directory: directoryPath, Offset: frontier.ContinuationOffset,
		Limit: c.cfg.BatchSize, ScanID: run.RunID, Settle: c.cfg.Settle, Now: nowTime,
	})
	if enumerateErr != nil {
		return result, c.failFrontier(ctx, run, frontier, frontierLease, "directory_enumeration_failed", enumerateErr)
	}

	rows, bytesQueued, transactionDuration, commitDuration, applyErr := c.applyDirectoryBatch(
		ctx, run, directoryNodeID, frontier, frontierLease, batch,
	)
	result.RowsApplied = rows
	result.BytesQueued = bytesQueued
	result.TransactionDuration = transactionDuration
	result.CommitDuration = commitDuration
	if applyErr != nil {
		return result, applyErr
	}
	return result, nil
}

func (c *Controller) applyDirectoryBatch(
	ctx context.Context,
	run repo.RepositoryScanRun,
	parentNodeID uuid.UUID,
	frontier repo.RepositoryScanFrontier,
	frontierLease string,
	batch storage.DirectoryReadBatch,
) (rowsApplied int, bytesQueued int64, transactionDuration, commitDuration time.Duration, resultErr error) {
	transactionStarted := time.Now()
	statementsStarted := transactionStarted
	statementsFinished := transactionStarted
	now := dbtypes.NewTimestamp(c.now())
	processedOffset := frontier.ContinuationOffset
	allObservationsApplied := true
	var directoryCount, fileCount int64
	resultErr = c.database.WithTx(ctx, catalogtx.OperationRepositoryObservationApplyDirectoryBatch, func(_ *sql.Tx, queries *repo.Queries) error {
		statementsStarted = time.Now()
		defer func() { statementsFinished = time.Now() }()
		firstRevision := int64(0)
		if len(batch.Entries) > 0 {
			var err error
			firstRevision, err = queries.AllocateRepositoryObservationRevisionRange(ctx, repo.AllocateRepositoryObservationRevisionRangeParams{
				RepositoryID: run.RepositoryID, NextRevision: int64(len(batch.Entries)), UpdatedAt: now,
			})
			if err != nil {
				return fmt.Errorf("allocate observation revision range: %w", err)
			}
		}
		state, err := queries.GetRepositoryObservationState(ctx, run.RepositoryID)
		if err != nil {
			return err
		}
		repository, err := queries.GetRepository(ctx, run.RepositoryID)
		if err != nil {
			return err
		}
		ownerID := nullableOwner(repository.DefaultOwnerID)
		semantics := pathsemantics.Semantics{
			Case:          pathsemantics.CaseMode(state.PathCaseMode),
			Normalization: pathsemantics.Normalization(state.PathNormalization),
		}
		for index, directoryEntry := range batch.Entries {
			if rowsApplied > 0 && time.Since(statementsStarted) >= c.cfg.TransactionBudget {
				allObservationsApplied = false
				break
			}
			observation := directoryEntry.Observation
			name := path.Base(observation.Path.String())
			nameKey, err := semantics.NameKey(name)
			if err != nil {
				return err
			}
			sourceEventKey := fmt.Sprintf("crawl:%s:%s:%d", run.RunID, parentNodeID, directoryEntry.NextOffset)
			if _, err := queries.GetRepositoryObservationBySourceEvent(ctx, repo.GetRepositoryObservationBySourceEventParams{
				RepositoryID: run.RepositoryID, Source: "crawl", SourceEventKey: &sourceEventKey,
			}); err == nil {
				processedOffset = directoryEntry.NextOffset
				continue
			} else if !errors.Is(err, sql.ErrNoRows) {
				return err
			}

			revision := firstRevision + int64(index)
			node, existing, err := c.resolveNode(ctx, queries, run.RepositoryID, parentNodeID, nameKey, observation)
			if err != nil {
				return err
			}
			beforeToken := existingToken(existing)
			nodeKind := observationNodeKind(observation.EntryKind)
			volumeIdentity := observationVolumeIdentity(observation)
			fileSize := observation.Size
			updated, err := queries.UpsertRepositoryNodeObservation(ctx, repo.UpsertRepositoryNodeObservationParams{
				NodeID: node.NodeID, RepositoryID: run.RepositoryID,
				ParentNodeID: uuid.NullUUID{UUID: parentNodeID, Valid: true},
				Name:         name, NameKey: nameKey, Kind: nodeKind,
				NativeIdentityKind:  observation.FileIdentityKind,
				NativeIdentityValue: observation.FileIdentity, VolumeIdentity: volumeIdentity,
				ObservationRevision: revision, StabilityToken: &observation.ObservationToken,
				FileSize: &fileSize, ModifiedAtNs: &observation.ModTimeNS,
				ChangedAtNs:   observation.ChangeTimeNS,
				LastSeenRunID: uuid.NullUUID{UUID: run.RunID, Valid: true}, CreatedAt: now,
			})
			if err != nil {
				return fmt.Errorf("apply repository node observation: %w", err)
			}
			pathHint := observation.Path.String()
			entryKind := nodeKind
			persisted, err := queries.InsertRepositoryObservation(ctx, repo.InsertRepositoryObservationParams{
				ObservationID: uuid.New(), RepositoryID: run.RepositoryID, Revision: revision,
				RunID: uuid.NullUUID{UUID: run.RunID, Valid: true}, Source: "crawl",
				SourceEventKey: &sourceEventKey, PathHint: &pathHint,
				ParentNodeID: uuid.NullUUID{UUID: parentNodeID, Valid: true},
				Name:         &name, NameKey: &nameKey, EntryKind: &entryKind,
				FileSize: &fileSize, ModifiedAtNs: &observation.ModTimeNS,
				ChangedAtNs:          observation.ChangeTimeNS,
				NativeIdentityKind:   observation.FileIdentityKind,
				NativeIdentityValue:  observation.FileIdentity,
				StabilityTokenBefore: beforeToken,
				StabilityTokenAfter:  &observation.ObservationToken,
				ResolvedOwnerID:      ownerID,
				MappedNodeID:         uuid.NullUUID{UUID: updated.NodeID, Valid: true},
				CreatedAt:            now,
			})
			if err != nil {
				return fmt.Errorf("persist repository observation: %w", err)
			}
			processingState := "applied"
			var failureCode *string
			if nodeKind == "file" && ownerID == nil {
				processingState = "terminal_unsupported"
				code := "default_owner_required"
				failureCode = &code
			}
			if _, err := queries.CompleteRepositoryObservationCAS(ctx, repo.CompleteRepositoryObservationCASParams{
				RepositoryID: run.RepositoryID, ObservationID: persisted.ObservationID,
				MappedNodeID:    uuid.NullUUID{UUID: updated.NodeID, Valid: true},
				ProcessingState: processingState, FailureCode: failureCode,
				ProcessedAt: int64Ptr(now.Time.UnixMicro()),
			}); err != nil {
				return fmt.Errorf("complete repository observation: %w", err)
			}

			switch nodeKind {
			case "directory":
				directoryCount++
				if frontier.Purpose == "crawl" || existing == nil {
					if _, err := queries.EnqueueRepositoryScanFrontier(ctx, repo.EnqueueRepositoryScanFrontierParams{
						RunID: run.RunID, DirectoryNodeID: updated.NodeID.String(), Purpose: "crawl", CreatedAt: now,
					}); err != nil {
						return fmt.Errorf("enqueue discovered directory: %w", err)
					}
				}
			case "file":
				fileCount++
				needsHash := beforeToken == nil || *beforeToken != observation.ObservationToken
				if !needsHash {
					if _, locationErr := queries.GetActiveAssetLocationByNode(ctx, updated.NodeID); errors.Is(locationErr, sql.ErrNoRows) {
						needsHash = true
					} else if locationErr != nil {
						return locationErr
					}
				}
				if needsHash && ownerID != nil {
					effectKey := fmt.Sprintf("hash:%s:%d", updated.NodeID, revision)
					if _, err := queries.InsertRepositoryOutboxEffect(ctx, repo.InsertRepositoryOutboxEffectParams{
						OutboxID: uuid.New(), RepositoryID: run.RepositoryID,
						EffectKey: effectKey, EffectKind: "hash", EntityID: updated.NodeID.String(),
						ExpectedRevision: revision, Payload: `{}`, CreatedAt: now,
					}); err != nil {
						return fmt.Errorf("publish stable hash effect: %w", err)
					}
					bytesQueued += observation.Size
				}
			}
			processedOffset = directoryEntry.NextOffset
			rowsApplied++
		}

		if allObservationsApplied {
			processedOffset = batch.NextOffset
		}
		coverageSafe := batch.Authoritative && frontier.CoverageSafe != 0
		finished := batch.Done && allObservationsApplied
		if finished {
			if coverageSafe {
				if err := c.recordAuthoritativeCoverage(ctx, queries, run, parentNodeID, now); err != nil {
					return err
				}
				rowsApplied++
			}
			if _, err := queries.CompleteRepositoryScanFrontier(ctx, repo.CompleteRepositoryScanFrontierParams{
				RunID: run.RunID, DirectoryNodeID: frontier.DirectoryNodeID,
				AuthoritativeChildSet: boolInt(coverageSafe), UpdatedAt: now,
				LeaseID: &frontierLease,
			}); err != nil {
				return fmt.Errorf("complete repository scan frontier: %w", err)
			}
		} else {
			if processedOffset <= frontier.ContinuationOffset {
				return errors.New("repository directory page made no durable progress")
			}
			if _, err := queries.ContinueRepositoryScanFrontier(ctx, repo.ContinueRepositoryScanFrontierParams{
				RunID: run.RunID, DirectoryNodeID: frontier.DirectoryNodeID,
				ContinuationOffset: processedOffset, CoverageSafe: boolInt(batch.Authoritative),
				UpdatedAt: now, LeaseID: &frontierLease,
			}); err != nil {
				return fmt.Errorf("checkpoint repository scan frontier: %w", err)
			}
		}
		outboxDepth, err := queries.CountPendingRepositoryOutbox(ctx, run.RepositoryID)
		if err != nil {
			return err
		}
		if _, err := queries.UpdateRepositoryScanRunProgress(ctx, repo.UpdateRepositoryScanRunProgressParams{
			RunID: run.RunID, DirectoriesObserved: directoryCount, FilesObserved: fileCount,
			BytesQueued: bytesQueued, AuthoritativeDirectories: boolInt(finished && coverageSafe),
			ErrorDirectories: boolInt(finished && !coverageSafe), OutboxDepth: outboxDepth, UpdatedAt: now,
		}); err != nil {
			return fmt.Errorf("update repository observation progress: %w", err)
		}
		return nil
	})
	transactionDuration = statementsFinished.Sub(statementsStarted)
	totalDuration := time.Since(transactionStarted)
	if totalDuration > transactionDuration {
		commitDuration = totalDuration - transactionDuration
	}
	if transactionDuration > c.cfg.TransactionBudget {
		c.logger.Warn("repository observation statements exceeded budget",
			zap.String("repository_id", run.RepositoryID.String()),
			zap.String("operation_id", run.RunID.String()),
			zap.Int("rows", rowsApplied), zap.Duration("duration", transactionDuration),
		)
	}
	if commitDuration > maximumTransactionBudget {
		c.logger.Warn("repository observation begin/commit overhead exceeded budget",
			zap.String("repository_id", run.RepositoryID.String()),
			zap.String("operation_id", run.RunID.String()),
			zap.Int("rows", rowsApplied), zap.Duration("duration", commitDuration),
		)
	}
	return rowsApplied, bytesQueued, transactionDuration, commitDuration, resultErr
}

func (c *Controller) resolveNode(
	ctx context.Context,
	queries *repo.Queries,
	repositoryID, parentNodeID uuid.UUID,
	nameKey string,
	observation storage.FileObservation,
) (repo.RepositoryNode, *repo.RepositoryNode, error) {
	byName, nameErr := queries.GetActiveRepositoryChildByName(ctx, repo.GetActiveRepositoryChildByNameParams{
		RepositoryID: repositoryID,
		ParentNodeID: uuid.NullUUID{UUID: parentNodeID, Valid: true}, NameKey: nameKey,
	})
	if nameErr != nil && !errors.Is(nameErr, sql.ErrNoRows) {
		return repo.RepositoryNode{}, nil, nameErr
	}
	if nameErr == nil {
		return byName, &byName, nil
	}
	volumeIdentity := observationVolumeIdentity(observation)
	if volumeIdentity != nil && observation.FileIdentityKind != nil && observation.FileIdentity != nil {
		matches, err := queries.ListActiveRepositoryNodesByNativeIdentity(ctx, repo.ListActiveRepositoryNodesByNativeIdentityParams{
			RepositoryID: repositoryID, VolumeIdentity: volumeIdentity,
			NativeIdentityKind:  observation.FileIdentityKind,
			NativeIdentityValue: observation.FileIdentity,
		})
		if err != nil {
			return repo.RepositoryNode{}, nil, err
		}
		if len(matches) == 1 {
			return matches[0], &matches[0], nil
		}
	}
	created := repo.RepositoryNode{NodeID: uuid.New(), RepositoryID: repositoryID}
	return created, nil, nil
}

func (c *Controller) recordAuthoritativeCoverage(
	ctx context.Context,
	queries *repo.Queries,
	run repo.RepositoryScanRun,
	directoryNodeID uuid.UUID,
	now dbtypes.Timestamp,
) error {
	eventKey := fmt.Sprintf("coverage:%s:%s", run.RunID, directoryNodeID)
	prior, err := queries.GetRepositoryObservationBySourceEvent(ctx, repo.GetRepositoryObservationBySourceEventParams{
		RepositoryID: run.RepositoryID, Source: "verifier", SourceEventKey: &eventKey,
	})
	if err == nil {
		if _, err := queries.UpdateRepositoryDirectoryCoverageCAS(ctx, repo.UpdateRepositoryDirectoryCoverageCASParams{
			RepositoryID: run.RepositoryID, NodeID: directoryNodeID,
			LastAuthoritativeCoverageRevision: prior.Revision, UpdatedAt: now,
		}); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("restore repository directory coverage: %w", err)
		}
		if prior.ProcessingState != "applied" {
			if _, err := queries.CompleteRepositoryObservationCAS(ctx, repo.CompleteRepositoryObservationCASParams{
				RepositoryID: run.RepositoryID, ObservationID: prior.ObservationID,
				MappedNodeID:    uuid.NullUUID{UUID: directoryNodeID, Valid: true},
				ProcessingState: "applied", ProcessedAt: int64Ptr(now.Time.UnixMicro()),
			}); err != nil && !errors.Is(err, sql.ErrNoRows) {
				return err
			}
		}
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	revision, err := queries.AllocateRepositoryObservationRevision(ctx, repo.AllocateRepositoryObservationRevisionParams{
		RepositoryID: run.RepositoryID, UpdatedAt: now,
	})
	if err != nil {
		return fmt.Errorf("allocate repository directory coverage revision: %w", err)
	}
	if _, err := queries.UpdateRepositoryDirectoryCoverageCAS(ctx, repo.UpdateRepositoryDirectoryCoverageCASParams{
		RepositoryID: run.RepositoryID, NodeID: directoryNodeID,
		LastAuthoritativeCoverageRevision: revision, UpdatedAt: now,
	}); err != nil {
		return fmt.Errorf("record repository directory coverage: %w", err)
	}
	entryKind := "directory"
	observation, err := queries.InsertRepositoryObservation(ctx, repo.InsertRepositoryObservationParams{
		ObservationID: uuid.New(), RepositoryID: run.RepositoryID, Revision: revision,
		RunID: uuid.NullUUID{UUID: run.RunID, Valid: true}, Source: "verifier",
		SourceEventKey: &eventKey, EntryKind: &entryKind,
		MappedNodeID:          uuid.NullUUID{UUID: directoryNodeID, Valid: true},
		AuthoritativeChildSet: 1, CreatedAt: now,
	})
	if err != nil {
		return fmt.Errorf("persist repository directory coverage: %w", err)
	}
	if _, err := queries.CompleteRepositoryObservationCAS(ctx, repo.CompleteRepositoryObservationCASParams{
		RepositoryID: run.RepositoryID, ObservationID: observation.ObservationID,
		MappedNodeID:    uuid.NullUUID{UUID: directoryNodeID, Valid: true},
		ProcessingState: "applied", ProcessedAt: int64Ptr(now.Time.UnixMicro()),
	}); err != nil {
		return fmt.Errorf("complete repository directory coverage: %w", err)
	}
	return nil
}

func (c *Controller) catchUpTurn(ctx context.Context, run repo.RepositoryScanRun) (TurnResult, error) {
	result := TurnResult{OperationID: run.RunID, Status: StatusCatchingUp, HasMore: true}
	repository, err := c.database.ReaderQueries.GetRepository(ctx, run.RepositoryID)
	if err != nil {
		return result, fmt.Errorf("load repository for change catch-up: %w", err)
	}
	state, err := c.database.ReaderQueries.GetRepositoryObservationState(ctx, run.RepositoryID)
	if err != nil {
		return result, fmt.Errorf("load repository adapter state: %w", err)
	}
	current, snapshotErr := c.feed.Snapshot(ctx, repository)
	if snapshotErr != nil {
		return c.finishUnsafeCatchup(ctx, run, changefeed.HealthUnavailable, snapshotErr)
	}
	current = normalizedCheckpoint(current)
	from := changefeed.Checkpoint{
		AdapterKind: state.AdapterKind, Cursor: append([]byte(nil), run.CursorEnd...),
		VolumeIdentity: valueOrEmpty(state.VolumeIdentity), VolumeKind: state.VolumeKind,
		JournalIdentity: valueOrEmpty(state.AdapterIdentity), Health: changefeed.Health(state.CursorHealth),
	}
	if len(from.Cursor) == 0 {
		from.Cursor = append([]byte(nil), run.CursorStart...)
	}
	if !from.Valid() || !current.Valid() || !from.SameIdentity(current) {
		return c.finishUnsafeCatchup(ctx, run, cursorFailureHealth(from, current), changefeed.ErrCursorInvalid)
	}
	if len(run.CursorTarget) == 0 {
		captured, captureErr := c.database.Queries.CaptureRepositoryScanRunCursorTarget(ctx, repo.CaptureRepositoryScanRunCursorTargetParams{
			RunID: run.RunID, CursorTarget: current.Cursor, UpdatedAt: dbtypes.NewTimestamp(c.now()),
		})
		if captureErr != nil {
			return result, fmt.Errorf("capture repository catch-up boundary: %w", captureErr)
		}
		run.CursorTarget = append([]byte(nil), captured.CursorTarget...)
	}
	through := current
	through.Cursor = append([]byte(nil), run.CursorTarget...)

	// Directory enumeration can amortize a larger read page because it happens
	// outside SQLite. A native change event expands into several catalog
	// statements inside applyChangeBatch, so cap that commit quantum separately
	// and yield the sole writer between small change-feed turns.
	changeBatchSize := min(c.cfg.BatchSize, maximumChangeBatchSize)
	batch, err := c.feed.Read(ctx, repository, from, through, changeBatchSize)
	if err != nil {
		health := changefeed.HealthGap
		switch batch.Next.Health {
		case changefeed.HealthOverflow:
			health = changefeed.HealthOverflow
		case changefeed.HealthUnavailable:
			health = changefeed.HealthUnavailable
		}
		return c.finishUnsafeCatchup(ctx, run, health, err)
	}
	batch.Next = normalizedCheckpoint(batch.Next)
	if !batch.Next.Valid() || !batch.Next.SameIdentity(through) {
		return c.finishUnsafeCatchup(ctx, run, changefeed.HealthGap, changefeed.ErrCursorInvalid)
	}
	if len(batch.Events) == 0 && !batch.Done && batch.Next.SamePosition(from) {
		return result, errors.New("repository change adapter made no cursor progress")
	}
	if err := validateChangeBatch(batch); err != nil {
		return c.finishUnsafeCatchupWithCode(
			ctx,
			run,
			changefeed.HealthGap,
			"change_event_invalid",
			err,
		)
	}
	if err := c.applyChangeBatch(ctx, run, state, batch); err != nil {
		return result, err
	}
	if len(batch.Events) > 0 || !batch.Done {
		return result, nil
	}
	if err := c.transitionRun(ctx, run.RunID, StatusCatchingUp, StatusFinalizing, run.PartialCoverage, nil); err != nil {
		return result, err
	}
	result.Status = StatusFinalizing
	return result, nil
}

func (c *Controller) finishUnsafeCatchup(
	ctx context.Context,
	run repo.RepositoryScanRun,
	health changefeed.Health,
	cause error,
) (TurnResult, error) {
	return c.finishUnsafeCatchupWithCode(
		ctx,
		run,
		health,
		"change_cursor_"+string(health),
		cause,
	)
}

func (c *Controller) finishUnsafeCatchupWithCode(
	ctx context.Context,
	run repo.RepositoryScanRun,
	health changefeed.Health,
	failureCode string,
	cause error,
) (TurnResult, error) {
	result := TurnResult{OperationID: run.RunID, Status: StatusFinalizing, HasMore: true}
	state, stateErr := c.database.ReaderQueries.GetRepositoryObservationState(ctx, run.RepositoryID)
	if stateErr != nil {
		return result, stateErr
	}
	_, updateErr := c.database.Queries.UpdateRepositoryObservationAdapter(ctx, repo.UpdateRepositoryObservationAdapterParams{
		RepositoryID: run.RepositoryID, AdapterKind: state.AdapterKind,
		AdapterIdentity: state.AdapterIdentity, VolumeIdentity: state.VolumeIdentity,
		VolumeKind: normalizeVolumeKind(state.VolumeKind), CursorHealth: string(health),
		FullVerificationRequired: 1, UpdatedAt: dbtypes.NewTimestamp(c.now()),
	})
	if updateErr != nil {
		return result, updateErr
	}
	if transitionErr := c.transitionRun(ctx, run.RunID, StatusCatchingUp, StatusFinalizing, 1, &failureCode); transitionErr != nil {
		return result, transitionErr
	}
	c.logger.Warn("repository change catch-up is not authoritative",
		zap.String("repository_id", run.RepositoryID.String()),
		zap.String("operation_id", run.RunID.String()), zap.String("cursor_health", string(health)),
		zap.String("failure_code", failureCode), zap.Error(cause))
	return result, nil
}

func validateChangeBatch(batch changefeed.Batch) error {
	for index, event := range batch.Events {
		if strings.TrimSpace(event.Key) == "" {
			return fmt.Errorf("change event %d: event key is required", index)
		}
		switch event.Kind {
		case changefeed.EventCreate, changefeed.EventModify, changefeed.EventRemove, changefeed.EventRename:
		default:
			return fmt.Errorf("change event %d: unsupported event kind %q", index, event.Kind)
		}
		if _, err := normalizeEventPath(event.Path); err != nil {
			return fmt.Errorf("change event %d path: %w", index, err)
		}
		if event.OldPath != "" {
			if _, err := normalizeEventPath(event.OldPath); err != nil {
				return fmt.Errorf("change event %d old path: %w", index, err)
			}
		}
	}
	return nil
}

func (c *Controller) applyChangeBatch(
	ctx context.Context,
	run repo.RepositoryScanRun,
	state repo.RepositoryObservationState,
	batch changefeed.Batch,
) error {
	now := dbtypes.NewTimestamp(c.now())
	return c.database.WithTx(ctx, catalogtx.OperationRepositoryObservationApplyChangeBatch, func(_ *sql.Tx, queries *repo.Queries) error {
		firstRevision := int64(0)
		if len(batch.Events) > 0 {
			var err error
			firstRevision, err = queries.AllocateRepositoryObservationRevisionRange(ctx, repo.AllocateRepositoryObservationRevisionRangeParams{
				RepositoryID: run.RepositoryID, NextRevision: int64(len(batch.Events)), UpdatedAt: now,
			})
			if err != nil {
				return err
			}
		}
		semantics := pathsemantics.Semantics{
			Case:          pathsemantics.CaseMode(state.PathCaseMode),
			Normalization: pathsemantics.Normalization(state.PathNormalization),
		}
		for index, event := range batch.Events {
			if strings.TrimSpace(event.Key) == "" {
				return errors.New("repository change event key is required")
			}
			pathHint, err := normalizeEventPath(event.Path)
			if err != nil {
				return err
			}
			source := "watcher"
			if state.AdapterKind == "usn" || state.AdapterKind == "fsevents" {
				source = "journal"
			}
			cursor := event.Cursor
			if len(cursor) == 0 {
				cursor = batch.Next.Cursor
			}
			persisted, err := queries.InsertRepositoryObservation(ctx, repo.InsertRepositoryObservationParams{
				ObservationID: uuid.New(), RepositoryID: run.RepositoryID,
				Revision: firstRevision + int64(index), RunID: uuid.NullUUID{UUID: run.RunID, Valid: true},
				Source: source, SourceEventKey: stringPtr(event.Key), SourceCursor: cursor,
				PathHint: stringPtr(pathHint), CreatedAt: now,
			})
			if err != nil {
				return fmt.Errorf("persist native change observation: %w", err)
			}
			if persisted.ProcessingState != "applied" {
				if _, err := queries.CompleteRepositoryObservationCAS(ctx, repo.CompleteRepositoryObservationCASParams{
					RepositoryID: run.RepositoryID, ObservationID: persisted.ObservationID,
					ProcessingState: "applied", ProcessedAt: int64Ptr(now.Time.UnixMicro()),
				}); err != nil && !errors.Is(err, sql.ErrNoRows) {
					return err
				}
			}
			for _, dirty := range dirtyPrefixes(event) {
				directory, resolveErr := resolveClosestDirectory(ctx, queries, run.RepositoryID, dirty.path, semantics)
				if resolveErr != nil {
					return resolveErr
				}
				purpose := "verify"
				if dirty.recursive {
					purpose = "crawl"
				}
				if _, queueErr := queries.RequeueRepositoryScanFrontierForVerification(ctx, repo.RequeueRepositoryScanFrontierForVerificationParams{
					RunID: run.RunID, DirectoryNodeID: directory.NodeID.String(), Purpose: purpose, CreatedAt: now,
				}); queueErr != nil && !errors.Is(queueErr, sql.ErrNoRows) {
					return fmt.Errorf("queue dirty repository prefix: %w", queueErr)
				}
			}
		}
		_, err := queries.UpdateRepositoryScanRunCursor(ctx, repo.UpdateRepositoryScanRunCursorParams{
			RunID: run.RunID, CursorEnd: batch.Next.Cursor,
			VolumeIdentity: optionalString(batch.Next.VolumeIdentity), UpdatedAt: now,
		})
		return err
	})
}

type dirtyPrefix struct {
	path      string
	recursive bool
}

func dirtyPrefixes(event changefeed.Event) []dirtyPrefix {
	unique := make(map[string]bool)
	addParent := func(value string) {
		value, err := normalizeEventPath(value)
		if err != nil {
			return
		}
		parent := path.Dir(value)
		if parent == "." {
			parent = ""
		}
		unique[parent] = unique[parent] || false
	}
	addParent(event.Path)
	if event.OldPath != "" {
		addParent(event.OldPath)
	}
	if event.Recursive {
		if value, err := normalizeEventPath(event.Path); err == nil {
			unique[value] = true
		}
	}
	paths := make([]string, 0, len(unique))
	for value := range unique {
		paths = append(paths, value)
	}
	sort.Strings(paths)
	result := make([]dirtyPrefix, 0, len(paths))
	for _, value := range paths {
		result = append(result, dirtyPrefix{path: value, recursive: unique[value]})
	}
	return result
}

func resolveClosestDirectory(
	ctx context.Context,
	queries *repo.Queries,
	repositoryID uuid.UUID,
	relativePath string,
	semantics pathsemantics.Semantics,
) (repo.RepositoryNode, error) {
	current, err := queries.GetRepositoryRootNode(ctx, repositoryID)
	if err != nil {
		return repo.RepositoryNode{}, err
	}
	if relativePath == "" {
		return current, nil
	}
	for _, component := range strings.Split(relativePath, "/") {
		nameKey, keyErr := semantics.NameKey(component)
		if keyErr != nil {
			return repo.RepositoryNode{}, keyErr
		}
		next, childErr := queries.GetActiveRepositoryChildByName(ctx, repo.GetActiveRepositoryChildByNameParams{
			RepositoryID: repositoryID,
			ParentNodeID: uuid.NullUUID{UUID: current.NodeID, Valid: true}, NameKey: nameKey,
		})
		if errors.Is(childErr, sql.ErrNoRows) {
			return current, nil
		}
		if childErr != nil {
			return repo.RepositoryNode{}, childErr
		}
		if next.Kind != "directory" {
			return current, nil
		}
		current = next
	}
	return current, nil
}

func normalizeEventPath(value string) (string, error) {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	if value == "" || value == "." {
		return "", nil
	}
	cleaned := path.Clean(value)
	if strings.HasPrefix(cleaned, "/") || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("native change path escapes repository: %q", value)
	}
	parsed, err := storage.ParseUserMediaPath(cleaned)
	if err != nil {
		return "", err
	}
	return parsed.String(), nil
}

func (c *Controller) finalizeTurn(
	ctx context.Context,
	run repo.RepositoryScanRun,
	controllerLease string,
) (TurnResult, error) {
	result := TurnResult{OperationID: run.RunID, Status: StatusFinalizing, HasMore: true}
	state, err := c.database.ReaderQueries.GetRepositoryObservationState(ctx, run.RepositoryID)
	if err != nil {
		return result, err
	}
	authoritative := state.CursorHealth == string(changefeed.HealthHealthy) &&
		run.PartialCoverage == 0 && run.ErrorDirectories == 0 && len(run.CursorEnd) > 0
	if authoritative {
		open, countErr := c.database.ReaderQueries.CountOpenRepositoryAbsenceFrontier(ctx, run.RunID)
		if countErr != nil {
			return result, countErr
		}
		if open > 0 {
			return c.finalizeAbsenceTurn(ctx, run, controllerLease, state.VolumeKind)
		}
	}
	return c.finishRun(ctx, run, state, authoritative)
}

func (c *Controller) finalizeAbsenceTurn(
	ctx context.Context,
	run repo.RepositoryScanRun,
	controllerLease string,
	volumeKind string,
) (TurnResult, error) {
	result := TurnResult{OperationID: run.RunID, Status: StatusFinalizing, HasMore: true}
	nowTime := c.now()
	now := dbtypes.NewTimestamp(nowTime)
	lease := controllerLease + ":absence"
	expires := nowTime.Add(c.cfg.ControllerLease).UnixMicro()
	frontier, err := c.database.Queries.ClaimRepositoryAbsenceFrontier(ctx, repo.ClaimRepositoryAbsenceFrontierParams{
		RunID: run.RunID, LeaseID: &lease, LeaseExpiresAt: &expires, UpdatedAt: now,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return result, nil
	}
	if err != nil {
		return result, fmt.Errorf("claim repository absence frontier: %w", err)
	}
	parentNodeID, err := uuid.Parse(frontier.DirectoryNodeID)
	if err != nil {
		return result, err
	}
	after := uuid.Nil
	if frontier.AbsenceCursor != "" {
		after, err = uuid.Parse(frontier.AbsenceCursor)
		if err != nil {
			return result, err
		}
	}
	transactionStarted := time.Now()
	statementsStarted := transactionStarted
	statementsFinished := transactionStarted
	err = c.database.WithTx(ctx, catalogtx.OperationRepositoryObservationFinalizeAbsence, func(_ *sql.Tx, queries *repo.Queries) error {
		statementsStarted = time.Now()
		defer func() { statementsFinished = time.Now() }()
		absenceBatchSize := min(c.cfg.BatchSize, maximumAbsenceBatchSize)
		children, listErr := queries.ListUnseenRepositoryNodeChildrenPage(ctx, repo.ListUnseenRepositoryNodeChildrenPageParams{
			RepositoryID:  run.RepositoryID,
			ParentNodeID:  uuid.NullUUID{UUID: parentNodeID, Valid: true},
			LastSeenRunID: uuid.NullUUID{UUID: run.RunID, Valid: true},
			NodeID:        after, Limit: int64(absenceBatchSize),
		})
		if listErr != nil {
			return listErr
		}
		if len(children) == 0 {
			_, completeErr := queries.CompleteRepositoryAbsenceFrontier(ctx, repo.CompleteRepositoryAbsenceFrontierParams{
				RunID: run.RunID, DirectoryNodeID: frontier.DirectoryNodeID,
				UpdatedAt: now, LeaseID: &lease,
			})
			return completeErr
		}
		eligible := make([]repo.RepositoryNode, 0, len(children))
		for _, child := range children {
			if frontier.Purpose != "absence" && requiresAbsenceSettle(volumeKind) && c.cfg.Settle > 0 {
				candidate, candidateErr := queries.MarkRepositoryNodeAbsenceCandidateCAS(ctx, repo.MarkRepositoryNodeAbsenceCandidateCASParams{
					RepositoryID: run.RepositoryID, NodeID: child.NodeID,
					AbsenceFirstObservedAt: int64Ptr(now.Time.UnixMicro()), UpdatedAt: now,
				})
				if candidateErr != nil {
					return candidateErr
				}
				if candidate.AbsenceFirstObservedAt == nil ||
					now.Time.UnixMicro()-*candidate.AbsenceFirstObservedAt < c.cfg.Settle.Microseconds() {
					continue
				}
				child = candidate
			}
			eligible = append(eligible, child)
		}
		firstRevision := int64(0)
		if len(eligible) > 0 {
			var allocateErr error
			firstRevision, allocateErr = queries.AllocateRepositoryObservationRevisionRange(ctx, repo.AllocateRepositoryObservationRevisionRangeParams{
				RepositoryID: run.RepositoryID, NextRevision: int64(len(eligible)), UpdatedAt: now,
			})
			if allocateErr != nil {
				return allocateErr
			}
		}
		eligibleVisited := 0
		allEligibleApplied := true
		for index, child := range eligible {
			if eligibleVisited > 0 && time.Since(statementsStarted) >= c.cfg.TransactionBudget {
				allEligibleApplied = false
				break
			}
			eligibleVisited++
			revision := firstRevision + int64(index)
			tombstoned, tombstoneErr := queries.TombstoneRepositoryNodeCAS(ctx, repo.TombstoneRepositoryNodeCASParams{
				RepositoryID: run.RepositoryID, NodeID: child.NodeID,
				ObservationRevision: revision, UpdatedAt: now,
			})
			if errors.Is(tombstoneErr, sql.ErrNoRows) {
				continue
			}
			if tombstoneErr != nil {
				return tombstoneErr
			}
			if _, closeErr := queries.CloseActiveAssetLocationCAS(ctx, repo.CloseActiveAssetLocationCASParams{
				NodeID: child.NodeID, UnboundObservationRevision: int64Ptr(revision), UpdatedAt: now,
			}); closeErr != nil {
				return closeErr
			}
			if tombstoned.Kind == "directory" {
				if _, enqueueErr := queries.EnqueueRepositoryAbsenceCascadeFrontier(
					ctx,
					repo.EnqueueRepositoryAbsenceCascadeFrontierParams{
						RunID: run.RunID, DirectoryNodeID: tombstoned.NodeID.String(), CreatedAt: now,
					},
				); enqueueErr != nil {
					return fmt.Errorf("enqueue repository absence cascade: %w", enqueueErr)
				}
			}
			eventKey := fmt.Sprintf("absence:%s:%s", run.RunID, child.NodeID)
			entryKind := tombstoned.Kind
			persisted, insertErr := queries.InsertRepositoryObservation(ctx, repo.InsertRepositoryObservationParams{
				ObservationID: uuid.New(), RepositoryID: run.RepositoryID, Revision: revision,
				RunID: uuid.NullUUID{UUID: run.RunID, Valid: true}, Source: "verifier",
				SourceEventKey: &eventKey,
				ParentNodeID:   uuid.NullUUID{UUID: parentNodeID, Valid: true},
				Name:           &tombstoned.Name, NameKey: &tombstoned.NameKey, EntryKind: &entryKind,
				MappedNodeID: uuid.NullUUID{UUID: tombstoned.NodeID, Valid: true}, CreatedAt: now,
			})
			if insertErr != nil {
				return insertErr
			}
			if persisted.ProcessingState != "applied" {
				if _, completeErr := queries.CompleteRepositoryObservationCAS(ctx, repo.CompleteRepositoryObservationCASParams{
					RepositoryID: run.RepositoryID, ObservationID: persisted.ObservationID,
					MappedNodeID:    uuid.NullUUID{UUID: tombstoned.NodeID, Valid: true},
					ProcessingState: "applied", ProcessedAt: int64Ptr(now.Time.UnixMicro()),
				}); completeErr != nil && !errors.Is(completeErr, sql.ErrNoRows) {
					return completeErr
				}
			}
			result.RowsApplied++
		}
		last := children[len(children)-1].NodeID.String()
		if !allEligibleApplied {
			last = eligible[eligibleVisited-1].NodeID.String()
		}
		_, continueErr := queries.ContinueRepositoryAbsenceFrontier(ctx, repo.ContinueRepositoryAbsenceFrontierParams{
			RunID: run.RunID, DirectoryNodeID: frontier.DirectoryNodeID,
			AbsenceCursor: last, UpdatedAt: now, LeaseID: &lease,
		})
		return continueErr
	})
	result.TransactionDuration = statementsFinished.Sub(statementsStarted)
	totalDuration := time.Since(transactionStarted)
	if totalDuration > result.TransactionDuration {
		result.CommitDuration = totalDuration - result.TransactionDuration
	}
	if result.TransactionDuration > c.cfg.TransactionBudget {
		c.logger.Warn("repository absence statements exceeded budget",
			zap.String("repository_id", run.RepositoryID.String()),
			zap.String("operation_id", run.RunID.String()),
			zap.Int("rows", result.RowsApplied), zap.Duration("duration", result.TransactionDuration),
		)
	}
	if result.CommitDuration > maximumTransactionBudget {
		c.logger.Warn("repository absence begin/commit overhead exceeded budget",
			zap.String("repository_id", run.RepositoryID.String()),
			zap.String("operation_id", run.RunID.String()),
			zap.Int("rows", result.RowsApplied), zap.Duration("duration", result.CommitDuration),
		)
	}
	if err != nil {
		return result, fmt.Errorf("finalize authoritative repository absences: %w", err)
	}
	return result, nil
}

func (c *Controller) finishRun(
	ctx context.Context,
	run repo.RepositoryScanRun,
	state repo.RepositoryObservationState,
	authoritative bool,
) (TurnResult, error) {
	result := TurnResult{OperationID: run.RunID, Status: StatusPartial}
	status := StatusCompleted
	partial := int64(0)
	fullVerificationRequired := int64(0)
	if !authoritative {
		status = StatusPartial
		partial = 1
		fullVerificationRequired = 1
	}
	now := dbtypes.NewTimestamp(c.now())
	err := c.database.WithTx(ctx, catalogtx.OperationRepositoryObservationFinish, func(_ *sql.Tx, queries *repo.Queries) error {
		if authoritative {
			if _, err := queries.UpsertRepositoryChangeCursor(ctx, repo.UpsertRepositoryChangeCursorParams{
				RepositoryID: run.RepositoryID, AdapterKind: state.AdapterKind,
				Cursor: run.CursorEnd, VolumeIdentity: state.VolumeIdentity,
				JournalIdentity: state.AdapterIdentity, Status: string(changefeed.HealthHealthy),
				AppliedRevision: max(int64(0), state.NextRevision-1), UpdatedAt: now,
			}); err != nil {
				return fmt.Errorf("advance repository change cursor: %w", err)
			}
		}
		if _, err := queries.TransitionRepositoryScanRun(ctx, repo.TransitionRepositoryScanRunParams{
			RunID: run.RunID, Status: status, UpdatedAt: now,
			PartialCoverage: partial, FailureCode: run.FailureCode,
			FailureProblemType: run.FailureProblemType, Status_2: StatusFinalizing,
		}); err != nil {
			return fmt.Errorf("finish repository observation run: %w", err)
		}
		state, err := queries.AdvanceRepositoryObservationEpoch(ctx, repo.AdvanceRepositoryObservationEpochParams{
			RepositoryID: run.RepositoryID, AppliedEpoch: run.RequestedEpoch,
			FullVerificationRequired: fullVerificationRequired, UpdatedAt: now,
		})
		if err != nil {
			return fmt.Errorf("advance repository observation epoch: %w", err)
		}
		return scheduleCoalescedRepositoryRun(ctx, queries, run, state, now)
	})
	if err != nil {
		return result, err
	}
	result.Status = status
	result.HasMore = false
	return result, nil
}

func (c *Controller) failFrontier(
	ctx context.Context,
	run repo.RepositoryScanRun,
	frontier repo.RepositoryScanFrontier,
	lease, failureCode string,
	cause error,
) error {
	now := dbtypes.NewTimestamp(c.now())
	return c.database.WithTx(ctx, catalogtx.OperationRepositoryObservationFailFrontier, func(_ *sql.Tx, queries *repo.Queries) error {
		if _, err := queries.CompleteRepositoryScanFrontier(ctx, repo.CompleteRepositoryScanFrontierParams{
			RunID: run.RunID, DirectoryNodeID: frontier.DirectoryNodeID,
			AuthoritativeChildSet: 0, ErrorCode: &failureCode,
			UpdatedAt: now, LeaseID: &lease,
		}); err != nil {
			return fmt.Errorf("record failed repository frontier: %w", err)
		}
		outboxDepth, err := queries.CountPendingRepositoryOutbox(ctx, run.RepositoryID)
		if err != nil {
			return err
		}
		if _, err := queries.UpdateRepositoryScanRunProgress(ctx, repo.UpdateRepositoryScanRunProgressParams{
			RunID: run.RunID, ErrorDirectories: 1, OutboxDepth: outboxDepth, UpdatedAt: now,
		}); err != nil {
			return err
		}
		c.logger.Warn("repository directory observation failed",
			zap.String("repository_id", run.RepositoryID.String()),
			zap.String("operation_id", run.RunID.String()),
			zap.String("failure_code", failureCode), zap.Error(cause),
		)
		return nil
	})
}

func (c *Controller) cancelRun(ctx context.Context, run repo.RepositoryScanRun) error {
	now := dbtypes.NewTimestamp(c.now())
	return c.database.WithTx(ctx, catalogtx.OperationRepositoryObservationCancelRun, func(_ *sql.Tx, queries *repo.Queries) error {
		if _, err := queries.TransitionRepositoryScanRun(ctx, repo.TransitionRepositoryScanRunParams{
			RunID: run.RunID, Status: StatusCancelled, UpdatedAt: now,
			PartialCoverage: 1, Status_2: run.Status,
		}); err != nil {
			return err
		}
		state, err := queries.AdvanceRepositoryObservationEpoch(ctx, repo.AdvanceRepositoryObservationEpochParams{
			RepositoryID: run.RepositoryID, AppliedEpoch: run.RequestedEpoch,
			FullVerificationRequired: 1, UpdatedAt: now,
		})
		if err != nil {
			return err
		}
		return scheduleCoalescedRepositoryRun(ctx, queries, run, state, now)
	})
}

func scheduleCoalescedRepositoryRun(
	ctx context.Context,
	queries *repo.Queries,
	completed repo.RepositoryScanRun,
	state repo.RepositoryObservationState,
	now dbtypes.Timestamp,
) error {
	if state.DesiredEpoch <= completed.RequestedEpoch {
		return nil
	}
	runID := uuid.New()
	if _, err := queries.CreateRepositoryScanRun(ctx, repo.CreateRepositoryScanRunParams{
		RunID: runID, RepositoryID: completed.RepositoryID, RequestedEpoch: state.DesiredEpoch,
		Mode: "recovery", ForceFullVerification: state.FullVerificationRequired, CreatedAt: now,
	}); err != nil {
		return fmt.Errorf("create coalesced repository observation run: %w", err)
	}
	if _, err := queries.SetActiveRepositoryObservationRun(ctx, repo.SetActiveRepositoryObservationRunParams{
		RepositoryID: completed.RepositoryID,
		ActiveRunID:  uuid.NullUUID{UUID: runID, Valid: true}, UpdatedAt: now,
	}); err != nil {
		return fmt.Errorf("activate coalesced repository observation run: %w", err)
	}
	effectKey := fmt.Sprintf("controller:%s:%d", completed.RepositoryID, state.DesiredEpoch)
	if _, err := queries.InsertRepositoryOutboxEffect(ctx, repo.InsertRepositoryOutboxEffectParams{
		OutboxID: uuid.New(), RepositoryID: completed.RepositoryID, EffectKey: effectKey,
		EffectKind: "controller", EntityID: runID.String(), ExpectedRevision: state.DesiredEpoch,
		Payload: `{}`, CreatedAt: now,
	}); err != nil {
		return fmt.Errorf("publish coalesced repository controller wakeup: %w", err)
	}
	return nil
}

func (c *Controller) transitionRun(
	ctx context.Context,
	runID uuid.UUID,
	from, to string,
	partial int64,
	failureCode *string,
) error {
	_, err := c.database.Queries.TransitionRepositoryScanRun(ctx, repo.TransitionRepositoryScanRunParams{
		RunID: runID, Status: to, UpdatedAt: dbtypes.NewTimestamp(c.now()),
		PartialCoverage: partial, FailureCode: failureCode, Status_2: from,
	})
	return err
}

func (c *Controller) releaseController(repositoryID uuid.UUID, lease string) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := c.database.Queries.ReleaseRepositoryObservationController(ctx, repo.ReleaseRepositoryObservationControllerParams{
		RepositoryID: repositoryID, ControllerLeaseID: &lease,
		UpdatedAt: dbtypes.NewTimestamp(c.now()),
	})
	if err != nil {
		c.logger.Warn("release repository observation controller lease", zap.Error(err))
	}
}

func normalizeMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "manual":
		return "manual"
	case "watcher":
		return "watcher"
	case "recovery":
		return "recovery"
	case "migration":
		return "migration"
	default:
		return "periodic"
	}
}

func observationNodeKind(kind storage.EntryKind) string {
	switch kind {
	case storage.EntryKindDirectory:
		return "directory"
	case storage.EntryKindSymlink:
		return "symlink"
	default:
		return "file"
	}
}

func observationVolumeIdentity(observation storage.FileObservation) *string {
	if observation.FileIdentityKind == nil || observation.FileIdentity == nil {
		return nil
	}
	first, _, ok := strings.Cut(*observation.FileIdentity, ":")
	if !ok || first == "" {
		return nil
	}
	value := *observation.FileIdentityKind + ":" + first
	return &value
}

func nullableOwner(ownerID *int32) *int64 {
	if ownerID == nil {
		return nil
	}
	value := int64(*ownerID)
	return &value
}

func existingToken(node *repo.RepositoryNode) *string {
	if node == nil {
		return nil
	}
	return node.StabilityToken
}

func boolInt(value bool) int64 {
	if value {
		return 1
	}
	return 0
}

func int64Ptr(value int64) *int64 { return &value }

func stringPtr(value string) *string { return &value }

func optionalString(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func normalizedCheckpoint(checkpoint changefeed.Checkpoint) changefeed.Checkpoint {
	checkpoint.AdapterKind = strings.ToLower(strings.TrimSpace(checkpoint.AdapterKind))
	switch checkpoint.AdapterKind {
	case "usn", "rdcw", "fsevents", "inotify", "periodic":
	default:
		checkpoint.AdapterKind = "periodic"
		checkpoint.Cursor = nil
		checkpoint.VolumeIdentity = ""
		checkpoint.JournalIdentity = ""
		checkpoint.Health = changefeed.HealthUnavailable
	}
	switch checkpoint.Health {
	case changefeed.HealthHealthy, changefeed.HealthGap, changefeed.HealthOverflow, changefeed.HealthUnavailable:
	default:
		checkpoint.Health = changefeed.HealthUnavailable
	}
	checkpoint.VolumeKind = normalizeVolumeKind(checkpoint.VolumeKind)
	return checkpoint
}

func normalizeVolumeKind(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "local":
		return "local"
	case "network":
		return "network"
	case "removable":
		return "removable"
	case "unsupported":
		return "unsupported"
	default:
		return "unknown"
	}
}

func requiresAbsenceSettle(volumeKind string) bool {
	return normalizeVolumeKind(volumeKind) != "local"
}

func checkpointFromCatalog(cursor repo.RepositoryChangeCursor, volumeKind string) changefeed.Checkpoint {
	return normalizedCheckpoint(changefeed.Checkpoint{
		AdapterKind: cursor.AdapterKind, Cursor: append([]byte(nil), cursor.Cursor...),
		VolumeIdentity: valueOrEmpty(cursor.VolumeIdentity), VolumeKind: volumeKind,
		JournalIdentity: valueOrEmpty(cursor.JournalIdentity), Health: changefeed.Health(cursor.Status),
	})
}

func cursorFailureHealth(from, through changefeed.Checkpoint) changefeed.Health {
	for _, health := range []changefeed.Health{from.Health, through.Health} {
		if health == changefeed.HealthOverflow {
			return changefeed.HealthOverflow
		}
	}
	for _, health := range []changefeed.Health{from.Health, through.Health} {
		if health == changefeed.HealthUnavailable {
			return changefeed.HealthUnavailable
		}
	}
	return changefeed.HealthGap
}

func isTerminal(status string) bool {
	switch status {
	case StatusCompleted, StatusPartial, StatusFailed, StatusCancelled:
		return true
	default:
		return false
	}
}
