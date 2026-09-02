package controller

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"server/internal/db"
	"server/internal/db/catalogtx"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
)

// Commands owns the foreground repository-observation command transactions.
// It is deliberately separate from Controller, whose asynchronous turns have
// only a query-only reader and the typed commit coordinator.
type Commands struct {
	database *db.DB
	cfg      Config
	logger   *zap.Logger
	now      func() time.Time
}

func NewCommands(database *db.DB, cfg Config, logger *zap.Logger) *Commands {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Commands{
		database: database,
		cfg:      cfg.normalized(),
		logger:   logger.With(zap.String("component", "repository_observation_commands")),
		now:      func() time.Time { return time.Now().UTC() },
	}
}

// Request merges a durable desired epoch and operation receipt in the same
// transaction as its controller wakeup outbox effect.
func (commands *Commands) Request(
	ctx context.Context,
	repositoryID uuid.UUID,
	mode string,
	requestedBy string,
	forceFullVerification bool,
) (Receipt, error) {
	if commands == nil || commands.database == nil {
		return Receipt{}, errors.New("repository observation commands unavailable")
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
	now := dbtypes.NewTimestamp(commands.now())
	receipt := Receipt{RepositoryID: repositoryID, Mode: mode, Status: StatusQueued}
	err := commands.database.WithTx(ctx, catalogtx.OperationRepositoryObservationRequest, func(tx *sql.Tx, queries *repo.Queries) error {
		if _, err := queries.GetRepository(ctx, repositoryID); err != nil {
			return fmt.Errorf("load repository: %w", err)
		}
		semantics := commands.cfg.DefaultPathSemantics
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

func (commands *Commands) EnqueueManualScan(ctx context.Context, repositoryID, requestedBy string, force bool) (Receipt, error) {
	parsed, err := uuid.Parse(strings.TrimSpace(repositoryID))
	if err != nil {
		return Receipt{}, fmt.Errorf("invalid repository ID: %w", err)
	}
	return commands.Request(ctx, parsed, "manual", requestedBy, force)
}

func (commands *Commands) EnqueuePeriodicScan(ctx context.Context, repositoryID string) (Receipt, error) {
	parsed, err := uuid.Parse(strings.TrimSpace(repositoryID))
	if err != nil {
		return Receipt{}, fmt.Errorf("invalid repository ID: %w", err)
	}
	return commands.Request(ctx, parsed, "periodic", "", true)
}

func (commands *Commands) EnqueueAllPeriodicScans(ctx context.Context) error {
	if commands == nil || commands.database == nil {
		return errors.New("repository observation commands unavailable")
	}
	repositories, err := commands.database.ReaderQueries.ListActiveRepositories(ctx)
	if err != nil {
		return fmt.Errorf("list repositories for periodic observation: %w", err)
	}
	var requestErrors []error
	for _, repository := range repositories {
		if _, err := commands.Request(ctx, repository.RepoID, "periodic", "", true); err != nil {
			commands.logger.Warn("request periodic repository observation",
				zap.String("repository_id", repository.RepoID.String()), zap.Error(err))
			requestErrors = append(requestErrors, fmt.Errorf("repository %s: %w", repository.RepoID, err))
		}
	}
	return errors.Join(requestErrors...)
}

func (commands *Commands) GetScanRun(ctx context.Context, repositoryID, operationID string) (repo.RepositoryScanRun, error) {
	repositoryUUID, err := uuid.Parse(strings.TrimSpace(repositoryID))
	if err != nil {
		return repo.RepositoryScanRun{}, fmt.Errorf("invalid repository ID: %w", err)
	}
	operationUUID, err := uuid.Parse(strings.TrimSpace(operationID))
	if err != nil {
		return repo.RepositoryScanRun{}, fmt.Errorf("invalid operation ID: %w", err)
	}
	return commands.database.ReaderQueries.GetRepositoryScanRun(ctx, repo.GetRepositoryScanRunParams{
		RepositoryID: repositoryUUID, RunID: operationUUID,
	})
}

func (commands *Commands) GetLatestScanRun(ctx context.Context, repositoryID string) (repo.RepositoryScanRun, error) {
	repositoryUUID, err := uuid.Parse(strings.TrimSpace(repositoryID))
	if err != nil {
		return repo.RepositoryScanRun{}, fmt.Errorf("invalid repository ID: %w", err)
	}
	return commands.database.ReaderQueries.GetLatestRepositoryScanRun(ctx, repositoryUUID)
}

func (commands *Commands) ListScanRuns(ctx context.Context, repositoryID string, limit, offset int32) ([]repo.RepositoryScanRun, error) {
	repositoryUUID, err := uuid.Parse(strings.TrimSpace(repositoryID))
	if err != nil {
		return nil, fmt.Errorf("invalid repository ID: %w", err)
	}
	return commands.database.ReaderQueries.ListRepositoryScanRuns(ctx, repo.ListRepositoryScanRunsParams{
		RepositoryID: repositoryUUID, Limit: int64(limit), Offset: int64(offset),
	})
}

// CancelScanRun is a foreground command. The asynchronous controller observes
// the flag before its next turn and commits the terminal transition through the
// coordinator.
func (commands *Commands) CancelScanRun(ctx context.Context, repositoryID, operationID string) (repo.RepositoryScanRun, error) {
	repositoryUUID, err := uuid.Parse(strings.TrimSpace(repositoryID))
	if err != nil {
		return repo.RepositoryScanRun{}, fmt.Errorf("invalid repository ID: %w", err)
	}
	operationUUID, err := uuid.Parse(strings.TrimSpace(operationID))
	if err != nil {
		return repo.RepositoryScanRun{}, fmt.Errorf("invalid operation ID: %w", err)
	}
	var cancelled repo.RepositoryScanRun
	err = commands.database.WithTx(ctx, catalogtx.OperationRepositoryObservationCancelRequest, func(_ *sql.Tx, queries *repo.Queries) error {
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
			RunID: operationUUID, UpdatedAt: dbtypes.NewTimestamp(commands.now()),
		})
		return err
	})
	return cancelled, err
}
