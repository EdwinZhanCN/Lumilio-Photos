package search

import (
	"context"
	"fmt"
	"math"

	"github.com/google/uuid"
	"github.com/pgvector/pgvector-go"
)

// Set retrieval turns the dense embedding channel into a membership test:
// instead of "the K nearest" it answers "everything relevant to this query".
//
// Membership is a fixed cosine floor. Embeddings are L2-normalized, so cosine
// similarity is cos = 1 − d²/2 for an L2 distance d, and "belongs to the set"
// means cos ≥ floor (equivalently d ≤ √(2·(1−floor))). A query with nothing
// above the floor legitimately returns the empty set; obvious matches return.
//
// The floor is an absolute cosine, not a SigLIP probability. SigLIP's sigmoid
// (exp(logit_scale)·cos + logit_bias) is calibrated but its zero-shot match
// probabilities are intrinsically tiny — a clean match scores p≈0.15, a typical
// one p≈0.005 — so a probability bar is unintuitive and razor-thin. In cosine
// space the separation is workable: on siglip2-base, present concepts land at
// cos≈0.12–0.15, while absent queries (including semantically related but
// missing ones, e.g. "cat" against an animals-but-no-cats library) sit at
// cos≈0.04–0.09. A floor near 0.105 divides them. The margin is narrow and the
// floors below are model- and library-specific and meant to be tuned; English
// queries also score higher than other languages on this model.

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

// cosFloor is the minimum cosine similarity an asset must reach to belong to
// the set. Tuned to siglip2-base's observed scale (present matches ≈0.12–0.15,
// absent/near-miss ≈0.04–0.09); see the package doc. Loose favors recall,
// strict favors precision.
func (s SetStrictness) cosFloor() float64 {
	switch s {
	case StrictnessLoose:
		return 0.080
	case StrictnessStrict:
		return 0.105
	default:
		return 0.090
	}
}

// SetMeta reports how a set retrieval ran; the agent receipt surfaces it so
// the model can decide whether a strict retry is warranted.
type SetMeta struct {
	// Calibrated is always true: a cosine floor is applied unconditionally
	// (it is pure geometry on unit vectors). Retained for the agent receipt.
	Calibrated bool
	// CosFloor is the cosine bar applied.
	CosFloor float64
	// Cutoff is the max L2 distance admitted (√(2·(1−CosFloor))).
	Cutoff float64
	// Scanned is the candidate pool size examined.
	Scanned int
	// Complete is true when the set is provably whole: the cutoff bit
	// inside the scanned pool, or the scan was exact/exhaustive.
	Complete bool
	// Exact marks the strict full-scan path.
	Exact bool
}

const setInitialPoolSize = 1000

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

	// Membership cutoff: cos ≥ floor ⇔ d ≤ √(2·(1−floor)) for unit vectors.
	cosFloor := strictness.cosFloor()
	cutoff := math.Sqrt(math.Max(0, 2*(1-cosFloor)))
	meta := SetMeta{Calibrated: true, CosFloor: cosFloor, Cutoff: cutoff}

	// First pool fetch anchors the set in nearest-distance order.
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

	if len(pool) == 0 {
		meta.Complete = true
		return pool, meta, nil
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
// BM25-matched OCR, tsquery-matched place, filename match).
func FuseSet(candidates []Candidate, weights map[string]float64) []ScoredAsset {
	fused := fuseWeightedRRF(candidates, weights, DefaultRRFK)
	out := make([]ScoredAsset, len(fused))
	for i, item := range fused {
		out[i] = ScoredAsset{AssetID: item.assetID, Score: item.score}
	}
	return out
}
