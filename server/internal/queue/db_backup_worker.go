package queue

import (
	"context"
	"fmt"
	"time"

	"server/internal/queue/jobs"

	"github.com/riverqueue/river"
)

const DatabaseBackupTimeout = 30 * time.Minute

type DatabaseBackupArgs = jobs.DatabaseBackupArgs

// DatabaseBackupWorker runs one backup-scheduler tick (see
// server/internal/db/backup.Scheduler): decide due-ness from runtime settings,
// dump, prune. Skips return nil so River only retries real failures.
type DatabaseBackupWorker struct {
	river.WorkerDefaults[DatabaseBackupArgs]

	Run func(ctx context.Context, force bool) error
}

func (w *DatabaseBackupWorker) Work(ctx context.Context, job *river.Job[DatabaseBackupArgs]) error {
	if w.Run == nil {
		return fmt.Errorf("database backup worker missing Run")
	}
	return w.Run(ctx, job.Args.Force)
}

// Timeout is explicit because backup duration scales with catalog size and
// filesystem throughput. The Online Backup implementation independently pins
// a bounded read snapshot so this cap is not used to hide write-starvation.
func (*DatabaseBackupWorker) Timeout(*river.Job[DatabaseBackupArgs]) time.Duration {
	return DatabaseBackupTimeout
}
