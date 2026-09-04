package pipeline

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

func TestSourceFenceRoundTripsAsAUUID(t *testing.T) {
	id := uuid.New()
	fence, err := NewSourceFence(id)
	if err != nil {
		t.Fatal(err)
	}
	if fence.UUID() != id || fence.String() != id.String() {
		t.Fatalf("source fence = %s, want %s", fence, id)
	}
	encoded, err := json.Marshal(fence)
	if err != nil {
		t.Fatal(err)
	}
	var decoded SourceFence
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded != fence {
		t.Fatalf("decoded source fence = %s, want %s", decoded, fence)
	}
}

func TestSourceFenceRejectsNil(t *testing.T) {
	if _, err := NewSourceFence(uuid.Nil); err == nil {
		t.Fatal("nil source fence was accepted")
	}
}
