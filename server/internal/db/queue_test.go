package db

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"server/config"
)

func TestOpenQueueWithRecoveryQuarantinesCorruptFile(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	catalogPath := filepath.Join(root, "catalog.sqlite3")
	queuePath := filepath.Join(root, "river.sqlite3")
	if err := os.WriteFile(queuePath, []byte("not a sqlite database"), 0o600); err != nil {
		t.Fatal(err)
	}
	queueDB, quarantined, err := OpenQueueWithRecovery(ctx, config.DatabaseConfig{Path: catalogPath, QueuePath: queuePath})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = queueDB.Close(context.Background()) })
	if quarantined == "" {
		t.Fatal("corrupt queue was not quarantined")
	}
	if _, err := os.Stat(quarantined); err != nil {
		t.Fatalf("quarantined queue missing: %v", err)
	}
	if err := queueDB.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestOpenQueueUsesIndependentRiverDatabase(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	catalogPath := filepath.Join(root, "catalog.sqlite3")
	queuePath := filepath.Join(root, "river.sqlite3")
	queueDB, err := OpenQueue(ctx, config.DatabaseConfig{Path: catalogPath, QueuePath: queuePath})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = queueDB.Close(context.Background()) })
	if err := queueDB.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	var applicationID int
	if err := queueDB.SQL.QueryRowContext(ctx, "PRAGMA application_id").Scan(&applicationID); err != nil {
		t.Fatal(err)
	}
	if applicationID != queueApplicationID {
		t.Fatalf("queue application_id = %#x, want %#x", applicationID, queueApplicationID)
	}
	var riverTables int
	if err := queueDB.ReaderSQL.QueryRowContext(ctx, `
		SELECT count(*) FROM sqlite_schema WHERE type = 'table' AND name = 'river_job'
	`).Scan(&riverTables); err != nil {
		t.Fatal(err)
	}
	if riverTables != 1 {
		t.Fatalf("queue river_job tables = %d, want 1", riverTables)
	}
	if _, err := filepath.Abs(queueDB.Path); err != nil {
		t.Fatal(err)
	}
}

// TestQueueDatabaseContainsOnlyRiverSchema proves the queue file never adopts
// catalog tables, so it can be deleted and rebuilt without losing product
// state.
func TestQueueDatabaseContainsOnlyRiverSchema(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	queueDB, err := OpenQueue(ctx, config.DatabaseConfig{
		Path:      filepath.Join(root, "catalog.sqlite3"),
		QueuePath: filepath.Join(root, "river.sqlite3"),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = queueDB.Close(context.Background()) })
	if err := queueDB.Migrate(ctx); err != nil {
		t.Fatal(err)
	}

	for _, table := range []string{"river_job", "river_migration"} {
		var count int
		if err := queueDB.SQL.QueryRowContext(ctx, `
			SELECT count(*) FROM sqlite_schema WHERE type = 'table' AND name = ?
		`, table).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("queue table %s count = %d, want 1", table, count)
		}
	}
	for _, table := range []string{"assets", "repositories", "repository_nodes", "asset_pipeline_state"} {
		var count int
		if err := queueDB.SQL.QueryRowContext(ctx, `
			SELECT count(*) FROM sqlite_schema WHERE type = 'table' AND name = ?
		`, table).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("queue contains catalog table %s", table)
		}
	}
}

// TestCatalogProductionPathContainsNoRiverSchema proves MigrateCatalog, the
// production catalog entry point, never applies River's tables. DB.Migrate is
// the standalone test helper that intentionally combines both.
func TestCatalogProductionPathContainsNoRiverSchema(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	catalog, err := Open(ctx, config.DatabaseConfig{
		Path: filepath.Join(secureTempDir(t), "catalog-river-free.sqlite3"),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = catalog.Close(context.Background()) })
	if err := catalog.MigrateCatalog(ctx); err != nil {
		t.Fatal(err)
	}
	var riverTables int
	if err := catalog.SQL.QueryRowContext(ctx, `
		SELECT count(*) FROM sqlite_schema
		WHERE type = 'table' AND (name = 'river_job' OR name = 'river_migration')
	`).Scan(&riverTables); err != nil {
		t.Fatal(err)
	}
	if riverTables != 0 {
		t.Fatalf("production catalog contains %d River tables, want 0", riverTables)
	}
}

