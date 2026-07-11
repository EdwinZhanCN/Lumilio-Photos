package queue

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/riverqueue/river"

	"server/internal/queue/jobs"
	"server/internal/service"
)

type DetectStacksArgs = jobs.DetectStacksArgs

// DetectStacksWorker merges structural components and detects burst stacks.
type DetectStacksWorker struct {
	river.WorkerDefaults[DetectStacksArgs]
	StackService service.StackService
}

func (w *DetectStacksWorker) Work(ctx context.Context, job *river.Job[DetectStacksArgs]) error {
	if w.StackService == nil {
		return fmt.Errorf("detect stacks worker not configured")
	}

	repoID, err := uuid.Parse(job.Args.RepositoryID)
	if err != nil {
		return fmt.Errorf("parse repository ID: %w", err)
	}

	created, err := w.StackService.AutoDetectStacks(ctx, repoID)
	if err != nil {
		return fmt.Errorf("detect stacks for repository %s: %w", repoID, err)
	}

	_ = created
	return nil
}
