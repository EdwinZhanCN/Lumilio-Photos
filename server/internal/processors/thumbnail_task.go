package processors

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"go.uber.org/zap"

	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/queue/jobs"
	"server/internal/utils/imagesource"
)

// ProcessThumbnailTask handles thumbnail generation for photos and videos; waveform for audio.
func (ap *AssetProcessor) ProcessThumbnailTask(ctx context.Context, args jobs.ThumbnailArgs) error {
	start := time.Now()
	defer func() {
		ap.logger.Debug("thumbnail_task",
			zap.String("asset_id", args.AssetID.String()),
			zap.String("type", string(args.AssetType)),
			zap.Duration("duration", time.Since(start)),
		)
	}()
	asset, repository, err := ap.loadAssetAndRepo(ctx, args.AssetID)
	if err != nil {
		return err
	}

	needsPHashFallback := false
	if err := ap.runTrackedAssetTask(
		ctx,
		args.AssetID,
		taskThumbnail,
		"Generating thumbnails",
		"Thumbnails generated",
		func() error {
			fullPath := filepath.Join(args.RepoPath, args.StoragePath)
			switch args.AssetType {
			case dbtypes.AssetTypePhoto:
				fallback, err := ap.generatePhotoThumbnails(ctx, fullPath, asset.OriginalFilename, repository, asset)
				needsPHashFallback = fallback
				return err
			case dbtypes.AssetTypeVideo:
				info, err := ap.getVideoInfo(fullPath)
				if err != nil {
					return err
				}
				return ap.generateVideoThumbnail(ctx, repository.Path, asset, fullPath, info, ap.transcodeConfig)
			case dbtypes.AssetTypeAudio:
				// Optional waveform thumbnail for audio
				return ap.generateWaveform(ctx, repository.Path, asset, fullPath)
			default:
				return fmt.Errorf("unsupported asset type for thumbnails: %s", args.AssetType)
			}
		},
	); err != nil {
		return err
	}

	if args.AssetType == dbtypes.AssetTypePhoto {
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
func (ap *AssetProcessor) generatePhotoThumbnails(ctx context.Context, fullPath, originalFilename string, repository repo.Repository, asset *repo.Asset) (bool, error) {
	reader, err := imagesource.OpenPhoto(ctx, fullPath, originalFilename)
	if err != nil {
		return false, fmt.Errorf("open photo source: %w", err)
	}
	defer reader.Close()

	return ap.generateThumbnails(ctx, reader, repository, asset)
}
