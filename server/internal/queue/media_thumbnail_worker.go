package queue

import (
	"context"
	"fmt"

	"github.com/riverqueue/river"

	"server/internal/queue/jobs"
)

// ThumbnailArgs is the job payload alias to avoid import cycles.
type ThumbnailArgs = jobs.ThumbnailArgs

// ThumbnailWorker executes thumbnail generation tasks.
// Provide a Process function to plug in concrete behavior.
type ThumbnailWorker struct {
	river.WorkerDefaults[ThumbnailArgs]

	// Process performs thumbnail generation for the given asset.
	// It should be provided by the caller when registering the worker.
	Process func(ctx context.Context, args ThumbnailArgs) error
}

func (w *ThumbnailWorker) Work(ctx context.Context, job *river.Job[ThumbnailArgs]) error {
	if w.Process == nil {
		return fmt.Errorf("thumbnail worker not configured")
	}
	return w.Process(ctx, job.Args)
}
