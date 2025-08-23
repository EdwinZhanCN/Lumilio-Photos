package queue

import (
	"context"

	"github.com/riverqueue/river"

	"server/internal/processors"
)

// ProcessAssetArgs Define the arguments of this Job
type ProcessAssetArgs processors.AssetPayload

func (ProcessAssetArgs) Kind() string { return "process_asset" }

// ProcessAssetWorker wrap the AssetProcessor as a RiverQueue Worker
type ProcessAssetWorker struct {
	river.WorkerDefaults[ProcessAssetArgs]
	Processor *processors.AssetProcessor
}

func (w *ProcessAssetWorker) Work(ctx context.Context, job *river.Job[ProcessAssetArgs]) error {
	_, err := w.Processor.ProcessAsset(ctx, processors.AssetPayload(job.Args))
	return err
}
