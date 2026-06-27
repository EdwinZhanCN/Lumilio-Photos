package ref

import "time"

// FacetSummary is the describe tool's output and the hydration API's ref
// metadata: the agent's only "eyesight" over a ref. All values are computed
// by SQL aggregates over the snapshot; all user-content strings must pass
// through SanitizeUserText before they land here. Serialized size budget is
// roughly 600 tokens — keep top-N limits small.
type FacetSummary struct {
	Count     int        `json:"count"`
	DateRange *DateRange `json:"date_range,omitempty"`
	Histogram []Bucket   `json:"histogram,omitempty"`
	// HistogramGranularity is one of hour, day, month or year.
	HistogramGranularity string         `json:"histogram_granularity,omitempty"`
	Types                map[string]int `json:"types,omitempty"`
	TopPlaces            []NameCount    `json:"top_places,omitempty"`
	TopPeople            []NameCount    `json:"top_people,omitempty"`
	Cameras              []NameCount    `json:"cameras,omitempty"`
	// FocalLengths and Lenses are the most-used gear settings (top-N), so the
	// agent can answer "most-used focal length / lens" without per-asset inspect.
	FocalLengths []NameCount `json:"focal_lengths,omitempty"`
	Lenses       []NameCount `json:"lenses,omitempty"`
	LikedCount   int         `json:"liked_count,omitempty"`
	// RatingDist counts assets per rating 0..5; omitted when all zero.
	RatingDist []int `json:"rating_dist,omitempty"`
	// Quality summarizes the aesthetic-score distribution; omitted when no
	// asset in the set is scored.
	Quality *QualityStats `json:"quality,omitempty"`
}

// QualityStats is the aesthetic-score distribution over a ref. Percentiles are
// computed over scored assets only; Unscored counts the rest of the snapshot.
// Scores run 1-10 and cluster in the 5-7 range — read percentiles as the
// shape of the set, not as pass/fail grades.
type QualityStats struct {
	Scored   int     `json:"scored"`
	Unscored int     `json:"unscored"`
	P25      float64 `json:"p25"`
	P50      float64 `json:"p50"`
	P75      float64 `json:"p75"`
	P90      float64 `json:"p90"`
}

// DateRange spans min/max capture time of the snapshot.
// Fields are time.Time (RFC3339, always UTC) plus the capture timezone offset
// in minutes from UTC (e.g. +480 for CST). The agent can reconstruct local
// time as From.Add(time.Duration(OffsetMinutes)*time.Minute).
type DateRange struct {
	From          time.Time `json:"from"`
	To            time.Time `json:"to"`
	OffsetMinutes *int16    `json:"offset_minutes,omitempty"`
}

// Bucket is one adaptive time-histogram bin. Bucket labels match the
// granularity: hour "2025-09-28 14:00", day "2025-09-28", month "2025-09",
// or year "2025".
type Bucket struct {
	Bucket string `json:"bucket"`
	Count  int    `json:"count"`
}

// NameCount is a top-N facet value with its frequency.
type NameCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}
