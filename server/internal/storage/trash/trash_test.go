package trash

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
)

func (f *fixture) setRating(assetID uuid.UUID, rating int) {
	f.t.Helper()
	if _, err := f.database.SQL.ExecContext(f.ctx, `UPDATE assets SET rating = ?, liked = 1 WHERE asset_id = ?`, rating, assetID); err != nil {
		f.t.Fatal(err)
	}
}

func (f *fixture) rating(assetID uuid.UUID) (int, bool) {
	f.t.Helper()
	var rating int
	var liked bool
	if err := f.database.ReaderSQL.QueryRowContext(f.ctx, `SELECT rating, liked FROM assets WHERE asset_id = ?`, assetID).Scan(&rating, &liked); err != nil {
		f.t.Fatal(err)
	}
	return rating, liked
}

// trashedFiles lists the files in a repository's trash and its sidecars.
func trashedFiles(t *testing.T, repositoryPath string) (files, sidecars []string) {
	t.Helper()
	files, _ = filepath.Glob(filepath.Join(repositoryPath, ".lumilio", "trash", "files", "*", "*"))
	sidecars, _ = filepath.Glob(filepath.Join(repositoryPath, ".lumilio", "trash", "info", "*.json"))
	return files, sidecars
}

func TestDeleteMovesEveryCopyAndRestoreBringsThemBackWithMetadata(t *testing.T) {
	f := newFixture(t)
	first, second := f.addRepository("first"), f.addRepository("second")
	f.write(first, "trips/a.jpg", "same photo")
	f.write(second, "copies/a.jpg", "same photo")
	f.index(first)
	f.index(second)
	assetID := f.assetAt(first, "trips/a.jpg")
	if other := f.assetAt(second, "copies/a.jpg"); other != assetID {
		t.Fatalf("copies bound to %s and %s, want one Asset", assetID, other)
	}
	f.setRating(assetID, 5)

	result, err := f.trash.Delete(f.ctx, f.request(assetID))
	if err != nil {
		t.Fatal(err)
	}
	if result.Assets != 1 || result.Files != 2 || len(result.Repositories) != 2 {
		t.Fatalf("delete result = %+v, want one Asset with two files in two repositories", result)
	}
	for _, check := range []struct {
		repository string
		original   string
	}{{first.Path, "trips/a.jpg"}, {second.Path, "copies/a.jpg"}} {
		if _, err := os.Lstat(filepath.Join(check.repository, filepath.FromSlash(check.original))); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("%s is still at its original path (%v)", check.original, err)
		}
		files, sidecars := trashedFiles(t, check.repository)
		if len(files) != 1 || len(sidecars) != 1 || filepath.Base(files[0]) != "a.jpg" {
			t.Fatalf("trash of %s holds files %v and sidecars %v", check.repository, files, sidecars)
		}
		data, err := os.ReadFile(sidecars[0])
		if err != nil {
			t.Fatal(err)
		}
		sidecar, err := ParseSidecar(data)
		if err != nil {
			t.Fatal(err)
		}
		if sidecar.OriginalPath != check.original || sidecar.AssetID != assetID.String() || sidecar.Actor != "test" {
			t.Fatalf("sidecar = %+v", sidecar)
		}
	}
	if state := f.lifecycleState(assetID); state != "trashed" {
		t.Fatalf("lifecycle state after delete = %s, want trashed", state)
	}
	if f.runningOperations() != 0 {
		t.Fatal("delete left its journal running")
	}

	if _, err := f.trash.Restore(f.ctx, f.request(assetID)); err != nil {
		t.Fatal(err)
	}
	if !f.exists(first, "trips/a.jpg") || !f.exists(second, "copies/a.jpg") {
		t.Fatal("restore did not put both files back at their original paths")
	}
	if state := f.lifecycleState(assetID); state != "active" {
		t.Fatalf("lifecycle state after restore = %s, want active", state)
	}
	if rating, liked := f.rating(assetID); rating != 5 || !liked {
		t.Fatalf("metadata after restore = rating %d liked %t, want 5 and true", rating, liked)
	}
	for _, repository := range []string{first.Path, second.Path} {
		if files, sidecars := trashedFiles(t, repository); len(files)+len(sidecars) != 0 {
			t.Fatalf("restore left %v and %v in the trash", files, sidecars)
		}
	}
	// A scan after the restore finds the files where the catalog says.
	f.index(first)
	if states := f.entryStates(first)["trips/a.jpg"]; len(states) != 1 || states[0] != "present" {
		t.Fatalf("entries at trips/a.jpg after a rescan = %v, want one present", states)
	}
}

