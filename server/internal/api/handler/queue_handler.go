package handler

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"time"

	"server/internal/api"
	"server/internal/db/catalogtx"
	"server/internal/processing"

	"github.com/gin-gonic/gin"
)

// QueueHandler serves the administrator Processing read model and its one
// command, stage retry. Reads use query-only pools; retry is bound separately
// to the Catalog writer.
type QueueHandler struct {
	dbpool     *sql.DB
	catalog    *sql.DB
	processing *processing.Reader
	retrier    *processing.Retrier
}

// NewQueueHandler binds the QueueDB and Catalog reader pools.
func NewQueueHandler(dbpool, catalog *sql.DB) *QueueHandler {
	return &QueueHandler{
		dbpool:     dbpool,
		catalog:    catalog,
		processing: processing.NewReader(catalog, dbpool),
	}
}

// SetRetryWriter enables stage retry through the Catalog writer.
func (h *QueueHandler) SetRetryWriter(writer *catalogtx.Writer) {
	h.retrier = processing.NewRetrier(writer)
}

// DeliveryStatsDTO contains queue execution diagnostics, not file completion counts.
type DeliveryStatsDTO struct {
	Available int64             `json:"available"`
	Scheduled int64             `json:"scheduled"`
	Running   int64             `json:"running"`
	Retryable int64             `json:"retryable"`
	Completed int64             `json:"completed"`
	Cancelled int64             `json:"cancelled"`
	Discarded int64             `json:"discarded"`
	Queues    []QueueSummaryDTO `json:"queues"`
}

// QueueSummaryDTO represents a single queue's aggregated activity.
type QueueSummaryDTO struct {
	Name              string                `json:"name"`
	TotalJobs         int64                 `json:"total_jobs"`
	ProcessedJobs     int64                 `json:"processed_jobs"`
	RemainingJobs     int64                 `json:"remaining_jobs"`
	RunningJobs       int64                 `json:"running_jobs"`
	AttentionJobs     int64                 `json:"attention_jobs"`
	AverageLatencyMs  *int64                `json:"average_latency_ms,omitempty"`
	AverageRuntimeMs  *int64                `json:"average_runtime_ms,omitempty"`
	OldestRemainingAt *time.Time            `json:"oldest_remaining_at,omitempty"`
	LatestActivityAt  *time.Time            `json:"latest_activity_at,omitempty"`
	ErrorSamples      []QueueErrorSampleDTO `json:"error_samples,omitempty"`
}

// QueueErrorSampleDTO represents a recent failed or retryable job for a queue.
type QueueErrorSampleDTO struct {
	JobID       int64      `json:"job_id"`
	Kind        string     `json:"kind"`
	State       string     `json:"state"`
	Attempt     int        `json:"attempt"`
	MaxAttempts int        `json:"max_attempts"`
	LastError   string     `json:"last_error"`
	CreatedAt   time.Time  `json:"created_at"`
	ScheduledAt time.Time  `json:"scheduled_at"`
	AttemptedAt *time.Time `json:"attempted_at,omitempty"`
	FinalizedAt *time.Time `json:"finalized_at,omitempty"`
}

// GetProcessing godoc
// @Summary Get processing stages
// @Description One card per processing stage in a declared unit, plus overall totals. Catalog facts, with River supplying only running and retryable deliveries; not an atomic cross-database snapshot.
// @Tags Queue
// @Produce json
// @Success 200 {object} processing.Summary
// @Router /api/v1/admin/processing [get]
func (h *QueueHandler) GetProcessing(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	summary, err := h.processing.Summary(ctx)
	if err != nil {
		api.WriteProblem(c, api.StatusProblem(http.StatusInternalServerError, err))
		return
	}
	api.JSONOK(c, summary)
}

