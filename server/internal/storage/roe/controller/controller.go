// Package controller prepares and advances bounded Repository Observation
// Engine turns. Filesystem and changefeed work happens outside catalog write
// ownership; every asynchronous mutation is acknowledged by commit.Coordinator.
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

	"server/internal/commit"
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

	defaultBatchSize = 32
	maximumBatchSize = 48
	// One native event can dirty several prefixes and therefore expands into
	// several bounded catalog statements.
	maximumChangeBatchSize = 1
	// Each absence child performs several revision-fenced catalog writes.
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
		return pathsemantics.Semantics{Case: pathsemantics.CaseInsensitive, Normalization: pathsemantics.NormalizationNFC}
	default:
		return pathsemantics.Semantics{Case: pathsemantics.CaseSensitive, Normalization: pathsemantics.NormalizationNFC}
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

type committer interface {
	Submit(context.Context, commit.Intent) (commit.Result, error)
}

type Reader interface {
	GetRepositoryScanRun(context.Context, repo.GetRepositoryScanRunParams) (repo.RepositoryScanRun, error)
	CountOpenRepositoryScanFrontier(context.Context, uuid.UUID) (int64, error)
	CountOpenRepositoryAbsenceFrontier(context.Context, uuid.UUID) (int64, error)
	CountPendingRepositoryMaterialization(context.Context, uuid.UUID) (int64, error)
	GetRepository(context.Context, uuid.UUID) (repo.Repository, error)
	GetRepositoryObservationState(context.Context, uuid.UUID) (repo.RepositoryObservationState, error)
	GetRepositoryChangeCursor(context.Context, repo.GetRepositoryChangeCursorParams) (repo.RepositoryChangeCursor, error)
	GetRepositoryNode(context.Context, repo.GetRepositoryNodeParams) (repo.RepositoryNode, error)
}

// Controller owns only query-only planning, external observation, and typed
// commit submission. Foreground write commands live in Commands.
type Controller struct {
	reader      Reader
	commits     committer
	directories directorySource
	cfg         Config
	logger      *zap.Logger
	now         func() time.Time
	feed        changefeed.Feed
}

func New(reader Reader, commits committer, files *storage.RepositoryFSFactory, cfg Config, logger *zap.Logger) *Controller {
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
		reader: reader, commits: commits, directories: directories, cfg: cfg.normalized(),
		logger: logger.With(zap.String("component", "repository_observation_controller")),
		now:    func() time.Time { return time.Now().UTC() },
		feed:   feed,
	}
}

// directorySource is the filesystem-free seam used by deterministic scale
// profiles. Production always uses repositoryDirectorySource.
type directorySource interface {
	ReadDirectory(context.Context, repo.Repository, storage.DirectoryReadOptions) (storage.DirectoryReadBatch, error)
}

type repositoryDirectorySource struct {
	files *storage.RepositoryFSFactory
}

func (source repositoryDirectorySource) ReadDirectory(ctx context.Context, repository repo.Repository, options storage.DirectoryReadOptions) (storage.DirectoryReadBatch, error) {
	repositoryFS, err := source.files.OpenContext(ctx, repository)
	if err != nil {
		return storage.DirectoryReadBatch{}, err
	}
	batch, readErr := repositoryFS.ReadUserMediaDirectory(ctx, options)
	closeErr := repositoryFS.Close()
	return batch, errors.Join(readErr, closeErr)
}

func (controller *Controller) Notifications() <-chan uuid.UUID {
	if controller == nil {
		return nil
	}
	if source, ok := controller.feed.(changefeed.NotificationSource); ok {
		return source.Notifications()
	}
	return nil
}

func (controller *Controller) Close() error {
	if controller == nil {
		return nil
	}
	if closer, ok := controller.feed.(changefeed.Closer); ok {
		return closer.Close()
	}
	return nil
}

