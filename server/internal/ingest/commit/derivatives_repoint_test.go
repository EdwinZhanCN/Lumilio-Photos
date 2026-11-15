package commit

import (
	"context"
	"database/sql"
	"testing"

	"github.com/google/uuid"
)

// An in-place edit re-points an Asset to new content, and the derivatives
// stage runs again under the new source fence. Artifact identities are
// immutable per content, so the regenerated thumbnails live at new paths; the
// catalog row per (asset, size) must follow them instead of refusing the
// commit, which would leave the viewer on the old content's thumbnail and the
// macro retrying forever.
func TestDerivativesOfRepointedContentReplaceTheThumbnailRows(t *testing.T) {
	_, database := testWriter(t)
	for _, statement := range []string{
		`CREATE TABLE assets(asset_id TEXT PRIMARY KEY,type TEXT,status TEXT,updated_at INTEGER)`,
		`CREATE TABLE asset_pipeline_state(asset_id TEXT,source_content_id TEXT,stage TEXT,pipeline_version TEXT,desired_version INTEGER,applied_version INTEGER,terminal_error TEXT,updated_at INTEGER,PRIMARY KEY(asset_id,stage))`,
		`CREATE TABLE thumbnails(thumbnail_id INTEGER PRIMARY KEY AUTOINCREMENT,asset_id TEXT,size TEXT,storage_path TEXT,mime_type TEXT,created_at INTEGER,repository_id TEXT,UNIQUE(asset_id,size))`,
	} {
		if _, err := database.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	assetID, oldContent, newContent, repositoryID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	if _, err := database.Exec(`INSERT INTO assets VALUES(?, 'PHOTO', '{"state":"processing"}', 1)`, assetID.String()); err != nil {
		t.Fatal(err)
	}
	oldPath := ".lumilio/artifacts/" + oldContent.String() + "/derivatives/asset-v1/small.webp"
	newPath := ".lumilio/artifacts/" + newContent.String() + "/derivatives/asset-v1/small.webp"
	if _, err := database.Exec(`INSERT INTO thumbnails(asset_id,size,storage_path,mime_type,created_at,repository_id) VALUES(?, 'small', ?, 'image/webp', 1, ?)`,
		assetID.String(), oldPath, repositoryID.String()); err != nil {
		t.Fatal(err)
	}
	// The Asset now holds the new content: the derivatives stage was reset
	// under the new fence.
	if _, err := database.Exec(`INSERT INTO asset_pipeline_state VALUES(?, ?, 'derivatives', 'asset-v1', 2, 1, NULL, 1)`, assetID.String(), newContent.String()); err != nil {
		t.Fatal(err)
	}
	apply := func(fence uuid.UUID, version uint64, path string) (Outcome, error) {
		tx, err := database.BeginTx(context.Background(), nil)
		if err != nil {
			t.Fatal(err)
		}
		outcome, err := applyAssetDerivatives(context.Background(), tx, AssetDerivativesApplied{
			AssetID: assetID, SourceFence: fence, PipelineVersion: "asset-v1", DesiredVersion: version,
			Artifacts: []ThumbnailArtifact{{RepositoryID: repositoryID, Size: "small", StoragePath: path, MimeType: "image/webp"}},
		})
		if err != nil {
			_ = tx.Rollback()
			return outcome, err
		}
		return outcome, tx.Commit()
	}
	rowPath := func() (string, int) {
		var path string
		var count int
		if err := database.QueryRow(`SELECT storage_path, (SELECT count(*) FROM thumbnails) FROM thumbnails WHERE asset_id = ? AND size = 'small'`, assetID.String()).Scan(&path, &count); err != nil && err != sql.ErrNoRows {
			t.Fatal(err)
		}
		return path, count
	}

	// A late result for the old content is stale and changes nothing.
	if outcome, err := apply(oldContent, 1, oldPath); err != nil || outcome != OutcomeStale {
		t.Fatalf("stale result = %v, %v; want stale", outcome, err)
	}
	if path, count := rowPath(); path != oldPath || count != 1 {
		t.Fatalf("after the stale result: path %s, %d rows", path, count)
	}
	// The result for the current content replaces the row.
	if outcome, err := apply(newContent, 2, newPath); err != nil || outcome != OutcomeApplied {
		t.Fatalf("re-pointed result = %v, %v; want applied", outcome, err)
	}
	if path, count := rowPath(); path != newPath || count != 1 {
		t.Fatalf("after the re-pointed result: path %s, %d rows; want %s and one row", path, count, newPath)
	}
	// Redelivery of the same result is idempotent.
	if _, err := database.Exec(`UPDATE asset_pipeline_state SET applied_version = 2`); err != nil {
		t.Fatal(err)
	}
	if outcome, err := apply(newContent, 2, newPath); err != nil || outcome != OutcomeDuplicate {
		t.Fatalf("redelivered result = %v, %v; want duplicate", outcome, err)
	}
}
