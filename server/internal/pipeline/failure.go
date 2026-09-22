package pipeline

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
)

// AssetFailureLimit counts failed domain executions, independently of River
// attempts. Exhaustion means user action is required, not that media is corrupt.
const AssetFailureLimit = 5

var ErrUnsupportedMedia = errors.New("unsupported media type")

type AssetExecutionIdentity struct {
	AssetID, SourceFence uuid.UUID
	Stage                Stage
	PipelineVersion      string
	DesiredVersion       uint64
}

type AssetExecutionState struct {
	Current    bool
	Failures   int
	RetryAfter time.Time
}

// ReadAssetExecutionState also guards old River rows after a terminal outcome,
// a successful commit, or an explicit request for a newer generation.
func ReadAssetExecutionState(ctx context.Context, db Queryer, identity AssetExecutionIdentity) (AssetExecutionState, error) {
	var state AssetExecutionState
	var retryAfter int64
	err := db.QueryRowContext(ctx, `
 SELECT COALESCE(f.failure_count,0),COALESCE(f.retry_after,0)
 FROM asset_pipeline_state s
 JOIN assets a ON a.asset_id=s.asset_id AND a.content_id=s.source_content_id AND a.is_deleted=0
 LEFT JOIN asset_pipeline_failures f ON f.asset_id=s.asset_id AND f.stage=s.stage
   AND f.source_content_id=s.source_content_id AND f.pipeline_version=s.pipeline_version AND f.desired_version=s.desired_version
 WHERE s.asset_id=? AND s.stage=? AND s.source_content_id=? AND s.pipeline_version=?
   AND s.desired_version=? AND s.applied_version<s.desired_version AND s.terminal_error IS NULL`,
		identity.AssetID.String(), string(identity.Stage), identity.SourceFence.String(), identity.PipelineVersion, identity.DesiredVersion).Scan(&state.Failures, &retryAfter)
	if errors.Is(err, sql.ErrNoRows) {
		return state, nil
	}
	if err != nil {
		return state, err
	}
	state.Current = true
	if retryAfter > 0 {
		state.RetryAfter = time.UnixMicro(retryAfter)
	}
	return state, nil
}

func AssetFailureDelay(failures int) time.Duration {
	if failures < 1 {
		failures = 1
	}
	if failures > AssetFailureLimit {
		failures = AssetFailureLimit
	}
	return 15 * time.Second * time.Duration(1<<(failures-1))
}
