package handler

import (
	"server/internal/api"
	"server/internal/db/repo"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
)

// StatsHandler handles HTTP requests for photo statistics
type StatsHandler struct {
	queries *repo.Queries
}

// NewStatsHandler creates a new StatsHandler instance
func NewStatsHandler(queries *repo.Queries) *StatsHandler {
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

// GetFocalLengthDistribution godoc
// @Summary Get focal length distribution
// @Description Get distribution of commonly used focal lengths
// @Tags stats
// @Produce json
// @Success 200 {object} api.Result{data=FocalLengthDistributionResponse}
// @Failure 500 {object} api.Result
// @Router /api/v1/stats/focal-length [get]
func (h *StatsHandler) GetFocalLengthDistribution(c *gin.Context) {
	rows, err := h.queries.GetFocalLengthDistribution(c.Request.Context())
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

	rows, err := h.queries.GetCameraLensStats(c.Request.Context(), int32(limit))
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
// @Success 200 {object} api.Result{data=TimeDistributionResponse}
// @Failure 400 {object} api.Result
// @Failure 500 {object} api.Result
// @Router /api/v1/stats/time-distribution [get]
func (h *StatsHandler) GetTimeDistribution(c *gin.Context) {
	distType := c.DefaultQuery("type", "hourly")

	var data []TimeBucket
	var err error

	switch distType {
	case "hourly":
		data, err = h.getHourlyDistribution(c)
	case "monthly":
		data, err = h.getMonthlyDistribution(c)
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
func (h *StatsHandler) getHourlyDistribution(c *gin.Context) ([]TimeBucket, error) {
	rows, err := h.queries.GetTimeDistributionHourly(c.Request.Context())
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
func (h *StatsHandler) getMonthlyDistribution(c *gin.Context) ([]TimeBucket, error) {
	rows, err := h.queries.GetTimeDistributionMonthly(c.Request.Context())
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
// @Description Get daily shooting activity heatmap data for the past year
// @Tags stats
// @Produce json
// @Param days query int false "Number of days to look back" default(365)
// @Success 200 {object} api.Result{data=HeatmapResponse}
// @Failure 400 {object} api.Result
// @Failure 500 {object} api.Result
// @Router /api/v1/stats/daily-activity [get]
func (h *StatsHandler) GetDailyActivityHeatmap(c *gin.Context) {
	// Parse days parameter
	daysStr := c.DefaultQuery("days", "365")
	days, err := strconv.Atoi(daysStr)
	if err != nil || days <= 0 || days > 730 {
		api.GinBadRequest(c, err, "Invalid days parameter. Must be between 1 and 730")
		return
	}

	// Calculate date range
	endDate := time.Now()
	startDate := endDate.AddDate(0, 0, -days)

	// Convert to pgtype.Timestamptz
	params := repo.GetDailyActivityHeatmapParams{
		TakenTime: pgtype.Timestamptz{
			Time:  startDate,
			Valid: true,
		},
		TakenTime_2: pgtype.Timestamptz{
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

// GetAvailableYears godoc
// @Summary Get available years
// @Description Get list of years that have photo data
// @Tags stats
// @Produce json
// @Success 200 {object} api.Result{data=AvailableYearsResponse}
// @Failure 500 {object} api.Result
// @Router /api/v1/stats/available-years [get]
func (h *StatsHandler) GetAvailableYears(c *gin.Context) {
	rows, err := h.queries.GetAvailableYears(c.Request.Context())
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
