package search

// ImageQueryCosineFloor drops unrelated tail hits for image-to-image KNN.
// Text set-retrieval floors (~0.105) are calibrated for text→image, not
// image→image. Near-duplicates score far above 0.25; typical same-scene /
// same-subject hits should pass; random catalog noise should not.
// Tune only with a fixture comment and a test, not a setting.
const ImageQueryCosineFloor = 0.25

// CosineFromL2 converts an L2 distance between unit vectors to cosine
// similarity: cos = 1 − d²/2.
func CosineFromL2(l2 float64) float64 {
	return 1 - (l2*l2)/2
}

// FilterByCosineFloor keeps candidates whose L2 RawScore maps to cosine
// similarity at or above floor, preserving input order.
func FilterByCosineFloor(candidates []Candidate, floor float64) []Candidate {
	out := make([]Candidate, 0, len(candidates))
	for _, candidate := range candidates {
		if CosineFromL2(candidate.RawScore) >= floor {
			out = append(out, candidate)
		}
	}
	return out
}
