package controller

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"server/internal/commit"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/storage"
	"server/internal/storage/roe/changefeed"
	"server/internal/storage/roe/pathsemantics"
)

type commitAction string

const (
	actionClaimController   commitAction = "claim_controller"
	actionReleaseController commitAction = "release_controller"
	actionStartRun          commitAction = "start_run"
	actionClaimFrontier     commitAction = "claim_frontier"
	actionApplyDirectory    commitAction = "apply_directory"
	actionCaptureTarget     commitAction = "capture_target"
	actionApplyChanges      commitAction = "apply_changes"
	actionUnsafeCatchup     commitAction = "unsafe_catchup"
	actionTransitionRun     commitAction = "transition_run"
	actionClaimAbsence      commitAction = "claim_absence"
	actionApplyAbsence      commitAction = "apply_absence"
	actionFinishRun         commitAction = "finish_run"
	actionFailFrontier      commitAction = "fail_frontier"
	actionCancelRun         commitAction = "cancel_run"
)

// observationCommit is the closed immutable input union for asynchronous ROE
// state changes. It deliberately contains no transaction, query object,
// catalog service, or callback.
type observationCommit struct {
	Action            commitAction
	RepositoryID      uuid.UUID
	RunID             uuid.UUID
	RequestedEpoch    uint64
	Now               dbtypes.Timestamp
	Lease             string
	LeaseExpiresAt    int64
	Run               repo.RepositoryScanRun
	State             repo.RepositoryObservationState
	Frontier          repo.RepositoryScanFrontier
	ParentNodeID      uuid.UUID
	DirectoryBatch    storage.DirectoryReadBatch
	ChangeBatch       changefeed.Batch
	Checkpoint        changefeed.Checkpoint
	StartCheckpoint   changefeed.Checkpoint
	TargetCursor      []byte
	InitialStatus     string
	FullVerification  bool
	RootNodeID        uuid.UUID
	FromStatus        string
	ToStatus          string
	PartialCoverage   int64
	FailureCode       string
	CursorHealth      changefeed.Health
	VolumeKind        string
	Authoritative     bool
	FollowUpRunID     uuid.UUID
	BatchSize         int
	TransactionBudget time.Duration
	Settle            time.Duration
}

// TurnAcknowledgement is returned only after the coordinator transaction has
// committed. Controller merges the generic coordinator timings into Turn.
type TurnAcknowledgement struct {
	Turn         TurnResult
	Claimed      bool
	Frontier     repo.RepositoryScanFrontier
	CursorTarget []byte
}

func (TurnAcknowledgement) CommitAcknowledgement() {}

type commitApplier struct{}

func (applier *commitApplier) apply(ctx context.Context, tx *sql.Tx, payload observationCommit) (commit.Result, error) {
	if payload.Action == "" || payload.RepositoryID == uuid.Nil || payload.RunID == uuid.Nil {
		return commit.Result{}, errors.New("invalid repository observation result")
	}
	started := time.Now()
	acknowledgement, outcome, err := applier.applyOne(ctx, tx, payload)
	if err != nil {
		return commit.Result{}, err
	}
	return commit.Result{
		Outcome:             outcome,
		Acknowledgement:     acknowledgement,
		TransactionDuration: time.Since(started),
	}, nil
}

