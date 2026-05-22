package queue

import (
	"context"
	"fmt"

	"github.com/riverqueue/river"

	"server/internal/queue/jobs"
)

type RebuildLocationClustersArgs = jobs.RebuildLocationClustersArgs

type LocationClusterService interface {
	RebuildLocationClusters(ctx context.Context, repositoryID *string, ownerID *int32) error
}

type RebuildLocationClustersWorker struct {
	river.WorkerDefaults[RebuildLocationClustersArgs]

	LocationService LocationClusterService
}

func (w *RebuildLocationClustersWorker) Work(ctx context.Context, job *river.Job[RebuildLocationClustersArgs]) error {
	if w.LocationService == nil {
		return fmt.Errorf("location cluster worker not configured")
	}
	return w.LocationService.RebuildLocationClusters(ctx, job.Args.RepositoryID, job.Args.OwnerID)
}
