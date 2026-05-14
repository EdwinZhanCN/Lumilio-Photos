package service

import (
	"testing"

	"server/internal/utils/imagesource"
)

func TestNewRGBTensorInferRequestAddsTensorMetadata(t *testing.T) {
	image := &imagesource.MLImage{
		Data:       make([]byte, 224*224*3),
		Width:      224,
		Height:     224,
		Channels:   3,
		Layout:     "HWC",
		DType:      "uint8",
		ColorSpace: "RGB",
	}

	req, err := newRGBTensorInferRequest("clip_image_embed", image)
	if err != nil {
		t.Fatalf("newRGBTensorInferRequest: %v", err)
	}

	if req.PayloadMime != rgbTensorPayloadMime {
		t.Fatalf("payload mime = %q, want %q", req.PayloadMime, rgbTensorPayloadMime)
	}
	if len(req.Payload) != len(image.Data) {
		t.Fatalf("payload len = %d, want %d", len(req.Payload), len(image.Data))
	}
	if req.Meta["tensor_width"] != "224" || req.Meta["tensor_height"] != "224" || req.Meta["tensor_channels"] != "3" {
		t.Fatalf("unexpected tensor shape metadata: %#v", req.Meta)
	}
	if req.Meta["tensor_layout"] != "HWC" || req.Meta["tensor_dtype"] != "uint8" || req.Meta["tensor_color_space"] != "RGB" {
		t.Fatalf("unexpected tensor format metadata: %#v", req.Meta)
	}
}

func TestNewRGBTensorInferRequestRejectsShapeMismatch(t *testing.T) {
	image := &imagesource.MLImage{
		Data:     make([]byte, 10),
		Width:    2,
		Height:   2,
		Channels: 3,
	}

	if _, err := newRGBTensorInferRequest("clip_image_embed", image); err == nil {
		t.Fatal("expected shape mismatch error")
	}
}
