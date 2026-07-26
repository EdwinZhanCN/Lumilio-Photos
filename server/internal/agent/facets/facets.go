// Package facets computes ref.FacetSummary aggregates over a ref snapshot.
// It is shared by the describe tool (the agent's eyesight) and the hydration
// API (ref metadata for the frontend). All user-content strings are passed
// through ref.SanitizeUserText here, so callers can emit values as-is (INV-7).
package facets

import (
	"context"
	"math"
	"strconv"
	"time"

	"server/internal/agent/ref"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"

	"github.com/google/uuid"
)

const (
	topPlaces  = 5
	topPeople  = 5
	topCameras = 3
	topGear    = 5
	// yearGranularityThreshold switches multi-month histograms to year
	// buckets when the snapshot spans more than this many years.
	yearGranularityThreshold = 3
)

// Build aggregates the snapshot's facets. Individual facet failures degrade
// to omitted fields; only the overview query is load-bearing.
func Build(ctx context.Context, queries *repo.Queries, r *ref.Ref) (*ref.FacetSummary, error) {
	summary := &ref.FacetSummary{Count: r.Count()}
	if r.Count() == 0 {
		return summary, nil
	}

	assetIDs := append([]uuid.UUID(nil), r.AssetIDs...)

	overview, err := queries.AgentFacetOverview(ctx, assetIDs)
	if err != nil {
		return nil, err
	}
	summary.LikedCount = int(overview.LikedCount)

	granularity := "month"
	dateFrom, fromOK := sqliteTime(overview.DateFrom)
	dateTo, toOK := sqliteTime(overview.DateTo)
	if fromOK && toOK {
		summary.DateRange = &ref.DateRange{
			From: dateFrom,
			To:   dateTo,
		}
		if cm := sqliteInt64(overview.CaptureOffsetMinutes); cm != 0 {
			offset := int16(cm)
			summary.DateRange.OffsetMinutes = &offset
		}
		granularity = chooseHistogramGranularity(dateFrom, dateTo)
	}
	summary.HistogramGranularity = granularity

	if buckets, err := queries.AgentFacetTimeHistogram(ctx, repo.AgentFacetTimeHistogramParams{
		Granularity: granularity,
		AssetIds:    dbtypes.UUIDsJSONParam(assetIDs),
	}); err == nil {
		summary.Histogram = make([]ref.Bucket, 0, len(buckets))
		for _, b := range buckets {
			summary.Histogram = append(summary.Histogram, ref.Bucket{Bucket: sqliteString(b.Bucket), Count: int(b.Count)})
		}
	}

	if types, err := queries.AgentFacetTypeCounts(ctx, assetIDs); err == nil && len(types) > 0 {
		summary.Types = make(map[string]int, len(types))
		for _, t := range types {
			summary.Types[t.Type] = int(t.Count)
		}
	}

	if places, err := queries.AgentFacetTopPlaces(ctx, repo.AgentFacetTopPlacesParams{
		AssetIds: dbtypes.UUIDsJSONParam(assetIDs),
		TopN:     topPlaces,
	}); err == nil {
		summary.TopPlaces = toNameCounts(places, func(row repo.AgentFacetTopPlacesRow) (string, int64) {
			return sqliteString(row.Name), row.Count
		})
	}

	if people, err := queries.AgentFacetTopPeople(ctx, repo.AgentFacetTopPeopleParams{
		AssetIds: dbtypes.UUIDsJSONParam(assetIDs),
		TopN:     topPeople,
	}); err == nil {
		summary.TopPeople = toNameCounts(people, func(row repo.AgentFacetTopPeopleRow) (string, int64) {
			return sqliteString(row.Name), row.Count
		})
	}

	if cameras, err := queries.AgentFacetCameraCounts(ctx, repo.AgentFacetCameraCountsParams{
		AssetIds: dbtypes.UUIDsJSONParam(assetIDs),
		TopN:     topCameras,
	}); err == nil {
		summary.Cameras = toNameCounts(cameras, func(row repo.AgentFacetCameraCountsRow) (string, int64) {
			return sqliteString(row.Name), row.Count
		})
	}

	if focals, err := queries.AgentFacetTopFocalLengths(ctx, repo.AgentFacetTopFocalLengthsParams{
		AssetIds: dbtypes.UUIDsJSONParam(assetIDs),
		TopN:     topGear,
	}); err == nil {
		summary.FocalLengths = toNameCounts(focals, func(row repo.AgentFacetTopFocalLengthsRow) (string, int64) {
			return sqliteString(row.Name), row.Count
		})
	}

	if lenses, err := queries.AgentFacetTopLenses(ctx, repo.AgentFacetTopLensesParams{
		AssetIds: dbtypes.UUIDsJSONParam(assetIDs),
		TopN:     topGear,
	}); err == nil {
		summary.Lenses = toNameCounts(lenses, func(row repo.AgentFacetTopLensesRow) (string, int64) {
			return sqliteString(row.Name), row.Count
		})
	}

	if ratings, err := queries.AgentFacetRatingDist(ctx, assetIDs); err == nil && len(ratings) > 0 {
		dist := make([]int, 6)
		nonZero := false
		for _, row := range ratings {
			if row.Rating >= 0 && row.Rating <= 5 {
				dist[row.Rating] = int(row.Count)
				if row.Rating > 0 && row.Count > 0 {
					nonZero = true
				}
			}
		}
		if nonZero {
			summary.RatingDist = dist
		}
	}

	if q, err := queries.AgentFacetQualityStats(ctx, assetIDs); err == nil && q.ScoredCount > 0 {
		summary.Quality = &ref.QualityStats{
			Scored:   int(q.ScoredCount),
			Unscored: r.Count() - int(q.ScoredCount),
			P25:      round1(q.P25),
			P50:      round1(q.P50),
			P75:      round1(q.P75),
			P90:      round1(q.P90),
		}
	}

	return summary, nil
}

