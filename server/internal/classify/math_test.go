package classify

import (
	"math"
	"testing"
)

func almostEqual(a, b, eps float64) bool {
	return math.Abs(a-b) <= eps
}

func TestNormalize(t *testing.T) {
	got := Normalize([]float32{3, 4})
	if !almostEqual(float64(got[0]), 0.6, 1e-6) || !almostEqual(float64(got[1]), 0.8, 1e-6) {
		t.Fatalf("expected unit vector (0.6,0.8), got %v", got)
	}
	if n := math.Sqrt(Dot(got, got)); !almostEqual(n, 1.0, 1e-6) {
		t.Fatalf("expected unit norm, got %f", n)
	}

	zero := Normalize([]float32{0, 0})
	if zero[0] != 0 || zero[1] != 0 {
		t.Fatalf("expected zero vector unchanged, got %v", zero)
	}
}

func TestEnsemblePrototypeIsUnitAndOrderInvariant(t *testing.T) {
	a := [][]float32{{1, 0}, {0, 1}}
	b := [][]float32{{0, 1}, {1, 0}}
	pa, err := EnsemblePrototype(a)
	if err != nil {
		t.Fatal(err)
	}
	pb, err := EnsemblePrototype(b)
	if err != nil {
		t.Fatal(err)
	}
	if n := math.Sqrt(Dot(pa, pa)); !almostEqual(n, 1.0, 1e-6) {
		t.Fatalf("prototype not unit length: %f", n)
	}
	for i := range pa {
		if !almostEqual(float64(pa[i]), float64(pb[i]), 1e-6) {
			t.Fatalf("ensemble not order invariant: %v vs %v", pa, pb)
		}
		if !almostEqual(float64(pa[i]), math.Sqrt2/2, 1e-6) {
			t.Fatalf("unexpected prototype component: %v", pa)
		}
	}
}

func TestEnsemblePrototypeErrors(t *testing.T) {
	if _, err := EnsemblePrototype(nil); err == nil {
		t.Fatal("expected error for empty input")
	}
	if _, err := EnsemblePrototype([][]float32{{1, 0}, {1, 0, 0}}); err == nil {
		t.Fatal("expected dimension mismatch error")
	}
}

func TestContrastiveScore(t *testing.T) {
	asset := []float32{1, 0}
	pos := []float32{1, 0}
	neg := []float32{0, 1}

	if s := ContrastiveScore(asset, pos, nil); !almostEqual(s, 1.0, 1e-6) {
		t.Fatalf("expected positive cosine 1.0, got %f", s)
	}
	if s := ContrastiveScore(asset, pos, neg); !almostEqual(s, 1.0, 1e-6) {
		t.Fatalf("expected contrastive 1.0, got %f", s)
	}
	if s := ContrastiveScore([]float32{0, 1}, pos, neg); !almostEqual(s, -1.0, 1e-6) {
		t.Fatalf("expected contrastive -1.0, got %f", s)
	}
}

func TestScoreToConfidence(t *testing.T) {
	if c := ScoreToConfidence(0.2, 0.2); !almostEqual(c, 0.5, 1e-9) {
		t.Fatalf("expected 0.5 at threshold, got %f", c)
	}
	low := ScoreToConfidence(-1, 0.2)
	high := ScoreToConfidence(1, 0.2)
	if !(low < 0.5 && high > 0.5) {
		t.Fatalf("expected monotonic around threshold, got low=%f high=%f", low, high)
	}
	if low < 0 || high > 1 {
		t.Fatalf("confidence out of [0,1]: low=%f high=%f", low, high)
	}
}
