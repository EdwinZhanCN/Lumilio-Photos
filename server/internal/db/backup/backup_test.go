package backup

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"server/config"
	"server/internal/db"
	"server/internal/settings"
	"server/platform/sqliteuri"
)

func openTestCatalog(t *testing.T, path string) *db.DB {
	t.Helper()
	catalog, err := db.Open(context.Background(), config.DatabaseConfig{Path: path})
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	if err := catalog.Migrate(context.Background()); err != nil {
		_ = catalog.Close(context.Background())
		t.Fatalf("db.Migrate: %v", err)
	}
	return catalog
}

func closeTestCatalog(t *testing.T, catalog *db.DB) {
	t.Helper()
	if err := catalog.Close(context.Background()); err != nil {
		t.Fatalf("db.Close: %v", err)
	}
}

func compatibilityFor(t *testing.T, catalog *db.DB, schemaVersion int) Compatibility {
	t.Helper()
	info, err := db.InspectCatalog(context.Background(), catalog.Path)
	if err != nil {
		t.Fatalf("db.InspectCatalog: %v", err)
	}
	return Compatibility{
		LibraryID:           info.LibraryID,
		ConfigSchemaVersion: schemaVersion,
		SchemaVersion:       info.SchemaVersion,
		MaxRiverMigration:   info.RiverMigration,
	}
}

func TestCreateSnapshotIsStandaloneAndChecksumProtected(t *testing.T) {
	root := t.TempDir()
	catalog := openTestCatalog(t, filepath.Join(root, "app-state", "library.sqlite3"))
	defer closeTestCatalog(t, catalog)

	if _, err := catalog.SQL.ExecContext(context.Background(), `
		UPDATE system_state
		SET bootstrap_phase = 'admin_created', updated_at = ?
		WHERE id = 1
	`, time.Now().UTC().UnixMicro()); err != nil {
		t.Fatal(err)
	}

	metadata := SnapshotMetadata{AppVersion: "test", ConfigSchemaVersion: 2}
	snapshot, err := CreateSnapshot(
		context.Background(),
		catalog.SQL,
		filepath.Join(root, "backups"),
		"",
		metadata,
		t.Logf,
	)
	if err != nil {
		t.Fatalf("CreateSnapshot: %v", err)
	}
	compatibility := compatibilityFor(t, catalog, metadata.ConfigSchemaVersion)
	manifest, info, err := ValidateSnapshot(context.Background(), snapshot.Path, compatibility)
	if err != nil {
		t.Fatalf("ValidateSnapshot: %v", err)
	}
	if info.LibraryID != compatibility.LibraryID || manifest.QuickCheck != "ok" {
		t.Fatalf("snapshot identity/check = %q/%q", info.LibraryID, manifest.QuickCheck)
	}
	for _, sidecar := range []string{snapshot.Path + "-wal", snapshot.Path + "-shm"} {
		if _, err := os.Stat(sidecar); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("snapshot validation left SQLite sidecar %s: %v", sidecar, err)
		}
	}

	query := make(url.Values)
	query.Set("mode", "ro")
	snapshotDB, err := sql.Open("sqlite3", sqliteuri.DSN(snapshot.Path, query))
	if err != nil {
		t.Fatalf("open standalone snapshot: %v", err)
	}
	var phase string
	if err := snapshotDB.QueryRowContext(context.Background(), "SELECT bootstrap_phase FROM system_state WHERE id = 1").Scan(&phase); err != nil {
		t.Fatal(err)
	}
	if phase != "admin_created" {
		t.Fatalf("snapshot phase = %q", phase)
	}
	if err := snapshotDB.Close(); err != nil {
		t.Fatal(err)
	}

	file, err := os.OpenFile(snapshot.Path, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	value := []byte{0}
	if _, err := file.ReadAt(value, 100); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	value[0] ^= 0xff
	if _, err := file.WriteAt(value, 100); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ValidateSnapshot(context.Background(), snapshot.Path, compatibility); err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("corrupt snapshot error = %v, want checksum rejection", err)
	}
}