// round1 rounds a score to one decimal place — the percentiles are presented
// to the model as distribution shape, so sub-decimal precision is just noise.
func round1(v any) float64 {
	return math.Round(sqliteFloat64(v)*10) / 10
}

func sqliteString(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case []byte:
		return string(typed)
	case *string:
		if typed != nil {
			return *typed
		}
	}
	return ""
}

func sqliteInt64(value any) int64 {
	switch typed := value.(type) {
	case int64:
		return typed
	case int:
		return int64(typed)
	case float64:
		return int64(typed)
	case []byte:
		parsed, _ := strconv.ParseInt(string(typed), 10, 64)
		return parsed
	case string:
		parsed, _ := strconv.ParseInt(typed, 10, 64)
		return parsed
	}
	return 0
}

func sqliteFloat64(value any) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int64:
		return float64(typed)
	case int:
		return float64(typed)
	case []byte:
		parsed, _ := strconv.ParseFloat(string(typed), 64)
		return parsed
	case string:
		parsed, _ := strconv.ParseFloat(typed, 64)
		return parsed
	}
	return 0
}

func sqliteTime(value any) (time.Time, bool) {
	micros := sqliteInt64(value)
	if micros == 0 {
		return time.Time{}, false
	}
	return time.UnixMicro(micros).UTC(), true
}

func chooseHistogramGranularity(from, to time.Time) string {
	fromYear, fromMonth, fromDay := from.Date()
	toYear, toMonth, toDay := to.Date()
	if fromYear == toYear && fromMonth == toMonth && fromDay == toDay {
		return "hour"
	}
	if fromYear == toYear && fromMonth == toMonth {
		return "day"
	}
	if to.Sub(from) > yearGranularityThreshold*365*24*time.Hour {
		return "year"
	}
	return "month"
}

func toNameCounts[T any](rows []T, extract func(T) (string, int64)) []ref.NameCount {
	out := make([]ref.NameCount, 0, len(rows))
	for _, row := range rows {
		name, count := extract(row)
		name = ref.SanitizeUserText(name, ref.MaxFacetValueLen)
		if name == "" {
			continue
		}
		out = append(out, ref.NameCount{Name: name, Count: int(count)})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
