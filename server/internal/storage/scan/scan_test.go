package scan

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"

	"server/internal/db/dbtypes"
)

func TestFirstScanIndexesTreeAndBindsAssets(t *testing.T) {
	f := newFixture(t, 0)
	f.write(f.primary, "a.jpg", "alpha")
	f.write(f.primary, "trip/b.jpg", "bravo")
	f.write(f.primary, "trip/day/c.jpg", "charlie")
	f.write(f.primary, "notes.txt", "not media")
	scan := f.scan(f.primary)
	if scan.Status != StatusCompleted || scan.NewEntries != 5 || scan.Errors != 0 {
		t.Fatalf("scan = %+v, want completed with 3 files and 2 directories", scan)
	}
	if bound := f.hash(f.primary); bound != 3 {
		t.Fatalf("bound %d files, want 3", bound)
	}
	for _, relative := range []string{"a.jpg", "trip/b.jpg", "trip/day/c.jpg"} {
		row := f.mustEntry(f.primary, relative, StatePresent)
		if got, want := f.assetContentHash(row.AssetID.UUID), fileHash(t, f.path(f.primary, relative)); got != want {
			t.Fatalf("%s bound to content %s, want %s", relative, got, want)
		}
	}
	f.mustEntry(f.primary, "trip", StatePresent)
	if _, ok := f.entry(f.primary, "notes.txt"); ok {
		t.Fatal("a file with an unsupported extension was indexed")
	}
}

func TestNoChangeRescanHashesAndWritesNothing(t *testing.T) {
	f := newFixture(t, 0)
	for index := 0; index < 40; index++ {
		f.write(f.primary, "d"+strconv.Itoa(index%5)+"/f"+strconv.Itoa(index)+".jpg", "content-"+strconv.Itoa(index))
	}
	f.scan(f.primary)
	f.hash(f.primary)
	before := f.entries(f.primary)
	// The files are an hour old, so none is racily clean and a rescan is free.
	scan := f.scan(f.primary)
	if scan.Status != StatusCompleted || scan.NewEntries+scan.Changed+scan.Missing+scan.Restored != 0 {
		t.Fatalf("no-change rescan = %+v", scan)
	}
	if bound := f.hash(f.primary); bound != 0 {
		t.Fatalf("no-change rescan hashed %d files", bound)
	}
	after := f.entries(f.primary)
	if len(before) != len(after) {
		t.Fatalf("entries %d -> %d", len(before), len(after))
	}
	for index := range before {
		if before[index].EntryID != after[index].EntryID || before[index].Revision != after[index].Revision {
			t.Fatalf("no-change rescan wrote entry %s: %+v -> %+v", before[index].Path, before[index], after[index])
		}
	}
}

func TestOfflineRepositoryEndsOfflineWithoutEntryChanges(t *testing.T) {
	cases := map[string]func(f *fixture){
		"root missing": func(f *fixture) {
			if err := os.Rename(f.repos[f.primary.RepoID], f.repos[f.primary.RepoID]+"-unmounted"); err != nil {
				t.Fatal(err)
			}
		},
		"marker missing": func(f *fixture) {
			if err := os.Remove(filepath.Join(f.repos[f.primary.RepoID], ".lumiliorepo")); err != nil {
				t.Fatal(err)
			}
		},
	}
	for name, takeOffline := range cases {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t, 0)
			f.write(f.primary, "keep/a.jpg", "alpha")
			f.scan(f.primary)
			f.hash(f.primary)
			before := f.entries(f.primary)
			takeOffline(f)
			scan := f.scan(f.primary)
			if scan.Status != StatusOffline {
				t.Fatalf("scan of an offline repository = %+v, want offline", scan)
			}
			after := f.entries(f.primary)
			if len(after) != len(before) {
				t.Fatalf("entries changed while offline: %+v -> %+v", before, after)
			}
			for index := range before {
				if before[index].Revision != after[index].Revision || after[index].State != before[index].State {
					t.Fatalf("entry changed while offline: %+v -> %+v", before[index], after[index])
				}
			}
		})
	}
}

