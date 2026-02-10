package queue

import (
	"context"
	"fmt"
	"server/internal/queue/jobs"
	"server/internal/service"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/riverqueue/river"
)

// ProcessCaptionArgs is the job payload.
type ProcessCaptionArgs = jobs.ProcessCaptionArgs

// ProcessCaptionWorker handles AI image captioning for assets
type ProcessCaptionWorker struct {
	river.WorkerDefaults[ProcessCaptionArgs]

	CaptionService service.CaptionService
	LumenService   service.LumenService
}

func (w *ProcessCaptionWorker) Work(ctx context.Context, job *river.Job[ProcessCaptionArgs]) error {
	args := job.Args
	assetID := args.AssetID

	// Convert UUID to pgtype.UUID for database operations
	pgUUID := pgtype.UUID{}
	if err := pgUUID.Scan(assetID.String()); err != nil {
		return fmt.Errorf("invalid UUID: %w", err)
	}

	if !w.LumenService.IsTaskAvailable("vlm_generate") {
		return river.JobSnooze(30 * time.Second)
	}

	// Save caption using CaptionService
	_, err := w.CaptionService.GenerateAndSaveCaption(ctx, pgUUID, args.ImageData, args.CustomPrompt)
	if err != nil {
		return fmt.Errorf("failed to save caption: %w", err)
	}

	return nil
}
