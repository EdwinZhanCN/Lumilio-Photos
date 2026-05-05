package imagesource

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/h2non/bimg"

	"server/internal/utils/imaging"
	"server/internal/utils/raw"
)

type Purpose string

const (
	PurposeClip    Purpose = "clip"
	PurposeBioClip Purpose = "bioclip"
	PurposeOCR     Purpose = "ocr"
	PurposeCaption Purpose = "caption"
	PurposeFace    Purpose = "face"
)

// OpenPhoto returns a decodable image source for a photo. RAW files are resolved
// to their embedded preview first, falling back to the RAW processor's full render.
func OpenPhoto(ctx context.Context, fullPath string, originalFilename string) (io.ReadCloser, error) {
	if !raw.IsRAWFile(originalFilename) {
		f, err := os.Open(fullPath)
		if err != nil {
			return nil, fmt.Errorf("open photo: %w", err)
		}
		return f, nil
	}

	f, err := os.Open(fullPath)
	if err != nil {
		return nil, fmt.Errorf("open RAW file: %w", err)
	}
	defer f.Close()

	opts := raw.DefaultProcessingOptions()
	opts.FullRenderTimeout = 30 * time.Second
	opts.PreferEmbedded = true
	opts.Quality = 90

	result, err := raw.NewProcessor(opts).ProcessRAW(ctx, f, originalFilename)
	if err != nil {
		return nil, fmt.Errorf("process RAW: %w", err)
	}
	if !result.IsRAW {
		return nil, fmt.Errorf("file not detected as RAW")
	}
	if len(result.PreviewData) == 0 {
		return nil, fmt.Errorf("no preview data generated")
	}

	return io.NopCloser(bytes.NewReader(result.PreviewData)), nil
}

func ProcessMLImage(ctx context.Context, fullPath string, originalFilename string, purpose Purpose) ([]byte, error) {
	reader, err := OpenPhoto(ctx, fullPath, originalFilename)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	opts, err := mlOptions(purpose)
	if err != nil {
		return nil, err
	}

	return imaging.ProcessImageStream(reader, opts)
}

func mlOptions(purpose Purpose) (bimg.Options, error) {
	switch purpose {
	case PurposeClip, PurposeBioClip:
		return bimg.Options{
			Width:     224,
			Height:    224,
			Crop:      true,
			Gravity:   bimg.GravitySmart,
			Quality:   90,
			Type:      bimg.WEBP,
			NoProfile: true,
		}, nil
	case PurposeOCR, PurposeFace:
		return bimg.Options{
			Width:     1920,
			Height:    1920,
			Quality:   90,
			Type:      bimg.WEBP,
			NoProfile: true,
		}, nil
	case PurposeCaption:
		return bimg.Options{
			Width:     1024,
			Height:    1024,
			Quality:   90,
			Type:      bimg.WEBP,
			NoProfile: true,
		}, nil
	default:
		return bimg.Options{}, fmt.Errorf("unsupported image purpose: %s", purpose)
	}
}
