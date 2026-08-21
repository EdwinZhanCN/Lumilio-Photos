package queue

import (
	"context"
	"fmt"
	"time"

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

// Timeout disables River's fixed job deadline because scan duration depends on
// repository size. River still cancels the context when the client shuts down.
func (w *ScanRepositoryWorker) Timeout(*river.Job[ScanRepositoryArgs]) time.Duration {
	return -1
}

func (w *ScanRepositoryWorker) Work(ctx context.Context, job *river.Job[ScanRepositoryArgs]) error {
	if w.ProcessScan == nil {
		return fmt.Errorf("scan repository worker missing processor")
	}
	return w.ProcessScan(ctx, job.Args)
}
