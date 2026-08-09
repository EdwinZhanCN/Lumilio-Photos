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
	"server/internal/logging"
	"server/internal/queue/jobs"
	"server/internal/storage"
	"server/internal/storage/repocfg"
	"server/internal/storage/rootcfg"

	"github.com/google/uuid"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riversqlite"
	"go.uber.org/zap"
)

type metadataNoopWorker struct {
	river.WorkerDefaults[jobs.MetadataArgs]
}

type recordingCapacityGuard struct {
	repositoryID  string
	expectedBytes uint64
}

func (g *recordingCapacityGuard) CheckRepositoryWriteCapacity(_ context.Context, repositoryID string, expectedBytes uint64) (storage.CapacityDecision, error) {
	g.repositoryID = repositoryID
	g.expectedBytes = expectedBytes
	return storage.CapacityDecision{Allowed: true}, nil
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

func newTestPipelineClient(t *testing.T, catalog *db.DB) *river.Client[*sql.Tx] {
	t.Helper()
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
	return client
}

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

	client := newTestPipelineClient(t, catalog)
	materializer := &SourceMaterializer{database: catalog, queries: catalog.Queries, queueClient: client}
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
		committed, createErr = createAssetWithMediaItem(ctx, queries, create("commit"), repo.StackRelationJpegOriginal)
		if createErr != nil {
			return createErr
		}
		return materializer.enqueuePipelineTx(ctx, tx, committed, dbtypes.AssetTypePhoto, "obs-test")
	}); err != nil {
		t.Fatalf("commit asset/media/jobs: %v", err)
	}
	assertCatalogCount(t, catalog.SQL, "SELECT COUNT(*) FROM assets WHERE content_hash = 'commit'", 1)
	assertCatalogCount(t, catalog.SQL, "SELECT COUNT(*) FROM media_items WHERE primary_asset_id = ?", 1, committed.AssetID)
	assertCatalogCount(t, catalog.SQL, "SELECT COUNT(*) FROM media_item_assets WHERE asset_id = ?", 1, committed.AssetID)
	assertCatalogCount(t, catalog.SQL, "SELECT COUNT(*) FROM river_job WHERE queue IN ('metadata_asset', 'thumbnail_asset')", 2)

	err = catalog.WithTx(ctx, func(tx *sql.Tx, queries *repo.Queries) error {
		asset, createErr := createAssetWithMediaItem(ctx, queries, create("rollback"), repo.StackRelationJpegOriginal)
		if createErr != nil {
			return createErr
		}
		// Metadata is inserted first; the unsupported type then forces the whole
		// asset/media/River unit to roll back.
		return materializer.enqueuePipelineTx(ctx, tx, asset, dbtypes.AssetType("UNSUPPORTED"), "obs-test")
	})
	if err == nil {
		t.Fatal("rollback transaction unexpectedly succeeded")
	}
	assertCatalogCount(t, catalog.SQL, "SELECT COUNT(*) FROM assets WHERE content_hash = 'rollback'", 0)
	assertCatalogCount(t, catalog.SQL, "SELECT COUNT(*) FROM media_items", 1)
	assertCatalogCount(t, catalog.SQL, "SELECT COUNT(*) FROM media_item_assets", 1)
	assertCatalogCount(t, catalog.SQL, "SELECT COUNT(*) FROM river_job", 2)
}

