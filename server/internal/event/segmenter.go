package event

import (
	"errors"
	"fmt"
	"math"
	"sort"
)

var ErrConstraintConflict = errors.New("event constraint conflict")

type interval struct{ left, right int }

// SegmentCandidates is pure and deterministic. Callers must provide a complete
// owner-scoped snapshot; ownership validation belongs at the storage boundary.
func SegmentCandidates(input []Candidate, constraints []Constraint, policy Policy) ([]Segment, error) {
	items := append([]Candidate(nil), input...)
	sort.Slice(items, func(i, j int) bool {
		if items[i].CapturedAt.Equal(items[j].CapturedAt) {
			return items[i].MediaItemID < items[j].MediaItemID
		}
		return items[i].CapturedAt.Before(items[j].CapturedAt)
	})
	index := make(map[string]int, len(items))
	for i, item := range items {
		if _, exists := index[item.MediaItemID]; exists {
			return nil, fmt.Errorf("%w: duplicate media %s", ErrConstraintConflict, item.MediaItemID)
		}
		index[item.MediaItemID] = i
	}

	locked := make([]bool, max(0, len(items)-1))
	cannot := make([]bool, max(0, len(items)-1))
	includes := map[string]string{}
	excludes := map[string]map[string]bool{}
	var mustSpans []interval
	for _, constraint := range constraints {
		left, ok := index[constraint.LeftMediaItemID]
		if !ok {
			return nil, fmt.Errorf("%w: stale media %s", ErrConstraintConflict, constraint.LeftMediaItemID)
		}
		switch constraint.Kind {
		case ConstraintInclude:
			if prior := includes[constraint.LeftMediaItemID]; prior != "" && prior != constraint.EventID {
				return nil, fmt.Errorf("%w: media included in two events", ErrConstraintConflict)
			}
			includes[constraint.LeftMediaItemID] = constraint.EventID
		case ConstraintExclude:
			if excludes[constraint.EventID] == nil {
				excludes[constraint.EventID] = map[string]bool{}
			}
			excludes[constraint.EventID][constraint.LeftMediaItemID] = true
		case ConstraintMustLink, ConstraintCannotLink:
			right, ok := index[constraint.RightMediaItemID]
			if !ok || left == right {
				return nil, fmt.Errorf("%w: invalid pair", ErrConstraintConflict)
			}
			if left > right {
				left, right = right, left
			}
			if constraint.Kind == ConstraintMustLink {
				mustSpans = append(mustSpans, interval{left, right})
				for edge := left; edge < right; edge++ {
					locked[edge] = true
				}
			} else {
				// A cannot-link is represented by the deterministic boundary
				// immediately before its right endpoint.
				cannot[right-1] = true
			}
		default:
			return nil, fmt.Errorf("%w: unknown kind %q", ErrConstraintConflict, constraint.Kind)
		}
	}
	for edge := range cannot {
		if cannot[edge] && locked[edge] {
			return nil, fmt.Errorf("%w: cannot-link inside must-link span", ErrConstraintConflict)
		}
	}
	for eventID, media := range excludes {
		for mediaID := range media {
			if includes[mediaID] == eventID {
				return nil, fmt.Errorf("%w: include/exclude contradiction", ErrConstraintConflict)
			}
		}
	}

	// Stacks lock automatic boundaries across their complete chronological
	// span, while an explicit cannot-link remains authoritative.
	stackBounds := map[string]interval{}
	for i, item := range items {
		if item.StackID == "" {
			continue
		}
		bounds, ok := stackBounds[item.StackID]
		if !ok {
			stackBounds[item.StackID] = interval{i, i}
		} else {
			bounds.right = i
			stackBounds[item.StackID] = bounds
		}
	}
	for _, bounds := range stackBounds {
		for edge := bounds.left; edge < bounds.right; edge++ {
			if !cannot[edge] {
				locked[edge] = true
			}
		}
	}
	_ = mustSpans

	boundaries := make([]bool, len(locked))
	for edge := range boundaries {
		if cannot[edge] {
			boundaries[edge] = true
			continue
		}
		if locked[edge] {
			continue
		}
		left, right := items[edge], items[edge+1]
		delta := right.CapturedAt.Sub(left.CapturedAt)
		if delta >= policy.HardGap {
			boundaries[edge] = true
			continue
		}
		if left.Coordinate == nil || right.Coordinate == nil {
			boundaries[edge] = delta >= policy.MissingLocationGap
			continue
		}
		distance := haversineMeters(*left.Coordinate, *right.Coordinate)
		if delta <= 0 && distance > policy.NearbyMeters {
			boundaries[edge] = true
			continue
		}
		if delta > 0 && distance/(delta.Hours()*1000) > policy.MaxTravelSpeedKPH {
			boundaries[edge] = true
			continue
		}
		if distance <= policy.NearbyMeters {
			boundaries[edge] = delta >= policy.NearbyGap
		} else {
			boundaries[edge] = delta >= policy.StrongGap
		}
	}

	// Enforce the duration cap by repeatedly selecting the weakest unlocked
	// edge in each oversized range.
	for {
		changed := false
		start := 0
		for end := 0; end < len(items); end++ {
			if end+1 < len(items) && !boundaries[end] {
				continue
			}
			if items[end].CapturedAt.Sub(items[start].CapturedAt) > policy.MaxEventDuration {
				best := -1
				for edge := start; edge < end; edge++ {
					if locked[edge] || boundaries[edge] {
						continue
					}
					if best < 0 || weakerEdge(items, edge, best) {
						best = edge
					}
				}
				if best >= 0 {
					boundaries[best], changed = true, true
				}
			}
			start = end + 1
		}
		if !changed {
			break
		}
	}

	segments := buildSegments(items, boundaries)
	// Hard labels combine segments even if non-contiguous.
	labelIndex := map[string]int{}
	for i := 0; i < len(segments); {
		labels := map[string]bool{}
		for _, mediaID := range segments[i].MediaItemIDs {
			if label := includes[mediaID]; label != "" {
				labels[label] = true
			}
		}
		if len(labels) > 1 {
			return nil, fmt.Errorf("%w: candidate has multiple include labels", ErrConstraintConflict)
		}
		var label string
		for value := range labels {
			label = value
		}
		if label == "" {
			i++
			continue
		}
		var retained []string
		var detached []string
		for _, mediaID := range segments[i].MediaItemIDs {
			if excludes[label][mediaID] {
				detached = append(detached, mediaID)
			} else {
				retained = append(retained, mediaID)
			}
		}
		if len(detached) > 0 {
			if len(retained) == 0 {
				return nil, fmt.Errorf("%w: labelled candidate contains only excluded media", ErrConstraintConflict)
			}
			segments[i] = segmentFromIDs(retained, items)
			for _, mediaID := range detached {
				segments = append(segments, segmentFromIDs([]string{mediaID}, items))
			}
		}
		segments[i].HardEventID = label
		if target, exists := labelIndex[label]; exists {
			segments[target] = mergeSegments(segments[target], segments[i], items)
			segments = append(segments[:i], segments[i+1:]...)
			continue
		}
		labelIndex[label] = i
		i++
	}
	return segments, nil
}

