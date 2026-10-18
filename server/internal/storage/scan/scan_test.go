package scan

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"

	"server/internal/db/dbtypes"
	"server/internal/db/repo"
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
	f.resetObservations()
	// The files are an hour old, so none is racily clean and a rescan is free.
	scan := f.scan(f.primary)
	if scan.Status != StatusCompleted || scan.NewEntries+scan.Changed+scan.Missing+scan.Restored != 0 {
		t.Fatalf("no-change rescan = %+v", scan)
	}
	if bound := f.hash(f.primary); bound != 0 {
		t.Fatalf("no-change rescan hashed %d files", bound)
	}
	// Counted outside the scanner: the hash library saw no computation and
	// SQLite recorded no row write to repository_entries (#222: "0 hashes
	// and 0 entry writes").
	if hashes := f.hashComputations(); hashes != 0 {
		t.Fatalf("no-change rescan computed %d content hashes", hashes)
	}
	if writes := f.entryWrites(); len(writes) != 0 {
		t.Fatalf("no-change rescan wrote %d entry rows: %+v", len(writes), writes)
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

// #222: "Moving a file or directory, within a repository and across
// repositories: Asset IDs and album membership are unchanged, and the old
// entry is auto-removed." Album membership is keyed by Asset, so it survives
// exactly when the Asset ID does; each move is checked for both.
func TestMovesKeepAssetIDsAndAlbumsAndRemoveTheOldEntry(t *testing.T) {
	f := newFixture(t, 0)
	other := f.addRepository("other")
	moves := []struct {
		name, from, to string
		into           repo.Repository
	}{
		{"file within the repository", "a.jpg", "sorted/a.jpg", f.primary},
		{"directory within the repository", "album/b.jpg", "albums/2026/b.jpg", f.primary},
		{"file across repositories", "c.jpg", "c.jpg", other},
		{"directory across repositories", "trip/day1/d.jpg", "trip/day1/d.jpg", other},
	}
	for _, move := range moves {
		f.write(f.primary, move.from, move.name)
	}
	f.scan(f.primary)
	f.hash(f.primary)
	assets := make(map[string]uuid.UUID, len(moves))
	albums := make(map[string]int64, len(moves))
	for _, move := range moves {
		assets[move.name] = f.mustEntry(f.primary, move.from, StatePresent).AssetID.UUID
		albums[move.name] = f.addToNewAlbum(assets[move.name])
	}

	f.rename(f.primary, "a.jpg", f.primary, "sorted/a.jpg")
	f.rename(f.primary, "album", f.primary, "albums/2026")
	f.rename(f.primary, "c.jpg", other, "c.jpg")
	f.rename(f.primary, "trip", other, "trip")
	f.scan(f.primary)
	f.scan(other)
	f.hash(f.primary)
	f.hash(other)

	for _, move := range moves {
		if got := f.mustEntry(move.into, move.to, StatePresent).AssetID.UUID; got != assets[move.name] {
			t.Fatalf("%s changed the Asset: %v -> %v", move.name, assets[move.name], got)
		}
		f.requireAlbumMember(albums[move.name], assets[move.name])
		if move.into.RepoID == f.primary.RepoID && move.from == move.to {
			continue
		}
		if row, ok := f.entry(f.primary, move.from); ok {
			t.Fatalf("%s: old entry %s survived the move: %+v", move.name, move.from, row)
		}
	}
	for _, directory := range []string{"album", "trip", "trip/day1"} {
		if row, ok := f.entry(f.primary, directory); ok {
			t.Fatalf("old directory entry %s survived the move: %+v", directory, row)
		}
	}
	if count := f.assetCount(); count != len(moves) {
		t.Fatalf("assets = %d, want %d", count, len(moves))
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

// userMetadata is the user-owned metadata of an Asset the in-place
// carry-over branches must copy (fork) or leave alone (bind without merge):
// rating and like, album membership, user tags, and the user-edited
// description with its edited flag. An AI tag is listed separately because
// a fork copies only the user's tags.
type userMetadata struct {
	Rating      int
	Liked       bool
	Albums      []int64
	UserTags    []string
	AITags      []string
	Description string
	Edited      bool
	Lifecycle   string
}

func (f *fixture) userMetadata(asset uuid.UUID) userMetadata {
	f.t.Helper()
	var m userMetadata
	var description, edited *string
	if err := f.database.ReaderSQL.QueryRowContext(f.ctx, `
		SELECT rating, liked, lifecycle_state,
		       json_extract(specific_metadata, '$.description'),
		       json_extract(specific_metadata, '$.description_edited')
		FROM assets WHERE asset_id = ?`, asset).Scan(&m.Rating, &m.Liked, &m.Lifecycle, &description, &edited); err != nil {
		f.t.Fatal(err)
	}
	if description != nil {
		m.Description = *description
	}
	m.Edited = edited != nil && (*edited == "1" || *edited == "true")
	collect := func(query string, out *[]int64) {
		rows, err := f.database.ReaderSQL.QueryContext(f.ctx, query, asset)
		if err != nil {
			f.t.Fatal(err)
		}
		defer rows.Close()
		for rows.Next() {
			var value int64
			if err := rows.Scan(&value); err != nil {
				f.t.Fatal(err)
			}
			*out = append(*out, value)
		}
	}
	collect(`SELECT album_id FROM album_assets WHERE asset_id = ? ORDER BY album_id`, &m.Albums)
	rows, err := f.database.ReaderSQL.QueryContext(f.ctx, `
		SELECT tag.tag_name, link.source FROM asset_tags link JOIN tags tag ON tag.tag_id = link.tag_id
		WHERE link.asset_id = ? ORDER BY tag.tag_name`, asset)
	if err != nil {
		f.t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var name, source string
		if err := rows.Scan(&name, &source); err != nil {
			f.t.Fatal(err)
		}
		if source == "user" {
			m.UserTags = append(m.UserTags, name)
		} else {
			m.AITags = append(m.AITags, name)
		}
	}
	return m
}

// attachUserMetadata gives an Asset a rating and like, membership in a new
// album, one user tag and one AI tag, and a user-edited description.
func (f *fixture) attachUserMetadata(asset uuid.UUID, rating int, tag, description string) int64 {
	f.t.Helper()
	album := f.addToNewAlbum(asset)
	if _, err := f.database.SQL.ExecContext(f.ctx, `
		UPDATE assets SET rating = ?, liked = 1,
		    specific_metadata = json_set(COALESCE(specific_metadata, '{}'), '$.description', ?, '$.description_edited', json('true'))
		WHERE asset_id = ?`, rating, description, asset); err != nil {
		f.t.Fatal(err)
	}
	for _, link := range []struct{ name, source string }{{tag, "user"}, {tag + "-ai", "ai"}} {
		if _, err := f.database.SQL.ExecContext(f.ctx, `INSERT OR IGNORE INTO tags (tag_name, is_ai_generated) VALUES (?, ?)`, link.name, link.source != "user"); err != nil {
			f.t.Fatal(err)
		}
		if _, err := f.database.SQL.ExecContext(f.ctx, `
			INSERT INTO asset_tags (asset_id, tag_id, confidence, source)
			VALUES (?, (SELECT tag_id FROM tags WHERE tag_name = ?), 1, ?)`, asset, link.name, link.source); err != nil {
			f.t.Fatal(err)
		}
	}
	return album
}

// In-place carry-over branch 2 (#223): editing one of two copies forks a new
// Asset for the edited file. The fork inherits the user's metadata in full
// (rating, like, albums, user tags, the edited description), not the AI
// tags; the untouched copy keeps the original Asset with everything it had.
func TestInPlaceRewriteOfOneCopyForksTheAssetWithItsMetadata(t *testing.T) {
	f := newFixture(t, 0)
	f.write(f.primary, "a.jpg", "shared")
	f.write(f.primary, "copy/a.jpg", "shared")
	f.scan(f.primary)
	f.hash(f.primary)
	original := f.mustEntry(f.primary, "a.jpg", StatePresent).AssetID.UUID
	album := f.attachUserMetadata(original, 4, "family", "edited by the user")
	before := f.userMetadata(original)
	want := userMetadata{Rating: 4, Liked: true, Albums: []int64{album}, UserTags: []string{"family"}, AITags: []string{"family-ai"},
		Description: "edited by the user", Edited: true, Lifecycle: "active"}
	if !reflect.DeepEqual(before, want) {
		t.Fatalf("fixture metadata = %+v, want %+v", before, want)
	}

	f.write(f.primary, "copy/a.jpg", "edited copy")
	f.scan(f.primary)
	f.hash(f.primary)
	forked := f.mustEntry(f.primary, "copy/a.jpg", StatePresent).AssetID.UUID
	if forked == original || f.mustEntry(f.primary, "a.jpg", StatePresent).AssetID.UUID != original {
		t.Fatalf("edited copy bound to %v, untouched copy must keep %v", forked, original)
	}
	if after := f.userMetadata(original); !reflect.DeepEqual(after, before) {
		t.Fatalf("original Asset metadata after the fork = %+v, want unchanged %+v", after, before)
	}
	wantFork := want
	wantFork.AITags = nil
	if got := f.userMetadata(forked); !reflect.DeepEqual(got, wantFork) {
		t.Fatalf("forked Asset metadata = %+v, want the user's metadata copied %+v", got, wantFork)
	}
	if got, want := f.assetContentHash(forked), fileHash(t, f.path(f.primary, "copy/a.jpg")); got != want {
		t.Fatalf("forked Asset content %s, want the edited content %s", got, want)
	}
}

// In-place carry-over branch 3 (#223): a rewrite whose new content already
// belongs to another Asset binds the file to that Asset without merging. The
// bound Asset's metadata is untouched; the old Asset keeps all of its own as
// a missing entry, so nothing the user attached to either is lost or mixed.
func TestInPlaceRewriteMatchingAnotherAssetBindsWithoutMerge(t *testing.T) {
	f := newFixture(t, 0)
	f.write(f.primary, "a.jpg", "first")
	f.write(f.primary, "b.jpg", "second")
	f.scan(f.primary)
	f.hash(f.primary)
	first := f.mustEntry(f.primary, "a.jpg", StatePresent).AssetID.UUID
	second := f.mustEntry(f.primary, "b.jpg", StatePresent).AssetID.UUID
	f.attachUserMetadata(first, 2, "old", "the first caption")
	f.attachUserMetadata(second, 5, "new", "the second caption")
	firstBefore, secondBefore := f.userMetadata(first), f.userMetadata(second)

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
	if got := f.userMetadata(second); !reflect.DeepEqual(got, secondBefore) {
		t.Fatalf("bound Asset metadata = %+v, want unchanged %+v (no merge)", got, secondBefore)
	}
	firstWant := firstBefore
	firstWant.Lifecycle = "missing"
	if got := f.userMetadata(first); !reflect.DeepEqual(got, firstWant) {
		t.Fatalf("old Asset metadata = %+v, want kept %+v", got, firstWant)
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

func TestScanRequestForARemovedRepositoryReportsItGone(t *testing.T) {
	f := newFixture(t, 0)
	if _, _, err := f.scanner.Request(f.ctx, uuid.New(), TriggerWatcher, "", "", time.Time{}); !errors.Is(err, ErrRepositoryGone) {
		t.Fatalf("request for an unknown repository = %v, want ErrRepositoryGone", err)
	}
}
