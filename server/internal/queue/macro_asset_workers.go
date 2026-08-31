package queue

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/riverqueue/river"

	"server/internal/queue/jobs"
)

type IngestMacroWorker struct {
	river.WorkerDefaults[jobs.IngestAssetArgs]
	Execute func(context.Context, jobs.IngestAssetArgs) error
}

func (*IngestMacroWorker) Timeout(*river.Job[jobs.IngestAssetArgs]) time.Duration {
	return 30 * time.Minute
}

func (w *IngestMacroWorker) Work(ctx context.Context, job *river.Job[jobs.IngestAssetArgs]) error {
	if w.Execute == nil {
		return errors.New("ingest macro worker is not configured")
	}
	if job.Args.ReceiptID == uuid.Nil || job.Args.CommitID == uuid.Nil {
		return errors.New("ingest macro job has an invalid receipt fence")
	}
	return w.Execute(ctx, job.Args)
}

type assetMacroWorker[T any] struct {
	execute  func(context.Context, T) error
	identify func(T) (uuid.UUID, uuid.UUID, uint64, string)
}

func (w *assetMacroWorker[T]) work(ctx context.Context, args T) error {
	if w.execute == nil || w.identify == nil {
		return errors.New("asset macro worker is not configured")
	}
	subject, fence, desired, pipelineVersion := w.identify(args)
	if subject == uuid.Nil || fence == uuid.Nil || desired == 0 || pipelineVersion == "" {
		return errors.New("asset macro job has an invalid catalog fence")
	}
	return w.execute(ctx, args)
}

type AnalyzeAssetWorker struct {
	river.WorkerDefaults[jobs.AnalyzeAssetArgs]
	worker assetMacroWorker[jobs.AnalyzeAssetArgs]
}

func (*AnalyzeAssetWorker) Timeout(*river.Job[jobs.AnalyzeAssetArgs]) time.Duration {
	return 10 * time.Minute
}

func NewAnalyzeAssetWorker(execute func(context.Context, jobs.AnalyzeAssetArgs) error) *AnalyzeAssetWorker {
	return &AnalyzeAssetWorker{worker: assetMacroWorker[jobs.AnalyzeAssetArgs]{execute: execute, identify: func(a jobs.AnalyzeAssetArgs) (uuid.UUID, uuid.UUID, uint64, string) {
		return a.AssetID, a.SourceFence, a.DesiredVersion, a.PipelineVersion
	}}}
}
func (w *AnalyzeAssetWorker) Work(ctx context.Context, job *river.Job[jobs.AnalyzeAssetArgs]) error {
	return w.worker.work(ctx, job.Args)
}

type GenerateAssetDerivativesWorker struct {
	river.WorkerDefaults[jobs.GenerateAssetDerivativesArgs]
	worker assetMacroWorker[jobs.GenerateAssetDerivativesArgs]
}

func (*GenerateAssetDerivativesWorker) Timeout(*river.Job[jobs.GenerateAssetDerivativesArgs]) time.Duration {
	return 30 * time.Minute
}

func NewGenerateAssetDerivativesWorker(execute func(context.Context, jobs.GenerateAssetDerivativesArgs) error) *GenerateAssetDerivativesWorker {
	return &GenerateAssetDerivativesWorker{worker: assetMacroWorker[jobs.GenerateAssetDerivativesArgs]{execute: execute, identify: func(a jobs.GenerateAssetDerivativesArgs) (uuid.UUID, uuid.UUID, uint64, string) {
		return a.AssetID, a.SourceFence, a.DesiredVersion, a.PipelineVersion
	}}}
}
func (w *GenerateAssetDerivativesWorker) Work(ctx context.Context, job *river.Job[jobs.GenerateAssetDerivativesArgs]) error {
	return w.worker.work(ctx, job.Args)
}

type TranscodeMediaWorker struct {
	river.WorkerDefaults[jobs.TranscodeMediaArgs]
	worker assetMacroWorker[jobs.TranscodeMediaArgs]
}

func (*TranscodeMediaWorker) Timeout(*river.Job[jobs.TranscodeMediaArgs]) time.Duration {
	return 2 * time.Hour
}

func NewTranscodeMediaWorker(execute func(context.Context, jobs.TranscodeMediaArgs) error) *TranscodeMediaWorker {
	return &TranscodeMediaWorker{worker: assetMacroWorker[jobs.TranscodeMediaArgs]{execute: execute, identify: func(a jobs.TranscodeMediaArgs) (uuid.UUID, uuid.UUID, uint64, string) {
		return a.AssetID, a.SourceFence, a.DesiredVersion, a.PipelineVersion
	}}}
}
func (w *TranscodeMediaWorker) Work(ctx context.Context, job *river.Job[jobs.TranscodeMediaArgs]) error {
	return w.worker.work(ctx, job.Args)
}
