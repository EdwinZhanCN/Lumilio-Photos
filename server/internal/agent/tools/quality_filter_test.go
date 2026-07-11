package tools

import (
	"math"
	"testing"

	"github.com/google/uuid"
)

func TestKeepAtOrAboveQualityPercentile(t *testing.T) {
	ids := make([]uuid.UUID, 5)
	for i := range ids {
		ids[i] = uuid.New()
	}
	// Ordered snapshot; scores: 4, 5, 6, 7, unscored.
	scoreOf := map[uuid.UUID]float32{
		ids[0]: 4,
		ids[1]: 5,
		ids[2]: 6,
		ids[3]: 7,
	}

	kept, cut, scored := keepAtOrAboveQualityPercentile(ids, scoreOf, 75)
	if scored != 4 {
		t.Fatalf("scored = %d, want 4", scored)
	}
	// p75 of [4,5,6,7] via percentile_cont: 6.25 → keep only 7.
	if math.Abs(float64(cut)-6.25) > 1e-6 {
		t.Fatalf("cut = %v, want 6.25", cut)
	}
	if len(kept) != 1 || kept[0] != ids[3] {
		t.Fatalf("kept = %v, want [%v]", kept, ids[3])
	}
}

func TestKeepAtOrAboveQualityPercentilePreservesOrder(t *testing.T) {
	ids := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}
	scoreOf := map[uuid.UUID]float32{ids[0]: 9, ids[1]: 5, ids[2]: 8}
	kept, _, _ := keepAtOrAboveQualityPercentile(ids, scoreOf, 50)
	// p50 of [5,8,9] = 8 → keep 9 then 8 in original order.
	want := []uuid.UUID{ids[0], ids[2]}
	if len(kept) != len(want) {
		t.Fatalf("kept %d, want %d", len(kept), len(want))
	}
	for i := range want {
		if kept[i] != want[i] {
			t.Errorf("kept[%d] = %v, want %v", i, kept[i], want[i])
		}
	}
}

func TestKeepAtOrAboveQualityPercentileNoScores(t *testing.T) {
	ids := []uuid.UUID{uuid.New()}
	kept, cut, scored := keepAtOrAboveQualityPercentile(ids, nil, 75)
	if len(kept) != 0 || cut != 0 || scored != 0 {
		t.Errorf("got kept=%d cut=%v scored=%d, want empty", len(kept), cut, scored)
	}
}

func TestPercentileCont(t *testing.T) {
	vals := []float64{1, 2, 3, 4}
	if got := percentileCont(vals, 0.5); math.Abs(got-2.5) > 1e-9 {
		t.Errorf("p50 = %v, want 2.5", got)
	}
	if got := percentileCont(vals, 0); got != 1 {
		t.Errorf("p0 = %v, want 1", got)
	}
	if got := percentileCont(vals, 1); got != 4 {
		t.Errorf("p100 = %v, want 4", got)
	}
}