func TestMarkerRemovedBeforeMissingCommitCommitsNothing(t *testing.T) {
	f := newFixture(t, 0)
	f.write(f.primary, "a.jpg", "alpha")
	f.write(f.primary, "gone/b.jpg", "bravo")
	f.scan(f.primary)
	f.hash(f.primary)
	f.remove(f.primary, "a.jpg")
	f.remove(f.primary, "gone")
	testHookBeforeRemovalCommit = func() {
		_ = os.Remove(filepath.Join(f.repos[f.primary.RepoID], ".lumiliorepo"))
	}
	t.Cleanup(func() { testHookBeforeRemovalCommit = nil })
	scan := f.scan(f.primary)
	if scan.Status != StatusOffline || scan.Missing != 0 {
		t.Fatalf("scan = %+v, want offline with nothing missing", scan)
	}
	for _, row := range f.entries(f.primary) {
		if row.State == StateMissing {
			t.Fatalf("entry %s committed missing after the marker vanished", row.Path)
		}
	}
}

func TestUnreadableDirectoryKeepsItsChildrenPresent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits do not deny directory listing on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	f := newFixture(t, 0)
	f.write(f.primary, "locked/a.jpg", "alpha")
	f.scan(f.primary)
	f.hash(f.primary)
	locked := f.path(f.primary, "locked")
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })
	scan := f.scan(f.primary)
	if scan.Status != StatusCompleted || scan.Errors == 0 {
		t.Fatalf("scan = %+v, want completed with an error counted", scan)
	}
	f.mustEntry(f.primary, "locked/a.jpg", StatePresent)
}

func TestMovesKeepAssetIDsAndRemoveTheOldEntry(t *testing.T) {
	f := newFixture(t, 0)
	other := f.addRepository("other")
	f.write(f.primary, "a.jpg", "file move")
	f.write(f.primary, "album/b.jpg", "directory move")
	f.write(f.primary, "c.jpg", "repository move")
	f.scan(f.primary)
	f.hash(f.primary)
	fileAsset := f.mustEntry(f.primary, "a.jpg", StatePresent).AssetID
	directoryAsset := f.mustEntry(f.primary, "album/b.jpg", StatePresent).AssetID
	repositoryAsset := f.mustEntry(f.primary, "c.jpg", StatePresent).AssetID

	f.rename(f.primary, "a.jpg", f.primary, "sorted/a.jpg")
	f.rename(f.primary, "album", f.primary, "albums/2026")
	f.rename(f.primary, "c.jpg", other, "c.jpg")
	f.scan(f.primary)
	f.scan(other)
	f.hash(f.primary)
	f.hash(other)

	if got := f.mustEntry(f.primary, "sorted/a.jpg", StatePresent).AssetID; got != fileAsset {
		t.Fatalf("file move changed the Asset: %v -> %v", fileAsset, got)
	}
	if got := f.mustEntry(f.primary, "albums/2026/b.jpg", StatePresent).AssetID; got != directoryAsset {
		t.Fatalf("directory move changed the Asset: %v -> %v", directoryAsset, got)
	}
	if got := f.mustEntry(other, "c.jpg", StatePresent).AssetID; got != repositoryAsset {
		t.Fatalf("cross-repository move changed the Asset: %v -> %v", repositoryAsset, got)
	}
	for _, relative := range []string{"a.jpg", "album/b.jpg", "c.jpg"} {
		if row, ok := f.entry(f.primary, relative); ok {
			t.Fatalf("old entry %s survived the move: %+v", relative, row)
		}
	}
	if count := f.assetCount(); count != 3 {
		t.Fatalf("assets = %d, want 3", count)
	}
}

