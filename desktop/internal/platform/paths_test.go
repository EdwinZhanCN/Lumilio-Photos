package platform

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPathsUsePrivateSchemaAndRepairDirectoryPrivacy(t *testing.T) {
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
	private, err := directoryIsPrivate(paths.SecretsDir)
	if err != nil {
		t.Fatal(err)
	}
	if !private {
		t.Fatal("secrets directory is not private")
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

func TestWriteAtomicWritesAndReplacesFile(t *testing.T) {
	target := filepath.Join(t.TempDir(), "metadata.json")
	for _, contents := range []string{"first", "second"} {
		if err := WriteAtomic(target, []byte(contents), 0o600); err != nil {
			t.Fatalf("write %q: %v", contents, err)
		}
		data, err := os.ReadFile(target)
		if err != nil {
			t.Fatalf("read %q: %v", contents, err)
		}
		if string(data) != contents {
			t.Fatalf("contents = %q, want %q", data, contents)
		}
	}
}
