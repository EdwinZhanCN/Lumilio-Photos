package storage

import (
	"path/filepath"
	"testing"

	"server/internal/db/dbtypes"
)

func TestResolveRepositoryCreatePathUsesStorageRoot(t *testing.T) {
	root := canonicalTempDir(t)

	got, err := resolveRepositoryCreatePath(root, "Family Photos", dbtypes.RepoRoleRegular)
	if err != nil {
		t.Fatalf("resolveRepositoryCreatePath returned error: %v", err)
	}

	want := filepath.Join(root, "family-photos")
	if got != want {
		t.Fatalf("resolveRepositoryCreatePath = %q, want %q", got, want)
	}
}

func TestResolveRepositoryCreatePathUsesPrimaryFolderForPrimaryRole(t *testing.T) {
	root := canonicalTempDir(t)

	got, err := resolveRepositoryCreatePath(root, "Library", dbtypes.RepoRolePrimary)
	if err != nil {
		t.Fatalf("resolveRepositoryCreatePath returned error: %v", err)
	}

	want := filepath.Join(root, "primary")
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