func segmentFromIDs(ids []string, all []Candidate) Segment {
	wanted := make(map[string]bool, len(ids))
	for _, id := range ids {
		wanted[id] = true
	}
	selected := make([]Candidate, 0, len(ids))
	for _, item := range all {
		if wanted[item.MediaItemID] {
			selected = append(selected, item)
		}
	}
	return segmentFrom(selected)
}

func buildSegments(items []Candidate, boundaries []bool) []Segment {
	if len(items) == 0 {
		return nil
	}
	var result []Segment
	start := 0
	for end := range items {
		if end+1 < len(items) && !boundaries[end] {
			continue
		}
		result = append(result, segmentFrom(items[start:end+1]))
		start = end + 1
	}
	return result
}

func segmentFrom(items []Candidate) Segment {
	ids := make([]string, len(items))
	for i := range items {
		ids[i] = items[i].MediaItemID
	}
	timezone := items[0].Timezone
	return Segment{
		MediaItemIDs: ids,
		StartAt:      items[0].CapturedAt,
		EndAt:        items[len(items)-1].CapturedAt,
		Timezone:     timezone,
		Coordinate:   sphericalCentroidCandidates(items),
	}
}

func mergeSegments(left, right Segment, all []Candidate) Segment {
	ids := append(append([]string(nil), left.MediaItemIDs...), right.MediaItemIDs...)
	order := make(map[string]int, len(all))
	for i := range all {
		order[all[i].MediaItemID] = i
	}
	sort.Slice(ids, func(i, j int) bool { return order[ids[i]] < order[ids[j]] })
	selected := make([]Candidate, 0, len(ids))
	for _, id := range ids {
		selected = append(selected, all[order[id]])
	}
	merged := segmentFrom(selected)
	merged.HardEventID = left.HardEventID
	return merged
}

func weakerEdge(items []Candidate, candidate, current int) bool {
	candidateGap := items[candidate+1].CapturedAt.Sub(items[candidate].CapturedAt)
	currentGap := items[current+1].CapturedAt.Sub(items[current].CapturedAt)
	if candidateGap != currentGap {
		return candidateGap > currentGap
	}
	candidateDistance, currentDistance := -1.0, -1.0
	if items[candidate].Coordinate != nil && items[candidate+1].Coordinate != nil {
		candidateDistance = haversineMeters(*items[candidate].Coordinate, *items[candidate+1].Coordinate)
	}
	if items[current].Coordinate != nil && items[current+1].Coordinate != nil {
		currentDistance = haversineMeters(*items[current].Coordinate, *items[current+1].Coordinate)
	}
	if candidateDistance != currentDistance {
		return candidateDistance > currentDistance
	}
	if candidate != current {
		return candidate < current
	}
	return items[candidate+1].MediaItemID < items[current+1].MediaItemID
}

func haversineMeters(a, b Coordinate) float64 {
	const radius = 6371008.8
	lat1, lat2 := a.Latitude*math.Pi/180, b.Latitude*math.Pi/180
	dlat := (b.Latitude - a.Latitude) * math.Pi / 180
	dlon := (b.Longitude - a.Longitude) * math.Pi / 180
	value := math.Sin(dlat/2)*math.Sin(dlat/2) +
		math.Cos(lat1)*math.Cos(lat2)*math.Sin(dlon/2)*math.Sin(dlon/2)
	return radius * 2 * math.Atan2(math.Sqrt(value), math.Sqrt(1-value))
}

func sphericalCentroidCandidates(items []Candidate) *Coordinate {
	var x, y, z float64
	var count int
	for _, item := range items {
		if item.Coordinate == nil {
			continue
		}
		lat := item.Coordinate.Latitude * math.Pi / 180
		lon := item.Coordinate.Longitude * math.Pi / 180
		x += math.Cos(lat) * math.Cos(lon)
		y += math.Cos(lat) * math.Sin(lon)
		z += math.Sin(lat)
		count++
	}
	if count == 0 {
		return nil
	}
	lon := math.Atan2(y, x)
	hyp := math.Sqrt(x*x + y*y)
	lat := math.Atan2(z, hyp)
	return &Coordinate{Latitude: lat * 180 / math.Pi, Longitude: lon * 180 / math.Pi}
}
