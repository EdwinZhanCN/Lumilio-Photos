package queue

import (
	"context"

	"server/config"
)

type MLConfigProvider interface {
	GetEffectiveMLConfig(ctx context.Context) (config.MLConfig, error)
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
	case "process_clip":
		return cfg.CLIPEnabled, nil
	case "process_ocr":
		return cfg.OCREnabled, nil
	case "process_caption":
		return cfg.CaptionEnabled, nil
	case "process_face":
		return cfg.FaceEnabled, nil
	default:
		return false, nil
	}
}
