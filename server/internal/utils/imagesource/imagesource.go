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
	rgb, err := mlRGB(source, purpose)
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

// mlRGB produces the HWC RGB uint8 pixels for an ML purpose. The semantic and
// BioCLIP variants replicate the exact resize/crop semantics and resampling
// kernels of the model contracts (validated by the Lumen tensor conformance
// test) so the SDK tensor fast path can consume the pixels without another
// decode.
func mlRGB(source []byte, purpose Purpose) (*imaging.RGBImage, error) {
	switch purpose {
	case PurposeSemantic:
		// SigLIP: one direct bilinear resize to 224x224 (do_center_crop=false).
		return imaging.DecodeRGBResizeExact(source, 224, 224, imaging.KernelBilinear)
	case PurposeBioClip:
		// BioCLIP follows CLIP preprocessing: bicubic shortest-edge resize,
		// then center crop to 224x224.
		return imaging.DecodeRGBShortestEdgeCenterCrop(source, 224, 224, imaging.KernelBicubic)
	case PurposeOCR, PurposeFace:
		return imaging.ProcessImageRGBBytes(source, imaging.ProcessOptions{
			Width:     1920,
			Height:    1920,
			Quality:   90,
			Format:    vips.ImageTypeWEBP,
			NoProfile: true,
		})
	default:
		return nil, fmt.Errorf("unsupported image purpose: %s", purpose)
	}
}
