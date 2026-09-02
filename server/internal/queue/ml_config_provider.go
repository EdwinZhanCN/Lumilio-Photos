package queue

import (
	"context"

	"server/internal/settings"
)

type MLConfigProvider interface {
	GetEffectiveMLConfig(ctx context.Context) (settings.ML, error)
}
