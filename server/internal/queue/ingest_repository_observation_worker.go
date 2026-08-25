package queue

import (
	"context"
	"fmt"
	"time"

	"github.com/riverqueue/river"

	"server/internal/queue/jobs"
)

type ObserveRepositoryArgs = jobs.ObserveRepositoryArgs

type ObserveRepositoryWorker struct {
	river.WorkerDefaults[ObserveRepositoryArgs]
	Process func(context.Context, ObserveRepositoryArgs) (bool, time.Duration, error)
}

func (w *ObserveRepositoryWorker) Work(ctx context.Context, job *river.Job[ObserveRepositoryArgs]) error {
	if w.Process == nil {
		return fmt.Errorf("repository observation worker not configured")
	}
	hasMore, delay, err := w.Process(ctx, job.Args)
	if err != nil {
		return err
	}
	if hasMore {
		return river.JobSnooze(delay)
	}
	return nil
}

type HashRepositoryNodeArgs = jobs.HashRepositoryNodeArgs

type HashRepositoryNodeWorker struct {
	river.WorkerDefaults[HashRepositoryNodeArgs]
	Process func(context.Context, HashRepositoryNodeArgs) error
}

func (w *HashRepositoryNodeWorker) Work(ctx context.Context, job *river.Job[HashRepositoryNodeArgs]) error {
	if w.Process == nil {
		return fmt.Errorf("repository hash worker not configured")
	}
	return w.Process(ctx, job.Args)
}

type DrainRepositoryOutboxArgs = jobs.DrainRepositoryOutboxArgs

type DrainRepositoryOutboxWorker struct {
	river.WorkerDefaults[DrainRepositoryOutboxArgs]
	Drain func(context.Context, DrainRepositoryOutboxArgs) (bool, error)
}

func (w *DrainRepositoryOutboxWorker) Work(ctx context.Context, job *river.Job[DrainRepositoryOutboxArgs]) error {
	if w.Drain == nil {
		return fmt.Errorf("repository outbox worker not configured")
	}
	hasMore, err := w.Drain(ctx, job.Args)
	if err != nil {
		return err
	}
	if hasMore {
		// Keep one active unique job and yield the SQLite writer between bounded
		// outbox pages instead of inserting followers while this job is running.
		return river.JobSnooze(25 * time.Millisecond)
	}
	return nil
}
