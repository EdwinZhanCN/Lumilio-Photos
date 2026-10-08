package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/riverqueue/river"
	"go.uber.org/zap"
	"server/internal/commit"
	"server/internal/pipeline"
	"server/internal/queue/jobs"
	"server/internal/workqos"
)

func guardAssetExecution[T any](runtime *pipelineRuntime, execute func(context.Context, workqos.Class, T) error) func(context.Context, workqos.Class, T) error {
	return func(ctx context.Context, qos workqos.Class, args T) error {
		identity, err := assetExecutionIdentity(args)
		if err != nil {
			return err
		}
		state, err := pipeline.ReadAssetExecutionState(ctx, runtime.pipelineReader, identity)
		if err != nil || !state.Current {
			return err
		}
		ready, err := pipeline.AssetStageReady(ctx, runtime.pipelineReader, identity.AssetID, identity.SourceFence, identity.Stage, identity.DesiredVersion, identity.PipelineVersion)
		if err != nil || !ready {
			return err
		}
		if delay := time.Until(state.RetryAfter); delay > 0 {
			return river.JobSnooze(delay)
		}
		err = execute(ctx, qos, args)
		if err == nil || ctx.Err() != nil || errors.Is(err, context.Canceled) || commit.IsUnacknowledged(err) {
			return err
		}
		// A committed failure is a completed delivery. Catalog owns the cooldown,
		// bounded execution budget, and eventual terminal product outcome.
		result, commitErr := runtime.commits.ApplyAssetFailure(ctx, commit.AssetStageFailure{Identity: identity, PreviousFailures: state.Failures, UnsupportedMedia: errors.Is(err, pipeline.ErrUnsupportedMedia)})
		if commitErr != nil {
			return errors.Join(err, commitErr)
		}
		if result.Outcome == commit.OutcomeApplied && runtime.logger != nil {
			runtime.logger.Warn("asset processing failure recorded",
				zap.String("asset_id", identity.AssetID.String()), zap.String("stage", string(identity.Stage)),
				zap.Uint64("desired_version", identity.DesiredVersion), zap.Int("failure_count", state.Failures+1), zap.Error(err))
		}
		return nil
	}
}

func assetExecutionIdentity(args any) (pipeline.AssetExecutionIdentity, error) {
	var identity pipeline.AssetExecutionIdentity
	switch args := args.(type) {
	case jobs.AnalyzeAssetArgs:
		identity = pipeline.AssetExecutionIdentity{AssetID: args.AssetID, SourceFence: args.SourceFence, Stage: pipeline.StageAnalyze, PipelineVersion: args.PipelineVersion, DesiredVersion: args.DesiredVersion}
	case jobs.GenerateAssetDerivativesArgs:
		identity = pipeline.AssetExecutionIdentity{AssetID: args.AssetID, SourceFence: args.SourceFence, Stage: pipeline.StageDerivatives, PipelineVersion: args.PipelineVersion, DesiredVersion: args.DesiredVersion}
	case jobs.TranscodeMediaArgs:
		identity = pipeline.AssetExecutionIdentity{AssetID: args.AssetID, SourceFence: args.SourceFence, Stage: pipeline.StageTranscode, PipelineVersion: args.PipelineVersion, DesiredVersion: args.DesiredVersion}
	case jobs.EnrichAssetArgs:
		identity = pipeline.AssetExecutionIdentity{AssetID: args.AssetID, SourceFence: args.SourceFence, Stage: pipeline.StageEnrich, PipelineVersion: args.PipelineVersion, DesiredVersion: args.DesiredVersion}
	default:
		return identity, fmt.Errorf("unsupported asset execution arguments %T", args)
	}
	return identity, nil
}
