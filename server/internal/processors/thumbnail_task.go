package processors

import (
	"context"
	"fmt"
	"path/filepath"

	"server/config"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/queue/jobs"
	"server/internal/utils/imagesource"
)

// ProcessThumbnailTask handles thumbnail generation for photos and videos; waveform for audio.
func (ap *AssetProcessor) ProcessThumbnailTask(ctx context.Context, args jobs.ThumbnailArgs) error {
	asset, repository, err := ap.loadAssetAndRepo(ctx, args.AssetID)
	if err != nil {
		return err
	}

	return ap.runTrackedAssetTask(
		ctx,
		args.AssetID,
		taskThumbnail,
		"Generating thumbnails",
		"Thumbnails generated",
		func() error {
			fullPath := filepath.Join(args.RepoPath, args.StoragePath)
			switch args.AssetType {
			case dbtypes.AssetTypePhoto:
				return ap.generatePhotoThumbnails(ctx, fullPath, asset.OriginalFilename, repository, asset)
			case dbtypes.AssetTypeVideo:
				info, err := ap.getVideoInfo(fullPath)
				if err != nil {
					return err
				}
				transcodeCfg := config.LoadTranscodeConfig()
				return ap.generateVideoThumbnail(ctx, repository.Path, asset, fullPath, info, transcodeCfg)
			case dbtypes.AssetTypeAudio:
				// Optional waveform thumbnail for audio
				return ap.generateWaveform(ctx, repository.Path, asset, fullPath)
			default:
				return fmt.Errorf("unsupported asset type for thumbnails: %s", args.AssetType)
			}
		},
	)
}

// generatePhotoThumbnails handles photo thumbnail generation with RAW support.
func (ap *AssetProcessor) generatePhotoThumbnails(ctx context.Context, fullPath, originalFilename string, repository repo.Repository, asset *repo.Asset) error {
	reader, err := imagesource.OpenPhoto(ctx, fullPath, originalFilename)
	if err != nil {
		return fmt.Errorf("open photo source: %w", err)
	}
	defer reader.Close()

	return ap.generateThumbnails(ctx, reader, repository, asset)
}
