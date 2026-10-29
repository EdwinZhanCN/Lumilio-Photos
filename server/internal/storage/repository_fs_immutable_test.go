package storage

import (
	"bytes"
	"errors"
	"os"
	"path"
	"path/filepath"
	"syscall"
	"testing"
)

// FAT32 and exFAT refuse hard links before the destination is consulted. The
// immutable publish then has to keep its contract by rename: the first
// complete file takes the name, an identical retry adopts it, a different
// retry is a conflict, and no temporary file is left behind. The
// filesystem-image CI jobs exercise the real refusal; this test stands in
// for it on the runner's filesystem. It is not parallel: it swaps the link
// hook for the whole package.
func TestRepositoryFSImmutablePrivateWritePublishesWhereHardLinksAreRefused(t *testing.T) {
	previous := linkImmutable
	linkImmutable = func(root *os.Root, oldname, newname string) error {
		return &os.LinkError{Op: "link", Old: oldname, New: newname, Err: syscall.EPERM}
	}
	t.Cleanup(func() { linkImmutable = previous })

	repository := createRepositoryFSTestRoot(t)
	repositoryFS := openRepositoryFSTestFS(t, repository)
	destination, err := ParsePrivateRepositoryPath(".lumilio/artifacts/fence/derivatives/asset-v1/small.webp")
	if err != nil {
		t.Fatal(err)
	}
	directory, err := ParsePrivateRepositoryPath(path.Dir(destination.String()))
	if err != nil {
		t.Fatal(err)
	}
	if err := repositoryFS.MkdirAllPrivate(directory, 0o755); err != nil {
		t.Fatal(err)
	}

	if written, err := repositoryFS.WritePrivateFileImmutable(destination, bytes.NewReader([]byte("first")), 0o644); err != nil || written != 5 {
		t.Fatalf("first immutable write without hard links = %d/%v", written, err)
	}
	if written, err := repositoryFS.WritePrivateFileImmutable(destination, bytes.NewReader([]byte("first")), 0o644); err != nil || written != 5 {
		t.Fatalf("identical retry without hard links = %d/%v", written, err)
	}
	if _, err := repositoryFS.WritePrivateFileImmutable(destination, bytes.NewReader([]byte("second")), 0o644); !errors.Is(err, ErrRepositoryImmutableConflict) {
		t.Fatalf("different retry without hard links = %v, want immutable conflict", err)
	}
	contents, err := repositoryFS.ReadPrivateFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != "first" {
		t.Fatalf("published content = %q, want the first complete file", contents)
	}
	entries, err := os.ReadDir(filepath.Join(repository.Path, filepath.FromSlash(directory.String())))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "small.webp" {
		names := make([]string, 0, len(entries))
		for _, entry := range entries {
			names = append(names, entry.Name())
		}
		t.Fatalf("artifact directory holds %v, want only small.webp", names)
	}
}
