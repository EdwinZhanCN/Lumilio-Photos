package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"server/config"
	"server/internal/db"
)

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