// RunTurn advances at most one bounded unit. Every durable mutation is applied
// by the coordinator handler and returned through a post-commit acknowledgement.
func (controller *Controller) RunTurn(ctx context.Context, repositoryID, operationID uuid.UUID) (TurnResult, error) {
	result := TurnResult{OperationID: operationID, HasMore: true}
	if controller == nil || controller.reader == nil || controller.commits == nil || controller.directories == nil {
		return result, errors.New("repository observation controller unavailable")
	}
	nowTime := controller.now()
	lease := uuid.NewString()
	claim, _, err := controller.submit(ctx, observationCommit{
		Action: actionClaimController, RepositoryID: repositoryID, RunID: operationID,
		Now: dbtypes.NewTimestamp(nowTime), Lease: lease,
		LeaseExpiresAt: nowTime.Add(controller.cfg.ControllerLease).UnixMicro(),
	})
	if err != nil {
		return result, fmt.Errorf("claim repository controller: %w", err)
	}
	if !claim.Claimed {
		return result, nil
	}
	defer controller.releaseController(repositoryID, operationID, lease)

	run, err := controller.reader.GetRepositoryScanRun(ctx, repo.GetRepositoryScanRunParams{RepositoryID: repositoryID, RunID: operationID})
	if err != nil {
		return result, fmt.Errorf("load repository observation run: %w", err)
	}
	result.Status = run.Status
	if isTerminal(run.Status) {
		result.HasMore = false
		return result, nil
	}
	if run.CancellationRequested != 0 {
		return controller.cancelRun(ctx, run)
	}

	switch run.Status {
	case StatusQueued:
		return controller.startRun(ctx, run)
	case StatusCrawling:
		return controller.crawlTurn(ctx, run, lease)
	case StatusCatchingUp:
		cursor := run.CursorEnd
		if len(cursor) == 0 {
			cursor = run.CursorStart
		}
		if len(run.CursorTarget) == 0 || !bytes.Equal(cursor, run.CursorTarget) {
			return controller.catchUpTurn(ctx, run)
		}
		open, err := controller.reader.CountOpenRepositoryScanFrontier(ctx, run.RunID)
		if err != nil {
			return result, fmt.Errorf("count repository verification frontier: %w", err)
		}
		if open > 0 {
			return controller.crawlTurn(ctx, run, lease)
		}
		return controller.catchUpTurn(ctx, run)
	case StatusFinalizing:
		return controller.finalizeTurn(ctx, run, lease)
	default:
		return result, fmt.Errorf("unsupported repository observation status %q", run.Status)
	}
}

func (controller *Controller) startRun(ctx context.Context, run repo.RepositoryScanRun) (TurnResult, error) {
	result := TurnResult{OperationID: run.RunID, Status: run.Status, HasMore: true}
	repository, err := controller.reader.GetRepository(ctx, run.RepositoryID)
	if err != nil {
		return result, fmt.Errorf("load repository before change capture: %w", err)
	}
	state, err := controller.reader.GetRepositoryObservationState(ctx, run.RepositoryID)
	if err != nil {
		return result, fmt.Errorf("load repository observation state: %w", err)
	}
	checkpoint, captureErr := controller.feed.Snapshot(ctx, repository)
	if captureErr != nil {
		controller.logger.Warn("repository change capture unavailable", zap.String("repository_id", run.RepositoryID.String()), zap.Error(captureErr))
		checkpoint = changefeed.Checkpoint{AdapterKind: "periodic", Health: changefeed.HealthUnavailable}
	}
	checkpoint = normalizedCheckpoint(checkpoint)
	fullVerification := run.ForceFullVerification != 0 || state.FullVerificationRequired != 0 || !checkpoint.Valid()
	start := checkpoint
	if checkpoint.Valid() {
		persisted, cursorErr := controller.reader.GetRepositoryChangeCursor(ctx, repo.GetRepositoryChangeCursorParams{RepositoryID: run.RepositoryID, AdapterKind: checkpoint.AdapterKind})
		switch {
		case cursorErr == nil:
			prior := checkpointFromCatalog(persisted, checkpoint.VolumeKind)
			if prior.Valid() && prior.VolumeIdentity != checkpoint.VolumeIdentity {
				checkpoint.Health = changefeed.HealthGap
				fullVerification = true
			} else if !prior.Valid() || !prior.SameIdentity(checkpoint) {
				fullVerification = true
			} else if !fullVerification {
				start = prior
			}
		case !errors.Is(cursorErr, sql.ErrNoRows):
			return result, fmt.Errorf("load repository change cursor: %w", cursorErr)
		default:
			fullVerification = true
		}
	}
	initialStatus := StatusCatchingUp
	target := []byte{}
	if fullVerification {
		initialStatus = StatusCrawling
		start = checkpoint
	} else {
		target = append([]byte(nil), checkpoint.Cursor...)
	}
	acknowledgement, commitResult, err := controller.submit(ctx, observationCommit{
		Action: actionStartRun, RepositoryID: run.RepositoryID, RunID: run.RunID,
		RequestedEpoch: uint64(run.RequestedEpoch), Now: dbtypes.NewTimestamp(controller.now()),
		Run: run, Checkpoint: checkpoint, StartCheckpoint: start, TargetCursor: target,
		InitialStatus: initialStatus, FullVerification: fullVerification, RootNodeID: uuid.New(),
	})
	if err != nil {
		return result, err
	}
	return controller.turnFromAcknowledgement(acknowledgement, commitResult), nil
}

