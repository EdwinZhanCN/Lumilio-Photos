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

	// Owner-only permissions on the secret/cloud working areas.
	for _, dir := range []string{cfg.SecretsDir(), cfg.CloudDir()} {
		info, _ := os.Stat(dir)
		if perm := info.Mode().Perm(); perm != 0o700 {
			t.Fatalf("%s perm = %o, want 0700", dir, perm)
		}
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
