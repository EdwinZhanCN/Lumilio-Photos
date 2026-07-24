package processors

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"server/internal/db/dbtypes"
	"server/internal/queue/jobs"
	"server/internal/service"
	"server/internal/utils/imagesource"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/riverqueue/river"
	"go.uber.org/zap"
)

// ProcessVideoFramesTask extracts frames from the transcoded web.mp4, embeds
// them with SigLIP2, and replaces the asset's search_embeddings rows.
func (ap *AssetProcessor) ProcessVideoFramesTask(ctx context.Context, args jobs.ProcessVideoFramesArgs) error {
	if ap == nil {
		return fmt.Errorf("asset processor is nil")
	}
	if !args.AssetID.Valid {
		return fmt.Errorf("invalid asset id")
	}

	mlConfig, err := ap.settingsService.GetEffectiveMLConfig(ctx)
	if err != nil {
		return fmt.Errorf("load ML settings: %w", err)
	}
	if !mlConfig.SemanticEnabled || !mlConfig.VideoSemanticEnabled {
		return nil
	}
	if ap.lumenService == nil {
		return river.JobSnooze(30 * time.Second)
	}
	if ap.embeddingService == nil {
		return fmt.Errorf("embedding service unavailable")
	}

	asset, repository, err := ap.loadAssetAndRepo(ctx, args.AssetID)
	if err != nil {
		return err
	}
	if dbtypes.AssetType(asset.Type) != dbtypes.AssetTypeVideo {
		return fmt.Errorf("asset %s is not a video: %s", asset.AssetID.String(), asset.Type)
	}
	if asset.ContentHash == "" {
		return fmt.Errorf("asset content hash is required")
	}

	webPath := webVideoPath(repository.Path, asset.ContentHash)
	if _, err := os.Stat(webPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// Transcode may still be in flight on a retry race; snooze briefly.
			return river.JobSnooze(30 * time.Second)
		}
		return fmt.Errorf("stat web video: %w", err)
	}

	durationSec := 0.0
	if asset.Duration != nil && *asset.Duration > 0 {
		durationSec = *asset.Duration
	} else {
		info, probeErr := ap.getVideoInfo(webPath)
		if probeErr != nil {
			return fmt.Errorf("probe web video duration: %w", probeErr)
		}
		durationSec = info.Duration
	}

	frames, err := ap.extractSemanticFrames(ctx, webPath, durationSec, mlConfig)
	if err != nil {
		return fmt.Errorf("extract semantic frames: %w", err)
	}
	if len(frames) == 0 {
		return fmt.Errorf("no semantic frames extracted")
	}

	frameEmbeddings := make([]service.VideoFrameEmbedding, 0, len(frames))
	var modelID string
	for _, frame := range frames {
		mlImage, imgErr := imagesource.ProcessMLImageTensorBytes(frame.Bytes, imagesource.PurposeSemantic)
		if imgErr != nil {
			return fmt.Errorf("preprocess frame at %dms: %w", frame.FrameTsMs, imgErr)
		}
		embedding, embErr := ap.lumenService.SemanticImageEmbed(ctx, mlImage)
		if embErr != nil {
			return fmt.Errorf("embed frame at %dms: %w", frame.FrameTsMs, embErr)
		}
		if embedding == nil || len(embedding.Vector) == 0 {
			return fmt.Errorf("empty embedding for frame at %dms", frame.FrameTsMs)
		}
		if modelID == "" {
			modelID = embedding.ModelID
		}
		frameEmbeddings = append(frameEmbeddings, service.VideoFrameEmbedding{
			FrameTsMs: frame.FrameTsMs,
			Vector:    embedding.Vector,
		})
	}

	if err := ap.embeddingService.SaveVideoFrameEmbeddings(ctx, args.AssetID, modelID, frameEmbeddings); err != nil {
		return fmt.Errorf("save video frame embeddings: %w", err)
	}

	if ap.logger != nil {
		ap.logger.Info("video semantic frames indexed",
			zap.String("asset_id", args.AssetID.String()),
			zap.Int("frames", len(frameEmbeddings)),
			zap.String("model_id", modelID),
		)
	}
	return nil
}

// enqueueVideoFramesJob inserts a process_video_frames job when video semantic
// indexing is enabled. Best-effort: failures are logged by the caller.
func (ap *AssetProcessor) enqueueVideoFramesJob(ctx context.Context, assetID pgtype.UUID) error {
	if ap == nil || ap.queueClient == nil {
		return fmt.Errorf("queue client unavailable")
	}
	mlConfig, err := ap.settingsService.GetEffectiveMLConfig(ctx)
	if err != nil {
		return fmt.Errorf("load ML settings: %w", err)
	}
	if !mlConfig.SemanticEnabled || !mlConfig.VideoSemanticEnabled {
		return nil
	}
	_, err = ap.queueClient.Insert(ctx, jobs.ProcessVideoFramesArgs{
		AssetID:           assetID,
		PreprocessVersion: jobs.MLPreprocessVersionV1,
	}, &river.InsertOpts{Queue: "process_video_frames"})
	return err
}
