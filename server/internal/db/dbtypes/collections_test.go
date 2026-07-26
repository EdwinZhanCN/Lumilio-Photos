package dbtypes

import (
	"testing"

	"github.com/google/uuid"
)

func TestSQLiteCollectionAndVectorTypes(t *testing.T) {
	t.Parallel()

	ids := UUIDs{uuid.New(), uuid.New()}
	storedIDs, err := ids.Value()
	if err != nil {
		t.Fatalf("UUIDs.Value() error = %v", err)
	}
	var scannedIDs UUIDs
	if err := scannedIDs.Scan(storedIDs); err != nil {
		t.Fatalf("UUIDs.Scan() error = %v", err)
	}
	if len(scannedIDs) != len(ids) || scannedIDs[0] != ids[0] || scannedIDs[1] != ids[1] {
		t.Fatalf("UUID round trip = %v, want %v", scannedIDs, ids)
	}

	strings := Strings{"one", "two"}
	storedStrings, err := strings.Value()
	if err != nil {
		t.Fatalf("Strings.Value() error = %v", err)
	}
	var scannedStrings Strings
	if err := scannedStrings.Scan(storedStrings); err != nil {
		t.Fatalf("Strings.Scan() error = %v", err)
	}
	if len(scannedStrings) != 2 || scannedStrings[0] != "one" || scannedStrings[1] != "two" {
		t.Fatalf("string round trip = %v", scannedStrings)
	}

	vector := Vector{0.25, -1, 4.5}
	storedVector, err := vector.Value()
	if err != nil {
		t.Fatalf("Vector.Value() error = %v", err)
	}
	var scannedVector Vector
	if err := scannedVector.Scan(storedVector); err != nil {
		t.Fatalf("Vector.Scan() error = %v", err)
	}
	if len(scannedVector) != len(vector) || scannedVector[0] != vector[0] || scannedVector[2] != vector[2] {
		t.Fatalf("vector round trip = %v, want %v", scannedVector, vector)
	}
}