func (applier *commitApplier) applyOne(
	ctx context.Context,
	tx *sql.Tx,
	payload observationCommit,
) (TurnAcknowledgement, commit.Outcome, error) {
	queries := repo.New(tx)
	acknowledgement := TurnAcknowledgement{Turn: TurnResult{
		OperationID: payload.RunID,
		Status:      payload.Run.Status,
		HasMore:     true,
	}}
	switch payload.Action {
	case actionClaimController:
		_, err := queries.ClaimRepositoryObservationController(ctx, repo.ClaimRepositoryObservationControllerParams{
			RepositoryID: payload.RepositoryID, ControllerLeaseID: &payload.Lease,
			ControllerLeaseExpiresAt: &payload.LeaseExpiresAt, UpdatedAt: payload.Now,
		})
		if errors.Is(err, sql.ErrNoRows) {
			return acknowledgement, commit.OutcomeStale, nil
		}
		acknowledgement.Claimed = err == nil
		return acknowledgement, commit.OutcomeApplied, err

	case actionReleaseController:
		_, err := queries.ReleaseRepositoryObservationController(ctx, repo.ReleaseRepositoryObservationControllerParams{
			RepositoryID: payload.RepositoryID, ControllerLeaseID: &payload.Lease, UpdatedAt: payload.Now,
		})
		if errors.Is(err, sql.ErrNoRows) {
			return acknowledgement, commit.OutcomeStale, nil
		}
		return acknowledgement, commit.OutcomeApplied, err

	case actionStartRun:
		if err := applier.startRun(ctx, queries, payload); err != nil {
			return acknowledgement, 0, err
		}
		acknowledgement.Turn.Status = payload.InitialStatus
		return acknowledgement, commit.OutcomeApplied, nil

	case actionClaimFrontier:
		frontier, err := queries.ClaimRepositoryScanFrontier(ctx, repo.ClaimRepositoryScanFrontierParams{
			RunID: payload.RunID, LeaseID: &payload.Lease,
			LeaseExpiresAt: &payload.LeaseExpiresAt, UpdatedAt: payload.Now,
		})
		if errors.Is(err, sql.ErrNoRows) {
			return acknowledgement, commit.OutcomeStale, nil
		}
		acknowledgement.Claimed = err == nil
		acknowledgement.Frontier = frontier
		return acknowledgement, commit.OutcomeApplied, err

	case actionApplyDirectory:
		turn, err := applier.applyDirectoryBatch(ctx, tx, queries, payload)
		acknowledgement.Turn = turn
		return acknowledgement, commit.OutcomeApplied, err

	case actionCaptureTarget:
		captured, err := queries.CaptureRepositoryScanRunCursorTarget(ctx, repo.CaptureRepositoryScanRunCursorTargetParams{
			RunID: payload.RunID, CursorTarget: payload.Checkpoint.Cursor, UpdatedAt: payload.Now,
		})
		if errors.Is(err, sql.ErrNoRows) {
			return acknowledgement, commit.OutcomeStale, nil
		}
		acknowledgement.CursorTarget = append([]byte(nil), captured.CursorTarget...)
		return acknowledgement, commit.OutcomeApplied, err

	case actionApplyChanges:
		if err := applier.applyChangeBatch(ctx, queries, payload); err != nil {
			return acknowledgement, 0, err
		}
		return acknowledgement, commit.OutcomeApplied, nil

	case actionUnsafeCatchup:
		if err := applier.finishUnsafeCatchup(ctx, queries, payload); err != nil {
			return acknowledgement, 0, err
		}
		acknowledgement.Turn.Status = StatusFinalizing
		return acknowledgement, commit.OutcomeApplied, nil

	case actionTransitionRun:
		_, err := queries.TransitionRepositoryScanRun(ctx, repo.TransitionRepositoryScanRunParams{
			RunID: payload.RunID, Status: payload.ToStatus, UpdatedAt: payload.Now,
			PartialCoverage: payload.PartialCoverage, FailureCode: optionalString(payload.FailureCode),
			Status_2: payload.FromStatus,
		})
		if errors.Is(err, sql.ErrNoRows) {
			return acknowledgement, commit.OutcomeStale, nil
		}
		acknowledgement.Turn.Status = payload.ToStatus
		return acknowledgement, commit.OutcomeApplied, err

	case actionClaimAbsence:
		frontier, err := queries.ClaimRepositoryAbsenceFrontier(ctx, repo.ClaimRepositoryAbsenceFrontierParams{
			RunID: payload.RunID, LeaseID: &payload.Lease,
			LeaseExpiresAt: &payload.LeaseExpiresAt, UpdatedAt: payload.Now,
		})
		if errors.Is(err, sql.ErrNoRows) {
			return acknowledgement, commit.OutcomeStale, nil
		}
		acknowledgement.Claimed = err == nil
		acknowledgement.Frontier = frontier
		return acknowledgement, commit.OutcomeApplied, err

	case actionApplyAbsence:
		turn, err := applier.applyAbsenceBatch(ctx, queries, payload)
		acknowledgement.Turn = turn
		return acknowledgement, commit.OutcomeApplied, err

	case actionFinishRun:
		turn, err := applier.finishRun(ctx, tx, queries, payload)
		acknowledgement.Turn = turn
		return acknowledgement, commit.OutcomeApplied, err

	case actionFailFrontier:
		if err := applier.failFrontier(ctx, queries, payload); err != nil {
			return acknowledgement, 0, err
		}
		return acknowledgement, commit.OutcomeApplied, nil

	case actionCancelRun:
		if err := applier.cancelRun(ctx, tx, queries, payload); err != nil {
			return acknowledgement, 0, err
		}
		acknowledgement.Turn.Status = StatusCancelled
		acknowledgement.Turn.HasMore = false
		return acknowledgement, commit.OutcomeApplied, nil
	default:
		return acknowledgement, 0, fmt.Errorf("unsupported repository observation commit action %q", payload.Action)
	}
}

