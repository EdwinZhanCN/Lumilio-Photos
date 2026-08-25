package processors

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"

	"server/internal/db/repo"
	"server/internal/queue/jobs"
	"server/internal/storage"
	"server/internal/utils/exif"
	"server/internal/utils/imaging"
)

// Thumbnail target sizes reused across photo and video thumbnail generation.
var thumbnailSizes = map[string][2]int{
	"small":  {400, 400},
	"medium": {800, 800},
	"large":  {1920, 1920},
}

// createEXIFConfig centralizes EXIF extraction settings for photos.
func (ap *AssetProcessor) createEXIFConfig() *exif.Config {
	return &exif.Config{
		ExifToolPath: ap.toolsConfig.ExifToolCommand(),
		MaxFileSize:  2 * 1024 * 1024 * 1024, // 2GB
		Timeout:      60 * time.Second,
		BufferSize:   128 * 1024,
		FastMode:     false, // Full EXIF for photos
		IncludeRaw:   true,
	}
}

// generateThumbnails builds all configured thumbnail sizes from the provided
// image stream. pHash publication deliberately stays on its own River queue so
// CPU work, retries, and the final SQLite write are independently observable
// and cannot extend the thumbnail worker's catalog footprint.
func (ap *AssetProcessor) generateThumbnails(ctx context.Context, reader io.Reader, files *storage.RepositoryFS, asset *repo.Asset) (bool, error) {
	outputs := make(map[string]io.Writer, len(thumbnailSizes))
	buffers := make(map[string]*bytes.Buffer, len(thumbnailSizes))

	for name := range thumbnailSizes {
		buf := &bytes.Buffer{}
		buffers[name] = buf
		outputs[name] = buf
	}

	if err := imaging.StreamThumbnails(reader, thumbnailSizes, outputs); err != nil {
		return false, fmt.Errorf("generate_thumbnails: %w", err)
	}

	for name, buf := range buffers {
		if buf.Len() == 0 {
			continue
		}
		if err := ap.saveThumbnail(ctx, files, buf, asset, name); err != nil {
			return false, fmt.Errorf("save_thumbnails: %w", err)
		}
	}

	return true, nil
}

func (ap *AssetProcessor) enqueuePHashJob(ctx context.Context, assetID, expectedContentID uuid.UUID) error {
	if _, err := ap.queueClient.Insert(ctx, jobs.ProcessPHashArgs{
		AssetID: assetID, ExpectedContentID: expectedContentID,
	}, nil); err != nil {
		return fmt.Errorf("enqueue pHash: %w", err)
	}

	return nil
}

// enqueueMLJobs enqueues enabled ML jobs based on runtime settings.
// This is called during ingestion/discovery for photos to enqueue ML processing tasks.
func (ap *AssetProcessor) enqueueMLJobs(ctx context.Context, asset *repo.Asset) error {
	mlConfig, err := ap.settingsService.GetEffectiveMLConfig(ctx)
	if err != nil {
		return fmt.Errorf("load ML settings: %w", err)
	}

	// Early return if no ML services are enabled by runtime config.
	if !mlConfig.SemanticEnabled && !mlConfig.OCREnabled && !mlConfig.FaceEnabled {
		return nil
	}

	if mlConfig.SemanticEnabled {
		if ap.lumenService == nil || ap.lumenService.IsTaskAvailable("semantic_image_embed") {
			_, err = ap.queueClient.Insert(ctx, jobs.ProcessSemanticArgs{
				AssetID:           asset.AssetID,
				ExpectedContentID: asset.ContentID,
				PreprocessVersion: jobs.MLPreprocessVersionV1,
			}, nil)
			if err != nil {
				return fmt.Errorf("enqueue semantic: %w", err)
			}
			// zero-shot classification is chained off the semantic worker once
			// the embedding is written (see ProcessSemanticWorker), so no separate
			// enqueue is needed here.
		}
	}

	if mlConfig.OCREnabled {
		if ap.lumenService == nil || ap.lumenService.IsTaskAvailable("ocr") {
			_, err = ap.queueClient.Insert(ctx, jobs.ProcessOcrArgs{
				AssetID:           asset.AssetID,
				ExpectedContentID: asset.ContentID,
				PreprocessVersion: jobs.MLPreprocessVersionV1,
			}, nil)
			if err != nil {
				return fmt.Errorf("enqueue OCR: %w", err)
			}
		}
	}

	if mlConfig.FaceEnabled {
		if ap.lumenService == nil || ap.lumenService.IsTaskAvailable("face_recognition") {
			_, err = ap.queueClient.Insert(ctx, jobs.ProcessFaceArgs{
				AssetID:           asset.AssetID,
				ExpectedContentID: asset.ContentID,
				PreprocessVersion: jobs.MLPreprocessVersionV1,
			}, nil)
			if err != nil {
				return fmt.Errorf("enqueue face: %w", err)
			}
		}
	}

	return nil
}
