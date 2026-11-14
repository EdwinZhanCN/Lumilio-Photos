package db

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var (
	unresolvedSQLCMacro = regexp.MustCompile(`sqlc\.(arg|narg|slice|embed)|@[[:alpha:]_][[:alnum:]_]*`)
	databaseRandomness  = regexp.MustCompile(`randomblob[[:space:]]*\(`)
	generatedQueryConst = regexp.MustCompile("^const [[:alnum:]_]+ = `")
	numberedParameter   = regexp.MustCompile(`\?[[:digit:]]+`)
)

// Three sqlc hazards SQLite does not report until a query runs:
//   - a parameter macro sqlc left unresolved in the generated SQL;
//   - randomblob() in an application query, where UUIDs and secrets must be
//     generated in Go and passed in;
//   - a sqlc.slice expansion in a query that also has numbered parameters:
//     the expansion shifts every placeholder after it. Use a JSON1 list
//     parameter instead.
func TestGeneratedQueriesAvoidSQLCHazards(t *testing.T) {
	generated, err := filepath.Glob("repo/*.sql.go")
	if err != nil || len(generated) == 0 {
		t.Fatalf("generated queries: %v (found %d)", err, len(generated))
	}
	for _, file := range generated {
		source := readSource(t, file)
		for number, line := range strings.Split(source, "\n") {
			if unresolvedSQLCMacro.MatchString(line) {
				t.Errorf("%s:%d: unresolved sqlc parameter macro: %s", file, number+1, strings.TrimSpace(line))
			}
		}
		for _, start := range mixedSliceQueryStarts(source) {
			t.Errorf("%s:%d: sqlc slice query mixed with numbered parameters; use a JSON1 list parameter", file, start)
		}
	}
	queries, err := filepath.Glob("repo/queries/*.sql")
	if err != nil || len(queries) == 0 {
		t.Fatalf("query files: %v (found %d)", err, len(queries))
	}
	for _, file := range queries {
		for number, line := range strings.Split(readSource(t, file), "\n") {
			if databaseRandomness.MatchString(line) {
				t.Errorf("%s:%d: database-side randomness; generate the value in Go: %s", file, number+1, strings.TrimSpace(line))
			}
		}
	}
}

func TestMixedSliceQueryDetection(t *testing.T) {
	mixed := "const listByIDs = `-- name: ListByIDs :many\nSELECT * FROM assets WHERE owner_id = ?1 AND asset_id IN (/*SLICE:ids*/?)\n`\n"
	clean := "const listByIDs = `-- name: ListByIDs :many\nSELECT * FROM assets WHERE asset_id IN (/*SLICE:ids*/?)\n`\n" +
		"const getAsset = `-- name: GetAsset :one\nSELECT * FROM assets WHERE asset_id = ?1\n`\n"
	if got := mixedSliceQueryStarts(mixed); len(got) != 1 || got[0] != 1 {
		t.Fatalf("mixed query starts = %v, want [1]", got)
	}
	if got := mixedSliceQueryStarts(clean); len(got) != 0 {
		t.Fatalf("clean queries flagged at %v", got)
	}
}

// mixedSliceQueryStarts returns the first line of each generated query
// constant that has both a slice expansion and a numbered parameter.
func mixedSliceQueryStarts(source string) []int {
	var starts []int
	inQuery, hasSlice, hasNumbered, start := false, false, false, 0
	for number, line := range strings.Split(source, "\n") {
		if generatedQueryConst.MatchString(line) {
			inQuery, hasSlice, hasNumbered, start = true, false, false, number+1
		}
		if inQuery && strings.Contains(line, "/*SLICE:") && strings.Contains(line, "*/") {
			hasSlice = true
		}
		if inQuery && numberedParameter.MatchString(line) {
			hasNumbered = true
		}
		if inQuery && line == "`" {
			if hasSlice && hasNumbered {
				starts = append(starts, start)
			}
			inQuery = false
		}
	}
	return starts
}

func readSource(t *testing.T, path string) string {
	t.Helper()
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(source)
}
