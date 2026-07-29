package event

import (
	"testing"
	"time"
)

func TestReconcileRetainsUnchangedIdentity(t *testing.T) {
	now := time.Now().UTC()
	old := []PublishedEvent{{
		EventID: "old", MediaItemIDs: []string{"a", "b"},
		StartAt: now, EndAt: now.Add(time.Hour), CreatedAt: now,
	}}
	segments := []Segment{{
		MediaItemIDs: []string{"a", "b"}, StartAt: now, EndAt: now.Add(time.Hour),
	}}
	result, err := Reconcile(old, segments, nil, V1, func() string { return "new" })
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Assignments) != 1 || result.Assignments[0].EventID != "old" ||
		!result.Assignments[0].Reused || len(result.Redirects) != 0 {
		t.Fatalf("unchanged reconciliation = %#v", result)
	}
}

func TestReconcileSplitGivesCoverCandidateOldIdentity(t *testing.T) {
	now := time.Now().UTC()
	old := []PublishedEvent{{
		EventID: "old", MediaItemIDs: []string{"a", "b"}, CoverOverrideID: "b",
		StartAt: now, EndAt: now.Add(time.Hour), CreatedAt: now,
	}}
	segments := []Segment{
		{MediaItemIDs: []string{"a"}, StartAt: now, EndAt: now},
		{MediaItemIDs: []string{"b"}, StartAt: now.Add(time.Hour), EndAt: now.Add(time.Hour)},
	}
	next := 0
	result, err := Reconcile(old, segments, nil, V1, func() string {
		next++
		return "new"
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Assignments[1].EventID != "old" {
		t.Fatalf("cover split assignment = %#v", result.Assignments)
	}
}

func TestSphericalCentroidHandlesAntimeridian(t *testing.T) {
	centroid := sphericalCentroidCandidates([]Candidate{
		{Coordinate: &Coordinate{Latitude: 0, Longitude: 179}},
		{Coordinate: &Coordinate{Latitude: 0, Longitude: -179}},
	})
	if centroid == nil || centroid.Longitude > -179 && centroid.Longitude < 179 {
		t.Fatalf("antimeridian centroid = %#v", centroid)
	}
}
