package lifecycle

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"

	"server/config"
	"server/internal/db"
	"server/internal/db/catalogtx"
	"server/internal/db/repo"
	"server/internal/testutil"
)

func openPurgeTestDatabase(t *testing.T) (*db.DB, uuid.UUID, uuid.UUID) {
	t.Helper()
	directory := t.TempDir()
	if err := os.Chmod(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	database, err := db.Open(context.Background(), config.DatabaseConfig{Path: filepath.Join(directory, "catalog.sqlite3")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close(context.Background()) })
	if err := database.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	location, first, second := uuid.New(), uuid.New(), uuid.New()
	exec(t, database,
		`INSERT INTO users (user_id, username, password, created_at, updated_at, webauthn_user_handle)
		 VALUES (1, 'purge-owner', 'hash', 1, 1, x'01')`)
	exec(t, database,
		`INSERT INTO storage_locations (storage_location_id, name, path, kind, created_at, updated_at)
		 VALUES (?, 'root', '/media', 'default', 1, 1)`, location)
	exec(t, database,
		`INSERT INTO repositories (repo_id, name, path, created_at, updated_at, default_owner_id, storage_location_id)
		 VALUES (?, 'first', '/media/first', 1, 1, 1, ?), (?, 'second', '/media/second', 1, 1, 1, ?)`,
		first, location, second, location)
	return database, first, second
}

func exec(t *testing.T, database *db.DB, query string, args ...any) {
	t.Helper()
	if _, err := database.SQL.Exec(query, args...); err != nil {
		t.Fatal(err)
	}
}

func purge(t *testing.T, database *db.DB, entries ...uuid.UUID) PurgeResult {
	t.Helper()
	var result PurgeResult
	err := database.WithTx(context.Background(), catalogtx.OperationRepositoryRemove, func(tx *sql.Tx, _ *repo.Queries) error {
		var err error
		result, err = PurgeEntriesTx(context.Background(), tx, entries)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func count(t *testing.T, database *db.DB, query string, args ...any) int {
	t.Helper()
	var value int
	if err := database.SQL.QueryRow(query, args...).Scan(&value); err != nil {
		t.Fatal(err)
	}
	return value
}

func TestPurgeEntriesDeletesOnlyAssetsLeftWithoutAnEntry(t *testing.T) {
	database, first, second := openPurgeTestDatabase(t)
	ctx := context.Background()
	lonely, shared := uuid.New(), uuid.New()
	lonelyEntry, err := testutil.InsertAssetOccurrence(ctx, database.SQL, testutil.AssetOccurrenceParams{
		AssetID: lonely, RepositoryID: first, OwnerID: 1, Filename: "lonely.jpg", FileSize: 1, EntryState: "missing",
	})
	if err != nil {
		t.Fatal(err)
	}
	sharedEntry, err := testutil.InsertAssetOccurrence(ctx, database.SQL, testutil.AssetOccurrenceParams{
		AssetID: shared, RepositoryID: first, OwnerID: 1, Filename: "shared.jpg", FileSize: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	// The shared Asset has a second copy in another repository.
	exec(t, database, `
		INSERT INTO repository_entries (
			entry_id, repository_id, path, path_key, parent_key, kind, size, mtime_ns,
			stat_checked_ns, state, content_id, asset_id, revision, updated_at
		) VALUES (?, ?, 'shared.jpg', 'shared.jpg', '', 'file', 1, 1, 1, 'present', ?, ?, 1, 1)`,
		uuid.New(), second, sharedEntry.ContentID, shared)
	exec(t, database, `INSERT INTO albums (album_id, user_id, album_name, created_at, updated_at) VALUES (7, 1, 'Trip', 1, 1)`)
	exec(t, database, `INSERT INTO album_assets (album_id, asset_id, added_time) VALUES (7, ?, 1), (7, ?, 1)`, shared, lonely)

	result := purge(t, database, lonelyEntry.EntryID, sharedEntry.EntryID)
	if result.Entries != 2 || len(result.Assets) != 1 || result.Assets[0] != lonely {
		t.Fatalf("purge = %+v, want two entries and only the lonely Asset", result)
	}
	if count(t, database, `SELECT count(*) FROM assets WHERE asset_id = ?`, lonely) != 0 {
		t.Fatal("the Asset left without an entry still exists")
	}
	if count(t, database, `SELECT count(*) FROM assets WHERE asset_id = ? AND lifecycle_state = 'active'`, shared) != 1 {
		t.Fatal("the Asset with a copy elsewhere did not survive as active")
	}
	if count(t, database, `SELECT count(*) FROM album_assets WHERE album_id = 7`) != 1 {
		t.Fatal("album membership of the purged Asset was kept, or the survivor's was lost")
	}
}

func TestPurgeEntriesDissolvesMediaItemsWithNoMemberLeft(t *testing.T) {
	database, first, _ := openPurgeTestDatabase(t)
	ctx := context.Background()
	assetID, mediaItemID := uuid.New(), uuid.New()
	occurrence, err := testutil.InsertAssetOccurrence(ctx, database.SQL, testutil.AssetOccurrenceParams{
		AssetID: assetID, RepositoryID: first, OwnerID: 1, Filename: "gone.jpg", FileSize: 1, EntryState: "trashed",
	})
	if err != nil {
		t.Fatal(err)
	}
	exec(t, database, `
		INSERT INTO media_items (media_item_id, owner_id, repository_id, primary_asset_id, created_at, updated_at)
		VALUES (?, 1, ?, ?, 1, 1)`, mediaItemID, first, assetID)
	exec(t, database, `INSERT INTO media_item_assets (asset_id, media_item_id, created_at) VALUES (?, ?, 1)`, assetID, mediaItemID)
	purge(t, database, occurrence.EntryID)
	if count(t, database, `SELECT count(*) FROM media_items WHERE media_item_id = ?`, mediaItemID) != 0 {
		t.Fatal("a media item with no member left survived the purge")
	}
}
