package app

import (
	"context"
	"math/rand"
	"time"

	"go.uber.org/zap"

	"server/internal/storage/scan"
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

// jitteredTicks delivers one tick per periodic interval, each delay drawn
// from 3/4 to 5/4 of the interval so repositories do not scan in lockstep.
func jitteredTicks(ctx context.Context, interval time.Duration) <-chan time.Time {
	ticks := make(chan time.Time)
	random := rand.New(rand.NewSource(time.Now().UnixNano()))
	go func() {
		for {
			timer := time.NewTimer(scan.PeriodicDelay(interval, random))
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case now := <-timer.C:
				select {
				case ticks <- now:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	return ticks
}
