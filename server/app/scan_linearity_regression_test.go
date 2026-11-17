package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// #222: scans must complete without gross per-file cost growth. Generous
// deadlines tolerate shared runners; LUMILIO_PERF=1 checks the precise 1.5x
// target in the non-required perf lane. N100 is optional reference hardware.
func TestLifecycleRegressionScanCostIsLinear(t *testing.T) {
	// Each size gets its own server in a subtest, so the first server has
	// shut down, with its import processing, before the second is measured.
	var small, large time.Duration
	t.Run("10k", func(t *testing.T) { small = blackboxTimedFullScan(t, 10_000, 3*time.Minute) })
	t.Run("40k", func(t *testing.T) { large = blackboxTimedFullScan(t, 40_000, 6*time.Minute) })
	if t.Failed() {
		return
	}
	costs := []float64{float64(small) / 10_000, float64(large) / 40_000}
	ratio := costs[1] / costs[0]
	t.Logf("full scan: 10k %s (%.0f ns/file), 40k %s (%.0f ns/file), growth %.2fx", small, costs[0], large, costs[1], ratio)
	precise := os.Getenv("LUMILIO_PERF") == "1"
	if out := os.Getenv("LUMILIO_PERF_OUT"); precise && out != "" {
		result := struct {
			Test      string    `json:"test"`
			Sizes     []int     `json:"sizes"`
			Durations []int64   `json:"durations_ns"`
			Costs     []float64 `json:"per_file_ns"`
			Ratio     float64   `json:"ratio"`
			GOOS      string    `json:"goos"`
			GOARCH    string    `json:"goarch"`
		}{t.Name(), []int{10_000, 40_000}, []int64{int64(small), int64(large)}, costs, ratio, runtime.GOOS, runtime.GOARCH}
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
	if precise && ratio > 1.5 {
		t.Errorf("per-file scan cost grew %.2fx, precise target <=1.5x", ratio)
	}
}

// blackboxTimedFullScan moves a generated tree of settled JPEGs into a fresh
// primary repository in one rename and returns the wall time until the scan
// that covers it is terminal.
func blackboxTimedFullScan(t *testing.T, files int, budget time.Duration) time.Duration {
	t.Helper()
	server := startBlackboxServer(t)
	server.scan(server.primary, time.Minute)
	staging := filepath.Join(filepath.Dir(server.primary.Path), fmt.Sprintf("generated-%d", files))
	base := blackboxJPEG(t, 90, "linearity")
	settled := time.Now().Add(-time.Hour)
	const perDirectory = 250
	for index := 0; index < files; index++ {
		directory := filepath.Join(staging, fmt.Sprintf("d%04d", index/perDirectory))
		if index%perDirectory == 0 {
			if err := os.MkdirAll(directory, 0o755); err != nil {
				t.Fatal(err)
			}
		}
		target := filepath.Join(directory, fmt.Sprintf("f%06d.jpg", index))
		if err := os.WriteFile(target, blackboxWithComment(base, fmt.Sprintf("linearity-%d", index)), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(target, settled, settled); err != nil {
			t.Fatal(err)
		}
	}
	started := time.Now()
	if err := os.Rename(staging, filepath.Join(server.primary.Path, "generated")); err != nil {
		t.Fatal(err)
	}
	run := server.scanOnce(server.primary, started.Add(budget))
	elapsed := time.Since(started)
	if run["status"] != "completed" {
		t.Fatalf("%d-file scan = %v, want completed", files, run)
	}
	return elapsed
}
