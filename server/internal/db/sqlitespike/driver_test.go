package sqlitespike

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"

	sqlitevec "github.com/asg017/sqlite-vec-go-bindings/cgo"
)

func TestSQLiteDriverPragmasStrictJSONAndVector(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := openTestDatabase(t, ctx)

	if got := database.Stats().MaxOpenConnections; got != 1 {
		t.Fatalf("MaxOpenConnections = %d, want 1", got)
	}

	assertPragmaInt(t, database, "foreign_keys", 1)
	assertPragmaText(t, database, "journal_mode", "wal")
	assertPragmaInt(t, database, "synchronous", 1)
	assertPragmaInt(t, database, "busy_timeout", 5000)
	assertPragmaInt(t, database, "temp_store", 2)
	assertPragmaInt(t, database, "wal_autocheckpoint", 1000)

	var sqliteVersion string
	var vectorVersion string
	if err := database.QueryRowContext(ctx, "SELECT sqlite_version(), vec_version()").Scan(&sqliteVersion, &vectorVersion); err != nil {
		t.Fatalf("query SQLite/vector versions: %v", err)
	}
	if sqliteVersion == "" {
		t.Fatal("sqlite_version() returned an empty version")
	}
	if vectorVersion != "v0.1.6" {
		t.Fatalf("vec_version() = %q, want v0.1.6", vectorVersion)
	}

	if _, err := database.ExecContext(ctx, `
		CREATE TABLE strict_json (
			id INTEGER PRIMARY KEY,
			payload TEXT NOT NULL CHECK(json_valid(payload))
		) STRICT
	`); err != nil {
		t.Fatalf("create STRICT JSON table: %v", err)
	}
	if _, err := database.ExecContext(ctx, "INSERT INTO strict_json (id, payload) VALUES (?, ?)", 1, `{"name":"sqlite"}`); err != nil {
		t.Fatalf("insert valid JSON: %v", err)
	}
	if _, err := database.ExecContext(ctx, "INSERT INTO strict_json (id, payload) VALUES (?, ?)", 2, `{"broken"`); err == nil {
		t.Fatal("insert invalid JSON error = nil, want CHECK failure")
	}
	var jsonName string
	if err := database.QueryRowContext(ctx, "SELECT json_extract(payload, '$.name') FROM strict_json WHERE id = 1").Scan(&jsonName); err != nil {
		t.Fatalf("query JSON1 value: %v", err)
	}
	if jsonName != "sqlite" {
		t.Fatalf("json_extract name = %q, want sqlite", jsonName)
	}

	testVectorDimensions(t, ctx, database, 768)
	testVectorDimensions(t, ctx, database, 512)
}

func openTestDatabase(t *testing.T, ctx context.Context) *sql.DB {
	t.Helper()

	database, err := Open(ctx, filepath.Join(t.TempDir(), "library.sqlite3"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() {
		if err := database.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})
	return database
}

func assertPragmaInt(t *testing.T, database *sql.DB, name string, want int) {
	t.Helper()

	var got int
	if err := database.QueryRow("PRAGMA " + name).Scan(&got); err != nil {
		t.Fatalf("query PRAGMA %s: %v", name, err)
	}
	if got != want {
		t.Fatalf("PRAGMA %s = %d, want %d", name, got, want)
	}
}

func assertPragmaText(t *testing.T, database *sql.DB, name, want string) {
	t.Helper()

	var got string
	if err := database.QueryRow("PRAGMA " + name).Scan(&got); err != nil {
		t.Fatalf("query PRAGMA %s: %v", name, err)
	}
	if got != want {
		t.Fatalf("PRAGMA %s = %q, want %q", name, got, want)
	}
}

func testVectorDimensions(t *testing.T, ctx context.Context, database *sql.DB, dimensions int) {
	t.Helper()

	table := fmt.Sprintf("vectors_%d", dimensions)
	if _, err := database.ExecContext(ctx, fmt.Sprintf(
		"CREATE VIRTUAL TABLE %s USING vec0(embedding float[%d])",
		table,
		dimensions,
	)); err != nil {
		t.Fatalf("create %dD vec0 table: %v", dimensions, err)
	}

	vectors := map[int][]float32{
		1: constantVector(dimensions, 0),
		2: constantVector(dimensions, 0.25),
		3: constantVector(dimensions, 1),
	}
	for rowID, vector := range vectors {
		serialized, err := sqlitevec.SerializeFloat32(vector)
		if err != nil {
			t.Fatalf("serialize %dD vector: %v", dimensions, err)
		}
		if _, err := database.ExecContext(
			ctx,
			fmt.Sprintf("INSERT INTO %s (rowid, embedding) VALUES (?, ?)", table),
			rowID,
			serialized,
		); err != nil {
			t.Fatalf("insert %dD vector row %d: %v", dimensions, rowID, err)
		}
	}

	query, err := sqlitevec.SerializeFloat32(constantVector(dimensions, 0.2))
	if err != nil {
		t.Fatalf("serialize %dD query: %v", dimensions, err)
	}
	got := vectorTopK(t, ctx, database, table, query, 2)
	want := []int64{2, 1}
	if encoded, _ := json.Marshal(got); string(encoded) != "[2,1]" {
		t.Fatalf("%dD top-k rowids = %v, want %v", dimensions, got, want)
	}

	if _, err := database.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE rowid = 2", table)); err != nil {
		t.Fatalf("delete %dD nearest vector: %v", dimensions, err)
	}
	got = vectorTopK(t, ctx, database, table, query, 2)
	if encoded, _ := json.Marshal(got); string(encoded) != "[1,3]" {
		t.Fatalf("%dD top-k after delete = %v, want [1 3]", dimensions, got)
	}
}

func constantVector(dimensions int, value float32) []float32 {
	vector := make([]float32, dimensions)
	for index := range vector {
		vector[index] = value
	}
	return vector
}

func vectorTopK(t *testing.T, ctx context.Context, database *sql.DB, table string, query []byte, limit int) []int64 {
	t.Helper()

	rows, err := database.QueryContext(ctx, fmt.Sprintf(`
		SELECT rowid
		FROM %s
		WHERE embedding MATCH ?
		ORDER BY distance
		LIMIT ?
	`, table), query, limit)
	if err != nil {
		t.Fatalf("query %s top-k: %v", table, err)
	}
	defer rows.Close()

	var rowIDs []int64
	for rows.Next() {
		var rowID int64
		if err := rows.Scan(&rowID); err != nil {
			t.Fatalf("scan %s top-k: %v", table, err)
		}
		rowIDs = append(rowIDs, rowID)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate %s top-k: %v", table, err)
	}
	return rowIDs
}
