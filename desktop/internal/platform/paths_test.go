package platform

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPathsUsePrivateSchemaAndRepairDirectoryMode(t *testing.T) {
	root := filepath.Join(t.TempDir(), "app-data")
	paths, err := NewPaths(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := paths.Ensure(); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(paths.SecretsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := paths.Ensure(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(paths.SecretsDir)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o700 {
		t.Fatalf("secrets mode = %o, want 700", info.Mode().Perm())
	}
}

func TestWriteAtomicRejectsSymlink(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "metadata.json")
	if err := os.Symlink(filepath.Join(root, "outside"), target); err != nil {
		t.Fatal(err)
	}
	if err := WriteAtomic(target, []byte("{}"), 0o600); err == nil {
		t.Fatal("WriteAtomic accepted symlink")
	}
}
