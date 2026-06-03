package processors

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/riverqueue/river"
	"go.uber.org/zap"

	"server/internal/db/repo"
	"server/internal/queue/jobs"
	"server/internal/service"
	"server/internal/utils/exif"
	"server/internal/utils/imaging"
	"server/internal/utils/phash"
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

// generateThumbnails builds all configured thumbnail sizes from the provided
// image stream and opportunistically stores pHash from the generated small WebP.
func (ap *AssetProcessor) generateThumbnails(ctx context.Context, reader io.Reader, repository repo.Repository, asset *repo.Asset) (bool, error) {
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

	var smallBytes []byte
	if small, ok := buffers["small"]; ok && small.Len() > 0 {
		smallBytes = append([]byte(nil), small.Bytes()...)
	}

	for name, buf := range buffers {
		if buf.Len() == 0 {
			continue
		}
		if err := ap.assetService.SaveNewThumbnail(ctx, repository.Path, buf, asset, name); err != nil {
			return false, fmt.Errorf("save_thumbnails: %w", err)
		}
	}

	if len(smallBytes) == 0 {
		return true, nil
	}
	if err := ap.savePHashEmbeddingFromReader(ctx, asset.AssetID, bytes.NewReader(smallBytes)); err != nil {
		if ap.logger != nil {
			ap.logger.Warn("inline pHash failed; falling back to process_phash",
				zap.String("asset_id", fmt.Sprintf("%x", asset.AssetID.Bytes)),
				zap.Error(err),
			)
		}
		return true, nil
	}

	return false, nil
}

func (ap *AssetProcessor) enqueuePHashJob(ctx context.Context, assetID pgtype.UUID) error {
	if _, err := ap.queueClient.Insert(ctx, jobs.ProcessPHashArgs{
		AssetID: assetID,
	}, &river.InsertOpts{Queue: "process_phash"}); err != nil {
		return fmt.Errorf("enqueue pHash: %w", err)
	}

	return nil
}

func (ap *AssetProcessor) savePHashEmbeddingFromReader(ctx context.Context, assetID pgtype.UUID, reader io.Reader) error {
	if ap.embeddingService == nil {
		return fmt.Errorf("embedding service is not configured")
	}

	hash, err := phash.ComputeFromReader(reader)
	if err != nil {
		return err
	}

	if err := ap.embeddingService.SaveEmbedding(ctx, assetID, service.EmbeddingTypePHash, phash.ModelDCTPHashV1, phash.ToVector(hash), true); err != nil {
		return fmt.Errorf("save phash embedding: %w", err)
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
				PreprocessVersion: jobs.MLPreprocessVersionV1,
			}, &river.InsertOpts{Queue: "process_semantic"})
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
				PreprocessVersion: jobs.MLPreprocessVersionV1,
			}, &river.InsertOpts{Queue: "process_ocr"})
			if err != nil {
				return fmt.Errorf("enqueue OCR: %w", err)
			}
		}
	}

	if mlConfig.FaceEnabled {
		if ap.lumenService == nil || ap.lumenService.IsTaskAvailable("face_recognition") {
			_, err = ap.queueClient.Insert(ctx, jobs.ProcessFaceArgs{
				AssetID:           asset.AssetID,
				PreprocessVersion: jobs.MLPreprocessVersionV1,
			}, &river.InsertOpts{Queue: "process_face"})
			if err != nil {
				return fmt.Errorf("enqueue face: %w", err)
			}
		}
	}

	return nil
}
