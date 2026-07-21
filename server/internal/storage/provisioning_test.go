package storage

import (
	"os"
	"path/filepath"
	"testing"

	"server/config"
)

func TestEnsureRootLayout(t *testing.T) {
	root := filepath.Join(t.TempDir(), "storage")
	cfg := config.StorageConfig{Path: root}

	if err := EnsureRootLayout(cfg); err != nil {
		t.Fatalf("EnsureRootLayout: %v", err)
	}

	for _, dir := range []string{root, cfg.SecretsDir(), cfg.CloudDir()} {
		info, err := os.Stat(dir)
		if err != nil {
			t.Fatalf("expected %s to exist: %v", dir, err)
		}
		if !info.IsDir() {
			t.Fatalf("expected %s to be a directory", dir)
		}
	}

	// Owner-only access on the secret/cloud working areas, expressed in whatever
	// mechanism the platform actually enforces.
	for _, dir := range []string{cfg.SecretsDir(), cfg.CloudDir()} {
		requireDirectoryIsPrivate(t, dir)
	}

	// Idempotent: a second call is a no-op.
	if err := EnsureRootLayout(cfg); err != nil {
		t.Fatalf("EnsureRootLayout (second call): %v", err)
	}
}

func TestEnsureRootLayoutRejectsEmptyPath(t *testing.T) {
	if err := EnsureRootLayout(config.StorageConfig{}); err == nil {
		t.Fatal("expected error for empty storage path")
	}
}
