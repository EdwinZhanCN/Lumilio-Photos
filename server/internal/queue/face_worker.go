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

// ProcessFaceArgs is the job payload.
type ProcessFaceArgs = jobs.ProcessFaceArgs

// ProcessFaceWorker handles face detection and recognition for assets
type ProcessFaceWorker struct {
	river.WorkerDefaults[ProcessFaceArgs]

	FaceService    service.FaceService
	LumenService   service.LumenService
	ConfigProvider MLConfigProvider
}

func (w *ProcessFaceWorker) Work(ctx context.Context, job *river.Job[ProcessFaceArgs]) error {
	args := job.Args
	assetID := args.AssetID

	enabled, err := isMLTaskEnabled(ctx, w.ConfigProvider, "process_face")
	if err != nil {
		return fmt.Errorf("load ml settings: %w", err)
	}
	if !enabled {
		return nil
	}

	// Convert UUID to pgtype.UUID for database operations
	pgUUID := pgtype.UUID{}
	if err := pgUUID.Scan(assetID.String()); err != nil {
		return fmt.Errorf("invalid UUID: %w", err)
	}

	// If task temporarily unavailable, snooze
	if w.LumenService == nil {
		return river.JobSnooze(30 * time.Second)
	}
	if !w.LumenService.IsTaskAvailable("face_detect_and_embed") {
		return river.JobSnooze(30 * time.Second)
	}

	// Start timing the face processing
	startTime := time.Now()

	// Perform face detection using LumenService
	faceV1, err := w.LumenService.FaceDetectEmbed(ctx, args.ImageData)
	if err != nil {
		return fmt.Errorf("failed to perform face detection: %w", err)
	}

	// Calculate processing time
	processingTimeMs := int(time.Since(startTime).Milliseconds())

	// Save face results using FaceService (conversion, crops, clustering, and cleanup happen there).
	err = w.FaceService.SaveFaceResults(ctx, pgUUID, faceV1, args.ImageData, processingTimeMs)
	if err != nil {
		return fmt.Errorf("failed to save face results: %w", err)
	}

	return nil
}

// No additional types needed - using types.FaceV1 directly from lumen-sdk
