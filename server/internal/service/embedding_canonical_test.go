package service

import (
	"math"
	"testing"
)

func l2Norm(vec []float32) float64 {
	var sumSq float64
	for _, v := range vec {
		sumSq += float64(v) * float64(v)
	}
	return math.Sqrt(sumSq)
}

func TestCanonicalizeSemanticVectorTruncatesAndNormalizes(t *testing.T) {
	raw := make([]float32, CanonicalEmbeddingDim+384) // e.g. so400m 1152 -> 768
	for i := range raw {
		raw[i] = float32(i%7) + 1 // nonzero, non-unit
	}

	got := canonicalizeSemanticVector(raw)

	if len(got) != CanonicalEmbeddingDim {
		t.Fatalf("expected truncation to %d dims, got %d", CanonicalEmbeddingDim, len(got))
	}
	if norm := l2Norm(got); math.Abs(norm-1.0) > 1e-5 {
		t.Fatalf("expected unit norm, got %v", norm)
	}
	// Truncation must keep the leading prefix (MRL ordering), not a suffix.
	for i := 0; i < CanonicalEmbeddingDim; i++ {
		if raw[i] == 0 {
			continue
		}
		// Direction of the prefix is preserved up to the positive scale factor.
		if (got[i] < 0) != (raw[i] < 0) {
			t.Fatalf("sign flipped at %d: raw=%v got=%v", i, raw[i], got[i])
		}
	}
}

func TestCanonicalizeSemanticVectorShorterThanCanonical(t *testing.T) {
	raw := []float32{3, 4} // norm 5, shorter than canonical dim
	got := canonicalizeSemanticVector(raw)

	if len(got) != len(raw) {
		t.Fatalf("expected no truncation for short vector, got len %d", len(got))
	}
	if norm := l2Norm(got); math.Abs(norm-1.0) > 1e-6 {
		t.Fatalf("expected unit norm, got %v", norm)
	}
	if math.Abs(float64(got[0])-0.6) > 1e-6 || math.Abs(float64(got[1])-0.8) > 1e-6 {
		t.Fatalf("unexpected normalized values: %v", got)
	}
}

func TestCanonicalizeSemanticVectorZeroVectorIsSafe(t *testing.T) {
	raw := make([]float32, 8)
	got := canonicalizeSemanticVector(raw)
	if len(got) != len(raw) {
		t.Fatalf("expected same length, got %d", len(got))
	}
	for i, v := range got {
		if v != 0 {
			t.Fatalf("expected zero at %d, got %v", i, v)
		}
	}
}

func TestCanonicalizeSemanticVectorDoesNotMutateInput(t *testing.T) {
	raw := []float32{3, 4}
	_ = canonicalizeSemanticVector(raw)
	if raw[0] != 3 || raw[1] != 4 {
		t.Fatalf("input mutated: %v", raw)
	}
}