func TestCatalogSnapshotExcludesIndependentQueueDatabase(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	catalogPath := filepath.Join(root, "app-state", "library.sqlite3")
	queuePath := filepath.Join(root, "app-state", "river.sqlite3")
	catalog, err := db.Open(ctx, config.DatabaseConfig{Path: catalogPath, QueuePath: queuePath})
	if err != nil {
		t.Fatal(err)
	}
	if err := catalog.MigrateCatalog(ctx, nil); err != nil {
		_ = catalog.Close(ctx)
		t.Fatal(err)
	}
	queueDB, err := db.OpenQueue(ctx, config.DatabaseConfig{Path: catalogPath, QueuePath: queuePath})
	if err != nil {
		_ = catalog.Close(ctx)
		t.Fatal(err)
	}
	if err := queueDB.Migrate(ctx); err != nil {
		_ = queueDB.Close(ctx)
		_ = catalog.Close(ctx)
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = queueDB.Close(context.Background())
		_ = catalog.Close(context.Background())
	})

	snapshot, err := CreateSnapshot(ctx, catalog.SQL, filepath.Join(root, "backups"), "", SnapshotMetadata{
		AppVersion: "test", ConfigSchemaVersion: config.SchemaVersion,
	}, t.Logf)
	if err != nil {
		t.Fatal(err)
	}
	info, err := db.InspectStandaloneCatalog(ctx, snapshot.Path)
	if err != nil {
		t.Fatal(err)
	}
	if info.RiverMigration != 0 {
		t.Fatalf("catalog snapshot reports River migration %d, want 0", info.RiverMigration)
	}
	query := make(url.Values)
	query.Set("mode", "ro")
	snapshotDB, err := sql.Open("sqlite3", sqliteuri.DSN(snapshot.Path, query))
	if err != nil {
		t.Fatal(err)
	}
	defer snapshotDB.Close()
	var riverTables int
	if err := snapshotDB.QueryRowContext(ctx, `
		SELECT count(*) FROM sqlite_schema WHERE type = 'table' AND name = 'river_job'
	`).Scan(&riverTables); err != nil {
		t.Fatal(err)
	}
	if riverTables != 0 {
		t.Fatalf("catalog snapshot contains %d River tables, want 0", riverTables)
	}
}

func TestCreateSnapshotFromReaderDoesNotWaitForWriterTransaction(t *testing.T) {
	root := t.TempDir()
	catalog := openTestCatalog(t, filepath.Join(root, "app-state", "library.sqlite3"))
	defer closeTestCatalog(t, catalog)

	// Hold SQLite's sole writer and its only database/sql connection. WAL
	// readers must remain usable, and Online Backup must consume one of those
	// reader connections instead of queuing behind the writer.
	tx, err := catalog.SQL.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(context.Background(), `
		UPDATE system_state
		SET updated_at = updated_at
		WHERE id = 1
	`); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := CreateSnapshot(
		ctx,
		catalog.ReaderSQL,
		filepath.Join(root, "backups"),
		"",
		SnapshotMetadata{AppVersion: "test", ConfigSchemaVersion: 2},
		t.Logf,
	); err != nil {
		t.Fatalf("reader-backed snapshot waited for writer: %v", err)
	}
}

