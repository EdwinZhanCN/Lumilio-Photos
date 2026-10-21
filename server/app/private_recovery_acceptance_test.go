package app

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"server/internal/api/dto"
	"server/internal/storage/testfixture"
)

// Storage P4 / R4, through the public API: a Repository tree that carries a
// lost catalog's private state (the P0 leftover-primary fixture) is opened
// as a regular Repository of the Default Storage Location. Its inherited
// Trash is listed and held, its Studio record is reassociated to the new
// Asset once the scan proved the content, its abandoned staging and unknown
// bytes are preserved and exportable, and the user's decisions (delete an
// item, accept retention) are the only way that state changes.
func TestPrivateStateOfAnOpenedRepositoryIsRecoverable(t *testing.T) {
	server := startBlackboxServer(t)
	_, leftover := testfixture.NewLeftoverPrimary(t)
	inherited := filepath.Join(filepath.Dir(server.primary.Path), "inherited")
	if err := os.CopyFS(inherited, os.DirFS(leftover.Path)); err != nil {
		t.Fatal(err)
	}
	stagingBytes, err := os.ReadFile(filepath.Join(leftover.Path, ".lumilio", "staging", "incoming", "abandoned.part"))
	if err != nil {
		t.Fatal(err)
	}

	var opened struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Path string `json:"path"`
	}
	server.mustJSON(http.MethodPost, "/api/v1/storage/candidates/open", map[string]any{
		"directory_name": "inherited", "risk_confirmation": true,
	}, &opened)
	repository := blackboxRepository{ID: opened.ID, Name: opened.Name, Path: opened.Path}
	recoveryPath := "/api/v1/storage/repositories/" + url.PathEscape(repository.ID) + "/recovery"

	// The initial scan proves the original's content, then private recovery
	// decides the Studio record; the Trash is adopted and held along the way.
	var inventory dto.RepositoryRecoveryDTO
	var original []blackboxAsset
	if !waitFor(2*time.Minute, func() bool {
		server.mustJSON(http.MethodGet, recoveryPath, nil, &inventory)
		original = server.listByFilename("original.png")
		for _, item := range inventory.Items {
			if item.Class == "studio" && item.Path == testfixture.StudioPath && item.Disposition == "reassociated" {
				return inventory.Trash.HeldCount == 1 && len(original) == 1
			}
		}
		return false
	}) {
		t.Fatalf("recovery inventory never settled: %+v (original: %+v)", inventory, original)
	}
	if len(inventory.Receipts) != 1 || inventory.Receipts[0].Event != "open" || inventory.Receipts[0].TrashFiles != 2 {
		t.Fatalf("receipts = %+v", inventory.Receipts)
	}
	items := map[string]dto.RepositoryRecoveryItemDTO{}
	for _, item := range inventory.Items {
		items[item.Path] = item
	}
	studio := items[testfixture.StudioPath]
	if studio.ResolvedAssetID != original[0].AssetID || !studio.Present || len(studio.Actions) != 0 {
		t.Fatalf("studio item = %+v, want reassociated to %s", studio, original[0].AssetID)
	}
	staging := items[".lumilio/staging/incoming/abandoned.part"]
	if staging.Class != "staging" || staging.Disposition != "moved" || !staging.Present || strings.Join(staging.Actions, ",") != "export,delete" {
		t.Fatalf("staging item = %+v", staging)
	}
	if unknown := items[".lumilio/unknown-child/opaque.bin"]; unknown.Class != "unknown" || !unknown.Present {
		t.Fatalf("unknown item = %+v", unknown)
	}
	for _, path := range []string{".lumilio/trash/info", ".lumilio/logs", ".lumilio/assets"} {
		for listed := range items {
			if strings.HasPrefix(listed, path) {
				t.Fatalf("%s is listed as an item; its class is preserved as a whole", listed)
			}
		}
	}

	// The Studio record is served for the new Asset through the ordinary
	// Studio endpoint, with the edit the user made before the catalog was lost.
	var sidecar dto.AssetSidecarResponseDTO
	server.mustJSON(http.MethodGet, "/api/v1/assets/"+original[0].AssetID+"/sidecar", nil, &sidecar)
	if !sidecar.Exists || sidecar.Sidecar.AssetID != original[0].AssetID || sidecar.Sidecar.Adjustments.Exposure != 0.5 {
		t.Fatalf("reassociated sidecar = exists %t, asset %s, exposure %v", sidecar.Exists, sidecar.Sidecar.AssetID, sidecar.Sidecar.Adjustments.Exposure)
	}

	// The inherited Trash lists the deleted file, held; the Storage view
	// carries the counts the page shows.
	trashed := server.listTrashedAssets(repository.ID)
	if len(trashed) != 1 || trashed[0].OriginalFilename != "deleted.png" {
		t.Fatalf("Trash lists %+v, want deleted.png", trashed)
	}
	var holds dto.AssetTrashHoldsDTO
	server.mustJSON(http.MethodGet, "/api/v1/assets/trash-holds?repository_id="+url.QueryEscape(repository.ID), nil, &holds)
	if holds.Count != 1 || holds.AssetIDs[0] != trashed[0].AssetID {
		t.Fatalf("trash holds = %+v, want %s", holds, trashed[0].AssetID)
	}
	var view struct {
		Repositories []struct {
			ID                string `json:"id"`
			TrashHeldCount    int64  `json:"trash_held_count"`
			RecoveryItemCount int64  `json:"recovery_item_count"`
		} `json:"repositories"`
	}
	server.mustJSON(http.MethodGet, "/api/v1/storage/view", nil, &view)
	var heldInView, pendingInView int64 = -1, -1
	for _, row := range view.Repositories {
		if row.ID == repository.ID {
			heldInView, pendingInView = row.TrashHeldCount, row.RecoveryItemCount
		}
	}
	if heldInView != 1 || pendingInView < 3 {
		t.Fatalf("storage view counts = held %d, pending %d", heldInView, pendingInView)
	}

	// Export streams the preserved staging bytes unchanged; delete is the
	// explicit decision, idempotent, and recorded in the inventory.
	status, exported := server.do(http.MethodGet, recoveryPath+"/items/"+staging.ID+"/content", nil, "")
	if status != http.StatusOK || string(exported) != string(stagingBytes) {
		t.Fatalf("export = %d %q", status, exported)
	}
	var deleted dto.RepositoryRecoveryItemDTO
	server.mustJSON(http.MethodDelete, recoveryPath+"/items/"+staging.ID, nil, &deleted)
	if deleted.Disposition != "deleted" || deleted.Present {
		t.Fatalf("deleted item = %+v", deleted)
	}
	if status, _ := server.do(http.MethodDelete, recoveryPath+"/items/"+studio.ID, nil, ""); status != http.StatusConflict {
		t.Fatalf("deleting a reassociated Studio record answered %d, want 409", status)
	}
	if status, _ := server.do(http.MethodGet, recoveryPath+"/items/"+staging.ID+"/content", nil, ""); status != http.StatusNotFound {
		t.Fatalf("export after delete answered %d, want 404", status)
	}
	if _, err := os.Stat(filepath.Join(repository.Path, filepath.FromSlash(".lumilio/staging/incoming/abandoned.part"))); !os.IsNotExist(err) {
		t.Fatal("staging bytes were left in the ordinary staging directory")
	}

	// Accepting retention releases the hold; the file stays in the Trash and
	// now follows ordinary retention.
	var accepted dto.AssetRetentionAcceptedDTO
	server.mustJSON(http.MethodPost, "/api/v1/assets/accept-retention", map[string]any{
		"repository_id": repository.ID, "confirm": true,
	}, &accepted)
	if accepted.Entries != 1 {
		t.Fatalf("accept retention released %d entries, want 1", accepted.Entries)
	}
	server.mustJSON(http.MethodGet, "/api/v1/assets/trash-holds?repository_id="+url.QueryEscape(repository.ID), nil, &holds)
	if holds.Count != 0 {
		t.Fatalf("trash holds after acceptance = %+v", holds)
	}
	if still := server.listTrashedAssets(repository.ID); len(still) != 1 {
		t.Fatalf("Trash after acceptance lists %+v, want the file kept", still)
	}
	server.mustJSON(http.MethodGet, recoveryPath, nil, &inventory)
	if inventory.Trash.HeldCount != 0 {
		t.Fatalf("inventory after acceptance still holds %d", inventory.Trash.HeldCount)
	}
	// Restore from the Trash still works for the inherited file.
	server.mustJSON(http.MethodPost, "/api/v1/assets/"+trashed[0].AssetID+"/restore", nil, nil)
	if _, err := os.Stat(filepath.Join(repository.Path, "inbox", "deleted.png")); err != nil {
		t.Fatalf("restored inherited file is not back: %v", err)
	}
}