func (applier *commitApplier) startRun(ctx context.Context, queries *repo.Queries, payload observationCommit) error {
	checkpoint := payload.Checkpoint
	if _, err := queries.UpdateRepositoryObservationAdapter(ctx, repo.UpdateRepositoryObservationAdapterParams{
		RepositoryID: payload.RepositoryID, AdapterKind: checkpoint.AdapterKind,
		AdapterIdentity: optionalString(checkpoint.JournalIdentity),
		VolumeIdentity:  optionalString(checkpoint.VolumeIdentity), VolumeKind: normalizeVolumeKind(checkpoint.VolumeKind),
		CursorHealth: string(checkpoint.Health), FullVerificationRequired: boolInt(payload.FullVerification),
		UpdatedAt: payload.Now,
	}); err != nil {
		return fmt.Errorf("persist repository change adapter: %w", err)
	}
	if payload.FullVerification {
		rootNode, rootErr := queries.GetRepositoryRootNode(ctx, payload.RepositoryID)
		if errors.Is(rootErr, sql.ErrNoRows) {
			revision, revisionErr := queries.AllocateRepositoryObservationRevision(ctx, repo.AllocateRepositoryObservationRevisionParams{
				RepositoryID: payload.RepositoryID, UpdatedAt: payload.Now,
			})
			if revisionErr != nil {
				return revisionErr
			}
			rootNode, rootErr = queries.InsertRepositoryRootNode(ctx, repo.InsertRepositoryRootNodeParams{
				NodeID: payload.RootNodeID, RepositoryID: payload.RepositoryID,
				ObservationRevision: revision, CreatedAt: payload.Now,
			})
		}
		if rootErr != nil {
			return fmt.Errorf("ensure repository root node: %w", rootErr)
		}
		if _, err := queries.EnqueueRepositoryScanFrontier(ctx, repo.EnqueueRepositoryScanFrontierParams{
			RunID: payload.RunID, DirectoryNodeID: rootNode.NodeID.String(), Purpose: "crawl", CreatedAt: payload.Now,
		}); err != nil {
			return fmt.Errorf("enqueue repository root frontier: %w", err)
		}
	}
	if _, err := queries.StartRepositoryScanRun(ctx, repo.StartRepositoryScanRunParams{
		RunID: payload.RunID, Status: payload.InitialStatus, UpdatedAt: payload.Now,
		CursorStart: payload.StartCheckpoint.Cursor, CursorTarget: payload.TargetCursor,
		VolumeIdentity: optionalString(checkpoint.VolumeIdentity),
	}); err != nil {
		return fmt.Errorf("start repository observation run: %w", err)
	}
	return nil
}

