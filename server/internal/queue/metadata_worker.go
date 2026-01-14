package queue

import (
	"context"
	"fmt"

	"github.com/riverqueue/river"

	"server/internal/queue/jobs"
)

// MetadataArgs is the job payload alias to avoid import cycles.
type MetadataArgs = jobs.MetadataArgs

// MetadataWorker executes metadata extraction tasks (EXIF / ffprobe).
// Provide a Process function to plug in concrete behavior.
type MetadataWorker struct {
	river.WorkerDefaults[MetadataArgs]

	// Process performs metadata extraction for the given asset.
	// It should be provided by the caller when registering the worker.
	Process func(ctx context.Context, args MetadataArgs) error
}

func (w *MetadataWorker) Work(ctx context.Context, job *river.Job[MetadataArgs]) error {
	if w.Process == nil {
		return fmt.Errorf("metadata worker not configured")
	}
	return w.Process(ctx, job.Args)
}
