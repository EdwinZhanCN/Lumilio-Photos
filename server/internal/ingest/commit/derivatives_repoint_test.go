package commit

import (
	"context"
	"database/sql"
	"testing"

	storageartifact "server/internal/storage/artifact"

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
		`CREATE TABLE thumbnails(thumbnail_id INTEGER PRIMARY KEY AUTOINCREMENT,asset_id TEXT,size TEXT,artifact_fence TEXT,artifact_version TEXT,artifact_name TEXT,mime_type TEXT,created_at INTEGER,repository_id TEXT,UNIQUE(asset_id,size))`,
	} {
		if _, err := database.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	assetID, oldContent, newContent, repositoryID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	if _, err := database.Exec(`INSERT INTO assets VALUES(?, 'PHOTO', '{"state":"processing"}', 1)`, assetID.String()); err != nil {
		t.Fatal(err)
	}
	small := func(content uuid.UUID) storageartifact.Identity {
		return storageartifact.Identity{SourceFence: content.String(), Stage: storageartifact.StageDerivatives, PipelineVersion: "asset-v1", Name: "small.webp"}
	}
	if _, err := database.Exec(`INSERT INTO thumbnails(asset_id,size,artifact_fence,artifact_version,artifact_name,mime_type,created_at,repository_id) VALUES(?, 'small', ?, 'asset-v1', 'small.webp', 'image/webp', 1, ?)`,
		assetID.String(), oldContent.String(), repositoryID.String()); err != nil {
		t.Fatal(err)
	}
	// The Asset now holds the new content: the derivatives stage was reset
	// under the new fence.
	if _, err := database.Exec(`INSERT INTO asset_pipeline_state VALUES(?, ?, 'derivatives', 'asset-v1', 2, 1, NULL, 1)`, assetID.String(), newContent.String()); err != nil {
		t.Fatal(err)
	}
	apply := func(fence uuid.UUID, version uint64) (Outcome, error) {
		tx, err := database.BeginTx(context.Background(), nil)
		if err != nil {
			t.Fatal(err)
		}
		outcome, err := applyAssetDerivatives(context.Background(), tx, AssetDerivativesApplied{
			AssetID: assetID, SourceFence: fence, PipelineVersion: "asset-v1", DesiredVersion: version,
			Artifacts: []ThumbnailArtifact{{RepositoryID: repositoryID, Size: "small", Artifact: small(fence), MimeType: "image/webp"}},
		})
		if err != nil {
			_ = tx.Rollback()
			return outcome, err
		}
		return outcome, tx.Commit()
	}
	rowFence := func() (string, int) {
		var fence string
		var count int
		if err := database.QueryRow(`SELECT artifact_fence, (SELECT count(*) FROM thumbnails) FROM thumbnails WHERE asset_id = ? AND size = 'small'`, assetID.String()).Scan(&fence, &count); err != nil && err != sql.ErrNoRows {
			t.Fatal(err)
		}
		return fence, count
	}

	// A late result for the old content is stale and changes nothing.
	if outcome, err := apply(oldContent, 1); err != nil || outcome != OutcomeStale {
		t.Fatalf("stale result = %v, %v; want stale", outcome, err)
	}
	if fence, count := rowFence(); fence != oldContent.String() || count != 1 {
		t.Fatalf("after the stale result: fence %s, %d rows", fence, count)
	}
	// The result for the current content replaces the row.
	if outcome, err := apply(newContent, 2); err != nil || outcome != OutcomeApplied {
		t.Fatalf("re-pointed result = %v, %v; want applied", outcome, err)
	}
	if fence, count := rowFence(); fence != newContent.String() || count != 1 {
		t.Fatalf("after the re-pointed result: fence %s, %d rows; want %s and one row", fence, count, newContent)
	}
	// Redelivery of the same result is idempotent.
	if _, err := database.Exec(`UPDATE asset_pipeline_state SET applied_version = 2`); err != nil {
		t.Fatal(err)
	}
	if outcome, err := apply(newContent, 2); err != nil || outcome != OutcomeDuplicate {
		t.Fatalf("redelivered result = %v, %v; want duplicate", outcome, err)
	}
}
