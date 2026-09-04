package app

import (
	"context"
	"time"

	"go.uber.org/zap"
)

// runRepositoryVerifierLoop performs an authoritative sweep at startup and on
// every cadence tick. Catalog failures are isolated to one sweep: catalog truth
// remains authoritative and the next tick retries without relying on River or
// native watcher state.
func runRepositoryVerifierLoop(
	ctx context.Context,
	ticks <-chan time.Time,
	verify func(context.Context) error,
	logger *zap.Logger,
) {
	if logger == nil {
		logger = zap.NewNop()
	}
	run := func() {
		if err := verify(ctx); err != nil && ctx.Err() == nil {
			logger.Warn("periodic authoritative repository verification failed", zap.Error(err))
		}
	}
	if ctx.Err() != nil {
		return
	}
	run()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticks:
			run()
		}
	}
}
