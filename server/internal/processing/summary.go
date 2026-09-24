package processing

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"

	"server/internal/queue/jobs"
)

// ImportFailureWindow bounds how long a failed import stays on the Import
// card. The receipt itself is kept; an upload cannot be retried server-side,
// so an old failure would otherwise hold the card in attention forever.
const ImportFailureWindow = 7 * 24 * time.Hour

// DoneRefreshInterval bounds how often per-stage done counts are recomputed.
// They need a full pass over the pipeline table, change slowly, and are a
// detail fact, so every other count reads only the pending partial index.
const DoneRefreshInterval = time.Minute

// StageSummary is one stage card. Every stage has the same shape; Done and
// Sources are present only for per-file pipeline stages.
type StageSummary struct {
	ID             StageID          `json:"id"`
	Group          Group            `json:"group"`
	Unit           Unit             `json:"unit"`
	Status         Status           `json:"status"`
	Remaining      int64            `json:"remaining"`
	Queued         int64            `json:"queued"`
	Running        int64            `json:"running"`
	Retrying       int64            `json:"retrying"`
	Failed         int64            `json:"failed"`
	Done           *int64           `json:"done,omitempty"`
	Sources        map[string]int64 `json:"sources,omitempty"`
	OldestQueuedAt *time.Time       `json:"oldest_queued_at,omitempty"`
	LastActivityAt *time.Time       `json:"last_activity_at,omitempty"`
	Retryable      bool             `json:"retryable"`
}

// Overview is the idle panel: totals across stages.
type Overview struct {
	MediaTotal      int64      `json:"media_total"`
	MediaInProgress int64      `json:"media_in_progress"`
	Running         int64      `json:"running"`
	FailedMedia     int64      `json:"failed_media"`
	CatalogPending  int64      `json:"catalog_pending"`
	LastActivityAt  *time.Time `json:"last_activity_at,omitempty"`
}

// Summary is the complete Processing snapshot. Catalog and QueueDB are read
// separately, so it is not an atomic cross-database snapshot.
type Summary struct {
	GeneratedAt time.Time      `json:"generated_at"`
	Overview    Overview       `json:"overview"`
	Stages      []StageSummary `json:"stages"`
}

// Reader computes the Processing read model from query-only pools.
type Reader struct {
	catalog *sql.DB
	queue   *sql.DB
	now     func() time.Time

	doneMu sync.Mutex
	done   map[string]int64
	doneAt time.Time
}

// NewReader binds the Catalog and QueueDB reader pools.
func NewReader(catalog, queue *sql.DB) *Reader {
	return &Reader{catalog: catalog, queue: queue, now: time.Now}
}

type catalogCounts struct {
	pending  int64
	failed   int64
	retrying int64
	done     *int64
	sources  map[string]int64
	oldest   *time.Time
}

type deliveryFacts struct {
	running      int64
	retryable    int64
	lastActivity *time.Time
}

// Summary reads every stage and the overview.
func (r *Reader) Summary(ctx context.Context) (Summary, error) {
	if r == nil || r.catalog == nil {
		return Summary{}, errors.New("processing reader is not configured")
	}
	now := r.now().UTC()
	counts, err := r.catalogCounts(ctx, now)
	if err != nil {
		return Summary{}, err
	}
	deliveries, err := r.deliveryFacts(ctx)
	if err != nil {
		return Summary{}, err
	}
	summary := Summary{GeneratedAt: now, Stages: make([]StageSummary, 0, len(Stages))}
	for _, spec := range Stages {
		c := counts[spec.ID]
		d := deliveries[spec.ID]
		stage := StageSummary{
			ID: spec.ID, Group: spec.Group, Unit: spec.Unit,
			Failed: c.failed, Running: d.running, Done: c.done, Sources: c.sources,
			OldestQueuedAt: c.oldest, LastActivityAt: d.lastActivity, Retryable: spec.Retryable,
		}
		// Per-file stages own a Catalog retry schedule; every other stage only
		// has River's retryable deliveries as its retry fact.
		if spec.AssetStage != "" {
			stage.Retrying = c.retrying
		} else {
			stage.Retrying = d.retryable
		}
		stage.Queued = max(0, c.pending-stage.Running-stage.Retrying)
		stage.Remaining = stage.Queued + stage.Running + stage.Retrying
		stage.Status = deriveStatus(stage)
		summary.Stages = append(summary.Stages, stage)

		summary.Overview.Running += stage.Running
		if spec.Group == GroupCatalog {
			summary.Overview.CatalogPending += stage.Remaining
		}
		if d.lastActivity != nil && (summary.Overview.LastActivityAt == nil || d.lastActivity.After(*summary.Overview.LastActivityAt)) {
			summary.Overview.LastActivityAt = d.lastActivity
		}
	}
	if err := r.catalog.QueryRowContext(ctx, `
 WITH asset_work AS (
  SELECT s.asset_id, MAX(s.terminal_error IS NOT NULL) AS failed
  FROM asset_pipeline_state s INDEXED BY idx_asset_pipeline_pending
  JOIN assets a ON a.asset_id=s.asset_id
  WHERE s.desired_version>s.applied_version AND a.is_deleted=0
  GROUP BY s.asset_id
 )
 SELECT (SELECT count(*) FROM assets WHERE is_deleted=0),
  COALESCE(SUM(failed=0),0), COALESCE(SUM(failed=1),0) FROM asset_work`).Scan(
		&summary.Overview.MediaTotal, &summary.Overview.MediaInProgress, &summary.Overview.FailedMedia); err != nil {
		return Summary{}, fmt.Errorf("read media overview: %w", err)
	}
	return summary, nil
}

