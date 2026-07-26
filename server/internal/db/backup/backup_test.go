package backup

import (
	"context"
	"database/sql"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
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
		LibraryID:               info.LibraryID,
		ConfigSchemaVersion:     schemaVersion,
		MaxApplicationMigration: info.ApplicationMigration,
		MaxRiverMigration:       info.RiverMigration,
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
	if marker.State != pendingStateApplied || marker.RestorePoint == "" || marker.PreviousPath == "" {
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
	for _, name := range []string{oldest, middle, newest, restorePoint} {
		touchPair(t, dir, name, time.Time{})
	}

	removed, err := Prune(dir, 2, t.Logf)
	if err != nil {
		t.Fatal(err)
	}
	if len(removed) != 1 || removed[0] != oldest {
		t.Fatalf("removed = %v", removed)
	}
	for _, name := range []string{middle, newest, restorePoint} {
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
