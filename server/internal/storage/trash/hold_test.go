package trash

import (
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"

	"server/internal/db/catalogtx"
	"server/internal/db/repo"
	"server/internal/lifecycle"
)

// forgetTrash purges the catalog rows of an Asset's trashed files, as a lost
// catalog would; the disk keeps the files and their sidecars.
func (f *fixture) forgetTrash(assetID uuid.UUID) {
	f.t.Helper()
	rows, err := f.database.ReaderSQL.QueryContext(f.ctx, `SELECT entry_id FROM repository_entries WHERE asset_id = ?`, assetID)
	if err != nil {
		f.t.Fatal(err)
	}
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			f.t.Fatal(err)
		}
		ids = append(ids, id)
	}
	_ = rows.Close()
	if err := f.database.WithTx(f.ctx, catalogtx.OperationRepositoryRemove, func(tx *sql.Tx, _ *repo.Queries) error {
		_, err := lifecycle.PurgeEntriesTx(f.ctx, tx, ids)
		return err
	}); err != nil {
		f.t.Fatal(err)
	}
}

func (f *fixture) trashHold(repository repo.Repository, relative string) (hold *string, trashedAt int64) {
	f.t.Helper()
	if err := f.database.ReaderSQL.QueryRowContext(f.ctx, `
		SELECT trash_hold, trashed_at FROM repository_entries
		WHERE repository_id = ? AND path = ? AND state = 'trashed'`, repository.RepoID, relative).Scan(&hold, &trashedAt); err != nil {
		f.t.Fatalf("no trashed entry at %s: %v", relative, err)
	}
	return hold, trashedAt
}

func (f *fixture) auditCount(action string) int {
	f.t.Helper()
	var count int
	if err := f.database.ReaderSQL.QueryRowContext(f.ctx, `SELECT count(*) FROM lifecycle_audit_events WHERE action = ?`, action).Scan(&count); err != nil {
		f.t.Fatal(err)
	}
	return count
}

// P4 / R4: inherited Trash is held. A file the rebuild adopts from a sidecar
// never expires on its own, however old its deletion; a file this catalog
// deleted follows ordinary retention; the hold survives another rebuild, and
// Restore ends it so a later Delete is ordinary again.
func TestInheritedTrashIsHeldPastRetentionUntilTheUserDecides(t *testing.T) {
	f := newFixture(t)
	repository := f.addRepository("photos")
	f.write(repository, "inherited.jpg", "deleted before this catalog existed")
	f.write(repository, "ordinary.jpg", "deleted by this catalog")
	f.index(repository)
	inherited, ordinary := f.assetAt(repository, "inherited.jpg"), f.assetAt(repository, "ordinary.jpg")
	if _, err := f.trash.Delete(f.ctx, f.request(inherited, ordinary)); err != nil {
		t.Fatal(err)
	}
	f.forgetTrash(inherited)
	f.rebuild(repository, 1)
	adopted := f.assetAtAnyState(repository, "inherited.jpg")
	if hold, _ := f.trashHold(repository, "inherited.jpg"); hold == nil || *hold != "inherited" {
		t.Fatalf("adopted entry hold = %v, want inherited", hold)
	}
	if hold, _ := f.trashHold(repository, "ordinary.jpg"); hold != nil {
		t.Fatalf("this catalog's own Delete is held: %q", *hold)
	}
	held, err := f.trash.HeldAssetIDs(f.ctx, uuid.NullUUID{UUID: repository.RepoID, Valid: true}, nil)
	if err != nil || len(held) != 1 || held[0] != adopted {
		t.Fatalf("held Assets = %v, err=%v, want [%s]", held, err, adopted)
	}
	// Both are far past retention; only the ordinary one expires.
	f.ageTrash(adopted, 400*24*time.Hour)
	f.ageTrash(ordinary, 400*24*time.Hour)
	expired, err := f.trash.Expire(f.ctx, 30*24*time.Hour)
	if err != nil || expired.Files != 1 {
		t.Fatalf("expiry = %+v, err=%v, want exactly the ordinary file", expired, err)
	}
	if f.assetExists(ordinary) || !f.assetExists(adopted) {
		t.Fatalf("after expiry: ordinary exists %t, inherited exists %t", f.assetExists(ordinary), f.assetExists(adopted))
	}
	if files, _ := trashedFiles(t, repository.Path); len(files) != 1 {
		t.Fatalf("trash holds %v, want the inherited file only", files)
	}
	// The hold survives a second rebuild and a second catalog loss.
	f.rebuild(repository, 0)
	if hold, _ := f.trashHold(repository, "inherited.jpg"); hold == nil {
		t.Fatal("a rebuild released the hold")
	}
	f.forgetTrash(adopted)
	f.rebuild(repository, 1)
	readopted := f.assetAtAnyState(repository, "inherited.jpg")
	if hold, _ := f.trashHold(repository, "inherited.jpg"); hold == nil {
		t.Fatal("a second catalog loss lost the hold")
	}
	f.ageTrash(readopted, 400*24*time.Hour)
	if expired, err := f.trash.Expire(f.ctx, 30*24*time.Hour); err != nil || expired.Files != 0 {
		t.Fatalf("expiry after the second rebuild = %+v, err=%v, want nothing", expired, err)
	}
	// Restore ends the hold; a later Delete is this catalog's own and ordinary.
	if _, err := f.trash.Restore(f.ctx, f.request(readopted)); err != nil {
		t.Fatal(err)
	}
	if _, err := f.trash.Delete(f.ctx, f.request(readopted)); err != nil {
		t.Fatal(err)
	}
	if hold, _ := f.trashHold(repository, "inherited.jpg"); hold != nil {
		t.Fatalf("a Delete after Restore is held: %q", *hold)
	}
	f.ageTrash(readopted, 400*24*time.Hour)
	if expired, err := f.trash.Expire(f.ctx, 30*24*time.Hour); err != nil || expired.Files != 1 {
		t.Fatalf("expiry of the re-deleted file = %+v, err=%v", expired, err)
	}
}

