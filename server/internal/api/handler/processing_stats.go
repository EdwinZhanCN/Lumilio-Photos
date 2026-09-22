package handler

import (
	"context"
	"time"
)

// ProcessingStatsResponse reports current Catalog work, not delivery history.
// A file with any terminal stage is counted in FailedAssets instead of
// PendingAssets, even if other stages still await a prerequisite.
type ProcessingStatsResponse struct {
	PendingAssets       int64 `json:"pending_assets"`
	FailedAssets        int64 `json:"failed_assets"`
	RetryWaitingStages  int64 `json:"retry_waiting_stages"`
	PendingRepositories int64 `json:"pending_repositories"`
	FailedRepositories  int64 `json:"failed_repositories"`
	PendingProjections  int64 `json:"pending_projections"`
	FailedProjections   int64 `json:"failed_projections"`
	PendingOperations   int64 `json:"pending_operations"`
	FailedOperations    int64 `json:"failed_operations"`
}

func (h *QueueHandler) loadProcessingStats(ctx context.Context) (ProcessingStatsResponse, error) {
	var stats ProcessingStatsResponse
	err := h.catalog.QueryRowContext(ctx, `
 WITH asset_work AS (
  SELECT s.asset_id,MAX(s.terminal_error IS NOT NULL) AS failed
  FROM asset_pipeline_state s JOIN assets a ON a.asset_id=s.asset_id
  WHERE s.desired_version>s.applied_version AND a.is_deleted=0 GROUP BY s.asset_id
 )
 SELECT COALESCE(SUM(failed=0),0),COALESCE(SUM(failed=1),0) FROM asset_work`).Scan(&stats.PendingAssets, &stats.FailedAssets)
	if err != nil {
		return stats, err
	}
	err = h.catalog.QueryRowContext(ctx, `
 SELECT COUNT(*) FROM asset_pipeline_state s JOIN asset_pipeline_failures f
 ON f.asset_id=s.asset_id AND f.stage=s.stage AND f.source_content_id=s.source_content_id
 AND f.pipeline_version=s.pipeline_version AND f.desired_version=s.desired_version
 JOIN assets a ON a.asset_id=s.asset_id
 WHERE s.desired_version>s.applied_version AND s.terminal_error IS NULL AND a.is_deleted=0 AND f.retry_after>?`, time.Now().UTC().UnixMicro()).Scan(&stats.RetryWaitingStages)
	if err != nil {
		return stats, err
	}
	err = h.catalog.QueryRowContext(ctx, `
 SELECT COALESCE(SUM(terminal_error IS NULL),0),COALESCE(SUM(terminal_error IS NOT NULL),0)
 FROM repository_observation_state WHERE desired_epoch>applied_epoch`).Scan(&stats.PendingRepositories, &stats.FailedRepositories)
	if err != nil {
		return stats, err
	}
	err = h.catalog.QueryRowContext(ctx, `
 WITH projection_work AS (
  SELECT terminal_error FROM event_projection_pipeline_state WHERE source_revision>applied_revision
  UNION ALL SELECT terminal_error FROM location_resolution_pipeline_state WHERE projection_version>applied_revision
  UNION ALL SELECT terminal_error FROM ocr_projection_pipeline_state WHERE projection_version>applied_revision
  UNION ALL SELECT terminal_error FROM location_projection_state WHERE source_revision>published_revision
 )
 SELECT COALESCE(SUM(terminal_error IS NULL),0),COALESCE(SUM(terminal_error IS NOT NULL),0) FROM projection_work`).Scan(&stats.PendingProjections, &stats.FailedProjections)
	if err != nil {
		return stats, err
	}
	err = h.catalog.QueryRowContext(ctx, `
 WITH latest_operations AS (
  SELECT state,ROW_NUMBER() OVER(PARTITION BY kind,subject_id ORDER BY created_at DESC,receipt_id DESC) AS position
  FROM catalog_operation_receipts WHERE kind IN ('ingest','reindex','backup')
 )
 SELECT COALESCE(SUM(state='pending'),0),COALESCE(SUM(state='failed'),0)
 FROM latest_operations WHERE position=1`).Scan(&stats.PendingOperations, &stats.FailedOperations)
	return stats, err
}
