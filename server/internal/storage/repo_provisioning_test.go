package storage

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"server/internal/db/dbtypes"
)

func TestResolveRepositoryCreatePathUsesStorageRoot(t *testing.T) {
	root := canonicalTempDir(t)

	got, err := resolveRepositoryCreatePath(root, "Family Photos", dbtypes.RepoRoleRegular)
	if err != nil {
		t.Fatalf("resolveRepositoryCreatePath returned error: %v", err)
	}

	want := filepath.Join(root, "Family Photos")
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
		{name: "hyphen and underscore", value: "Media_2026-Archive"},
		{name: "slash", value: "Family/Media", wantErr: true},
		{name: "backslash", value: `Family\Media`, wantErr: true},
		{name: "dot", value: "Family.Media", wantErr: true},
		{name: "punctuation", value: "Family (Media)", wantErr: true},
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

func TestResolveRepositoryCreatePathRejectsCaseInsensitiveSiblingConflict(t *testing.T) {
	root := canonicalTempDir(t)
	if err := os.Mkdir(filepath.Join(root, "Family Media"), 0o755); err != nil {
		t.Fatalf("create sibling directory: %v", err)
	}

	_, err := resolveRepositoryCreatePath(root, "family media", dbtypes.RepoRoleRegular)
	if !errors.Is(err, ErrRepositoryNameConflict) {
		t.Fatalf("resolveRepositoryCreatePath error = %v, want ErrRepositoryNameConflict", err)
	}
}