func TestDuplicateCopyIsOneAssetWithTwoEntries(t *testing.T) {
	f := newFixture(t, 0)
	f.write(f.primary, "a.jpg", "same bytes")
	f.write(f.primary, "copy/a.jpg", "same bytes")
	f.scan(f.primary)
	f.hash(f.primary)
	first := f.mustEntry(f.primary, "a.jpg", StatePresent)
	second := f.mustEntry(f.primary, "copy/a.jpg", StatePresent)
	if first.AssetID != second.AssetID || f.assetCount() != 1 {
		t.Fatalf("copies bound to %v and %v with %d assets, want one Asset", first.AssetID, second.AssetID, f.assetCount())
	}
}

func TestMissingFileReturnsWithoutANewAsset(t *testing.T) {
	f := newFixture(t, 0)
	f.write(f.primary, "a.jpg", "alpha")
	f.scan(f.primary)
	f.hash(f.primary)
	asset := f.mustEntry(f.primary, "a.jpg", StatePresent).AssetID
	aside := filepath.Join(t.TempDir(), "a.jpg")
	if err := os.Rename(f.path(f.primary, "a.jpg"), aside); err != nil {
		t.Fatal(err)
	}
	if scan := f.scan(f.primary); scan.Missing != 1 {
		t.Fatalf("scan after removal = %+v, want one missing", scan)
	}
	f.mustEntry(f.primary, "a.jpg", StateMissing)
	if err := os.Rename(aside, f.path(f.primary, "a.jpg")); err != nil {
		t.Fatal(err)
	}
	if scan := f.scan(f.primary); scan.Restored != 1 {
		t.Fatalf("scan after return = %+v, want one restored", scan)
	}
	f.hash(f.primary)
	if got := f.mustEntry(f.primary, "a.jpg", StatePresent).AssetID; got != asset || f.assetCount() != 1 {
		t.Fatalf("returned file bound to %v with %d assets, want %v alone", got, f.assetCount(), asset)
	}
}

func TestDeletedDirectoryMarksItsSubtreeMissing(t *testing.T) {
	f := newFixture(t, 0)
	for index := 0; index < 600; index++ {
		f.write(f.primary, filepath.ToSlash(filepath.Join("trip", "day", "f"+strconv.Itoa(index)+".jpg")), "trip-"+strconv.Itoa(index))
	}
	f.scan(f.primary)
	f.hash(f.primary)
	f.remove(f.primary, "trip")
	scan := f.scan(f.primary)
	if scan.Status != StatusCompleted || scan.Missing != 600 {
		t.Fatalf("scan = %+v, want 600 missing", scan)
	}
	for _, row := range f.entries(f.primary) {
		if row.Kind == KindFile && row.State != StateMissing {
			t.Fatalf("entry %s is %s after its directory was deleted", row.Path, row.State)
		}
		if row.Kind == KindDirectory {
			t.Fatalf("directory entry %s survived", row.Path)
		}
	}
}

func TestRacilyCleanEntryIsRehashedUntilItsCheckIsLater(t *testing.T) {
	f := newFixture(t, 0)
	target := f.path(f.primary, "a.jpg")
	if err := os.WriteFile(target, []byte("fresh"), 0o644); err != nil {
		t.Fatal(err)
	}
	// The file's mtime is now, so a check made now cannot prove a later write
	// within the same timestamp tick did not happen.
	f.scan(f.primary)
	f.hash(f.primary)
	f.scan(f.primary)
	if bound := f.hash(f.primary); bound != 1 {
		t.Fatalf("racily clean entry was rehashed %d times, want 1", bound)
	}
	f.clock.advance(time.Minute)
	f.scan(f.primary)
	f.hash(f.primary)
	f.scan(f.primary)
	if bound := f.hash(f.primary); bound != 0 {
		t.Fatalf("entry checked well after its mtime was rehashed %d times", bound)
	}
}

