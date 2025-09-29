package processors

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/queue/jobs"
	"server/internal/utils/exif"
	"server/internal/utils/imaging"
	"server/internal/utils/raw"
	"time"

	"github.com/h2non/bimg"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/riverqueue/river"
	"golang.org/x/sync/errgroup"
)

var thumbnailSizes = map[string][2]int{
	"small":  {400, 400},
	"medium": {800, 800},
	"large":  {1920, 1920},
}

type CLIPPayload struct {
	AssetID   pgtype.UUID
	ImageData []byte
}

func (ap *AssetProcessor) processPhotoAsset(
	ctx context.Context,
	asset *repo.Asset,
	fileReader io.Reader,
) error {
	// First check if this is a RAW file
	isRAWFile := raw.IsRAWFile(asset.OriginalFilename)

	if isRAWFile {
		return ap.processRAWAsset(ctx, asset, fileReader)
	} else {
		return ap.processStandardPhotoAsset(ctx, asset, fileReader)
	}
}

// processRAWAsset handles RAW file processing with the strategy: embedded preview first, then full render
func (ap *AssetProcessor) processRAWAsset(
	ctx context.Context,
	asset *repo.Asset,
	fileReader io.Reader,
) error {
	log.Printf("Processing RAW file: %s", asset.OriginalFilename)

	// Convert reader to ReadSeeker (needed for RAW processing)
	data, err := io.ReadAll(fileReader)
	if err != nil {
		return fmt.Errorf("failed to read RAW data: %w", err)
	}
	readSeeker := bytes.NewReader(data)

	// Create RAW processor
	rawOpts := raw.DefaultProcessingOptions()
	rawOpts.PreferEmbedded = true
	rawProcessor := raw.NewProcessor(rawOpts)

	// Process RAW file
	timeoutCtx, cancel := context.WithTimeout(ctx, 45*time.Second) // Longer timeout for RAW
	defer cancel()

	rawResult, err := rawProcessor.ProcessRAW(timeoutCtx, readSeeker, asset.OriginalFilename)
	if err != nil {
		return fmt.Errorf("failed to process RAW: %w", err)
	}

	if len(rawResult.PreviewData) == 0 {
		return fmt.Errorf("RAW processing produced no preview data")
	}

	log.Printf("RAW processed using %v strategy (embedded: %v) in %v",
		rawResult.Strategy, rawResult.UsedEmbedded, rawResult.ProcessingTime)

	// Now process the preview data like a regular photo
	previewReader := bytes.NewReader(rawResult.PreviewData)

	// Use the standard processing pipeline but with different readers
	extractor := exif.NewExtractor(nil)

	// Reset the original reader for EXIF extraction (we want RAW EXIF data)
	readSeeker.Seek(0, io.SeekStart)

	exifR, exifW := io.Pipe()
	clipR, clipW := io.Pipe()
	thumbR, thumbW := io.Pipe()
	defer exifR.Close()
	defer clipR.Close()
	defer thumbR.Close()

	clipEnabled := ap.appConfig.CLIPEnabled
	if !clipEnabled {
		_ = clipW.Close()
	}

	// Copy RAW data for EXIF extraction and preview data for thumbnails/CLIP
	go func() {
		defer exifW.Close()
		if clipEnabled {
			defer clipW.Close()
		}
		defer thumbW.Close()

		// Copy RAW data to EXIF extractor
		go func() {
			defer exifW.Close()
			_, _ = io.Copy(exifW, readSeeker)
		}()

		// Copy preview data to thumbnail generator and CLIP processor
		writers := []io.Writer{thumbW}
		if clipEnabled {
			writers = append(writers, clipW)
		}
		mw := io.MultiWriter(writers...)
		_, _ = io.Copy(mw, previewReader)
	}()

	g, gCtx := errgroup.WithContext(timeoutCtx)

	go func() {
		<-timeoutCtx.Done()
		exifW.Close()
		thumbW.Close()
		if clipEnabled {
			clipW.Close()
		}
	}()

	// Extract EXIF from original RAW file
	g.Go(func() error {
		req := &exif.StreamingExtractRequest{
			Reader:    exifR,
			AssetType: dbtypes.AssetType(asset.Type),
			Filename:  asset.OriginalFilename,
			Size:      asset.FileSize,
		}
		exifResult, err := extractor.ExtractFromStream(gCtx, req)
		if err != nil {
			return fmt.Errorf("extract exif: %w", err)
		}
		if meta, ok := exifResult.Metadata.(*dbtypes.PhotoSpecificMetadata); ok {
			// Mark as RAW file
			meta.IsRAW = true

			// Add dimensions from processed preview if available
			if rawResult.Width > 0 && rawResult.Height > 0 {
				// Note: These are preview dimensions, not full RAW dimensions
				// Full RAW dimensions would require parsing RAW metadata
			}

			sm, err := dbtypes.MarshalMeta(meta)
			if err != nil {
				return fmt.Errorf("metadata marshal: %w", err)
			}

			if err := ap.assetService.UpdateAssetMetadata(gCtx, asset.AssetID.Bytes, sm); err != nil {
				return fmt.Errorf("save exif meta: %w", err)
			}
		}
		return nil
	})

	// Generate thumbnails from preview data
	g.Go(func() error {
		outputs := make(map[string]io.Writer, len(thumbnailSizes))
		buffers := make(map[string]*bytes.Buffer, len(thumbnailSizes))
		for name := range thumbnailSizes {
			buf := &bytes.Buffer{}
			buffers[name] = buf
			outputs[name] = buf
		}

		if err := imaging.StreamThumbnails(thumbR, thumbnailSizes, outputs); err != nil {
			return fmt.Errorf("stream thumbnails: %w", err)
		}

		for name, buf := range buffers {
			if buf.Len() == 0 {
				continue
			}
			if err := ap.assetService.SaveNewThumbnail(gCtx, buf, asset, name); err != nil {
				return fmt.Errorf("save thumb %s: %w", name, err)
			}
		}
		return nil
	})

	// Process for CLIP if enabled
	if clipEnabled {
		g.Go(func() error {
			tinyOpts := bimg.Options{
				Width:     224,
				Height:    224,
				Crop:      true,
				Gravity:   bimg.GravitySmart,
				Quality:   90,
				Type:      bimg.WEBP,
				NoProfile: true,
			}
			tiny, err := imaging.ProcessImageStream(clipR, tinyOpts)
			if err != nil {
				return fmt.Errorf("process CLIP thumbnail: %w", err)
			}

			payload := CLIPPayload{
				asset.AssetID,
				tiny,
			}
			_, err = ap.queueClient.Insert(gCtx, jobs.ProcessClipArgs(payload), &river.InsertOpts{Queue: "process_clip"})
			if err != nil {
				return fmt.Errorf("enqueue CLIP job: %w", err)
			}
			_, _ = io.Copy(io.Discard, clipR)
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return err
	}

	return nil
}

// processStandardPhotoAsset handles non-RAW photo processing
func (ap *AssetProcessor) processStandardPhotoAsset(
	ctx context.Context,
	asset *repo.Asset,
	fileReader io.Reader,
) error {
	extractor := exif.NewExtractor(nil)

	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	exifR, exifW := io.Pipe()
	clipR, clipW := io.Pipe()
	thumbR, thumbW := io.Pipe()
	defer exifR.Close()
	defer clipR.Close()
	defer thumbR.Close()

	clipEnabled := ap.appConfig.CLIPEnabled
	if !clipEnabled {
		_ = clipW.Close()
	}

	go func() {
		defer exifW.Close()
		if clipEnabled {
			defer clipW.Close()
		}
		defer thumbW.Close()

		writers := []io.Writer{exifW, thumbW}
		if clipEnabled {
			writers = append(writers, clipW)
		}
		mw := io.MultiWriter(writers...)
		_, _ = io.Copy(mw, fileReader)
	}()

	g, gCtx := errgroup.WithContext(timeoutCtx)

	go func() {
		<-timeoutCtx.Done()
		// Closing writers will unblock io.Copy and downstream readers
		exifW.Close()
		thumbW.Close()
		if clipEnabled {
			clipW.Close()
		}
	}()

	g.Go(func() error {
		req := &exif.StreamingExtractRequest{
			Reader:    exifR,
			AssetType: dbtypes.AssetType(asset.Type),
			Filename:  asset.OriginalFilename,
			Size:      asset.FileSize,
		}
		exifResult, err := extractor.ExtractFromStream(gCtx, req)
		if err != nil {
			return fmt.Errorf("extract exif: %w", err)
		}
		if meta, ok := exifResult.Metadata.(*dbtypes.PhotoSpecificMetadata); ok {
			// Mark as non-RAW
			meta.IsRAW = false

			sm, err := dbtypes.MarshalMeta(meta)
			if err != nil {
				return fmt.Errorf("metadata marshal: %w", err)
			}

			if err := ap.assetService.UpdateAssetMetadata(gCtx, asset.AssetID.Bytes, sm); err != nil {
				return fmt.Errorf("save exif meta: %w", err)
			}
		}
		return nil
	})

	g.Go(func() error {
		// 准备 outputs
		outputs := make(map[string]io.Writer, len(thumbnailSizes))
		buffers := make(map[string]*bytes.Buffer, len(thumbnailSizes))
		for name := range thumbnailSizes {
			buf := &bytes.Buffer{}
			buffers[name] = buf
			outputs[name] = buf
		}

		if err := imaging.StreamThumbnails(thumbR, thumbnailSizes, outputs); err != nil {
			return fmt.Errorf("stream thumbnails: %w", err)
		}

		for name, buf := range buffers {
			if buf.Len() == 0 {
				continue
			}
			if err := ap.assetService.SaveNewThumbnail(gCtx, buf, asset, name); err != nil {
				return fmt.Errorf("save thumb %s: %w", name, err)
			}
		}
		return nil
	})

	if clipEnabled {
		g.Go(func() error {
			tinyOpts := bimg.Options{
				Width:     224,
				Height:    224,
				Crop:      true,
				Gravity:   bimg.GravitySmart,
				Quality:   90,
				Type:      bimg.WEBP,
				NoProfile: true,
			}
			tiny, err := imaging.ProcessImageStream(clipR, tinyOpts)

			if err != nil {
				return fmt.Errorf("process CLIP thumbnail: %w", err)
			}

			payload := CLIPPayload{
				asset.AssetID,
				tiny,
			}
			_, err = ap.queueClient.Insert(gCtx, jobs.ProcessClipArgs(payload), &river.InsertOpts{Queue: "process_clip"})
			if err != nil {
				return fmt.Errorf("enqueue CLIP job: %w", err)
			}
			_, _ = io.Copy(io.Discard, clipR)
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return err
	}

	return nil
}
