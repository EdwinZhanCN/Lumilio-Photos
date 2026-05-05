package handler

import (
	"path/filepath"
	"testing"
)

func TestResolveRepositoryCreatePathUsesStorageRoot(t *testing.T) {
	root := t.TempDir()

	got, err := resolveRepositoryCreatePath(root, "Family Photos")
	if err != nil {
		t.Fatalf("resolveRepositoryCreatePath returned error: %v", err)
	}

	want := filepath.Join(root, "family-photos")
	if got != want {
		t.Fatalf("resolveRepositoryCreatePath = %q, want %q", got, want)
	}
}

func TestRepositoryFolderNameFromNameKeepsUnicodeLetters(t *testing.T) {
	got := repositoryFolderNameFromName(" 家庭 照片! 2026 ")
	want := "家庭-照片-2026"
	if got != want {
		t.Fatalf("repositoryFolderNameFromName = %q, want %q", got, want)
	}
}
