package recovery

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/zeebo/blake3"
	"go.uber.org/zap"

	"server/config"
	"server/internal/db"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/storage"
	"server/internal/storage/scan"
	"server/internal/storage/testfixture"
	"server/internal/storage/trash"
)

// fixture is a catalog with the storage manager, scanner, trash, and the
// recovery service under test; registrations go through the real commands.
type fixture struct {
	t        *testing.T
	ctx      context.Context
	database *db.DB
	files    *storage.RepositoryFSFactory
	manager  *storage.DefaultRepositoryManager
	scanner  *scan.Scanner
	trash    *trash.Service
	recovery *Service
	owner    int32
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	ctx := context.Background()
	databaseDirectory := t.TempDir()
	if err := os.Chmod(databaseDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	database, err := db.Open(ctx, config.DatabaseConfig{Path: filepath.Join(databaseDirectory, "catalog.sqlite3")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close(context.Background()) })
	if err := database.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	owner, err := database.Queries.CreateUser(ctx, repo.CreateUserParams{
		Username: "recovery-owner", Password: "unused", DisplayName: "Recovery Owner",
		Role: "admin", WebauthnUserHandle: []byte("recovery-owner-handle"),
	})
	if err != nil {
		t.Fatal(err)
	}
	files := storage.NewRepositoryFSFactory(storage.NewRepositoryAccessCoordinator(), database.Queries)
	manager, err := storage.NewRepositoryManager(database.SQL, database.Queries, zap.NewNop(), nil, files)
	if err != nil {
		t.Fatal(err)
	}
	scanner, err := scan.New(database.ReaderQueries, database, files, scan.Config{})
	if err != nil {
		t.Fatal(err)
	}
	assetTrash, err := trash.New(database.ReaderQueries, database, files, scanner, nil)
	if err != nil {
		t.Fatal(err)
	}
	service, err := New(database.ReaderQueries, database, files, scanner, assetTrash, nil)
	if err != nil {
		t.Fatal(err)
	}
	return &fixture{t: t, ctx: ctx, database: database, files: files, manager: manager, scanner: scanner, trash: assetTrash, recovery: service, owner: owner.UserID}
}

// openLeftover registers the P0 leftover-primary fixture as a regular
// Repository of an external Storage Location, the way a user opens a tree
// whose catalog is gone. seed runs before the open to add more private state.
func (f *fixture) openLeftover(seed func(repository testfixture.Repository)) (*repo.Repository, testfixture.Repository) {
	f.t.Helper()
	location, primary := testfixture.NewLeftoverPrimary(f.t)
	if seed != nil {
		seed(primary)
	}
	if _, err := f.manager.AddStorageLocation(f.ctx, location.Path, "Fixture", storage.LifecycleRequest{RequestID: "add-" + uuid.NewString(), Actor: "test"}); err != nil {
		f.t.Fatal(err)
	}
	opened, err := f.manager.OpenRepository(f.ctx, primary.Path, &f.owner, dbtypes.RepoRoleRegular,
		storage.LifecycleRequest{RequestID: "open-" + uuid.NewString(), Actor: "web:user:1"})
	if err != nil {
		f.t.Fatal(err)
	}
	return opened, primary
}

// index scans and hashes a repository until every file is bound.
func (f *fixture) index(repository *repo.Repository) {
	f.t.Helper()
	requested, _, err := f.scanner.Request(f.ctx, repository.RepoID, scan.TriggerManual, "", "test", time.Time{})
	if err != nil {
		f.t.Fatal(err)
	}
	for turn := 0; ; turn++ {
		current, err := f.database.ReaderQueries.GetRepositoryScan(f.ctx, requested.ScanID)
		if err != nil {
			f.t.Fatal(err)
		}
		if current.Status != scan.StatusQueued && current.Status != scan.StatusWalking && current.Status != scan.StatusSweeping {
			break
		}
		if turn > 10_000 {
			f.t.Fatal("scan did not finish")
		}
		if _, err := f.scanner.RunTurn(f.ctx, repository.RepoID); err != nil {
			f.t.Fatal(err)
		}
	}
	for pass := 0; pass < 10_000; pass++ {
		result, err := f.scanner.HashTurn(f.ctx, repository.RepoID, scan.MaxBatchRows)
		if err != nil {
			f.t.Fatal(err)
		}
		if result.Bound+result.Deferred+result.Errors == 0 {
			return
		}
	}
	f.t.Fatal("hashing did not drain")
}

func (f *fixture) assetAt(repository *repo.Repository, relative string) uuid.UUID {
	f.t.Helper()
	var id uuid.UUID
	if err := f.database.ReaderSQL.QueryRowContext(f.ctx, `
		SELECT asset_id FROM repository_entries WHERE repository_id = ? AND path = ? AND state = 'present'`,
		repository.RepoID, relative).Scan(&id); err != nil {
		f.t.Fatalf("no present entry at %s: %v", relative, err)
	}
	return id
}

func (f *fixture) auditCount(action string) int {
	f.t.Helper()
	var count int
	if err := f.database.ReaderSQL.QueryRowContext(f.ctx, `SELECT count(*) FROM lifecycle_audit_events WHERE action = ?`, action).Scan(&count); err != nil {
		f.t.Fatal(err)
	}
	return count
}

func (f *fixture) trashHolds(repository *repo.Repository) (held, total int) {
	f.t.Helper()
	if err := f.database.ReaderSQL.QueryRowContext(f.ctx, `
		SELECT count(CASE WHEN trash_hold IS NOT NULL THEN 1 END), count(*) FROM repository_entries
		WHERE repository_id = ? AND state = 'trashed'`, repository.RepoID).Scan(&held, &total); err != nil {
		f.t.Fatal(err)
	}
	return held, total
}

func writeStudioRecord(t *testing.T, repository testfixture.Repository, assetID string, source map[string]any, version int) string {
	t.Helper()
	record := map[string]any{"version": version, "asset_id": assetID, "updated_at": "2020-01-02T03:04:05Z", "source": source, "adjustments": map[string]any{"exposure": 0.25}}
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	relative := ".lumilio/sidecars/" + assetID + ".lumilio-sidecar"
	if err := os.WriteFile(filepath.Join(repository.Path, filepath.FromSlash(relative)), data, 0o644); err != nil {
		t.Fatal(err)
	}
	return relative
}

func contentHash(data []byte) string {
	sum := blake3.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func itemAt(inventory Inventory, path string) (Item, bool) {
	for _, item := range inventory.Items {
		if item.Path == path {
			return item, true
		}
	}
	return Item{}, false
}

// R4 / P4: a Repository opened into a catalog that never knew it keeps its
// private state reachable. The inherited Trash is adopted and held, the
// Studio record whose content is proven is reassociated to the new Asset,
// records that cannot be proven stay for review, staging and unknown bytes
// are listed with export and delete, and a second pass changes nothing.
func TestReconcileRecoversInheritedTrashAndProvenStudioRecords(t *testing.T) {
	f := newFixture(t)
	const orphanID = "66666666-6666-4666-8666-666666666666"
	const replacedID = "77777777-7777-4777-8777-777777777777"
	const newerID = "88888888-8888-4888-8888-888888888888"
	var original []byte
	opened, primary := f.openLeftover(func(repository testfixture.Repository) {
		var err error
		original, err = os.ReadFile(filepath.Join(repository.Path, filepath.FromSlash(testfixture.OriginalPath)))
		if err != nil {
			t.Fatal(err)
		}
		other := append([]byte{}, original...)
		other = append(other, "\x00other"...)
		if err := os.WriteFile(filepath.Join(repository.Path, "inbox", "other.png"), other, 0o644); err != nil {
			t.Fatal(err)
		}
		// No present file has this content.
		writeStudioRecord(t, repository, orphanID, map[string]any{
			"original_filename": "gone.png", "storage_path": "inbox/gone.png", "mime_type": "image/png",
			"file_size": 7, "hash": strings.Repeat("ef", 32),
		}, 1)
		// The content exists, but the file at the record's source path is a
		// different one: the record is not applied to it.
		writeStudioRecord(t, repository, replacedID, map[string]any{
			"original_filename": "other.png", "storage_path": "inbox/other.png", "mime_type": "image/png",
			"file_size": len(original), "hash": contentHash(original),
		}, 1)
		writeStudioRecord(t, repository, newerID, map[string]any{"storage_path": testfixture.OriginalPath}, 2)
	})
	studioBefore, err := os.ReadFile(filepath.Join(primary.Path, filepath.FromSlash(testfixture.StudioPath)))
	if err != nil {
		t.Fatal(err)
	}

	// Before the scan nothing about content is proven: the Trash is adopted
	// and held, every Studio record waits.
	first, err := f.recovery.Reconcile(f.ctx, opened.RepoID)
	if err != nil || first.TrashAdopted != 1 || first.StudioReassociated != 0 || first.StudioAmbiguous != 0 {
		t.Fatalf("reconcile before the scan = %+v, err=%v", first, err)
	}
	if held, total := f.trashHolds(opened); held != 1 || total != 1 {
		t.Fatalf("trash holds = %d of %d, want the inherited file held", held, total)
	}
	if _, err := os.Stat(filepath.Join(primary.Path, filepath.FromSlash(testfixture.StudioPath))); err != nil {
		t.Fatalf("the Studio record was touched before content was proven: %v", err)
	}

	f.index(opened)
	assetID := f.assetAt(opened, testfixture.OriginalPath)
	otherID := f.assetAt(opened, "inbox/other.png")
	if assetID == otherID || assetID.String() == testfixture.AssetID {
		t.Fatalf("new catalog minted assets %s and %s", assetID, otherID)
	}
	second, err := f.recovery.Reconcile(f.ctx, opened.RepoID)
	if err != nil || second.TrashAdopted != 0 || second.StudioReassociated != 1 || second.StudioAmbiguous != 2 {
		t.Fatalf("reconcile after the scan = %+v, err=%v", second, err)
	}
	reassociated := filepath.Join(primary.Path, filepath.FromSlash(".lumilio/sidecars/"+assetID.String()+".lumilio-sidecar"))
	data, err := os.ReadFile(reassociated)
	if err != nil {
		t.Fatalf("reassociated record missing: %v", err)
	}
	var record map[string]any
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatal(err)
	}
	var before map[string]any
	_ = json.Unmarshal(studioBefore, &before)
	if record["asset_id"] != assetID.String() || record["adjustments"].(map[string]any)["exposure"] != before["adjustments"].(map[string]any)["exposure"] {
		t.Fatalf("reassociated record = %v", record)
	}
	if record["source"].(map[string]any)["hash"] != before["source"].(map[string]any)["hash"] {
		t.Fatal("reassociation changed the record's content identity")
	}
	if _, err := os.Stat(filepath.Join(primary.Path, filepath.FromSlash(testfixture.StudioPath))); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the superseded record name remains: %v", err)
	}
	for _, kept := range []string{orphanID, replacedID, newerID} {
		if _, err := os.Stat(filepath.Join(primary.Path, filepath.FromSlash(".lumilio/sidecars/"+kept+".lumilio-sidecar"))); err != nil {
			t.Fatalf("an unproven record was moved or removed: %s: %v", kept, err)
		}
	}
	if f.auditCount("studio_reassociated") != 1 {
		t.Fatalf("studio_reassociated audits = %d", f.auditCount("studio_reassociated"))
	}
	// Idempotent: a third pass changes nothing and moves nothing.
	third, err := f.recovery.Reconcile(f.ctx, opened.RepoID)
	if err != nil || third != (ReconcileResult{}) {
		t.Fatalf("third reconcile = %+v, err=%v", third, err)
	}
	if f.auditCount("studio_reassociated") != 1 {
		t.Fatal("a second pass audited a reassociation again")
	}

	inventory, err := f.recovery.Inventory(f.ctx, opened.RepoID)
	if err != nil {
		t.Fatal(err)
	}
	if len(inventory.Receipts) != 1 || inventory.Receipts[0].Event != storage.RecoveryEventOpen || inventory.Receipts[0].Trash.Files != 2 || inventory.TrashHeld.Count != 1 {
		t.Fatalf("inventory header = %+v held=%+v", inventory.Receipts, inventory.TrashHeld)
	}
	studio, ok := itemAt(inventory, testfixture.StudioPath)
	if !ok || studio.Disposition != storage.DispositionReassociated || studio.ResolvedAssetID != assetID.String() || !studio.Present || len(studio.Actions) != 0 {
		t.Fatalf("studio item = %+v", studio)
	}
	orphan, _ := itemAt(inventory, ".lumilio/sidecars/"+orphanID+".lumilio-sidecar")
	replaced, _ := itemAt(inventory, ".lumilio/sidecars/"+replacedID+".lumilio-sidecar")
	newer, _ := itemAt(inventory, ".lumilio/sidecars/"+newerID+".lumilio-sidecar")
	if orphan.Disposition != storage.DispositionAmbiguous || !strings.Contains(orphan.Detail, "no present file") ||
		replaced.Disposition != storage.DispositionAmbiguous || !strings.Contains(replaced.Detail, "different content") ||
		newer.Disposition != storage.DispositionUnreadable {
		t.Fatalf("unproven records: orphan=%+v replaced=%+v newer=%+v", orphan, replaced, newer)
	}
	for _, reviewable := range []Item{orphan, replaced, newer} {
		if !reviewable.Present || strings.Join(reviewable.Actions, ",") != "export,delete" {
			t.Fatalf("reviewable record actions = %+v", reviewable)
		}
	}
	staging, ok := itemAt(inventory, ".lumilio/staging/incoming/abandoned.part")
	if !ok || staging.Class != storage.PrivateClassStaging || staging.Disposition != storage.DispositionMoved || !staging.Present || strings.Join(staging.Actions, ",") != "export,delete" {
		t.Fatalf("staging item = %+v", staging)
	}
	unknown, ok := itemAt(inventory, ".lumilio/unknown-child/opaque.bin")
	if !ok || unknown.Class != storage.PrivateClassUnknown || !unknown.Present {
		t.Fatalf("unknown item = %+v", unknown)
	}

	// Export reads the preserved bytes; delete removes them once, explicitly.
	reader, exported, err := f.recovery.OpenItem(f.ctx, opened.RepoID, staging.ItemID)
	if err != nil {
		t.Fatal(err)
	}
	bytesOut, err := io.ReadAll(reader)
	_ = reader.Close()
	if err != nil || string(bytesOut) != "uncommitted incoming bytes\n" || exported.Size != int64(len(bytesOut)) {
		t.Fatalf("exported %q (size %d), err=%v", bytesOut, exported.Size, err)
	}
	if _, err := f.recovery.DeleteItem(f.ctx, opened.RepoID, studio.ItemID, Request{Actor: "web:user:1"}); !errors.Is(err, ErrItemNotDeletable) {
		t.Fatalf("deleting a reassociated record = %v, want ErrItemNotDeletable", err)
	}
	deleted, err := f.recovery.DeleteItem(f.ctx, opened.RepoID, staging.ItemID, Request{Actor: "web:user:1"})
	if err != nil || deleted.Disposition != storage.DispositionDeleted || deleted.Present {
		t.Fatalf("deleted item = %+v, err=%v", deleted, err)
	}
	if _, err := os.Stat(filepath.Join(primary.Path, filepath.FromSlash(staging.MovedTo))); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("deleted item still on disk: %v", err)
	}
	if again, err := f.recovery.DeleteItem(f.ctx, opened.RepoID, staging.ItemID, Request{Actor: "web:user:1"}); err != nil || again.Disposition != storage.DispositionDeleted {
		t.Fatalf("second delete = %+v, err=%v", again, err)
	}
	if f.auditCount("recovery_item_deleted") != 1 {
		t.Fatalf("recovery_item_deleted audits = %d", f.auditCount("recovery_item_deleted"))
	}
	if _, _, err := f.recovery.OpenItem(f.ctx, opened.RepoID, staging.ItemID); !errors.Is(err, ErrItemNotFound) {
		t.Fatalf("export after delete = %v", err)
	}
	if _, _, err := f.recovery.OpenItem(f.ctx, opened.RepoID, uuid.NewString()); !errors.Is(err, ErrItemNotFound) {
		t.Fatalf("export of an unknown item = %v", err)
	}
	after, err := f.recovery.Inventory(f.ctx, opened.RepoID)
	if err != nil {
		t.Fatal(err)
	}
	if item, _ := itemAt(after, staging.Path); item.Disposition != storage.DispositionDeleted || item.Present || len(item.Actions) != 0 {
		t.Fatalf("deleted item in the inventory = %+v", item)
	}
}

