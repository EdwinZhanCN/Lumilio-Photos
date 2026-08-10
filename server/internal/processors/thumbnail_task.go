package processors

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"

	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/queue/jobs"
	"server/internal/storage"
	"server/internal/utils/imagesource"
)

// ProcessThumbnailTask handles thumbnail generation for photos and videos; waveform for audio.
func (ap *AssetProcessor) ProcessThumbnailTask(ctx context.Context, args jobs.ThumbnailArgs) error {
	start := time.Now()
	defer func() {
		ap.logger.Debug("thumbnail_task",
			zap.String("asset_id", args.AssetID.String()),
			zap.Duration("duration", time.Since(start)),
		)
	}()
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

	needsPHashFallback := false
	if err := ap.runTrackedAssetTask(
		ctx,
		args.AssetID,
		taskThumbnail,
		"Generating thumbnails",
		"Thumbnails generated",
		func() error {
			switch assetType {
			case dbtypes.AssetTypePhoto:
				fallback, err := ap.generatePhotoThumbnails(ctx, source.localPath, asset.OriginalFilename, source.files, asset)
				needsPHashFallback = fallback
				return err
			case dbtypes.AssetTypeVideo:
				info, err := ap.getVideoInfo(source.localPath)
				if err != nil {
					return err
				}
				return ap.generateVideoThumbnail(ctx, source.files, asset, source.localPath, info, ap.transcodeConfig)
			case dbtypes.AssetTypeAudio:
				// Optional waveform thumbnail for audio
				return ap.generateWaveform(ctx, source.files, asset, source.localPath)
			default:
				return fmt.Errorf("unsupported asset type for thumbnails: %s", assetType)
			}
		},
	); err != nil {
		return err
	}

	if assetType == dbtypes.AssetTypePhoto {
		if needsPHashFallback {
			if err := ap.enqueuePHashJob(ctx, args.AssetID); err != nil {
				return err
			}
		}

		if err := ap.enqueueMLJobs(ctx, asset); err != nil {
			return fmt.Errorf("enqueue ML jobs: %w", err)
		}
	}

	return nil
}

// generatePhotoThumbnails handles photo thumbnail generation with RAW support.
func (ap *AssetProcessor) generatePhotoThumbnails(ctx context.Context, fullPath, originalFilename string, files *storage.RepositoryFS, asset *repo.Asset) (bool, error) {
	reader, err := imagesource.OpenPhoto(ctx, fullPath, originalFilename)
	if err != nil {
		return false, fmt.Errorf("open photo source: %w", err)
	}
	defer reader.Close()

	return ap.generateThumbnails(ctx, reader, files, asset)
}
