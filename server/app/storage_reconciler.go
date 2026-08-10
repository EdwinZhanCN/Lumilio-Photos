package app

import (
	"context"
	"time"

	"go.uber.org/zap"
)

type storageReconciler interface {
	ReconcileRepositoryRoots(context.Context) error
	ReconcileAll(context.Context) error
}

// startStorageReconciler keeps reachability facts current even when nobody has
// the Manage page open. Mount notifications are platform-specific and lossy;
// this low-frequency poll is the portable correctness backstop.
func startStorageReconciler(ctx context.Context, manager storageReconciler, interval time.Duration, logger *zap.Logger) {
	if manager == nil {
		return
	}
	if interval <= 0 {
		interval = time.Minute
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := manager.ReconcileRepositoryRoots(ctx); err != nil {
					logger.Warn("background Storage Location reconciliation failed", zap.Error(err))
					continue
				}
				if err := manager.ReconcileAll(ctx); err != nil {
					logger.Warn("background repository reconciliation failed", zap.Error(err))
				}
			}
		}
	}()
}