func (applier *commitApplier) applyDirectoryBatch(
	ctx context.Context,
	tx *sql.Tx,
	queries *repo.Queries,
	payload observationCommit,
) (TurnResult, error) {
	run := payload.Run
	frontier := payload.Frontier
	batch := payload.DirectoryBatch
	result := TurnResult{OperationID: run.RunID, Status: StatusCrawling, HasMore: true}
	state, err := queries.GetRepositoryObservationState(ctx, run.RepositoryID)
	if err != nil {
		return result, err
	}
	repository, err := queries.GetRepository(ctx, run.RepositoryID)
	if err != nil {
		return result, err
	}
	semantics := pathsemantics.Semantics{
		Case: pathsemantics.CaseMode(state.PathCaseMode), Normalization: pathsemantics.Normalization(state.PathNormalization),
	}
	publication, err := applier.publishDirectoryBatchSetBased(
		ctx, tx, queries, run, payload.ParentNodeID, frontier, batch, semantics,
		nullableOwner(repository.DefaultOwnerID), payload.Now,
	)
	if err != nil {
		return result, err
	}
	result.RowsApplied = publication.rowsApplied
	result.BytesQueued = publication.bytesQueued
	finished := batch.Done
	coverageSafe := batch.Authoritative && frontier.CoverageSafe != 0
	if finished {
		if coverageSafe {
			if err := applier.recordAuthoritativeCoverage(ctx, queries, run, payload.ParentNodeID, payload.Now); err != nil {
				return result, err
			}
			result.RowsApplied++
		}
		if _, err := queries.CompleteRepositoryScanFrontier(ctx, repo.CompleteRepositoryScanFrontierParams{
			RunID: run.RunID, DirectoryNodeID: frontier.DirectoryNodeID,
			AuthoritativeChildSet: boolInt(coverageSafe), UpdatedAt: payload.Now, LeaseID: &payload.Lease,
		}); err != nil {
			return result, fmt.Errorf("complete repository scan frontier: %w", err)
		}
	} else {
		if batch.NextOffset <= frontier.ContinuationOffset {
			return result, errors.New("repository directory page made no durable progress")
		}
		if _, err := queries.ContinueRepositoryScanFrontier(ctx, repo.ContinueRepositoryScanFrontierParams{
			RunID: run.RunID, DirectoryNodeID: frontier.DirectoryNodeID,
			ContinuationOffset: batch.NextOffset, CoverageSafe: boolInt(batch.Authoritative),
			UpdatedAt: payload.Now, LeaseID: &payload.Lease,
		}); err != nil {
			return result, fmt.Errorf("checkpoint repository scan frontier: %w", err)
		}
	}
	outboxDepth, err := queries.CountPendingRepositoryMaterialization(ctx, run.RepositoryID)
	if err != nil {
		return result, err
	}
	if _, err := queries.UpdateRepositoryScanRunProgress(ctx, repo.UpdateRepositoryScanRunProgressParams{
		RunID: run.RunID, DirectoriesObserved: publication.directoryCount, FilesObserved: publication.fileCount,
		BytesQueued: publication.bytesQueued, AuthoritativeDirectories: boolInt(finished && coverageSafe),
		ErrorDirectories: boolInt(finished && !coverageSafe), OutboxDepth: outboxDepth, UpdatedAt: payload.Now,
	}); err != nil {
		return result, fmt.Errorf("update repository observation progress: %w", err)
	}
	return result, nil
}

func (applier *commitApplier) recordAuthoritativeCoverage(
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

func (applier *commitApplier) applyChangeBatch(ctx context.Context, queries *repo.Queries, payload observationCommit) error {
	run := payload.Run
	state := payload.State
	batch := payload.ChangeBatch
	firstRevision := int64(0)
	if len(batch.Events) > 0 {
		var err error
		firstRevision, err = queries.AllocateRepositoryObservationRevisionRange(ctx, repo.AllocateRepositoryObservationRevisionRangeParams{
			RepositoryID: run.RepositoryID, NextRevision: int64(len(batch.Events)), UpdatedAt: payload.Now,
		})
		if err != nil {
			return err
		}
	}
	semantics := pathsemantics.Semantics{
		Case: pathsemantics.CaseMode(state.PathCaseMode), Normalization: pathsemantics.Normalization(state.PathNormalization),
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
			PathHint: stringPtr(pathHint), CreatedAt: payload.Now,
		})
		if err != nil {
			return fmt.Errorf("persist native change observation: %w", err)
		}
		if persisted.ProcessingState != "applied" {
			if _, err := queries.CompleteRepositoryObservationCAS(ctx, repo.CompleteRepositoryObservationCASParams{
				RepositoryID: run.RepositoryID, ObservationID: persisted.ObservationID,
				ProcessingState: "applied", ProcessedAt: int64Ptr(payload.Now.Time.UnixMicro()),
			}); err != nil && !errors.Is(err, sql.ErrNoRows) {
				return err
			}
		}
		for _, dirty := range dirtyPrefixes(event) {
			directory, err := resolveClosestDirectory(ctx, queries, run.RepositoryID, dirty.path, semantics)
			if err != nil {
				return err
			}
			purpose := "verify"
			if dirty.recursive {
				purpose = "crawl"
			}
			if _, err := queries.RequeueRepositoryScanFrontierForVerification(ctx, repo.RequeueRepositoryScanFrontierForVerificationParams{
				RunID: run.RunID, DirectoryNodeID: directory.NodeID.String(), Purpose: purpose, CreatedAt: payload.Now,
			}); err != nil && !errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("queue dirty repository prefix: %w", err)
			}
		}
	}
	_, err := queries.UpdateRepositoryScanRunCursor(ctx, repo.UpdateRepositoryScanRunCursorParams{
		RunID: run.RunID, CursorEnd: batch.Next.Cursor,
		VolumeIdentity: optionalString(batch.Next.VolumeIdentity), UpdatedAt: payload.Now,
	})
	return err
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
		nameKey, err := semantics.NameKey(component)
		if err != nil {
			return repo.RepositoryNode{}, err
		}
		next, err := queries.GetActiveRepositoryChildByName(ctx, repo.GetActiveRepositoryChildByNameParams{
			RepositoryID: repositoryID,
			ParentNodeID: uuid.NullUUID{UUID: current.NodeID, Valid: true},
			NameKey:      nameKey,
		})
		if errors.Is(err, sql.ErrNoRows) {
			return current, nil
		}
		if err != nil {
			return repo.RepositoryNode{}, err
		}
		if next.Kind != "directory" {
			return current, nil
		}
		current = next
	}
	return current, nil
}

