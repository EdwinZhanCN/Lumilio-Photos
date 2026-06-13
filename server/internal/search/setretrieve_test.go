package search

import (
	"math/rand"
	"testing"

	"github.com/google/uuid"
)

func TestParseStrictness(t *testing.T) {
	cases := map[string]SetStrictness{
		"loose":  StrictnessLoose,
		"normal": StrictnessNormal,
		"strict": StrictnessStrict,
		"":       StrictnessNormal,
		"bogus":  StrictnessNormal,
		"STRICT": StrictnessNormal, // case-sensitive contract: enums come from tool schema
	}
	for in, want := range cases {
		if got := ParseStrictness(in); got != want {
			t.Errorf("ParseStrictness(%q) = %q, want %q", in, got, want)
		}
	}
	if StrictnessLoose.minSignal() >= StrictnessNormal.minSignal() ||
		StrictnessNormal.minSignal() >= StrictnessStrict.minSignal() {
		t.Error("signal gates must rise with strictness")
	}
	if StrictnessLoose.gapFraction() <= StrictnessNormal.gapFraction() ||
		StrictnessNormal.gapFraction() <= StrictnessStrict.gapFraction() {
		t.Error("gap fractions must shrink with strictness")
	}
}

// background generates a tight, slightly skewed distance sample around the
// given median — the shape CLIP-style text→image distances actually have.
func background(median, spread float64, n int, seed int64) []float64 {
	rng := rand.New(rand.NewSource(seed))
	out := make([]float64, n)
	for i := range out {
		d := median + rng.NormFloat64()*spread
		if rng.Float64() < 0.1 {
			d += spread // light right tail
		}
		out[i] = d
	}
	return out
}

func TestCalibrateBackground(t *testing.T) {
	sample := background(1.2, 0.04, 256, 42)
	stats, ok := calibrateBackground(sample)
	if !ok {
		t.Fatal("healthy sample must calibrate")
	}
	if stats.median < 1.1 || stats.median > 1.3 {
		t.Fatalf("median = %f, want ≈1.2", stats.median)
	}
	if stats.spread <= 0 || stats.spread > 0.15 {
		t.Fatalf("spread = %f, want small positive", stats.spread)
	}

	// Degenerate cases.
	if _, ok := calibrateBackground(make([]float64, 10)); ok {
		t.Error("tiny sample must not calibrate")
	}
	flat := make([]float64, 256)
	for i := range flat {
		flat[i] = 1.0
	}
	if _, ok := calibrateBackground(flat); ok {
		t.Error("zero-spread sample must not calibrate")
	}
}

// The regression that motivated v2: matches sit only ~1.5–3 robust-σ from a
// tight background. The old μ−2.5σ cutoff returned nothing; gap admission
// anchored at the best match must keep the matching cluster.
func TestAdmissionKeepsObviousMatchesInTightDistributions(t *testing.T) {
	stats, ok := calibrateBackground(background(1.2, 0.04, 256, 7))
	if !ok {
		t.Fatal("calibration failed")
	}

	// Best match is ~4 robust-σ better than background median — a clear hit,
	// but close in absolute distance because the distribution is tight.
	dBest := stats.median - 4*stats.spread

	cutoff, signal, hasSignal := admissionCutoff(stats, dBest, StrictnessNormal)
	if !hasSignal {
		t.Fatalf("clear best match rejected: signal=%f", signal)
	}
	if cutoff <= dBest {
		t.Fatalf("cutoff %f must admit at least the best match %f", cutoff, dBest)
	}

	// A sibling match slightly worse than the best must be admitted...
	sibling := dBest + 0.3*(stats.median-dBest)
	if sibling > cutoff {
		t.Fatalf("sibling match %f rejected by cutoff %f", sibling, cutoff)
	}
	// ...while a background-level distance must not.
	if stats.median <= cutoff {
		t.Fatalf("background median %f admitted by cutoff %f", stats.median, cutoff)
	}
}

// Nonsense queries: the best "match" is just the background tail — the
// signal gate must yield the empty set.
func TestAdmissionRejectsNoSignalQueries(t *testing.T) {
	stats, ok := calibrateBackground(background(1.25, 0.05, 256, 11))
	if !ok {
		t.Fatal("calibration failed")
	}

	dBest := stats.median - 1.0*stats.spread // indistinguishable from background
	_, signal, hasSignal := admissionCutoff(stats, dBest, StrictnessNormal)
	if hasSignal {
		t.Fatalf("no-signal query admitted: signal=%f", signal)
	}

	// loose is more forgiving than strict on the same gap.
	borderline := stats.median - 1.8*stats.spread
	_, _, looseOK := admissionCutoff(stats, borderline, StrictnessLoose)
	_, _, strictOK := admissionCutoff(stats, borderline, StrictnessStrict)
	if !looseOK || strictOK {
		t.Fatalf("gate ordering wrong: loose=%v strict=%v", looseOK, strictOK)
	}
}

// Stricter settings must admit a subset of what looser settings admit.
func TestCutoffMonotonicInStrictness(t *testing.T) {
	stats, ok := calibrateBackground(background(1.2, 0.04, 256, 21))
	if !ok {
		t.Fatal("calibration failed")
	}
	dBest := stats.median - 5*stats.spread

	looseCutoff, _, _ := admissionCutoff(stats, dBest, StrictnessLoose)
	normalCutoff, _, _ := admissionCutoff(stats, dBest, StrictnessNormal)
	strictCutoff, _, _ := admissionCutoff(stats, dBest, StrictnessStrict)
	if !(strictCutoff < normalCutoff && normalCutoff < looseCutoff) {
		t.Fatalf("cutoffs not monotonic: strict=%f normal=%f loose=%f",
			strictCutoff, normalCutoff, looseCutoff)
	}
}

func TestFilterWithinCutoffPreservesOrder(t *testing.T) {
	candidates := []Candidate{
		{AssetID: uuid.New(), Rank: 1, RawScore: 0.4},
		{AssetID: uuid.New(), Rank: 2, RawScore: 0.7},
		{AssetID: uuid.New(), Rank: 3, RawScore: 0.9},
		{AssetID: uuid.New(), Rank: 4, RawScore: 1.3},
	}
	kept := filterWithinCutoff(candidates, 0.9)
	if len(kept) != 3 {
		t.Fatalf("kept %d, want 3", len(kept))
	}
	for i := 1; i < len(kept); i++ {
		if kept[i].RawScore < kept[i-1].RawScore {
			t.Fatal("relevance order broken")
		}
	}
}
