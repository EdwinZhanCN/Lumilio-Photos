package processors

import (
	"testing"

	"server/internal/settings"
)

func TestEffectiveVideoSamplingDefaults(t *testing.T) {
	t.Parallel()

	cfg := settings.ML{}
	if cfg.EffectiveVideoMaxFrames() != settings.DefaultVideoMaxFrames {
		t.Fatalf("max frames = %d", cfg.EffectiveVideoMaxFrames())
	}
	if cfg.EffectiveVideoLongThresholdSeconds() != settings.DefaultVideoLongThresholdSeconds {
		t.Fatalf("long threshold = %d", cfg.EffectiveVideoLongThresholdSeconds())
	}
	if cfg.EffectiveVideoSceneThreshold() != settings.DefaultVideoSceneThreshold {
		t.Fatalf("scene threshold = %v", cfg.EffectiveVideoSceneThreshold())
	}
}
