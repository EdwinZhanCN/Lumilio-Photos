package artifact

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"

	"server/config"
	"server/internal/catalog"
	"server/internal/catalog/repo"
	"server/internal/foundation/testutil"
)

// A transcode stays referenced while its Asset exists with any entry in the
// repository, so a trashed or missing video keeps it for a restore.
// newCleanerCatalog is a migrated catalog with one owner, one Storage
// Location, and the named repositories in it.
func newCleanerCatalog(t *testing.T, repositories ...uuid.UUID) *catalog.DB {
	t.Helper()
	ctx := context.Background()
	directory := t.TempDir()
	if err := os.Chmod(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	database, err := catalog.Open(ctx, config.DatabaseConfig{Path: filepath.Join(directory, "catalog.sqlite3")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close(context.Background()) })
	if err := database.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	location := uuid.New()
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := database.SQL.ExecContext(ctx, query, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`INSERT INTO users (user_id, username, password, created_at, updated_at, webauthn_user_handle) VALUES (1, 'owner', 'hash', 1, 1, x'01')`)
	exec(`INSERT INTO storage_locations (storage_location_id, name, path, kind, created_at, updated_at) VALUES (?, 'root', '/media', 'default', 1, 1)`, location)
	for _, repositoryID := range repositories {
		exec(`INSERT INTO repositories (repo_id, name, path, created_at, updated_at, default_owner_id, storage_location_id, path_case, path_normalization) VALUES (?, ?, ?, 1, 1, 1, ?, 'sensitive', 'none')`,
			repositoryID, repositoryID.String(), "/media/"+repositoryID.String(), location)
	}
	return database
}

func TestCleanerKeepsTranscodesOfAssetsThatStillExist(t *testing.T) {
	ctx := context.Background()
	repositoryID := uuid.New()
	database := newCleanerCatalog(t, repositoryID)
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

// A face crop stays referenced while a face item of this repository names
// it; another repository's crops are not this repository's to keep.
func TestCleanerKeepsFaceCropsTheirRepositoryNames(t *testing.T) {
	ctx := context.Background()
	here, elsewhere := uuid.New(), uuid.New()
	database := newCleanerCatalog(t, here, elsewhere)
	crops := map[uuid.UUID]string{}
	for _, repositoryID := range []uuid.UUID{here, elsewhere} {
		assetID := uuid.New()
		occurrence, err := testutil.InsertAssetOccurrence(ctx, database.SQL, testutil.AssetOccurrenceParams{
			AssetID: assetID, RepositoryID: repositoryID, OwnerID: 1, AssetType: "PHOTO", Filename: "face.jpg",
			MIMEType: "image/jpeg", FileSize: 1, EntryState: "present",
		})
		if err != nil {
			t.Fatal(err)
		}
		identity := Identity{SourceFence: occurrence.ContentID.String(), Stage: StageFaces, PipelineVersion: "1", Name: "0-face.webp"}
		crop, err := identity.Path()
		if err != nil {
			t.Fatal(err)
		}
		crops[repositoryID] = crop.String()
		if _, err := database.SQL.ExecContext(ctx, `INSERT INTO face_results (asset_id, model_id, created_at, updated_at) VALUES (?, 'model', 1, 1)`, assetID); err != nil {
			t.Fatal(err)
		}
		if _, err := database.SQL.ExecContext(ctx, `
			INSERT INTO face_items (asset_id, bounding_box, confidence, crop_fence, crop_version, crop_name, created_at, repository_id)
			VALUES (?, '{}', 0.9, ?, ?, ?, 1, ?)`, assetID, identity.SourceFence, identity.PipelineVersion, identity.Name, repositoryID); err != nil {
			t.Fatal(err)
		}
	}
	referenced, err := (&Cleaner{database: database}).references(ctx, repo.Repository{RepoID: here})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := referenced[crops[here]]; !ok {
		t.Error("the face crop of this repository is not referenced")
	}
	if _, ok := referenced[crops[elsewhere]]; ok {
		t.Error("another repository's face crop is referenced here")
	}
}