func (controller *Controller) crawlTurn(ctx context.Context, run repo.RepositoryScanRun, controllerLease string) (TurnResult, error) {
	result := TurnResult{OperationID: run.RunID, Status: StatusCrawling, HasMore: true}
	outboxDepth, err := controller.reader.CountPendingRepositoryMaterialization(ctx, run.RepositoryID)
	if err != nil {
		return result, fmt.Errorf("count repository materialization candidates: %w", err)
	}
	if outboxDepth >= controller.cfg.OutboxHighWaterMark {
		result.Backpressure = true
		return result, nil
	}
	repository, err := controller.reader.GetRepository(ctx, run.RepositoryID)
	if err != nil {
		return result, fmt.Errorf("load repository for crawl: %w", err)
	}
	nowTime := controller.now()
	frontierLease := controllerLease + ":frontier"
	claim, _, err := controller.submit(ctx, observationCommit{
		Action: actionClaimFrontier, RepositoryID: run.RepositoryID, RunID: run.RunID,
		RequestedEpoch: uint64(run.RequestedEpoch), Now: dbtypes.NewTimestamp(nowTime),
		Lease: frontierLease, LeaseExpiresAt: nowTime.Add(controller.cfg.ControllerLease).UnixMicro(), Run: run,
	})
	if err != nil {
		return result, fmt.Errorf("claim repository scan frontier: %w", err)
	}
	if !claim.Claimed {
		if run.Status == StatusCrawling {
			return controller.transitionRun(ctx, run, StatusCrawling, StatusCatchingUp, run.PartialCoverage, "")
		}
		return result, nil
	}
	frontier := claim.Frontier
	directoryNodeID, err := uuid.Parse(frontier.DirectoryNodeID)
	if err != nil {
		return result, fmt.Errorf("invalid frontier node ID: %w", err)
	}
	directoryPath, err := nodegraph.ProjectPath(ctx, controller.reader, run.RepositoryID, directoryNodeID)
	if err != nil {
		return result, controller.failFrontier(ctx, run, frontier, frontierLease, "node_path_unavailable", err)
	}
	if watcher, ok := controller.feed.(changefeed.DirectoryWatcher); ok {
		if err := watcher.WatchDirectory(ctx, repository, directoryPath); err != nil {
			return result, controller.failFrontier(ctx, run, frontier, frontierLease, "watch_directory_failed", err)
		}
	}
	batch, err := controller.directories.ReadDirectory(ctx, repository, storage.DirectoryReadOptions{
		Directory: directoryPath, Offset: frontier.ContinuationOffset,
		Limit: controller.cfg.BatchSize, ScanID: run.RunID, Settle: controller.cfg.Settle, Now: nowTime,
	})
	if err != nil {
		return result, controller.failFrontier(ctx, run, frontier, frontierLease, "directory_enumeration_failed", err)
	}
	acknowledgement, commitResult, err := controller.submit(ctx, observationCommit{
		Action: actionApplyDirectory, RepositoryID: run.RepositoryID, RunID: run.RunID,
		RequestedEpoch: uint64(run.RequestedEpoch), Now: dbtypes.NewTimestamp(controller.now()),
		Lease: frontierLease, Run: run, Frontier: frontier, ParentNodeID: directoryNodeID, DirectoryBatch: batch,
	})
	if err != nil {
		return result, err
	}
	result = controller.turnFromAcknowledgement(acknowledgement, commitResult)
	controller.observeTurnBudget("repository observation", run, result)
	return result, nil
}

