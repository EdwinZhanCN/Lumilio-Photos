package app

import (
	"net/http"
	"os"
	"testing"
	"time"

	"server/internal/api/dto"
)

// #223: "External deletion hides the Asset from browse, search, albums, and
// counts; the file coming back restores it; Remove missing items purges it."
// Browse is covered by TestLifecycleRegressionMissingFileLeavesLibraryBrowse;
// this test observes the other views the Web client reads.
func TestLifecycleExternalDeletionIsHiddenEverywhereAndReversible(t *testing.T) {
	server := startBlackboxServer(t)
	const filename = "externally-deleted.jpg"
	contents := blackboxJPEG(t, 90, "external-deletion")
	target := server.writeFile(server.primary, "photos/"+filename, contents)
	server.scan(server.primary, 2*time.Minute)
	var listed []blackboxAsset
	if !waitFor(time.Minute, func() bool { listed = server.listByFilename(filename); return len(listed) == 1 }) {
		t.Fatalf("scanned file never appeared in library browse: %+v", listed)
	}
	server.waitProcessingSettled()
	assetID := listed[0].AssetID
	album := server.createAlbum("External deletion")
	server.addToAlbum(album, assetID)
	server.setRatingAndLike(assetID, 4, true)
	if names := server.searchByFilename(filename); len(names) != 1 {
		t.Fatalf("search before the deletion = %v, want the file", names)
	}
	if count := server.activeAssetCount(server.primary); count != 1 {
		t.Fatalf("storage count before the deletion = %d, want 1", count)
	}

	// Deleted behind the Server's back.
	if err := os.Remove(target); err != nil {
		t.Fatal(err)
	}
	if run := server.scan(server.primary, 2*time.Minute); run["status"] != "completed" || run["missing"] != float64(1) {
		t.Fatalf("rescan after the deletion = %v, want completed with missing = 1", run)
	}
	if names := server.searchByFilename(filename); len(names) != 0 {
		t.Fatalf("search still finds the deleted file: %v", names)
	}
	if assets := server.albumAssets(album); len(assets) != 0 {
		t.Fatalf("album still lists the deleted file: %+v", assets)
	}
	if count := server.activeAssetCount(server.primary); count != 0 {
		t.Fatalf("storage count after the deletion = %d, want 0", count)
	}
	if detail := server.asset(assetID); detail.LifecycleState != "missing" || detail.Rating == nil || *detail.Rating != 4 {
		t.Fatalf("missing Asset detail = lifecycle %q rating %v, want missing with the user's rating 4 kept", detail.LifecycleState, detail.Rating)
	}

	// The file comes back: the same Asset is active again, with its album and
	// rating, and no second Asset is created.
	server.writeFile(server.primary, "photos/"+filename, contents)
	if run := server.scan(server.primary, 2*time.Minute); run["status"] != "completed" {
		t.Fatalf("rescan after the return = %v, want completed", run)
	}
	if !waitFor(time.Minute, func() bool { listed = server.listByFilename(filename); return len(listed) == 1 }) {
		t.Fatalf("returned file is not back in library browse: %+v", listed)
	}
	if listed[0].AssetID != assetID {
		t.Fatalf("returned file is Asset %s, want the original %s", listed[0].AssetID, assetID)
	}
	if assets := server.albumAssets(album); len(assets) != 1 || assets[0].AssetID != assetID {
		t.Fatalf("album after the return = %+v, want the original Asset", assets)
	}
	if names := server.searchByFilename(filename); len(names) != 1 {
		t.Fatalf("search after the return = %v, want the file", names)
	}
	if count := server.activeAssetCount(server.primary); count != 1 {
		t.Fatalf("storage count after the return = %d, want 1", count)
	}

	// Deleted again and removed from the catalog by "Remove missing items":
	// the Asset is gone, the album is empty, and nothing else was touched.
	server.waitProcessingSettled()
	if err := os.Remove(target); err != nil {
		t.Fatal(err)
	}
	if run := server.scan(server.primary, 2*time.Minute); run["status"] != "completed" {
		t.Fatalf("second rescan after deletion = %v, want completed", run)
	}
	var purged dto.AssetLifecycleResultDTO
	server.mustJSON(http.MethodPost, "/api/v1/assets/remove-missing",
		dto.AssetPurgeRequestDTO{RepositoryID: &server.primary.ID, Confirm: true}, &purged)
	if purged.Assets != 1 || purged.Entries != 1 {
		t.Fatalf("remove missing = %+v, want one Asset and one entry", purged)
	}
	if status := server.assetStatus(assetID); status != http.StatusNotFound {
		t.Fatalf("purged Asset detail = %d, want 404", status)
	}
	if assets := server.albumAssets(album); len(assets) != 0 {
		t.Fatalf("album after the purge = %+v, want empty", assets)
	}
	if names := server.listTrashed("", server.primary.ID); len(names) != 0 {
		t.Fatalf("Trash after the purge lists %v, want nothing (a purge is not a Delete)", names)
	}
}