// TestCatalogAndQueueSharePragmaPolicy locks the shared connection policy on
// both independent files, including the query-only reader pools.
func TestCatalogAndQueueSharePragmaPolicy(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	catalog, err := Open(ctx, config.DatabaseConfig{
		Path:      filepath.Join(root, "catalog.sqlite3"),
		QueuePath: filepath.Join(root, "river.sqlite3"),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = catalog.Close(context.Background()) })
	if err := catalog.MigrateCatalog(ctx); err != nil {
		t.Fatal(err)
	}
	queueDB, err := OpenQueue(ctx, config.DatabaseConfig{
		Path:      filepath.Join(root, "catalog.sqlite3"),
		QueuePath: filepath.Join(root, "river.sqlite3"),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = queueDB.Close(context.Background()) })
	if err := queueDB.Migrate(ctx); err != nil {
		t.Fatal(err)
	}

	for name, database := range map[string]*DB{"catalog": catalog} {
		if err := verifyPragmas(ctx, database.SQL); err != nil {
			t.Errorf("%s writer pragmas: %v", name, err)
		}
		if err := verifyReaderPragmas(ctx, database.ReaderSQL); err != nil {
			t.Errorf("%s reader pragmas: %v", name, err)
		}
	}
	if err := verifyPragmas(ctx, queueDB.SQL); err != nil {
		t.Errorf("queue writer pragmas: %v", err)
	}
	if err := verifyReaderPragmas(ctx, queueDB.ReaderSQL); err != nil {
		t.Errorf("queue reader pragmas: %v", err)
	}
}

// TestQueueDatabaseIsDisposableAndRebuildsIndependently deletes the queue file
// between two opens while catalog state exists, proving the queue is
// rebuildable delivery state rather than product truth.
func TestQueueDatabaseIsDisposableAndRebuildsIndependently(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	catalogPath := filepath.Join(root, "catalog.sqlite3")
	queuePath := filepath.Join(root, "river.sqlite3")
	cfg := config.DatabaseConfig{Path: catalogPath, QueuePath: queuePath}

	catalog, err := Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = catalog.Close(context.Background()) })
	if err := catalog.MigrateCatalog(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := catalog.SQL.ExecContext(ctx, `
		UPDATE system_state SET bootstrap_phase = 'ready', updated_at = 42 WHERE id = 1
	`); err != nil {
		t.Fatal(err)
	}

	first, err := OpenQueue(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := first.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if err := first.Close(ctx); err != nil {
		t.Fatal(err)
	}

	for _, artifact := range []string{queuePath, queuePath + "-wal", queuePath + "-shm"} {
		if err := os.Remove(artifact); err != nil && !os.IsNotExist(err) {
			t.Fatalf("delete queue artifact %s: %v", filepath.Base(artifact), err)
		}
	}

	rebuilt, err := OpenQueue(ctx, cfg)
	if err != nil {
		t.Fatalf("reopen deleted queue: %v", err)
	}
	t.Cleanup(func() { _ = rebuilt.Close(context.Background()) })
	if err := rebuilt.Migrate(ctx); err != nil {
		t.Fatalf("remigrate deleted queue: %v", err)
	}

	var phase string
	var updatedAt int64
	if err := catalog.SQL.QueryRowContext(ctx, `
		SELECT bootstrap_phase, updated_at FROM system_state WHERE id = 1
	`).Scan(&phase, &updatedAt); err != nil {
		t.Fatal(err)
	}
	if phase != "ready" || updatedAt != 42 {
		t.Fatalf("catalog state changed by queue disposal: phase=%q updated_at=%d", phase, updatedAt)
	}
	if !strings.HasPrefix(rebuilt.Path, root) {
		t.Fatalf("rebuilt queue path = %s", rebuilt.Path)
	}
}
