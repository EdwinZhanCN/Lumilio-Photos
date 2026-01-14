package queue

import (
	"context"
	"fmt"
	"server/internal/db/dbtypes"
	"server/internal/queue/jobs"
	"server/internal/service"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/riverqueue/river"
)

// ProcessClipArgs is the job payload.
type ProcessClipArgs = jobs.ProcessClipArgs

// ProcessClipWorker handles CLIP embedding generation for assets
type ProcessClipWorker struct {
	river.WorkerDefaults[ProcessClipArgs]
	EmbeddingService service.EmbeddingService
	LumenService     service.LumenService
	SpeciesService   service.SpeciesService
}

func (w *ProcessClipWorker) Work(ctx context.Context, job *river.Job[ProcessClipArgs]) error {
	args := job.Args
	assetID := args.AssetID

	// Convert UUID to pgtype.UUID for database operations
	pgUUID := pgtype.UUID{}
	if err := pgUUID.Scan(assetID.String()); err != nil {
		return fmt.Errorf("invalid UUID: %w", err)
	}

	embedding, err := w.LumenService.ClipImageEmbed(ctx, args.ImageData)
	if err != nil {
		return fmt.Errorf("failed to generate CLIP embedding: %w", err)
	}

	err = w.EmbeddingService.SaveEmbedding(ctx, pgUUID,
		service.EmbeddingTypeCLIP, embedding.ModelID, embedding.Vector, true)
	if err != nil {
		return fmt.Errorf("failed to save embedding: %w", err)
	}

	labels, err := w.LumenService.BioClipClassify(ctx, args.ImageData, 3)
	if err != nil {
		return fmt.Errorf("failed to classify image: %w", err)
	}

	// Save species predictions to ML metadata if available
	if labels != nil && len(labels) > 0 {
		predictions := make([]dbtypes.SpeciesPredictionMeta, len(labels))
		for i, pred := range labels {
			predictions[i] = dbtypes.SpeciesPredictionMeta{
				Label: pred.Label,
				Score: pred.Score,
			}
		}

		// Save species predictions to database
		err = w.SpeciesService.SaveSpeciesPredictions(ctx, pgUUID, predictions)
		if err != nil {
			// Log error but don't fail the job
			fmt.Printf("Failed to save species predictions: %v\n", err)
		}
	}

	return nil
}
