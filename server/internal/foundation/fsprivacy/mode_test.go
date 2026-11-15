package fsprivacy

import (
	"os"
	"path/filepath"
	"testing"
)

func TestApplyPrivateModes(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "private")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := ApplyDirectoryMode(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	private, err := IsPrivate(dir)
	if err != nil || !private {
		t.Fatalf("directory private = %v, err = %v", private, err)
	}

	file := filepath.Join(dir, "secret")
	if err := os.WriteFile(file, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ApplyFileMode(file, 0o600); err != nil {
		t.Fatal(err)
	}
	private, err = IsPrivate(file)
	if err != nil || !private {
		t.Fatalf("file private = %v, err = %v", private, err)
	}
}
