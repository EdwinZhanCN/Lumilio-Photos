package processors

import (
	"context"
	"errors"
	"fmt"

	"server/internal/db/dbtypes"
	"server/internal/queue/jobs"

	"go.uber.org/zap"
)

// ProcessTranscodeTask handles video/audio transcoding.
func (ap *AssetProcessor) ProcessTranscodeTask(ctx context.Context, args jobs.TranscodeArgs) error {
	source, err := ap.resolveCurrentAssetSource(ctx, args.AssetID, args.ObservationToken, args.ExpectedContentHash)
	if err != nil {
		if errors.Is(err, ErrAssetSourceStale) {
			return nil
		}
		return err
	}
	defer source.Close()
	asset := source.asset
	assetType := dbtypes.AssetType(asset.Type)

	return ap.runTrackedAssetTask(
		ctx,
		args.AssetID,
		taskTranscode,
		"Transcoding asset",
		"Transcoding completed",
		func() error {
			switch assetType {
			case dbtypes.AssetTypeVideo:
				info, err := ap.getVideoInfo(source.localPath)
				if err != nil {
					return err
				}
				if err := ap.transcodeVideoSmart(ctx, source.files, source.path, asset, source.localPath, info, ap.transcodeConfig); err != nil {
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
				info, err := ap.getAudioInfo(source.localPath)
				if err != nil {
					return err
				}
				return ap.transcodeAudioSmart(ctx, source.files, source.path, asset, source.localPath, info)
			case dbtypes.AssetTypePhoto:
				// No transcode needed for photos
				return nil
			default:
				return fmt.Errorf("unsupported asset type for transcode: %s", assetType)
			}
		},
	)
}
