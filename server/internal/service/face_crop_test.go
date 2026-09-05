package service

import (
	"bytes"
	"image"
	"image/jpeg"
	"math"
	"testing"

	"server/internal/db/dbtypes"
	"server/internal/utils/imaging"

	"github.com/davidbyttow/govips/v2/vips"
)

func TestFaceCropUsesDisplayCoordinates(t *testing.T) {
	imaging.StartVips()
	var b bytes.Buffer
	if err := jpeg.Encode(&b, image.NewRGBA(image.Rect(0, 0, 80, 40)), nil); err != nil {
		t.Fatal(err)
	}
	exif := []byte{'E', 'x', 'i', 'f', 0, 0, 'I', 'I', 42, 0, 8, 0, 0, 0, 1, 0, 0x12, 1, 3, 0, 1, 0, 0, 0, 6, 0, 0, 0, 0, 0, 0, 0}
	segment := append([]byte{0xff, 0xe1, 0, byte(len(exif) + 2)}, exif...)
	src := append(append(append([]byte{}, b.Bytes()[:2]...), segment...), b.Bytes()[2:]...)
	// This face exists in the lower half of the displayed portrait, outside
	// the unrotated landscape's height. No crop can succeed before orientation.
	out, err := encodeFaceCrop(src, &dbtypes.FaceBoundingBox{X1: 10, Y1: 55, X2: 30, Y2: 75})
	if err != nil {
		t.Fatal(err)
	}
	crop, err := vips.NewImageFromBuffer(out)
	if err != nil {
		t.Fatal(err)
	}
	defer crop.Close()
	if _, err := crop.ToGoImage(); err != nil {
		t.Fatal(err)
	}
	if crop.Width() <= 0 || crop.Height() <= 0 || crop.Height() > 80 {
		t.Fatal("invalid crop size")
	}
	for _, bbox := range []*dbtypes.FaceBoundingBox{nil, {X1: float32(math.NaN()), Y1: 0, X2: 10, Y2: 10}, {X1: 5, Y1: 5, X2: 4, Y2: 10}, {X1: 200, Y1: 200, X2: 210, Y2: 210}} {
		if _, err := encodeFaceCrop(src, bbox); err == nil {
			t.Fatalf("invalid bounds accepted: %+v", bbox)
		}
	}
}
