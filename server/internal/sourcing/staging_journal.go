package sourcing

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"

	"server/internal/commit"
	"server/internal/db/repo"
)

type stagingAction string

const (
	stagingCreate     stagingAction = "create"
	stagingClaim      stagingAction = "claim"
	stagingSetTarget  stagingAction = "set_target"
	stagingMarkOnDisk stagingAction = "mark_on_disk"
	stagingComplete   stagingAction = "complete"
	stagingQuarantine stagingAction = "quarantine"
)

type stagingMutation struct {
	Action     stagingAction
	CommitID   uuid.UUID
	Create     repo.CreateRepositoryStagingCommitParams
	Claim      repo.ClaimRepositoryStagingCommitParams
	SetTarget  repo.SetRepositoryStagingCommitTargetParams
	MarkOnDisk repo.MarkRepositoryStagingCommitOnDiskParams
	Complete   repo.CompleteRepositoryStagingCommitParams
	Quarantine repo.QuarantineRepositoryStagingCommitParams
}

type stagingAcknowledgement struct {
	Record repo.RepositoryStagingCommit
}

func (stagingAcknowledgement) CommitAcknowledgement() {}

// StagingJournal is the only mutation boundary exposed to SourceMaterializer.
// It contains the closed staging lifecycle rather than a generic database or
// query surface.
type StagingJournal interface {
	Create(context.Context, repo.CreateRepositoryStagingCommitParams) error
	Claim(context.Context, repo.ClaimRepositoryStagingCommitParams) (repo.RepositoryStagingCommit, error)
	SetTarget(context.Context, repo.SetRepositoryStagingCommitTargetParams) (repo.RepositoryStagingCommit, error)
	MarkOnDisk(context.Context, repo.MarkRepositoryStagingCommitOnDiskParams) error
	Complete(context.Context, repo.CompleteRepositoryStagingCommitParams) error
	Quarantine(context.Context, repo.QuarantineRepositoryStagingCommitParams) error
}

type CoordinatorStagingJournal struct {
	coordinator *commit.Coordinator
}

func NewCoordinatorStagingJournal(coordinator *commit.Coordinator) *CoordinatorStagingJournal {
	return &CoordinatorStagingJournal{coordinator: coordinator}
}

func (journal *CoordinatorStagingJournal) submit(ctx context.Context, mutation stagingMutation) (repo.RepositoryStagingCommit, error) {
	if journal == nil || journal.coordinator == nil || mutation.CommitID == uuid.Nil || mutation.Action == "" {
		return repo.RepositoryStagingCommit{}, errors.New("repository staging journal is not configured")
	}
	result, err := journal.coordinator.SubmitOperation(ctx, commit.Operation{
		Kind: commit.OperationKindRepositoryStaging, BatchLimit: 1,
		Apply: func(ctx context.Context, tx *sql.Tx) (commit.Result, error) {
			return applyStagingMutation(ctx, tx, mutation)
		},
	})
	if err != nil {
		return repo.RepositoryStagingCommit{}, err
	}
	acknowledgement, ok := result.Acknowledgement.(stagingAcknowledgement)
	if !ok {
		return repo.RepositoryStagingCommit{}, errors.New("repository staging commit returned no acknowledgement")
	}
	return acknowledgement.Record, nil
}

func (journal *CoordinatorStagingJournal) Create(ctx context.Context, params repo.CreateRepositoryStagingCommitParams) error {
	_, err := journal.submit(ctx, stagingMutation{Action: stagingCreate, CommitID: params.CommitID, Create: params})
	return err
}

func (journal *CoordinatorStagingJournal) Claim(ctx context.Context, params repo.ClaimRepositoryStagingCommitParams) (repo.RepositoryStagingCommit, error) {
	return journal.submit(ctx, stagingMutation{Action: stagingClaim, CommitID: params.CommitID, Claim: params})
}

func (journal *CoordinatorStagingJournal) SetTarget(ctx context.Context, params repo.SetRepositoryStagingCommitTargetParams) (repo.RepositoryStagingCommit, error) {
	return journal.submit(ctx, stagingMutation{Action: stagingSetTarget, CommitID: params.CommitID, SetTarget: params})
}

func (journal *CoordinatorStagingJournal) MarkOnDisk(ctx context.Context, params repo.MarkRepositoryStagingCommitOnDiskParams) error {
	_, err := journal.submit(ctx, stagingMutation{Action: stagingMarkOnDisk, CommitID: params.CommitID, MarkOnDisk: params})
	return err
}

func (journal *CoordinatorStagingJournal) Complete(ctx context.Context, params repo.CompleteRepositoryStagingCommitParams) error {
	_, err := journal.submit(ctx, stagingMutation{Action: stagingComplete, CommitID: params.CommitID, Complete: params})
	return err
}

func (journal *CoordinatorStagingJournal) Quarantine(ctx context.Context, params repo.QuarantineRepositoryStagingCommitParams) error {
	_, err := journal.submit(ctx, stagingMutation{Action: stagingQuarantine, CommitID: params.CommitID, Quarantine: params})
	return err
}

func applyStagingMutation(ctx context.Context, tx *sql.Tx, mutation stagingMutation) (commit.Result, error) {
	queries := repo.New(tx)
	if mutation.CommitID == uuid.Nil || mutation.Action == "" {
		return commit.Result{}, errors.New("invalid repository staging result")
	}
	var record repo.RepositoryStagingCommit
	var err error
	switch mutation.Action {
	case stagingCreate:
		record, err = queries.CreateRepositoryStagingCommit(ctx, mutation.Create)
	case stagingClaim:
		record, err = queries.ClaimRepositoryStagingCommit(ctx, mutation.Claim)
	case stagingSetTarget:
		record, err = queries.SetRepositoryStagingCommitTarget(ctx, mutation.SetTarget)
	case stagingMarkOnDisk:
		record, err = queries.MarkRepositoryStagingCommitOnDisk(ctx, mutation.MarkOnDisk)
	case stagingComplete:
		record, err = queries.CompleteRepositoryStagingCommit(ctx, mutation.Complete)
	case stagingQuarantine:
		record, err = queries.QuarantineRepositoryStagingCommit(ctx, mutation.Quarantine)
	default:
		return commit.Result{}, errors.New("unsupported repository staging mutation")
	}
	if err != nil {
		return commit.Result{}, err
	}
	return commit.Result{Outcome: commit.OutcomeApplied, Acknowledgement: stagingAcknowledgement{Record: record}}, nil
}
