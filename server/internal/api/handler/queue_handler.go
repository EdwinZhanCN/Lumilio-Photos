package handler

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"server/internal/api"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
)

// QueueHandler handles River queue monitoring endpoints (read-only)
type QueueHandler struct {
	queueClient *river.Client[pgx.Tx]
	dbpool      *pgxpool.Pool
}

// NewQueueHandler creates a new queue handler
func NewQueueHandler(queueClient *river.Client[pgx.Tx], dbpool *pgxpool.Pool) *QueueHandler {
	return &QueueHandler{
		queueClient: queueClient,
		dbpool:      dbpool,
	}
}

// JobListResponse represents the response for job list endpoint
type JobListResponse struct {
	Jobs       []JobDTO `json:"jobs"`
	Cursor     *string  `json:"cursor,omitempty"`
	TotalCount *int64   `json:"total_count,omitempty"`
}

// JobDTO represents a job for API response
type JobDTO struct {
	ID          int64      `json:"id"`
	Queue       string     `json:"queue"`
	Kind        string     `json:"kind"`
	State       string     `json:"state"`
	Attempt     int        `json:"attempt"`
	MaxAttempts int        `json:"max_attempts"`
	Priority    int        `json:"priority"`
	ScheduledAt time.Time  `json:"scheduled_at"`
	CreatedAt   time.Time  `json:"created_at"`
	AttemptedAt *time.Time `json:"attempted_at,omitempty"`
	FinalizedAt *time.Time `json:"finalized_at,omitempty"`
	Errors      []string   `json:"errors,omitempty"`
	Args        any        `json:"args,omitempty"`
	Metadata    any        `json:"metadata,omitempty"`
}

// QueueStatsResponse represents queue statistics
type QueueStatsResponse struct {
	Queues []QueueStatsDTO `json:"queues"`
}

// QueueStatsDTO represents statistics for a single queue
type QueueStatsDTO struct {
	Name      string    `json:"name"`
	UpdatedAt time.Time `json:"updated_at"`
	Metadata  any       `json:"metadata,omitempty"`
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

// ListJobs godoc
// @Summary List jobs with filters
// @Description Get a paginated list of jobs with optional state, queue, kind, and time range filters
// @Tags Queue
// @Accept json
// @Produce json
// @Param state query string false "Job state filter (available,scheduled,running,retryable,completed,cancelled,discarded)"
// @Param queue query string false "Queue name filter"
// @Param kind query string false "Job kind filter"
// @Param limit query int false "Number of jobs to return (default: 50, max: 200)"
// @Param cursor query string false "Pagination cursor for next page"
// @Param time_range query string false "Time range filter (1h, 24h, 30d) - filters by created_at"
// @Param include_count query bool false "Include total count of matching jobs (may be slower)"
// @Success 200 {object} api.Result{data=JobListResponse}
// @Router /api/v1/admin/river/jobs [get]
func (h *QueueHandler) ListJobs(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	// Parse query parameters
	stateStr := c.Query("state")
	queueName := c.Query("queue")
	kindStr := c.Query("kind")
	limitStr := c.DefaultQuery("limit", "50")
	cursorStr := c.Query("cursor")
	timeRange := c.Query("time_range")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	// Parse cursor for pagination
	var cursor *river.JobListCursor
	if cursorStr != "" {
		cursor = &river.JobListCursor{}
		// Decode base64 cursor
		cursorBytes, err := base64.StdEncoding.DecodeString(cursorStr)
		if err != nil {
			api.GinError(c, http.StatusBadRequest, err, http.StatusBadRequest, "Invalid cursor parameter")
			return
		}
		if err := cursor.UnmarshalText(cursorBytes); err != nil {
			api.GinError(c, http.StatusBadRequest, err, http.StatusBadRequest, "Failed to parse cursor")
			return
		}
	}

	// Calculate time threshold based on time_range
	var timeThreshold time.Time
	if timeRange != "" {
		now := time.Now()
		switch timeRange {
		case "1h":
			timeThreshold = now.Add(-1 * time.Hour)
		case "24h":
			timeThreshold = now.Add(-24 * time.Hour)
		case "30d":
			timeThreshold = now.Add(-30 * 24 * time.Hour)
		default:
			api.GinError(c, http.StatusBadRequest, nil, http.StatusBadRequest, "Invalid time_range parameter. Use: 1h, 24h, or 30d")
			return
		}
	}

	// Count total jobs matching filters (optional, only if needed)
	var totalCount *int64
	if c.Query("include_count") == "true" {
		countQuery := `SELECT COUNT(*) FROM river_job WHERE 1=1`
		countArgs := []interface{}{}
		countIdx := 1

		if stateStr != "" {
			countQuery += fmt.Sprintf(" AND state = $%d", countIdx)
			countArgs = append(countArgs, stateStr)
			countIdx++
		}
		if queueName != "" {
			countQuery += fmt.Sprintf(" AND queue = $%d", countIdx)
			countArgs = append(countArgs, queueName)
			countIdx++
		}
		if kindStr != "" {
			countQuery += fmt.Sprintf(" AND kind = $%d", countIdx)
			countArgs = append(countArgs, kindStr)
			countIdx++
		}
		if !timeThreshold.IsZero() {
			countQuery += fmt.Sprintf(" AND created_at >= $%d", countIdx)
			countArgs = append(countArgs, timeThreshold)
		}

		var count int64
		if err := h.dbpool.QueryRow(ctx, countQuery, countArgs...).Scan(&count); err == nil {
			totalCount = &count
		}
	}

	// Build job list params using River API
	params := river.NewJobListParams().
		First(limit).
		OrderBy(river.JobListOrderByID, river.SortOrderDesc)

	// Apply cursor for pagination
	if cursor != nil {
		params = params.After(cursor)
	}

	// Apply state filter
	if stateStr != "" {
		state := rivertype.JobState(stateStr)
		params = params.States(state)
	}

	// Apply queue filter
	if queueName != "" {
		params = params.Queues(queueName)
	}

	// Apply kind filter
	if kindStr != "" {
		params = params.Kinds(kindStr)
	}

	// Apply time range filter using Where clause
	if !timeThreshold.IsZero() {
		params = params.Where("created_at >= @time_threshold", river.NamedArgs{
			"time_threshold": timeThreshold,
		})
	}

	// Fetch jobs using River API
	result, err := h.queueClient.JobList(ctx, params)
	if err != nil {
		api.GinError(c, http.StatusInternalServerError, err, http.StatusInternalServerError, "Failed to fetch jobs")
		return
	}

	// Convert to DTOs
	jobs := make([]JobDTO, len(result.Jobs))
	for i, job := range result.Jobs {
		jobs[i] = convertJobRowToDTO(job)
	}

	// Build response with cursor for pagination
	response := JobListResponse{
		Jobs:       jobs,
		TotalCount: totalCount,
	}

	// Encode cursor for next page if available
	if result.LastCursor != nil {
		cursorBytes, err := result.LastCursor.MarshalText()
		if err == nil {
			cursorStr := base64.StdEncoding.EncodeToString(cursorBytes)
			response.Cursor = &cursorStr
		}
	}

	api.GinSuccess(c, response)
}

// GetJob godoc
// @Summary Get job by ID
// @Description Get detailed information about a specific job
// @Tags Queue
// @Accept json
// @Produce json
// @Param id path int true "Job ID"
// @Success 200 {object} api.Result{data=JobDTO}
// @Failure 404 {object} api.Result "Job not found"
// @Router /api/v1/admin/river/jobs/{id} [get]
func (h *QueueHandler) GetJob(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	idStr := c.Param("id")
	jobID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		api.GinError(c, http.StatusBadRequest, err, http.StatusBadRequest, "Invalid job ID")
		return
	}

	job, err := h.queueClient.JobGet(ctx, jobID)
	if err != nil {
		if err == rivertype.ErrNotFound {
			api.GinError(c, http.StatusNotFound, err, http.StatusNotFound, "Job not found")
			return
		}
		api.GinError(c, http.StatusInternalServerError, err, http.StatusInternalServerError, "Failed to fetch job")
		return
	}

	dto := convertJobRowToDTO(job)
	api.GinSuccess(c, dto)
}

