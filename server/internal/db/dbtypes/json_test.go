package dbtypes

import (
	"database/sql/driver"
	"encoding/json"
	"testing"
)

func TestJSONRoundTripUsesTextStorage(t *testing.T) {
	t.Parallel()

	input := JSON(`{"kind":"sqlite"}`)
	stored, err := input.Value()
	if err != nil {
		t.Fatalf("Value() error = %v", err)
	}
	if _, ok := stored.(string); !ok {
		t.Fatalf("Value() type = %T, want string", stored)
	}

	var roundTrip JSON
	if err := roundTrip.Scan(stored); err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if string(roundTrip) != string(input) {
		t.Fatalf("roundTrip = %s, want %s", roundTrip, input)
	}

	encoded, err := json.Marshal(roundTrip)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if string(encoded) != string(input) {
		t.Fatalf("json.Marshal() = %s, want %s", encoded, input)
	}
}

func TestJSONRejectsInvalidValues(t *testing.T) {
	t.Parallel()

	for name, test := range map[string]func() error{
		"scan": func() error {
			var value JSON
			return value.Scan(`{"broken"`)
		},
		"value": func() error {
			_, err := JSON(`{"broken"`).Value()
			return err
		},
		"unmarshal": func() error {
			var value JSON
			return value.UnmarshalJSON([]byte(`{"broken"`))
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if err := test(); err == nil {
				t.Fatal("error = nil, want invalid JSON error")
			}
		})
	}
}

var _ driver.Valuer = JSON(nil)
