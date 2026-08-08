package queue

import (
	"context"

	"github.com/riverqueue/river"

	"server/internal/processors"
	"server/internal/queue/jobs"
)

// IngestAssetArgs is the job payload alias to avoid import cycles.
type IngestAssetArgs = jobs.IngestAssetArgs

// IngestAssetWorker wraps the AssetProcessor ingest entrypoint as a River worker.
type IngestAssetWorker struct {
	river.WorkerDefaults[IngestAssetArgs]

	Processor *processors.AssetProcessor
}

func (w *IngestAssetWorker) Work(ctx context.Context, job *river.Job[IngestAssetArgs]) error {
	_, err := w.Processor.IngestAsset(ctx, job.Args)
	return err
}
