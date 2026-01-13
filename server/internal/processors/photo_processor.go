package processors

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"regexp"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/queue/jobs"
	"server/internal/utils/exif"
	"server/internal/utils/imaging"
	"server/internal/utils/raw"
	"strconv"
	"time"

	"github.com/h2non/bimg"

	"server/internal/utils/errgroup"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/riverqueue/river"
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

// processRAWAsset handles RAW file processing with the strategy: embedded preview first, then full render
func (ap *AssetProcessor) processRAWAsset(
	ctx context.Context,
	repository repo.Repository,
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

	// Create streaming pipeline
	config := ap.createEXIFConfig()
	extractor := exif.NewExtractor(config)

	// Reset the original reader for EXIF extraction (we want RAW EXIF data)
	readSeeker.Seek(0, io.SeekStart)

	streams := ap.createStreams()
	defer ap.closeStreams(streams)

	// Copy RAW data for EXIF extraction and preview data for thumbnails/CLIP
	go func() {
		defer streams.EXIF.Writer.Close()
		if ap.appConfig.MLConfig.CLIPEnabled {
			defer streams.CLIP.Writer.Close()
		}
		defer streams.Thumb.Writer.Close()

		// Copy RAW data to EXIF extractor
		go func() {
			defer streams.EXIF.Writer.Close()
			_, _ = io.Copy(streams.EXIF.Writer, readSeeker)
		}()

		// Copy preview data to thumbnail generator and CLIP processor
		writers := []io.Writer{streams.Thumb.Writer}
		if ap.appConfig.MLConfig.CLIPEnabled {
			writers = append(writers, streams.CLIP.Writer)
		}
		mw := io.MultiWriter(writers...)
		_, _ = io.Copy(mw, previewReader)
	}()

	g := errgroup.NewFaultTolerant()

	go func() {
		<-timeoutCtx.Done()
		streams.EXIF.Writer.Close()
		streams.Thumb.Writer.Close()
		if ap.appConfig.MLConfig.CLIPEnabled {
			streams.CLIP.Writer.Close()
		}
	}()

	// Extract EXIF from original RAW file
	g.Go(func() error {
		req := &exif.StreamingExtractRequest{
			Reader:    streams.EXIF.Reader,
			AssetType: dbtypes.AssetType(asset.Type),
			Filename:  asset.OriginalFilename,
			Size:      asset.FileSize,
		}
		exifResult, err := extractor.ExtractFromStream(timeoutCtx, req)
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

			if err := ap.assetService.UpdateAssetMetadata(timeoutCtx, asset.AssetID.Bytes, sm); err != nil {
				return fmt.Errorf("save exif meta: %w", err)
			}
		}
		return nil
	})

	// Generate thumbnails from preview data
	g.Go(func() error {
		return ap.generateThumbnails(timeoutCtx, streams.Thumb.Reader, repository, asset)
	})

	// Process for CLIP if enabled
	if ap.appConfig.MLConfig.CLIPEnabled {
		g.Go(func() error {
			return ap.processCLIP(timeoutCtx, streams.CLIP.Reader, asset)
		})
	}

	// Wait for all tasks to complete, but don't fail the entire process if some tasks fail
	errors := g.Wait()
	if len(errors) > 0 {
		ap.logProcessingErrors(errors, "RAW photo processing")
		// Return success even if some tasks failed, as partial processing is acceptable
	}

	return nil
}

