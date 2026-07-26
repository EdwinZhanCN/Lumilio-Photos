package search

import (
	"context"
	"fmt"
	"math"
	"strings"
	"unicode"

	"server/internal/db/dbtypes"

	"github.com/google/uuid"
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

// EmbeddingSpaceProfile versions the semantic membership policy separately
// from tool code. The selected profile is bound to a model/index/language
// tuple and recorded in SetMeta and replayable Plan IR.
type EmbeddingSpaceProfile struct {
	ProfileVersion  string
	ModelVersion    string
	IndexVersion    string
	LanguageVersion string
	LooseFloor      float64
	NormalFloor     float64
	StrictFloor     float64
}

func (p EmbeddingSpaceProfile) CosFloor(strictness SetStrictness) float64 {
	switch strictness {
	case StrictnessLoose:
		return p.LooseFloor
	case StrictnessStrict:
		return p.StrictFloor
	default:
		return p.NormalFloor
	}
}

func queryLanguageVersion(query string) string {
	for _, r := range query {
		if unicode.Is(unicode.Han, r) {
			return "zh-hans-v1"
		}
	}
	return "en-v1"
}

// SelectEmbeddingSpaceProfile is deterministic and intentionally explicit:
// adding or relaxing a threshold requires a new profile version.
func SelectEmbeddingSpaceProfile(modelVersion, indexVersion, query string) EmbeddingSpaceProfile {
	language := queryLanguageVersion(query)
	profile := EmbeddingSpaceProfile{
		ProfileVersion: "generic-cosine-v1", ModelVersion: modelVersion,
		IndexVersion: indexVersion, LanguageVersion: language,
		LooseFloor: 0.080, NormalFloor: 0.090, StrictFloor: 0.105,
	}
	if strings.Contains(strings.ToLower(modelVersion), "siglip2") {
		profile.ProfileVersion = "siglip2-set-v1"
		if language == "zh-hans-v1" {
			// SigLIP 2's observed Chinese-query cosine scale is slightly
			// lower than English for the same image set.
			profile.LooseFloor = 0.065
			profile.NormalFloor = 0.075
			profile.StrictFloor = 0.090
		}
	}
	return profile
}

func (s SetStrictness) cosFloor() float64 {
	return SelectEmbeddingSpaceProfile("siglip2-base", "test-index", "english").CosFloor(s)
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
	// Version coordinates make the membership cutoff reproducible.
	ProfileVersion  string
	ModelVersion    string
	IndexVersion    string
	LanguageVersion string
}

// A vec0 KNN query can return at most sqliteVecKNNMax vectors. Keep the ANN
// asset pool within that bound after video-frame expansion; if every ANN
// candidate passes the cutoff, RetrieveSet falls back to an exact scan rather
// than exceeding sqlite-vec's k limit or claiming a truncated pool is complete.
const setInitialPoolSize = maxANNAssetCandidateSet

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
	queryVector := dbtypes.NewVector(embedding.Vector)

	indexVersion := fmt.Sprintf("embedding-space/%d", space.ID)
	modelVersion := space.ModelID
	if modelVersion == "" {
		modelVersion = embedding.Model
	}
	profile := SelectEmbeddingSpaceProfile(modelVersion, indexVersion, req.Query)

	// Membership cutoff: cos ≥ floor ⇔ d ≤ √(2·(1−floor)) for unit vectors.
	cosFloor := profile.CosFloor(strictness)
	cutoff := math.Sqrt(math.Max(0, 2*(1-cosFloor)))
	meta := SetMeta{
		Calibrated: true, CosFloor: cosFloor, Cutoff: cutoff,
		ProfileVersion: profile.ProfileVersion, ModelVersion: profile.ModelVersion,
		IndexVersion: profile.IndexVersion, LanguageVersion: profile.LanguageVersion,
	}

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
		candidates, truncated, err := r.retrieveExactWithinCutoff(ctx, req, queryVector, space.ID, space.Dimensions, cutoff, maxResults)
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
		case k >= maxANNAssetCandidateSet:
			// sqlite-vec cannot widen the ANN vector pool further. Preserve
			// set completeness with the authoritative scalar-distance path.
			candidates, truncated, err := r.retrieveExactWithinCutoff(ctx, req, queryVector, space.ID, space.Dimensions, cutoff, maxResults)
			if err != nil {
				return nil, meta, err
			}
			meta.Exact = true
			meta.Complete = !truncated
			return candidates, meta, nil
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

// retrieveExactWithinCutoff runs the strict path over authoritative vector
// BLOBs with sqlite-vec's exact scalar distance function.
func (r *EmbeddingRetriever) retrieveExactWithinCutoff(ctx context.Context, req Request, queryVector dbtypes.Vector, spaceID int64, dimensions int64, cutoff float64, maxResults int) ([]Candidate, bool, error) {
	if int64(len(queryVector.Slice())) != dimensions {
		return nil, false, fmt.Errorf("query vector dimension mismatch")
	}
	builder := &sqlBuilder{}
	vectorPlaceholder := builder.addArg(queryVector)
	spacePlaceholder := builder.addArg(spaceID)
	conditions, err := buildAssetFilterConditions(builder, req.Filter, "a")
	if err != nil {
		return nil, false, err
	}
	distanceExpr := fmt.Sprintf("vec_distance_L2(e.vector, %s)", vectorPlaceholder)
	// The cutoff is a per-frame predicate: an asset qualifies if any of its frames
	// falls within the cutoff, then it is ranked by its best (nearest) frame.
	conditions = append(conditions,
		fmt.Sprintf("e.space_id = %s", spacePlaceholder),
	)
	cutoffPlaceholder := builder.addArg(cutoff)
	limitPlaceholder := builder.addArg(maxResults + 1)

	query := fmt.Sprintf(`
WITH scored AS (
  SELECT
    a.asset_id,
    e.frame_ts_ms,
    %s AS distance
  FROM search_embeddings e
  JOIN assets a ON a.asset_id = e.asset_id
  WHERE %s
),
ranked AS (
  SELECT
    scored.*,
    ROW_NUMBER() OVER (
      PARTITION BY asset_id
      ORDER BY distance, frame_ts_ms IS NULL, frame_ts_ms
    ) AS distance_rank
  FROM scored
  WHERE distance <= %s
)
SELECT
  asset_id,
  MIN(distance) AS raw_score,
  MAX(CASE WHEN distance_rank = 1 THEN frame_ts_ms END) AS best_ts
FROM ranked
GROUP BY asset_id
ORDER BY raw_score, asset_id DESC
LIMIT %s
`, distanceExpr, joinConditions(conditions), cutoffPlaceholder, limitPlaceholder)

	rows, err := r.pool.QueryContext(ctx, query, builder.args...)
	if err != nil {
		return nil, false, fmt.Errorf("exact embedding retrieve: %w", err)
	}
	defer rows.Close()

	candidates, err := collectCandidates(rows, SourceEmbedding)
	if err != nil {
		return nil, false, err
	}
	truncated := len(candidates) > maxResults
	if truncated {
		candidates = candidates[:maxResults]
	}
	return candidates, truncated, nil
}

// filterWithinCutoff keeps candidates whose distance passes the cutoff,
// preserving relevance order. RawScore for the embedding channel is the
// sqlite-vec L2 distance (smaller = closer).
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
	AssetID  uuid.UUID
	Score    float64
	BestTsMs *int32
}

// FuseSet fuses per-channel candidate rankings with weighted RRF and returns
// the entire fused set in confidence order. No TopK is applied anywhere —
// each channel is expected to be self-thresholded (calibrated semantic set,
// BM25-matched OCR, tsquery-matched place, filename match).
func FuseSet(candidates []Candidate, weights map[string]float64) []ScoredAsset {
	fused := fuseWeightedRRF(candidates, weights, DefaultRRFK)
	out := make([]ScoredAsset, len(fused))
	for i, item := range fused {
		out[i] = ScoredAsset{AssetID: item.assetID, Score: item.score, BestTsMs: item.bestTsMs}
	}
	return out
}