func TestSettleWindowDefersAndSchedulesAFollowUp(t *testing.T) {
	f := newFixture(t, 5*time.Second)
	f.write(f.primary, "old.jpg", "settled")
	target := f.path(f.primary, "inbox/new.jpg")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("just landed"), 0o644); err != nil {
		t.Fatal(err)
	}
	scan := f.scan(f.primary)
	if scan.Deferred != 1 {
		t.Fatalf("scan = %+v, want one deferred file", scan)
	}
	if _, ok := f.entry(f.primary, "inbox/new.jpg"); ok {
		t.Fatal("a file inside the settle window was indexed")
	}
	followUp, err := f.database.ReaderQueries.GetQueuedRepositoryScan(f.ctx, f.primary.RepoID)
	if err != nil {
		t.Fatalf("no follow-up scan was queued: %v", err)
	}
	if followUp.Trigger != TriggerSettle || followUp.ScopePath != "inbox" || !followUp.NotBefore.Valid {
		t.Fatalf("follow-up = %+v, want a delayed settle scan of inbox", followUp)
	}
	if result, err := f.scanner.RunTurn(f.ctx, f.primary.RepoID); err != nil || result.Status != StatusQueued {
		t.Fatalf("follow-up ran before its window: %+v, %v", result, err)
	}
	f.clock.advance(10 * time.Second)
	f.turnUntilDone(f.primary, followUp.ScanID)
	f.hash(f.primary)
	f.mustEntry(f.primary, "inbox/new.jpg", StatePresent)
}

func TestRequestsCoalesceIntoOneQueuedScan(t *testing.T) {
	f := newFixture(t, 0)
	first, coalesced, err := f.scanner.Request(f.ctx, f.primary.RepoID, TriggerWatcher, "a/b", "", time.Time{})
	if err != nil || coalesced {
		t.Fatalf("first request = %+v, %t, %v", first, coalesced, err)
	}
	second, coalesced, err := f.scanner.Request(f.ctx, f.primary.RepoID, TriggerWatcher, "a/c", "", time.Time{})
	if err != nil || !coalesced || second.ScanID != first.ScanID || second.ScopePath != "a" {
		t.Fatalf("second request = %+v, %t, %v; want joined with scope a", second, coalesced, err)
	}
	full, coalesced, err := f.scanner.Request(f.ctx, f.primary.RepoID, TriggerManual, "", "", time.Time{})
	if err != nil || !coalesced || full.ScanID != first.ScanID || full.ScopePath != "" {
		t.Fatalf("full request = %+v, %t, %v; want the queued scan widened to the repository", full, coalesced, err)
	}
}

// In-place carry-over (#223). Album membership is keyed by Asset, so keeping
// the Asset ID keeps the album.
func TestInPlaceRewriteRepointsTheAsset(t *testing.T) {
	f := newFixture(t, 0)
	f.write(f.primary, "a.jpg", "before edit")
	f.scan(f.primary)
	f.hash(f.primary)
	asset := f.mustEntry(f.primary, "a.jpg", StatePresent).AssetID.UUID
	album := f.addToNewAlbum(asset)
	f.write(f.primary, "a.jpg", "after an in-place edit")
	f.scan(f.primary)
	f.hash(f.primary)
	row := f.mustEntry(f.primary, "a.jpg", StatePresent)
	if row.AssetID.UUID != asset || f.assetCount() != 1 {
		t.Fatalf("rewrite bound %v with %d assets, want %v re-pointed", row.AssetID, f.assetCount(), asset)
	}
	if got, want := f.assetContentHash(asset), fileHash(t, f.path(f.primary, "a.jpg")); got != want {
		t.Fatalf("Asset content %s, want the new content %s", got, want)
	}
	f.requireAlbumMember(album, asset)
}

