package imagesource

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"testing"

	"server/internal/utils/imaging"
)

func TestProcessMLImageTensorFromReaderCaptionPadsTo448Square(t *testing.T) {
	imaging.StartVips()

	out, err := ProcessMLImageTensorFromReader(bytes.NewReader(synthJPEG(t, 1200, 800)), PurposeCaption)
	if err != nil {
		t.Fatalf("ProcessMLImageTensorFromReader: %v", err)
	}

	if out.Width != 448 || out.Height != 448 || out.Channels != 3 {
		t.Fatalf("caption tensor shape = %dx%dx%d, want 448x448x3", out.Width, out.Height, out.Channels)
	}
	if out.Layout != "HWC" || out.DType != "uint8" || out.ColorSpace != "RGB" {
		t.Fatalf("caption tensor metadata = %s/%s/%s, want HWC/uint8/RGB", out.Layout, out.DType, out.ColorSpace)
	}
	if len(out.Data) != 448*448*3 {
		t.Fatalf("caption tensor len = %d, want %d", len(out.Data), 448*448*3)
	}

	padOffset := (224 * 3)
	padPixel := out.Data[padOffset : padOffset+3]
	if padPixel[0] != 128 || padPixel[1] != 128 || padPixel[2] != 128 {
		t.Fatalf("top padding pixel = [%d %d %d], want [128 128 128]", padPixel[0], padPixel[1], padPixel[2])
	}
}

func TestProcessMLImageTensorFromReaderClipReturns224RGB(t *testing.T) {
	imaging.StartVips()

	out, err := ProcessMLImageTensorFromReader(bytes.NewReader(synthJPEG(t, 1200, 800)), PurposeClip)
	if err != nil {
		t.Fatalf("ProcessMLImageTensorFromReader: %v", err)
	}

	if out.Width != 224 || out.Height != 224 || out.Channels != 3 {
		t.Fatalf("clip tensor shape = %dx%dx%d, want 224x224x3", out.Width, out.Height, out.Channels)
	}
	if len(out.Data) != 224*224*3 {
		t.Fatalf("clip tensor len = %d, want %d", len(out.Data), 224*224*3)
	}
}

func synthJPEG(t *testing.T, w, h int) []byte {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{
				R: uint8(x % 256),
				G: uint8(y % 256),
				B: uint8((x + y) % 256),
				A: 255,
			})
		}
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}
	return buf.Bytes()
}
