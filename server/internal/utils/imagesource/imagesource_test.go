package imagesource

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"

	"server/internal/utils/imaging"
)

func TestProcessMLImageTensorFromReaderSemanticReturns224RGB(t *testing.T) {
	imaging.StartVips()

	out, err := ProcessMLImageTensorFromReader(bytes.NewReader(synthJPEG(t, 1200, 800)), PurposeSemantic)
	if err != nil {
		t.Fatalf("ProcessMLImageTensorFromReader: %v", err)
	}

	if out.Width != 224 || out.Height != 224 || out.Channels != 3 {
		t.Fatalf("semantic tensor shape = %dx%dx%d, want 224x224x3", out.Width, out.Height, out.Channels)
	}
	if len(out.Data) != 224*224*3 {
		t.Fatalf("semantic tensor len = %d, want %d", len(out.Data), 224*224*3)
	}
}

func TestPrepareSemanticThumbnailReturnsMediumWebP(t *testing.T) {
	imaging.StartVips()

	jpeg := synthJPEG(t, 1200, 800)
	thumb, err := PrepareSemanticThumbnailBytes(context.Background(), jpeg, "query.jpg")
	if err != nil {
		t.Fatalf("PrepareSemanticThumbnailBytes: %v", err)
	}
	if len(thumb) < 12 || string(thumb[:4]) != "RIFF" || string(thumb[8:12]) != "WEBP" {
		t.Fatalf("thumbnail is not WebP")
	}

	path := filepath.Join(t.TempDir(), "query.jpg")
	if err := os.WriteFile(path, jpeg, 0o600); err != nil {
		t.Fatal(err)
	}
	fromPath, err := PrepareSemanticThumbnail(context.Background(), path, "query.jpg")
	if err != nil {
		t.Fatalf("PrepareSemanticThumbnail: %v", err)
	}
	if len(fromPath) < 12 || string(fromPath[:4]) != "RIFF" {
		t.Fatalf("path thumbnail is not WebP")
	}

	if _, err := PrepareSemanticThumbnailBytes(context.Background(), []byte("not-an-image"), "query.jpg"); err == nil {
		t.Fatal("expected invalid image to fail")
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
