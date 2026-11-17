package app

import (
	"net/http"
	"testing"
	"time"
)

// #223: "A backup taken before a Delete is restored; the Trash view is
// rebuilt from the sidecars." The restored catalog lists the deleted file as
// present while its repository trash holds the file. After the in-process
// runtime restart the Trash maintenance adopts the sidecar and the next scan
// drops the stale row, so the Trash lists the file and Restore returns it.
func TestLifecycleBackupTakenBeforeADeleteRestoresIntoTheTrash(t *testing.T) {
	server := startBlackboxServer(t)
	const filename = "backup-before-delete.jpg"
	server.upload(server.primary, filename, blackboxJPEG(t, 150, "backup-before-delete"))
	var listed []blackboxAsset
	if !waitFor(time.Minute, func() bool { listed = server.listByFilename(filename); return len(listed) == 1 }) {
		t.Fatalf("uploaded file never appeared in browse: %+v", listed)
	}
	assetID := listed[0].AssetID
	server.waitProcessingSettled()
	server.setRatingAndLike(assetID, 3, true)

	backupName := server.requestBackup()

	// Then the Delete.
	server.mustJSON(http.MethodDelete, "/api/v1/assets/"+assetID, nil, nil)
	if names := server.listTrashed("", server.primary.ID); len(names) != 1 || names[0] != filename {
		t.Fatalf("Trash after the delete lists %v", names)
	}
	if trashed := trashedFilesOf(server.primary); len(trashed) != 1 {
		t.Fatalf("repository trash holds %v, want one file", trashed)
	}

	// Restore the backup through the in-process runtime restart.
	server.restoreBackup(backupName)

	// The restored catalog knew the file as present; the Trash rebuild and
	// the scan agree it is in the Trash, with the metadata the backup held.
	if !waitFor(2*time.Minute, func() bool {
		server.scan(server.primary, 2*time.Minute)
		names := server.listTrashed("", server.primary.ID)
		return len(names) == 1 && names[0] == filename
	}) {
		t.Fatalf("Trash after the restored backup lists %v, want [%s]", server.listTrashed("", server.primary.ID), filename)
	}
	if got := server.listByFilename(filename); len(got) != 0 {
		t.Fatalf("browse after the restored backup still lists %+v", got)
	}
	detail := server.asset(assetID)
	if detail.LifecycleState != "trashed" || detail.Rating == nil || *detail.Rating != 3 {
		t.Fatalf("Asset after the restored backup = lifecycle %q rating %v, want trashed with rating 3", detail.LifecycleState, detail.Rating)
	}
	if count := server.activeAssetCount(server.primary); count != 0 {
		t.Fatalf("storage count after the restored backup = %d, want 0", count)
	}

	// Restore from the Trash brings the file and the Asset back.
	server.mustJSON(http.MethodPost, "/api/v1/assets/"+assetID+"/restore", nil, nil)
	if path := originalPath(t, server.primary, filename); path == "" {
		t.Fatal("restored file is not back in the repository")
	}
	if got := server.listByFilename(filename); len(got) != 1 || got[0].AssetID != assetID {
		t.Fatalf("browse after Restore lists %+v, want the same Asset", got)
	}
	if names := server.listTrashed("", server.primary.ID); len(names) != 0 {
		t.Fatalf("Trash after Restore lists %v", names)
	}
	if trashed := trashedFilesOf(server.primary); len(trashed) != 0 {
		t.Fatalf("repository trash still holds %v", trashed)
	}
}

// #223 / Phase 4b: "The Trash view can be rebuilt from the sidecars." Here the
// restored backup predates the upload, so the catalog never knew the file:
// the rebuild adopts the sidecar into a new trashed Asset, which the Trash
// view lists and Restore returns to the library.
func TestLifecycleTrashIsRebuiltIntoACatalogThatNeverKnewTheFile(t *testing.T) {
	server := startBlackboxServer(t)
	backupName := server.requestBackup()
	const filename = "unknown-to-the-backup.jpg"
	server.upload(server.primary, filename, blackboxJPEG(t, 170, "unknown-to-the-backup"))
	var listed []blackboxAsset
	if !waitFor(time.Minute, func() bool { listed = server.listByFilename(filename); return len(listed) == 1 }) {
		t.Fatalf("uploaded file never appeared in browse: %+v", listed)
	}
	server.waitProcessingSettled()
	server.mustJSON(http.MethodDelete, "/api/v1/assets/"+listed[0].AssetID, nil, nil)
	if trashed := trashedFilesOf(server.primary); len(trashed) != 1 {
		t.Fatalf("repository trash holds %v, want one file", trashed)
	}

	server.restoreBackup(backupName)

	var adopted []blackboxAsset
	if !waitFor(2*time.Minute, func() bool {
		adopted = server.listTrashedAssets(server.primary.ID)
		return len(adopted) == 1 && adopted[0].OriginalFilename == filename
	}) {
		t.Fatalf("Trash view after the rebuild lists %+v, want %s", adopted, filename)
	}
	if status := server.assetStatus(listed[0].AssetID); status != http.StatusNotFound {
		t.Fatalf("the Asset the backup never knew answers %d, want 404 (the rebuild made a new one)", status)
	}
	if got := server.listByFilename(filename); len(got) != 0 {
		t.Fatalf("browse lists the trashed file: %+v", got)
	}

	server.mustJSON(http.MethodPost, "/api/v1/assets/"+adopted[0].AssetID+"/restore", nil, nil)
	if path := originalPath(t, server.primary, filename); path == "" {
		t.Fatal("restored file is not back in the repository")
	}
	if !waitFor(time.Minute, func() bool {
		got := server.listByFilename(filename)
		return len(got) == 1 && got[0].AssetID == adopted[0].AssetID
	}) {
		t.Fatalf("browse after Restore lists %+v, want the adopted Asset", server.listByFilename(filename))
	}
	if names := server.listTrashed("", server.primary.ID); len(names) != 0 {
		t.Fatalf("Trash after Restore lists %v", names)
	}
	// The restored file is processed like any other: it gets a thumbnail.
	if !waitFor(2*time.Minute, func() bool { return server.thumbnail(adopted[0].AssetID, "small") != nil }) {
		t.Fatal("the restored Asset never got a thumbnail")
	}
}
