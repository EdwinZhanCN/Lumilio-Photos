package phash

import (
	"sort"
	"testing"
)

func TestFromVectorRoundTrip(t *testing.T) {
	if _, ok := FromVector(make([]float32, 10)); ok {
		t.Error("non-64 vector should be rejected")
	}
	vec := make([]float32, 64)
	vec[0] = 1
	vec[63] = 1
	h, ok := FromVector(vec)
	if !ok {
		t.Fatal("64-element vector should convert")
	}
	if h != (uint64(1)<<0 | uint64(1)<<63) {
		t.Errorf("hash bits wrong: %b", h)
	}
}

func TestHammingDistance(t *testing.T) {
	if d := HammingDistance(0b1011, 0b1110); d != 2 {
		t.Errorf("hamming = %d, want 2", d)
	}
}

func TestClusterGroupsNearDuplicatesAndKeepsSingletons(t *testing.T) {
	// 0,1,2 are within threshold of each other (<=2 bits apart); 3 is far.
	base := uint64(0xFFFF_0000_FFFF_0000)
	hashes := []uint64{
		base,
		base ^ 0b1,  // 1 bit
		base ^ 0b11, // 2 bits from base
		^base,       // far away (64 bits)
	}
	groups := Cluster(hashes, 6)

	// Normalize: sort each group and sort groups by first index.
	for _, g := range groups {
		sort.Ints(g)
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i][0] < groups[j][0] })

	if len(groups) != 2 {
		t.Fatalf("got %d groups, want 2", len(groups))
	}
	if len(groups[0]) != 3 || groups[0][0] != 0 || groups[0][2] != 2 {
		t.Errorf("first group = %v, want [0 1 2]", groups[0])
	}
	if len(groups[1]) != 1 || groups[1][0] != 3 {
		t.Errorf("second group = %v, want [3]", groups[1])
	}
}

func TestClusterEveryIndexAppearsOnce(t *testing.T) {
	hashes := []uint64{1, 2, 4, 8, 16, 1 << 40}
	seen := make(map[int]int)
	for _, g := range Cluster(hashes, 6) {
		for _, idx := range g {
			seen[idx]++
		}
	}
	if len(seen) != len(hashes) {
		t.Fatalf("covered %d indices, want %d", len(seen), len(hashes))
	}
	for idx, n := range seen {
		if n != 1 {
			t.Errorf("index %d appeared %d times", idx, n)
		}
	}
}
