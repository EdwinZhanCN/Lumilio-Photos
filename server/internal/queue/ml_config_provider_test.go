package queue

import (
	"context"
	"testing"

	"server/internal/settings"
)

type staticMLConfigProvider struct {
	cfg settings.ML
}

func (p staticMLConfigProvider) GetEffectiveMLConfig(context.Context) (settings.ML, error) {
	return p.cfg, nil
}

func TestZeroshotClassifyFollowsSemanticEnabled(t *testing.T) {
	enabled, err := isMLTaskEnabled(context.Background(), staticMLConfigProvider{
		cfg: settings.ML{
			SemanticEnabled: true,
		},
	}, "classify_zeroshot")
	if err != nil {
		t.Fatalf("isMLTaskEnabled returned error: %v", err)
	}
	if !enabled {
		t.Fatal("expected zero-shot classification to follow semantic_enabled")
	}

	disabled, err := isMLTaskEnabled(context.Background(), staticMLConfigProvider{
		cfg: settings.ML{
			SemanticEnabled: false,
		},
	}, "classify_zeroshot")
	if err != nil {
		t.Fatalf("isMLTaskEnabled returned error: %v", err)
	}
	if disabled {
		t.Fatal("expected zero-shot classification to be disabled with semantic_enabled=false")
	}
}
