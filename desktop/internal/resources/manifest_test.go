package resources

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestManifestVerifyAndMaterialize(t *testing.T) {
	source := t.TempDir()
	data := []byte("signed resource payload")
	hash := sha256.Sum256(data)
	path := filepath.Join(source, "tools", "probe")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o700); err != nil {
		t.Fatal(err)
	}
	manifest, err := Load([]byte(`{"schemaVersion":1,"entries":[{"logicalName":"probe","platform":"` + runtime.GOOS + `","arch":"` + runtime.GOARCH + `","version":"1","sha256":"` + hex.EncodeToString(hash[:]) + `","mode":448,"path":"tools/probe"}]}`))
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}
	if err := Verify(source, manifest); err != nil {
		t.Fatalf("verify: %v", err)
	}
	destination := filepath.Join(t.TempDir(), "version")
	if err := Materialize(source, destination, manifest); err != nil {
		t.Fatalf("materialize: %v", err)
	}
	if err := Verify(destination, manifest); err != nil {
		t.Fatalf("verify materialized: %v", err)
	}
	if err := Materialize(source, destination, manifest); err == nil {
		t.Fatal("materialize overwrote an existing version")
	}
}

func TestManifestRejectsPathEscape(t *testing.T) {
	_, err := Load([]byte(`{"schemaVersion":1,"entries":[{"logicalName":"bad","platform":"` + runtime.GOOS + `","arch":"` + runtime.GOARCH + `","version":"1","sha256":"0000000000000000000000000000000000000000000000000000000000000000","mode":420,"path":"../bad"}]}`))
	if err != nil {
		t.Fatalf("load should defer path validation to root verification: %v", err)
	}
	if err := Verify(t.TempDir(), Manifest{SchemaVersion: SchemaVersion, Entries: []Entry{{LogicalName: "bad", Platform: runtime.GOOS, Arch: runtime.GOARCH, Version: "1", SHA256: "0000000000000000000000000000000000000000000000000000000000000000", Mode: 420, Path: "../bad"}}}); err == nil {
		t.Fatal("path escape was accepted")
	}
}
