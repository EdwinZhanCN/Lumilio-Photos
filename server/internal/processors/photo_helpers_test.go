package processors

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"server/internal/utils/imaging"
	"testing"
)

func TestGenerateThumbnailsKeepsAllSizes(t *testing.T) {
	imaging.StartVips()

	outputs, err := thumbnailBuffers(bytes.NewReader(testJPEG(t)))
	if err != nil {
		t.Fatalf("thumbnailBuffers: %v", err)
	}
	for _, size := range []string{"small", "medium", "large"} {
		if len(outputs[size]) == 0 {
			t.Fatalf("expected %s thumbnail codec output", size)
		}
	}
}

func testJPEG(t *testing.T) []byte {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, 640, 480))
	for y := 0; y < 480; y++ {
		for x := 0; x < 640; x++ {
			img.Set(x, y, color.RGBA{
				R: uint8(x % 256),
				G: uint8(y % 256),
				B: uint8((x + y) % 256),
				A: 255,
			})
		}
	}

	var jpegBuf bytes.Buffer
	if err := jpeg.Encode(&jpegBuf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}
	return jpegBuf.Bytes()
}

func stringPtr(s string) *string {
	return &s
}
