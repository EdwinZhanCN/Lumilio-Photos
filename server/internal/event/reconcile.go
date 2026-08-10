package event

import (
	"fmt"
	"sort"
	"time"
)

type scoredPair struct {
	oldIndex     int
	segmentIndex int
	score        float64
	shared       int
	cover        bool
}

// Reconcile assigns stable identities using the events-v1 greedy contract.
// newID is invoked in segment order only for segments that cannot retain an ID.
func Reconcile(old []PublishedEvent, segments []Segment, constraints []Constraint, policy Policy, newID func() string) (ReconcileResult, error) {
	result := ReconcileResult{}
	oldByID := make(map[string]int, len(old))
	for i := range old {
		oldByID[old[i].EventID] = i
	}
	assignedOld := make([]bool, len(old))
	assignedSegment := make([]bool, len(segments))
	eventForSegment := make([]string, len(segments))
	excluded := map[string]map[string]bool{}
	for _, constraint := range constraints {
		if constraint.Kind == ConstraintExclude {
			if excluded[constraint.EventID] == nil {
				excluded[constraint.EventID] = map[string]bool{}
			}
			excluded[constraint.EventID][constraint.LeftMediaItemID] = true
		}
	}

	// Hard labels are preassigned regardless of score.
	for segmentIndex, segment := range segments {
		if segment.HardEventID == "" {
			continue
		}
		oldIndex, ok := oldByID[segment.HardEventID]
		if !ok || assignedOld[oldIndex] {
			return ReconcileResult{}, fmt.Errorf("%w: invalid hard event label", ErrConstraintConflict)
		}
		if segmentContainsAny(segment, excluded[segment.HardEventID]) {
			return ReconcileResult{}, fmt.Errorf("%w: hard label violates exclude", ErrConstraintConflict)
		}
		assignedOld[oldIndex], assignedSegment[segmentIndex] = true, true
		eventForSegment[segmentIndex] = segment.HardEventID
		result.Assignments = append(result.Assignments, Assignment{segment.HardEventID, segmentIndex, true})
	}

	pairs := make([]scoredPair, 0)
	oldDegree := make([]int, len(old))
	segmentDegree := make([]int, len(segments))
	for oldIndex := range old {
		if assignedOld[oldIndex] {
			continue
		}
		for segmentIndex := range segments {
			if assignedSegment[segmentIndex] || segmentContainsAny(segments[segmentIndex], excluded[old[oldIndex].EventID]) {
				continue
			}
			shared := sharedCount(old[oldIndex].MediaItemIDs, segments[segmentIndex].MediaItemIDs)
			if shared == 0 {
				continue
			}
			pair := scoredPair{
				oldIndex:     oldIndex,
				segmentIndex: segmentIndex,
				score:        reconcileScore(old[oldIndex], segments[segmentIndex], policy),
				shared:       shared,
				cover:        contains(segments[segmentIndex].MediaItemIDs, old[oldIndex].CoverOverrideID),
			}
			pairs = append(pairs, pair)
			oldDegree[oldIndex]++
			segmentDegree[segmentIndex]++
		}
	}
	sort.SliceStable(pairs, func(i, j int) bool {
		a, b := pairs[i], pairs[j]
		// One-old/many split ordering.
		if a.oldIndex == b.oldIndex && oldDegree[a.oldIndex] > 1 {
			if a.cover != b.cover {
				return a.cover
			}
			if a.shared != b.shared {
				return a.shared > b.shared
			}
			if a.score != b.score {
				return a.score > b.score
			}
			return firstID(segments[a.segmentIndex]) < firstID(segments[b.segmentIndex])
		}
		if a.cover != b.cover {
			return a.cover
		}
		if old[a.oldIndex].HasUserState != old[b.oldIndex].HasUserState {
			return old[a.oldIndex].HasUserState
		}
		if a.score != b.score {
			return a.score > b.score
		}
		if a.shared != b.shared {
			return a.shared > b.shared
		}
		if !old[a.oldIndex].CreatedAt.Equal(old[b.oldIndex].CreatedAt) {
			return old[a.oldIndex].CreatedAt.Before(old[b.oldIndex].CreatedAt)
		}
		if old[a.oldIndex].EventID != old[b.oldIndex].EventID {
			return old[a.oldIndex].EventID < old[b.oldIndex].EventID
		}
		return firstID(segments[a.segmentIndex]) < firstID(segments[b.segmentIndex])
	})
	for _, pair := range pairs {
		if assignedOld[pair.oldIndex] || assignedSegment[pair.segmentIndex] {
			continue
		}
		// Threshold applies only to isolated one-to-one components.
		if oldDegree[pair.oldIndex] == 1 && segmentDegree[pair.segmentIndex] == 1 &&
			pair.score < policy.ReconcileThreshold {
			continue
		}
		eventID := old[pair.oldIndex].EventID
		assignedOld[pair.oldIndex], assignedSegment[pair.segmentIndex] = true, true
		eventForSegment[pair.segmentIndex] = eventID
		result.Assignments = append(result.Assignments, Assignment{eventID, pair.segmentIndex, true})
	}
	for segmentIndex := range segments {
		if assignedSegment[segmentIndex] {
			continue
		}
		id := newID()
		if id == "" {
			return ReconcileResult{}, fmt.Errorf("allocate event ID")
		}
		assignedSegment[segmentIndex] = true
		eventForSegment[segmentIndex] = id
		result.Assignments = append(result.Assignments, Assignment{id, segmentIndex, false})
	}

	for oldIndex := range old {
		if assignedOld[oldIndex] {
			continue
		}
		target := -1
		bestShared := -1
		bestScore := -1.0
		for segmentIndex := range segments {
			if segmentContainsAny(segments[segmentIndex], excluded[old[oldIndex].EventID]) {
				continue
			}
			shared := sharedCount(old[oldIndex].MediaItemIDs, segments[segmentIndex].MediaItemIDs)
			score := reconcileScore(old[oldIndex], segments[segmentIndex], policy)
			if shared > bestShared || (shared == bestShared && score > bestScore) ||
				(shared == bestShared && score == bestScore &&
					(target < 0 || firstID(segments[segmentIndex]) < firstID(segments[target]))) {
				target, bestShared, bestScore = segmentIndex, shared, score
			}
		}
		if target < 0 || bestShared == 0 {
			// The old identity has no factual overlap with the new
			// topology. It is retired rather than redirected to an
			// unrelated Event.
			continue
		}
		result.Redirects = append(result.Redirects, Redirect{old[oldIndex].EventID, eventForSegment[target]})
	}
	sort.Slice(result.Assignments, func(i, j int) bool {
		return result.Assignments[i].SegmentIndex < result.Assignments[j].SegmentIndex
	})
	sort.Slice(result.Redirects, func(i, j int) bool {
		return result.Redirects[i].OldEventID < result.Redirects[j].OldEventID
	})
	return result, nil
}

