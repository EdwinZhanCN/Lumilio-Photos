package queue

import (
	"context"
	"fmt"
	"server/internal/queue/jobs"
	"server/internal/service"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/riverqueue/river"
)

// ProcessCaptionArgs is the job payload.
type ProcessCaptionArgs = jobs.ProcessCaptionArgs

// ProcessCaptionWorker handles AI image captioning for assets
type ProcessCaptionWorker struct {
	river.WorkerDefaults[ProcessCaptionArgs]

	AIDescriptionService service.AIDescriptionService
	LumenService         service.LumenService
}

func (w *ProcessCaptionWorker) Work(ctx context.Context, job *river.Job[ProcessCaptionArgs]) error {
	args := job.Args
	assetID := args.AssetID

	// Convert UUID to pgtype.UUID for database operations
	pgUUID := pgtype.UUID{}
	if err := pgUUID.Scan(assetID.String()); err != nil {
		return fmt.Errorf("invalid UUID: %w", err)
	}

	// Save AI description using AIDescriptionService
	_, err := w.AIDescriptionService.GenerateAndSaveDescription(ctx, pgUUID, args.ImageData, args.CustomPrompt)
	if err != nil {
		return fmt.Errorf("failed to save AI description: %w", err)
	}

	return nil
}
