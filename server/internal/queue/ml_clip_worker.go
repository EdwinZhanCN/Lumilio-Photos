package queue

import (
	"context"
	"fmt"
	"server/internal/queue/jobs"
	"server/internal/service"
	"server/internal/utils/imagesource"
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
	ConfigProvider   MLConfigProvider
	ImageLoader      MLImageLoader
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
	if w.ImageLoader == nil {
		return fmt.Errorf("ml image loader unavailable")
	}

	imageData, err := w.ImageLoader.LoadMLImage(ctx, assetID, imagesource.PurposeClip, args.PreprocessVersion)
	if err != nil {
		return fmt.Errorf("load CLIP image: %w", err)
	}

	embedding, err := w.LumenService.SemanticImageEmbed(ctx, imageData)
	if err != nil {
		return fmt.Errorf("failed to generate CLIP embedding: %w", err)
	}

	err = w.EmbeddingService.SaveEmbedding(ctx, pgUUID,
		service.EmbeddingTypeCLIP, embedding.ModelID, embedding.Vector, true)
	if err != nil {
		return fmt.Errorf("failed to save embedding: %w", err)
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
