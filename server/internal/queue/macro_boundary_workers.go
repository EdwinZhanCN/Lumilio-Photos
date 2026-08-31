package queue

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/riverqueue/river"

	"server/internal/queue/jobs"
)

var ErrMacroStageUnavailable = errors.New("macro stage is not configured")

type EnrichAssetWorker struct {
	river.WorkerDefaults[jobs.EnrichAssetArgs]
	Execute func(context.Context, jobs.EnrichAssetArgs) error
}

func (*EnrichAssetWorker) Timeout(*river.Job[jobs.EnrichAssetArgs]) time.Duration {
	return 2 * time.Hour
}

func (w *EnrichAssetWorker) Work(ctx context.Context, job *river.Job[jobs.EnrichAssetArgs]) error {
	if w.Execute == nil || job.Args.AssetID == uuid.Nil || job.Args.SourceFence == uuid.Nil || job.Args.DesiredVersion == 0 || job.Args.PipelineVersion == "" {
		return ErrMacroStageUnavailable
	}
	return w.Execute(ctx, job.Args)
}

type ScanRepositoryBatchWorker struct {
	river.WorkerDefaults[jobs.ScanRepositoryBatchArgs]
	Execute func(context.Context, jobs.ScanRepositoryBatchArgs) (bool, error)
}

func (*ScanRepositoryBatchWorker) Timeout(*river.Job[jobs.ScanRepositoryBatchArgs]) time.Duration {
	return 15 * time.Minute
}

func (w *ScanRepositoryBatchWorker) Work(ctx context.Context, job *river.Job[jobs.ScanRepositoryBatchArgs]) error {
	if w.Execute == nil || job.Args.RepositoryID == uuid.Nil || job.Args.RequestedEpoch == 0 || job.Args.DesiredVersion != job.Args.RequestedEpoch {
		return ErrMacroStageUnavailable
	}
	more, err := w.Execute(ctx, job.Args)
	if err != nil {
		return err
	}
	if more {
		return river.JobSnooze(25 * time.Millisecond)
	}
	return nil
}

type RebuildProjectionBatchWorker struct {
	river.WorkerDefaults[jobs.RebuildProjectionBatchArgs]
	Execute func(context.Context, jobs.RebuildProjectionBatchArgs) (ProjectionExecution, error)
}

func (*RebuildProjectionBatchWorker) Timeout(*river.Job[jobs.RebuildProjectionBatchArgs]) time.Duration {
	return 30 * time.Minute
}

// ProjectionExecution contains the delivery decision after one bounded
// projection step has already received its catalog acknowledgement.
type ProjectionExecution struct {
	More         bool
	Noop         bool
	Acknowledged bool
	Snooze       time.Duration
}

func (w *RebuildProjectionBatchWorker) Work(ctx context.Context, job *river.Job[jobs.RebuildProjectionBatchArgs]) error {
	if w.Execute == nil || job.Args.Scope == "" || job.Args.SourceRevision == 0 || job.Args.ProjectionVersion == 0 {
		return ErrMacroStageUnavailable
	}
	result, err := w.Execute(ctx, job.Args)
	if err != nil {
		return err
	}
	if result.Noop && (result.More || result.Acknowledged) {
		return ErrMacroStageUnavailable
	}
	if !result.Noop && !result.Acknowledged {
		return ErrMacroStageUnavailable
	}
	if result.More {
		delay := result.Snooze
		if delay < 25*time.Millisecond {
			delay = 25 * time.Millisecond
		}
		return river.JobSnooze(delay)
	}
	return nil
}

type BackupCatalogWorker struct {
	river.WorkerDefaults[jobs.BackupCatalogArgs]
	Execute func(context.Context, jobs.BackupCatalogArgs) error
}

func (w *BackupCatalogWorker) Timeout(*river.Job[jobs.BackupCatalogArgs]) time.Duration {
	return 30 * time.Minute
}
func (w *BackupCatalogWorker) Work(ctx context.Context, job *river.Job[jobs.BackupCatalogArgs]) error {
	if w.Execute == nil || job.Args.RequestID == uuid.Nil {
		return ErrMacroStageUnavailable
	}
	return w.Execute(ctx, job.Args)
}
