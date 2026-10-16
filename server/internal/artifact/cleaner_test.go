package artifact

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"

	"server/config"
	"server/internal/db"
	"server/internal/db/repo"
	"server/internal/testutil"
)

// A transcode stays referenced while its Asset exists with any entry in the
// repository, so a trashed or missing video keeps it for a restore.
func TestCleanerKeepsTranscodesOfAssetsThatStillExist(t *testing.T) {
	ctx := context.Background()
	directory := t.TempDir()
	if err := os.Chmod(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	database, err := db.Open(ctx, config.DatabaseConfig{Path: filepath.Join(directory, "catalog.sqlite3")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close(context.Background()) })
	if err := database.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	location, repositoryID := uuid.New(), uuid.New()
	for _, statement := range []struct {
		query string
		args  []any
	}{
		{`INSERT INTO users (user_id, username, password, created_at, updated_at, webauthn_user_handle) VALUES (1, 'owner', 'hash', 1, 1, x'01')`, nil},
		{`INSERT INTO storage_locations (storage_location_id, name, path, kind, created_at, updated_at) VALUES (?, 'root', '/media', 'default', 1, 1)`, []any{location}},
		{`INSERT INTO repositories (repo_id, name, path, created_at, updated_at, default_owner_id, storage_location_id) VALUES (?, 'videos', '/media/videos', 1, 1, 1, ?)`, []any{repositoryID, location}},
	} {
		if _, err := database.SQL.ExecContext(ctx, statement.query, statement.args...); err != nil {
			t.Fatal(err)
		}
	}
	names := map[string]string{}
	for state, name := range map[string]string{"present": "active.mp4", "missing": "missing.mp4", "trashed": "trashed.mp4"} {
		assetID := uuid.New()
		occurrence, err := testutil.InsertAssetOccurrence(ctx, database.SQL, testutil.AssetOccurrenceParams{
			AssetID: assetID, RepositoryID: repositoryID, OwnerID: 1, AssetType: "VIDEO", Filename: name,
			MIMEType: "video/mp4", FileSize: 1, EntryState: state,
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := database.SQL.ExecContext(ctx, `
			INSERT INTO asset_pipeline_state (asset_id, source_content_id, stage, pipeline_version, desired_version, applied_version, priority, updated_at)
			VALUES (?, ?, 'transcode', 'v1', 1, 1, 3, 1)`, assetID, occurrence.ContentID); err != nil {
			t.Fatal(err)
		}
		path, err := (Identity{SourceFence: occurrence.ContentID.String(), Stage: "transcode", PipelineVersion: "v1", Name: "web.mp4"}).Path()
		if err != nil {
			t.Fatal(err)
		}
		names[state] = path.String()
	}
	cleaner := &Cleaner{database: database}
	referenced, err := cleaner.references(ctx, repo.Repository{RepoID: repositoryID})
	if err != nil {
		t.Fatal(err)
	}
	for state, path := range names {
		if _, ok := referenced[path]; !ok {
			t.Errorf("the transcode of a %s video is not referenced", state)
		}
	}
}
