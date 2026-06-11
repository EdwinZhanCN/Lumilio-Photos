package ref

import "time"

// FacetSummary is the describe tool's output and the hydration API's ref
// metadata: the agent's only "eyesight" over a ref. All values are computed
// by SQL aggregates over the snapshot; all user-content strings must pass
// through SanitizeUserText before they land here. Serialized size budget is
// roughly 600 tokens — keep top-N limits small.
type FacetSummary struct {
	Count      int            `json:"count"`
	DateRange  *DateRange     `json:"date_range,omitempty"`
	Histogram  []Bucket       `json:"histogram,omitempty"`
	Types      map[string]int `json:"types,omitempty"`
	TopPlaces  []NameCount    `json:"top_places,omitempty"`
	TopPeople  []NameCount    `json:"top_people,omitempty"`
	Cameras    []NameCount    `json:"cameras,omitempty"`
	LikedCount int            `json:"liked_count,omitempty"`
	// RatingDist counts assets per rating 0..5; omitted when all zero.
	RatingDist []int `json:"rating_dist,omitempty"`
}

// DateRange spans min/max capture time of the snapshot.
type DateRange struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
}

// Bucket is one time-histogram bin. Bucket labels are "2025-04" (month
// granularity) or "2025" (year granularity for ranges over three years).
type Bucket struct {
	Bucket string `json:"bucket"`
	Count  int    `json:"count"`
}

// NameCount is a top-N facet value with its frequency.
type NameCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}