func reconcileScore(old PublishedEvent, segment Segment, policy Policy) float64 {
	intersection := sharedCount(old.MediaItemIDs, segment.MediaItemIDs)
	union := len(old.MediaItemIDs) + len(segment.MediaItemIDs) - intersection
	jaccard := 0.0
	if union > 0 {
		jaccard = float64(intersection) / float64(union)
	}
	timeScore := intervalIoU(old.StartAt, old.EndAt, segment.StartAt, segment.EndAt)
	geo := 0.0
	if old.Coordinate != nil && segment.Coordinate != nil {
		geo = 1 - min(haversineMeters(*old.Coordinate, *segment.Coordinate)/policy.NearbyMeters, 1)
	}
	return .7*jaccard + .2*timeScore + .1*geo
}

func intervalIoU(aStart, aEnd, bStart, bEnd time.Time) float64 {
	if aStart.Equal(aEnd) && bStart.Equal(bEnd) {
		if aStart.Equal(bStart) {
			return 1
		}
		return 0
	}
	start := aStart
	if bStart.After(start) {
		start = bStart
	}
	end := aEnd
	if bEnd.Before(end) {
		end = bEnd
	}
	intersection := end.Sub(start)
	if intersection < 0 {
		intersection = 0
	}
	unionStart, unionEnd := aStart, aEnd
	if bStart.Before(unionStart) {
		unionStart = bStart
	}
	if bEnd.After(unionEnd) {
		unionEnd = bEnd
	}
	union := unionEnd.Sub(unionStart)
	if union <= 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}

func sharedCount(left, right []string) int {
	set := make(map[string]bool, len(left))
	for _, id := range left {
		set[id] = true
	}
	count := 0
	for _, id := range right {
		if set[id] {
			count++
		}
	}
	return count
}

func segmentContainsAny(segment Segment, prohibited map[string]bool) bool {
	for _, id := range segment.MediaItemIDs {
		if prohibited[id] {
			return true
		}
	}
	return false
}

func contains(values []string, value string) bool {
	if value == "" {
		return false
	}
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

func firstID(segment Segment) string {
	if len(segment.MediaItemIDs) == 0 {
		return ""
	}
	return segment.MediaItemIDs[0]
}
