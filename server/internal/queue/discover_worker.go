package queue

import (
	"context"
	"fmt"

	"github.com/riverqueue/river"

	"server/internal/queue/jobs"
)

// DiscoverAssetArgs is the job payload alias to avoid import cycles.
type DiscoverAssetArgs = jobs.DiscoverAssetArgs

// DiscoverAssetWorker executes repository discovery ingestion tasks.
type DiscoverAssetWorker struct {
	river.WorkerDefaults[DiscoverAssetArgs]

	// ProcessDiscover ingests discovered repository files.
	ProcessDiscover func(ctx context.Context, args DiscoverAssetArgs) error
}

func (w *DiscoverAssetWorker) Work(ctx context.Context, job *river.Job[DiscoverAssetArgs]) error {
	if w.ProcessDiscover == nil {
		return fmt.Errorf("discover worker not configured")
	}
	return w.ProcessDiscover(ctx, job.Args)
}
