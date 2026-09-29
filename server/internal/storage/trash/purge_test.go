package trash

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"

	"server/internal/db/catalogtx"
	"server/internal/db/repo"
	"server/internal/lifecycle"
)

func (f *fixture) assetExists(assetID uuid.UUID) bool {
	f.t.Helper()
	var count int
	if err := f.database.ReaderSQL.QueryRowContext(f.ctx, `SELECT count(*) FROM assets WHERE asset_id = ?`, assetID).Scan(&count); err != nil {
		f.t.Fatal(err)
	}
	return count == 1
}

func (f *fixture) ageTrash(assetID uuid.UUID, age time.Duration) {
	f.t.Helper()
	if _, err := f.database.SQL.ExecContext(f.ctx, `
		UPDATE repository_entries SET trashed_at = ? WHERE asset_id = ? AND state = 'trashed'`,
		time.Now().Add(-age).UnixMicro(), assetID); err != nil {
		f.t.Fatal(err)
	}
}

func TestExpiryDeletesOnlyFilesPastRetention(t *testing.T) {
	f := newFixture(t)
	repository := f.addRepository("photos")
	f.write(repository, "old.jpg", "an old photo")
	f.write(repository, "recent.jpg", "a recent photo")
	f.index(repository)
	old, recent := f.assetAt(repository, "old.jpg"), f.assetAt(repository, "recent.jpg")
	if _, err := f.trash.Delete(f.ctx, f.request(old, recent)); err != nil {
		t.Fatal(err)
	}
	f.ageTrash(old, 31*24*time.Hour)
	f.ageTrash(recent, 29*24*time.Hour)

	result, err := f.trash.Expire(f.ctx, 30*24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if result.Files != 1 || result.Assets != 1 {
		t.Fatalf("expiry = %+v, want one file and one Asset", result)
	}
	if f.assetExists(old) || !f.assetExists(recent) {
		t.Fatalf("after expiry: old Asset exists %t, recent Asset exists %t", f.assetExists(old), f.assetExists(recent))
	}
	files, sidecars := trashedFiles(t, repository.Path)
	if len(files) != 1 || len(sidecars) != 1 || filepath.Base(files[0]) != "recent.jpg" {
		t.Fatalf("trash after expiry holds %v and %v, want only recent.jpg", files, sidecars)
	}
	// Nothing is left to expire, and a second pass changes nothing.
	if again, err := f.trash.Expire(f.ctx, 30*24*time.Hour); err != nil || again.Entries != 0 {
		t.Fatalf("second expiry = %+v, %v", again, err)
	}
}

func TestExpiryPurgesATrashedEntryWhoseFileIsAlreadyGone(t *testing.T) {
	f := newFixture(t)
	repository := f.addRepository("photos")
	f.write(repository, "a.jpg", "photo a")
	f.index(repository)
	assetID := f.assetAt(repository, "a.jpg")
	if _, err := f.trash.Delete(f.ctx, f.request(assetID)); err != nil {
		t.Fatal(err)
	}
	// A crash after the unlink and before the purge, or a user emptying the
	// trash folder by hand, leaves the entry without its file.
	if err := os.RemoveAll(filepath.Join(repository.Path, ".lumilio", "trash")); err != nil {
		t.Fatal(err)
	}
	f.ageTrash(assetID, 31*24*time.Hour)
	if _, err := f.trash.Expire(f.ctx, 30*24*time.Hour); err != nil {
		t.Fatal(err)
	}
	if f.assetExists(assetID) {
		t.Fatal("an expired entry whose file was already gone was not purged")
	}
}

func TestDeletePermanentlyUnlinksAndPurgesTrashedAssets(t *testing.T) {
	f := newFixture(t)
	repository := f.addRepository("photos")
	f.write(repository, "a.jpg", "photo a")
	f.write(repository, "b.jpg", "photo b")
	f.index(repository)
	a, b := f.assetAt(repository, "a.jpg"), f.assetAt(repository, "b.jpg")

	var rejection *Rejection
	if _, err := f.trash.DeletePermanently(f.ctx, f.request(a)); !errors.As(err, &rejection) || rejection.Reason != ReasonNotTrashed {
		t.Fatalf("deleting an active Asset permanently = %v, want not_trashed", err)
	}
	if !f.exists(repository, "a.jpg") {
		t.Fatal("a refused permanent delete removed the file")
	}
	if _, err := f.trash.Delete(f.ctx, f.request(a, b)); err != nil {
		t.Fatal(err)
	}
	result, err := f.trash.DeletePermanently(f.ctx, f.request(a))
	if err != nil {
		t.Fatal(err)
	}
	if result.Files != 1 || result.Assets != 1 || result.Bytes != int64(len("photo a")) {
		t.Fatalf("permanent delete = %+v", result)
	}
	if f.assetExists(a) || !f.assetExists(b) {
		t.Fatal("permanent delete removed the wrong Asset")
	}
	files, _ := trashedFiles(t, repository.Path)
	if len(files) != 1 || filepath.Base(files[0]) != "b.jpg" {
		t.Fatalf("trash holds %v, want only b.jpg", files)
	}
}

func TestRemoveMissingPurgesOnlyMissingEntries(t *testing.T) {
	f := newFixture(t)
	repository := f.addRepository("photos")
	f.write(repository, "gone.jpg", "a photo deleted outside Lumilio")
	f.write(repository, "kept.jpg", "a photo still on disk")
	f.index(repository)
	gone, kept := f.assetAt(repository, "gone.jpg"), f.assetAt(repository, "kept.jpg")
	if err := os.Remove(f.path(repository, "gone.jpg")); err != nil {
		t.Fatal(err)
	}
	f.index(repository)
	if f.lifecycleState(gone) != "missing" {
		t.Fatalf("deleted file's Asset is %s, want missing", f.lifecycleState(gone))
	}

	result, err := f.trash.RemoveMissing(f.ctx, RemoveMissingRequest{Request: f.request(), RepositoryID: repository.RepoID})
	if err != nil {
		t.Fatal(err)
	}
	if result.Entries != 1 || result.Assets != 1 || result.Files != 0 {
		t.Fatalf("remove missing = %+v, want one entry and one Asset, no file", result)
	}
	if f.assetExists(gone) || f.lifecycleState(kept) != "active" {
		t.Fatal("remove missing purged the wrong Asset")
	}
	if !f.exists(repository, "kept.jpg") {
		t.Fatal("remove missing touched a file")
	}
}

func TestTrashIsRebuiltFromSidecarsAndCanBeRestored(t *testing.T) {
	f := newFixture(t)
	repository := f.addRepository("photos")
	f.write(repository, "trips/a.jpg", "photo a")
	f.index(repository)
	assetID := f.assetAt(repository, "trips/a.jpg")
	if _, err := f.trash.Delete(f.ctx, f.request(assetID)); err != nil {
		t.Fatal(err)
	}
	// The catalog forgets the Trash, as a catalog rebuilt from disk would.
	var entryID uuid.UUID
	if err := f.database.ReaderSQL.QueryRowContext(f.ctx, `SELECT entry_id FROM repository_entries WHERE asset_id = ?`, assetID).Scan(&entryID); err != nil {
		t.Fatal(err)
	}
	if err := f.database.WithTx(f.ctx, catalogtx.OperationRepositoryRemove, func(tx *sql.Tx, _ *repo.Queries) error {
		_, err := lifecycle.PurgeEntriesTx(f.ctx, tx, []uuid.UUID{entryID})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	// A sidecar from a newer build is reported and left alone.
	newer := filepath.Join(repository.Path, ".lumilio", "trash", "info", uuid.NewString()+".json")
	if err := os.WriteFile(newer, []byte(`{"format": 2}`), 0o600); err != nil {
		t.Fatal(err)
	}

	adopted, err := f.trash.Rebuild(f.ctx, repository.RepoID)
	if err != nil || adopted != 1 {
		t.Fatalf("rebuild adopted %d (%v), want 1", adopted, err)
	}
	if again, err := f.trash.Rebuild(f.ctx, repository.RepoID); err != nil || again != 0 {
		t.Fatalf("a second rebuild adopted %d (%v), want 0", again, err)
	}
	var rebuilt uuid.UUID
	var state string
	if err := f.database.ReaderSQL.QueryRowContext(f.ctx, `
		SELECT asset.asset_id, asset.lifecycle_state FROM repository_entries entry
		JOIN assets asset ON asset.asset_id = entry.asset_id
		WHERE entry.repository_id = ? AND entry.path = 'trips/a.jpg'`, repository.RepoID).Scan(&rebuilt, &state); err != nil {
		t.Fatal(err)
	}
	if state != "trashed" {
		t.Fatalf("rebuilt Asset is %s, want trashed", state)
	}
	if _, err := os.Stat(newer); err != nil {
		t.Fatalf("the newer-format sidecar was touched: %v", err)
	}
	if _, err := f.trash.Restore(f.ctx, f.request(rebuilt)); err != nil {
		t.Fatal(err)
	}
	if !f.exists(repository, "trips/a.jpg") || f.lifecycleState(rebuilt) != "active" {
		t.Fatal("the rebuilt Trash entry did not restore")
	}
}