func TestInPlaceRewriteOfOneCopyForksTheAssetWithItsMetadata(t *testing.T) {
	f := newFixture(t, 0)
	f.write(f.primary, "a.jpg", "shared")
	f.write(f.primary, "copy/a.jpg", "shared")
	f.scan(f.primary)
	f.hash(f.primary)
	original := f.mustEntry(f.primary, "a.jpg", StatePresent).AssetID.UUID
	album := f.addToNewAlbum(original)
	if _, err := f.database.SQL.ExecContext(f.ctx, `UPDATE assets SET rating = 4, liked = 1 WHERE asset_id = ?`, original); err != nil {
		t.Fatal(err)
	}
	f.write(f.primary, "copy/a.jpg", "edited copy")
	f.scan(f.primary)
	f.hash(f.primary)
	forked := f.mustEntry(f.primary, "copy/a.jpg", StatePresent).AssetID.UUID
	if forked == original || f.mustEntry(f.primary, "a.jpg", StatePresent).AssetID.UUID != original {
		t.Fatalf("edited copy bound to %v, untouched copy must keep %v", forked, original)
	}
	var rating, liked int
	if err := f.database.ReaderSQL.QueryRowContext(f.ctx, `SELECT rating, liked FROM assets WHERE asset_id = ?`, forked).Scan(&rating, &liked); err != nil {
		t.Fatal(err)
	}
	if rating != 4 || liked != 1 {
		t.Fatalf("forked Asset rating=%d liked=%d, want the copied 4 and 1", rating, liked)
	}
	f.requireAlbumMember(album, forked)
	f.requireAlbumMember(album, original)
}

func TestInPlaceRewriteMatchingAnotherAssetBindsWithoutMerge(t *testing.T) {
	f := newFixture(t, 0)
	f.write(f.primary, "a.jpg", "first")
	f.write(f.primary, "b.jpg", "second")
	f.scan(f.primary)
	f.hash(f.primary)
	first := f.mustEntry(f.primary, "a.jpg", StatePresent).AssetID.UUID
	second := f.mustEntry(f.primary, "b.jpg", StatePresent).AssetID.UUID
	f.write(f.primary, "a.jpg", "second")
	f.scan(f.primary)
	f.hash(f.primary)
	if got := f.mustEntry(f.primary, "a.jpg", StatePresent).AssetID.UUID; got != second {
		t.Fatalf("a.jpg bound to %v, want the existing Asset %v", got, second)
	}
	kept := false
	for _, row := range f.entries(f.primary) {
		if row.Path == "a.jpg" && row.State == StateMissing && row.AssetID.UUID == first {
			kept = true
		}
	}
	if !kept || f.assetCount() != 2 {
		t.Fatalf("the old Asset %v must stay as a missing entry: %+v", first, f.entries(f.primary))
	}
}

func (f *fixture) addToNewAlbum(asset uuid.UUID) int64 {
	f.t.Helper()
	now := dbtypes.NewTimestamp(time.Now().UTC())
	var album int64
	if err := f.database.SQL.QueryRowContext(f.ctx, `
		INSERT INTO albums (user_id, album_name, created_at, updated_at) VALUES (?, 'carry-over', ?, ?)
		RETURNING album_id`, f.owner, now, now).Scan(&album); err != nil {
		f.t.Fatal(err)
	}
	if _, err := f.database.SQL.ExecContext(f.ctx, `
		INSERT INTO album_assets (album_id, asset_id, position, added_time) VALUES (?, ?, 0, ?)`,
		album, asset, now); err != nil {
		f.t.Fatal(err)
	}
	return album
}

func (f *fixture) requireAlbumMember(album int64, asset uuid.UUID) {
	f.t.Helper()
	var count int
	if err := f.database.ReaderSQL.QueryRowContext(f.ctx, `
		SELECT count(*) FROM album_assets WHERE album_id = ? AND asset_id = ?`, album, asset).Scan(&count); err != nil {
		f.t.Fatal(err)
	}
	if count != 1 {
		f.t.Fatalf("Asset %v is not in album %d", asset, album)
	}
}
