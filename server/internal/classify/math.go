// Package classify holds the pure, dependency-free math for zero-shot
// classification: prompt-ensemble prototypes, contrastive scoring against
// L2-normalized embeddings, and confidence calibration. Keeping it free of DB,
// ML, and imaging (libvips) dependencies makes it cheap to unit-test and safe to
// import from both the service and queue layers.
package classify

import (
	"fmt"
	"math"
)

// Normalize returns a unit-length copy of v. A zero (or near-zero) vector is
// returned unchanged so callers can detect/skip it.
func Normalize(v []float32) []float32 {
	var sum float64
	for _, x := range v {
		sum += float64(x) * float64(x)
	}
	norm := math.Sqrt(sum)
	out := make([]float32, len(v))
	if norm == 0 {
		copy(out, v)
		return out
	}
	for i, x := range v {
		out[i] = float32(float64(x) / norm)
	}
	return out
}

// MeanPool averages a set of equal-length vectors into a single vector.
func MeanPool(vectors [][]float32) ([]float32, error) {
	if len(vectors) == 0 {
		return nil, fmt.Errorf("meanpool: no vectors")
	}
	dim := len(vectors[0])
	if dim == 0 {
		return nil, fmt.Errorf("meanpool: empty vector")
	}
	acc := make([]float64, dim)
	for _, v := range vectors {
		if len(v) != dim {
			return nil, fmt.Errorf("meanpool: dimension mismatch (%d != %d)", len(v), dim)
		}
		for i, x := range v {
			acc[i] += float64(x)
		}
	}
	out := make([]float32, dim)
	for i, x := range acc {
		out[i] = float32(x / float64(len(vectors)))
	}
	return out, nil
}

// EnsemblePrototype builds a prompt-ensemble prototype: L2-normalize each input
// vector, mean-pool them, then L2-normalize the result. This is the standard
// zero-shot prompt-ensembling recipe and yields a unit vector so that the dot
// product against a (unit) image embedding equals cosine similarity.
func EnsemblePrototype(vectors [][]float32) ([]float32, error) {
	normalized := make([][]float32, 0, len(vectors))
	for _, v := range vectors {
		normalized = append(normalized, Normalize(v))
	}
	mean, err := MeanPool(normalized)
	if err != nil {
		return nil, err
	}
	return Normalize(mean), nil
}

// Dot computes the dot product of two equal-length vectors. For unit vectors
// this is the cosine similarity. Mismatched lengths are guarded by truncating
// to the shorter slice; callers should validate dimensions first.
func Dot(a, b []float32) float64 {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	var sum float64
	for i := 0; i < n; i++ {
		sum += float64(a[i]) * float64(b[i])
	}
	return sum
}

// ContrastiveScore is the zero-shot binary decision: cos(asset, positive) −
// cos(asset, negative), i.e. argmax over {positive, background}. All inputs are
// unit vectors. A nil/empty negative degrades to the plain positive cosine.
// score > 0 means the positive prompt beats the background.
func ContrastiveScore(assetVec, positive, negative []float32) float64 {
	score := Dot(assetVec, positive)
	if len(negative) > 0 {
		score -= Dot(assetVec, negative)
	}
	return score
}

// ConfidenceGain shapes how sharply the contrastive margin maps to [0,1].
const ConfidenceGain = 12.0

// ScoreToConfidence maps a contrastive margin to a [0,1] confidence via a
// logistic centered on the threshold (exactly 0.5 at the threshold).
func ScoreToConfidence(score, threshold float64) float64 {
	c := 1.0 / (1.0 + math.Exp(-ConfidenceGain*(score-threshold)))
	if math.IsNaN(c) || c < 0 {
		return 0
	}
	if c > 1 {
		return 1
	}
	return c
}
