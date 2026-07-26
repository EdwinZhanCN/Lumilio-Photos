package sourcing

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"server/config"
	"server/internal/db"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/queue/jobs"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riversqlite"
)

type metadataNoopWorker struct {
	river.WorkerDefaults[jobs.MetadataArgs]
}

func (*metadataNoopWorker) Work(context.Context, *river.Job[jobs.MetadataArgs]) error { return nil }

type thumbnailNoopWorker struct {
	river.WorkerDefaults[jobs.ThumbnailArgs]
}

func (*thumbnailNoopWorker) Work(context.Context, *river.Job[jobs.ThumbnailArgs]) error { return nil }

type transcodeNoopWorker struct {
	river.WorkerDefaults[jobs.TranscodeArgs]
}

func (*transcodeNoopWorker) Work(context.Context, *river.Job[jobs.TranscodeArgs]) error { return nil }

func TestAssetMediaAndPipelineCommitOrRollbackTogether(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	catalogDir := t.TempDir()
	if err := os.Chmod(catalogDir, 0o700); err != nil {
		t.Fatalf("secure test catalog directory: %v", err)
	}
	catalog, err := db.Open(ctx, config.DatabaseConfig{Path: filepath.Join(catalogDir, "library.sqlite3")})
	if err != nil {
		t.Fatalf("open test catalog: %v", err)
	}
	defer func() {
		if err := catalog.Close(context.Background()); err != nil {
			t.Errorf("close test catalog: %v", err)
		}
	}()
	if err := catalog.Migrate(ctx); err != nil {
		t.Fatalf("migrate test catalog: %v", err)
	}

	workers := river.NewWorkers()
	river.AddWorker(workers, &metadataNoopWorker{})
	river.AddWorker(workers, &thumbnailNoopWorker{})
	river.AddWorker(workers, &transcodeNoopWorker{})
	client, err := river.NewClient(riversqlite.New(catalog.SQL), &river.Config{
		Logger:  slog.Default(),
		Workers: workers,
		Queues: map[string]river.QueueConfig{
			"metadata_asset":  {MaxWorkers: 1},
			"thumbnail_asset": {MaxWorkers: 1},
			"transcode_asset": {MaxWorkers: 1},
		},
	})
	if err != nil {
		t.Fatalf("create River client: %v", err)
	}
	materializer := &SourceMaterializer{database: catalog, queries: catalog.Queries, queueClient: client}
	repository := repo.Repository{Path: t.TempDir()}

	create := func(contentHash string) repo.CreateAssetParams {
		status, err := buildTrackedProcessingStatus(dbtypes.AssetTypePhoto, queuedStatusMessage)
		if err != nil {
			t.Fatalf("build status: %v", err)
		}
		return repo.CreateAssetParams{
			Type:             string(dbtypes.AssetTypePhoto),
			OriginalFilename: contentHash + ".jpg",
			MimeType:         "image/jpeg",
			FileSize:         10,
			ContentHash:      contentHash,
			TakenTime:        dbtypes.NewTimestamp(time.Now()),
			Status:           status,
		}
	}

	var committed *repo.Asset
	if err := catalog.WithTx(ctx, func(tx *sql.Tx, queries *repo.Queries) error {
		var createErr error
		committed, createErr = createAssetWithMediaItem(ctx, queries, create("commit"))
		if createErr != nil {
			return createErr
		}
		return materializer.enqueuePipelineTx(ctx, tx, repository, committed, "inbox/commit.jpg", dbtypes.AssetTypePhoto)
	}); err != nil {
		t.Fatalf("commit asset/media/jobs: %v", err)
	}
	assertCatalogCount(t, catalog.SQL, "SELECT COUNT(*) FROM assets WHERE content_hash = 'commit'", 1)
	assertCatalogCount(t, catalog.SQL, "SELECT COUNT(*) FROM media_items WHERE primary_asset_id = ?", 1, committed.AssetID)
	assertCatalogCount(t, catalog.SQL, "SELECT COUNT(*) FROM media_item_assets WHERE asset_id = ?", 1, committed.AssetID)
	assertCatalogCount(t, catalog.SQL, "SELECT COUNT(*) FROM river_job WHERE queue IN ('metadata_asset', 'thumbnail_asset')", 2)

	err = catalog.WithTx(ctx, func(tx *sql.Tx, queries *repo.Queries) error {
		asset, createErr := createAssetWithMediaItem(ctx, queries, create("rollback"))
		if createErr != nil {
			return createErr
		}
		// Metadata is inserted first; the unsupported type then forces the whole
		// asset/media/River unit to roll back.
		return materializer.enqueuePipelineTx(ctx, tx, repository, asset, "inbox/rollback.bin", dbtypes.AssetType("UNSUPPORTED"))
	})
	if err == nil {
		t.Fatal("rollback transaction unexpectedly succeeded")
	}
	assertCatalogCount(t, catalog.SQL, "SELECT COUNT(*) FROM assets WHERE content_hash = 'rollback'", 0)
	assertCatalogCount(t, catalog.SQL, "SELECT COUNT(*) FROM media_items", 1)
	assertCatalogCount(t, catalog.SQL, "SELECT COUNT(*) FROM media_item_assets", 1)
	assertCatalogCount(t, catalog.SQL, "SELECT COUNT(*) FROM river_job", 2)
}

func assertCatalogCount(t *testing.T, database *sql.DB, query string, want int, args ...any) {
	t.Helper()
	var got int
	if err := database.QueryRow(query, args...).Scan(&got); err != nil {
		t.Fatalf("count query failed: %v", err)
	}
	if got != want {
		t.Fatalf("count = %d, want %d for %q", got, want, query)
	}
}
