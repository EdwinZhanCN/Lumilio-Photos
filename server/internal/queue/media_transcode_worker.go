package queue

import (
	"context"
	"fmt"

	"github.com/riverqueue/river"

	"server/internal/queue/jobs"
)

// TranscodeArgs is the job payload alias to avoid import cycles.
type TranscodeArgs = jobs.TranscodeArgs

// TranscodeWorker executes audio/video transcoding tasks.
// Provide a Process function to plug in concrete behavior.
type TranscodeWorker struct {
	river.WorkerDefaults[TranscodeArgs]

	// Process performs transcoding for the given asset.
	// It should be provided by the caller when registering the worker.
	Process func(ctx context.Context, args TranscodeArgs) error
}

func (w *TranscodeWorker) Work(ctx context.Context, job *river.Job[TranscodeArgs]) error {
	if w.Process == nil {
		return fmt.Errorf("transcode worker not configured")
	}
	return w.Process(ctx, job.Args)
}
