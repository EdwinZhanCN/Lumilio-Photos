package queue

import (
	"context"
	"errors"
	"fmt"
	"time"

	"server/internal/queue/jobs"
	"server/internal/service"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
)

// ClassifySiglipArgs is the job payload.
type ClassifySiglipArgs = jobs.ClassifySiglipArgs

// ClassifySiglipWorker scores an asset's stored CLIP image embedding against the
// configured SigLIP zero-shot classifiers and persists matches as asset tags.
// It never invokes the ML model: classification is pure vector math over the
// embedding written by ProcessClipWorker.
type ClassifySiglipWorker struct {
	river.WorkerDefaults[ClassifySiglipArgs]
	EmbeddingService  service.EmbeddingService
	ClassifierService service.ClassifierService
	AITagService      service.AIGeneratedTagService
	ConfigProvider    MLConfigProvider
}

func (w *ClassifySiglipWorker) Work(ctx context.Context, job *river.Job[ClassifySiglipArgs]) error {
	assetID := job.Args.AssetID

	enabled, err := isMLTaskEnabled(ctx, w.ConfigProvider, "classify_siglip")
	if err != nil {
		return fmt.Errorf("load ml settings: %w", err)
	}
	if !enabled {
		return nil
	}

	embedding, err := w.EmbeddingService.GetPrimaryEmbeddingVector(ctx, assetID, service.EmbeddingTypeCLIP)
	if err != nil {
		// The CLIP embedding may not have landed yet; snooze and retry.
		if errors.Is(err, pgx.ErrNoRows) {
			return river.JobSnooze(30 * time.Second)
		}
		return fmt.Errorf("load primary CLIP embedding: %w", err)
	}

	hits, err := w.ClassifierService.Classify(ctx, embedding)
	if err != nil {
		return fmt.Errorf("classify asset: %w", err)
	}

	tags := make([]service.AIGeneratedTag, 0, len(hits))
	for _, hit := range hits {
		tags = append(tags, service.AIGeneratedTag{
			Name:       hit.TagName,
			Confidence: float32(hit.Confidence),
			Source:     service.AssetTagSourceSiglipZeroshot,
			Category:   hit.Category,
		})
	}

	// Passing the source clears any prior siglip tags first, so assets that no
	// longer match a classifier have their stale tags removed on re-run.
	if err := w.AITagService.ReplaceAssetAIGeneratedTags(ctx, assetID, tags, []string{service.AssetTagSourceSiglipZeroshot}); err != nil {
		return fmt.Errorf("replace siglip tags: %w", err)
	}

	return nil
}
