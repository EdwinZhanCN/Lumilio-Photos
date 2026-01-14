package processors

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"

	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/queue/jobs"
)

// ProcessThumbnailTask handles thumbnail generation for photos and videos; waveform for audio.
func (ap *AssetProcessor) ProcessThumbnailTask(ctx context.Context, args jobs.ThumbnailArgs) error {
	asset, repository, err := ap.loadAssetAndRepo(ctx, args.AssetID)
	if err != nil {
		return err
	}

	fullPath := filepath.Join(args.RepoPath, args.StoragePath)
	switch args.AssetType {
	case dbtypes.AssetTypePhoto:
		return ap.generatePhotoThumbnails(ctx, fullPath, asset.OriginalFilename, repository, asset)
	case dbtypes.AssetTypeVideo:
		info, err := ap.getVideoInfo(fullPath)
		if err != nil {
			return err
		}
		return ap.generateVideoThumbnail(ctx, repository.Path, asset, fullPath, info)
	case dbtypes.AssetTypeAudio:
		// Optional waveform thumbnail for audio
		return ap.generateWaveform(ctx, repository.Path, asset, fullPath)
	default:
		return fmt.Errorf("unsupported asset type for thumbnails: %s", args.AssetType)
	}
}

// generatePhotoThumbnails handles photo thumbnail generation with RAW support.
func (ap *AssetProcessor) generatePhotoThumbnails(ctx context.Context, fullPath, originalFilename string, repository repo.Repository, asset *repo.Asset) error {
	// Check if RAW and extract preview
	previewData, err := ap.extractRAWPreview(ctx, fullPath, originalFilename)
	if err != nil {
		return fmt.Errorf("extract RAW preview: %w", err)
	}

	if previewData != nil {
		// RAW file - use preview data for thumbnails
		return ap.generateThumbnails(ctx, bytes.NewReader(previewData), repository, asset)
	}

	// Not RAW or preview extraction returned nil - use original file
	f, err := os.Open(fullPath)
	if err != nil {
		return fmt.Errorf("open photo for thumbnails: %w", err)
	}
	defer f.Close()
	return ap.generateThumbnails(ctx, f, repository, asset)
}
