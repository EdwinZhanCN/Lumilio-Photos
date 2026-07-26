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

func TestEmbeddingSpaceProfilesCoverLanguageAndLibraryScaleFixtures(t *testing.T) {
	fixtures := []struct {
		name        string
		query       string
		librarySize int
		presentCos  float64
		absentCos   float64
		language    string
	}{
		{name: "english-small", query: "sunset over ocean", librarySize: 250, presentCos: 0.126, absentCos: 0.064, language: "en-v1"},
		{name: "english-large", query: "red sports car", librarySize: 100000, presentCos: 0.138, absentCos: 0.043, language: "en-v1"},
		{name: "chinese-small", query: "海边日落", librarySize: 250, presentCos: 0.092, absentCos: 0.051, language: "zh-hans-v1"},
		{name: "chinese-large", query: "红色跑车", librarySize: 100000, presentCos: 0.088, absentCos: 0.056, language: "zh-hans-v1"},
	}
	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			profile := SelectEmbeddingSpaceProfile("siglip2-base", "embedding-space/9", fixture.query)
			if profile.ProfileVersion != "siglip2-set-v1" || profile.LanguageVersion != fixture.language {
				t.Fatalf("unexpected profile: %+v", profile)
			}
			if profile.IndexVersion != "embedding-space/9" {
				t.Fatalf("index version = %q", profile.IndexVersion)
			}
			floor := profile.CosFloor(StrictnessNormal)
			if fixture.presentCos < floor {
				t.Fatalf("present fixture excluded at library size %d: cos=%f floor=%f", fixture.librarySize, fixture.presentCos, floor)
			}
			if fixture.absentCos >= floor {
				t.Fatalf("absent fixture admitted at library size %d: cos=%f floor=%f", fixture.librarySize, fixture.absentCos, floor)
			}
		})
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

func TestExpandedEmbeddingKNNLimitRespectsSQLiteVecMaximum(t *testing.T) {
	cases := map[int]int{
		0:                           1,
		1:                           embeddingKNNExpansion,
		maxANNAssetCandidateSet:     sqliteVecKNNMax,
		maxANNAssetCandidateSet + 1: sqliteVecKNNMax,
		10_000:                      sqliteVecKNNMax,
	}
	for topK, want := range cases {
		if got := expandedEmbeddingKNNLimit(topK); got != want {
			t.Errorf("expandedEmbeddingKNNLimit(%d) = %d, want %d", topK, got, want)
		}
	}
}
