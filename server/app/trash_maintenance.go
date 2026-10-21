package app

import (
	"context"
	"time"

	"go.uber.org/zap"

	"server/internal/storage/recovery"
	"server/internal/storage/trash"
)

// trashMaintenanceInterval paces the repository trash upkeep. Retention is
// counted in days, so an hourly pass expires files within an hour of their
// deadline.
const trashMaintenanceInterval = time.Hour

// runTrashMaintenanceLoop keeps every repository trash consistent with the
// catalog: it retries recovery of moves whose repository was offline at
// startup, adopts trashed files known only from their sidecars (held, as
// inherited Trash) together with the rest of private recovery, and deletes
// permanently the unheld files past retention. Holds precede expiry in every
// pass. It runs once at start, then hourly.
func runTrashMaintenanceLoop(ctx context.Context, assetTrash *trash.Service, privateRecovery *recovery.Service, retention time.Duration, logger *zap.Logger) {
	pass := func() {
		if err := assetTrash.Recover(ctx); err != nil && ctx.Err() == nil {
			logger.Warn("interrupted trash operations wait for their repositories", zap.Error(err))
		}
		if reconciled, err := privateRecovery.ReconcileAll(ctx); err != nil && ctx.Err() == nil {
			logger.Warn("private recovery pass", zap.Error(err))
		} else if reconciled.TrashAdopted > 0 || reconciled.StudioReassociated > 0 || reconciled.StudioAmbiguous > 0 {
			logger.Info("private recovery pass",
				zap.Int("trash_adopted", reconciled.TrashAdopted), zap.Int("studio_reassociated", reconciled.StudioReassociated), zap.Int("studio_for_review", reconciled.StudioAmbiguous))
		}
		if expired, err := assetTrash.Expire(ctx, retention); err != nil && ctx.Err() == nil {
			logger.Warn("expire trashed files", zap.Error(err))
		} else if expired.Files > 0 || expired.Entries > 0 {
			logger.Info("trashed files expired",
				zap.Int("files", expired.Files), zap.Int64("bytes", expired.Bytes), zap.Int("assets", expired.Assets))
		}
	}
	pass()
	ticker := time.NewTicker(trashMaintenanceInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pass()
		}
	}
}
