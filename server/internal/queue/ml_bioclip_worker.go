package queue

import (
	"context"
	"fmt"
	"server/internal/queue/jobs"
	"server/internal/service"
	"server/internal/utils/imagesource"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/riverqueue/river"
)

// ProcessBioClipArgs is the job payload.
type ProcessBioClipArgs = jobs.ProcessBioClipArgs

// ProcessBioClipWorker handles BioCLIP classification for assets.
type ProcessBioClipWorker struct {
	river.WorkerDefaults[ProcessBioClipArgs]

	LumenService   service.LumenService
	TagService     service.AIGeneratedTagService
	ConfigProvider MLConfigProvider
	ImageLoader    MLImageLoader
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
	if !w.LumenService.IsTaskAvailable("bioclip_classify") {
		return river.JobSnooze(30 * time.Second)
	}
	if w.TagService == nil {
		return fmt.Errorf("ai generated tag service unavailable")
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

	if err := w.TagService.ReplaceAssetAIGeneratedTags(ctx, pgUUID, labelsToAIGeneratedTags(labels, "bioclip_classify"), []string{
		"bioclip_classify",
	}); err != nil {
		return fmt.Errorf("failed to save BioCLIP tags: %w", err)
	}

	return nil
}