func (controller *Controller) catchUpTurn(ctx context.Context, run repo.RepositoryScanRun) (TurnResult, error) {
	result := TurnResult{OperationID: run.RunID, Status: StatusCatchingUp, HasMore: true}
	repository, err := controller.reader.GetRepository(ctx, run.RepositoryID)
	if err != nil {
		return result, fmt.Errorf("load repository for change catch-up: %w", err)
	}
	state, err := controller.reader.GetRepositoryObservationState(ctx, run.RepositoryID)
	if err != nil {
		return result, fmt.Errorf("load repository adapter state: %w", err)
	}
	current, snapshotErr := controller.feed.Snapshot(ctx, repository)
	if snapshotErr != nil {
		return controller.finishUnsafeCatchup(ctx, run, state, changefeed.HealthUnavailable, "change_cursor_unavailable", snapshotErr)
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
		health := cursorFailureHealth(from, current)
		return controller.finishUnsafeCatchup(ctx, run, state, health, "change_cursor_"+string(health), changefeed.ErrCursorInvalid)
	}
	if len(run.CursorTarget) == 0 {
		captured, _, err := controller.submit(ctx, observationCommit{
			Action: actionCaptureTarget, RepositoryID: run.RepositoryID, RunID: run.RunID,
			RequestedEpoch: uint64(run.RequestedEpoch), Now: dbtypes.NewTimestamp(controller.now()), Run: run, Checkpoint: current,
		})
		if err != nil {
			return result, fmt.Errorf("capture repository catch-up boundary: %w", err)
		}
		if len(captured.CursorTarget) == 0 {
			return result, nil
		}
		run.CursorTarget = append([]byte(nil), captured.CursorTarget...)
	}
	through := current
	through.Cursor = append([]byte(nil), run.CursorTarget...)
	batch, err := controller.feed.Read(ctx, repository, from, through, min(controller.cfg.BatchSize, maximumChangeBatchSize))
	if err != nil {
		health := changefeed.HealthGap
		switch batch.Next.Health {
		case changefeed.HealthOverflow:
			health = changefeed.HealthOverflow
		case changefeed.HealthUnavailable:
			health = changefeed.HealthUnavailable
		}
		return controller.finishUnsafeCatchup(ctx, run, state, health, "change_cursor_"+string(health), err)
	}
	batch.Next = normalizedCheckpoint(batch.Next)
	if !batch.Next.Valid() || !batch.Next.SameIdentity(through) {
		return controller.finishUnsafeCatchup(ctx, run, state, changefeed.HealthGap, "change_cursor_gap", changefeed.ErrCursorInvalid)
	}
	if len(batch.Events) == 0 && !batch.Done && batch.Next.SamePosition(from) {
		return result, errors.New("repository change adapter made no cursor progress")
	}
	if err := validateChangeBatch(batch); err != nil {
		return controller.finishUnsafeCatchup(ctx, run, state, changefeed.HealthGap, "change_event_invalid", err)
	}
	acknowledgement, commitResult, err := controller.submit(ctx, observationCommit{
		Action: actionApplyChanges, RepositoryID: run.RepositoryID, RunID: run.RunID,
		RequestedEpoch: uint64(run.RequestedEpoch), Now: dbtypes.NewTimestamp(controller.now()), Run: run, State: state, ChangeBatch: batch,
	})
	if err != nil {
		return result, err
	}
	result = controller.turnFromAcknowledgement(acknowledgement, commitResult)
	if len(batch.Events) > 0 || !batch.Done {
		return result, nil
	}
	return controller.transitionRun(ctx, run, StatusCatchingUp, StatusFinalizing, run.PartialCoverage, "")
}

func (controller *Controller) finishUnsafeCatchup(ctx context.Context, run repo.RepositoryScanRun, state repo.RepositoryObservationState, health changefeed.Health, failureCode string, cause error) (TurnResult, error) {
	acknowledgement, commitResult, err := controller.submit(ctx, observationCommit{
		Action: actionUnsafeCatchup, RepositoryID: run.RepositoryID, RunID: run.RunID,
		RequestedEpoch: uint64(run.RequestedEpoch), Now: dbtypes.NewTimestamp(controller.now()),
		Run: run, State: state, CursorHealth: health, FailureCode: failureCode,
	})
	if err != nil {
		return TurnResult{OperationID: run.RunID, Status: StatusFinalizing, HasMore: true}, err
	}
	controller.logger.Warn("repository change catch-up is not authoritative",
		zap.String("repository_id", run.RepositoryID.String()), zap.String("operation_id", run.RunID.String()),
		zap.String("cursor_health", string(health)), zap.String("failure_code", failureCode), zap.Error(cause))
	return controller.turnFromAcknowledgement(acknowledgement, commitResult), nil
}

