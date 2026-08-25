package handler

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"server/config"
	"server/internal/db"
	"server/internal/db/dbtypes"
	"server/internal/db/dbtypes/status"
	"server/internal/db/repo"
	"server/internal/queue/jobs"

	"github.com/google/uuid"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riversqlite"
)

func TestFullReprocessStatusAndJobsShareTransaction(t *testing.T) {
	ctx := context.Background()
	catalogDir := t.TempDir()
	if err := os.Chmod(catalogDir, 0o700); err != nil {
		t.Fatal(err)
	}
	catalog, err := db.Open(ctx, config.DatabaseConfig{Path: filepath.Join(catalogDir, "catalog.sqlite3")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = catalog.Close(context.Background()) })
	if err := catalog.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	owner, err := catalog.Queries.CreateUser(ctx, repo.CreateUserParams{
		Username: "handler-retry-owner", Password: "unused", DisplayName: "Handler Retry Owner", Role: "admin",
		WebauthnUserHandle: []byte("handler-retry-owner"),
	})
	if err != nil {
		t.Fatal(err)
	}
	failed := status.AssetStatus{State: status.StateFailed, Tasks: map[string]status.TaskStatus{
		"metadata_asset": {State: status.TaskFailed},
	}}
	encoded, err := failed.ToJSON()
	if err != nil {
		t.Fatal(err)
	}
	assetID := uuid.New()
	now := dbtypes.NewTimestamp(time.Now().UTC())
	content, err := catalog.Queries.InsertContentObject(ctx, repo.InsertContentObjectParams{
		ContentID: uuid.New(), HashAlgorithm: "blake3-v1", FullHash: strings.Repeat("a", 64),
		FileSize: 1, CreatedAt: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	rating := int64(0)
	if _, err := catalog.Queries.CreateAsset(ctx, repo.CreateAssetParams{
		AssetID: assetID, OwnerID: &owner.UserID, ContentID: content.ContentID,
		Type: string(dbtypes.AssetTypePhoto), OriginalFilename: "retry.jpg", MimeType: "image/jpeg", TakenTime: now,
		SpecificMetadata: dbtypes.SpecificMetadata([]byte("{}")), Rating: &rating, Status: encoded,
	}); err != nil {
		t.Fatal(err)
	}
	queueClient, err := river.NewClient(riversqlite.New(catalog.SQL), &river.Config{})
	if err != nil {
		t.Fatal(err)
	}

	tx, err := catalog.SQL.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := catalog.Queries.WithTx(tx).ResetAssetStatusForRetry(ctx, assetID); err != nil {
		t.Fatal(err)
	}
	effectID := uuid.New()
	metadata := jobs.MetadataArgs{AssetID: assetID, ExpectedContentID: content.ContentID, EffectID: effectID}
	if err := insertReprocessJobTx(ctx, queueClient, tx, metadata, "metadata_asset"); err != nil {
		t.Fatal(err)
	}
	// Simulate the handler disappearing after a partial fan-out. Neither the
	// status reset nor the already-inserted River row may survive rollback.
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	assertHandlerCount(t, catalog.SQL, "SELECT count(*) FROM river_job", 0)
	asset, err := catalog.Queries.GetAssetByID(ctx, assetID)
	if err != nil {
		t.Fatal(err)
	}
	current, err := status.FromJSON(asset.Status)
	if err != nil {
		t.Fatal(err)
	}
	if current.State != status.StateFailed {
		t.Fatalf("status reset survived rolled-back jobs: %+v", current)
	}

	commit := func(reset bool) error {
		tx, err := catalog.SQL.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()
		if reset {
			if _, err := catalog.Queries.WithTx(tx).ResetAssetStatusForRetry(ctx, assetID); err != nil {
				return err
			}
		}
		if err := insertReprocessJobTx(ctx, queueClient, tx, metadata, "metadata_asset"); err != nil {
			return err
		}
		thumbnail := jobs.ThumbnailArgs{AssetID: assetID, ExpectedContentID: content.ContentID, EffectID: effectID}
		if err := insertReprocessJobTx(ctx, queueClient, tx, thumbnail, "thumbnail_asset"); err != nil {
			return err
		}
		return tx.Commit()
	}
	if err := commit(true); err != nil {
		t.Fatal(err)
	}
	if err := commit(false); err != nil {
		t.Fatal(err)
	}
	assertHandlerCount(t, catalog.SQL, "SELECT count(*) FROM river_job WHERE queue IN ('metadata_asset','thumbnail_asset')", 2)

	payload := jobs.AssetRetryPayload{AssetID: assetID.String(), RetryTasks: []string{"metadata_asset"}}
	var wait sync.WaitGroup
	errors := make(chan error, 8)
	for range 8 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			_, err := insertSelectiveRetryReceipt(ctx, queueClient, payload)
			errors <- err
		}()
	}
	wait.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatalf("overlapping selective receipt: %v", err)
		}
	}
	assertHandlerCount(t, catalog.SQL, "SELECT count(*) FROM river_job WHERE kind = 'retry_asset'", 1)
	if _, err := catalog.SQL.ExecContext(ctx, `
		UPDATE river_job SET state = 'completed', finalized_at = CURRENT_TIMESTAMP
		WHERE kind = 'retry_asset'
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := insertSelectiveRetryReceipt(ctx, queueClient, payload); err != nil {
		t.Fatal(err)
	}
	assertHandlerCount(t, catalog.SQL, "SELECT count(*) FROM river_job WHERE kind = 'retry_asset'", 2)
}

func assertHandlerCount(t *testing.T, database *sql.DB, query string, want int) {
	t.Helper()
	var got int
	if err := database.QueryRow(query).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("count for %q = %d, want %d", query, got, want)
	}
}
