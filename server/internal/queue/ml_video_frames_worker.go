package queue

import (
	"context"
	"fmt"
	"time"

	"github.com/riverqueue/river"

	"server/internal/queue/jobs"
)

// ProcessVideoFramesArgs is the job payload alias to avoid import cycles.
type ProcessVideoFramesArgs = jobs.ProcessVideoFramesArgs

// ProcessVideoFramesWorker extracts and embeds video frames for semantic search.
// Provide a Process function (typically AssetProcessor.ProcessVideoFramesTask).
type ProcessVideoFramesWorker struct {
	river.WorkerDefaults[ProcessVideoFramesArgs]

	Process func(ctx context.Context, args ProcessVideoFramesArgs) error
}

func (w *ProcessVideoFramesWorker) Timeout(job *river.Job[ProcessVideoFramesArgs]) time.Duration {
	return 15 * time.Minute
}

func (w *ProcessVideoFramesWorker) Work(ctx context.Context, job *river.Job[ProcessVideoFramesArgs]) error {
	if w.Process == nil {
		return fmt.Errorf("video frames worker not configured")
	}
	return w.Process(ctx, job.Args)
}