func (controller *Controller) finalizeTurn(ctx context.Context, run repo.RepositoryScanRun, controllerLease string) (TurnResult, error) {
	result := TurnResult{OperationID: run.RunID, Status: StatusFinalizing, HasMore: true}
	state, err := controller.reader.GetRepositoryObservationState(ctx, run.RepositoryID)
	if err != nil {
		return result, err
	}
	authoritative := state.CursorHealth == string(changefeed.HealthHealthy) && run.PartialCoverage == 0 && run.ErrorDirectories == 0 && len(run.CursorEnd) > 0
	if authoritative {
		open, err := controller.reader.CountOpenRepositoryAbsenceFrontier(ctx, run.RunID)
		if err != nil {
			return result, err
		}
		if open > 0 {
			return controller.finalizeAbsenceTurn(ctx, run, controllerLease, state.VolumeKind)
		}
	}
	return controller.finishRun(ctx, run, state, authoritative)
}

func (controller *Controller) finalizeAbsenceTurn(ctx context.Context, run repo.RepositoryScanRun, controllerLease, volumeKind string) (TurnResult, error) {
	result := TurnResult{OperationID: run.RunID, Status: StatusFinalizing, HasMore: true}
	nowTime := controller.now()
	lease := controllerLease + ":absence"
	claim, _, err := controller.submit(ctx, observationCommit{
		Action: actionClaimAbsence, RepositoryID: run.RepositoryID, RunID: run.RunID,
		RequestedEpoch: uint64(run.RequestedEpoch), Now: dbtypes.NewTimestamp(nowTime),
		Lease: lease, LeaseExpiresAt: nowTime.Add(controller.cfg.ControllerLease).UnixMicro(), Run: run,
	})
	if err != nil {
		return result, fmt.Errorf("claim repository absence frontier: %w", err)
	}
	if !claim.Claimed {
		return result, nil
	}
	acknowledgement, commitResult, err := controller.submit(ctx, observationCommit{
		Action: actionApplyAbsence, RepositoryID: run.RepositoryID, RunID: run.RunID,
		RequestedEpoch: uint64(run.RequestedEpoch), Now: dbtypes.NewTimestamp(controller.now()),
		Lease: lease, Run: run, Frontier: claim.Frontier, VolumeKind: volumeKind,
		BatchSize: controller.cfg.BatchSize, TransactionBudget: controller.cfg.TransactionBudget, Settle: controller.cfg.Settle,
	})
	if err != nil {
		return result, fmt.Errorf("finalize authoritative repository absences: %w", err)
	}
	result = controller.turnFromAcknowledgement(acknowledgement, commitResult)
	controller.observeTurnBudget("repository absence", run, result)
	return result, nil
}

func (controller *Controller) finishRun(ctx context.Context, run repo.RepositoryScanRun, state repo.RepositoryObservationState, authoritative bool) (TurnResult, error) {
	acknowledgement, commitResult, err := controller.submit(ctx, observationCommit{
		Action: actionFinishRun, RepositoryID: run.RepositoryID, RunID: run.RunID,
		RequestedEpoch: uint64(run.RequestedEpoch), Now: dbtypes.NewTimestamp(controller.now()),
		Run: run, State: state, Authoritative: authoritative, FollowUpRunID: uuid.New(),
	})
	if err != nil {
		return TurnResult{OperationID: run.RunID, Status: StatusFinalizing, HasMore: true}, err
	}
	return controller.turnFromAcknowledgement(acknowledgement, commitResult), nil
}