// A crash after the reassociated record is written and recorded, before
// the superseded name is removed, is finished by the next pass: one record,
// under the new name, and no second write.
func TestReconcileFinishesAnInterruptedReassociation(t *testing.T) {
	f := newFixture(t)
	opened, primary := f.openLeftover(nil)
	f.index(opened)
	assetID := f.assetAt(opened, testfixture.OriginalPath)
	crash := errors.New("simulated crash")
	f.recovery.afterStudioWrite = func() error { return crash }
	if _, err := f.recovery.Reconcile(f.ctx, opened.RepoID); !errors.Is(err, crash) {
		t.Fatalf("reconcile error = %v, want the injected crash", err)
	}
	f.recovery.afterStudioWrite = nil
	newName := filepath.Join(primary.Path, filepath.FromSlash(".lumilio/sidecars/"+assetID.String()+".lumilio-sidecar"))
	oldName := filepath.Join(primary.Path, filepath.FromSlash(testfixture.StudioPath))
	if _, err := os.Stat(newName); err != nil {
		t.Fatalf("the new record was not written before the crash: %v", err)
	}
	if _, err := os.Stat(oldName); err != nil {
		t.Fatalf("the old record was removed before the crash point: %v", err)
	}
	inventory, err := f.recovery.Inventory(f.ctx, opened.RepoID)
	if err != nil {
		t.Fatal(err)
	}
	if item, _ := itemAt(inventory, testfixture.StudioPath); item.Disposition != storage.DispositionReassociated || item.ResolvedAssetID != assetID.String() {
		t.Fatalf("receipt after the crash = %+v, want the reassociation recorded before the removal", item)
	}
	written, err := os.ReadFile(newName)
	if err != nil {
		t.Fatal(err)
	}
	result, err := f.recovery.Reconcile(f.ctx, opened.RepoID)
	if err != nil || result.StudioReassociated != 0 {
		t.Fatalf("retry = %+v, err=%v", result, err)
	}
	if _, err := os.Stat(oldName); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the retry left the superseded record: %v", err)
	}
	rewritten, err := os.ReadFile(newName)
	if err != nil || string(rewritten) != string(written) {
		t.Fatalf("the retry rewrote the record: %v", err)
	}
	if again, err := f.recovery.Reconcile(f.ctx, opened.RepoID); err != nil || again != (ReconcileResult{}) {
		t.Fatalf("pass after the retry = %+v, err=%v", again, err)
	}
}

// An offline repository is skipped by the maintenance pass and refuses the
// explicit actions with a typed error, never a partial write.
func TestRecoveryRefusesAnUnreachableRepository(t *testing.T) {
	f := newFixture(t)
	opened, _ := f.openLeftover(nil)
	now := dbtypes.NewTimestamp(time.Now().UTC())
	if _, err := f.database.Queries.UpdateRepositoryReachability(f.ctx, repo.UpdateRepositoryReachabilityParams{
		RepoID: opened.RepoID, Reachability: dbtypes.RepositoryReachabilityOffline, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.recovery.Inventory(f.ctx, opened.RepoID); !errors.Is(err, ErrRepositoryOffline) {
		t.Fatalf("inventory of an offline repository = %v", err)
	}
	if _, err := f.recovery.DeleteItem(f.ctx, opened.RepoID, "any", Request{}); !errors.Is(err, ErrRepositoryOffline) {
		t.Fatalf("delete on an offline repository = %v", err)
	}
	if result, err := f.recovery.ReconcileAll(f.ctx); err != nil || result != (ReconcileResult{}) {
		t.Fatalf("reconcile all with an offline repository = %+v, err=%v", result, err)
	}
}
