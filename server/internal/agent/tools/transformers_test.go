package tools

import (
	"strings"
	"testing"
	"time"

	"server/internal/agent/ref"

	"github.com/google/uuid"
)

func TestCombineSummaryReportsOperandSizesAndDrops(t *testing.T) {
	base := &ref.Ref{AssetIDs: make([]uuid.UUID, 320)}
	other := &ref.Ref{AssetIDs: make([]uuid.UUID, 180)}
	got := combineSummary("intersect", []string{"rA", "rB"}, []*ref.Ref{base, other}, 95)
	if !strings.Contains(got, "rA[320]") || !strings.Contains(got, "rB[180]") {
		t.Errorf("missing operand sizes: %q", got)
	}
	if !strings.Contains(got, "225") { // 320 - 95 dropped from base
		t.Errorf("missing drop count: %q", got)
	}

	diff := combineSummary("diff", []string{"rA", "rB"}, []*ref.Ref{base, other}, 300)
	if !strings.Contains(diff, "20 of base rA excluded") {
		t.Errorf("diff summary = %q", diff)
	}
}

func mustTime(t *testing.T, s string) time.Time {
	t.Helper()
	parsed, err := time.Parse("2006-01-02", s)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return parsed
}

func TestBucketTimesEmpty(t *testing.T) {
	if got := bucketTimes(nil, 6); got != nil {
		t.Errorf("empty input: got %v, want nil", got)
	}
}

func TestBucketTimesSingleSpanCollapsesToOneBucket(t *testing.T) {
	day := mustTime(t, "2025-07-04")
	got := bucketTimes([]time.Time{day, day, day}, 6)
	if len(got) != 1 {
		t.Fatalf("zero-span: got %d buckets, want 1", len(got))
	}
	if got[0].Count != 3 || got[0].Bucket != "2025-07-04" {
		t.Errorf("zero-span bucket = %+v, want {2025-07-04 3}", got[0])
	}
}

func TestBucketTimesCountsAndCap(t *testing.T) {
	// A full year of monthly points; ask for more bins than the cap.
	var times []time.Time
	for m := 1; m <= 12; m++ {
		times = append(times, time.Date(2025, time.Month(m), 15, 0, 0, 0, 0, time.UTC))
	}
	got := bucketTimes(times, 100)
	if len(got) != maxSampleBuckets {
		t.Fatalf("bin count = %d, want capped at %d", len(got), maxSampleBuckets)
	}
	total := 0
	for _, b := range got {
		total += b.Count
	}
	if total != len(times) {
		t.Errorf("bucketed total = %d, want %d (every point assigned)", total, len(times))
	}
	// Span > 90 days → month labels.
	if len(got[0].Bucket) != len("2025-01") {
		t.Errorf("expected month label for a year span, got %q", got[0].Bucket)
	}
}

func TestSanitizePeople(t *testing.T) {
	if got := sanitizePeople(nil); got != "" {
		t.Errorf("nil people = %q, want empty", got)
	}
	if got := sanitizePeople([]string{"Alice"}); got != "with Alice" {
		t.Errorf("single = %q, want %q", got, "with Alice")
	}
	// More than maxPeekPeople names → truncated with a trailing marker.
	got := sanitizePeople([]string{"A", "B", "C", "D"})
	if got != "with A, B, C +" {
		t.Errorf("overflow = %q, want %q", got, "with A, B, C +")
	}
}
