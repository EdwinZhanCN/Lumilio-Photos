package event

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestSegmentCandidatesPolicyBoundaries(t *testing.T) {
	base := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	near := &Coordinate{Latitude: 40.7128, Longitude: -74.006}
	tests := []struct {
		name  string
		items []Candidate
		count int
	}{
		{
			name:  "missing GPS stays below three hours",
			items: []Candidate{{MediaItemID: "a", CapturedAt: base}, {MediaItemID: "b", CapturedAt: base.Add(2 * time.Hour)}},
			count: 1,
		},
		{
			name:  "missing GPS splits at three hours",
			items: []Candidate{{MediaItemID: "a", CapturedAt: base}, {MediaItemID: "b", CapturedAt: base.Add(3 * time.Hour)}},
			count: 2,
		},
		{
			name: "nearby GPS stays below six hours",
			items: []Candidate{
				{MediaItemID: "a", CapturedAt: base, Coordinate: near},
				{MediaItemID: "b", CapturedAt: base.Add(5 * time.Hour), Coordinate: near},
			},
			count: 1,
		},
		{
			name: "hard gap splits",
			items: []Candidate{
				{MediaItemID: "a", CapturedAt: base, Coordinate: near},
				{MediaItemID: "b", CapturedAt: base.Add(12 * time.Hour), Coordinate: near},
			},
			count: 2,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			segments, err := SegmentCandidates(test.items, nil, V1)
			if err != nil {
				t.Fatal(err)
			}
			if len(segments) != test.count {
				t.Fatalf("segments = %d, want %d", len(segments), test.count)
			}
		})
	}
}

func TestSegmentCandidatesConstraintsAndDeterminism(t *testing.T) {
	base := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	items := []Candidate{
		{MediaItemID: "c", CapturedAt: base.Add(24 * time.Hour)},
		{MediaItemID: "a", CapturedAt: base},
		{MediaItemID: "b", CapturedAt: base.Add(time.Hour)},
	}
	constraints := []Constraint{{
		Kind: ConstraintMustLink, LeftMediaItemID: "a", RightMediaItemID: "c",
	}}
	first, err := SegmentCandidates(items, constraints, V1)
	if err != nil {
		t.Fatal(err)
	}
	second, err := SegmentCandidates(items, constraints, V1)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("non-deterministic output:\n%#v\n%#v", first, second)
	}
	if len(first) != 1 || !reflect.DeepEqual(first[0].MediaItemIDs, []string{"a", "b", "c"}) {
		t.Fatalf("must-link span not preserved: %#v", first)
	}
	_, err = SegmentCandidates(items, append(constraints, Constraint{
		Kind: ConstraintCannotLink, LeftMediaItemID: "b", RightMediaItemID: "c",
	}), V1)
	if !errors.Is(err, ErrConstraintConflict) {
		t.Fatalf("must/cannot conflict = %v", err)
	}
}

func TestHardLabelCombinesNonContiguousSegments(t *testing.T) {
	base := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	items := []Candidate{
		{MediaItemID: "a", CapturedAt: base},
		{MediaItemID: "b", CapturedAt: base.Add(24 * time.Hour)},
		{MediaItemID: "c", CapturedAt: base.Add(48 * time.Hour)},
	}
	segments, err := SegmentCandidates(items, []Constraint{
		{Kind: ConstraintInclude, EventID: "event", LeftMediaItemID: "a"},
		{Kind: ConstraintInclude, EventID: "event", LeftMediaItemID: "c"},
	}, V1)
	if err != nil {
		t.Fatal(err)
	}
	if len(segments) != 2 || segments[0].HardEventID != "event" ||
		!reflect.DeepEqual(segments[0].MediaItemIDs, []string{"a", "c"}) {
		t.Fatalf("hard-labelled segments = %#v", segments)
	}
}
