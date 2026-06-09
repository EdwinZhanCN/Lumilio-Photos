package queue

import (
	"context"
	"testing"

	"server/config"
)

type staticMLConfigProvider struct {
	cfg config.MLConfig
}

func (p staticMLConfigProvider) GetEffectiveMLConfig(context.Context) (config.MLConfig, error) {
	return p.cfg, nil
}

func TestZeroshotClassifyFollowsSemanticEnabled(t *testing.T) {
	enabled, err := isMLTaskEnabled(context.Background(), staticMLConfigProvider{
		cfg: config.MLConfig{
			SemanticEnabled:         true,
			ZeroshotClassifyEnabled: false,
		},
	}, "classify_zeroshot")
	if err != nil {
		t.Fatalf("isMLTaskEnabled returned error: %v", err)
	}
	if !enabled {
		t.Fatal("expected zero-shot classification to follow semantic_enabled")
	}

	disabled, err := isMLTaskEnabled(context.Background(), staticMLConfigProvider{
		cfg: config.MLConfig{
			SemanticEnabled:         false,
			ZeroshotClassifyEnabled: true,
		},
	}, "classify_zeroshot")
	if err != nil {
		t.Fatalf("isMLTaskEnabled returned error: %v", err)
	}
	if disabled {
		t.Fatal("expected zero-shot classification to be disabled with semantic_enabled=false")
	}
}
