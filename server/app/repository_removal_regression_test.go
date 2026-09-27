package app

import (
	"net/http"
	"os"
	"testing"
	"time"
)

// #223 defect 2: removing a repository must purge Assets whose only
// occurrence there is already missing, not only those still present.
func TestLifecycleRegressionRepositoryRemovalPurgesMissingOnlyAssets(t *testing.T) {
	server := startBlackboxServer(t)
	repository := server.createRepository("Removal Regression", "removal-regression")
	const filename = "removal-regression.jpg"
	target := server.writeFile(repository, "photos/"+filename, blackboxJPEG(t, 30, "removal-regression"))
	server.scan(repository, 2*time.Minute)
	var listed []blackboxAsset
	if !waitFor(time.Minute, func() bool { listed = server.listByFilename(filename); return len(listed) == 1 }) {
		t.Fatalf("scanned file never appeared in library browse: %+v", listed)
	}
	assetID := listed[0].AssetID

	if err := os.Remove(target); err != nil {
		t.Fatal(err)
	}
	if run := server.scan(repository, 2*time.Minute); run["status"] != "completed" {
		t.Fatalf("rescan after deletion = %v, want completed", run)
	}
	// Removal must see the Asset as missing-only, not merely not yet swept.
	server.waitNoActiveAssets(repository)
	server.removeRepository(repository)
	if !waitFor(15*time.Second, func() bool { return server.assetStatus(assetID) == http.StatusNotFound }) {
		t.Fatalf("Asset %s whose only occurrence was missing survived repository removal: GET status %d",
			assetID, server.assetStatus(assetID))
	}
}
