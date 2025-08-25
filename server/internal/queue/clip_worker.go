package queue

import (
	"context"
	"fmt"
	"server/internal/db/dbtypes"
	"server/internal/queue/jobs"
	"server/internal/service"

	"github.com/riverqueue/river"
)

// ProcessClipArgs is the job payload (reusing your processors.CLIPPayload struct).
type ProcessClipArgs = jobs.ProcessClipArgs

// Kind is defined on jobs.ProcessClipArgs

type ProcessClipWorker struct {
	river.WorkerDefaults[ProcessClipArgs]

	Dispatcher   *ClipBatchDispatcher // injected at bootstrap; call Dispatcher.Start(context) once
	AssetService service.AssetService
}

func (w *ProcessClipWorker) Work(ctx context.Context, job *river.Job[ProcessClipArgs]) error {
	args := job.Args
	assetID := args.AssetID.String()

	// Submit to batcher (this will batch with other concurrent jobs).
	res, err := w.Dispatcher.Submit(ctx, assetID, args.ImageData, "image/webp")
	if err != nil {
		return fmt.Errorf("batch submit: %w", err)
	}

	// 1) Store embeddings in the database (replace with your persistence)
	err = w.AssetService.SaveNewEmbedding(ctx, args.AssetID, res.Embedding.Vector)
	if err != nil {
		return fmt.Errorf("save embedding: %w", err)
	}

	// 2) Store smart classification (labels + meta["source"])
	if res.Labels != nil {
		preds := make([]dbtypes.SpeciesPredictionMeta, 0, len(res.Labels.Labels))
		for _, l := range res.Labels.Labels {
			preds = append(preds, dbtypes.SpeciesPredictionMeta{
				Label: l.Label,
				Score: l.Score,
			})
		}
		if len(preds) > 0 {
			if err = w.AssetService.SaveNewSpeciesPredictions(ctx, args.AssetID, preds); err != nil {
				return fmt.Errorf("save prediction: %w", err)
			}
		}
	}
	return nil
}
