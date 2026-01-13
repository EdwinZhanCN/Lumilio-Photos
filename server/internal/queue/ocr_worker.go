package queue

import (
	"context"
	"fmt"
	"server/internal/queue/jobs"
	"server/internal/service"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/riverqueue/river"
)

// ProcessOcrArgs is the job payload.
type ProcessOcrArgs = jobs.ProcessOcrArgs

// ProcessOcrWorker handles OCR text extraction for assets
type ProcessOcrWorker struct {
	river.WorkerDefaults[ProcessOcrArgs]

	OCRService service.OCRService
	LumenService service.LumenService
}

func (w *ProcessOcrWorker) Work(ctx context.Context, job *river.Job[ProcessOcrArgs]) error {
	args := job.Args
	assetID := args.AssetID

	// Convert UUID to pgtype.UUID for database operations
	pgUUID := pgtype.UUID{}
	if err := pgUUID.Scan(assetID.String()); err != nil {
		return fmt.Errorf("invalid UUID: %w", err)
	}

	// Perform OCR using LumenService
	ocrResult, err := w.LumenService.OCR(ctx, args.ImageData)
	if err != nil {
		return fmt.Errorf("failed to perform OCR: %w", err)
	}

	// Save OCR results using OCRService
	err = w.OCRService.SaveOCRResults(ctx, pgUUID, ocrResult, 0) // Processing time will be calculated inside SaveOCRResults
	if err != nil {
		return fmt.Errorf("failed to save OCR results: %w", err)
	}

	return nil
}