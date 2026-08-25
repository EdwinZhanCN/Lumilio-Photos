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

	"github.com/google/uuid"
	"github.com/riverqueue/river"
	"go.uber.org/zap"
)

// ProcessVideoFramesTask extracts frames from the transcoded web.mp4, embeds
// them with SigLIP2, and replaces the asset's search_embeddings rows.
func (ap *AssetProcessor) ProcessVideoFramesTask(ctx context.Context, args jobs.ProcessVideoFramesArgs) error {
	if ap == nil {
		return fmt.Errorf("asset processor is nil")
	}
	if args.AssetID == uuid.Nil {
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

	asset, repository, err := ap.loadAssetAndRepoForContent(ctx, args.AssetID, args.ExpectedContentID)
	if errors.Is(err, ErrAssetSourceStale) {
		return nil
	}
	if err != nil {
		return err
	}
	if dbtypes.AssetType(asset.Type) != dbtypes.AssetTypeVideo {
		return fmt.Errorf("asset %s is not a video: %s", asset.AssetID.String(), asset.Type)
	}
	repositoryFS, err := ap.files.Open(repository)
	if err != nil {
		return err
	}
	defer repositoryFS.Close()
	privatePath, err := derivedPath("videos", "web", fmt.Sprintf("%s_web.mp4", asset.ContentID))
	if err != nil {
		return err
	}
	webPath, err := repositoryFS.LocalPrivatePath(privatePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// Transcode may still be in flight on a retry race; snooze briefly.
			return river.JobSnooze(30 * time.Second)
		}
		return fmt.Errorf("open web video: %w", err)
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

	if _, _, err := ap.loadAssetAndRepoForContent(ctx, args.AssetID, args.ExpectedContentID); errors.Is(err, ErrAssetSourceStale) {
		return nil
	} else if err != nil {
		return fmt.Errorf("revalidate video asset work: %w", err)
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
func (ap *AssetProcessor) enqueueVideoFramesJob(ctx context.Context, assetID, expectedContentID uuid.UUID) error {
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
		ExpectedContentID: expectedContentID,
		PreprocessVersion: jobs.MLPreprocessVersionV1,
	}, nil)
	return err
}
