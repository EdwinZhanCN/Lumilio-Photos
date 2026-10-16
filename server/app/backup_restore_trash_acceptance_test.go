package app

import (
	"net/http"
	"testing"
	"time"

	"server/internal/api/dto"
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

	// A backup now. The routine backup the Server took at startup predates
	// the upload, so the test waits for the one it requested.
	listBackups := func() map[string]bool {
		var backups dto.BackupListDTO
		server.mustJSON(http.MethodGet, "/api/v1/settings/backups", nil, &backups)
		names := map[string]bool{}
		for _, backup := range backups.Backups {
			if !backup.RestorePoint {
				names[backup.Name] = true
			}
		}
		return names
	}
	earlier := listBackups()
	if status, payload := server.do(http.MethodPost, "/api/v1/settings/backups", nil, ""); status != http.StatusAccepted {
		t.Fatalf("create backup: %d %s", status, payload)
	}
	backupName := ""
	if !waitFor(2*time.Minute, func() bool {
		for name := range listBackups() {
			if !earlier[name] {
				backupName = name
			}
		}
		return backupName != ""
	}) {
		t.Fatal("the requested backup never appeared in the list")
	}

	// Then the Delete.
	server.mustJSON(http.MethodDelete, "/api/v1/assets/"+assetID, nil, nil)
	if names := server.listTrashed("", server.primary.ID); len(names) != 1 || names[0] != filename {
		t.Fatalf("Trash after the delete lists %v", names)
	}
	if trashed := trashedFilesOf(server.primary); len(trashed) != 1 {
		t.Fatalf("repository trash holds %v, want one file", trashed)
	}

	// Restore the backup: accepted, then the runtime restarts in-process.
	var operation dto.RestoreOperationDTO
	if _, err := server.try(http.MethodPost, "/api/v1/settings/backups/"+backupName+"/restore", nil, &operation); err != nil {
		t.Fatal(err)
	}
	if operation.ID == "" {
		t.Fatalf("restore accepted without an operation: %+v", operation)
	}
	if !waitFor(3*time.Minute, func() bool {
		var latest dto.RestoreOperationDTO
		if _, err := server.try(http.MethodGet, "/api/v1/settings/backup-restores/latest", nil, &latest); err != nil {
			return false
		}
		switch latest.Status {
		case "completed":
			return latest.ID == operation.ID
		case "failed", "rolled_back":
			t.Fatalf("restore operation ended %s: %+v", latest.Status, latest)
		}
		return false
	}) {
		t.Fatal("restore operation did not complete")
	}
	server.waitIdle(server.primary)

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
