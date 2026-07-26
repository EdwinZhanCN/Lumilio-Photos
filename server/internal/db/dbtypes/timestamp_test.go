package dbtypes

import (
	"testing"
	"time"
)

func TestTimestampRoundTrip(t *testing.T) {
	t.Parallel()

	input := time.Date(2026, time.July, 25, 14, 33, 19, 987654321, time.FixedZone("offset", -4*60*60))
	value := NewTimestamp(input)

	stored, err := value.Value()
	if err != nil {
		t.Fatalf("Value() error = %v", err)
	}

	var roundTrip Timestamp
	if err := roundTrip.Scan(stored); err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	want := input.UTC().Truncate(time.Microsecond)
	if !roundTrip.Time.Equal(want) {
		t.Fatalf("roundTrip = %s, want %s", roundTrip.Time, want)
	}
	if roundTrip.Location() != time.UTC {
		t.Fatalf("roundTrip location = %s, want UTC", roundTrip.Location())
	}
}

func TestTimestampRejectsInvalidValues(t *testing.T) {
	t.Parallel()

	var value Timestamp
	if err := value.Scan(nil); err == nil {
		t.Fatal("Scan(nil) error = nil, want unsupported source error")
	}
	if _, err := value.Value(); err == nil {
		t.Fatal("zero Timestamp.Value() error = nil, want error")
	}
}
