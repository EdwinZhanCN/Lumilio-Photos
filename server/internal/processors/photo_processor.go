package processors

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"server/internal/models"
	"server/internal/utils/exif"
	"server/internal/utils/imaging"
	pb "server/proto"

	"golang.org/x/sync/errgroup"
	"github.com/h2non/bimg"
)

var thumbnailSizes = map[string][2]int{
	"small":  {400, 400},
	"medium": {800, 800},
	"large":  {1920, 1920},
}

type CLIPPayload struct {
	AssetID   uuid.UUID
	ImageData []byte
}

func (ap *AssetProcessor) processPhotoAsset(
	ctx context.Context,
	asset *models.Asset,
	fileReader io.Reader,
) error {
	extractor := exif.NewExtractor(nil)

	exifR, exifW := io.Pipe()
	clipR, clipW := io.Pipe()
	thumbR, thumbW := io.Pipe()
	defer exifR.Close()
	defer clipR.Close()
	defer thumbR.Close()

	go func() {
		defer exifW.Close()
		defer clipW.Close()
		defer thumbW.Close()

		mw := io.MultiWriter(exifW, clipW, thumbW)
		if _, err := io.Copy(mw, fileReader); err != nil {
			log.Printf("broadcast error: %v", err)
		}
	}()

	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		req := &exif.StreamingExtractRequest{
			Reader:    exifR,
			AssetType: asset.Type,
			Filename:  asset.OriginalFilename,
			Size:      asset.FileSize,
		}
		exifResult, err := extractor.ExtractFromStream(ctx, req)
		if err != nil {
			return fmt.Errorf("extract exif: %w", err)
		}
		if meta, ok := exifResult.Metadata.(*models.PhotoSpecificMetadata); ok {
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
			if err := ap.assetService.SaveNewThumbnail(ctx, buf, asset, name); err != nil {
				return fmt.Errorf("save thumb %s: %w", name, err)
			}
		}
		return nil
	})

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

		// TODO: enqueue CLIP queue to let another CLIP woker to call CLIP Process Accrodingly
		response, err := ap.mlService.ProcessImageForCLIP(&pb.ImageProcessRequest{
			ImageId:   asset.AssetID.String(),
			ImageData: tiny,
		})

		if err != nil {
			return fmt.Errorf("ProcessImageForCLIP: %w", err)
		}

		if embedResponse == nil {
			return fmt.Errorf("CLIP response is nil")
		}

		if embedResponse.ImageFeatureVector == nil || len(embedResponse.ImageFeatureVector) == 0 {
			return fmt.Errorf("CLIP image feature vector is empty")
		}

		// 只有当上面的错误检查都通过后，才保存 CLIP embedding
		if err := ap.assetService.SaveNewEmbedding(ctx, asset.AssetID, embedResponse.ImageFeatureVector); err != nil {
			return fmt.Errorf("save CLIP embedding: %w", err)
		}

		return nil
	})

	if err := g.Wait(); err != nil {
		return err
	}

	return nil
}
