package queue

import (
	"context"
	"fmt"
	"server/internal/queue/jobs"
	"server/internal/service"
	"time"

	"github.com/edwinzhancn/lumen-sdk/pkg/types"
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
	TagService       service.AIGeneratedTagService
	ConfigProvider   MLConfigProvider
}

func (w *ProcessClipWorker) Work(ctx context.Context, job *river.Job[ProcessClipArgs]) error {
	args := job.Args
	assetID := args.AssetID

	enabled, err := isMLTaskEnabled(ctx, w.ConfigProvider, "process_clip")
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

	// If tasks are temporarily unavailable, snooze for a short period
	if w.LumenService == nil {
		return river.JobSnooze(30 * time.Second)
	}
	if !w.LumenService.IsTaskAvailable("clip_image_embed") ||
		!w.LumenService.IsTaskAvailable("clip_classify") ||
		!w.LumenService.IsTaskAvailable("clip_scene_classify") {
		return river.JobSnooze(30 * time.Second)
	}
	if w.TagService == nil {
		return fmt.Errorf("ai generated tag service unavailable")
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

	clipLabels, err := w.LumenService.ClipClassify(ctx, args.ImageData, 3)
	if err != nil {
		return fmt.Errorf("failed to classify image with CLIP: %w", err)
	}

	sceneLabels, err := w.LumenService.ClipSceneClassify(ctx, args.ImageData, 1)
	if err != nil {
		return fmt.Errorf("failed to classify image scene with CLIP: %w", err)
	}

	tags := labelsToAIGeneratedTags(clipLabels, "clip_classify")
	tags = append(tags, labelsToAIGeneratedTags(sceneLabels, "clip_scene_classify")...)

	if w.LumenService.IsTaskAvailable("bioclip_classify") {
		bioClipLabels, err := w.LumenService.BioClipClassify(ctx, args.ImageData, 3)
		if err != nil {
			return fmt.Errorf("failed to classify image with BioCLIP: %w", err)
		}
		tags = append(tags, labelsToAIGeneratedTags(bioClipLabels, "bioclip_classify")...)
	}

	if err := w.TagService.ReplaceAssetAIGeneratedTags(ctx, pgUUID, tags, []string{
		"clip_classify",
		"clip_scene_classify",
		"bioclip_classify",
	}); err != nil {
		return fmt.Errorf("failed to save ai generated tags: %w", err)
	}

	return nil
}

func labelsToAIGeneratedTags(labels []types.Label, source string) []service.AIGeneratedTag {
	tags := make([]service.AIGeneratedTag, 0, len(labels))
	for _, label := range labels {
		tags = append(tags, service.AIGeneratedTag{
			Name:       label.Label,
			Confidence: label.Score,
			Source:     source,
		})
	}
	return tags
}
