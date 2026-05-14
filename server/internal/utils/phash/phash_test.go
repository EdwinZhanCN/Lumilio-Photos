package phash

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"testing"

	"server/internal/utils/imaging"
)

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
		t.Fatalf("encode synth jpeg: %v", err)
	}
	return buf.Bytes()
}

func TestComputeFromReaderDecodesWebPThumbnail(t *testing.T) {
	imaging.StartVips()

	var small bytes.Buffer
	writers := map[string]io.Writer{"small": &small}
	if err := imaging.StreamThumbnails(bytes.NewReader(synthJPEG(t, 640, 480)), map[string][2]int{
		"small": {400, 400},
	}, writers); err != nil {
		t.Fatalf("create webp thumbnail: %v", err)
	}

	hash, err := ComputeFromReader(bytes.NewReader(small.Bytes()))
	if err != nil {
		t.Fatalf("ComputeFromReader: %v", err)
	}

	vector := ToVector(hash)
	if len(vector) != 64 {
		t.Fatalf("vector length = %d, want 64", len(vector))
	}
	for i, v := range vector {
		if v != 0 && v != 1 {
			t.Fatalf("vector[%d] = %v, want 0 or 1", i, v)
		}
	}
}
