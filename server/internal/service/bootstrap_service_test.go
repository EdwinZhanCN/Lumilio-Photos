package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"server/config"
	"server/internal/db"
)

func TestBootstrapPhaseStaysAvailableWhileWriterIsBusy(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	catalogDirectory := t.TempDir()
	if err := os.Chmod(catalogDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	catalog, err := db.Open(ctx, config.DatabaseConfig{Path: filepath.Join(catalogDirectory, "catalog.sqlite3")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = catalog.Close(context.Background()) })
	if err := catalog.Migrate(ctx); err != nil {
		t.Fatal(err)
	}

	writer, err := catalog.SQL.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Rollback()
	if _, err := writer.ExecContext(ctx, `UPDATE system_state SET updated_at = updated_at WHERE id = 1`); err != nil {
		t.Fatal(err)
	}

	bootstrap := NewBootstrapServiceWithReader(catalog.Queries, catalog.ReaderQueries)
	readCtx, readCancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer readCancel()
	phase, err := bootstrap.Phase(readCtx)
	if err != nil {
		t.Fatalf("bootstrap phase waited for the writer connection: %v", err)
	}
	if phase != BootstrapPhaseCatalogReady {
		t.Fatalf("bootstrap phase = %q, want %q", phase, BootstrapPhaseCatalogReady)
	}
}

func TestBootstrapReadyDoesNotRegressWhenStorageFactsDisappear(t *testing.T) {
	ctx := context.Background()
	catalogDirectory := t.TempDir()
	if err := os.Chmod(catalogDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	catalog, err := db.Open(ctx, config.DatabaseConfig{Path: filepath.Join(catalogDirectory, "catalog.sqlite3")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = catalog.Close(context.Background()) })
	if err := catalog.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := catalog.Queries.SetBootstrapPhase(ctx, BootstrapPhaseReady); err != nil {
		t.Fatal(err)
	}

	bootstrap := NewBootstrapService(catalog.Queries)
	phase, err := bootstrap.Reconcile(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if phase != BootstrapPhaseReady {
		t.Fatalf("reconciled phase = %q, want ready", phase)
	}
	stored, err := catalog.Queries.GetSystemState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if stored.BootstrapPhase != BootstrapPhaseReady {
		t.Fatalf("stored phase regressed to %q", stored.BootstrapPhase)
	}
}
