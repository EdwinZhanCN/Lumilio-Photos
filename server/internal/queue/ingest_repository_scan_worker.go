package queue

import (
	"context"
	"fmt"

	"server/internal/queue/jobs"

	"github.com/riverqueue/river"
)

// ScanRepositoryArgs is the job payload alias to avoid import cycles.
type ScanRepositoryArgs = jobs.ScanRepositoryArgs

// ScanRepositoryWorker executes repository free-workspace scan tasks.
type ScanRepositoryWorker struct {
	river.WorkerDefaults[ScanRepositoryArgs]

	ProcessScan func(ctx context.Context, args ScanRepositoryArgs) error
}

func (w *ScanRepositoryWorker) Work(ctx context.Context, job *river.Job[ScanRepositoryArgs]) error {
	if w.ProcessScan == nil {
		return fmt.Errorf("scan repository worker missing processor")
	}
	return w.ProcessScan(ctx, job.Args)
}
