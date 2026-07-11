package tools

import (
	"math"
	"sort"

	"github.com/google/uuid"
)

// keepAtOrAboveQualityPercentile keeps ordered assets whose aesthetic score is
// at or above the given percentile of the scored subset (e.g. 75 → p75).
// Unscored assets are dropped. Returns the kept ids, the score cutoff used,
// and how many assets in the input carried a score. When scoredCount is 0,
// kept is empty and cut is 0.
func keepAtOrAboveQualityPercentile(
	ordered []uuid.UUID,
	scoreOf map[uuid.UUID]float32,
	percentile float64,
) (kept []uuid.UUID, cut float32, scoredCount int) {
	if len(ordered) == 0 || len(scoreOf) == 0 {
		return nil, 0, 0
	}

	vals := make([]float64, 0, len(scoreOf))
	for _, id := range ordered {
		if s, ok := scoreOf[id]; ok {
			vals = append(vals, float64(s))
		}
	}
	scoredCount = len(vals)
	if scoredCount == 0 {
		return nil, 0, 0
	}

	sort.Float64s(vals)
	cut = float32(percentileCont(vals, percentile/100.0))

	kept = make([]uuid.UUID, 0, scoredCount)
	for _, id := range ordered {
		if s, ok := scoreOf[id]; ok && s >= cut {
			kept = append(kept, id)
		}
	}
	return kept, cut, scoredCount
}

// percentileCont mirrors PostgreSQL percentile_cont: linear interpolation at
// fraction f in [0, 1] over a sorted sample.
func percentileCont(sorted []float64, f float64) float64 {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	if n == 1 || f <= 0 {
		return sorted[0]
	}
	if f >= 1 {
		return sorted[n-1]
	}
	pos := f * float64(n-1)
	lo := int(math.Floor(pos))
	hi := int(math.Ceil(pos))
	if lo == hi {
		return sorted[lo]
	}
	weight := pos - float64(lo)
	return sorted[lo]*(1-weight) + sorted[hi]*weight
}
