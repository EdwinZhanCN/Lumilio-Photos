package pins

import (
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