func TestCreateSnapshotMakesProgressDuringContinuousWriterCommits(t *testing.T) {
	root := t.TempDir()
	catalog := openTestCatalog(t, filepath.Join(root, "app-state", "library.sqlite3"))
	defer closeTestCatalog(t, catalog)

	if _, err := catalog.SQL.ExecContext(context.Background(), `
		CREATE TABLE backup_pressure (id INTEGER PRIMARY KEY, payload BLOB NOT NULL);
		INSERT INTO backup_pressure (id, payload) VALUES (1, zeroblob(16777216));
	`); err != nil {
		t.Fatal(err)
	}

	stop := make(chan struct{})
	done := make(chan struct{})
	writerErrors := make(chan error, 1)
	var commits atomic.Int64
	go func() {
		defer close(done)
		ticker := time.NewTicker(2 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				if _, err := catalog.SQL.ExecContext(context.Background(), `
					UPDATE system_state SET updated_at = updated_at + 1 WHERE id = 1
				`); err != nil {
					select {
					case writerErrors <- err:
					default:
					}
					return
				}
				commits.Add(1)
			}
		}
	}()
	defer func() {
		close(stop)
		<-done
	}()
	deadline := time.Now().Add(time.Second)
	for commits.Load() < 5 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if commits.Load() < 5 {
		t.Fatal("continuous writer did not establish load")
	}

	before := commits.Load()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	snapshot, err := CreateSnapshot(
		ctx,
		catalog.ReaderSQL,
		filepath.Join(root, "backups"),
		"",
		SnapshotMetadata{AppVersion: "test", ConfigSchemaVersion: 2},
		t.Logf,
	)
	if err != nil {
		t.Fatalf("reader snapshot was starved by continuous commits: %v", err)
	}
	if commits.Load()-before < 10 {
		t.Fatalf("only %d writers committed during snapshot; WAL concurrency was not exercised", commits.Load()-before)
	}
	select {
	case err := <-writerErrors:
		t.Fatalf("continuous writer failed while backup held a read snapshot: %v", err)
	default:
	}
	if snapshot.Manifest.QuickCheck != "ok" || snapshot.Manifest.DatabaseSize == 0 {
		t.Fatalf("snapshot manifest = %#v", snapshot.Manifest)
	}
}

func TestStagedRestoreReplacesOnlyBetweenGenerations(t *testing.T) {
	root := t.TempDir()
	activePath := filepath.Join(root, "app-state", "library.sqlite3")
	backupsDir := filepath.Join(root, "backups")
	catalog := openTestCatalog(t, activePath)

	setPhase := func(database *db.DB, phase string) {
		t.Helper()
		if _, err := database.SQL.ExecContext(context.Background(), `
			UPDATE system_state SET bootstrap_phase = ?, updated_at = ? WHERE id = 1
		`, phase, time.Now().UTC().UnixMicro()); err != nil {
			t.Fatal(err)
		}
	}
	setPhase(catalog, "admin_created")
	metadata := SnapshotMetadata{AppVersion: "test", ConfigSchemaVersion: 2}
	snapshot, err := CreateSnapshot(context.Background(), catalog.SQL, backupsDir, "", metadata, t.Logf)
	if err != nil {
		t.Fatalf("CreateSnapshot: %v", err)
	}
	compatibility := compatibilityFor(t, catalog, metadata.ConfigSchemaVersion)
	setPhase(catalog, "ready")

	if err := StageRestore(context.Background(), activePath, snapshot.Path, metadata, compatibility); err != nil {
		t.Fatalf("StageRestore: %v", err)
	}
	var livePhase string
	if err := catalog.SQL.QueryRowContext(context.Background(), "SELECT bootstrap_phase FROM system_state WHERE id = 1").Scan(&livePhase); err != nil {
		t.Fatal(err)
	}
	if livePhase != "ready" {
		t.Fatalf("staging changed live database: phase = %q", livePhase)
	}

	closeTestCatalog(t, catalog)
	applied, err := ApplyPendingRestore(context.Background(), activePath, t.Logf)
	if err != nil || !applied {
		t.Fatalf("ApplyPendingRestore = %v/%v", applied, err)
	}
	marker, err := readPendingRestore(PendingRestorePath(activePath))
	if err != nil {
		t.Fatal(err)
	}
	if marker.State != pendingStateVerified || marker.RestorePoint == "" || marker.PreviousPath == "" {
		t.Fatalf("applied marker = %+v", marker)
	}

	restored := openTestCatalog(t, activePath)
	if err := restored.SQL.QueryRowContext(context.Background(), "SELECT bootstrap_phase FROM system_state WHERE id = 1").Scan(&livePhase); err != nil {
		t.Fatal(err)
	}
	if livePhase != "admin_created" {
		t.Fatalf("restored phase = %q", livePhase)
	}
	if err := CompletePendingRestore(context.Background(), activePath); err != nil {
		t.Fatalf("CompletePendingRestore: %v", err)
	}
	if _, err := os.Stat(PendingRestorePath(activePath)); !os.IsNotExist(err) {
		t.Fatalf("pending marker still exists: %v", err)
	}
	if _, err := os.Stat(marker.PreviousPath); !os.IsNotExist(err) {
		t.Fatalf("previous active catalog still exists: %v", err)
	}
	if _, err := os.Stat(marker.RestorePoint); err != nil {
		t.Fatalf("restore point was not retained: %v", err)
	}
	closeTestCatalog(t, restored)
}

