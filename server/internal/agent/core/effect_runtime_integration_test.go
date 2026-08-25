package core

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"server/config"
	"server/internal/db"
	"server/internal/db/repo"

	"github.com/google/uuid"
)

func TestAgentBulkMembershipUsesSetBasedWriterStatements(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	database, err := db.Open(ctx, config.DatabaseConfig{Path: filepath.Join(dir, "agent-bulk.sqlite3")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close(context.Background()) })
	if err := database.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	user, err := database.Queries.CreateUser(ctx, repo.CreateUserParams{
		Username: "bulk-agent", Password: "unused", DisplayName: "Bulk Agent",
		Role: "admin", WebauthnUserHandle: []byte("bulk-agent-handle"),
	})
	if err != nil {
		t.Fatal(err)
	}
	album, err := database.Queries.CreateAlbum(ctx, repo.CreateAlbumParams{
		UserID: user.UserID, AlbumName: "Bulk", AlbumType: repo.AlbumTypeDefault,
	})
	if err != nil {
		t.Fatal(err)
	}

	assetIDs := make([]uuid.UUID, 300)
	for index := range assetIDs {
		assetIDs[index] = uuid.New()
	}
	encoded, err := json.Marshal(assetIDs)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.SQL.ExecContext(ctx, `
INSERT INTO content_objects (content_id, hash_algorithm, full_hash, file_size, created_at)
SELECT CAST(value AS TEXT), 'blake3-v1',
       replace(CAST(value AS TEXT), '-', '') || replace(CAST(value AS TEXT), '-', ''),
       1, 1
FROM json_each(?);
INSERT INTO assets (
  asset_id, owner_id, content_id, type, original_filename, mime_type, upload_time, updated_at
)
SELECT CAST(value AS TEXT), ?, CAST(value AS TEXT), 'PHOTO', CAST(value AS TEXT),
       'image/jpeg', 1, 1
FROM json_each(?)`, encoded, user.UserID, encoded); err != nil {
		t.Fatal(err)
	}

	tx, err := database.SQL.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := addAssetsToAlbumTx(ctx, tx, album.AlbumID, assetIDs); err != nil {
		_ = tx.Rollback()
		t.Fatal(err)
	}
	if err := applyTagsTx(ctx, tx, assetIDs, "add", []string{"reviewed", "favorite"}); err != nil {
		_ = tx.Rollback()
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}

	var albumCount, tagCount int
	if err := database.ReaderSQL.QueryRowContext(ctx, `SELECT count(*) FROM album_assets WHERE album_id = ?`, album.AlbumID).Scan(&albumCount); err != nil {
		t.Fatal(err)
	}
	if err := database.ReaderSQL.QueryRowContext(ctx, `SELECT count(*) FROM asset_tags`).Scan(&tagCount); err != nil {
		t.Fatal(err)
	}
	if albumCount != len(assetIDs) || tagCount != 2*len(assetIDs) {
		t.Fatalf("bulk membership counts album=%d tags=%d", albumCount, tagCount)
	}

	tx, err = database.SQL.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := applyTagsTx(ctx, tx, assetIDs, "remove", []string{"reviewed", "favorite"}); err != nil {
		_ = tx.Rollback()
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := database.ReaderSQL.QueryRowContext(ctx, `SELECT count(*) FROM asset_tags`).Scan(&tagCount); err != nil {
		t.Fatal(err)
	}
	if tagCount != 0 {
		t.Fatalf("bulk tag removal left %d rows", tagCount)
	}
}
