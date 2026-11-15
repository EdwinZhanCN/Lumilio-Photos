package queue

import (
	"context"

	"server/internal/foundation/settings"
)

type MLConfigProvider interface {
	GetEffectiveMLConfig(ctx context.Context) (settings.ML, error)
}
