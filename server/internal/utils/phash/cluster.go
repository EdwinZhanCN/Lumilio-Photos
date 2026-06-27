package phash

import "math/bits"

// DefaultDuplicateThreshold is the maximum Hamming distance (bits, out of 64)
// at which two perceptual hashes are treated as near-duplicates. Mirrors the
// detection pipeline's PHashDuplicateThreshold.
const DefaultDuplicateThreshold = 6

// FromVector reconstructs the 64-bit perceptual hash from the 0/1 float vector
// stored in embeddings.vector (the inverse of ToVector). Returns false when the
// vector is not a 64-element pHash.
func FromVector(vec []float32) (uint64, bool) {
	if len(vec) != 64 {
		return 0, false
	}
	var out uint64
	for i := 0; i < 64; i++ {
		if vec[i] >= 0.5 {
			out |= uint64(1) << uint(i)
		}
	}
	return out, true
}

// HammingDistance is the number of differing bits between two hashes.
func HammingDistance(a, b uint64) int {
	return bits.OnesCount64(a ^ b)
}

// Cluster groups hashes whose pairwise Hamming distance is within threshold,
// returning each group as a slice of indices into the input. Singletons are
// returned as one-element groups, so every index appears exactly once.
//
// A 16-bit prefix bucket over the 4 chunks of each hash is used as a cheap
// candidate filter before the exact distance check (pigeonhole: two hashes
// within k<=6 bits must share at least one identical 16-bit chunk), then a
// union-find merges connected near-duplicates transitively.
func Cluster(hashes []uint64, threshold int) [][]int {
	n := len(hashes)
	parent := make([]int, n)
	rank := make([]int, n)
	for i := range parent {
		parent[i] = i
	}
	var find func(int) int
	find = func(x int) int {
		for parent[x] != x {
			parent[x] = parent[parent[x]]
			x = parent[x]
		}
		return x
	}
	union := func(a, b int) {
		ra, rb := find(a), find(b)
		if ra == rb {
			return
		}
		if rank[ra] < rank[rb] {
			ra, rb = rb, ra
		}
		parent[rb] = ra
		if rank[ra] == rank[rb] {
			rank[ra]++
		}
	}

	// Candidate pairs share at least one identical 16-bit chunk.
	type pairKey struct{ a, b int }
	candidates := make(map[pairKey]struct{})
	for chunk := 0; chunk < 4; chunk++ {
		shift := uint(chunk * 16)
		buckets := make(map[uint16][]int)
		for idx, h := range hashes {
			prefix := uint16(h >> shift)
			buckets[prefix] = append(buckets[prefix], idx)
		}
		for _, idxs := range buckets {
			for i := 0; i < len(idxs); i++ {
				for j := i + 1; j < len(idxs); j++ {
					a, b := idxs[i], idxs[j]
					if a > b {
						a, b = b, a
					}
					candidates[pairKey{a, b}] = struct{}{}
				}
			}
		}
	}
	for pk := range candidates {
		if HammingDistance(hashes[pk.a], hashes[pk.b]) <= threshold {
			union(pk.a, pk.b)
		}
	}

	groups := make(map[int][]int, n)
	order := make([]int, 0, n) // preserve first-seen order of group roots
	for i := 0; i < n; i++ {
		root := find(i)
		if _, ok := groups[root]; !ok {
			order = append(order, root)
		}
		groups[root] = append(groups[root], i)
	}
	out := make([][]int, 0, len(order))
	for _, root := range order {
		out = append(out, groups[root])
	}
	return out
}
