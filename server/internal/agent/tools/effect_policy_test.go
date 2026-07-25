package tools

import (
	"testing"

	"server/internal/agent/core"
)

func TestEveryMutationUsesConfirmedVersionedEffectPolicy(t *testing.T) {
	RegisterAll()
	for _, name := range []string{
		"bulk_like_assets",
		"tag_assets",
		"create_album",
		"add_to_album",
	} {
		policy, ok := core.GetRegistry().EffectPolicy(name)
		if !ok {
			t.Fatalf("%s has no effect policy", name)
		}
		if !policy.Confirmation ||
			policy.MaxCardinality <= 0 ||
			policy.Idempotency == "" ||
			policy.Authorization == "" ||
			policy.PolicyVersion != core.CurrentAgentPolicyVersion {
			t.Errorf("%s has incomplete effect policy: %+v", name, policy)
		}
	}
}
