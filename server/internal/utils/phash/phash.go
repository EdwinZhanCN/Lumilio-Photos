package phash

import (
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"

	"github.com/corona10/goimagehash"
	_ "golang.org/x/image/webp"
)

const ModelDCTPHashV1 = "dct-phash-v1"

// ComputeFromReader decodes an image stream and computes the 64-bit DCT pHash.
func ComputeFromReader(r io.Reader) (*goimagehash.ImageHash, error) {
	img, _, err := image.Decode(r)
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}

	hash, err := goimagehash.PerceptionHash(img)
	if err != nil {
		return nil, fmt.Errorf("compute perceptual hash: %w", err)
	}
	return hash, nil
}

// ToVector converts a 64-bit perceptual hash into a 64-element float32 vector
// suitable for pgvector storage and HNSW similarity search.
func ToVector(h *goimagehash.ImageHash) []float32 {
	hashBits := h.GetHash()
	vector := make([]float32, 64)
	for i := range 64 {
		if (hashBits>>i)&1 == 1 {
			vector[i] = 1.0
		} else {
			vector[i] = 0.0
		}
	}
	return vector
}
