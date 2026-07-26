package handler

import (
	"context"
	"database/sql"
	"net/http"
	"strconv"
	"time"

	"server/internal/api"

	"github.com/gin-gonic/gin"
)

// QueueHandler handles River queue monitoring endpoints (read-only)
type QueueHandler struct {
	dbpool *sql.DB
}

// NewQueueHandler creates a new queue handler
func NewQueueHandler(dbpool *sql.DB) *QueueHandler {
	return &QueueHandler{
		dbpool: dbpool,
	}
}

// JobStatsResponse represents overall job statistics
type JobStatsResponse struct {
	Available int64 `json:"available"`
	Scheduled int64 `json:"scheduled"`
	Running   int64 `json:"running"`
	Retryable int64 `json:"retryable"`
	Completed int64 `json:"completed"`
	Cancelled int64 `json:"cancelled"`
	Discarded int64 `json:"discarded"`
}

// QueueSummaryResponse represents aggregated queue activity.
type QueueSummaryResponse struct {
	Queues      []QueueSummaryDTO `json:"queues"`
	GeneratedAt time.Time         `json:"generated_at"`
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

// GetQueueSummary godoc
// @Summary Get queue summaries
// @Description Get aggregated processing activity per queue, including recent error samples
// @Tags Queue
// @Accept json
// @Produce json
// @Param error_limit query int false "Recent error samples per queue (default: 5, max: 20)"
// @Success 200 {object} QueueSummaryResponse
// @Router /api/v1/admin/river/queue-summary [get]
func (h *QueueHandler) GetQueueSummary(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	errorLimit := parseErrorLimit(c.DefaultQuery("error_limit", "5"))
	queues, err := h.loadQueueSummaries(ctx)
	if err != nil {
		api.GinError(c, http.StatusInternalServerError, err, http.StatusInternalServerError, "Failed to fetch queue summaries")
		return
	}

	if len(queues) > 0 && errorLimit > 0 {
		if err := h.attachQueueErrorSamples(ctx, queues, errorLimit); err != nil {
			api.GinError(c, http.StatusInternalServerError, err, http.StatusInternalServerError, "Failed to fetch queue errors")
			return
		}
	}

	api.JSONOK(c, QueueSummaryResponse{
		Queues:      queues,
		GeneratedAt: time.Now(),
	})
}

// GetJobStats godoc
// @Summary Get job statistics
// @Description Get aggregated statistics about jobs by state
// @Tags Queue
// @Accept json
// @Produce json
// @Success 200 {object} JobStatsResponse
// @Router /api/v1/admin/river/stats [get]
func (h *QueueHandler) GetJobStats(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	// Count jobs by state using direct DB query for accurate counts
	stats := JobStatsResponse{}

	// Query for each state count
	stateQueries := map[string]*int64{
		"available": &stats.Available,
		"scheduled": &stats.Scheduled,
		"running":   &stats.Running,
		"retryable": &stats.Retryable,
		"completed": &stats.Completed,
		"cancelled": &stats.Cancelled,
		"discarded": &stats.Discarded,
	}

	for state, countPtr := range stateQueries {
		query := `SELECT COUNT(*) FROM river_job WHERE state = ?`
		err := h.dbpool.QueryRowContext(ctx, query, state).Scan(countPtr)
		if err != nil {
			api.GinError(c, http.StatusInternalServerError, err, http.StatusInternalServerError, "Failed to fetch job stats")
			return
		}
	}

	api.JSONOK(c, stats)
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
