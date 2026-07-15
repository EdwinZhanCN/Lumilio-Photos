package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureSecretIsIdempotentAndPrivate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "bootstrap")
	if err := ensureSecret(path); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) == 0 {
		t.Fatal("generated secret is empty")
	}
	if err := ensureSecret(path); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(second) != string(first) {
		t.Fatal("idempotent initialization replaced the secret")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("secret permissions = %o", info.Mode().Perm())
	}
}
