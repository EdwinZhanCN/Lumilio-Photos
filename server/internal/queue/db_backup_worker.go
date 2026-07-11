package queue

import (
	"context"
	"fmt"

	"server/internal/queue/jobs"

	"github.com/riverqueue/river"
)

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
