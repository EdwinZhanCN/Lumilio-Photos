package storage

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"

	"server/internal/db"
	"server/internal/db/dbtypes"
	"server/internal/storage/repocfg"
)

func sha256FileHex(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

// seedRelocationScanState gives a repository a running scan and one present
// entry whose change time and file ID were recorded on the old volume.
func seedRelocationScanState(t *testing.T, catalog *db.DB, repositoryID uuid.UUID) (entryID, runningScanID uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	now := dbtypes.NewTimestamp(time.Now().UTC())
	entryID, runningScanID = uuid.New(), uuid.New()
	if _, err := catalog.SQL.ExecContext(ctx, `
		INSERT INTO repository_entries (
			entry_id, repository_id, path, path_key, parent_key, kind, size, mtime_ns,
			ctime_ns, file_id, stat_checked_ns, state, revision, updated_at
		) VALUES (?, ?, 'tracked.jpg', 'tracked.jpg', '', 'file', 10, 1, 2, 'old-volume:7', 3, 'pending_hash', 1, ?)
	`, entryID, repositoryID, now); err != nil {
		t.Fatal(err)
	}
	if _, err := catalog.SQL.ExecContext(ctx, `
		INSERT INTO repository_scans (scan_id, repository_id, trigger, scope_path, status, created_at, started_at, updated_at)
		VALUES (?, ?, 'manual', '', 'walking', ?, ?, ?)
	`, runningScanID, repositoryID, now, now, now); err != nil {
		t.Fatal(err)
	}
	return entryID, runningScanID
}

func TestRelocateRepositorySucceedsWhenOriginalPathIsMissing(t *testing.T) {
	_, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()
	base := t.TempDir()
	initializeDefaultStorageForTest(t, manager, filepath.Join(base, "default"))

	firstPath := filepath.Join(base, "first-root")
	secondPath := filepath.Join(base, "second-root")
	for _, path := range []string{firstPath, secondPath} {
		if err := os.Mkdir(path, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	firstRoot, err := manager.AddStorageLocation(ctx, firstPath, "First")
	if err != nil {
		t.Fatal(err)
	}
	secondRoot, err := manager.AddStorageLocation(ctx, secondPath, "Second")
	if err != nil {
		t.Fatal(err)
	}
	created, err := manager.CreateRepository(ctx, CreateRepositorySpec{
		RequestID: "reconnect-missing-old", Actor: "test", Name: "Reconnect",
		DirectoryName: "reconnect", Role: dbtypes.RepoRoleRegular, StorageLocationID: firstRoot.StorageLocationID.String(),
	})
	if err != nil {
		t.Fatal(err)
	}
	originalPath := created.Repository.Path
	mediaPath := filepath.Join(originalPath, "original-bytes.jpg")
	if err := os.WriteFile(mediaPath, []byte("preserved original media"), 0o644); err != nil {
		t.Fatal(err)
	}
	beforeChecksum, err := sha256FileHex(mediaPath)
	if err != nil {
		t.Fatal(err)
	}

	newPath := filepath.Join(secondRoot.Path, "reconnect")
	if err := os.MkdirAll(newPath, 0o755); err != nil {
		t.Fatal(err)
	}
	config, err := repocfg.LoadConfigFromFile(originalPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := config.SaveConfigToFile(newPath); err != nil {
		t.Fatal(err)
	}
	if err := manager.dirManager.CreateStructure(newPath); err != nil {
		t.Fatal(err)
	}
	newMediaPath := filepath.Join(newPath, "original-bytes.jpg")
	if err := os.WriteFile(newMediaPath, []byte("preserved original media"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.RemoveAll(originalPath); err != nil {
		t.Fatal(err)
	}

	relocated, err := manager.RelocateRepository(ctx, created.Repository.RepoID.String(), newPath, LifecycleRequest{
		RequestID: "reconnect-missing-old-tx", Actor: "test", ConfirmationType: "update_location",
	})
	if err != nil {
		t.Fatal(err)
	}
	canonicalNewPath, err := CanonicalizeRepositoryPath(newPath)
	if err != nil {
		t.Fatal(err)
	}
	if relocated.StorageLocationID != secondRoot.StorageLocationID || relocated.Path != canonicalNewPath {
		t.Fatalf("relocated repository = root %s path %q", relocated.StorageLocationID, relocated.Path)
	}
	afterChecksum, err := sha256FileHex(filepath.Join(canonicalNewPath, "original-bytes.jpg"))
	if err != nil {
		t.Fatal(err)
	}
	if beforeChecksum != afterChecksum {
		t.Fatalf("relocate modified original bytes: before %s after %s", beforeChecksum, afterChecksum)
	}
	if _, err := os.Stat(originalPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("old path stat = %v, want not exist", err)
	}
}

func TestRelocateRepositoryClearsStaleActivityWhenOriginalPathIsMissing(t *testing.T) {
	_, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()
	base := t.TempDir()
	initializeDefaultStorageForTest(t, manager, filepath.Join(base, "default"))

	firstPath := filepath.Join(base, "stale-first")
	secondPath := filepath.Join(base, "stale-second")
	for _, path := range []string{firstPath, secondPath} {
		if err := os.Mkdir(path, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	firstRoot, err := manager.AddStorageLocation(ctx, firstPath, "StaleFirst")
	if err != nil {
		t.Fatal(err)
	}
	secondRoot, err := manager.AddStorageLocation(ctx, secondPath, "StaleSecond")
	if err != nil {
		t.Fatal(err)
	}
	created, err := manager.CreateRepository(ctx, CreateRepositorySpec{
		RequestID: "reconnect-stale-activity", Actor: "test", Name: "Stale Activity",
		DirectoryName: "stale-activity", Role: dbtypes.RepoRoleRegular, StorageLocationID: firstRoot.StorageLocationID.String(),
	})
	if err != nil {
		t.Fatal(err)
	}
	newPath := filepath.Join(secondRoot.Path, "stale-activity")
	if err := os.MkdirAll(newPath, 0o755); err != nil {
		t.Fatal(err)
	}
	config, err := repocfg.LoadConfigFromFile(created.Repository.Path)
	if err != nil {
		t.Fatal(err)
	}
	if err := config.SaveConfigToFile(newPath); err != nil {
		t.Fatal(err)
	}
	if err := manager.dirManager.CreateStructure(newPath); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(created.Repository.Path); err != nil {
		t.Fatal(err)
	}
	now := dbtypes.NewTimestamp(time.Now().UTC())
	if _, err := manager.readerDatabase.ExecContext(ctx, `
		UPDATE repositories SET activity = 'scanning', updated_at = ? WHERE repo_id = ?
	`, now, created.Repository.RepoID); err != nil {
		t.Fatal(err)
	}

	relocated, err := manager.RelocateRepository(ctx, created.Repository.RepoID.String(), newPath, LifecycleRequest{
		RequestID: "reconnect-stale-activity-tx", Actor: "test", ConfirmationType: "update_location",
	})
	if err != nil {
		t.Fatal(err)
	}
	if relocated.Activity != dbtypes.RepositoryActivityIdle {
		t.Fatalf("relocated activity = %q, want idle", relocated.Activity)
	}
}

func TestRelocateRepositoryRescansAndForgetsVolumeFileIdentity(t *testing.T) {
	catalog, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()
	base := t.TempDir()
	initializeDefaultStorageForTest(t, manager, filepath.Join(base, "default"))

	firstPath := filepath.Join(base, "fence-first")
	secondPath := filepath.Join(base, "fence-second")
	for _, path := range []string{firstPath, secondPath} {
		if err := os.Mkdir(path, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	firstRoot, err := manager.AddStorageLocation(ctx, firstPath, "FenceFirst")
	if err != nil {
		t.Fatal(err)
	}
	secondRoot, err := manager.AddStorageLocation(ctx, secondPath, "FenceSecond")
	if err != nil {
		t.Fatal(err)
	}
	created, err := manager.CreateRepository(ctx, CreateRepositorySpec{
		RequestID: "reconnect-fence", Actor: "test", Name: "Fence",
		DirectoryName: "fence", Role: dbtypes.RepoRoleRegular, StorageLocationID: firstRoot.StorageLocationID.String(),
	})
	if err != nil {
		t.Fatal(err)
	}
	entryID, runningScanID := seedRelocationScanState(t, catalog, created.Repository.RepoID)

	newPath := filepath.Join(secondRoot.Path, "fence")
	if err := os.MkdirAll(newPath, 0o755); err != nil {
		t.Fatal(err)
	}
	config, err := repocfg.LoadConfigFromFile(created.Repository.Path)
	if err != nil {
		t.Fatal(err)
	}
	if err := config.SaveConfigToFile(newPath); err != nil {
		t.Fatal(err)
	}
	if err := manager.dirManager.CreateStructure(newPath); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(created.Repository.Path); err != nil {
		t.Fatal(err)
	}

	if _, err := manager.RelocateRepository(ctx, created.Repository.RepoID.String(), newPath, LifecycleRequest{
		RequestID: "reconnect-fence-tx", Actor: "test", ConfirmationType: "update_location",
	}); err != nil {
		t.Fatal(err)
	}

	var status string
	if err := catalog.SQL.QueryRowContext(ctx, `SELECT status FROM repository_scans WHERE scan_id = ?`, runningScanID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "cancelled" {
		t.Fatalf("running scan after relocate = %s, want cancelled", status)
	}
	assertStorageCount(t, catalog.SQL, `
		SELECT count(*) FROM repository_scans
		WHERE repository_id = ? AND status = 'queued' AND scope_path = ''
	`, 1, created.Repository.RepoID.String())
	var ctime sql.NullInt64
	var fileID sql.NullString
	if err := catalog.SQL.QueryRowContext(ctx, `SELECT ctime_ns, file_id FROM repository_entries WHERE entry_id = ?`, entryID).Scan(&ctime, &fileID); err != nil {
		t.Fatal(err)
	}
	if ctime.Valid || fileID.Valid {
		t.Fatalf("relocated entry kept its old volume identity: ctime=%v file_id=%v", ctime, fileID)
	}
}

func TestRegisterRepositoryCopyLeavesSourceRepositoryUntouched(t *testing.T) {
	_, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()
	base := t.TempDir()
	initializeDefaultStorageForTest(t, manager, filepath.Join(base, "default"))
	defaultStorageLocation, err := manager.queries.GetDefaultStorageLocation(ctx)
	if err != nil {
		t.Fatal(err)
	}
	original, err := manager.CreateRepository(ctx, CreateRepositorySpec{
		RequestID: "copy-source-untouched", Actor: "test", Name: "Source",
		DirectoryName: "source", Role: dbtypes.RepoRoleRegular, StorageLocationID: defaultStorageLocation.StorageLocationID.String(),
	})
	if err != nil {
		t.Fatal(err)
	}
	sourceMedia := filepath.Join(original.Repository.Path, "source-original.jpg")
	if err := os.WriteFile(sourceMedia, []byte("source bytes stay"), 0o644); err != nil {
		t.Fatal(err)
	}
	sourceMarkerBefore, err := repocfg.LoadConfigFromFile(original.Repository.Path)
	if err != nil {
		t.Fatal(err)
	}
	sourceChecksum, err := sha256FileHex(sourceMedia)
	if err != nil {
		t.Fatal(err)
	}

	copyPath := filepath.Join(defaultStorageLocation.Path, "source-copy")
	if err := os.Mkdir(copyPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := manager.dirManager.CreateStructure(copyPath); err != nil {
		t.Fatal(err)
	}
	copyConfig := original.Repository.Config
	if err := copyConfig.SaveConfigToFile(copyPath); err != nil {
		t.Fatal(err)
	}
	copyMedia := filepath.Join(copyPath, "source-original.jpg")
	if err := os.WriteFile(copyMedia, []byte("source bytes stay"), 0o644); err != nil {
		t.Fatal(err)
	}

	registered, err := manager.RegisterRepositoryCopy(ctx, copyPath, nil, dbtypes.RepoRoleRegular, LifecycleRequest{
		RequestID: "copy-source-untouched-tx", Actor: "test", ConfirmationType: "add_separate",
	})
	if err != nil {
		t.Fatal(err)
	}
	if registered.RepoID == original.Repository.RepoID {
		t.Fatal("copy registration reused source repository id")
	}

	sourceMarkerAfter, err := repocfg.LoadConfigFromFile(original.Repository.Path)
	if err != nil || sourceMarkerAfter.ID != sourceMarkerBefore.ID {
		t.Fatalf("source marker changed: before=%s after=%+v err=%v", sourceMarkerBefore.ID, sourceMarkerAfter, err)
	}
	afterChecksum, err := sha256FileHex(sourceMedia)
	if err != nil || afterChecksum != sourceChecksum {
		t.Fatalf("source media checksum changed: before %s after %s err=%v", sourceChecksum, afterChecksum, err)
	}
	copyMarker, err := repocfg.LoadConfigFromFile(copyPath)
	if err != nil || copyMarker.ID != registered.RepoID.String() {
		t.Fatalf("copy marker = %+v err=%v", copyMarker, err)
	}
	copyChecksum, err := sha256FileHex(copyMedia)
	if err != nil || copyChecksum != sourceChecksum {
		t.Fatalf("copy media bytes rewritten: %s vs %s err=%v", copyChecksum, sourceChecksum, err)
	}
	storedOriginal, err := manager.GetRepository(original.Repository.RepoID.String())
	if err != nil || storedOriginal.Path != original.Repository.Path {
		t.Fatalf("source registration changed: %+v err=%v", storedOriginal, err)
	}
}

func TestReconcileAllUnreachablePrimaryKeepsPrimaryRole(t *testing.T) {
	_, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()
	defaultPath := filepath.Join(t.TempDir(), "default")
	initializeDefaultStorageForTest(t, manager, defaultPath)

	primary, err := manager.queries.GetPrimaryRepositoryRecord(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(primary.Path); err != nil {
		t.Fatal(err)
	}
	if err := manager.ReconcileAll(ctx); err != nil {
		t.Fatal(err)
	}
	reconciled, err := manager.queries.GetPrimaryRepositoryRecord(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if reconciled.Role != dbtypes.RepoRolePrimary {
		t.Fatalf("primary role = %q, want primary", reconciled.Role)
	}
	if reconciled.Reachability != dbtypes.RepositoryReachabilityOffline {
		t.Fatalf("primary reachability = %q, want offline", reconciled.Reachability)
	}
	status, err := manager.StorageRuntimeStatus(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if status.State != StorageRuntimeStateActive {
		t.Fatalf("instance runtime status = %+v, want active", status)
	}
}
