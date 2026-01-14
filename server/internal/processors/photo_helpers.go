package processors

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/h2non/bimg"
	"github.com/riverqueue/river"

	"server/internal/db/repo"
	"server/internal/queue/jobs"
	"server/internal/utils/exif"
	"server/internal/utils/imaging"
	"server/internal/utils/raw"
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

// extractRAWPreview attempts to extract a JPEG preview from a RAW file.
// Returns preview data if successful, nil if not RAW or extraction fails.
// This helper is used by both thumbnail and CLIP processing to avoid duplicate RAW rendering.
func (ap *AssetProcessor) extractRAWPreview(ctx context.Context, fullPath string, originalFilename string) ([]byte, error) {
	// Check if file is RAW
	if !raw.IsRAWFile(originalFilename) {
		return nil, nil // Not RAW, caller should use original file
	}

	// Open file for RAW processing
	f, err := os.Open(fullPath)
	if err != nil {
		return nil, fmt.Errorf("open RAW file: %w", err)
	}
	defer f.Close()

	// Configure RAW processor with reasonable defaults
	opts := raw.DefaultProcessingOptions()
	opts.FullRenderTimeout = 30 * time.Second
	opts.PreferEmbedded = true
	opts.Quality = 90

	processor := raw.NewProcessor(opts)

	// Process RAW file
	result, err := processor.ProcessRAW(ctx, f, originalFilename)
	if err != nil {
		return nil, fmt.Errorf("process RAW: %w", err)
	}

	if !result.IsRAW {
		return nil, fmt.Errorf("file not detected as RAW")
	}

	if len(result.PreviewData) == 0 {
		return nil, fmt.Errorf("no preview data generated")
	}

	return result.PreviewData, nil
}

// enqueueMLJobs enqueues enabled ML jobs based on appConfig.MLConfig.
// This is called during ingestion for photos to enqueue ML processing tasks.
func (ap *AssetProcessor) enqueueMLJobs(ctx context.Context, asset *repo.Asset, fullPath string) error {
	mlConfig := ap.appConfig.MLConfig

	// Early return if no ML services enabled
	if !mlConfig.CLIPEnabled && !mlConfig.OCREnabled && !mlConfig.CaptionEnabled && !mlConfig.FaceEnabled {
		return nil
	}

	// Extract RAW preview once (shared by all ML jobs)
	previewData, err := ap.extractRAWPreview(ctx, fullPath, asset.OriginalFilename)
	if err != nil {
		return fmt.Errorf("extract RAW preview for ML: %w", err)
	}

	// Prepare image input reader
	var imageInput io.Reader
	var closeFunc func() error

	if previewData != nil {
		// RAW file - use preview data
		imageInput = bytes.NewReader(previewData)
		closeFunc = func() error { return nil }
	} else {
		// Not RAW - use original file
		f, err := os.Open(fullPath)
		if err != nil {
			return fmt.Errorf("open photo for ML: %w", err)
		}
		imageInput = f
		closeFunc = f.Close
	}
	defer closeFunc()

	// CLIP: requires 224x224 WEBP
	if mlConfig.CLIPEnabled {
		clipData, err := imaging.ProcessImageStream(imageInput, bimg.Options{
			Width:     224,
			Height:    224,
			Crop:      true,
			Gravity:   bimg.GravitySmart,
			Quality:   90,
			Type:      bimg.WEBP,
			NoProfile: true,
		})
		if err != nil {
			return fmt.Errorf("CLIP preprocessing: %w", err)
		}

		_, err = ap.queueClient.Insert(ctx, jobs.ProcessClipArgs{
			AssetID:   asset.AssetID,
			ImageData: clipData,
		}, &river.InsertOpts{Queue: "process_clip"})
		if err != nil {
			return fmt.Errorf("enqueue CLIP: %w", err)
		}
	}

	// OCR: requires medium resolution for text extraction
	if mlConfig.OCREnabled {
		// Reset reader for OCR (need to re-read if already consumed by CLIP)
		if previewData != nil {
			imageInput = bytes.NewReader(previewData)
		} else {
			f, err := os.Open(fullPath)
			if err != nil {
				return fmt.Errorf("open photo for OCR: %w", err)
			}
			defer f.Close()
			imageInput = f
		}

		ocrData, err := imaging.ProcessImageStream(imageInput, bimg.Options{
			Width:     1920,
			Height:    1920,
			Quality:   90,
			Type:      bimg.JPEG,
			NoProfile: true,
		})
		if err != nil {
			return fmt.Errorf("OCR preprocessing: %w", err)
		}

		_, err = ap.queueClient.Insert(ctx, jobs.ProcessOcrArgs{
			AssetID:   asset.AssetID,
			ImageData: ocrData,
		}, &river.InsertOpts{Queue: "process_ocr"})
		if err != nil {
			return fmt.Errorf("enqueue OCR: %w", err)
		}
	}

	// Caption: requires medium-high resolution for AI captioning
	if mlConfig.CaptionEnabled {
		// Reset reader for Caption
		if previewData != nil {
			imageInput = bytes.NewReader(previewData)
		} else {
			f, err := os.Open(fullPath)
			if err != nil {
				return fmt.Errorf("open photo for caption: %w", err)
			}
			defer f.Close()
			imageInput = f
		}

		captionData, err := imaging.ProcessImageStream(imageInput, bimg.Options{
			Width:     1024,
			Height:    1024,
			Quality:   85,
			Type:      bimg.JPEG,
			NoProfile: true,
		})
		if err != nil {
			return fmt.Errorf("caption preprocessing: %w", err)
		}

		_, err = ap.queueClient.Insert(ctx, jobs.ProcessCaptionArgs{
			AssetID:   asset.AssetID,
			ImageData: captionData,
		}, &river.InsertOpts{Queue: "process_caption"})
		if err != nil {
			return fmt.Errorf("enqueue caption: %w", err)
		}
	}

	// Face: requires medium-high resolution for face detection
	if mlConfig.FaceEnabled {
		// Reset reader for Face
		if previewData != nil {
			imageInput = bytes.NewReader(previewData)
		} else {
			f, err := os.Open(fullPath)
			if err != nil {
				return fmt.Errorf("open photo for face: %w", err)
			}
			defer f.Close()
			imageInput = f
		}

		faceData, err := imaging.ProcessImageStream(imageInput, bimg.Options{
			Width:     1920,
			Height:    1920,
			Quality:   90,
			Type:      bimg.JPEG,
			NoProfile: true,
		})
		if err != nil {
			return fmt.Errorf("face preprocessing: %w", err)
		}

		_, err = ap.queueClient.Insert(ctx, jobs.ProcessFaceArgs{
			AssetID:   asset.AssetID,
			ImageData: faceData,
		}, &river.InsertOpts{Queue: "process_face"})
		if err != nil {
			return fmt.Errorf("enqueue face: %w", err)
		}
	}

	return nil
}