func deriveStatus(stage StageSummary) Status {
	switch {
	case stage.Failed > 0:
		return StatusAttention
	case stage.Running > 0:
		return StatusWorking
	case stage.Retrying > 0:
		return StatusRetrying
	case stage.Queued > 0:
		return StatusWaiting
	default:
		return StatusIdle
	}
}

func (r *Reader) catalogCounts(ctx context.Context, now time.Time) (map[StageID]*catalogCounts, error) {
	counts := make(map[StageID]*catalogCounts, len(Stages))
	for _, spec := range Stages {
		counts[spec.ID] = &catalogCounts{}
	}
	byAssetStage := make(map[string]StageID)
	for _, spec := range Stages {
		if spec.AssetStage != "" {
			byAssetStage[string(spec.AssetStage)] = spec.ID
		}
	}

	// Terminal rows stay behind their desired version, so pending and failed
	// both come from the pending partial index (idx_asset_pipeline_pending).
	rows, err := r.catalog.QueryContext(ctx, `
 SELECT s.stage,
  COALESCE(SUM(s.terminal_error IS NULL),0),
  COALESCE(SUM(s.terminal_error IS NOT NULL),0),
  COALESCE(SUM(s.terminal_error IS NULL AND f.retry_after>?),0),
  MIN(CASE WHEN s.terminal_error IS NULL THEN s.updated_at END)
 FROM asset_pipeline_state s
 JOIN assets a ON a.asset_id=s.asset_id AND a.is_deleted=0
 LEFT JOIN asset_pipeline_failures f ON f.asset_id=s.asset_id AND f.stage=s.stage
  AND f.source_content_id=s.source_content_id AND f.pipeline_version=s.pipeline_version
  AND f.desired_version=s.desired_version
 WHERE s.desired_version>s.applied_version
 GROUP BY s.stage`, now.UnixMicro())
	if err != nil {
		return nil, fmt.Errorf("read asset stage counts: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var stage string
		var pending, failed, retrying int64
		var oldest sql.NullInt64
		if err := rows.Scan(&stage, &pending, &failed, &retrying, &oldest); err != nil {
			return nil, err
		}
		id, ok := byAssetStage[stage]
		if !ok {
			continue
		}
		c := counts[id]
		c.pending, c.failed, c.retrying, c.oldest = pending, failed, retrying, microsPtr(oldest)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	done, err := r.doneCounts(ctx, now)
	if err != nil {
		return nil, err
	}
	for stage, id := range byAssetStage {
		value := done[stage]
		counts[id].done = &value
	}

	sourceRows, err := r.catalog.QueryContext(ctx, `
 SELECT s.stage, r.kind, count(DISTINCT s.asset_id)
 FROM asset_pipeline_state s
 JOIN assets a ON a.asset_id=s.asset_id AND a.is_deleted=0
 JOIN asset_pipeline_receipt_stages rs ON rs.asset_id=s.asset_id AND rs.stage=s.stage AND rs.desired_version=s.desired_version
 JOIN catalog_operation_receipts r ON r.receipt_id=rs.receipt_id
 WHERE s.desired_version>s.applied_version AND s.terminal_error IS NULL
 GROUP BY s.stage, r.kind`)
	if err != nil {
		return nil, fmt.Errorf("read asset stage sources: %w", err)
	}
	defer sourceRows.Close()
	for sourceRows.Next() {
		var stage, kind string
		var count int64
		if err := sourceRows.Scan(&stage, &kind, &count); err != nil {
			return nil, err
		}
		id, ok := byAssetStage[stage]
		if !ok {
			continue
		}
		if counts[id].sources == nil {
			counts[id].sources = map[string]int64{}
		}
		counts[id].sources[kind] = count
	}
	if err := sourceRows.Err(); err != nil {
		return nil, err
	}

	type single struct {
		id    StageID
		query string
		args  []any
	}
	singles := []single{
		{StageImport, `SELECT COALESCE(SUM(state='pending'),0), COALESCE(SUM(state='failed' AND updated_at>?),0),
		  MIN(CASE WHEN state='pending' THEN created_at END)
		  FROM catalog_operation_receipts WHERE kind='ingest'`, []any{now.Add(-ImportFailureWindow).UnixMicro()}},
		{StageScan, `SELECT COALESCE(SUM(desired_epoch>applied_epoch AND terminal_error IS NULL),0), COALESCE(SUM(terminal_error IS NOT NULL),0),
		  MIN(CASE WHEN desired_epoch>applied_epoch AND terminal_error IS NULL THEN updated_at END)
		  FROM repository_observation_state`, nil},
		{StageEvents, `SELECT COALESCE(SUM(source_revision>applied_revision AND terminal_error IS NULL),0), COALESCE(SUM(terminal_error IS NOT NULL),0),
		  MIN(CASE WHEN source_revision>applied_revision AND terminal_error IS NULL THEN updated_at END)
		  FROM event_projection_pipeline_state`, nil},
		{StagePlaces, `WITH work AS (
		   SELECT source_revision>published_revision AS pending, terminal_error, updated_at FROM location_projection_state
		   UNION ALL SELECT projection_version>applied_revision, terminal_error, updated_at FROM location_resolution_pipeline_state
		  ) SELECT COALESCE(SUM(pending AND terminal_error IS NULL),0), COALESCE(SUM(terminal_error IS NOT NULL),0),
		  MIN(CASE WHEN pending AND terminal_error IS NULL THEN updated_at END) FROM work`, nil},
		{StageTextSearch, `SELECT COALESCE(SUM(projection_version>applied_revision AND terminal_error IS NULL),0), COALESCE(SUM(terminal_error IS NOT NULL),0),
		  MIN(CASE WHEN projection_version>applied_revision AND terminal_error IS NULL THEN updated_at END)
		  FROM ocr_projection_pipeline_state`, nil},
		{StageBackup, `WITH latest AS (
		   SELECT state, created_at, ROW_NUMBER() OVER(PARTITION BY subject_id ORDER BY created_at DESC, receipt_id DESC) AS position
		   FROM catalog_operation_receipts WHERE kind='backup'
		  ) SELECT COALESCE(SUM(state='pending'),0), COALESCE(SUM(state='failed'),0),
		  MIN(CASE WHEN state='pending' THEN created_at END) FROM latest WHERE position=1`, nil},
	}
	for _, item := range singles {
		var oldest sql.NullInt64
		c := counts[item.id]
		if err := r.catalog.QueryRowContext(ctx, item.query, item.args...).Scan(&c.pending, &c.failed, &oldest); err != nil {
			return nil, fmt.Errorf("read %s counts: %w", item.id, err)
		}
		c.oldest = microsPtr(oldest)
	}
	return counts, nil
}

// doneCounts returns per-stage finished rows, recomputed at most once per
// DoneRefreshInterval.
func (r *Reader) doneCounts(ctx context.Context, now time.Time) (map[string]int64, error) {
	r.doneMu.Lock()
	defer r.doneMu.Unlock()
	if r.done != nil && now.Sub(r.doneAt) < DoneRefreshInterval {
		return r.done, nil
	}
	rows, err := r.catalog.QueryContext(ctx, `
 SELECT s.stage, count(*) FROM asset_pipeline_state s
 JOIN assets a ON a.asset_id=s.asset_id AND a.is_deleted=0
 WHERE s.desired_version=s.applied_version AND s.terminal_error IS NULL
 GROUP BY s.stage`)
	if err != nil {
		return nil, fmt.Errorf("read asset stage done counts: %w", err)
	}
	defer rows.Close()
	done := map[string]int64{}
	for rows.Next() {
		var stage string
		var count int64
		if err := rows.Scan(&stage, &count); err != nil {
			return nil, err
		}
		done[stage] = count
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	r.done, r.doneAt = done, now
	return done, nil
}

func (r *Reader) deliveryFacts(ctx context.Context) (map[StageID]deliveryFacts, error) {
	facts := make(map[StageID]deliveryFacts, len(Stages))
	if r.queue == nil {
		return facts, nil
	}
	groups, err := jobs.ReadDeliveryGroups(ctx, r.queue)
	if err != nil {
		return nil, fmt.Errorf("read delivery facts: %w", err)
	}
	for _, group := range groups {
		id, ok := stageForDelivery(group.Kind, group.ProjectionKind)
		if !ok {
			continue
		}
		f := facts[id]
		f.running += group.Running
		f.retryable += group.Retryable
		if group.LastActivityAt != nil && (f.lastActivity == nil || group.LastActivityAt.After(*f.lastActivity)) {
			f.lastActivity = group.LastActivityAt
		}
		facts[id] = f
	}
	return facts, nil
}

func microsPtr(value sql.NullInt64) *time.Time {
	if !value.Valid {
		return nil
	}
	at := time.UnixMicro(value.Int64).UTC()
	return &at
}
