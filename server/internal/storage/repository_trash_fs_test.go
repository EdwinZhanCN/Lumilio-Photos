package storage

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
)

func TestMoveNoReplaceMovesIntoTheTrashAndBack(t *testing.T) {
	t.Parallel()
	repository := createRepositoryFSTestRoot(t)
	repositoryFS := openRepositoryFSTestFS(t, repository)
	writeRepositoryFSTestFile(t, repository.Path, "trips/a.jpg", []byte("photo"))
	trashID := uuid.New()
	directory, err := TrashFileDirectory(trashID)
	if err != nil {
		t.Fatal(err)
	}
	if err := repositoryFS.MkdirAllPrivate(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	trashed, err := TrashFilePath(trashID, "a.jpg")
	if err != nil {
		t.Fatal(err)
	}
	original := mustUserMediaPath(t, "trips/a.jpg")

	if err := repositoryFS.MoveNoReplace(original, trashed); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(repository.Path, "trips", "a.jpg")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("source still exists after the move: %v", err)
	}
	if contents, err := os.ReadFile(filepath.Join(repository.Path, filepath.FromSlash(trashed.String()))); err != nil || string(contents) != "photo" {
		t.Fatalf("trashed file = %q, %v", contents, err)
	}
	if err := repositoryFS.MoveNoReplace(trashed, original); err != nil {
		t.Fatal(err)
	}
	if contents, err := os.ReadFile(filepath.Join(repository.Path, "trips", "a.jpg")); err != nil || string(contents) != "photo" {
		t.Fatalf("restored file = %q, %v", contents, err)
	}
}

func TestMoveNoReplaceNeverReplacesAndStaysBetweenNamespaces(t *testing.T) {
	t.Parallel()
	repository := createRepositoryFSTestRoot(t)
	repositoryFS := openRepositoryFSTestFS(t, repository)
	writeRepositoryFSTestFile(t, repository.Path, "a.jpg", []byte("taken"))
	writeRepositoryFSTestFile(t, repository.Path, ".lumilio/trash/files/x/a.jpg", []byte("trashed"))
	trashed, err := ParsePrivateRepositoryPath(".lumilio/trash/files/x/a.jpg")
	if err != nil {
		t.Fatal(err)
	}
	original := mustUserMediaPath(t, "a.jpg")

	if err := repositoryFS.MoveNoReplace(trashed, original); !errors.Is(err, ErrRepositoryDestinationExists) {
		t.Fatalf("move onto a taken path = %v, want ErrRepositoryDestinationExists", err)
	}
	for relative, want := range map[string]string{"a.jpg": "taken", ".lumilio/trash/files/x/a.jpg": "trashed"} {
		if contents, err := os.ReadFile(filepath.Join(repository.Path, filepath.FromSlash(relative))); err != nil || string(contents) != want {
			t.Fatalf("%s = %q, %v; want %q untouched", relative, contents, err, want)
		}
	}
	other := mustUserMediaPath(t, "b.jpg")
	if err := repositoryFS.MoveNoReplace(original, other); !errors.Is(err, ErrRepositoryPathNamespace) {
		t.Fatalf("media-to-media move = %v, want ErrRepositoryPathNamespace", err)
	}
}

func TestDropDuplicateLinkRemovesOnlyTheSameFile(t *testing.T) {
	t.Parallel()
	repository := createRepositoryFSTestRoot(t)
	repositoryFS := openRepositoryFSTestFS(t, repository)
	writeRepositoryFSTestFile(t, repository.Path, ".lumilio/trash/files/x/a.jpg", []byte("photo"))
	if err := os.Link(filepath.Join(repository.Path, ".lumilio", "trash", "files", "x", "a.jpg"), filepath.Join(repository.Path, "a.jpg")); err != nil {
		t.Skipf("hard links unavailable: %v", err)
	}
	writeRepositoryFSTestFile(t, repository.Path, "b.jpg", []byte("photo"))
	trashed, err := ParsePrivateRepositoryPath(".lumilio/trash/files/x/a.jpg")
	if err != nil {
		t.Fatal(err)
	}
	if err := repositoryFS.DropDuplicateLink(trashed, mustUserMediaPath(t, "b.jpg")); !errors.Is(err, ErrRepositoryDestinationExists) {
		t.Fatalf("dropping a different file = %v, want a refusal", err)
	}
	if err := repositoryFS.DropDuplicateLink(trashed, mustUserMediaPath(t, "a.jpg")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(repository.Path, "a.jpg")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("duplicate link still exists: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(repository.Path, "b.jpg")); err != nil {
		t.Fatalf("a different file was removed: %v", err)
	}
}
