package queue

import (
	"context"
	"fmt"
	"server/internal/db/dbtypes"
	"server/internal/queue/jobs"
	"server/internal/service"
	"server/internal/utils/imagesource"
	"time"

	"github.com/edwinzhancn/lumen-sdk/pkg/types"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/riverqueue/river"
)

// ProcessBioClipArgs is the job payload.
type ProcessBioClipArgs = jobs.ProcessBioClipArgs

// ProcessBioClipWorker handles BioCLIP classification for assets.
type ProcessBioClipWorker struct {
	river.WorkerDefaults[ProcessBioClipArgs]

	LumenService   service.LumenService
	SpeciesService speciesPredictionSaver
	ConfigProvider MLConfigProvider
	ImageLoader    MLImageLoader
}

type speciesPredictionSaver interface {
	SaveSpeciesPredictions(ctx context.Context, assetID pgtype.UUID, predictions []dbtypes.SpeciesPredictionMeta) error
}

func (w *ProcessBioClipWorker) Work(ctx context.Context, job *river.Job[ProcessBioClipArgs]) error {
	args := job.Args
	assetID := args.AssetID

	enabled, err := isMLTaskEnabled(ctx, w.ConfigProvider, "process_bioclip")
	if err != nil {
		return fmt.Errorf("load ml settings: %w", err)
	}
	if !enabled {
		return nil
	}

	pgUUID := pgtype.UUID{}
	if err := pgUUID.Scan(assetID.String()); err != nil {
		return fmt.Errorf("invalid UUID: %w", err)
	}

	if w.LumenService == nil {
		return river.JobSnooze(30 * time.Second)
	}
	if w.SpeciesService == nil {
		return fmt.Errorf("species prediction service unavailable")
	}
	if w.ImageLoader == nil {
		return fmt.Errorf("ml image loader unavailable")
	}

	imageData, err := w.ImageLoader.LoadMLImage(ctx, assetID, imagesource.PurposeBioClip, args.PreprocessVersion)
	if err != nil {
		return fmt.Errorf("load BioCLIP image: %w", err)
	}

	labels, err := w.LumenService.BioClipClassify(ctx, imageData, 3)
	if err != nil {
		return fmt.Errorf("failed to classify image with BioCLIP: %w", err)
	}

	if err := w.SpeciesService.SaveSpeciesPredictions(ctx, pgUUID, labelsToSpeciesPredictions(labels)); err != nil {
		return fmt.Errorf("failed to save BioCLIP species predictions: %w", err)
	}

	return nil
}

func labelsToSpeciesPredictions(labels []types.Label) []dbtypes.SpeciesPredictionMeta {
	predictions := make([]dbtypes.SpeciesPredictionMeta, 0, len(labels))
	for _, label := range labels {
		predictions = append(predictions, dbtypes.SpeciesPredictionMeta{
			Label: label.Label,
			Score: label.Score,
		})
	}
	return predictions
}
