package queue

import (
	"context"
	"fmt"

	"github.com/riverqueue/river"

	"server/internal/queue/jobs"
)

// AssetRetryArgs is the job payload alias to avoid import cycles.
type AssetRetryArgs = jobs.AssetRetryPayload

// AssetRetryWorker executes asset retry tasks.
// Provide a ProcessRetry function to plug in concrete behavior.
type AssetRetryWorker struct {
	river.WorkerDefaults[AssetRetryArgs]

	// ProcessRetry performs retry for the given asset.
	// It should be provided by the caller when registering the worker.
	ProcessRetry func(ctx context.Context, args AssetRetryArgs) error
}

func (w *AssetRetryWorker) Work(ctx context.Context, job *river.Job[AssetRetryArgs]) error {
	if w.ProcessRetry == nil {
		return fmt.Errorf("asset retry worker not configured")
	}
	return w.ProcessRetry(ctx, job.Args)
}
