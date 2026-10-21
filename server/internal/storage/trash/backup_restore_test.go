package trash

import (
	"testing"

	"github.com/google/uuid"

	"server/internal/db/repo"
)

// forgetDelete returns the catalog to what a backup taken before the Delete
// holds: the file is listed as present at its path and the catalog knows no
// Trash. The files on disk keep the Delete's state, so the original is gone
// and the trash holds the file with its sidecar.
func (f *fixture) forgetDelete(assetID uuid.UUID) {
	f.t.Helper()
	if _, err := f.database.SQL.ExecContext(f.ctx, `
		UPDATE repository_entries
		SET state = 'present', trash_id = NULL, trashed_at = NULL, missing_since = NULL, revision = revision + 1
		WHERE asset_id = ? AND state = 'trashed'`, assetID); err != nil {
		f.t.Fatal(err)
	}
}

func (f *fixture) rebuild(repository repo.Repository, wantAdopted int) {
	f.t.Helper()
	adopted, err := f.trash.Rebuild(f.ctx, repository.RepoID)
	if err != nil || adopted != wantAdopted {
		f.t.Fatalf("rebuild adopted %d (%v), want %d", adopted, err, wantAdopted)
	}
}

// #223: a backup taken before a Delete is restored. The catalog then lists
// the deleted file as present while the disk holds it in the trash. Whether
// the Trash rebuild or the scan runs first, the Asset ends in the Trash with
// its metadata, with exactly one row for the file, and Restore returns it.
func TestARestoredBackupTakenBeforeADeleteListsTheFileInTheTrash(t *testing.T) {
	orders := []struct {
		name string
		run  func(f *fixture, repository repo.Repository)
	}{
		{"rebuild then scan", func(f *fixture, repository repo.Repository) {
			f.rebuild(repository, 1)
			f.index(repository)
		}},
		{"scan then rebuild", func(f *fixture, repository repo.Repository) {
			f.index(repository)
			if state := f.lifecycleState(f.assetAtAnyState(repository, "trips/a.jpg")); state != "missing" {
				t.Fatalf("lifecycle state after the scan alone = %s, want missing", state)
			}
			f.rebuild(repository, 1)
		}},
	}
	for _, order := range orders {
		t.Run(order.name, func(t *testing.T) {
			f := newFixture(t)
			repository := f.addRepository("photos")
			f.write(repository, "trips/a.jpg", "photo a")
			f.index(repository)
			assetID := f.assetAt(repository, "trips/a.jpg")
			f.setRating(assetID, 5)
			if _, err := f.trash.Delete(f.ctx, f.request(assetID)); err != nil {
				t.Fatal(err)
			}
			f.forgetDelete(assetID)
			if state := f.lifecycleState(assetID); state != "active" {
				t.Fatalf("lifecycle state of the restored backup = %s, want active", state)
			}

			order.run(f, repository)

			if states := f.entryStates(repository)["trips/a.jpg"]; len(states) != 1 || states[0] != "trashed" {
				t.Fatalf("entries at trips/a.jpg = %v, want exactly one trashed row", states)
			}
			if state := f.lifecycleState(assetID); state != "trashed" {
				t.Fatalf("lifecycle state = %s, want trashed (the Trash view lists the file)", state)
			}
			if count := f.assetCount(); count != 1 {
				t.Fatalf("%d Assets, want the one the backup knew", count)
			}
			if _, err := f.trash.Restore(f.ctx, f.request(assetID)); err != nil {
				t.Fatal(err)
			}
			if !f.exists(repository, "trips/a.jpg") || f.lifecycleState(assetID) != "active" {
				t.Fatal("the rebuilt Trash entry did not restore")
			}
			if rating, liked := f.rating(assetID); rating != 5 || !liked {
				t.Fatalf("metadata after restore = rating %d liked %t, want 5 and true", rating, liked)
			}
			if files, sidecars := trashedFiles(t, repository.Path); len(files)+len(sidecars) != 0 {
				t.Fatalf("restore left %v and %v in the trash", files, sidecars)
			}
		})
	}
}

// assetAtAnyState is the Asset bound to the file at a path, in any state.
func (f *fixture) assetAtAnyState(repository repo.Repository, relative string) uuid.UUID {
	f.t.Helper()
	var id uuid.UUID
	if err := f.database.ReaderSQL.QueryRowContext(f.ctx, `
		SELECT asset_id FROM repository_entries
		WHERE repository_id = ? AND path = ? AND asset_id IS NOT NULL LIMIT 1
	`, repository.RepoID, relative).Scan(&id); err != nil {
		f.t.Fatalf("no entry at %s: %v", relative, err)
	}
	return id
}

func (f *fixture) assetCount() int {
	f.t.Helper()
	var count int
	if err := f.database.ReaderSQL.QueryRowContext(f.ctx, `SELECT count(*) FROM assets`).Scan(&count); err != nil {
		f.t.Fatal(err)
	}
	return count
}