func TestStagedUploadBindsCommittedFileIndexBeforeReturning(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	catalogDir := t.TempDir()
	if err := os.Chmod(catalogDir, 0o700); err != nil {
		t.Fatal(err)
	}
	catalog, err := db.Open(ctx, config.DatabaseConfig{Path: filepath.Join(catalogDir, "library.sqlite3")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = catalog.Close(context.Background()) })
	if err := catalog.Migrate(ctx); err != nil {
		t.Fatal(err)
	}

	repositoryID := uuid.New()
	repositoryPath := t.TempDir()
	repositoryConfig := repocfg.NewRepositoryConfig("upload index")
	repositoryConfig.ID = repositoryID.String()
	if err := repositoryConfig.SaveConfigToFile(repositoryPath); err != nil {
		t.Fatal(err)
	}
	now := dbtypes.NewTimestamp(time.Now().UTC())
	rootID := uuid.New()
	rootConfig := rootcfg.New("upload root")
	rootConfig.ID = rootID.String()
	if err := rootConfig.Save(filepath.Dir(repositoryPath)); err != nil {
		t.Fatal(err)
	}
	if _, err := catalog.Queries.UpsertRepositoryRoot(ctx, repo.UpsertRepositoryRootParams{
		RootID: rootID, Name: "upload root", Path: filepath.Dir(repositoryPath),
		Kind: dbtypes.RepositoryRootKindExternal, Status: dbtypes.RepositoryRootStatusActive,
		CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	repository, err := catalog.Queries.CreateRepository(ctx, repo.CreateRepositoryParams{
		RepoID: repositoryID, Name: "upload index", Path: repositoryPath, Config: *repositoryConfig,
		Role: dbtypes.RepoRoleRegular, Reachability: dbtypes.RepositoryReachabilityActive, Activity: dbtypes.RepositoryActivityIdle,
		CreatedAt: now, UpdatedAt: now, RootID: rootID,
	})
	if err != nil {
		t.Fatal(err)
	}

	files := storage.NewRepositoryFSFactory(nil, catalog.Queries)
	staging := storage.NewStagingManager(files)
	staged, writer, err := staging.CreateStagingFile(repository, "upload.jpg")
	if err != nil {
		t.Fatal(err)
	}
	content := []byte("uploaded original bytes")
	if _, err := writer.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := writer.Sync(); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	materializer := NewSourceMaterializer(
		catalog,
		staging,
		newTestPipelineClient(t, catalog),
		zap.NewNop(),
		logging.NewRepositoryAuditProvider(zap.NewNop(), false),
		files,
	)
	guard := &recordingCapacityGuard{}
	materializer.SetCapacityGuard(guard)
	asset, err := materializer.MaterializeStaged(ctx, IngestSource{
		RepositoryID: repositoryID, Kind: IngestSourceUpload, StagingPath: staged.PrivatePath,
		OriginalFilename: "upload.jpg", Timestamp: staged.CreatedAt, ContentType: "image/jpeg",
	})
	if err != nil {
		t.Fatal(err)
	}
	if guard.repositoryID != repositoryID.String() || guard.expectedBytes != uint64(len(content)) {
		t.Fatalf("capacity preflight = %s/%d, want %s/%d", guard.repositoryID, guard.expectedBytes, repositoryID, len(content))
	}
	if asset.StoragePath == nil {
		t.Fatal("materialized asset has no repository-relative path")
	}
	indexed, err := catalog.Queries.GetRepositoryFileIndexEntry(ctx, repo.GetRepositoryFileIndexEntryParams{
		RepositoryID: repositoryID, StoragePath: *asset.StoragePath,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !indexed.AssetID.Valid || indexed.AssetID.UUID != asset.AssetID || indexed.State != "present" {
		t.Fatalf("upload index binding = %+v, want asset %s present", indexed, asset.AssetID)
	}
	if indexed.ContentHash == nil || *indexed.ContentHash != asset.ContentHash {
		t.Fatalf("upload index hash = %v, want %s", indexed.ContentHash, asset.ContentHash)
	}
	retryHash := asset.ContentHash
	retried, err := materializer.MaterializeStaged(ctx, IngestSource{
		RepositoryID: repositoryID, Kind: IngestSourceUpload, StagingPath: staged.PrivatePath,
		OriginalFilename: "upload.jpg", Timestamp: staged.CreatedAt, ContentType: "image/jpeg", ContentHash: &retryHash,
	})
	if err != nil {
		t.Fatalf("retry committed upload: %v", err)
	}
	if retried.AssetID != asset.AssetID {
		t.Fatalf("retry asset = %s, want %s", retried.AssetID, asset.AssetID)
	}
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
