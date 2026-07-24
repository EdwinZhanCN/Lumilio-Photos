package queue

import (
	"context"

	"server/internal/settings"
)

type MLConfigProvider interface {
	GetEffectiveMLConfig(ctx context.Context) (settings.ML, error)
}

func isMLTaskEnabled(ctx context.Context, provider MLConfigProvider, queueName string) (bool, error) {
	if provider == nil {
		return true, nil
	}

	cfg, err := provider.GetEffectiveMLConfig(ctx)
	if err != nil {
		return false, err
	}

	switch queueName {
	case "process_semantic":
		return cfg.SemanticEnabled, nil
	case "process_bioclip":
		return cfg.BioCLIPEnabled, nil
	case "process_ocr":
		return cfg.OCREnabled, nil
	case "process_face":
		return cfg.FaceEnabled, nil
	case "process_video_frames":
		return cfg.SemanticEnabled && cfg.VideoSemanticEnabled, nil
	case "classify_zeroshot":
		return cfg.SemanticEnabled, nil
	default:
		return false, nil
	}
}
