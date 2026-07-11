package tools

import (
	"testing"

	"github.com/google/uuid"
)

func TestDedupeByPHashCollapsesAndPreservesOrder(t *testing.T) {
	ids := make([]uuid.UUID, 5)
	for i := range ids {
		ids[i] = uuid.New()
	}
	base := uint64(0xABCD_1234_ABCD_1234)
	hashOf := map[uuid.UUID]uint64{
		ids[0]: base,        // representative of the burst
		ids[1]: base ^ 0b1,  // near-dup of ids[0]
		ids[2]: 0xFFFF,      // unique
		ids[3]: base ^ 0b11, // near-dup of ids[0]
		// ids[4] has no pHash → must always survive
	}

	kept, clusters, removed := dedupeByPHash(ids, hashOf, 1)

	if clusters != 1 {
		t.Errorf("clusters = %d, want 1", clusters)
	}
	if removed != 2 {
		t.Errorf("removed = %d, want 2", removed)
	}
	// Expect order-preserving kept set: ids[0] (rep), ids[2], ids[4].
	want := []uuid.UUID{ids[0], ids[2], ids[4]}
	if len(kept) != len(want) {
		t.Fatalf("kept %d, want %d", len(kept), len(want))
	}
	for i := range want {
		if kept[i] != want[i] {
			t.Errorf("kept[%d] = %v, want %v", i, kept[i], want[i])
		}
	}
}

func TestDedupeByPHashKeepPerCluster(t *testing.T) {
	ids := make([]uuid.UUID, 4)
	for i := range ids {
		ids[i] = uuid.New()
	}
	base := uint64(0x1111_2222_3333_4444)
	hashOf := map[uuid.UUID]uint64{
		ids[0]: base,
		ids[1]: base ^ 0b1,
		ids[2]: base ^ 0b10,
		ids[3]: 0xAAAA, // unique
	}

	kept, clusters, removed := dedupeByPHash(ids, hashOf, 2)
	if clusters != 1 {
		t.Errorf("clusters = %d, want 1", clusters)
	}
	if removed != 1 {
		t.Errorf("removed = %d, want 1", removed)
	}
	want := []uuid.UUID{ids[0], ids[1], ids[3]}
	if len(kept) != len(want) {
		t.Fatalf("kept %d, want %d (%v)", len(kept), len(want), kept)
	}
	for i := range want {
		if kept[i] != want[i] {
			t.Errorf("kept[%d] = %v, want %v", i, kept[i], want[i])
		}
	}
}

func TestDedupeByPHashNoDuplicates(t *testing.T) {
	ids := []uuid.UUID{uuid.New(), uuid.New()}
	// 16 bits apart — well beyond the duplicate threshold.
	hashOf := map[uuid.UUID]uint64{ids[0]: 0x0, ids[1]: 0xFFFF}
	kept, clusters, removed := dedupeByPHash(ids, hashOf, 1)
	if clusters != 0 || removed != 0 || len(kept) != 2 {
		t.Errorf("got clusters=%d removed=%d kept=%d, want 0/0/2", clusters, removed, len(kept))
	}
}