func TestDeleteRefusesAChangedFileAndMovesNothing(t *testing.T) {
	f := newFixture(t)
	repository := f.addRepository("photos")
	f.write(repository, "a.jpg", "photo a")
	f.write(repository, "b.jpg", "photo b")
	f.index(repository)
	a, b := f.assetAt(repository, "a.jpg"), f.assetAt(repository, "b.jpg")
	f.write(repository, "b.jpg", "photo b, edited since the scan")

	_, err := f.trash.Delete(f.ctx, f.request(a, b))
	var rejection *Rejection
	if !errors.As(err, &rejection) || rejection.Reason != ReasonFileChanged || rejection.Path != "b.jpg" {
		t.Fatalf("delete error = %v, want a file_changed rejection for b.jpg", err)
	}
	if !f.exists(repository, "a.jpg") || !f.exists(repository, "b.jpg") {
		t.Fatal("a refused delete moved a file")
	}
	if f.lifecycleState(a) != "active" || f.lifecycleState(b) != "active" {
		t.Fatal("a refused delete changed the catalog")
	}
	if files, sidecars := trashedFiles(t, repository.Path); len(files)+len(sidecars) != 0 {
		t.Fatalf("a refused delete wrote %v %v", files, sidecars)
	}
}

func TestDeleteRefusesAnOfflineRepositoryAndMovesNothing(t *testing.T) {
	f := newFixture(t)
	online, offline := f.addRepository("online"), f.addRepository("offline")
	f.write(online, "a.jpg", "shared photo")
	f.write(offline, "a.jpg", "shared photo")
	f.index(online)
	f.index(offline)
	assetID := f.assetAt(online, "a.jpg")
	if _, err := f.database.SQL.ExecContext(f.ctx, `UPDATE repositories SET reachability = 'offline' WHERE repo_id = ?`, offline.RepoID); err != nil {
		t.Fatal(err)
	}

	_, err := f.trash.Delete(f.ctx, f.request(assetID))
	var rejection *Rejection
	if !errors.As(err, &rejection) || rejection.Reason != ReasonRepositoryOffline || rejection.RepositoryID != offline.RepoID {
		t.Fatalf("delete error = %v, want a repository_offline rejection", err)
	}
	if !f.exists(online, "a.jpg") || f.lifecycleState(assetID) != "active" {
		t.Fatal("a refused delete moved a file or changed the catalog")
	}
}

func TestRestoreNeverOverwritesATakenPath(t *testing.T) {
	f := newFixture(t)
	repository := f.addRepository("photos")
	f.write(repository, "a.jpg", "original photo")
	f.index(repository)
	assetID := f.assetAt(repository, "a.jpg")
	if _, err := f.trash.Delete(f.ctx, f.request(assetID)); err != nil {
		t.Fatal(err)
	}
	f.write(repository, "a.jpg", "a different photo, created later")

	result, err := f.trash.Restore(f.ctx, f.request(assetID))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Renamed) != 1 || result.Renamed[0].RestoredPath != "a (restored).jpg" {
		t.Fatalf("restore result = %+v, want a.jpg restored as a (restored).jpg", result)
	}
	current, err := os.ReadFile(f.path(repository, "a.jpg"))
	if err != nil || string(current) != "a different photo, created later" {
		t.Fatalf("the file at the taken path was replaced: %q (%v)", current, err)
	}
	restored, err := os.ReadFile(f.path(repository, "a (restored).jpg"))
	if err != nil || string(restored) != "original photo" {
		t.Fatalf("restored file = %q (%v)", restored, err)
	}
	if f.lifecycleState(assetID) != "active" {
		t.Fatal("restored Asset is not active")
	}
}

