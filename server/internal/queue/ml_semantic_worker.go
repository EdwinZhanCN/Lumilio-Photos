package queue

import (
	"context"
	"fmt"
	"server/internal/queue/jobs"
	"server/internal/service"
	"server/internal/utils/imagesource"
	"time"

	"github.com/edwinzhancn/lumen-sdk/pkg/types"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/riverqueue/river"
)

// ProcessSemanticArgs is the job payload.
type ProcessSemanticArgs = jobs.ProcessSemanticArgs

// ProcessSemanticWorker handles semantic embedding generation for assets
type ProcessSemanticWorker struct {
	river.WorkerDefaults[ProcessSemanticArgs]
	EmbeddingService service.EmbeddingService
	LumenService     service.LumenService
	ConfigProvider   MLConfigProvider
	ImageLoader      MLImageLoader
}

func (w *ProcessSemanticWorker) Work(ctx context.Context, job *river.Job[ProcessSemanticArgs]) error {
	args := job.Args
	assetID := args.AssetID

	enabled, err := isMLTaskEnabled(ctx, w.ConfigProvider, "process_semantic")
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

	imageData, err := w.ImageLoader.LoadMLImage(ctx, assetID, imagesource.PurposeSemantic, args.PreprocessVersion)
	if err != nil {
		return fmt.Errorf("load semantic image: %w", err)
	}

	embedding, err := w.LumenService.SemanticImageEmbed(ctx, imageData)
	if err != nil {
		return fmt.Errorf("failed to generate semantic embedding: %w", err)
	}

	err = w.EmbeddingService.SaveEmbedding(ctx, pgUUID,
		service.EmbeddingTypeSemantic, embedding.ModelID, embedding.Vector, true)
	if err != nil {
		return fmt.Errorf("failed to save embedding: %w", err)
	}

	// Chain zero-shot classification now that the embedding exists. This
	// also doubles as backfill: re-running the semantic index over the library
	// reclassifies every asset. Best-effort: a failed enqueue must not force a
	// costly re-embed, and the next reindex will recover it.
	if classifyEnabled, cfgErr := isMLTaskEnabled(ctx, w.ConfigProvider, "classify_zeroshot"); cfgErr == nil && classifyEnabled {
		if client, clientErr := river.ClientFromContextSafely[pgx.Tx](ctx); clientErr == nil {
			_, _ = client.Insert(ctx, jobs.ZeroshotClassifyArgs{AssetID: pgUUID}, &river.InsertOpts{Queue: "classify_zeroshot"})
		}
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
