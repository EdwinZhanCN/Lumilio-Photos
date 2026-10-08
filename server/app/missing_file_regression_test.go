package app

import (
	"os"
	"testing"
	"time"
)

// #223 defect 1: a file deleted outside Lumilio must leave library browse
// once a completed scan has seen it gone.
func TestLifecycleRegressionMissingFileLeavesLibraryBrowse(t *testing.T) {
	server := startBlackboxServer(t)
	const filename = "missing-regression.jpg"
	target := server.writeFile(server.primary, "photos/"+filename, blackboxJPEG(t, 10, "missing-regression"))
	server.scan(server.primary, 2*time.Minute)
	if !waitFor(time.Minute, func() bool { return len(server.listByFilename(filename)) == 1 }) {
		t.Fatalf("scanned file never appeared in library browse: %+v", server.listByFilename(filename))
	}

	if err := os.Remove(target); err != nil {
		t.Fatal(err)
	}
	if run := server.scan(server.primary, 2*time.Minute); run["status"] != "completed" {
		t.Fatalf("rescan after deletion = %v, want completed", run)
	}
	if !waitFor(15*time.Second, func() bool { return len(server.listByFilename(filename)) == 0 }) {
		t.Fatalf("a file deleted outside Lumilio is still listed in library browse after a completed scan: %+v",
			server.listByFilename(filename))
	}
}
