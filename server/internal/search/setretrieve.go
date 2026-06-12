package search

import (
	"context"
	"fmt"
	"math"
	"sort"

	"github.com/google/uuid"
	"github.com/pgvector/pgvector-go"
)

// Set retrieval turns the dense embedding channel into a membership test:
// instead of "the K nearest" it answers "everything relevant to this query".
//
// Membership is decided per query in two steps, with no global threshold:
//
//  1. Signal gate — the best match must itself be significantly closer than
//     the background distance distribution (median/MAD over a random sample
//     of the space). Queries with no real match ("nonsense" queries) fail
//     the gate and legitimately return the empty set.
//  2. Gap admission — when signal exists, the cutoff is anchored at the best
//     match: cutoff = d_best + α·(median − d_best). Everything that closes
//     at least (1−α) of the best match's gap to background is in. Anchoring
//     at d_best guarantees a non-empty set whenever an obvious match exists,
//     regardless of how tight the model's distance distribution is.
//
// Robust statistics (median, MAD) are used instead of mean/stddev because
// CLIP-style text→image distances are tight and skewed, and on small or
// homogeneous libraries the relevant cluster contaminates the background
// sample itself.

// SetStrictness selects the relevance bar and the retrieval mode.
// loose/normal run on the ANN index with iterative pool widening; strict
// tightens the bar AND performs an exact sequential scan (no ANN recall
// loss), which is why it is reserved for cap-hit retries and explicit
// user demand.
type SetStrictness string

const (
	StrictnessLoose  SetStrictness = "loose"
	StrictnessNormal SetStrictness = "normal"
	StrictnessStrict SetStrictness = "strict"
)

// ParseStrictness normalizes a strictness string, defaulting to normal.
func ParseStrictness(raw string) SetStrictness {
	switch SetStrictness(raw) {
	case StrictnessLoose, StrictnessNormal, StrictnessStrict:
		return SetStrictness(raw)
	default:
		return StrictnessNormal
	}
}

// minSignal is the significance the best match must reach against background
// before any result is admitted (in robust-σ units).
func (s SetStrictness) minSignal() float64 {
	switch s {
	case StrictnessLoose:
		return 1.5
	case StrictnessStrict:
		return 2.5
	default:
		return 2.0
	}
}

// gapFraction is α in cutoff = d_best + α·(median − d_best): how much of the
// best match's gap to background a candidate may give up and still belong.
func (s SetStrictness) gapFraction() float64 {
	switch s {
	case StrictnessLoose:
		return 0.55
	case StrictnessStrict:
		return 0.25
	default:
		return 0.40
	}
}

// SetMeta reports how a set retrieval ran; the agent receipt surfaces it so
// the model can decide whether a strict retry is warranted.
type SetMeta struct {
	// Calibrated is false when the library is too small to estimate a
	// background distribution; no cutoff was applied.
	Calibrated bool
	// Signal is how strongly the best match stands out from background, in
	// robust-σ units. Below the strictness gate the set is empty.
	Signal float64
	// Cutoff is the max distance admitted (when calibrated and gated).
	Cutoff float64
	// SampleSize is the background sample used for calibration.
	SampleSize int
	// Scanned is the candidate pool size examined.
	Scanned int
	// Complete is true when the set is provably whole: the cutoff bit
	// inside the scanned pool, or the scan was exact/exhaustive.
	Complete bool
	// Exact marks the strict full-scan path.
	Exact bool
}

const (
	calibrationSampleSize = 256
	// minCalibrationLibrary is the embedding count below which calibration
	// is meaningless; tiny libraries return the whole candidate pool.
	minCalibrationLibrary = 64
	setInitialPoolSize    = 1000
)

