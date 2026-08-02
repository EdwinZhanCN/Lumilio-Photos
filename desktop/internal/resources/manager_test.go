package resources

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"testing/fstest"

	"desktop/internal/platform"
)

func TestManagerMaterializesAndReusesPackagedPayload(t *testing.T) {
	payload := []byte("packaged probe")
	hash := sha256.Sum256(payload)
	manifest := Manifest{SchemaVersion: SchemaVersion, Entries: []Entry{{
		LogicalName: "probe", Platform: runtime.GOOS, Arch: runtime.GOARCH,
		Version: "2026.07.31", SHA256: hex.EncodeToString(hash[:]), Mode: 0o700, Path: "tools/probe",
	}}}
	paths, err := platform.NewPaths(filepath.Join(t.TempDir(), "app-data"))
	if err != nil {
		t.Fatal(err)
	}
	if err := paths.Ensure(); err != nil {
		t.Fatal(err)
	}
	source := fstest.MapFS{"payload/tools/probe": &fstest.MapFile{Data: payload, Mode: 0o700}}
	manager := NewManager(paths, source, "payload", manifest)
	if err := manager.Ensure(); err != nil {
		t.Fatalf("ensure resources: %v", err)
	}
	pointer, err := manager.loadPointer()
	if err != nil {
		t.Fatalf("load current pointer: %v", err)
	}
	materialized := filepath.Join(paths.ResourcesVersions, pointer.Directory, "tools", "probe")
	if got, err := os.ReadFile(materialized); err != nil || string(got) != string(payload) {
		t.Fatalf("materialized payload = %q, err %v", got, err)
	}
	if _, err := os.Stat(paths.ResourcesInstall); !os.IsNotExist(err) {
		t.Fatalf("install journal still exists: %v", err)
	}
	before := pointer.Directory
	if err := manager.Ensure(); err != nil {
		t.Fatalf("reuse resources: %v", err)
	}
	after, err := manager.loadPointer()
	if err != nil {
		t.Fatal(err)
	}
	if after.Directory != before {
		t.Fatalf("resource directory changed on reuse: %q -> %q", before, after.Directory)
	}
}

func TestManagerReconcilesStagingJournal(t *testing.T) {
	paths, err := platform.NewPaths(filepath.Join(t.TempDir(), "app-data"))
	if err != nil {
		t.Fatal(err)
	}
	if err := paths.Ensure(); err != nil {
		t.Fatal(err)
	}
	manager := NewManager(paths, fstest.MapFS{}, "payload", Manifest{SchemaVersion: SchemaVersion})
	staging := filepath.Join(paths.ResourcesVersions, ".staging-interrupted")
	if err := os.MkdirAll(staging, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := manager.writeJournal(journal{
		Phase: "staging", Version: "1", Platform: runtime.GOOS, Arch: runtime.GOARCH,
		ManifestSHA256: "sha256:0000000000000000000000000000000000000000000000000000000000000000", Directory: "target", Staging: ".staging-interrupted",
	}); err != nil {
		t.Fatal(err)
	}
	if err := manager.Reconcile(); err != nil {
		t.Fatalf("reconcile staging: %v", err)
	}
	if _, err := os.Stat(staging); !os.IsNotExist(err) {
		t.Fatalf("staging tree still exists: %v", err)
	}
	if _, err := os.Stat(paths.ResourcesInstall); !os.IsNotExist(err) {
		t.Fatalf("staging journal still exists: %v", err)
	}
}

func TestManagerRejectsJournalPathEscape(t *testing.T) {
	paths, err := platform.NewPaths(filepath.Join(t.TempDir(), "app-data"))
	if err != nil {
		t.Fatal(err)
	}
	if err := paths.Ensure(); err != nil {
		t.Fatal(err)
	}
	manager := NewManager(paths, fstest.MapFS{}, "payload", Manifest{SchemaVersion: SchemaVersion})
	if err := os.WriteFile(paths.ResourcesInstall, []byte(`{"schemaVersion":1,"phase":"staging","version":"1","platform":"darwin","arch":"arm64","manifestSHA256":"sha256:x","directory":"../outside","staging":".staging"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := manager.Reconcile(); err == nil {
		t.Fatal("journal path escape was accepted")
	}
}
