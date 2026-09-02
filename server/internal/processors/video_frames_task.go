package processors

import (
	"context"
	"errors"
	"fmt"
	"os"

	"server/internal/artifact"
	"server/internal/db/dbtypes"
	"server/internal/pipeline"
	"server/internal/utils/imagesource"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ErrVideoFramesNotReady asks the enclosing asset macro to retry after the
// transcode artifact becomes visible. It is deliberately a processor error,
// not a River control value; only the macro adapter knows how to back off.
var ErrVideoFramesNotReady = errors.New("video frames artifact is not ready")

// ProcessVideoFramesTask extracts frames from the transcoded web.mp4 and
// computes SigLIP2 vectors. Catalog activation is performed by the commit
// coordinator after this method returns.
func (ap *AssetProcessor) ProcessVideoFramesTask(ctx context.Context, args VideoFramesArgs) (VideoFramesResult, error) {
	if ap == nil {
		return VideoFramesResult{}, fmt.Errorf("asset processor is nil")
	}
	if args.AssetID == uuid.Nil {
		return VideoFramesResult{}, fmt.Errorf("invalid asset id")
	}

	mlConfig, err := ap.settingsService.GetEffectiveMLConfig(ctx)
	if err != nil {
		return VideoFramesResult{}, fmt.Errorf("load ML settings: %w", err)
	}
	if !mlConfig.SemanticEnabled || !mlConfig.VideoSemanticEnabled {
		return VideoFramesResult{}, nil
	}
	if ap.lumenService == nil {
		return VideoFramesResult{}, ErrVideoFramesNotReady
	}

	asset, repository, err := ap.loadAssetAndRepoForContent(ctx, args.AssetID, args.ExpectedContentID)
	if errors.Is(err, ErrAssetSourceStale) {
		return VideoFramesResult{}, nil
	}
	if err != nil {
		return VideoFramesResult{}, err
	}
	if dbtypes.AssetType(asset.Type) != dbtypes.AssetTypeVideo {
		return VideoFramesResult{}, fmt.Errorf("asset %s is not a video: %s", asset.AssetID.String(), asset.Type)
	}
	repositoryFS, err := ap.files.Open(repository)
	if err != nil {
		return VideoFramesResult{}, err
	}
	defer repositoryFS.Close()
	privatePath, err := (artifact.Identity{SourceFence: asset.ContentID.String(), Stage: "transcode", PipelineVersion: pipeline.AssetPipelineVersion, Name: "web.mp4"}).Path()
	if err != nil {
		return VideoFramesResult{}, err
	}
	webPath, err := repositoryFS.LocalPrivatePath(privatePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// Transcode may still be in flight on a retry race; retry later.
			return VideoFramesResult{}, ErrVideoFramesNotReady
		}
		return VideoFramesResult{}, fmt.Errorf("open web video: %w", err)
	}

	durationSec := 0.0
	if asset.Duration != nil && *asset.Duration > 0 {
		durationSec = *asset.Duration
	} else {
		info, probeErr := ap.getVideoInfo(webPath)
		if probeErr != nil {
			return VideoFramesResult{}, fmt.Errorf("probe web video duration: %w", probeErr)
		}
		durationSec = info.Duration
	}

	frames, err := ap.extractSemanticFrames(ctx, webPath, durationSec, mlConfig)
	if err != nil {
		return VideoFramesResult{}, fmt.Errorf("extract semantic frames: %w", err)
	}
	if len(frames) == 0 {
		return VideoFramesResult{}, fmt.Errorf("no semantic frames extracted")
	}

	frameEmbeddings := make([]VideoFrameEmbedding, 0, len(frames))
	var modelID string
	for _, frame := range frames {
		mlImage, imgErr := imagesource.ProcessMLImageTensorBytes(frame.Bytes, imagesource.PurposeSemantic)
		if imgErr != nil {
			return VideoFramesResult{}, fmt.Errorf("preprocess frame at %dms: %w", frame.FrameTsMs, imgErr)
		}
		embedding, embErr := ap.lumenService.SemanticImageEmbed(ctx, mlImage)
		if embErr != nil {
			return VideoFramesResult{}, fmt.Errorf("embed frame at %dms: %w", frame.FrameTsMs, embErr)
		}
		if embedding == nil || len(embedding.Vector) == 0 {
			return VideoFramesResult{}, fmt.Errorf("empty embedding for frame at %dms", frame.FrameTsMs)
		}
		if modelID == "" {
			modelID = embedding.ModelID
		}
		frameEmbeddings = append(frameEmbeddings, VideoFrameEmbedding{
			FrameTsMs: frame.FrameTsMs,
			Vector:    embedding.Vector,
		})
	}

	if _, _, err := ap.loadAssetAndRepoForContent(ctx, args.AssetID, args.ExpectedContentID); errors.Is(err, ErrAssetSourceStale) {
		return VideoFramesResult{}, nil
	} else if err != nil {
		return VideoFramesResult{}, fmt.Errorf("revalidate video asset work: %w", err)
	}

	if ap.logger != nil {
		ap.logger.Info("video semantic frames indexed",
			zap.String("asset_id", args.AssetID.String()),
			zap.Int("frames", len(frameEmbeddings)),
			zap.String("model_id", modelID),
		)
	}
	return VideoFramesResult{AssetID: asset.AssetID, SourceContentID: asset.ContentID, ModelID: modelID, Frames: frameEmbeddings}, nil
}