func (applier *commitApplier) finishUnsafeCatchup(ctx context.Context, queries *repo.Queries, payload observationCommit) error {
	state := payload.State
	if _, err := queries.UpdateRepositoryObservationAdapter(ctx, repo.UpdateRepositoryObservationAdapterParams{
		RepositoryID: payload.RepositoryID, AdapterKind: state.AdapterKind,
		AdapterIdentity: state.AdapterIdentity, VolumeIdentity: state.VolumeIdentity,
		VolumeKind: normalizeVolumeKind(state.VolumeKind), CursorHealth: string(payload.CursorHealth),
		FullVerificationRequired: 1, UpdatedAt: payload.Now,
	}); err != nil {
		return err
	}
	_, err := queries.TransitionRepositoryScanRun(ctx, repo.TransitionRepositoryScanRunParams{
		RunID: payload.RunID, Status: StatusFinalizing, UpdatedAt: payload.Now,
		PartialCoverage: 1, FailureCode: &payload.FailureCode, Status_2: StatusCatchingUp,
	})
	return err
}

func (applier *commitApplier) applyAbsenceBatch(ctx context.Context, queries *repo.Queries, payload observationCommit) (TurnResult, error) {
	run := payload.Run
	frontier := payload.Frontier
	result := TurnResult{OperationID: run.RunID, Status: StatusFinalizing, HasMore: true}
	started := time.Now()
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
	children, err := queries.ListUnseenRepositoryNodeChildrenPage(ctx, repo.ListUnseenRepositoryNodeChildrenPageParams{
		RepositoryID:  run.RepositoryID,
		ParentNodeID:  uuid.NullUUID{UUID: parentNodeID, Valid: true},
		LastSeenRunID: uuid.NullUUID{UUID: run.RunID, Valid: true},
		NodeID:        after, Limit: int64(min(payload.BatchSize, maximumAbsenceBatchSize)),
	})
	if err != nil {
		return result, err
	}
	if len(children) == 0 {
		_, err := queries.CompleteRepositoryAbsenceFrontier(ctx, repo.CompleteRepositoryAbsenceFrontierParams{
			RunID: run.RunID, DirectoryNodeID: frontier.DirectoryNodeID,
			UpdatedAt: payload.Now, LeaseID: &payload.Lease,
		})
		return result, err
	}
	eligible := make([]repo.RepositoryNode, 0, len(children))
	for _, child := range children {
		if frontier.Purpose != "absence" && requiresAbsenceSettle(payload.VolumeKind) && payload.Settle > 0 {
			candidate, err := queries.MarkRepositoryNodeAbsenceCandidateCAS(ctx, repo.MarkRepositoryNodeAbsenceCandidateCASParams{
				RepositoryID: run.RepositoryID, NodeID: child.NodeID,
				AbsenceFirstObservedAt: int64Ptr(payload.Now.Time.UnixMicro()), UpdatedAt: payload.Now,
			})
			if err != nil {
				return result, err
			}
			if candidate.AbsenceFirstObservedAt == nil ||
				payload.Now.Time.UnixMicro()-*candidate.AbsenceFirstObservedAt < payload.Settle.Microseconds() {
				continue
			}
			child = candidate
		}
		eligible = append(eligible, child)
	}
	firstRevision := int64(0)
	if len(eligible) > 0 {
		firstRevision, err = queries.AllocateRepositoryObservationRevisionRange(ctx, repo.AllocateRepositoryObservationRevisionRangeParams{
			RepositoryID: run.RepositoryID, NextRevision: int64(len(eligible)), UpdatedAt: payload.Now,
		})
		if err != nil {
			return result, err
		}
	}
	visited := 0
	allApplied := true
	for index, child := range eligible {
		if visited > 0 && time.Since(started) >= payload.TransactionBudget {
			allApplied = false
			break
		}
		visited++
		revision := firstRevision + int64(index)
		tombstoned, err := queries.TombstoneRepositoryNodeCAS(ctx, repo.TombstoneRepositoryNodeCASParams{
			RepositoryID: run.RepositoryID, NodeID: child.NodeID,
			ObservationRevision: revision, UpdatedAt: payload.Now,
		})
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return result, err
		}
		if _, err := queries.CloseActiveAssetLocationCAS(ctx, repo.CloseActiveAssetLocationCASParams{
			NodeID: child.NodeID, UnboundObservationRevision: int64Ptr(revision), UpdatedAt: payload.Now,
		}); err != nil {
			return result, err
		}
		if tombstoned.Kind == "directory" {
			if _, err := queries.EnqueueRepositoryAbsenceCascadeFrontier(ctx, repo.EnqueueRepositoryAbsenceCascadeFrontierParams{
				RunID: run.RunID, DirectoryNodeID: tombstoned.NodeID.String(), CreatedAt: payload.Now,
			}); err != nil {
				return result, fmt.Errorf("enqueue repository absence cascade: %w", err)
			}
		}
		eventKey := fmt.Sprintf("absence:%s:%s", run.RunID, child.NodeID)
		entryKind := tombstoned.Kind
		persisted, err := queries.InsertRepositoryObservation(ctx, repo.InsertRepositoryObservationParams{
			ObservationID: uuid.New(), RepositoryID: run.RepositoryID, Revision: revision,
			RunID: uuid.NullUUID{UUID: run.RunID, Valid: true}, Source: "verifier", SourceEventKey: &eventKey,
			ParentNodeID: uuid.NullUUID{UUID: parentNodeID, Valid: true},
			Name:         &tombstoned.Name, NameKey: &tombstoned.NameKey, EntryKind: &entryKind,
			MappedNodeID: uuid.NullUUID{UUID: tombstoned.NodeID, Valid: true}, CreatedAt: payload.Now,
		})
		if err != nil {
			return result, err
		}
		if persisted.ProcessingState != "applied" {
			if _, err := queries.CompleteRepositoryObservationCAS(ctx, repo.CompleteRepositoryObservationCASParams{
				RepositoryID: run.RepositoryID, ObservationID: persisted.ObservationID,
				MappedNodeID:    uuid.NullUUID{UUID: tombstoned.NodeID, Valid: true},
				ProcessingState: "applied", ProcessedAt: int64Ptr(payload.Now.Time.UnixMicro()),
			}); err != nil && !errors.Is(err, sql.ErrNoRows) {
				return result, err
			}
		}
		result.RowsApplied++
	}
	last := children[len(children)-1].NodeID.String()
	if !allApplied && visited > 0 {
		last = eligible[visited-1].NodeID.String()
	}
	_, err = queries.ContinueRepositoryAbsenceFrontier(ctx, repo.ContinueRepositoryAbsenceFrontierParams{
		RunID: run.RunID, DirectoryNodeID: frontier.DirectoryNodeID,
		AbsenceCursor: last, UpdatedAt: payload.Now, LeaseID: &payload.Lease,
	})
	return result, err
}

