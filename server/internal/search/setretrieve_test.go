package search

import (
	"math"
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
	// The cosine floor must rise with strictness (more precision).
	if !(StrictnessLoose.cosFloor() < StrictnessNormal.cosFloor() && StrictnessNormal.cosFloor() < StrictnessStrict.cosFloor()) {
		t.Errorf("cosFloor must rise with strictness: loose=%f normal=%f strict=%f",
			StrictnessLoose.cosFloor(), StrictnessNormal.cosFloor(), StrictnessStrict.cosFloor())
	}
}

// cutoffFor mirrors the RetrieveSet conversion cos floor -> L2 distance cutoff.
func cutoffFor(s SetStrictness) float64 {
	return math.Sqrt(math.Max(0, 2*(1-s.cosFloor())))
}

func TestCosFloorSeparatesObservedScale(t *testing.T) {
	// Observed siglip2-base cosines: present matches ≈0.126–0.150 sit clearly
	// above the normal floor, clearly-absent queries ≈0.043–0.064 clearly below.
	// (Near-miss ~0.091, e.g. "cat" against an animals-but-no-cats library, sits
	// right at the floor and is intentionally not asserted — it's the tuning knob.)
	// d = sqrt(2*(1-cos)); a candidate is admitted when its distance ≤ cutoff.
	cutoff := cutoffFor(StrictnessNormal)
	d := func(cos float64) float64 { return math.Sqrt(2 * (1 - cos)) }

	for _, present := range []float64{0.126, 0.138, 0.150} {
		if d(present) > cutoff {
			t.Errorf("present match cos=%.3f (d=%.4f) excluded by cutoff %.4f", present, d(present), cutoff)
		}
	}
	for _, absent := range []float64{0.064, 0.043} {
		if d(absent) <= cutoff {
			t.Errorf("absent query cos=%.3f (d=%.4f) admitted by cutoff %.4f", absent, d(absent), cutoff)
		}
	}
}

func TestCutoffMonotonicInStrictness(t *testing.T) {
	// Higher floor ⇒ smaller distance cutoff.
	if !(cutoffFor(StrictnessStrict) < cutoffFor(StrictnessNormal) && cutoffFor(StrictnessNormal) < cutoffFor(StrictnessLoose)) {
		t.Fatalf("cutoffs not monotonic: strict=%f normal=%f loose=%f",
			cutoffFor(StrictnessStrict), cutoffFor(StrictnessNormal), cutoffFor(StrictnessLoose))
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