// ListQueues godoc
// @Summary List all queues
// @Description Get a list of all active queues with their metadata
// @Tags Queue
// @Accept json
// @Produce json
// @Param limit query int false "Number of queues to return (default: 100)"
// @Success 200 {object} api.Result{data=QueueStatsResponse}
// @Router /api/v1/admin/river/queues [get]
func (h *QueueHandler) ListQueues(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	limitStr := c.DefaultQuery("limit", "100")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 100
	}

	params := river.NewQueueListParams().First(limit)
	result, err := h.queueClient.QueueList(ctx, params)
	if err != nil {
		api.GinError(c, http.StatusInternalServerError, err, http.StatusInternalServerError, "Failed to fetch queues")
		return
	}

	queues := make([]QueueStatsDTO, len(result.Queues))
	for i, q := range result.Queues {
		queues[i] = QueueStatsDTO{
			Name:      q.Name,
			UpdatedAt: q.UpdatedAt,
			Metadata:  q.Metadata,
		}
	}

	response := QueueStatsResponse{
		Queues: queues,
	}

	api.GinSuccess(c, response)
}

// GetJobStats godoc
// @Summary Get job statistics
// @Description Get aggregated statistics about jobs by state
// @Tags Queue
// @Accept json
// @Produce json
// @Success 200 {object} api.Result{data=JobStatsResponse}
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
		query := `SELECT COUNT(*) FROM river_job WHERE state = $1`
		err := h.dbpool.QueryRow(ctx, query, state).Scan(countPtr)
		if err != nil {
			api.GinError(c, http.StatusInternalServerError, err, http.StatusInternalServerError, "Failed to fetch job stats")
			return
		}
	}

	api.GinSuccess(c, stats)
}

// convertJobRowToDTO converts a River JobRow to our DTO
func convertJobRowToDTO(job *rivertype.JobRow) JobDTO {
	dto := JobDTO{
		ID:          job.ID,
		Queue:       job.Queue,
		Kind:        job.Kind,
		State:       string(job.State),
		Attempt:     job.Attempt,
		MaxAttempts: job.MaxAttempts,
		Priority:    job.Priority,
		ScheduledAt: job.ScheduledAt,
		CreatedAt:   job.CreatedAt,
		Args:        job.EncodedArgs,
		Metadata:    job.Metadata,
	}

	if job.AttemptedAt != nil && !job.AttemptedAt.IsZero() {
		dto.AttemptedAt = job.AttemptedAt
	}

	if job.FinalizedAt != nil && !job.FinalizedAt.IsZero() {
		dto.FinalizedAt = job.FinalizedAt
	}

	if len(job.Errors) > 0 {
		dto.Errors = make([]string, len(job.Errors))
		for i, e := range job.Errors {
			dto.Errors[i] = e.Error
		}
	}

	return dto
}
