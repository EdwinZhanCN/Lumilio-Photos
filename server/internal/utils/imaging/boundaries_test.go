package imaging

import (
	"bytes"
	"testing"

	"github.com/davidbyttow/govips/v2/vips"
)

// EXIF orientation 6 on a non-square JPEG, without a private fixture.
func rotatedJPEG(t *testing.T) []byte {
	src := synthJPEG(t, 80, 40)
	exif := []byte{'E', 'x', 'i', 'f', 0, 0, 'I', 'I', 42, 0, 8, 0, 0, 0, 1, 0, 0x12, 1, 3, 0, 1, 0, 0, 0, 6, 0, 0, 0, 0, 0, 0, 0}
	segment := append([]byte{0xff, 0xe1, 0, byte(len(exif) + 2)}, exif...)
	return append(append(append([]byte{}, src[:2]...), segment...), src[2:]...)
}

func TestMLDecodeUsesDisplayOrientation(t *testing.T) {
	StartVips()
	img, err := decodeForML(rotatedJPEG(t))
	if err != nil {
		t.Fatal(err)
	}
	defer img.Close()
	if img.Width() != 40 || img.Height() != 80 {
		t.Fatalf("dimensions %dx%d, want 40x80", img.Width(), img.Height())
	}
}

func TestImagingRejectsInvalidBounds(t *testing.T) {
	StartVips()
	src := synthJPEG(t, 80, 40)
	if _, err := ProcessImageBytes(src, ProcessOptions{Width: -1, Height: 40}); err == nil {
		t.Error("negative width accepted")
	}
	if _, err := runStreamThumbnails(src, map[string][2]int{"bad": {0, 40}}); err == nil {
		t.Error("zero thumbnail width accepted")
	}
	if _, err := DecodeRGBResizeExact(src, 0, 40, KernelBilinear); err == nil {
		t.Error("zero ML width accepted")
	}
}

func TestThumbnailAndExportOrientation(t *testing.T) {
	StartVips()
	src := rotatedJPEG(t)
	thumbnails, err := runStreamThumbnails(src, map[string][2]int{"small": {20, 40}})
	if err != nil {
		t.Fatal(err)
	}
	exported, _, _, err := ExportImageBytes(src, ExportParams{Format: "png"})
	if err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string][]byte{"thumbnail": thumbnails["small"], "export": exported} {
		img, err := vips.NewImageFromBuffer(data)
		if err != nil {
			t.Fatal(err)
		}
		if img.Height() != 2*img.Width() {
			t.Errorf("%s orientation: %dx%d", name, img.Width(), img.Height())
		}
		img.Close()
	}
	if bytes.Equal(src, exported) {
		t.Fatal("source returned without encoding")
	}
}

func TestMLRectangularCenterCrop(t *testing.T) {
	StartVips()
	for _, dim := range [][2]int{{40, 80}, {80, 40}, {40, 40}} {
		rgb, err := DecodeRGBShortestEdgeCenterCrop(synthJPEG(t, 80, 40), dim[0], dim[1], KernelBicubic)
		if err != nil {
			t.Fatal(err)
		}
		if rgb.Width != dim[0] || rgb.Height != dim[1] || len(rgb.Data) != dim[0]*dim[1]*3 {
			t.Fatalf("invalid crop: %+v", rgb)
		}
	}
}

func TestExportOrientsMetadataBeyondJPEG(t *testing.T) {
	StartVips()
	source, err := vips.NewImageFromBuffer(rotatedJPEG(t))
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	webp, _, err := source.ExportWebp(vips.NewWebpExportParams())
	if err != nil {
		t.Fatal(err)
	}
	png, _, err := source.ExportPng(vips.NewPngExportParams())
	if err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string][]byte{"webp": webp, "png": png} {
		t.Run(name, func(t *testing.T) {
			out, _, _, err := ExportImageBytes(data, ExportParams{Format: "png"})
			if err != nil {
				t.Fatal(err)
			}
			img, err := vips.NewImageFromBuffer(out)
			if err != nil {
				t.Fatal(err)
			}
			defer img.Close()
			if img.Width() != 40 || img.Height() != 80 {
				t.Fatalf("orientation ignored: %dx%d", img.Width(), img.Height())
			}
		})
	}
}