// RetrieveSet returns every candidate within the calibrated relevance
// cutoff, in relevance order, up to maxResults.
func (r *EmbeddingRetriever) RetrieveSet(ctx context.Context, req Request, strictness SetStrictness, maxResults int) ([]Candidate, SetMeta, error) {
	if r == nil || r.pool == nil || r.embed == nil || r.resolveSpace == nil {
		return nil, SetMeta{}, fmt.Errorf("embedding retriever is not configured")
	}
	if maxResults <= 0 {
		return nil, SetMeta{}, fmt.Errorf("maxResults must be positive")
	}

	embedding, space, err := r.resolveQuerySpace(ctx, req)
	if err != nil {
		return nil, SetMeta{}, err
	}
	queryVector := pgvector.NewVector(embedding.Vector)

	// Calibrate the background distance distribution for this query.
	sample, err := r.sampleBackgroundDistances(ctx, &queryVector, space.ID, space.Dimensions)
	if err != nil {
		return nil, SetMeta{}, err
	}
	background, calibrated := calibrateBackground(sample)
	meta := SetMeta{Calibrated: calibrated, SampleSize: len(sample)}

	// First pool fetch anchors the cutoff at the (approximate) best match.
	k := setInitialPoolSize
	if k > maxResults {
		k = maxResults
	}
	poolReq := req
	poolReq.TopK = k
	pool, err := r.Retrieve(ctx, poolReq)
	if err != nil {
		return nil, meta, err
	}
	meta.Scanned = len(pool)

	if !calibrated || len(pool) == 0 {
		// Library too small to calibrate (or empty): the pool is the set.
		meta.Complete = len(pool) < k
		return pool, meta, nil
	}

	cutoff, signal, hasSignal := admissionCutoff(background, pool[0].RawScore, strictness)
	meta.Signal = signal
	meta.Cutoff = cutoff
	if !hasSignal {
		// Even the best match is indistinguishable from background: nothing
		// in the library genuinely matches this query.
		meta.Complete = true
		return []Candidate{}, meta, nil
	}

	if strictness == StrictnessStrict {
		candidates, truncated, err := r.retrieveExactWithinCutoff(ctx, req, &queryVector, space.ID, space.Dimensions, cutoff, maxResults)
		if err != nil {
			return nil, meta, err
		}
		meta.Exact = true
		meta.Complete = !truncated
		return candidates, meta, nil
	}

	// ANN path with iterative widening: grow the KNN pool until the cutoff
	// provably bites inside it (the set is then complete) or the pool hits
	// the cap.
	for {
		kept := filterWithinCutoff(pool, cutoff)

		switch {
		case len(pool) < k:
			// Library exhausted inside the pool — the set is complete.
			meta.Complete = true
			return kept, meta, nil
		case len(kept) < len(pool):
			// The cutoff bit inside the pool: everything beyond the pool is
			// farther than the worst pool member, hence beyond the cutoff.
			meta.Complete = true
			return kept, meta, nil
		case k >= maxResults:
			// Cap reached and the cutoff never bit: truncated set.
			meta.Complete = false
			if len(kept) > maxResults {
				kept = kept[:maxResults]
			}
			return kept, meta, nil
		}

		k *= 2
		if k > maxResults {
			k = maxResults
		}
		poolReq.TopK = k
		pool, err = r.Retrieve(ctx, poolReq)
		if err != nil {
			return nil, meta, err
		}
		meta.Scanned = len(pool)
	}
}

// sampleBackgroundDistances estimates the null distribution: distances from
// the query to a random sample of the space's primary embeddings.
func (r *EmbeddingRetriever) sampleBackgroundDistances(ctx context.Context, queryVector *pgvector.Vector, spaceID int64, dimensions int32) ([]float64, error) {
	// ORDER BY random() scans the space's embeddings, which is fine at
	// personal-library scale; revisit with TABLESAMPLE if spaces grow.
	query := fmt.Sprintf(`
SELECT (e.vector::vector(%d) <-> $1::vector(%d))::float8
FROM embeddings e
WHERE e.space_id = $2 AND e.is_primary = true
ORDER BY random()
LIMIT %d
`, dimensions, dimensions, calibrationSampleSize)

	rows, err := r.pool.Query(ctx, query, queryVector, spaceID)
	if err != nil {
		return nil, fmt.Errorf("calibration sample: %w", err)
	}
	defer rows.Close()

	distances := make([]float64, 0, calibrationSampleSize)
	for rows.Next() {
		var d float64
		if err := rows.Scan(&d); err != nil {
			return nil, fmt.Errorf("scan calibration distance: %w", err)
		}
		distances = append(distances, d)
	}
	return distances, rows.Err()
}