func (applier *commitApplier) finishRun(ctx context.Context, tx *sql.Tx, queries *repo.Queries, payload observationCommit) (TurnResult, error) {
	run := payload.Run
	state := payload.State
	result := TurnResult{OperationID: run.RunID, Status: StatusPartial}
	status := StatusCompleted
	partial := int64(0)
	fullVerificationRequired := int64(0)
	if !payload.Authoritative {
		status = StatusPartial
		partial = 1
		fullVerificationRequired = 1
	}
	if payload.Authoritative {
		if _, err := queries.UpsertRepositoryChangeCursor(ctx, repo.UpsertRepositoryChangeCursorParams{
			RepositoryID: run.RepositoryID, AdapterKind: state.AdapterKind,
			Cursor: run.CursorEnd, VolumeIdentity: state.VolumeIdentity,
			JournalIdentity: state.AdapterIdentity, Status: string(changefeed.HealthHealthy),
			AppliedRevision: max(int64(0), state.NextRevision-1), UpdatedAt: payload.Now,
		}); err != nil {
			return result, fmt.Errorf("advance repository change cursor: %w", err)
		}
	}
	if _, err := queries.TransitionRepositoryScanRun(ctx, repo.TransitionRepositoryScanRunParams{
		RunID: run.RunID, Status: status, UpdatedAt: payload.Now,
		PartialCoverage: partial, FailureCode: run.FailureCode,
		FailureProblemType: run.FailureProblemType, Status_2: StatusFinalizing,
	}); err != nil {
		return result, fmt.Errorf("finish repository observation run: %w", err)
	}
	advanced, err := queries.AdvanceRepositoryObservationEpoch(ctx, repo.AdvanceRepositoryObservationEpochParams{
		RepositoryID: run.RepositoryID, AppliedEpoch: run.RequestedEpoch,
		FullVerificationRequired: fullVerificationRequired, UpdatedAt: payload.Now,
	})
	if err != nil {
		return result, fmt.Errorf("advance repository observation epoch: %w", err)
	}
	if err := scheduleCoalescedRepositoryRun(ctx, tx, queries, run, advanced, payload.FollowUpRunID, payload.Now); err != nil {
		return result, err
	}
	result.Status = status
	result.HasMore = false
	return result, nil
}

