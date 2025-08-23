package processors

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/utils/exif"
	"server/internal/utils/imaging"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
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
	extractor := exif.NewExtractor(nil)

	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	exifR, exifW := io.Pipe()
	clipR, clipW := io.Pipe()
	thumbR, thumbW := io.Pipe()
	defer exifR.Close()
	defer clipR.Close()
	defer thumbR.Close()

	clipEnabled := false
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
		n, err := io.Copy(mw, fileReader)
		if err != nil {
			log.Printf("❌ Broadcast error: %v", err)
		} else {
			log.Printf("✅ Successfully broadcast %d bytes to all pipes", n)
		}
	}()

	g, ctx := errgroup.WithContext(timeoutCtx)

	go func() {
		<-timeoutCtx.Done()
		// Closing writers will unblock io.Copy and downstream readers
		exifW.Close()
		thumbW.Close()
		clipW.Close()
	}()

	g.Go(func() error {
		req := &exif.StreamingExtractRequest{
			Reader:    exifR,
			AssetType: dbtypes.AssetType(asset.Type),
			Filename:  asset.OriginalFilename,
			Size:      asset.FileSize,
		}
		exifResult, err := extractor.ExtractFromStream(ctx, req)
		if err != nil {
			return fmt.Errorf("extract exif: %w", err)
		}
		if meta, ok := exifResult.Metadata.(*dbtypes.PhotoSpecificMetadata); ok {
			if err := asset.SetPhotoMetadata(meta); err != nil {
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
			}
			if err := ap.assetService.SaveNewThumbnail(ctx, buf, asset, name); err != nil {
				return fmt.Errorf("save thumb %s: %w", name, err)
			}
		}
		return nil
	})

	if clipEnabled {
		g.Go(func() error {
			// tinyOpts := bimg.Options{
			// 	Width:     224,
			// 	Height:    224,
			// 	Crop:      true,
			// 	Gravity:   bimg.GravitySmart,
			// 	Quality:   90,
			// 	Type:      bimg.WEBP,
			// 	NoProfile: true,
			// }
			// tiny, err := imaging.ProcessImageStream(clipR, tinyOpts)

			// if err != nil {
			// 	return fmt.Errorf("process CLIP thumbnail: %w", err)
			// }

			// payload := CLIPPayload{
			// 	asset.AssetID,
			// 	tiny,
			// }
			// ap.clipQueue.Enqueue(ctx, string(queue.JobCLIPProcess), payload)
			_, _ = io.Copy(io.Discard, clipR)
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return err
	}

	return nil
}