// retrieveExactWithinCutoff runs the strict path: a sequential scan with the
// cutoff as a hard predicate, immune to ANN recall loss. Index scans are
// disabled for the transaction so the planner cannot fall back to an
// approximate HNSW traversal.
func (r *EmbeddingRetriever) retrieveExactWithinCutoff(ctx context.Context, req Request, queryVector *pgvector.Vector, spaceID int64, dimensions int32, cutoff float64, maxResults int) ([]Candidate, bool, error) {
	builder := &sqlBuilder{}
	vectorPlaceholder := builder.addArg(queryVector)
	spacePlaceholder := builder.addArg(spaceID)
	conditions, err := buildAssetFilterConditions(builder, req.Filter, "a")
	if err != nil {
		return nil, false, err
	}
	distanceExpr := fmt.Sprintf("(e.vector::vector(%d) <-> %s::vector(%d))", dimensions, vectorPlaceholder, dimensions)
	conditions = append(conditions,
		fmt.Sprintf("e.space_id = %s", spacePlaceholder),
		"e.is_primary = true",
		fmt.Sprintf("%s <= %s", distanceExpr, builder.addArg(cutoff)),
	)
	limitPlaceholder := builder.addArg(maxResults + 1)

	query := fmt.Sprintf(`
SELECT
  a.asset_id,
  %s::float8 AS raw_score
FROM embeddings e
JOIN assets a ON a.asset_id = e.asset_id
WHERE %s
ORDER BY %s, a.asset_id DESC
LIMIT %s
`, distanceExpr, joinConditions(conditions), distanceExpr, limitPlaceholder)

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("exact retrieve begin: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, "SET LOCAL enable_indexscan = off"); err != nil {
		return nil, false, fmt.Errorf("exact retrieve disable index scan: %w", err)
	}

	rows, err := tx.Query(ctx, query, builder.args...)
	if err != nil {
		return nil, false, fmt.Errorf("exact embedding retrieve: %w", err)
	}
	defer rows.Close()

	candidates, err := collectCandidates(rows, SourceEmbedding)
	if err != nil {
		return nil, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, false, fmt.Errorf("exact retrieve commit: %w", err)
	}

	truncated := len(candidates) > maxResults
	if truncated {
		candidates = candidates[:maxResults]
	}
	return candidates, truncated, nil
}

// backgroundStats is the robust location/spread of the background distance
// distribution for one query.
type backgroundStats struct {
	median float64
	spread float64 // 1.4826 × MAD ≈ robust σ
}

// calibrateBackground derives robust statistics from the background sample.
// Returns ok=false when the sample is too small or degenerate to calibrate.
func calibrateBackground(distances []float64) (backgroundStats, bool) {
	if len(distances) < minCalibrationLibrary {
		return backgroundStats{}, false
	}

	med := median(distances)

	deviations := make([]float64, len(distances))
	for i, d := range distances {
		deviations[i] = math.Abs(d - med)
	}
	mad := median(deviations)
	spread := 1.4826 * mad
	if spread <= 1e-9 {
		return backgroundStats{}, false
	}
	return backgroundStats{median: med, spread: spread}, true
}

// admissionCutoff decides membership for one query: a signal gate on the
// best match, then a cutoff anchored at it. hasSignal=false means nothing
// in the library genuinely matches.
func admissionCutoff(background backgroundStats, dBest float64, strictness SetStrictness) (cutoff, signal float64, hasSignal bool) {
	gap := background.median - dBest
	signal = gap / background.spread
	if signal < strictness.minSignal() {
		return 0, signal, false
	}
	cutoff = dBest + strictness.gapFraction()*gap
	return cutoff, signal, true
}

func median(values []float64) float64 {
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	n := len(sorted)
	if n%2 == 1 {
		return sorted[n/2]
	}
	return (sorted[n/2-1] + sorted[n/2]) / 2
}

// filterWithinCutoff keeps candidates whose distance passes the cutoff,
// preserving relevance order. RawScore for the embedding channel is the
// pgvector distance (smaller = closer).
func filterWithinCutoff(candidates []Candidate, cutoff float64) []Candidate {
	kept := make([]Candidate, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.RawScore <= cutoff {
			kept = append(kept, candidate)
		}
	}
	return kept
}

// ScoredAsset is one member of a fused set with its aggregate RRF confidence.
type ScoredAsset struct {
	AssetID uuid.UUID
	Score   float64
}

// FuseSet fuses per-channel candidate rankings with weighted RRF and returns
// the entire fused set in confidence order. No TopK is applied anywhere —
// each channel is expected to be self-thresholded (calibrated semantic set,
// tsquery-matched OCR/place, filename match).
func FuseSet(candidates []Candidate, weights map[string]float64) []ScoredAsset {
	fused := fuseWeightedRRF(candidates, weights, DefaultRRFK)
	out := make([]ScoredAsset, len(fused))
	for i, item := range fused {
		out[i] = ScoredAsset{AssetID: item.assetID, Score: item.score}
	}
	return out
}