// Accepting retention releases the hold and starts the retention clock at
// the acceptance, so an inherited file deleted years ago still gets its full
// retention from the moment the user agreed to it.
func TestAcceptRetentionReleasesTheHoldFromTheAcceptance(t *testing.T) {
	f := newFixture(t)
	repository := f.addRepository("photos")
	f.write(repository, "a.jpg", "photo a")
	f.write(repository, "b.jpg", "photo b")
	f.index(repository)
	a, b := f.assetAt(repository, "a.jpg"), f.assetAt(repository, "b.jpg")
	if _, err := f.trash.Delete(f.ctx, f.request(a, b)); err != nil {
		t.Fatal(err)
	}
	f.forgetTrash(a)
	f.forgetTrash(b)
	f.rebuild(repository, 2)
	a, b = f.assetAtAnyState(repository, "a.jpg"), f.assetAtAnyState(repository, "b.jpg")
	f.ageTrash(a, 400*24*time.Hour)
	f.ageTrash(b, 400*24*time.Hour)

	accepted := time.Now().Add(-10 * 24 * time.Hour).UTC()
	f.trash.now = func() time.Time { return accepted }
	result, err := f.trash.AcceptRetention(f.ctx, f.request(a), uuid.NullUUID{}, nil)
	if err != nil || result.Entries != 1 {
		t.Fatalf("accept retention = %+v, err=%v", result, err)
	}
	if hold, trashedAt := f.trashHold(repository, "a.jpg"); hold != nil || trashedAt != accepted.UnixMicro() {
		t.Fatalf("after acceptance: hold=%v trashed_at=%d, want released at %d", hold, trashedAt, accepted.UnixMicro())
	}
	if hold, _ := f.trashHold(repository, "b.jpg"); hold == nil {
		t.Fatal("acceptance for a released b's hold")
	}
	if f.auditCount("trash_hold_released") != 1 {
		t.Fatal("acceptance was not audited")
	}
	// Ten days into a thirty-day retention nothing expires; after it, a does.
	f.trash.now = func() time.Time { return time.Now().UTC() }
	if expired, err := f.trash.Expire(f.ctx, 30*24*time.Hour); err != nil || expired.Files != 0 {
		t.Fatalf("expiry inside the accepted retention = %+v, err=%v", expired, err)
	}
	f.trash.now = func() time.Time { return time.Now().Add(25 * 24 * time.Hour).UTC() }
	if expired, err := f.trash.Expire(f.ctx, 30*24*time.Hour); err != nil || expired.Files != 1 || f.assetExists(a) || !f.assetExists(b) {
		t.Fatalf("expiry after the accepted retention = %+v, err=%v (a exists %t, b exists %t)", expired, err, f.assetExists(a), f.assetExists(b))
	}
	// Repository-wide acceptance in the owner scope releases the rest; an
	// acceptance with nothing held is a no-op without an audit.
	owner := f.owner
	if result, err := f.trash.AcceptRetention(f.ctx, f.request(), uuid.NullUUID{UUID: repository.RepoID, Valid: true}, &owner); err != nil || result.Entries != 1 {
		t.Fatalf("repository acceptance = %+v, err=%v", result, err)
	}
	if result, err := f.trash.AcceptRetention(f.ctx, f.request(), uuid.NullUUID{UUID: repository.RepoID, Valid: true}, &owner); err != nil || result.Entries != 0 {
		t.Fatalf("second repository acceptance = %+v, err=%v", result, err)
	}
	if f.auditCount("trash_hold_released") != 2 {
		t.Fatalf("hold-release audits = %d, want 2", f.auditCount("trash_hold_released"))
	}
	if held, _ := f.trash.HeldAssetIDs(f.ctx, uuid.NullUUID{}, nil); len(held) != 0 {
		t.Fatalf("held Assets after acceptance = %v", held)
	}
}
