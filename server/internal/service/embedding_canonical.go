package service

import "math"

// CanonicalEmbeddingDim is the fixed storage/query dimension for semantic
// (SigLIP2) vectors. SigLIP2 is Matryoshka-trained, so a model's output can be
// truncated to this leading prefix with negligible retrieval / zero-shot loss.
// Fixing the dimension lets a single cosine HNSW index serve every model and
// keeps the schema stable across model swaps (a swap is a full re-embed, never a
// column migration). base is natively 768; so400m (1152) truncates to it.
const CanonicalEmbeddingDim = 768

// canonicalizeSemanticVector maps a raw model embedding into the canonical
// semantic space: truncate to the leading CanonicalEmbeddingDim components, then
// L2-normalize. Every semantic embed site — stored image vectors, text queries,
// and zero-shot label prototypes — MUST pass through this so all vectors share
// one comparable, unit-length space. Vectors shorter than the canonical
// dimension are normalized without truncation.
//
// The returned slice is always freshly allocated; the input is never mutated.
func canonicalizeSemanticVector(vec []float32) []float32 {
	if len(vec) > CanonicalEmbeddingDim {
		vec = vec[:CanonicalEmbeddingDim]
	}
	return l2Normalize(vec)
}

// l2Normalize returns a unit-length copy of vec. A zero (or empty) vector is
// returned as a plain copy, since it has no direction to normalize.
func l2Normalize(vec []float32) []float32 {
	out := make([]float32, len(vec))
	copy(out, vec)

	var sumSq float64
	for _, v := range out {
		sumSq += float64(v) * float64(v)
	}
	if sumSq == 0 {
		return out
	}

	norm := float32(math.Sqrt(sumSq))
	for i := range out {
		out[i] /= norm
	}
	return out
}