func TestRestoreSwapReconcilesInjectedCrashBoundaries(t *testing.T) {
	points := []string{
		faultAfterStagedCopy,
		faultAfterPreviousRename,
		faultAfterActiveRename,
		faultBeforeActiveMarker,
		faultAfterActiveMarker,
		faultAfterVerifiedMarker,
	}
	for _, point := range points {
		t.Run(point, func(t *testing.T) {
			activePath := preparePendingRestore(t)
			fired := false
			_, err := applyPendingRestore(context.Background(), activePath, t.Logf, func(candidate string) error {
				if !fired && candidate == point {
					fired = true
					return errors.New("simulated process loss")
				}
				return nil
			})
			if err == nil || !fired {
				t.Fatalf("injected restore fault %q = %v, fired=%t", point, err, fired)
			}

			applied, err := ApplyPendingRestore(context.Background(), activePath, t.Logf)
			if err != nil || !applied {
				t.Fatalf("reconcile restore after %q = %t/%v", point, applied, err)
			}
			marker, err := readPendingRestore(PendingRestorePath(activePath))
			if err != nil {
				t.Fatal(err)
			}
			if marker.State != pendingStateVerified {
				t.Fatalf("reconciled marker state = %q, want %q", marker.State, pendingStateVerified)
			}
			if phase := catalogBootstrapPhase(t, activePath); phase != "admin_created" {
				t.Fatalf("restored phase = %q", phase)
			}
			if err := CompletePendingRestore(context.Background(), activePath); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestRestoreReplacesPartialStagedCopyAfterCrash(t *testing.T) {
	activePath := preparePendingRestore(t)
	marker, err := readPendingRestore(PendingRestorePath(activePath))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(marker.StagedPath, []byte("partial copy"), 0o600); err != nil {
		t.Fatal(err)
	}
	applied, err := ApplyPendingRestore(context.Background(), activePath, t.Logf)
	if err != nil || !applied {
		t.Fatalf("ApplyPendingRestore with partial staged copy = %t/%v", applied, err)
	}
	if phase := catalogBootstrapPhase(t, activePath); phase != "admin_created" {
		t.Fatalf("restored phase = %q", phase)
	}
	if err := CompletePendingRestore(context.Background(), activePath); err != nil {
		t.Fatal(err)
	}
}

func TestRestoreRollbackReconcilesInjectedCrashBoundaries(t *testing.T) {
	points := []string{
		faultAfterFailedRename,
		faultAfterRollbackRename,
		faultAfterRollbackMarker,
	}
	for _, point := range points {
		t.Run(point, func(t *testing.T) {
			activePath := preparePendingRestore(t)
			applied, err := ApplyPendingRestore(context.Background(), activePath, t.Logf)
			if err != nil || !applied {
				t.Fatalf("ApplyPendingRestore = %t/%v", applied, err)
			}

			fired := false
			err = rollbackPendingRestore(context.Background(), activePath, t.Logf, func(candidate string) error {
				if !fired && candidate == point {
					fired = true
					return errors.New("simulated process loss")
				}
				return nil
			})
			if err == nil || !fired {
				t.Fatalf("injected rollback fault %q = %v, fired=%t", point, err, fired)
			}

			applied, err = ApplyPendingRestore(context.Background(), activePath, t.Logf)
			if err != nil || applied {
				t.Fatalf("startup rollback reconciliation after %q = %t/%v", point, applied, err)
			}
			if phase := catalogBootstrapPhase(t, activePath); phase != "ready" {
				t.Fatalf("rolled-back phase = %q", phase)
			}
			if _, err := os.Stat(PendingRestorePath(activePath)); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("rollback marker remains: %v", err)
			}
		})
	}
}

func preparePendingRestore(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	activePath := filepath.Join(root, "app-state", "library.sqlite3")
	catalog := openTestCatalog(t, activePath)
	setCatalogBootstrapPhase(t, catalog, "admin_created")
	metadata := SnapshotMetadata{AppVersion: "test", ConfigSchemaVersion: 2}
	snapshot, err := CreateSnapshot(
		context.Background(),
		catalog.SQL,
		filepath.Join(root, "backups"),
		"",
		metadata,
		t.Logf,
	)
	if err != nil {
		t.Fatal(err)
	}
	compatibility := compatibilityFor(t, catalog, metadata.ConfigSchemaVersion)
	setCatalogBootstrapPhase(t, catalog, "ready")
	if err := StageRestore(context.Background(), activePath, snapshot.Path, metadata, compatibility); err != nil {
		t.Fatal(err)
	}
	closeTestCatalog(t, catalog)
	return activePath
}

func setCatalogBootstrapPhase(t *testing.T, catalog *db.DB, phase string) {
	t.Helper()
	if _, err := catalog.SQL.ExecContext(context.Background(), `
		UPDATE system_state SET bootstrap_phase = ?, updated_at = ? WHERE id = 1
	`, phase, time.Now().UTC().UnixMicro()); err != nil {
		t.Fatal(err)
	}
}

func catalogBootstrapPhase(t *testing.T, activePath string) string {
	t.Helper()
	database, err := sql.Open("sqlite3", activePath)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	var phase string
	if err := database.QueryRowContext(context.Background(), "SELECT bootstrap_phase FROM system_state WHERE id = 1").Scan(&phase); err != nil {
		t.Fatal(err)
	}
	return phase
}

func TestCorruptSnapshotRejectedBeforeMarker(t *testing.T) {
	root := t.TempDir()
	activePath := filepath.Join(root, "app-state", "library.sqlite3")
	catalog := openTestCatalog(t, activePath)
	defer closeTestCatalog(t, catalog)
	metadata := SnapshotMetadata{AppVersion: "test", ConfigSchemaVersion: 2}
	snapshot, err := CreateSnapshot(context.Background(), catalog.SQL, filepath.Join(root, "backups"), "", metadata, t.Logf)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(snapshot.Path, 128); err != nil {
		t.Fatal(err)
	}
	err = StageRestore(context.Background(), activePath, snapshot.Path, metadata, compatibilityFor(t, catalog, 2))
	if err == nil {
		t.Fatal("StageRestore accepted corrupt snapshot")
	}
	if _, statErr := os.Stat(PendingRestorePath(activePath)); !os.IsNotExist(statErr) {
		t.Fatalf("corrupt snapshot wrote pending marker: %v", statErr)
	}
}

func touchPair(t *testing.T, dir, name string, modTime time.Time) {
	t.Helper()
	for _, artifact := range []string{name, ManifestName(name)} {
		path := filepath.Join(dir, artifact)
		if err := os.WriteFile(path, []byte("fixture"), 0o600); err != nil {
			t.Fatal(err)
		}
		if !modTime.IsZero() {
			if err := os.Chtimes(path, modTime, modTime); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestPruneKeepsNewestSnapshotPairsAndRestorePoints(t *testing.T) {
	dir := t.TempDir()
	oldest := FileName(time.Date(2026, 7, 8, 2, 0, 0, 0, time.UTC))
	middle := FileName(time.Date(2026, 7, 9, 2, 0, 0, 0, time.UTC))
	newest := FileName(time.Date(2026, 7, 10, 2, 0, 0, 0, time.UTC))
	restorePoint := RestorePointPrefix + newest
	preUpgrade := PreUpgradePrefix + oldest
	for _, name := range []string{oldest, middle, newest, restorePoint, preUpgrade} {
		touchPair(t, dir, name, time.Time{})
	}

	removed, err := Prune(dir, 2, t.Logf)
	if err != nil {
		t.Fatal(err)
	}
	if len(removed) != 1 || removed[0] != oldest {
		t.Fatalf("removed = %v", removed)
	}
	for _, name := range []string{middle, newest, restorePoint, preUpgrade} {
		for _, artifact := range []string{name, ManifestName(name)} {
			if _, err := os.Stat(filepath.Join(dir, artifact)); err != nil {
				t.Errorf("%s should remain: %v", artifact, err)
			}
		}
	}
}

func TestSchedulerPolicyAndForcedSnapshot(t *testing.T) {
	root := t.TempDir()
	catalog := openTestCatalog(t, filepath.Join(root, "app-state", "library.sqlite3"))
	defer closeTestCatalog(t, catalog)
	dir := filepath.Join(root, "backups")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	cfg := settings.Backup{Enabled: false, IntervalHours: 24, KeepLast: 2}
	scheduler := &Scheduler{
		Source:   catalog.SQL,
		Dir:      dir,
		Metadata: SnapshotMetadata{AppVersion: "test", ConfigSchemaVersion: 2},
		Ready:    func(context.Context) (bool, error) { return true, nil },
		Settings: func(context.Context) (settings.Backup, error) { return cfg, nil },
		Logf:     t.Logf,
	}
	if err := scheduler.Run(context.Background(), false); err != nil {
		t.Fatalf("disabled periodic run: %v", err)
	}
	if _, ok := LatestRoutine(dir); ok {
		t.Fatal("disabled periodic run created a snapshot")
	}
	if err := scheduler.Run(context.Background(), true); err != nil {
		t.Fatalf("forced run: %v", err)
	}
	if _, ok := LatestRoutine(dir); !ok {
		t.Fatal("forced run did not create a snapshot")
	}
}

func TestInspectCatalogUsesUserVersionWithoutMigrationLedger(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	appState := filepath.Join(root, "app-state")
	if err := os.MkdirAll(appState, 0o700); err != nil {
		t.Fatal(err)
	}
	catalog := openTestCatalog(t, filepath.Join(appState, "library.sqlite3"))
	defer closeTestCatalog(t, catalog)

	var ledgerTables int
	if err := catalog.SQL.QueryRowContext(ctx, `
		SELECT count(*) FROM sqlite_schema
		WHERE type = 'table' AND name = 'lumilio_schema_migrations'
	`).Scan(&ledgerTables); err != nil {
		t.Fatal(err)
	}
	if ledgerTables != 0 {
		t.Fatal("migration ledger table must not exist on a current catalog")
	}

	info, err := db.InspectCatalog(ctx, catalog.Path)
	if err != nil {
		t.Fatalf("InspectCatalog: %v", err)
	}
	if info.SchemaVersion != db.SchemaVersion {
		t.Fatalf("SchemaVersion = %d, want PRAGMA user_version %d (not absent-ledger zero)", info.SchemaVersion, db.SchemaVersion)
	}

	backupDir := filepath.Join(root, "backups")
	if err := os.MkdirAll(backupDir, 0o700); err != nil {
		t.Fatal(err)
	}
	snapshot, err := CreateSnapshot(ctx, catalog.SQL, backupDir, "", SnapshotMetadata{
		AppVersion: "test", ConfigSchemaVersion: 2,
	}, t.Logf)
	if err != nil {
		t.Fatal(err)
	}
	missingVersion := Compatibility{LibraryID: info.LibraryID, SchemaVersion: 0}
	if _, _, err := ValidateSnapshot(ctx, snapshot.Path, missingVersion); err == nil || !strings.Contains(err.Error(), "missing the runtime schema version") {
		t.Fatalf("ValidateSnapshot with SchemaVersion=0 error = %v", err)
	}
	// A runtime newer than the snapshot restores it and upgrades it on start.
	newerRuntime := Compatibility{LibraryID: info.LibraryID, SchemaVersion: db.SchemaVersion + 1, ConfigSchemaVersion: 3}
	if _, _, err := ValidateSnapshot(ctx, snapshot.Path, newerRuntime); err != nil {
		t.Fatalf("ValidateSnapshot of an older snapshot for restore-then-upgrade: %v", err)
	}
}

// TestStageRestoreInstallsOlderSnapshotForUpgrade proves restore-then-upgrade
// reaches the installed state: a snapshot older than the runtime (schema and
// config) is staged and activated unchanged, leaving the steps to the
// runtime's MigrateCatalog on the next start.
func TestStageRestoreInstallsOlderSnapshotForUpgrade(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	activePath := filepath.Join(root, "app-state", "library.sqlite3")
	catalog := openTestCatalog(t, activePath)
	metadata := SnapshotMetadata{AppVersion: "test", ConfigSchemaVersion: 1}
	snapshot, err := CreateSnapshot(ctx, catalog.SQL, filepath.Join(root, "backups"), "", metadata, t.Logf)
	if err != nil {
		t.Fatal(err)
	}
	newerRuntime := compatibilityFor(t, catalog, 2)
	newerRuntime.SchemaVersion = db.SchemaVersion + 1
	if err := StageRestore(ctx, activePath, snapshot.Path, metadata, newerRuntime); err != nil {
		t.Fatalf("stage older snapshot: %v", err)
	}
	closeTestCatalog(t, catalog)
	installed, err := ApplyPendingRestore(ctx, activePath, t.Logf)
	if err != nil || !installed {
		t.Fatalf("ApplyPendingRestore = %t, %v", installed, err)
	}
	info, err := db.InspectCatalog(ctx, activePath)
	if err != nil {
		t.Fatal(err)
	}
	if info.SchemaVersion != int64(db.SchemaVersion) {
		t.Fatalf("installed schema version = %d, want the snapshot's %d awaiting upgrade", info.SchemaVersion, db.SchemaVersion)
	}
}

// TestValidateSnapshotRejectsNewerSnapshot proves a snapshot written by a
// newer build (schema or config) is rejected rather than restored.
func TestValidateSnapshotRejectsNewerSnapshot(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	catalog := openTestCatalog(t, filepath.Join(root, "app-state", "library.sqlite3"))
	defer closeTestCatalog(t, catalog)
	snapshot, err := CreateSnapshot(ctx, catalog.SQL, filepath.Join(root, "backups"), "", SnapshotMetadata{
		AppVersion: "test", ConfigSchemaVersion: 2,
	}, t.Logf)
	if err != nil {
		t.Fatal(err)
	}

	if _, _, err := ValidateSnapshot(ctx, snapshot.Path, compatibilityFor(t, catalog, 1)); err == nil || !strings.Contains(err.Error(), "config schema 2 is newer than runtime schema 1") {
		t.Fatalf("newer config snapshot error = %v", err)
	}

	database, err := sql.Open("sqlite3", snapshot.Path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(fmt.Sprintf("PRAGMA user_version = %d", db.SchemaVersion+1)); err != nil {
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	checksum, err := fileSHA256(snapshot.Path)
	if err != nil {
		t.Fatal(err)
	}
	rewriteManifest(t, ManifestPath(snapshot.Path), func(payload map[string]any) {
		payload["sha256"] = checksum
		payload["schema_version"] = db.SchemaVersion + 1
	})
	if _, _, err := ValidateSnapshot(ctx, snapshot.Path, compatibilityFor(t, catalog, 2)); err == nil || !strings.Contains(err.Error(), "newer than this build supports") {
		t.Fatalf("newer schema snapshot error = %v", err)
	}
}

// TestManifestSchemaProvenanceReplacesMigrationLedger proves the manifest
// contract itself changed: it carries schema_version and no longer carries the
// retired application_migration_version field.
func TestManifestSchemaProvenanceReplacesMigrationLedger(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	catalog := openTestCatalog(t, filepath.Join(root, "app-state", "library.sqlite3"))
	defer closeTestCatalog(t, catalog)

	snapshot, err := CreateSnapshot(ctx, catalog.SQL, filepath.Join(root, "backups"), "", SnapshotMetadata{
		AppVersion: "test", ConfigSchemaVersion: 2,
	}, t.Logf)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Manifest.FormatVersion != manifestFormatVersion {
		t.Fatalf("manifest format = %d, want %d", snapshot.Manifest.FormatVersion, manifestFormatVersion)
	}
	if snapshot.Manifest.SchemaVersion != db.SchemaVersion {
		t.Fatalf("manifest schema version = %d, want %d", snapshot.Manifest.SchemaVersion, db.SchemaVersion)
	}
	rawManifest, err := os.ReadFile(ManifestPath(snapshot.Path))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(rawManifest), fmt.Sprintf(`"schema_version": %d`, db.SchemaVersion)) {
		t.Fatalf("manifest lacks schema_version: %s", rawManifest)
	}
	if strings.Contains(string(rawManifest), "application_migration_version") {
		t.Fatalf("manifest still carries retired provenance: %s", rawManifest)
	}
}

// TestValidateSnapshotAttributesManifestFormatMismatch proves a manifest
// format other than the current one fails loudly, and that a pre-release
// snapshot is named as pre-release even though its manifest number (2 or 3)
// reads as newer than the rc.1 baseline.
func TestValidateSnapshotAttributesManifestFormatMismatch(t *testing.T) {
	for _, tc := range []struct {
		name          string
		formatVersion int
		preRelease    bool
		want          string
	}{
		{name: "newer build", formatVersion: manifestFormatVersion + 1, want: "newer than this build supports"},
		{name: "missing format", formatVersion: 0, want: "manifest format 0 is unsupported"},
		{name: "published beta", formatVersion: 2, preRelease: true},
		{name: "internal pre-release", formatVersion: 3, preRelease: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			root := t.TempDir()
			catalog := openTestCatalog(t, filepath.Join(root, "app-state", "library.sqlite3"))
			defer closeTestCatalog(t, catalog)

			snapshot, err := CreateSnapshot(ctx, catalog.SQL, filepath.Join(root, "backups"), "", SnapshotMetadata{
				AppVersion: "test", ConfigSchemaVersion: 2,
			}, t.Logf)
			if err != nil {
				t.Fatal(err)
			}
			if tc.preRelease {
				markSnapshotPreRelease(t, snapshot.Path)
			}
			rewriteManifest(t, ManifestPath(snapshot.Path), func(payload map[string]any) {
				payload["format_version"] = tc.formatVersion
			})

			_, _, err = ValidateSnapshot(ctx, snapshot.Path, compatibilityFor(t, catalog, 2))
			if tc.preRelease {
				if !errors.Is(err, db.ErrPreReleaseCatalog) {
					t.Fatalf("pre-release snapshot error = %v, want ErrPreReleaseCatalog", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("manifest format %d error = %v, want %q", tc.formatVersion, err, tc.want)
			}
		})
	}
}

// markSnapshotPreRelease stamps the identity every pre-release catalog
// carries ("LUMI") onto a finalized snapshot.
func markSnapshotPreRelease(t *testing.T, path string) {
	t.Helper()
	database, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if _, err := database.Exec(fmt.Sprintf("PRAGMA application_id = %d", 0x4c554d49)); err != nil {
		t.Fatal(err)
	}
}

func rewriteManifest(t *testing.T, path string, edit func(map[string]any)) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	edit(payload)
	encoded, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(encoded, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
}
