// Package facets computes ref.FacetSummary aggregates over a ref snapshot.
// It is shared by the describe tool (the agent's eyesight) and the hydration
// API (ref metadata for the frontend). All user-content strings are passed
// through ref.SanitizeUserText here, so callers can emit values as-is (INV-7).
package facets

import (
	"context"
	"time"

	"server/internal/agent/ref"
	"server/internal/db/repo"

	"github.com/jackc/pgx/v5/pgtype"
)

const (
	topPlaces  = 5
	topPeople  = 5
	topCameras = 3
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

	assetIDs := make([]pgtype.UUID, len(r.AssetIDs))
	for i, id := range r.AssetIDs {
		assetIDs[i] = pgtype.UUID{Bytes: id, Valid: true}
	}

	overview, err := queries.AgentFacetOverview(ctx, assetIDs)
	if err != nil {
		return nil, err
	}
	summary.LikedCount = int(overview.LikedCount)

	granularity := "month"
	if overview.DateFrom.Valid && overview.DateTo.Valid {
		summary.DateRange = &ref.DateRange{
			From: overview.DateFrom.Time.UTC(),
			To:   overview.DateTo.Time.UTC(),
		}
		if cm := overview.CaptureOffsetMinutes; cm != 0 {
			offset := int16(cm)
			summary.DateRange.OffsetMinutes = &offset
		}
		granularity = chooseHistogramGranularity(overview.DateFrom.Time, overview.DateTo.Time)
	}
	summary.HistogramGranularity = granularity

	if buckets, err := queries.AgentFacetTimeHistogram(ctx, repo.AgentFacetTimeHistogramParams{
		Granularity: granularity,
		AssetIds:    assetIDs,
	}); err == nil {
		summary.Histogram = make([]ref.Bucket, 0, len(buckets))
		for _, b := range buckets {
			summary.Histogram = append(summary.Histogram, ref.Bucket{Bucket: b.Bucket, Count: int(b.Count)})
		}
	}

	if types, err := queries.AgentFacetTypeCounts(ctx, assetIDs); err == nil && len(types) > 0 {
		summary.Types = make(map[string]int, len(types))
		for _, t := range types {
			summary.Types[t.Type] = int(t.Count)
		}
	}

	if places, err := queries.AgentFacetTopPlaces(ctx, repo.AgentFacetTopPlacesParams{
		AssetIds: assetIDs,
		TopN:     topPlaces,
	}); err == nil {
		summary.TopPlaces = toNameCounts(places, func(row repo.AgentFacetTopPlacesRow) (string, int64) {
			return row.Name, row.Count
		})
	}

	if people, err := queries.AgentFacetTopPeople(ctx, repo.AgentFacetTopPeopleParams{
		AssetIds: assetIDs,
		TopN:     topPeople,
	}); err == nil {
		summary.TopPeople = toNameCounts(people, func(row repo.AgentFacetTopPeopleRow) (string, int64) {
			return row.Name, row.Count
		})
	}

	if cameras, err := queries.AgentFacetCameraCounts(ctx, repo.AgentFacetCameraCountsParams{
		AssetIds: assetIDs,
		TopN:     topCameras,
	}); err == nil {
		summary.Cameras = toNameCounts(cameras, func(row repo.AgentFacetCameraCountsRow) (string, int64) {
			return row.Name, row.Count
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

	return summary, nil
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
