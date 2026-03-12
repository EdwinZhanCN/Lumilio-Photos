package handler

import (
	"context"
	"errors"
	"server/internal/api"
	"server/internal/db/repo"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type statsQuerier interface {
	GetFocalLengthDistribution(ctx context.Context, repositoryID pgtype.UUID) ([]repo.GetFocalLengthDistributionRow, error)
	GetCameraLensStats(ctx context.Context, arg repo.GetCameraLensStatsParams) ([]repo.GetCameraLensStatsRow, error)
	GetTimeDistributionHourly(ctx context.Context, repositoryID pgtype.UUID) ([]repo.GetTimeDistributionHourlyRow, error)
	GetTimeDistributionMonthly(ctx context.Context, repositoryID pgtype.UUID) ([]repo.GetTimeDistributionMonthlyRow, error)
	GetDailyActivityHeatmap(ctx context.Context, arg repo.GetDailyActivityHeatmapParams) ([]repo.GetDailyActivityHeatmapRow, error)
	GetAvailableYears(ctx context.Context, repositoryID pgtype.UUID) ([]int32, error)
}

// StatsHandler handles HTTP requests for photo statistics
type StatsHandler struct {
	queries statsQuerier
}

// NewStatsHandler creates a new StatsHandler instance
func NewStatsHandler(queries statsQuerier) *StatsHandler {
	return &StatsHandler{
		queries: queries,
	}
}

// FocalLengthBucket represents a focal length distribution data point
type FocalLengthBucket struct {
	FocalLength float64 `json:"focal_length"`
	Count       int64   `json:"count"`
}

// FocalLengthDistributionResponse represents the focal length distribution response
type FocalLengthDistributionResponse struct {
	Data  []FocalLengthBucket `json:"data"`
	Total int64               `json:"total"`
}

// CameraLensCombination represents a camera+lens combination data point
type CameraLensCombination struct {
	CameraModel string `json:"camera_model"`
	LensModel   string `json:"lens_model"`
	Count       int64  `json:"count"`
}

// CameraLensStatsResponse represents the camera lens stats response
type CameraLensStatsResponse struct {
	Data  []CameraLensCombination `json:"data"`
	Total int64                   `json:"total"`
}

// TimeBucket represents a time distribution data point
type TimeBucket struct {
	Label string `json:"label"`
	Value int    `json:"value"`
	Count int64  `json:"count"`
}

// TimeDistributionResponse represents the time distribution response
type TimeDistributionResponse struct {
	Data []TimeBucket `json:"data"`
	Type string       `json:"type"`
}

// HeatmapValue represents a single day's activity data
type HeatmapValue struct {
	Date  string `json:"date"`
	Count int64  `json:"count"`
}

// HeatmapResponse represents the heatmap data response
type HeatmapResponse struct {
	Data []HeatmapValue `json:"data"`
}

// AvailableYearsResponse represents the available years response
type AvailableYearsResponse struct {
	Years []int `json:"years"`
}

const (
	heatmapDateLayout = "2006-01-02"
)

// GetFocalLengthDistribution godoc
// @Summary Get focal length distribution
// @Description Get distribution of commonly used focal lengths
// @Tags stats
// @Produce json
// @Param repository_id query string false "Optional repository UUID filter"
// @Success 200 {object} api.Result{data=FocalLengthDistributionResponse}
// @Failure 400 {object} api.Result
// @Failure 500 {object} api.Result
// @Router /api/v1/stats/focal-length [get]
func (h *StatsHandler) GetFocalLengthDistribution(c *gin.Context) {
	repositoryID, ok := parseOptionalRepositoryUUID(c)
	if !ok {
		return
	}

	rows, err := h.queries.GetFocalLengthDistribution(c.Request.Context(), repositoryID)
	if err != nil {
		api.GinInternalError(c, err, "Failed to fetch focal length distribution")
		return
	}

	data := make([]FocalLengthBucket, 0, len(rows))
	var total int64

	for _, row := range rows {
		// Convert pgtype.Numeric to float64
		var focalLength float64
		if row.FocalLength.Valid {
			f, _ := row.FocalLength.Float64Value()
			focalLength = f.Float64
		}

		data = append(data, FocalLengthBucket{
			FocalLength: focalLength,
			Count:       row.Count,
		})
		total += row.Count
	}

	response := FocalLengthDistributionResponse{
		Data:  data,
		Total: total,
	}

	api.GinSuccess(c, response)
}

// GetCameraLensStats godoc
// @Summary Get camera lens combination stats
// @Description Get top N camera+lens combinations
// @Tags stats
// @Produce json
// @Param limit query int false "Number of results to return" default(20)
// @Param repository_id query string false "Optional repository UUID filter"
// @Success 200 {object} api.Result{data=CameraLensStatsResponse}
// @Failure 400 {object} api.Result
// @Failure 500 {object} api.Result
// @Router /api/v1/stats/camera-lens [get]
func (h *StatsHandler) GetCameraLensStats(c *gin.Context) {
	// Parse limit parameter
	limitStr := c.DefaultQuery("limit", "20")
	limit, err := strconv.ParseInt(limitStr, 10, 32)
	if err != nil || limit <= 0 {
		api.GinBadRequest(c, err, "Invalid limit parameter")
		return
	}

	repositoryID, ok := parseOptionalRepositoryUUID(c)
	if !ok {
		return
	}

	rows, err := h.queries.GetCameraLensStats(c.Request.Context(), repo.GetCameraLensStatsParams{
		RepositoryID: repositoryID,
		Limit:        int32(limit),
	})
	if err != nil {
		api.GinInternalError(c, err, "Failed to fetch camera lens stats")
		return
	}

	data := make([]CameraLensCombination, 0, len(rows))
	var total int64

	for _, row := range rows {
		var cameraModel, lensModel string

		// Extract camera model
		if str, ok := row.CameraModel.(string); ok {
			cameraModel = str
		}

		// Extract lens model
		if str, ok := row.LensModel.(string); ok {
			lensModel = str
		}

		data = append(data, CameraLensCombination{
			CameraModel: cameraModel,
			LensModel:   lensModel,
			Count:       row.Count,
		})
		total += row.Count
	}

	response := CameraLensStatsResponse{
		Data:  data,
		Total: total,
	}

	api.GinSuccess(c, response)
}

// GetTimeDistribution godoc
// @Summary Get time distribution
// @Description Get shooting time distribution by hour or month
// @Tags stats
// @Produce json
// @Param type query string false "Distribution type: hourly or monthly" default(hourly) Enums(hourly, monthly)
// @Param repository_id query string false "Optional repository UUID filter"
// @Success 200 {object} api.Result{data=TimeDistributionResponse}
// @Failure 400 {object} api.Result
// @Failure 500 {object} api.Result
// @Router /api/v1/stats/time-distribution [get]
func (h *StatsHandler) GetTimeDistribution(c *gin.Context) {
	distType := c.DefaultQuery("type", "hourly")
	repositoryID, ok := parseOptionalRepositoryUUID(c)
	if !ok {
		return
	}

	var data []TimeBucket
	var err error

	switch distType {
	case "hourly":
		data, err = h.getHourlyDistribution(c, repositoryID)
	case "monthly":
		data, err = h.getMonthlyDistribution(c, repositoryID)
	default:
		api.GinBadRequest(c, nil, "Invalid type parameter. Must be 'hourly' or 'monthly'")
		return
	}

	if err != nil {
		api.GinInternalError(c, err, "Failed to fetch time distribution")
		return
	}

	response := TimeDistributionResponse{
		Data: data,
		Type: distType,
	}

	api.GinSuccess(c, response)
}

// getHourlyDistribution fetches hourly time distribution
func (h *StatsHandler) getHourlyDistribution(c *gin.Context, repositoryID pgtype.UUID) ([]TimeBucket, error) {
	rows, err := h.queries.GetTimeDistributionHourly(c.Request.Context(), repositoryID)
	if err != nil {
		return nil, err
	}

	data := make([]TimeBucket, 0, len(rows))
	for _, row := range rows {
		label := strconv.Itoa(int(row.Hour)) + ":00"
		data = append(data, TimeBucket{
			Label: label,
			Value: int(row.Hour),
			Count: row.Count,
		})
	}

	return data, nil
}

// getMonthlyDistribution fetches monthly time distribution
func (h *StatsHandler) getMonthlyDistribution(c *gin.Context, repositoryID pgtype.UUID) ([]TimeBucket, error) {
	rows, err := h.queries.GetTimeDistributionMonthly(c.Request.Context(), repositoryID)
	if err != nil {
		return nil, err
	}

	data := make([]TimeBucket, 0, len(rows))
	for _, row := range rows {
		if row.Month.Valid {
			monthTime := row.Month.Time
			label := monthTime.Format("2006-01")
			value := int(monthTime.Unix())
			data = append(data, TimeBucket{
				Label: label,
				Value: value,
				Count: row.Count,
			})
		}
	}

	return data, nil
}

// GetDailyActivityHeatmap godoc
// @Summary Get daily activity heatmap
// @Description Get daily shooting activity heatmap data for a calendar year or custom date range.
// @Tags stats
// @Produce json
// @Param year query int false "Calendar year (e.g. 2024)"
// @Param start_date query string false "Start date in YYYY-MM-DD (must be used with end_date)"
// @Param end_date query string false "End date in YYYY-MM-DD, inclusive (must be used with start_date)"
// @Param days query int false "Deprecated fallback: number of days to look back (used only when year/start_date/end_date are absent)"
// @Param repository_id query string false "Optional repository UUID filter"
// @Success 200 {object} api.Result{data=HeatmapResponse}
// @Failure 400 {object} api.Result
// @Failure 500 {object} api.Result
// @Router /api/v1/stats/daily-activity [get]
func (h *StatsHandler) GetDailyActivityHeatmap(c *gin.Context) {
	startDate, endDate, err := resolveHeatmapRange(c, time.Now())
	if err != nil {
		api.GinBadRequest(c, err, err.Error())
		return
	}

	repositoryID, ok := parseOptionalRepositoryUUID(c)
	if !ok {
		return
	}

	// Convert to pgtype.Timestamptz
	params := repo.GetDailyActivityHeatmapParams{
		RepositoryID: repositoryID,
		StartTime: pgtype.Timestamptz{
			Time:  startDate,
			Valid: true,
		},
		EndTime: pgtype.Timestamptz{
			Time:  endDate,
			Valid: true,
		},
	}

	rows, err := h.queries.GetDailyActivityHeatmap(c.Request.Context(), params)
	if err != nil {
		api.GinInternalError(c, err, "Failed to fetch daily activity heatmap")
		return
	}

	data := make([]HeatmapValue, 0, len(rows))
	for _, row := range rows {
		if row.Date.Valid {
			data = append(data, HeatmapValue{
				Date:  row.Date.Time.Format("2006-01-02"),
				Count: row.Count,
			})
		}
	}

	response := HeatmapResponse{
		Data: data,
	}

	api.GinSuccess(c, response)
}

func resolveHeatmapRange(c *gin.Context, now time.Time) (time.Time, time.Time, error) {
	location := now.Location()
	yearRaw := strings.TrimSpace(c.Query("year"))
	startDateRaw := strings.TrimSpace(c.Query("start_date"))
	endDateRaw := strings.TrimSpace(c.Query("end_date"))
	daysRaw := strings.TrimSpace(c.Query("days"))

	// Highest-priority mode: calendar year.
	if yearRaw != "" {
		if startDateRaw != "" || endDateRaw != "" {
			return time.Time{}, time.Time{}, errors.New("year cannot be combined with start_date/end_date")
		}
		year, err := strconv.Atoi(yearRaw)
		if err != nil || year < 1900 || year > 2200 {
			return time.Time{}, time.Time{}, errors.New("invalid year parameter. Must be between 1900 and 2200")
		}
		start := time.Date(year, time.January, 1, 0, 0, 0, 0, location)
		end := time.Date(year+1, time.January, 1, 0, 0, 0, 0, location).Add(-time.Nanosecond)
		return start, end, nil
	}

	// Second mode: explicit date range.
	if startDateRaw != "" || endDateRaw != "" {
		if startDateRaw == "" || endDateRaw == "" {
			return time.Time{}, time.Time{}, errors.New("start_date and end_date must be provided together")
		}
		start, err := time.ParseInLocation(heatmapDateLayout, startDateRaw, location)
		if err != nil {
			return time.Time{}, time.Time{}, errors.New("invalid start_date format. Expected YYYY-MM-DD")
		}
		end, err := time.ParseInLocation(heatmapDateLayout, endDateRaw, location)
		if err != nil {
			return time.Time{}, time.Time{}, errors.New("invalid end_date format. Expected YYYY-MM-DD")
		}
		if end.Before(start) {
			return time.Time{}, time.Time{}, errors.New("end_date must be on or after start_date")
		}
		endInclusive := end.AddDate(0, 0, 1).Add(-time.Nanosecond)
		return start, endInclusive, nil
	}

	// Backward-compatible fallback for old callers.
	if daysRaw != "" {
		days, err := strconv.Atoi(daysRaw)
		if err != nil || days <= 0 || days > 36500 {
			return time.Time{}, time.Time{}, errors.New("invalid days parameter. Must be between 1 and 36500")
		}
		end := now
		start := end.AddDate(0, 0, -days)
		return start, end, nil
	}

	// Default mode: current calendar year.
	year := now.Year()
	start := time.Date(year, time.January, 1, 0, 0, 0, 0, location)
	end := time.Date(year+1, time.January, 1, 0, 0, 0, 0, location).Add(-time.Nanosecond)
	return start, end, nil
}

// GetAvailableYears godoc
// @Summary Get available years
// @Description Get list of years that have photo data
// @Tags stats
// @Produce json
// @Param repository_id query string false "Optional repository UUID filter"
// @Success 200 {object} api.Result{data=AvailableYearsResponse}
// @Failure 400 {object} api.Result
// @Failure 500 {object} api.Result
// @Router /api/v1/stats/available-years [get]
func (h *StatsHandler) GetAvailableYears(c *gin.Context) {
	repositoryID, ok := parseOptionalRepositoryUUID(c)
	if !ok {
		return
	}

	rows, err := h.queries.GetAvailableYears(c.Request.Context(), repositoryID)
	if err != nil {
		api.GinInternalError(c, err, "Failed to fetch available years")
		return
	}

	years := make([]int, 0, len(rows))
	for _, year := range rows {
		years = append(years, int(year))
	}

	response := AvailableYearsResponse{
		Years: years,
	}

	api.GinSuccess(c, response)
}

func parseOptionalRepositoryUUID(c *gin.Context) (pgtype.UUID, bool) {
	rawRepositoryID := strings.TrimSpace(c.Query("repository_id"))
	if rawRepositoryID == "" {
		return pgtype.UUID{}, true
	}

	repositoryID, err := uuid.Parse(rawRepositoryID)
	if err != nil {
		api.GinBadRequest(c, err, "Invalid repository_id parameter")
		return pgtype.UUID{}, false
	}

	return pgtype.UUID{Bytes: repositoryID, Valid: true}, true
}
