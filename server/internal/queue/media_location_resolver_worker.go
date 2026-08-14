package queue

import (
	"context"
	"fmt"
	"time"

	"github.com/riverqueue/river"

	"server/internal/queue/jobs"
)

type ResolveLocationClustersArgs = jobs.ResolveLocationClustersArgs

type LocationResolverService interface {
	ResolveLocationClusters(ctx context.Context, geocodingRevision int64) (time.Duration, error)
}

// ResolveLocationClustersWorker keeps one revisioned resolver job alive while
// durable pending rows remain. Provider failures are recorded by the service;
// only database/queue failures escape as River errors.
type ResolveLocationClustersWorker struct {
	river.WorkerDefaults[ResolveLocationClustersArgs]

	LocationService LocationResolverService
}

func (w *ResolveLocationClustersWorker) Work(ctx context.Context, job *river.Job[ResolveLocationClustersArgs]) error {
	if w.LocationService == nil {
		return fmt.Errorf("location resolver worker not configured")
	}
	delay, err := w.LocationService.ResolveLocationClusters(ctx, job.Args.GeocodingRevision)
	if err != nil {
		return err
	}
	if delay > 0 {
		return river.JobSnooze(delay)
	}
	return nil
}
