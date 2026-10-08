package scan

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"
)

// TestScanCostIsLinearAndWriterHoldIsBounded catches gross scan regressions.
// LUMILIO_PERF=1 checks the precise 1.5x growth and 25ms writer p99 targets
// in the non-required perf lane. N100 is optional reference hardware.
func TestScanCostIsLinearAndWriterHoldIsBounded(t *testing.T) {
	small, smallP99 := timedFirstScan(t, 10_000, true)
	large, largeP99 := timedFirstScan(t, 40_000, false)
	costs := []float64{float64(small) / 10_000, float64(large) / 40_000}
	ratio := costs[1] / costs[0]
	t.Logf("first full scan: 10k %s (%.0f ns/file, writer p99 %s), 40k %s (%.0f ns/file, writer p99 %s), growth %.2fx", small, costs[0], smallP99, large, costs[1], largeP99, ratio)
	precise := os.Getenv("LUMILIO_PERF") == "1"
	if out := os.Getenv("LUMILIO_PERF_OUT"); precise && out != "" {
		result := struct {
			Test      string    `json:"test"`
			Sizes     []int     `json:"sizes"`
			Durations []int64   `json:"durations_ns"`
			Costs     []float64 `json:"per_file_ns"`
			Ratio     float64   `json:"ratio"`
			P99       []int64   `json:"writer_p99_ns"`
			GOOS      string    `json:"goos"`
			GOARCH    string    `json:"goarch"`
		}{t.Name(), []int{10_000, 40_000}, []int64{int64(small), int64(large)}, costs, ratio, []int64{int64(smallP99), int64(largeP99)}, runtime.GOOS, runtime.GOARCH}
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(out, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(out, t.Name()+".json"), append(data, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if ratio >= 3 {
		t.Errorf("per-file scan cost grew %.2fx, want <3x", ratio)
	}
	for _, p99 := range []time.Duration{smallP99, largeP99} {
		if p99 >= 100*time.Millisecond {
			t.Errorf("writer hold p99 = %s, want <100ms", p99)
		}
		if precise && p99 > 25*time.Millisecond {
			t.Errorf("writer hold p99 = %s, precise target <=25ms", p99)
		}
	}
	if precise && ratio > 1.5 {
		t.Errorf("per-file scan cost grew %.2fx, precise target <=1.5x", ratio)
	}
}

// timedFirstScan builds a tree of files in directories of 250, runs a first
// full scan, and returns its wall time. With hash set it also drains the hash
// backlog; the returned p99 covers both passes.
func timedFirstScan(t *testing.T, files int, hash bool) (time.Duration, time.Duration) {
	t.Helper()
	f := newFixture(t, 0)
	root := f.repos[f.primary.RepoID]
	settled := time.Now().Add(-time.Hour)
	for index := 0; index < files; index++ {
		directory := filepath.Join(root, "d"+strconv.Itoa(index/250))
		if index%250 == 0 {
			if err := os.Mkdir(directory, 0o755); err != nil {
				t.Fatal(err)
			}
		}
		target := filepath.Join(directory, "f"+strconv.Itoa(index)+".jpg")
		if err := os.WriteFile(target, []byte("linearity-"+strconv.Itoa(index)), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(target, settled, settled); err != nil {
			t.Fatal(err)
		}
	}
	f.writer.reset()
	started := time.Now()
	scan := f.scan(f.primary)
	elapsed := time.Since(started)
	if scan.Status != StatusCompleted || scan.NewEntries != int64(files+files/250) {
		t.Fatalf("%d-file scan = %+v", files, scan)
	}
	if hash {
		if bound := f.hash(f.primary); bound != files {
			t.Fatalf("hash bound %d of %d files", bound, files)
		}
	}
	return elapsed, f.writer.p99()
}