// GetProcessingStageItems godoc
// @Summary List a processing stage's items
// @Description One bounded page of a stage's failed (newest first) or queued (oldest first) subjects. Failures carry reason codes, never raw errors.
// @Tags Queue
// @Produce json
// @Param stage path string true "Stage ID" Enums(import,scan,metadata,thumbnails,video,analysis,events,places,text_search,backup)
// @Param state query string true "Item state" Enums(failed,queued)
// @Param limit query int false "Page size (max 50)"
// @Param cursor query string false "Cursor from the previous page"
// @Success 200 {object} processing.ItemsPage
// @Router /api/v1/admin/processing/stages/{stage}/items [get]
func (h *QueueHandler) GetProcessingStageItems(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	state := processing.ItemState(c.Query("state"))
	if state != processing.ItemsFailed && state != processing.ItemsQueued {
		api.WriteProblem(c, api.BadRequest(errors.New("state must be failed or queued")))
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	page, err := h.processing.Items(ctx, processing.StageID(c.Param("stage")), state, limit, c.Query("cursor"))
	if errors.Is(err, processing.ErrUnknownStage) {
		api.WriteProblem(c, api.NotFound(err))
		return
	}
	if err != nil {
		api.WriteProblem(c, api.StatusProblem(http.StatusInternalServerError, err))
		return
	}
	api.JSONOK(c, page)
}

// RetryProcessingStage godoc
// @Summary Retry a processing stage's failures
// @Description Re-requests at most 500 failed subjects of a stage through the Catalog. Import, Scan, and Backup are not retryable here.
// @Tags Queue
// @Produce json
// @Param stage path string true "Stage ID" Enums(metadata,thumbnails,video,analysis,events,places,text_search)
// @Success 200 {object} processing.RetryResult
// @Router /api/v1/admin/processing/stages/{stage}/retry [post]
func (h *QueueHandler) RetryProcessingStage(c *gin.Context) {
	if h.retrier == nil {
		api.WriteProblem(c, api.StatusProblem(http.StatusServiceUnavailable, errors.New("processing retry is not configured")))
		return
	}
	result, err := h.retrier.Retry(c.Request.Context(), processing.StageID(c.Param("stage")))
	switch {
	case errors.Is(err, processing.ErrUnknownStage):
		api.WriteProblem(c, api.NotFound(err))
	case errors.Is(err, processing.ErrStageNotRetryable):
		api.WriteProblem(c, api.StatusProblem(http.StatusConflict, err))
	case err != nil:
		api.WriteProblem(c, api.StatusProblem(http.StatusInternalServerError, err))
	default:
		api.JSONOK(c, result)
	}
}

// GetProcessingDiagnostics godoc
// @Summary Get processing delivery diagnostics
// @Description Disposable River delivery totals and per-queue summaries with recent error samples. Delivery records are not file progress.
// @Tags Queue
// @Produce json
// @Param error_limit query int false "Recent error samples per queue (default: 5, max: 20)"
// @Success 200 {object} DeliveryStatsDTO
// @Router /api/v1/admin/processing/diagnostics [get]
func (h *QueueHandler) GetProcessingDiagnostics(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	queues, err := h.loadQueueSummaries(ctx)
	if err != nil {
		api.WriteProblem(c, api.StatusProblem(http.StatusInternalServerError, err))
		return
	}
	errorLimit := parseErrorLimit(c.DefaultQuery("error_limit", "5"))
	if len(queues) > 0 && errorLimit > 0 {
		if err := h.attachQueueErrorSamples(ctx, queues, errorLimit); err != nil {
			api.WriteProblem(c, api.StatusProblem(http.StatusInternalServerError, err))
			return
		}
	}
	deliveries := DeliveryStatsDTO{Queues: queues}
	err = h.dbpool.QueryRowContext(ctx, `SELECT
 COUNT(*) FILTER (WHERE state='available'), COUNT(*) FILTER (WHERE state='scheduled'),
 COUNT(*) FILTER (WHERE state='running'), COUNT(*) FILTER (WHERE state='retryable'),
 COUNT(*) FILTER (WHERE state='completed'), COUNT(*) FILTER (WHERE state='cancelled'),
 COUNT(*) FILTER (WHERE state='discarded') FROM river_job`).Scan(
		&deliveries.Available, &deliveries.Scheduled, &deliveries.Running, &deliveries.Retryable,
		&deliveries.Completed, &deliveries.Cancelled, &deliveries.Discarded)
	if err != nil {
		api.WriteProblem(c, api.StatusProblem(http.StatusInternalServerError, err))
		return
	}
	api.JSONOK(c, deliveries)
}

func parseErrorLimit(raw string) int {
	limit, err := strconv.Atoi(raw)
	if err != nil || limit < 0 {
		return 5
	}
	if limit > 20 {
		return 20
	}
	return limit
}

func (h *QueueHandler) loadQueueSummaries(ctx context.Context) ([]QueueSummaryDTO, error) {
	const query = `
WITH queue_names AS (
  SELECT name FROM river_queue
  UNION
  SELECT DISTINCT queue AS name FROM river_job
)
SELECT
  qn.name,
  COUNT(j.id) AS total_jobs,
  COUNT(j.id) FILTER (WHERE j.state = 'completed') AS processed_jobs,
  COUNT(j.id) FILTER (WHERE j.state IN ('available', 'scheduled', 'running', 'retryable')) AS remaining_jobs,
  COUNT(j.id) FILTER (WHERE j.state = 'running') AS running_jobs,
  COUNT(j.id) FILTER (WHERE j.state IN ('retryable', 'cancelled', 'discarded')) AS attention_jobs,
  AVG((julianday(j.finalized_at) - julianday(j.created_at)) * 86400000.0)
    FILTER (WHERE j.finalized_at IS NOT NULL) AS average_latency_ms,
  AVG((julianday(j.finalized_at) - julianday(j.attempted_at)) * 86400000.0)
    FILTER (WHERE j.finalized_at IS NOT NULL AND j.attempted_at IS NOT NULL) AS average_runtime_ms,
  CAST(
    unixepoch(
      MIN(j.created_at) FILTER (
        WHERE j.state IN ('available', 'scheduled', 'running', 'retryable')
      ),
      'subsec'
    ) * 1000000
    AS INTEGER
  ) AS oldest_remaining_micros,
  CAST(
    unixepoch(
      COALESCE(
        MAX(max(
          j.created_at,
          j.scheduled_at,
          coalesce(j.attempted_at, j.created_at),
          coalesce(j.finalized_at, j.created_at)
        )),
        MAX(rq.updated_at)
      ),
      'subsec'
    ) * 1000000
    AS INTEGER
  ) AS latest_activity_micros
FROM queue_names qn
LEFT JOIN river_queue rq ON rq.name = qn.name
LEFT JOIN river_job j ON j.queue = qn.name
GROUP BY qn.name
ORDER BY
  attention_jobs DESC,
  remaining_jobs DESC,
  latest_activity_micros IS NULL,
  latest_activity_micros DESC,
  qn.name
`

	rows, err := h.dbpool.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	summaries := make([]QueueSummaryDTO, 0)
	for rows.Next() {
		var summary QueueSummaryDTO
		var averageLatency sql.NullFloat64
		var averageRuntime sql.NullFloat64
		var oldestRemaining sql.NullInt64
		var latestActivity sql.NullInt64

		if err := rows.Scan(
			&summary.Name,
			&summary.TotalJobs,
			&summary.ProcessedJobs,
			&summary.RemainingJobs,
			&summary.RunningJobs,
			&summary.AttentionJobs,
			&averageLatency,
			&averageRuntime,
			&oldestRemaining,
			&latestActivity,
		); err != nil {
			return nil, err
		}

		summary.AverageLatencyMs = nullableMillis(averageLatency)
		summary.AverageRuntimeMs = nullableMillis(averageRuntime)
		summary.OldestRemainingAt = nullableUnixMicros(oldestRemaining)
		summary.LatestActivityAt = nullableUnixMicros(latestActivity)
		summaries = append(summaries, summary)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return summaries, nil
}

func (h *QueueHandler) attachQueueErrorSamples(ctx context.Context, queues []QueueSummaryDTO, limit int) error {
	const query = `
WITH ranked_errors AS (
  SELECT
    queue,
    id,
    kind,
    state,
    attempt,
    max_attempts,
    created_at,
    scheduled_at,
    attempted_at,
    finalized_at,
    COALESCE(json_extract(errors, '$[#-1].error'), '') AS last_error,
    row_number() OVER (
      PARTITION BY queue
      ORDER BY COALESCE(attempted_at, finalized_at, created_at) DESC, id DESC
    ) AS rn
  FROM river_job
  WHERE state IN ('retryable', 'cancelled', 'discarded')
)
SELECT
  queue,
  id,
  kind,
  state,
  attempt,
  max_attempts,
  CAST(unixepoch(created_at, 'subsec') * 1000000 AS INTEGER) AS created_at_micros,
  CAST(unixepoch(scheduled_at, 'subsec') * 1000000 AS INTEGER) AS scheduled_at_micros,
  CAST(unixepoch(attempted_at, 'subsec') * 1000000 AS INTEGER) AS attempted_at_micros,
  CAST(unixepoch(finalized_at, 'subsec') * 1000000 AS INTEGER) AS finalized_at_micros,
  last_error
FROM ranked_errors
WHERE rn <= ?
ORDER BY queue, rn
`

	rows, err := h.dbpool.QueryContext(ctx, query, limit)
	if err != nil {
		return err
	}
	defer rows.Close()

	samplesByQueue := make(map[string][]QueueErrorSampleDTO)
	for rows.Next() {
		var queueName string
		var sample QueueErrorSampleDTO
		var createdAt int64
		var scheduledAt int64
		var attemptedAt sql.NullInt64
		var finalizedAt sql.NullInt64

		if err := rows.Scan(
			&queueName,
			&sample.JobID,
			&sample.Kind,
			&sample.State,
			&sample.Attempt,
			&sample.MaxAttempts,
			&createdAt,
			&scheduledAt,
			&attemptedAt,
			&finalizedAt,
			&sample.LastError,
		); err != nil {
			return err
		}

		sample.CreatedAt = time.UnixMicro(createdAt).UTC()
		sample.ScheduledAt = time.UnixMicro(scheduledAt).UTC()
		sample.AttemptedAt = nullableUnixMicros(attemptedAt)
		sample.FinalizedAt = nullableUnixMicros(finalizedAt)
		samplesByQueue[queueName] = append(samplesByQueue[queueName], sample)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for i := range queues {
		queues[i].ErrorSamples = samplesByQueue[queues[i].Name]
	}

	return nil
}

func nullableMillis(value sql.NullFloat64) *int64 {
	if !value.Valid {
		return nil
	}
	millis := int64(value.Float64 + 0.5)
	return &millis
}

func nullableUnixMicros(value sql.NullInt64) *time.Time {
	if !value.Valid {
		return nil
	}
	timestamp := time.UnixMicro(value.Int64).UTC()
	return &timestamp
}
