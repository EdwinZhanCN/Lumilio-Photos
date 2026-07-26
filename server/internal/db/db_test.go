package db

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"server/config"
)

func TestOpenMigrateAndReopenSQLiteCatalog(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	dir := secureTempDir(t)
	path := filepath.Join(dir, "library.sqlite3")

	first, err := Open(ctx, config.DatabaseConfig{Path: path})
	if err != nil {
		t.Fatalf("open catalog: %v", err)
	}
	if err := first.Migrate(ctx); err != nil {
		t.Fatalf("first migration: %v", err)
	}
	if err := first.Migrate(ctx); err != nil {
		t.Fatalf("idempotent migration: %v", err)
	}

	var foreignKeys int
	var journalMode, synchronous string
	var busyTimeout int
	if err := first.SQL.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&foreignKeys); err != nil {
		t.Fatalf("read foreign_keys: %v", err)
	}
	if err := first.SQL.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&journalMode); err != nil {
		t.Fatalf("read journal_mode: %v", err)
	}
	if err := first.SQL.QueryRowContext(ctx, "PRAGMA synchronous").Scan(&synchronous); err != nil {
		t.Fatalf("read synchronous: %v", err)
	}
	if err := first.SQL.QueryRowContext(ctx, "PRAGMA busy_timeout").Scan(&busyTimeout); err != nil {
		t.Fatalf("read busy_timeout: %v", err)
	}
	if foreignKeys != 1 || !strings.EqualFold(journalMode, "wal") || synchronous != "1" || busyTimeout != 5000 {
		t.Fatalf(
			"unexpected pragmas: foreign_keys=%d journal_mode=%s synchronous=%s busy_timeout=%d",
			foreignKeys,
			journalMode,
			synchronous,
			busyTimeout,
		)
	}
	if first.SQL.Stats().MaxOpenConnections != 1 {
		t.Fatalf("max open connections = %d, want 1", first.SQL.Stats().MaxOpenConnections)
	}
	if err := first.Close(ctx); err != nil {
		t.Fatalf("close first catalog: %v", err)
	}

	reopened, err := Open(ctx, config.DatabaseConfig{Path: path})
	if err != nil {
		t.Fatalf("reopen catalog: %v", err)
	}
	defer reopened.Close(context.Background())
	if err := reopened.Migrate(ctx); err != nil {
		t.Fatalf("migrate reopened catalog: %v", err)
	}
	info, err := InspectCatalog(ctx, path)
	if err != nil {
		t.Fatalf("inspect reopened catalog: %v", err)
	}
	if info.ApplicationMigration != 1 || info.RiverMigration == 0 || info.LibraryID == "" {
		t.Fatalf("unexpected catalog identity: %+v", info)
	}
}

func TestOpenRejectsEphemeralAndCorruptCatalogs(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for _, path := range []string{"", ":memory:"} {
		if _, err := Open(ctx, config.DatabaseConfig{Path: path}); err == nil {
			t.Fatalf("Open(%q) unexpectedly succeeded", path)
		}
	}

	dir := secureTempDir(t)
	path := filepath.Join(dir, "library.sqlite3")
	if err := os.WriteFile(path, []byte("not a sqlite catalog"), 0o600); err != nil {
		t.Fatalf("write corrupt catalog: %v", err)
	}
	if _, err := Open(ctx, config.DatabaseConfig{Path: path}); err == nil {
		t.Fatal("open corrupt catalog unexpectedly succeeded")
	}
}

func secureTempDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatalf("secure temp directory: %v", err)
	}
	return dir
}
