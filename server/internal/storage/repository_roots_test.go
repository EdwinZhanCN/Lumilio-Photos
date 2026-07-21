package storage

import (
	"errors"
	"path/filepath"
	"runtime"
	"testing"
)

func TestPathIsStrictlyInside(t *testing.T) {
	base := canonicalTempDir(t)
	root := filepath.Join(base, "photos")
	cases := []struct {
		name string
		path string
		want bool
	}{
		{name: "direct child", path: filepath.Join(root, "family"), want: true},
		{name: "nested child", path: filepath.Join(root, "family", "2026"), want: true},
		{name: "same path", path: root, want: false},
		{name: "parent", path: base, want: false},
		{name: "prefix sibling", path: filepath.Join(base, "photos-archive"), want: false},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := pathIsStrictlyInside(root, test.path); got != test.want {
				t.Fatalf("pathIsStrictlyInside(%q, %q) = %v, want %v", root, test.path, got, test.want)
			}
		})
	}
}

func TestRelocatedRepositoryPath(t *testing.T) {
	base := canonicalTempDir(t)
	oldRoot := filepath.Join(base, "old")
	newRoot := filepath.Join(base, "new")
	repositoryPath := filepath.Join(oldRoot, "family", "2026")

	got, err := relocatedRepositoryPath(oldRoot, newRoot, repositoryPath)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(newRoot, "family", "2026"); got != want {
		t.Fatalf("relocated path = %q, want %q", got, want)
	}
	if _, err := relocatedRepositoryPath(oldRoot, newRoot, filepath.Join(base, "old-copy")); !errors.Is(err, ErrRepositoryRootInvalid) {
		t.Fatalf("prefix sibling error = %v, want ErrRepositoryRootInvalid", err)
	}
}

func TestRelocatedRepositoryPathAcrossWindowsDriveLetters(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows drive-letter semantics require a Windows filesystem runtime")
	}
	got, err := relocatedRepositoryPath(`D:\Lumilio`, `E:\Lumilio`, `D:\Lumilio\family`)
	if err != nil {
		t.Fatal(err)
	}
	if want := `E:\Lumilio\family`; got != want {
		t.Fatalf("relocated path = %q, want %q", got, want)
	}
}