func (applier *commitApplier) failFrontier(ctx context.Context, queries *repo.Queries, payload observationCommit) error {
	if _, err := queries.CompleteRepositoryScanFrontier(ctx, repo.CompleteRepositoryScanFrontierParams{
		RunID: payload.RunID, DirectoryNodeID: payload.Frontier.DirectoryNodeID,
		AuthoritativeChildSet: 0, ErrorCode: &payload.FailureCode,
		UpdatedAt: payload.Now, LeaseID: &payload.Lease,
	}); err != nil {
		return fmt.Errorf("record failed repository frontier: %w", err)
	}
	outboxDepth, err := queries.CountPendingRepositoryMaterialization(ctx, payload.RepositoryID)
	if err != nil {
		return err
	}
	_, err = queries.UpdateRepositoryScanRunProgress(ctx, repo.UpdateRepositoryScanRunProgressParams{
		RunID: payload.RunID, ErrorDirectories: 1, OutboxDepth: outboxDepth, UpdatedAt: payload.Now,
	})
	return err
}

func (applier *commitApplier) cancelRun(ctx context.Context, tx *sql.Tx, queries *repo.Queries, payload observationCommit) error {
	run := payload.Run
	if _, err := queries.TransitionRepositoryScanRun(ctx, repo.TransitionRepositoryScanRunParams{
		RunID: run.RunID, Status: StatusCancelled, UpdatedAt: payload.Now,
		PartialCoverage: 1, Status_2: run.Status,
	}); err != nil {
		return err
	}
	state, err := queries.AdvanceRepositoryObservationEpoch(ctx, repo.AdvanceRepositoryObservationEpochParams{
		RepositoryID: run.RepositoryID, AppliedEpoch: run.RequestedEpoch,
		FullVerificationRequired: 1, UpdatedAt: payload.Now,
	})
	if err != nil {
		return err
	}
	return scheduleCoalescedRepositoryRun(ctx, tx, queries, run, state, payload.FollowUpRunID, payload.Now)
}

func scheduleCoalescedRepositoryRun(
	ctx context.Context,
	tx *sql.Tx,
	queries *repo.Queries,
	completed repo.RepositoryScanRun,
	state repo.RepositoryObservationState,
	runID uuid.UUID,
	now dbtypes.Timestamp,
) error {
	if state.DesiredEpoch <= completed.RequestedEpoch {
		return nil
	}
	if runID == uuid.Nil {
		return errors.New("coalesced repository observation run ID is required")
	}
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
	return nil
}
