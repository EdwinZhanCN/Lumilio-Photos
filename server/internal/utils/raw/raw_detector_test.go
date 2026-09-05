package raw

import (
	"bytes"
	"image"
	"image/jpeg"
	"testing"

	"server/internal/utils/imaging"
)

func TestPreviewValidationAcceptsDecodableTrailingData(t *testing.T) {
	imaging.StartVips()
	var b bytes.Buffer
	if err := jpeg.Encode(&b, image.NewRGBA(image.Rect(0, 0, 80, 40)), nil); err != nil {
		t.Fatal(err)
	}
	src := append(b.Bytes(), make([]byte, 8192)...)
	ok, err := (&Detector{}).IsPreviewAcceptable(src, 40, 20)
	if err != nil || !ok {
		t.Fatalf("decodable preview rejected: %v %v", ok, err)
	}
}

func TestPreviewValidationRejectsTruncatedPixels(t *testing.T) {
	imaging.StartVips()
	var b bytes.Buffer
	if err := jpeg.Encode(&b, image.NewRGBA(image.Rect(0, 0, 800, 400)), nil); err != nil {
		t.Fatal(err)
	}
	// Keep a decodable header, truncate the entropy stream, then append EOI.
	src := append(append([]byte{}, b.Bytes()[:len(b.Bytes())/2]...), 0xff, 0xd9)
	ok, err := (&Detector{}).IsPreviewAcceptable(src, 40, 20)
	if err == nil && ok {
		t.Fatal("truncated pixel data accepted")
	}
}

func TestPreviewNormalizationHonorsEncoderContract(t *testing.T) {
	imaging.StartVips()
	var b bytes.Buffer
	if err := jpeg.Encode(&b, image.NewRGBA(image.Rect(0, 0, 80, 40)), nil); err != nil {
		t.Fatal(err)
	}
	p := NewProcessor(ProcessingOptions{Quality: 100})
	out, err := p.normalizeEmbeddedPreview(b.Bytes(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) < 12 || string(out[8:12]) != "WEBP" {
		t.Fatal("unset output format bypassed the default WebP encoder")
	}
}

func TestGenerateThumbnailsReportsFailure(t *testing.T) {
	imaging.StartVips()
	p := NewProcessor(DefaultProcessingOptions())
	if _, err := p.GenerateThumbnails([]byte("not an image"), map[string][2]int{"small": {40, 40}}); err == nil {
		t.Fatal("invalid source reported successful thumbnails")
	}
	var b bytes.Buffer
	if err := jpeg.Encode(&b, image.NewRGBA(image.Rect(0, 0, 80, 40)), nil); err != nil {
		t.Fatal(err)
	}
	if _, err := p.GenerateThumbnails(b.Bytes(), map[string][2]int{"invalid": {0, 0}}); err == nil {
		t.Fatal("zero thumbnail bounds accepted")
	}
}