func TestDeleteInterruptedAfterAMoveIsRecovered(t *testing.T) {
	f := newFixture(t)
	repository := f.addRepository("photos")
	f.write(repository, "a.jpg", "photo a")
	f.write(repository, "b.jpg", "photo b")
	f.index(repository)
	a, b := f.assetAt(repository, "a.jpg"), f.assetAt(repository, "b.jpg")
	f.trash.afterMove = func(moved int) error {
		if moved == 1 {
			return errSimulatedCrash
		}
		return nil
	}
	if _, err := f.trash.Delete(f.ctx, f.request(a, b)); !errors.Is(err, errSimulatedCrash) {
		t.Fatalf("delete error = %v, want the simulated crash", err)
	}
	if f.runningOperations() != 1 {
		t.Fatal("the interrupted delete is not journaled as running")
	}
	f.trash.afterMove = nil

	if err := f.trash.Recover(f.ctx); err != nil {
		t.Fatal(err)
	}
	if f.runningOperations() != 0 {
		t.Fatal("recovery left the journal running")
	}
	// The first file had moved, so it is trashed; the second had not.
	movedAsset, stayedAsset, stayedPath := a, b, "b.jpg"
	if f.exists(repository, "a.jpg") {
		movedAsset, stayedAsset, stayedPath = b, a, "a.jpg"
	}
	if f.lifecycleState(movedAsset) != "trashed" || f.lifecycleState(stayedAsset) != "active" {
		t.Fatalf("after recovery: moved Asset %s, unmoved Asset %s", f.lifecycleState(movedAsset), f.lifecycleState(stayedAsset))
	}
	if !f.exists(repository, stayedPath) {
		t.Fatal("recovery lost the file that never moved")
	}
	files, sidecars := trashedFiles(t, repository.Path)
	if len(files) != 1 || len(sidecars) != 1 {
		t.Fatalf("trash after recovery holds %v and %v, want exactly the moved file", files, sidecars)
	}
}

func TestRestoreInterruptedAfterAMoveIsRecovered(t *testing.T) {
	f := newFixture(t)
	repository := f.addRepository("photos")
	f.write(repository, "a.jpg", "photo a")
	f.index(repository)
	assetID := f.assetAt(repository, "a.jpg")
	if _, err := f.trash.Delete(f.ctx, f.request(assetID)); err != nil {
		t.Fatal(err)
	}
	f.trash.afterMove = func(int) error { return errSimulatedCrash }
	if _, err := f.trash.Restore(f.ctx, f.request(assetID)); !errors.Is(err, errSimulatedCrash) {
		t.Fatalf("restore error = %v, want the simulated crash", err)
	}
	f.trash.afterMove = nil

	if err := f.trash.Recover(f.ctx); err != nil {
		t.Fatal(err)
	}
	if !f.exists(repository, "a.jpg") || f.lifecycleState(assetID) != "active" {
		t.Fatal("recovery did not complete the interrupted restore")
	}
	if files, sidecars := trashedFiles(t, repository.Path); len(files)+len(sidecars) != 0 {
		t.Fatalf("recovery left %v and %v in the trash", files, sidecars)
	}
	if f.runningOperations() != 0 {
		t.Fatal("recovery left the journal running")
	}
}

func TestDeletingATrashedAssetAgainChangesNothing(t *testing.T) {
	f := newFixture(t)
	repository := f.addRepository("photos")
	f.write(repository, "a.jpg", "photo a")
	f.index(repository)
	assetID := f.assetAt(repository, "a.jpg")
	if _, err := f.trash.Delete(f.ctx, f.request(assetID)); err != nil {
		t.Fatal(err)
	}
	result, err := f.trash.Delete(f.ctx, f.request(assetID))
	if err != nil || result.Files != 0 {
		t.Fatalf("second delete = %+v, %v; want nothing to do", result, err)
	}
	var rejection *Rejection
	if _, err := f.trash.Restore(f.ctx, f.request(uuid.New())); !errors.As(err, &rejection) || rejection.Reason != ReasonNotTrashed {
		t.Fatalf("restoring an Asset that is not trashed = %v, want not_trashed", err)
	}
}
