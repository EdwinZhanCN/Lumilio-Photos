package pins

import (
	"encoding/json"
	"testing"

	"server/internal/agent/ref"
)

func TestLiveReplayRequiresExactVersionAndAuthorizationScope(t *testing.T) {
	plan, err := (ref.Plan{
		Op:      "search_text",
		Payload: ref.TypedPayload(map[string]any{"query": "receipt"}),
	}).Normalize(7)
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	if !isReplayable(plan, 7) {
		t.Fatal("current owner-scoped producer plan should replay")
	}

	cases := map[string]ref.Plan{
		"legacy schema": func() ref.Plan {
			copy := plan
			copy.SchemaVersion = 0
			return copy
		}(),
		"old tool": func() ref.Plan {
			copy := plan
			copy.ToolVersion = "agent-tools/v0"
			return copy
		}(),
		"old policy": func() ref.Plan {
			copy := plan
			copy.CreationPolicyVersion = 0
			return copy
		}(),
		"parent ref": func() ref.Plan {
			copy := plan
			copy.Parents = []string{"r1"}
			return copy
		}(),
	}
	for name, candidate := range cases {
		if isReplayable(candidate, 7) {
			t.Errorf("%s unexpectedly replayable", name)
		}
	}
	if isReplayable(plan, 8) {
		t.Fatal("cross-user plan unexpectedly replayable")
	}
}

// The filter replay payload carries the media-item vocabulary. There is no
// migration for pins written before this change: an old payload simply decodes
// without the new fields, so its composition/stack filters do not apply.
func TestFilterReplayPayloadCarriesCompositionAndStackFilters(t *testing.T) {
	var payload filterReplayPayload
	raw := []byte(`{
		"composition": "jpeg_raw",
		"stack_membership": "stacked",
		"stack_kinds": ["burst", "manual"]
	}`)
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("decode filter replay payload: %v", err)
	}

	if payload.Composition != "jpeg_raw" {
		t.Fatalf("composition = %q, want jpeg_raw", payload.Composition)
	}
	if payload.StackMembership != "stacked" {
		t.Fatalf("stack_membership = %q, want stacked", payload.StackMembership)
	}
	if len(payload.StackKinds) != 2 {
		t.Fatalf("stack_kinds = %v, want two kinds", payload.StackKinds)
	}
}

func TestFilterReplayPayloadIgnoresRetiredRawKey(t *testing.T) {
	var payload filterReplayPayload
	if err := json.Unmarshal([]byte(`{"raw": true, "type": "PHOTO"}`), &payload); err != nil {
		t.Fatalf("decode legacy filter replay payload: %v", err)
	}

	if payload.Type != "PHOTO" {
		t.Fatalf("type = %q, want PHOTO", payload.Type)
	}
	if payload.Composition != "" || payload.StackMembership != "" || len(payload.StackKinds) != 0 {
		t.Fatalf("legacy payload produced browse filters: %+v", payload)
	}
}
