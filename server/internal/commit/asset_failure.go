package commit

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"server/internal/pipeline"
)

// AssetStageFailure records one failed execution with a CAS against the
// previously read failure count. Replaying an acknowledgement cannot consume
// another attempt, and stale generations cannot affect a replacement request.
type AssetStageFailure struct {
	Identity         pipeline.AssetExecutionIdentity
	PreviousFailures int
	UnsupportedMedia bool
}

func (c *Coordinator) ApplyAssetFailure(ctx context.Context, payload AssetStageFailure) (Result, error) {
	identity := payload.Identity
	if err := validateAssetStage(AssetStageApplied{AssetID: identity.AssetID, SourceFence: identity.SourceFence, Stage: string(identity.Stage), PipelineVersion: identity.PipelineVersion, DesiredVersion: identity.DesiredVersion}); err != nil {
		return Result{}, err
	}
	if payload.PreviousFailures < 0 || payload.PreviousFailures >= pipeline.AssetFailureLimit {
		return Result{}, errors.New("invalid asset failure count")
	}
	return c.submitOutcome(ctx, OperationKindCatalogAssetStage, func(ctx context.Context, tx *sql.Tx) (Outcome, error) {
		return applyAssetFailure(ctx, tx, payload, time.Now().UTC())
	})
}

func applyAssetFailure(ctx context.Context, tx *sql.Tx, payload AssetStageFailure, now time.Time) (Outcome, error) {
	identity := payload.Identity
	state, err := pipeline.ReadAssetExecutionState(ctx, tx, identity)
	if err != nil {
		return 0, err
	}
	if !state.Current {
		return OutcomeStale, nil
	}
	if state.Failures != payload.PreviousFailures {
		return OutcomeDuplicate, nil
	}
	failures := state.Failures + 1
	code := "processing_retry_pending"
	terminal := ""
	if payload.UnsupportedMedia {
		code = "unsupported_media"
		terminal = code
	} else if failures >= pipeline.AssetFailureLimit {
		code = "processing_retry_exhausted"
		terminal = code
	}
	_, err = tx.ExecContext(ctx, `
 INSERT INTO asset_pipeline_failures(asset_id,stage,source_content_id,pipeline_version,desired_version,failure_count,retry_after,failure_code,updated_at)
 VALUES(?,?,?,?,?,?,?,?,?)
 ON CONFLICT(asset_id,stage) DO UPDATE SET source_content_id=excluded.source_content_id,pipeline_version=excluded.pipeline_version,
 desired_version=excluded.desired_version,failure_count=excluded.failure_count,retry_after=excluded.retry_after,failure_code=excluded.failure_code,updated_at=excluded.updated_at`,
		identity.AssetID.String(), string(identity.Stage), identity.SourceFence.String(), identity.PipelineVersion, identity.DesiredVersion, failures, now.Add(pipeline.AssetFailureDelay(failures)).UnixMicro(), code, now.UnixMicro())
	if err != nil {
		return 0, err
	}
	if terminal != "" {
		return applyAssetStages(ctx, tx, AssetStageApplied{AssetID: identity.AssetID, SourceFence: identity.SourceFence, Stage: string(identity.Stage), PipelineVersion: identity.PipelineVersion, DesiredVersion: identity.DesiredVersion, TerminalError: terminal})
	}
	return OutcomeApplied, nil
}
