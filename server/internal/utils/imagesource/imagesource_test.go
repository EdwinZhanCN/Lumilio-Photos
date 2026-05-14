package imagesource

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"testing"

	_ "golang.org/x/image/webp"

	"server/internal/utils/imaging"
)

func TestProcessMLImageFromReaderCaptionPadsTo448Square(t *testing.T) {
	imaging.StartVips()

	out, err := ProcessMLImageFromReader(bytes.NewReader(synthJPEG(t, 1200, 800)), PurposeCaption)
	if err != nil {
		t.Fatalf("ProcessMLImageFromReader: %v", err)
	}

	img, _, err := image.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("decode output: %v", err)
	}

	bounds := img.Bounds()
	if bounds.Dx() != 448 || bounds.Dy() != 448 {
		t.Fatalf("caption image bounds = %dx%d, want 448x448", bounds.Dx(), bounds.Dy())
	}

	padPixel := color.RGBAModel.Convert(img.At(224, 0)).(color.RGBA)
	if padPixel.R < 120 || padPixel.R > 136 || padPixel.G < 120 || padPixel.G > 136 || padPixel.B < 120 || padPixel.B > 136 {
		t.Fatalf("top padding pixel = [%d %d %d], want near [128 128 128]", padPixel.R, padPixel.G, padPixel.B)
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
