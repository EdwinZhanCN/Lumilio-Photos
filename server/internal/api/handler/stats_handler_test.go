package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"server/internal/api"
	"server/internal/db/repo"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
)

type stubStatsQuerier struct {
	getFocalLengthDistributionFn func(ctx context.Context, repositoryID pgtype.UUID) ([]repo.GetFocalLengthDistributionRow, error)
	getCameraLensStatsFn         func(arg repo.GetCameraLensStatsParams) ([]repo.GetCameraLensStatsRow, error)
	getTimeDistributionHourlyFn  func(repositoryID pgtype.UUID) ([]repo.GetTimeDistributionHourlyRow, error)
	getTimeDistributionMonthlyFn func(repositoryID pgtype.UUID) ([]repo.GetTimeDistributionMonthlyRow, error)
	getDailyActivityHeatmapFn    func(arg repo.GetDailyActivityHeatmapParams) ([]repo.GetDailyActivityHeatmapRow, error)
	getAvailableYearsFn          func(repositoryID pgtype.UUID) ([]int32, error)
}

func (s stubStatsQuerier) GetFocalLengthDistribution(_ context.Context, repositoryID pgtype.UUID) ([]repo.GetFocalLengthDistributionRow, error) {
	if s.getFocalLengthDistributionFn != nil {
		return s.getFocalLengthDistributionFn(context.Background(), repositoryID)
	}
	return nil, nil
}

func (s stubStatsQuerier) GetCameraLensStats(_ context.Context, arg repo.GetCameraLensStatsParams) ([]repo.GetCameraLensStatsRow, error) {
	if s.getCameraLensStatsFn != nil {
		return s.getCameraLensStatsFn(arg)
	}
	return nil, nil
}

func (s stubStatsQuerier) GetTimeDistributionHourly(_ context.Context, repositoryID pgtype.UUID) ([]repo.GetTimeDistributionHourlyRow, error) {
	if s.getTimeDistributionHourlyFn != nil {
		return s.getTimeDistributionHourlyFn(repositoryID)
	}
	return nil, nil
}

func (s stubStatsQuerier) GetTimeDistributionMonthly(_ context.Context, repositoryID pgtype.UUID) ([]repo.GetTimeDistributionMonthlyRow, error) {
	if s.getTimeDistributionMonthlyFn != nil {
		return s.getTimeDistributionMonthlyFn(repositoryID)
	}
	return nil, nil
}

func (s stubStatsQuerier) GetDailyActivityHeatmap(_ context.Context, arg repo.GetDailyActivityHeatmapParams) ([]repo.GetDailyActivityHeatmapRow, error) {
	if s.getDailyActivityHeatmapFn != nil {
		return s.getDailyActivityHeatmapFn(arg)
	}
	return nil, nil
}

func (s stubStatsQuerier) GetAvailableYears(_ context.Context, repositoryID pgtype.UUID) ([]int32, error) {
	if s.getAvailableYearsFn != nil {
		return s.getAvailableYearsFn(repositoryID)
	}
	return nil, nil
}

func TestStatsHandlerGetFocalLengthDistribution_UsesRepositoryScope(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rawRepositoryID := "550e8400-e29b-41d4-a716-446655440000"
	expectedRepositoryID := uuid.MustParse(rawRepositoryID)
	handler := &StatsHandler{
		queries: stubStatsQuerier{
			getFocalLengthDistributionFn: func(_ context.Context, repositoryID pgtype.UUID) ([]repo.GetFocalLengthDistributionRow, error) {
				require.True(t, repositoryID.Valid)
				require.Equal(t, [16]byte(expectedRepositoryID), repositoryID.Bytes)
				return []repo.GetFocalLengthDistributionRow{
					{
						FocalLength: pgtype.Numeric{},
						Count:       4,
					},
				}, nil
			},
		},
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/stats/focal-length?repository_id="+rawRepositoryID, nil)

	handler.GetFocalLengthDistribution(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)

	var response api.Result
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, 0, response.Code)
}

func TestStatsHandlerGetDailyActivityHeatmap_RejectsInvalidRepositoryID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := &StatsHandler{queries: stubStatsQuerier{}}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/stats/daily-activity?repository_id=bad-id", nil)

	handler.GetDailyActivityHeatmap(ctx)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestStatsHandlerGetDailyActivityHeatmap_UsesYearRange(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := &StatsHandler{
		queries: stubStatsQuerier{
			getDailyActivityHeatmapFn: func(arg repo.GetDailyActivityHeatmapParams) ([]repo.GetDailyActivityHeatmapRow, error) {
				require.Equal(t, 2024, arg.StartTime.Time.Year())
				require.Equal(t, time.January, arg.StartTime.Time.Month())
				require.Equal(t, 1, arg.StartTime.Time.Day())

				require.Equal(t, 2024, arg.EndTime.Time.Year())
				require.Equal(t, time.December, arg.EndTime.Time.Month())
				require.Equal(t, 31, arg.EndTime.Time.Day())
				return []repo.GetDailyActivityHeatmapRow{}, nil
			},
		},
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/stats/daily-activity?year=2024", nil)

	handler.GetDailyActivityHeatmap(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
}

func TestStatsHandlerGetDailyActivityHeatmap_RejectsIncompleteDateRange(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := &StatsHandler{queries: stubStatsQuerier{}}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/stats/daily-activity?start_date=2024-01-01", nil)

	handler.GetDailyActivityHeatmap(ctx)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestStatsHandlerGetAvailableYears_AllRepositories(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := &StatsHandler{
		queries: stubStatsQuerier{
			getAvailableYearsFn: func(repositoryID pgtype.UUID) ([]int32, error) {
				require.False(t, repositoryID.Valid)
				return []int32{2025, 2024}, nil
			},
		},
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/stats/available-years", nil)

	handler.GetAvailableYears(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
}
