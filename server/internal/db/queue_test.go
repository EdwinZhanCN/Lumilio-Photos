package db

import (
	"context"
	"os"
	"path/filepath"
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
