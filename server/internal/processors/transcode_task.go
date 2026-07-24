package processors

import (
	"context"
	"fmt"
	"path/filepath"

	"server/internal/db/dbtypes"
	"server/internal/queue/jobs"

	"go.uber.org/zap"
)

// ProcessTranscodeTask handles video/audio transcoding.
func (ap *AssetProcessor) ProcessTranscodeTask(ctx context.Context, args jobs.TranscodeArgs) error {
	asset, repository, err := ap.loadAssetAndRepo(ctx, args.AssetID)
	if err != nil {
		return err
	}

	return ap.runTrackedAssetTask(
		ctx,
		args.AssetID,
		taskTranscode,
		"Transcoding asset",
		"Transcoding completed",
		func() error {
			fullPath := filepath.Join(args.RepoPath, args.StoragePath)
			switch args.AssetType {
			case dbtypes.AssetTypeVideo:
				info, err := ap.getVideoInfo(fullPath)
				if err != nil {
					return err
				}
				if err := ap.transcodeVideoSmart(ctx, repository.Path, asset, fullPath, info, ap.transcodeConfig); err != nil {
					return err
				}
				if err := ap.enqueueVideoFramesJob(ctx, asset.AssetID); err != nil {
					if ap.logger != nil {
						ap.logger.Warn("enqueue video frames after transcode failed",
							zap.String("asset_id", asset.AssetID.String()),
							zap.Error(err),
						)
					}
				}
				return nil
			case dbtypes.AssetTypeAudio:
				info, err := ap.getAudioInfo(fullPath)
				if err != nil {
					return err
				}
				return ap.transcodeAudioSmart(ctx, repository.Path, asset, fullPath, info)
			case dbtypes.AssetTypePhoto:
				// No transcode needed for photos
				return nil
			default:
				return fmt.Errorf("unsupported asset type for transcode: %s", args.AssetType)
			}
		},
	)
}
