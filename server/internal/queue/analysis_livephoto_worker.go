package queue

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/riverqueue/river"

	"server/internal/queue/jobs"
	"server/internal/service"
)

type LivePhotoMatchArgs = jobs.LivePhotoMatchArgs

// LivePhotoMatchWorker executes exact Apple Live Photo matching for a single asset.
type LivePhotoMatchWorker struct {
	river.WorkerDefaults[LivePhotoMatchArgs]
	StackService service.StackService
}

func (w *LivePhotoMatchWorker) Work(ctx context.Context, job *river.Job[LivePhotoMatchArgs]) error {
	if w.StackService == nil {
		return fmt.Errorf("live photo matcher worker not configured")
	}

	assetID, err := uuid.FromBytes(job.Args.AssetID.Bytes[:])
	if err != nil {
		return fmt.Errorf("parse asset ID: %w", err)
	}

	if err := w.StackService.MatchLivePhotoStack(ctx, assetID); err != nil {
		return fmt.Errorf("match live photo stack for asset %s: %w", assetID, err)
	}

	return nil
}
