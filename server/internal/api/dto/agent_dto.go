package dto

import (
	"time"

	"server/internal/agent/ref"
)

// AgentRefDTO is the hydration metadata for one agent ref: the handle, its
// cardinality, provenance and facet summary. Asset data is served separately
// by the paginated /assets endpoint.
type AgentRefDTO struct {
	RefID     string             `json:"ref_id" example:"r3_kyoto"`
	Count     int                `json:"count" example:"97"`
	Truncated bool               `json:"truncated,omitempty"`
	Op        string             `json:"op,omitempty" example:"combine"`
	CreatedAt time.Time          `json:"created_at"`
	Facets    *AgentRefFacetsDTO `json:"facets,omitempty"`
}

// AgentRefFacetsDTO mirrors ref.FacetSummary for the wire.
type AgentRefFacetsDTO struct {
	Count                int                   `json:"count"`
	DateRange            *AgentDateRangeDTO    `json:"date_range,omitempty"`
	Histogram            []AgentFacetBucket    `json:"histogram,omitempty"`
	HistogramGranularity string                `json:"histogram_granularity,omitempty" example:"day" enums:"hour,day,month,year"`
	Types                map[string]int        `json:"types,omitempty"`
	TopPlaces            []AgentNameCountDTO   `json:"top_places,omitempty"`
	TopPeople            []AgentNameCountDTO   `json:"top_people,omitempty"`
	Cameras              []AgentNameCountDTO   `json:"cameras,omitempty"`
	FocalLengths         []AgentNameCountDTO   `json:"focal_lengths,omitempty"`
	Lenses               []AgentNameCountDTO   `json:"lenses,omitempty"`
	LikedCount           int                   `json:"liked_count,omitempty"`
	RatingDist           []int                 `json:"rating_dist,omitempty"`
	Quality              *AgentQualityStatsDTO `json:"quality,omitempty"`
}

// AgentQualityStatsDTO mirrors ref.QualityStats for the wire: the aesthetic
// score distribution over a ref (percentiles over scored assets only).
type AgentQualityStatsDTO struct {
	Scored   int     `json:"scored" example:"318"`
	Unscored int     `json:"unscored" example:"2"`
	P25      float64 `json:"p25" example:"5.1"`
	P50      float64 `json:"p50" example:"5.7"`
	P75      float64 `json:"p75" example:"6.3"`
	P90      float64 `json:"p90" example:"7"`
}

// AgentDateRangeDTO spans the snapshot's capture times.
type AgentDateRangeDTO struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
}

// AgentFacetBucket is one time-histogram bin ("2025-04" or "2025").
type AgentFacetBucket struct {
	Bucket string `json:"bucket" example:"2025-04"`
	Count  int    `json:"count" example:"12"`
}

// AgentNameCountDTO is a top-N facet value.
type AgentNameCountDTO struct {
	Name  string `json:"name" example:"Kyoto"`
	Count int    `json:"count" example:"42"`
}

// AgentRefAssetsDTO is one hydration page of a ref, in snapshot order.
type AgentRefAssetsDTO struct {
	Assets     []AssetDTO    `json:"assets"`
	Total      int           `json:"total" example:"97"`
	Pagination PaginationDTO `json:"pagination"`
}

// AgentPinLayoutDTO is a react-grid-layout cell.
type AgentPinLayoutDTO struct {
	X int `json:"x" example:"0"`
	Y int `json:"y" example:"0"`
	W int `json:"w" example:"4"`
	H int `json:"h" example:"4"`
}

// AgentPinDTO is a pinned widget on the agent board: a durable ref snapshot
// plus widget type and grid placement.
type AgentPinDTO struct {
	PinID     string             `json:"pin_id" example:"7d4df41e-9aa2-4d44-9a3d-111111111111"`
	Title     string             `json:"title" example:"Kyoto 2025"`
	Widget    string             `json:"widget" example:"cover_card"`
	Mode      string             `json:"mode" example:"frozen" enums:"frozen,live"`
	Count     int                `json:"count" example:"24"`
	Summary   string             `json:"summary,omitempty"`
	Truncated bool               `json:"truncated,omitempty"`
	Layout    AgentPinLayoutDTO  `json:"layout"`
	Facets    *AgentRefFacetsDTO `json:"facets,omitempty"`
	CreatedAt time.Time          `json:"created_at"`
}

// CreateAgentPinRequest pins a session ref onto the board.
type CreateAgentPinRequest struct {
	RefID    string             `json:"ref_id" binding:"required"`
	ThreadID string             `json:"thread_id" binding:"required"`
	Title    string             `json:"title,omitempty"`
	Widget   string             `json:"widget,omitempty" example:"cover_card"`
	Mode     string             `json:"mode,omitempty" enums:"frozen,live"`
	Layout   *AgentPinLayoutDTO `json:"layout,omitempty"`
}

// UpdateAgentPinLayoutRequest persists board layout changes in bulk.
type UpdateAgentPinLayoutRequest struct {
	Layouts []AgentPinLayoutItemDTO `json:"layouts" binding:"required"`
}

// UpdateAgentPinRequest patches a single pin. Both fields are optional: title
// renames the widget; widget switches which view the pinned ref renders through.
type UpdateAgentPinRequest struct {
	Title  *string `json:"title,omitempty" example:"Kyoto 2025"`
	Widget *string `json:"widget,omitempty" example:"number_card"`
}

// AgentPinLayoutItemDTO is one pin's new grid cell.
type AgentPinLayoutItemDTO struct {
	PinID string `json:"pin_id" binding:"required"`
	X     int    `json:"x"`
	Y     int    `json:"y"`
	W     int    `json:"w"`
	H     int    `json:"h"`
}

// ToAgentRefFacetsDTO converts the internal facet summary to its wire shape.
func ToAgentRefFacetsDTO(s *ref.FacetSummary) *AgentRefFacetsDTO {
	if s == nil {
		return nil
	}
	out := &AgentRefFacetsDTO{
		Count:                s.Count,
		HistogramGranularity: s.HistogramGranularity,
		Types:                s.Types,
		LikedCount:           s.LikedCount,
		RatingDist:           s.RatingDist,
	}
	if s.DateRange != nil {
		out.DateRange = &AgentDateRangeDTO{From: s.DateRange.From, To: s.DateRange.To}
	}
	for _, b := range s.Histogram {
		out.Histogram = append(out.Histogram, AgentFacetBucket{Bucket: b.Bucket, Count: b.Count})
	}
	out.TopPlaces = toAgentNameCounts(s.TopPlaces)
	out.TopPeople = toAgentNameCounts(s.TopPeople)
	out.Cameras = toAgentNameCounts(s.Cameras)
	out.FocalLengths = toAgentNameCounts(s.FocalLengths)
	out.Lenses = toAgentNameCounts(s.Lenses)
	if s.Quality != nil {
		out.Quality = &AgentQualityStatsDTO{
			Scored:   s.Quality.Scored,
			Unscored: s.Quality.Unscored,
			P25:      s.Quality.P25,
			P50:      s.Quality.P50,
			P75:      s.Quality.P75,
			P90:      s.Quality.P90,
		}
	}
	return out
}

func toAgentNameCounts(in []ref.NameCount) []AgentNameCountDTO {
	if len(in) == 0 {
		return nil
	}
	out := make([]AgentNameCountDTO, len(in))
	for i, nc := range in {
		out[i] = AgentNameCountDTO{Name: nc.Name, Count: nc.Count}
	}
	return out
}
