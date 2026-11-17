package vec1ext

import (
	"database/sql"
	"encoding/binary"
	"math"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestStaticRegistrationAndDistances(t *testing.T) {
	Auto()

	database, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	var info string
	if err := database.QueryRow("SELECT vec1_info()").Scan(&info); err != nil {
		t.Fatalf("probe Vec1: %v", err)
	}
	if !strings.Contains(info, "version 0.7") {
		t.Fatalf("vec1_info() = %q, want version 0.7", info)
	}

	a := vectorBlob(1, 0)
	b := vectorBlob(0, 1)
	var squaredL2, cosine float64
	if err := database.QueryRow(
		"SELECT vec1_l2_distance(?, ?), vec1_cos_distance(?, ?)",
		a, b, a, b,
	).Scan(&squaredL2, &cosine); err != nil {
		t.Fatalf("query Vec1 distances: %v", err)
	}
	if math.Abs(squaredL2-2) > 1e-6 || math.Abs(cosine-1) > 1e-6 {
		t.Fatalf("Vec1 distances = l2²:%f cosine:%f, want 2 and 1", squaredL2, cosine)
	}
}

func TestVec1VirtualTableLifecycle(t *testing.T) {
	Auto()

	database, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	if _, err := database.Exec(`
		CREATE VIRTUAL TABLE vectors USING vec1(vector, owner_id);
		INSERT INTO vectors(rowid, vector, owner_id) VALUES
		  (1, ?, 1),
		  (2, ?, 2),
		  (3, ?, 1);
		INSERT INTO vectors(cmd, arg)
		VALUES ('rebuild', '{index:"flat", distance:"l2"}');
	`, vectorBlob(0, 0), vectorBlob(10, 10), vectorBlob(1, 1)); err != nil {
		t.Fatalf("build Vec1 table: %v", err)
	}

	var rowID int64
	if err := database.QueryRow(`
		SELECT rowid
		FROM vectors(?, '{k:1}')
		WHERE owner_id = 1
	`, vectorBlob(0.9, 0.9)).Scan(&rowID); err != nil {
		t.Fatalf("query Vec1 table: %v", err)
	}
	if rowID != 3 {
		t.Fatalf("nearest rowid = %d, want 3", rowID)
	}

	if _, err := database.Exec("DELETE FROM vectors WHERE rowid = 3"); err != nil {
		t.Fatalf("delete Vec1 row: %v", err)
	}
	if err := database.QueryRow(`
		SELECT rowid
		FROM vectors(?, '{k:1}')
		WHERE owner_id = 1
	`, vectorBlob(0.9, 0.9)).Scan(&rowID); err != nil {
		t.Fatalf("query Vec1 after delete: %v", err)
	}
	if rowID != 1 {
		t.Fatalf("nearest rowid after delete = %d, want 1", rowID)
	}
}

func vectorBlob(values ...float32) []byte {
	blob := make([]byte, len(values)*4)
	for i, value := range values {
		binary.LittleEndian.PutUint32(blob[i*4:], math.Float32bits(value))
	}
	return blob
}
