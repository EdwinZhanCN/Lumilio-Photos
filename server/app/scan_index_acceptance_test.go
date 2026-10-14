package app

import (
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/syncthing/notify"

	hashutil "server/internal/utils/hash"
)

// #222: "An upload followed by a scan does not rehash." The upload commit
// binds the entry with the hash it computed from the handle it wrote, so the
// next full scan neither hashes nor rewrites it. Hashing is counted by the
// hash library itself, outside the scanner; the scan's own counters must
// agree.
func TestScanIndexUploadThenScanDoesNotRehash(t *testing.T) {
	server := startBlackboxServer(t)
	const filename = "uploaded-once.jpg"
	server.upload(server.primary, filename, blackboxJPEG(t, 120, "upload-then-scan"))
	if !waitFor(time.Minute, func() bool { return len(server.listByFilename(filename)) == 1 }) {
		t.Fatalf("uploaded file never appeared in library browse: %+v", server.listByFilename(filename))
	}
	server.waitProcessingSettled()

	before := hashutil.Computations()
	run := server.scan(server.primary, 2*time.Minute)
	if run["status"] != "completed" {
		t.Fatalf("scan after upload = %v, want completed", run)
	}
	for _, counter := range []string{"new_entries", "changed", "hashed", "bytes_hashed", "deferred", "missing"} {
		if value, _ := run[counter].(float64); value != 0 {
			t.Fatalf("scan after upload reported %s = %v, want 0: %v", counter, value, run)
		}
	}
	if hashes := hashutil.Computations() - before; hashes != 0 {
		t.Fatalf("scan after upload computed %d content hashes, want 0", hashes)
	}
	if assets := server.listByFilename(filename); len(assets) != 1 {
		t.Fatalf("upload is no longer exactly one Asset after the scan: %+v", assets)
	}
}

// silentChangeWatch is a watcher that starts and never reports an event: a
// disabled or non-functional native watcher. unavailableChangeWatch is one
// that cannot start at all.
type silentChangeWatch struct{}

func (silentChangeWatch) Watch(string, chan<- notify.EventInfo) error { return nil }
func (silentChangeWatch) Stop(chan<- notify.EventInfo)                {}

type unavailableChangeWatch struct{}

func (unavailableChangeWatch) Watch(string, chan<- notify.EventInfo) error {
	return errors.New("no native watcher on this host")
}
func (unavailableChangeWatch) Stop(chan<- notify.EventInfo) {}

// #222: "A disabled or unavailable watcher still converges on the periodic
// scan." A file written behind the Server's back must reach the library
// with no watcher hint, through the periodic full scan alone (silent
// watcher) or through the full scans an unavailable watcher schedules.
func TestScanIndexConvergesWithoutWatcherHints(t *testing.T) {
	cases := []struct {
		name            string
		options         blackboxOptions
		expectedTrigger string
	}{
		{"disabled watcher converges on the periodic scan", blackboxOptions{scanIntervalSeconds: 1, changeWatch: silentChangeWatch{}}, "periodic"},
		{"unavailable watcher converges on the full scans it schedules", blackboxOptions{scanIntervalSeconds: 3600, changeWatch: unavailableChangeWatch{}}, "watcher"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := startBlackboxServerWith(t, tc.options)
			filename := fmt.Sprintf("behind-the-back-%s.jpg", tc.expectedTrigger)
			server.writeFile(server.primary, "dropped/"+filename, blackboxJPEG(t, 60, filename))
			if !waitFor(90*time.Second, func() bool { return len(server.listByFilename(filename)) == 1 }) {
				t.Fatalf("file written without a watcher hint never appeared in library browse; scans: %+v", server.scans(server.primary))
			}
			indexedBy := ""
			for _, run := range server.scans(server.primary) {
				if run.Status == "completed" && run.NewEntries > 0 {
					indexedBy = run.Mode
				}
			}
			if indexedBy != tc.expectedTrigger {
				t.Fatalf("the file was indexed by a %q scan, want %q; scans: %+v", indexedBy, tc.expectedTrigger, server.scans(server.primary))
			}
		})
	}
}

type blackboxScanRun struct {
	Mode       string `json:"mode"`
	ScopePath  string `json:"scope_path"`
	Status     string `json:"status"`
	NewEntries int64  `json:"new_entries"`
	Hashed     int64  `json:"hashed"`
}

// scans lists a repository's scans, oldest first.
func (s *blackboxServer) scans(repository blackboxRepository) []blackboxScanRun {
	s.t.Helper()
	var response struct {
		Scans []blackboxScanRun `json:"scans"`
	}
	s.mustJSON(http.MethodGet, "/api/v1/storage/repositories/"+repository.ID+"/verifications?limit=100", nil, &response)
	runs := response.Scans
	for left, right := 0, len(runs)-1; left < right; left, right = left+1, right-1 {
		runs[left], runs[right] = runs[right], runs[left]
	}
	return runs
}
