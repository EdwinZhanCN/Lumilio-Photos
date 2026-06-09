package queue

import (
	"context"
	"fmt"

	"server/internal/queue/jobs"

	"github.com/riverqueue/river"
)

type ScheduleRepositoryScansArgs = jobs.ScheduleRepositoryScansArgs

// ScheduleRepositoryScansWorker fans out periodic scan jobs: it lists all
// active repositories and enqueues a ScanRepositoryArgs for each one.
type ScheduleRepositoryScansWorker struct {
	river.WorkerDefaults[ScheduleRepositoryScansArgs]

	EnqueueAll func(ctx context.Context)
}

func (w *ScheduleRepositoryScansWorker) Work(ctx context.Context, job *river.Job[ScheduleRepositoryScansArgs]) error {
	if w.EnqueueAll == nil {
		return fmt.Errorf("schedule repository scans worker missing EnqueueAll")
	}
	w.EnqueueAll(ctx)
	return nil
}
