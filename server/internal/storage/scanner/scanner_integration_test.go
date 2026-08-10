package scanner

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"server/config"
	"server/internal/db"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/queue/jobs"
	"server/internal/storage"
	"server/internal/storage/repocfg"
	"server/internal/storage/rootcfg"
	hashutil "server/internal/utils/hash"

	"github.com/google/uuid"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riversqlite"
)

type scannerFixture struct {
	ctx        context.Context
	database   *db.DB
	repository repo.Repository
	scanner    *Scanner
	manager    *storage.DefaultRepositoryManager
}

func TestScanWorkerReturnsRetryableBusyDuringMaintenance(t *testing.T) {
	fixture := newScannerFixture(t, 0)
	now := dbtypes.NewTimestamp(time.Now().UTC())
	if _, err := fixture.database.Queries.BeginRepositoryMaintenance(fixture.ctx, repo.BeginRepositoryMaintenanceParams{
		RepoID: fixture.repository.RepoID, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	err := fixture.scanner.ProcessScanRepository(fixture.ctx, jobs.ScanRepositoryArgs{
		RepositoryID: fixture.repository.RepoID.String(), Mode: jobs.RepositoryScanModeManual,
	})
	if !errors.Is(err, storage.ErrRepositoryBusy) {
		t.Fatalf("scan maintenance error = %v, want ErrRepositoryBusy for River retry", err)
	}
	var runs int
	if err := fixture.database.SQL.QueryRowContext(fixture.ctx,
		`SELECT count(*) FROM repository_scan_runs WHERE repository_id = ?`, fixture.repository.RepoID,
	).Scan(&runs); err != nil {
		t.Fatal(err)
	}
	if runs != 0 {
		t.Fatalf("maintenance scan created %d run receipts", runs)
	}
}

func TestScanEnqueueHoldsRepositoryGateAgainstRemoval(t *testing.T) {
	fixture := newScannerFixture(t, 0)
	fixture.scanner.beforeScanInsert = func() {
		removeCtx, cancel := context.WithTimeout(fixture.ctx, 30*time.Millisecond)
		defer cancel()
		err := fixture.manager.RemoveRepository(removeCtx, fixture.repository.RepoID.String())
		if !errors.Is(err, storage.ErrRepositoryBusy) {
			t.Fatalf("remove during scan enqueue = %v, want ErrRepositoryBusy", err)
		}
	}
	result, err := fixture.scanner.EnqueueManualScan(fixture.ctx, fixture.repository.RepoID.String(), "test", false)
	if err != nil {
		t.Fatal(err)
	}
	if result.JobID == 0 {
		t.Fatal("scan enqueue returned no job")
	}
	if _, err := fixture.database.Queries.GetRepository(fixture.ctx, fixture.repository.RepoID); err != nil {
		t.Fatalf("repository was removed during enqueue: %v", err)
	}

	if _, err := fixture.database.Queries.BeginRepositoryMaintenance(fixture.ctx, repo.BeginRepositoryMaintenanceParams{
		RepoID: fixture.repository.RepoID, UpdatedAt: dbtypes.NewTimestamp(time.Now().UTC()),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.scanner.EnqueueManualScan(fixture.ctx, fixture.repository.RepoID.String(), "test", false); !errors.Is(err, storage.ErrRepositoryBusy) {
		t.Fatalf("enqueue during maintenance = %v, want ErrRepositoryBusy", err)
	}
}

func newScannerFixture(t *testing.T, settleSeconds int) *scannerFixture {
	t.Helper()
	ctx := context.Background()
	databaseDirectory := t.TempDir()
	if err := os.Chmod(databaseDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	database, err := db.Open(ctx, config.DatabaseConfig{Path: filepath.Join(databaseDirectory, "catalog.sqlite3")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close(context.Background()) })
	if err := database.Migrate(ctx); err != nil {
		t.Fatal(err)
	}

	repositoryID := uuid.New()
	repositoryPath := t.TempDir()
	repositoryConfig := repocfg.NewRepositoryConfig("scanner integration")
	repositoryConfig.ID = repositoryID.String()
	if err := repositoryConfig.SaveConfigToFile(repositoryPath); err != nil {
		t.Fatal(err)
	}
	now := dbtypes.NewTimestamp(time.Now().UTC())
	rootID := uuid.New()
	rootConfig := rootcfg.New("scanner root")
	rootConfig.ID = rootID.String()
	if err := rootConfig.Save(filepath.Dir(repositoryPath)); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Queries.UpsertRepositoryRoot(ctx, repo.UpsertRepositoryRootParams{
		RootID: rootID, Name: "scanner root", Path: filepath.Dir(repositoryPath),
		Kind: dbtypes.RepositoryRootKindExternal, Status: dbtypes.RepositoryRootStatusActive,
		CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	repository, err := database.Queries.CreateRepository(ctx, repo.CreateRepositoryParams{
		RepoID: repositoryID, Name: "scanner integration", Path: repositoryPath,
		Config: *repositoryConfig, Role: dbtypes.RepoRoleRegular,
		Reachability: dbtypes.RepositoryReachabilityActive, Activity: dbtypes.RepositoryActivityIdle,
		CreatedAt: now, UpdatedAt: now, RootID: rootID,
	})
	if err != nil {
		t.Fatal(err)
	}
	queueClient, err := river.NewClient(riversqlite.New(database.SQL), &river.Config{})
	if err != nil {
		t.Fatal(err)
	}
	files := storage.NewRepositoryFSFactory(nil, database.Queries)
	manager, err := storage.NewRepositoryManager(database.SQL, database.Queries, nil, nil, files)
	if err != nil {
		t.Fatal(err)
	}
	return &scannerFixture{
		ctx:        ctx,
		database:   database,
		repository: repository,
		manager:    manager,
		scanner: NewScanner(database, queueClient, files, manager, config.RepositoryScanConfig{
			SettleSeconds:      settleSeconds,
			MaxConcurrentRepos: 1,
		}, nil),
	}
}

func (f *scannerFixture) writeMedia(t *testing.T, relative string, content []byte, age time.Duration) {
	t.Helper()
	filename := filepath.Join(f.repository.Path, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(filename), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filename, content, 0o644); err != nil {
		t.Fatal(err)
	}
	when := time.Now().Add(-age)
	if err := os.Chtimes(filename, when, when); err != nil {
		t.Fatal(err)
	}
}

func (f *scannerFixture) seedAsset(t *testing.T, relative string, content []byte) repo.Asset {
	t.Helper()
	hash, err := hashutil.CalculateReaderHash(bytes.NewReader(content), hashutil.AlgorithmBLAKE3)
	if err != nil {
		t.Fatal(err)
	}
	rating := int64(0)
	asset, err := f.database.Queries.CreateAsset(f.ctx, repo.CreateAssetParams{
		AssetID: uuid.New(), Type: string(dbtypes.AssetTypePhoto), OriginalFilename: filepath.Base(relative),
		StoragePath: &relative, MimeType: "image/jpeg", FileSize: int64(len(content)), ContentHash: hash,
		TakenTime: dbtypes.NewTimestamp(time.Now().UTC()), SpecificMetadata: dbtypes.SpecificMetadata([]byte("{}")),
		Rating: &rating, RepositoryID: uuid.NullUUID{UUID: f.repository.RepoID, Valid: true}, Status: dbtypes.JSON([]byte("{}")),
	})
	if err != nil {
		t.Fatal(err)
	}
	return asset
}

func (f *scannerFixture) run(t *testing.T) (scanCounters, repo.RepositoryScanRun) {
	t.Helper()
	scanID := uuid.New()
	startedAt := time.Now().UTC()
	if _, err := f.database.Queries.CreateRepositoryScanRun(f.ctx, repo.CreateRepositoryScanRunParams{
		ScanID: scanID, RepositoryID: f.repository.RepoID, Mode: "manual",
		Status: ScanStatusRunning, StartedAt: dbtypes.NewTimestamp(startedAt),
	}); err != nil {
		t.Fatal(err)
	}
	counters, run, err := f.scanner.scanRepository(f.ctx, f.repository, scanID, startedAt, false)
	if err != nil {
		t.Fatal(err)
	}
	return counters, run
}

func TestScannerMoveAndTwoScanDeletion(t *testing.T) {
	fixture := newScannerFixture(t, 0)
	content := []byte("same original bytes")
	oldPath := "inbox/2026/08/a.jpg"
	newPath := "Trips/a.jpg"
	fixture.writeMedia(t, oldPath, content, time.Minute)
	asset := fixture.seedAsset(t, oldPath, content)
	fixture.run(t)

	if err := os.MkdirAll(filepath.Join(fixture.repository.Path, "Trips"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(fixture.repository.Path, filepath.FromSlash(oldPath)), filepath.Join(fixture.repository.Path, filepath.FromSlash(newPath))); err != nil {
		t.Fatal(err)
	}
	counters, _ := fixture.run(t)
	if counters.moved != 1 {
		t.Fatalf("moved count = %d, want 1", counters.moved)
	}
	moved, err := fixture.database.Queries.GetAssetByID(fixture.ctx, asset.AssetID)
	if err != nil {
		t.Fatal(err)
	}
	if moved.StoragePath == nil || *moved.StoragePath != newPath || moved.IsDeleted {
		t.Fatalf("asset identity was not preserved across move: %+v", moved)
	}

	if err := os.Remove(filepath.Join(fixture.repository.Path, filepath.FromSlash(newPath))); err != nil {
		t.Fatal(err)
	}
	first, _ := fixture.run(t)
	if first.deleted != 0 {
		t.Fatal("first authoritative absence deleted the asset")
	}
	second, _ := fixture.run(t)
	if second.deleted != 1 {
		t.Fatalf("second authoritative absence deleted %d assets, want 1", second.deleted)
	}
	deleted, err := fixture.database.Queries.GetAssetByIDAny(fixture.ctx, asset.AssetID)
	if err != nil {
		t.Fatal(err)
	}
	if !deleted.IsDeleted {
		t.Fatal("asset remained active after two authoritative absences")
	}
}

func TestScannerPreservesAssetAcrossCaseOnlyRename(t *testing.T) {
	fixture := newScannerFixture(t, 0)
	content := []byte("case rename bytes")
	oldPath := "Trips/Photo.jpg"
	newPath := "Trips/photo.jpg"
	fixture.writeMedia(t, oldPath, content, time.Minute)
	asset := fixture.seedAsset(t, oldPath, content)
	fixture.run(t)
	if err := os.Rename(
		filepath.Join(fixture.repository.Path, filepath.FromSlash(oldPath)),
		filepath.Join(fixture.repository.Path, filepath.FromSlash(newPath)),
	); err != nil {
		t.Skipf("filesystem does not support case-only rename: %v", err)
	}
	counters, _ := fixture.run(t)
	if counters.moved != 1 {
		t.Fatalf("case-only move count = %d, want 1", counters.moved)
	}
	moved, err := fixture.database.Queries.GetAssetByID(fixture.ctx, asset.AssetID)
	if err != nil {
		t.Fatal(err)
	}
	if moved.StoragePath == nil || *moved.StoragePath != newPath {
		t.Fatalf("case-only rename did not update asset path: %+v", moved)
	}
}

func TestScannerDefersAmbiguousMoveAndResolvesLater(t *testing.T) {
	fixture := newScannerFixture(t, 0)
	content := []byte("ambiguous bytes")
	oldPath := "inbox/old.jpg"
	fixture.writeMedia(t, oldPath, content, time.Minute)
	asset := fixture.seedAsset(t, oldPath, content)
	fixture.run(t)
	if err := os.Remove(filepath.Join(fixture.repository.Path, filepath.FromSlash(oldPath))); err != nil {
		t.Fatal(err)
	}
	fixture.writeMedia(t, "Trips/one.jpg", content, time.Minute)
	fixture.writeMedia(t, "Trips/two.jpg", content, time.Minute)

	counters, _ := fixture.run(t)
	if counters.ambiguous != 3 || counters.moved != 0 {
		t.Fatalf("ambiguous=%d moved=%d, want 3 and 0", counters.ambiguous, counters.moved)
	}
	unchanged, err := fixture.database.Queries.GetAssetByID(fixture.ctx, asset.AssetID)
	if err != nil {
		t.Fatal(err)
	}
	if unchanged.StoragePath == nil || *unchanged.StoragePath != oldPath || unchanged.IsDeleted {
		t.Fatalf("ambiguous scan changed asset identity: %+v", unchanged)
	}
	if err := os.Remove(filepath.Join(fixture.repository.Path, "Trips", "two.jpg")); err != nil {
		t.Fatal(err)
	}
	resolved, _ := fixture.run(t)
	if resolved.moved != 1 {
		t.Fatalf("resolved move count = %d, want 1", resolved.moved)
	}
}

func TestScannerPartialGenerationResetsAbsenceConfirmation(t *testing.T) {
	fixture := newScannerFixture(t, 3600)
	content := []byte("missing bytes")
	oldPath := "Album/old.jpg"
	fixture.writeMedia(t, oldPath, content, 2*time.Hour)
	asset := fixture.seedAsset(t, oldPath, content)
	fixture.run(t)
	if err := os.Remove(filepath.Join(fixture.repository.Path, filepath.FromSlash(oldPath))); err != nil {
		t.Fatal(err)
	}
	fixture.writeMedia(t, "Album/still-writing.jpg", []byte("partial"), 0)

	counters, _ := fixture.run(t)
	if counters.authoritative || counters.deleted != 0 {
		t.Fatalf("partial scan authoritative=%v deleted=%d", counters.authoritative, counters.deleted)
	}
	current, err := fixture.database.Queries.GetAssetByID(fixture.ctx, asset.AssetID)
	if err != nil {
		t.Fatal(err)
	}
	if current.IsDeleted {
		t.Fatal("partial scan deleted a missing asset")
	}
	indexed, err := fixture.database.Queries.GetRepositoryFileIndexEntry(fixture.ctx, repo.GetRepositoryFileIndexEntryParams{
		RepositoryID: fixture.repository.RepoID, StoragePath: oldPath,
	})
	if err != nil {
		t.Fatal(err)
	}
	if indexed.State != indexStateDeferred || indexed.MissingConfirmations != 0 {
		t.Fatalf("partial scan did not reset absence state: %+v", indexed)
	}
}

func TestScannerRebuildsSamePathBindingWithoutReplacingAsset(t *testing.T) {
	fixture := newScannerFixture(t, 0)
	content := []byte("rebuild bytes")
	storagePath := "Library/photo.jpg"
	fixture.writeMedia(t, storagePath, content, time.Minute)
	asset := fixture.seedAsset(t, storagePath, content)
	fixture.run(t)
	if err := fixture.database.Queries.ResetRepositoryFileIndex(fixture.ctx, fixture.repository.RepoID); err != nil {
		t.Fatal(err)
	}
	fixture.run(t)
	indexed, err := fixture.database.Queries.GetRepositoryFileIndexEntry(fixture.ctx, repo.GetRepositoryFileIndexEntryParams{
		RepositoryID: fixture.repository.RepoID, StoragePath: storagePath,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !indexed.AssetID.Valid || indexed.AssetID.UUID != asset.AssetID {
		t.Fatalf("rebuilt binding = %v, want %s", indexed.AssetID, asset.AssetID)
	}
	var count int
	if err := fixture.database.SQL.QueryRowContext(fixture.ctx, "SELECT COUNT(*) FROM assets WHERE repository_id = ?", fixture.repository.RepoID).Scan(&count); err != nil && err != sql.ErrNoRows {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("asset count after rebuild = %d, want 1", count)
	}
}

func TestScannerKeepsExistingOriginalWhenIdenticalCopyAppears(t *testing.T) {
	fixture := newScannerFixture(t, 0)
	content := []byte("copied bytes")
	originalPath := "Library/original.jpg"
	copyPath := "Exports/copy.jpg"
	fixture.writeMedia(t, originalPath, content, time.Minute)
	asset := fixture.seedAsset(t, originalPath, content)
	fixture.run(t)
	fixture.writeMedia(t, copyPath, content, time.Minute)

	counters, _ := fixture.run(t)
	if counters.moved != 0 || counters.discovered != 1 {
		t.Fatalf("copy scan moved=%d discovered=%d, want 0 and 1", counters.moved, counters.discovered)
	}
	current, err := fixture.database.Queries.GetAssetByID(fixture.ctx, asset.AssetID)
	if err != nil {
		t.Fatal(err)
	}
	if current.StoragePath == nil || *current.StoragePath != originalPath {
		t.Fatalf("copy stole original identity: %+v", current)
	}
	indexed, err := fixture.database.Queries.GetRepositoryFileIndexEntry(fixture.ctx, repo.GetRepositoryFileIndexEntryParams{
		RepositoryID: fixture.repository.RepoID, StoragePath: copyPath,
	})
	if err != nil {
		t.Fatal(err)
	}
	if indexed.AssetID.Valid || indexed.State != indexStatePresent {
		t.Fatalf("copy index should remain an unbound discovery candidate: %+v", indexed)
	}
}

func TestScannerDefersRecoverableIngestClaim(t *testing.T) {
	fixture := newScannerFixture(t, 0)
	content := []byte("prepared upload")
	targetPath := "inbox/prepared.jpg"
	fixture.writeMedia(t, targetPath, content, time.Minute)
	asset := fixture.seedAsset(t, targetPath, content)
	fixture.run(t)
	if err := os.Remove(filepath.Join(fixture.repository.Path, filepath.FromSlash(targetPath))); err != nil {
		t.Fatal(err)
	}
	status := `{"ingest":{"recoverable":true,"phase":"prepared","staging_path":".lumilio/staging/prepared.part"}}`
	if _, err := fixture.database.SQL.ExecContext(fixture.ctx, "UPDATE assets SET status = ? WHERE asset_id = ?", status, asset.AssetID); err != nil {
		t.Fatal(err)
	}

	counters, _ := fixture.run(t)
	if counters.deleted != 0 {
		t.Fatalf("recoverable ingest claim deleted %d assets", counters.deleted)
	}
	current, err := fixture.database.Queries.GetAssetByID(fixture.ctx, asset.AssetID)
	if err != nil {
		t.Fatal(err)
	}
	if current.IsDeleted || string(current.Status) != status {
		t.Fatalf("scanner altered recoverable ingest asset: %+v", current)
	}
}

func TestScannerReclaimsInterruptedGenerationWithoutReconciliation(t *testing.T) {
	fixture := newScannerFixture(t, 0)
	scanID := uuid.New()
	if _, err := fixture.database.Queries.CreateRepositoryScanRun(fixture.ctx, repo.CreateRepositoryScanRunParams{
		ScanID: scanID, RepositoryID: fixture.repository.RepoID, Mode: "periodic",
		Status: ScanStatusRunning, StartedAt: dbtypes.NewTimestamp(time.Now().UTC()),
	}); err != nil {
		t.Fatal(err)
	}
	if err := fixture.scanner.ReclaimInterruptedRuns(fixture.ctx); err != nil {
		t.Fatal(err)
	}
	run, err := fixture.database.Queries.GetRepositoryScanRun(fixture.ctx, scanID)
	if err != nil {
		t.Fatal(err)
	}
	if run.Status != ScanStatusFailed || run.Error == nil || run.FinishedAt.Time.IsZero() {
		t.Fatalf("reclaimed run = %+v", run)
	}
	indexRows, err := fixture.database.Queries.ListRepositoryFileIndex(fixture.ctx, fixture.repository.RepoID)
	if err != nil || len(indexRows) != 0 {
		t.Fatalf("reclaim mutated file index: %d/%v", len(indexRows), err)
	}
}
