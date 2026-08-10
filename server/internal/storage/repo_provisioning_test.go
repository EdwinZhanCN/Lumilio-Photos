package storage

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"server/internal/db/dbtypes"
	"server/internal/storage/repocfg"
)

func TestCreateRepositoryClassifiesAllExistingMarkerConflictsFromDisk(t *testing.T) {
	_, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()
	rootPath := filepath.Join(t.TempDir(), "default")
	initializeDefaultStorageForTest(t, manager, rootPath)
	root, err := manager.queries.GetDefaultRepositoryRoot(ctx)
	if err != nil {
		t.Fatal(err)
	}

	validPath := filepath.Join(rootPath, "valid-existing")
	if err := os.Mkdir(validPath, 0o755); err != nil {
		t.Fatal(err)
	}
	validMarker := repocfg.NewRepositoryConfig("Existing")
	if err := validMarker.SaveConfigToFile(validPath); err != nil {
		t.Fatal(err)
	}
	_, err = manager.CreateRepository(ctx, CreateRepositorySpec{
		RequestID: "create-valid-existing", Actor: "test", Name: "Existing",
		DirectoryName: "valid-existing", Role: dbtypes.RepoRoleRegular, RootID: root.RootID.String(),
	})
	var existing *ExistingRepositoryFoundError
	if !errors.As(err, &existing) || existing.RepositoryID != validMarker.ID {
		t.Fatalf("valid existing marker error = %#v (%v)", existing, err)
	}

	invalidPath := filepath.Join(rootPath, "invalid-existing")
	if err := os.Mkdir(invalidPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(invalidPath, ".lumiliorepo"), []byte("invalid: ["), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = manager.CreateRepository(ctx, CreateRepositorySpec{
		RequestID: "create-invalid-existing", Actor: "test", Name: "Invalid",
		DirectoryName: "invalid-existing", Role: dbtypes.RepoRoleRegular, RootID: root.RootID.String(),
	})
	var invalid *RepositoryMarkerInvalidError
	if !errors.As(err, &invalid) {
		t.Fatalf("invalid marker error = %#v (%v)", invalid, err)
	}

	registered, err := manager.CreateRepository(ctx, CreateRepositorySpec{
		RequestID: "create-conflict-source", Actor: "test", Name: "Source",
		DirectoryName: "conflict-source", Role: dbtypes.RepoRoleRegular, RootID: root.RootID.String(),
	})
	if err != nil {
		t.Fatal(err)
	}
	copyPath := filepath.Join(rootPath, "registered-copy")
	if err := os.Mkdir(copyPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := registered.Repository.Config.SaveConfigToFile(copyPath); err != nil {
		t.Fatal(err)
	}
	_, err = manager.CreateRepository(ctx, CreateRepositorySpec{
		RequestID: "create-registered-copy", Actor: "test", Name: "Copy",
		DirectoryName: "registered-copy", Role: dbtypes.RepoRoleRegular, RootID: root.RootID.String(),
	})
	var conflict *RepositoryConflictError
	if !errors.As(err, &conflict) || len(conflict.Actions) != 1 || conflict.Actions[0] != "copy" {
		t.Fatalf("registered identity conflict = %#v (%v)", conflict, err)
	}
}

func TestResolveRepositoryCreatePathUsesExplicitStorageFolder(t *testing.T) {
	root := canonicalTempDir(t)

	got, err := resolveRepositoryCreatePath(root, "family-media", dbtypes.RepoRoleRegular)
	if err != nil {
		t.Fatalf("resolveRepositoryCreatePath returned error: %v", err)
	}

	want := filepath.Join(root, "family-media")
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

func TestValidateRepositoryName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{name: "Chinese", value: "家庭媒体"},
		{name: "preserves case and spaces", value: "Family Media 2026"},
		{name: "punctuation", value: "Family.Media (2026)!"},
		{name: "leading space", value: " Family", wantErr: true},
		{name: "trailing space", value: "Family ", wantErr: true},
		{name: "empty", value: "", wantErr: true},
		{name: "too many characters", value: strings.Repeat("a", maxRepositoryNameRunes+1), wantErr: true},
		{name: "maximum three byte letters", value: strings.Repeat("界", 80), wantErr: false},
		{name: "over byte limit", value: strings.Repeat("𐐀", 61), wantErr: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateRepositoryName(tt.value)
			if tt.wantErr {
				if !errors.Is(err, ErrInvalidRepositoryName) {
					t.Fatalf("ValidateRepositoryName(%q) error = %v, want ErrInvalidRepositoryName", tt.value, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("ValidateRepositoryName(%q) returned error: %v", tt.value, err)
			}
		})
	}
}

func TestValidateRepositoryDirectoryName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{name: "Chinese", value: "家庭媒体"},
		{name: "preserves case and spaces", value: "Family Media 2026"},
		{name: "hyphen and underscore", value: "Media_2026-Archive"},
		{name: "slash", value: "Family/Media", wantErr: true},
		{name: "backslash", value: `Family\Media`, wantErr: true},
		{name: "dot", value: "Family.Media", wantErr: true},
		{name: "punctuation", value: "Family (Media)", wantErr: true},
		{name: "empty", value: "", wantErr: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateRepositoryDirectoryName(tt.value)
			if tt.wantErr && !errors.Is(err, ErrInvalidRepositoryDirectory) {
				t.Fatalf("ValidateRepositoryDirectoryName(%q) error = %v, want ErrInvalidRepositoryDirectory", tt.value, err)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("ValidateRepositoryDirectoryName(%q) returned error: %v", tt.value, err)
			}
		})
	}
}

func TestResolveRepositoryCreatePathRejectsCaseInsensitiveSiblingConflict(t *testing.T) {
	root := canonicalTempDir(t)
	if err := os.Mkdir(filepath.Join(root, "Family Media"), 0o755); err != nil {
		t.Fatalf("create sibling directory: %v", err)
	}

	_, err := resolveRepositoryCreatePath(root, "family media", dbtypes.RepoRoleRegular)
	if !errors.Is(err, ErrRepositoryDirectoryConflict) {
		t.Fatalf("resolveRepositoryCreatePath error = %v, want ErrRepositoryDirectoryConflict", err)
	}
}
