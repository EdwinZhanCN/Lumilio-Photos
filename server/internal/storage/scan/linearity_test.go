package scan

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

// TestScanCostIsLinearAndWriterHoldIsBounded is the #222 walk-linearity
// check at test scale: the per-file cost of a first full scan over 40k files
// stays within 1.5x of the cost over 10k files, and every writer transaction
// of the walk and the hash pass stays within the 25 ms budget at p99. The
// 100k profile runs under remote qualification.
func TestScanCostIsLinearAndWriterHoldIsBounded(t *testing.T) {
	small := timedFirstScan(t, 10_000, true)
	large := timedFirstScan(t, 40_000, false)
	smallPerFile := small / 10_000
	largePerFile := large / 40_000
	t.Logf("first full scan: 10k files %s (%s/file), 40k files %s (%s/file)", small, smallPerFile, large, largePerFile)
	if largePerFile > smallPerFile*3/2 {
		t.Fatalf("per-file scan cost grew %.2fx from 10k to 40k files, want at most 1.5x",
			float64(largePerFile)/float64(smallPerFile))
	}
}

// timedFirstScan builds a tree of files in directories of 250, runs a first
// full scan, and returns its wall time. With hash set it also drains the hash
// backlog; both passes must respect the writer budget.
func timedFirstScan(t *testing.T, files int, hash bool) time.Duration {
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
	if p99 := f.writer.p99(); p99 > 25*time.Millisecond {
		t.Fatalf("%d-file scan writer hold p99 = %s, want at most 25ms", files, p99)
	}
	return elapsed
}
