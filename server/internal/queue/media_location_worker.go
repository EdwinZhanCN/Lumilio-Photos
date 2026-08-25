package queue

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/riverqueue/river"

	"server/internal/db/repo"
	"server/internal/queue/jobs"
)

const maxLocationScopesPerScheduleTurn = 100

type RebuildLocationClustersArgs = jobs.RebuildLocationClustersArgs

type LocationClusterService interface {
	RebuildLocationClusters(ctx context.Context, repositoryID *string, ownerID *int32) (moreWork bool, err error)
}

type RebuildLocationClustersWorker struct {
	river.WorkerDefaults[RebuildLocationClustersArgs]

	LocationService LocationClusterService
}

func (w *RebuildLocationClustersWorker) Work(ctx context.Context, job *river.Job[RebuildLocationClustersArgs]) error {
	if w.LocationService == nil {
		return fmt.Errorf("location cluster worker not configured")
	}
	moreWork, err := w.LocationService.RebuildLocationClusters(ctx, job.Args.RepositoryID, job.Args.OwnerID)
	if err != nil {
		return err
	}
	if moreWork {
		return river.JobSnooze(0)
	}
	return nil
}

type ScheduleLocationRebuildsArgs = jobs.ScheduleLocationRebuildsArgs

type ScheduleLocationRebuildsWorker struct {
	river.WorkerDefaults[ScheduleLocationRebuildsArgs]

	ReadDB *sql.DB
}

func (w *ScheduleLocationRebuildsWorker) Work(ctx context.Context, _ *river.Job[ScheduleLocationRebuildsArgs]) error {
	if w.ReadDB == nil {
		return fmt.Errorf("location rebuild scheduler not configured")
	}
	scopes, err := repo.New(w.ReadDB).ListPendingLocationProjectionScopes(ctx, maxLocationScopesPerScheduleTurn)
	if err != nil {
		return fmt.Errorf("list pending location projections: %w", err)
	}
	client, err := river.ClientFromContextSafely[*sql.Tx](ctx)
	if err != nil {
		return err
	}
	for _, scope := range scopes {
		repositoryID := scope.RepositoryID.String()
		ownerID := scope.OwnerID
		args := jobs.RebuildLocationClustersArgs{RepositoryID: &repositoryID, OwnerID: &ownerID}
		opts := args.InsertOpts()
		if _, err := client.Insert(ctx, args, &opts); err != nil {
			return fmt.Errorf("enqueue location projection %s/%d: %w", repositoryID, ownerID, err)
		}
	}
	return nil
}
