package storage

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestPrepareRepositoryInitializationTargetAcceptsExistingEmptyDirectory(t *testing.T) {
	parent := t.TempDir()
	target := filepath.Join(parent, "Mounted Media")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatalf("create target: %v", err)
	}

	created, err := prepareRepositoryInitializationTarget(target)
	if err != nil {
		t.Fatalf("prepare target: %v", err)
	}
	if created {
		t.Fatal("prepare target reported a pre-existing directory as created")
	}
	entries, err := os.ReadDir(target)
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("probe left %d entries in target", len(entries))
	}
}

func TestPrepareRepositoryInitializationTargetRejectsNonEmptyDirectory(t *testing.T) {
	target := filepath.Join(t.TempDir(), "Mounted Media")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatalf("create target: %v", err)
	}
	sentinel := filepath.Join(target, "existing-media.jpg")
	if err := os.WriteFile(sentinel, []byte("preserve me"), 0o644); err != nil {
		t.Fatalf("create sentinel: %v", err)
	}

	created, err := prepareRepositoryInitializationTarget(target)
	if created {
		t.Fatal("prepare target reported a pre-existing directory as created")
	}
	if !errors.Is(err, ErrRepositoryTargetNotEmpty) {
		t.Fatalf("prepare target error = %v, want ErrRepositoryTargetNotEmpty", err)
	}
	if _, err := os.Stat(sentinel); err != nil {
		t.Fatalf("sentinel changed after rejected prepare: %v", err)
	}
}

func TestCleanupRepositoryInitializationTargetPreservesExistingMount(t *testing.T) {
	target := filepath.Join(t.TempDir(), "Mounted Media")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatalf("create target: %v", err)
	}
	for _, name := range []string{
		DefaultStructure.ConfigFile,
		DefaultStructure.SystemDir,
		DefaultStructure.InboxDir,
	} {
		path := filepath.Join(target, name)
		if name == DefaultStructure.ConfigFile {
			if err := os.WriteFile(path, []byte("partial"), 0o644); err != nil {
				t.Fatalf("create config: %v", err)
			}
			continue
		}
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatalf("create generated directory: %v", err)
		}
	}
	sentinel := filepath.Join(target, "created-concurrently.txt")
	if err := os.WriteFile(sentinel, []byte("preserve me"), 0o644); err != nil {
		t.Fatalf("create sentinel: %v", err)
	}

	if err := cleanupRepositoryInitializationTarget(target, false); err != nil {
		t.Fatalf("cleanup target: %v", err)
	}
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("pre-existing target was removed: %v", err)
	}
	if _, err := os.Stat(sentinel); err != nil {
		t.Fatalf("unowned file was removed: %v", err)
	}
	for _, name := range []string{
		DefaultStructure.ConfigFile,
		DefaultStructure.SystemDir,
		DefaultStructure.InboxDir,
	} {
		if _, err := os.Stat(filepath.Join(target, name)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("generated path %q still exists or returned unexpected error: %v", name, err)
		}
	}
}

func TestCleanupRepositoryInitializationTargetRemovesOwnedDirectory(t *testing.T) {
	target := filepath.Join(t.TempDir(), "New Repository")
	created, err := prepareRepositoryInitializationTarget(target)
	if err != nil {
		t.Fatalf("prepare target: %v", err)
	}
	if !created {
		t.Fatal("prepare target did not report newly created directory")
	}
	if err := os.Mkdir(filepath.Join(target, DefaultStructure.InboxDir), 0o755); err != nil {
		t.Fatalf("create generated directory: %v", err)
	}

	if err := cleanupRepositoryInitializationTarget(target, true); err != nil {
		t.Fatalf("cleanup target: %v", err)
	}
	if _, err := os.Stat(target); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("owned target still exists or returned unexpected error: %v", err)
	}
}
