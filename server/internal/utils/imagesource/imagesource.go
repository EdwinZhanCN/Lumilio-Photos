package imagesource

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/davidbyttow/govips/v2/vips"

	"server/internal/utils/imaging"
	"server/internal/utils/raw"
)

type Purpose string

const (
	PurposeSemantic Purpose = "semantic"
	PurposeBioClip  Purpose = "bioclip"
	PurposeOCR      Purpose = "ocr"
	PurposeFace     Purpose = "face"
)

// MLImage is the server-side image tensor payload handed to ML workers. Data is
// HWC RGB uint8; EncodedSource keeps the processed source container around for
// call sites that still need a decodable image, such as face crop persistence.
type MLImage struct {
	Data          []byte
	EncodedSource []byte
	Width         int
	Height        int
	Channels      int
	Layout        string
	DType         string
	ColorSpace    string
}

func openRAWPhoto(ctx context.Context, fullPath string, originalFilename string) (io.ReadCloser, error) {
	opts := raw.DefaultProcessingOptions()
	opts.FullRenderTimeout = 30 * time.Second
	opts.PreferEmbedded = true
	opts.Quality = 90

	result, err := raw.NewProcessor(opts).ProcessRAWFromPath(ctx, fullPath, originalFilename)
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

// OpenPhoto returns a decodable image source for a photo. RAW files are resolved
// to their embedded preview first, falling back to the RAW processor's full render.
func OpenPhoto(ctx context.Context, fullPath string, originalFilename string) (io.ReadCloser, error) {
	if raw.IsRAWFile(originalFilename) {
		return openRAWPhoto(ctx, fullPath, originalFilename)
	}

	f, err := os.Open(fullPath)
	if err != nil {
		return nil, fmt.Errorf("open photo: %w", err)
	}
	return f, nil
}

func ProcessMLImage(ctx context.Context, fullPath string, originalFilename string, purpose Purpose) (*MLImage, error) {
	reader, err := OpenPhoto(ctx, fullPath, originalFilename)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	return ProcessMLImageFromReader(reader, purpose)
}

func ProcessMLImageFromReader(reader io.Reader, purpose Purpose) (*MLImage, error) {
	return ProcessMLImageTensorFromReader(reader, purpose)
}

func ProcessMLImageTensorFromReader(reader io.Reader, purpose Purpose) (*MLImage, error) {
	source, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("read ml image source: %w", err)
	}
	return ProcessMLImageTensorBytes(source, purpose)
}

func ProcessMLImageTensorBytes(source []byte, purpose Purpose) (*MLImage, error) {
	opts, err := mlOptions(purpose)
	if err != nil {
		return nil, err
	}

	rgb, err := imaging.ProcessImageRGBBytes(source, opts)
	if err != nil {
		return nil, err
	}

	return &MLImage{
		Data:          rgb.Data,
		EncodedSource: append([]byte(nil), source...),
		Width:         rgb.Width,
		Height:        rgb.Height,
		Channels:      rgb.Channels,
		Layout:        rgb.Layout,
		DType:         rgb.DType,
		ColorSpace:    rgb.ColorSpace,
	}, nil
}

func mlOptions(purpose Purpose) (imaging.ProcessOptions, error) {
	switch purpose {
	case PurposeSemantic, PurposeBioClip:
		return imaging.ProcessOptions{
			Width:     224,
			Height:    224,
			Crop:      true,
			Quality:   90,
			Format:    vips.ImageTypeWEBP,
			NoProfile: true,
		}, nil
	case PurposeOCR, PurposeFace:
		return imaging.ProcessOptions{
			Width:     1920,
			Height:    1920,
			Quality:   90,
			Format:    vips.ImageTypeWEBP,
			NoProfile: true,
		}, nil
	default:
		return imaging.ProcessOptions{}, fmt.Errorf("unsupported image purpose: %s", purpose)
	}
}