func (controller *Controller) failFrontier(ctx context.Context, run repo.RepositoryScanRun, frontier repo.RepositoryScanFrontier, lease, failureCode string, cause error) error {
	_, _, err := controller.submit(ctx, observationCommit{
		Action: actionFailFrontier, RepositoryID: run.RepositoryID, RunID: run.RunID,
		RequestedEpoch: uint64(run.RequestedEpoch), Now: dbtypes.NewTimestamp(controller.now()),
		Lease: lease, Run: run, Frontier: frontier, FailureCode: failureCode,
	})
	if err == nil {
		controller.logger.Warn("repository directory observation failed",
			zap.String("repository_id", run.RepositoryID.String()), zap.String("operation_id", run.RunID.String()),
			zap.String("failure_code", failureCode), zap.Error(cause))
	}
	return err
}

func (controller *Controller) cancelRun(ctx context.Context, run repo.RepositoryScanRun) (TurnResult, error) {
	acknowledgement, commitResult, err := controller.submit(ctx, observationCommit{
		Action: actionCancelRun, RepositoryID: run.RepositoryID, RunID: run.RunID,
		RequestedEpoch: uint64(run.RequestedEpoch), Now: dbtypes.NewTimestamp(controller.now()), Run: run, FollowUpRunID: uuid.New(),
	})
	if err != nil {
		return TurnResult{OperationID: run.RunID, Status: run.Status, HasMore: true}, err
	}
	return controller.turnFromAcknowledgement(acknowledgement, commitResult), nil
}

func (controller *Controller) transitionRun(ctx context.Context, run repo.RepositoryScanRun, from, to string, partial int64, failureCode string) (TurnResult, error) {
	acknowledgement, commitResult, err := controller.submit(ctx, observationCommit{
		Action: actionTransitionRun, RepositoryID: run.RepositoryID, RunID: run.RunID,
		RequestedEpoch: uint64(run.RequestedEpoch), Now: dbtypes.NewTimestamp(controller.now()), Run: run,
		FromStatus: from, ToStatus: to, PartialCoverage: partial, FailureCode: failureCode,
	})
	if err != nil {
		return TurnResult{OperationID: run.RunID, Status: from, HasMore: true}, err
	}
	return controller.turnFromAcknowledgement(acknowledgement, commitResult), nil
}

func (controller *Controller) releaseController(repositoryID, operationID uuid.UUID, lease string) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, _, err := controller.submit(ctx, observationCommit{
		Action: actionReleaseController, RepositoryID: repositoryID, RunID: operationID,
		Now: dbtypes.NewTimestamp(controller.now()), Lease: lease,
	})
	if err != nil {
		controller.logger.Warn("release repository observation controller lease", zap.Error(err))
	}
}

func (controller *Controller) submit(ctx context.Context, payload observationCommit) (TurnAcknowledgement, commit.Result, error) {
	result, err := controller.commits.Submit(ctx, commit.Intent{
		Key: commit.Key{
			Family: FamilyRepositoryObservation, Subject: payload.RepositoryID.String(), Fence: payload.RunID.String(),
			Stage: string(payload.Action), DesiredVersion: max(uint64(1), payload.RequestedEpoch),
		},
		Payload: payload,
	})
	if err != nil {
		return TurnAcknowledgement{}, commit.Result{}, err
	}
	acknowledgement, ok := result.Acknowledgement.(TurnAcknowledgement)
	if !ok {
		return TurnAcknowledgement{}, result, errors.New("repository observation commit returned wrong acknowledgement type")
	}
	return acknowledgement, result, nil
}

func (controller *Controller) turnFromAcknowledgement(acknowledgement TurnAcknowledgement, result commit.Result) TurnResult {
	turn := acknowledgement.Turn
	turn.TransactionDuration = result.TransactionDuration
	turn.CommitDuration = result.CommitDuration
	return turn
}

func (controller *Controller) observeTurnBudget(operation string, run repo.RepositoryScanRun, result TurnResult) {
	if result.TransactionDuration > controller.cfg.TransactionBudget {
		controller.logger.Warn(operation+" statements exceeded budget",
			zap.String("repository_id", run.RepositoryID.String()), zap.String("operation_id", run.RunID.String()),
			zap.Int("rows", result.RowsApplied), zap.Duration("duration", result.TransactionDuration))
	}
	if result.CommitDuration > maximumTransactionBudget {
		controller.logger.Warn(operation+" begin/commit overhead exceeded budget",
			zap.String("repository_id", run.RepositoryID.String()), zap.String("operation_id", run.RunID.String()),
			zap.Int("rows", result.RowsApplied), zap.Duration("duration", result.CommitDuration))
	}
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
