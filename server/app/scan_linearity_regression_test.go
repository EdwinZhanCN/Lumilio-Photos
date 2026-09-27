package app

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// #222: a full scan's cost is linear in the tree. Per-entry wall time for a
// 40k-file tree stays within 1.5x of a 10k-file tree, and the whole check
// stays well inside the server test budget.
func TestLifecycleRegressionScanCostIsLinear(t *testing.T) {
	small := blackboxTimedFullScan(t, 10_000, time.Minute)
	large := blackboxTimedFullScan(t, 40_000, 90*time.Second)
	smallPerEntry := small / 10_000
	largePerEntry := large / 40_000
	t.Logf("full scan: 10k files %s (%s/file), 40k files %s (%s/file)", small, smallPerEntry, large, largePerEntry)
	if largePerEntry > smallPerEntry*3/2 {
		t.Fatalf("per-file scan cost grew %.2fx from 10k to 40k files, want at most 1.5x",
			float64(largePerEntry)/float64(smallPerEntry))
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