// processStandardPhotoAsset handles non-RAW photo processing
func (ap *AssetProcessor) processStandardPhotoAsset(
	ctx context.Context,
	repository repo.Repository,
	asset *repo.Asset,
	fileReader io.Reader,
) error {
	// Create streaming pipeline
	config := ap.createEXIFConfig()
	extractor := exif.NewExtractor(config)

	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	streams := ap.createStreams()
	defer ap.closeStreams(streams)

	// Copy image data to all processors
	go func() {
		defer streams.EXIF.Writer.Close()
		if ap.appConfig.MLConfig.CLIPEnabled {
			defer streams.CLIP.Writer.Close()
		}
		defer streams.Thumb.Writer.Close()

		writers := []io.Writer{streams.EXIF.Writer, streams.Thumb.Writer}
		if ap.appConfig.MLConfig.CLIPEnabled {
			writers = append(writers, streams.CLIP.Writer)
		}
		mw := io.MultiWriter(writers...)
		_, _ = io.Copy(mw, fileReader)
	}()

	g := errgroup.NewFaultTolerant()

	go func() {
		<-timeoutCtx.Done()
		// Closing writers will unblock io.Copy and downstream readers
		streams.EXIF.Writer.Close()
		streams.Thumb.Writer.Close()
		if ap.appConfig.MLConfig.CLIPEnabled {
			streams.CLIP.Writer.Close()
		}
	}()

	g.Go(func() error {
		req := &exif.StreamingExtractRequest{
			Reader:    streams.EXIF.Reader,
			AssetType: dbtypes.AssetType(asset.Type),
			Filename:  asset.OriginalFilename,
			Size:      asset.FileSize,
		}
		exifResult, err := extractor.ExtractFromStream(timeoutCtx, req)
		if err != nil {
			return fmt.Errorf("extract_exif: %w", err)
		}
		if meta, ok := exifResult.Metadata.(*dbtypes.PhotoSpecificMetadata); ok {
			// Mark as non-RAW
			meta.IsRAW = false

			sm, err := dbtypes.MarshalMeta(meta)
			if err != nil {
				return fmt.Errorf("metadata marshal: %w", err)
			}

			re := regexp.MustCompile(`(\d+)\D+(\d+)`)
			matches := re.FindStringSubmatch(meta.Dimensions)
			if len(matches) != 3 {
				return fmt.Errorf("invalid dimensions format: %s", meta.Dimensions)
			}

			width, _ := strconv.ParseInt(matches[1], 10, 32)
			height, _ := strconv.ParseInt(matches[2], 10, 32)

			if err := ap.assetService.UpdateAssetDimensions(timeoutCtx, asset.AssetID.Bytes, int32(width), int32(height)); err != nil {
				return fmt.Errorf("update_asset_dimensions: %w", err)
			}

			if err := ap.assetService.UpdateAssetMetadata(timeoutCtx, asset.AssetID.Bytes, sm); err != nil {
				return fmt.Errorf("save_exif_meta: %w", err)
			}
		}
		return nil
	})

	g.Go(func() error {
		return ap.generateThumbnails(timeoutCtx, streams.Thumb.Reader, repository, asset)
	})

	if ap.appConfig.MLConfig.CLIPEnabled {
		g.Go(func() error {
			return ap.processCLIP(timeoutCtx, streams.CLIP.Reader, asset)
		})
	}

	// Wait for all tasks to complete, but don't fail the entire process if some tasks fail
	errors := g.Wait()
	if len(errors) > 0 {
		ap.logProcessingErrors(errors, "Standard photo processing")
		// Return success even if some tasks failed, as partial processing is acceptable
	}

	return nil
}

// generateThumbnails generates thumbnails for all configured sizes
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

// processCLIP processes image for CLIP embedding generation
func (ap *AssetProcessor) processCLIP(ctx context.Context, reader io.Reader, asset *repo.Asset) error {
	tinyOpts := bimg.Options{
		Width:     224,
		Height:    224,
		Crop:      true,
		Gravity:   bimg.GravitySmart,
		Quality:   90,
		Type:      bimg.WEBP,
		NoProfile: true,
	}

	tiny, err := imaging.ProcessImageStream(reader, tinyOpts)
	if err != nil {
		return fmt.Errorf("clip_processing: %w", err)
	}

	payload := CLIPPayload{
		AssetID:   asset.AssetID,
		ImageData: tiny,
	}

	_, err = ap.queueClient.Insert(ctx, jobs.ProcessClipArgs(payload), &river.InsertOpts{Queue: "process_clip"})
	if err != nil {
		return fmt.Errorf("clip_processing: %w", err)
	}

	_, _ = io.Copy(io.Discard, reader)
	return nil
}

// ImageStreams holds the streaming pipes for image processing
type ImageStreams struct {
	EXIF struct {
		Reader io.ReadCloser
		Writer io.WriteCloser
	}
	Thumb struct {
		Reader io.ReadCloser
		Writer io.WriteCloser
	}
	CLIP struct {
		Reader io.ReadCloser
		Writer io.WriteCloser
	}
}

// createEXIFConfig creates the EXIF extraction configuration
func (ap *AssetProcessor) createEXIFConfig() *exif.Config {
	return &exif.Config{
		MaxFileSize: 2 * 1024 * 1024 * 1024, // 2GB
		Timeout:     60 * time.Second,
		BufferSize:  128 * 1024,
		FastMode:    false, // Don't use fast mode for photos to get complete EXIF data
	}
}

// createStreams creates the streaming pipes for concurrent processing
func (ap *AssetProcessor) createStreams() *ImageStreams {
	streams := &ImageStreams{}

	// Create pipes
	streams.EXIF.Reader, streams.EXIF.Writer = io.Pipe()
	streams.Thumb.Reader, streams.Thumb.Writer = io.Pipe()
	streams.CLIP.Reader, streams.CLIP.Writer = io.Pipe()

	// Close CLIP writer if disabled
	if !ap.appConfig.MLConfig.CLIPEnabled {
		streams.CLIP.Writer.Close()
	}

	return streams
}

// closeStreams safely closes all stream writers
func (ap *AssetProcessor) closeStreams(streams *ImageStreams) {
	if streams.EXIF.Writer != nil {
		streams.EXIF.Writer.Close()
	}
	if streams.Thumb.Writer != nil {
		streams.Thumb.Writer.Close()
	}
	if streams.CLIP.Writer != nil {
		streams.CLIP.Writer.Close()
	}
}

// logProcessingErrors logs processing errors with context
func (ap *AssetProcessor) logProcessingErrors(errors []error, context string) {
	for _, err := range errors {
		log.Printf("%s partial failure: %v", context, err)
	}
}
