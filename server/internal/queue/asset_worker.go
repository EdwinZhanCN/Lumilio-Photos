package queue

import (
	"context"

	"github.com/riverqueue/river"

	"server/internal/processors"
	"server/internal/queue/jobs"
)

// ProcessAssetArgs is an alias to jobs.ProcessAssetArgs to avoid import cycles.
type ProcessAssetArgs = jobs.ProcessAssetArgs

// ProcessAssetWorker wrap the AssetProcessor as a RiverQueue Worker
type ProcessAssetWorker struct {
	river.WorkerDefaults[ProcessAssetArgs]
	Processor *processors.AssetProcessor
}

func (w *ProcessAssetWorker) Work(ctx context.Context, job *river.Job[ProcessAssetArgs]) error {
	_, err := w.Processor.ProcessAsset(ctx, processors.AssetPayload{
		ClientHash:  job.Args.ClientHash,
		StagedPath:  job.Args.StagedPath,
		UserID:      job.Args.UserID,
		Timestamp:   job.Args.Timestamp,
		ContentType: job.Args.ContentType,
		FileName:    job.Args.FileName,
	})
	return err
}
