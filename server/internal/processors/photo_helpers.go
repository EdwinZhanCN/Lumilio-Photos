package processors

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/riverqueue/river"

	"server/internal/db/repo"
	"server/internal/queue/jobs"
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
		MaxFileSize: 2 * 1024 * 1024 * 1024, // 2GB
		Timeout:     60 * time.Second,
		BufferSize:  128 * 1024,
		FastMode:    false, // Full EXIF for photos
		IncludeRaw:  true,
	}
}

// generateThumbnails builds all configured thumbnail sizes from the provided image stream.
func (ap *AssetProcessor) generateThumbnails(ctx context.Context, reader io.Reader, repository repo.Repository, asset *repo.Asset) error {
	outputs := make(map[string]io.Writer, len(thumbnailSizes))
	buffers := make(map[string]*bytes.Buffer, len(thumbnailSizes))

	for name := range thumbnailSizes {
		buf := &bytes.Buffer{}
		buffers[name] = buf
		outputs[name] = buf
	}

	if err := imaging.StreamThumbnails(reader, thumbnailSizes, outputs); err != nil {
		return fmt.Errorf("generate_thumbnails: %w", err)
	}

	for name, buf := range buffers {
		if buf.Len() == 0 {
			continue
		}
		if err := ap.assetService.SaveNewThumbnail(ctx, repository.Path, buf, asset, name); err != nil {
			return fmt.Errorf("save_thumbnails: %w", err)
		}
	}
	return nil
}

// enqueueMLJobs enqueues enabled ML jobs based on runtime settings.
// This is called during ingestion for photos to enqueue ML processing tasks.
func (ap *AssetProcessor) enqueueMLJobs(ctx context.Context, asset *repo.Asset) error {
	mlConfig, err := ap.settingsService.GetEffectiveMLConfig(ctx)
	if err != nil {
		return fmt.Errorf("load ML settings: %w", err)
	}

	// Early return if no ML services are enabled by runtime config.
	if !mlConfig.CLIPEnabled && !mlConfig.BioCLIPEnabled && !mlConfig.OCREnabled && !mlConfig.CaptionEnabled && !mlConfig.FaceEnabled {
		return nil
	}

	if mlConfig.CLIPEnabled {
		_, err = ap.queueClient.Insert(ctx, jobs.ProcessClipArgs{
			AssetID:           asset.AssetID,
			PreprocessVersion: jobs.MLPreprocessVersionV1,
		}, &river.InsertOpts{Queue: "process_clip"})
		if err != nil {
			return fmt.Errorf("enqueue CLIP: %w", err)
		}
	}

	if mlConfig.BioCLIPEnabled {
		_, err = ap.queueClient.Insert(ctx, jobs.ProcessBioClipArgs{
			AssetID:           asset.AssetID,
			PreprocessVersion: jobs.MLPreprocessVersionV1,
		}, &river.InsertOpts{Queue: "process_bioclip"})
		if err != nil {
			return fmt.Errorf("enqueue BioCLIP: %w", err)
		}
	}

	if mlConfig.OCREnabled {
		_, err = ap.queueClient.Insert(ctx, jobs.ProcessOcrArgs{
			AssetID:           asset.AssetID,
			PreprocessVersion: jobs.MLPreprocessVersionV1,
		}, &river.InsertOpts{Queue: "process_ocr"})
		if err != nil {
			return fmt.Errorf("enqueue OCR: %w", err)
		}
	}

	if mlConfig.CaptionEnabled {
		_, err = ap.queueClient.Insert(ctx, jobs.ProcessCaptionArgs{
			AssetID:           asset.AssetID,
			PreprocessVersion: jobs.MLPreprocessVersionV1,
		}, &river.InsertOpts{Queue: "process_caption"})
		if err != nil {
			return fmt.Errorf("enqueue caption: %w", err)
		}
	}

	if mlConfig.FaceEnabled {
		_, err = ap.queueClient.Insert(ctx, jobs.ProcessFaceArgs{
			AssetID:           asset.AssetID,
			PreprocessVersion: jobs.MLPreprocessVersionV1,
		}, &river.InsertOpts{Queue: "process_face"})
		if err != nil {
			return fmt.Errorf("enqueue face: %w", err)
		}
	}

	return nil
}
