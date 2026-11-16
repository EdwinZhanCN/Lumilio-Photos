package storage

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"

	"github.com/google/uuid"

	"server/internal/storage/layout"
)

// ErrRepositoryDestinationExists reports a move whose destination is taken;
// a lifecycle move never replaces a file.
var ErrRepositoryDestinationExists = errors.New("repository move destination already exists")

// Trash paths (files/<trash_id>/<name>, info/<trash_id>.json) come from layout.

// TrashFileDirectory is the directory that holds one trashed file.
func TrashFileDirectory(trashID uuid.UUID) (RepositoryPath, error) { return layout.TrashDir(trashID) }

// TrashFilePath is where a trashed file lives, under its original name.
func TrashFilePath(trashID uuid.UUID, name string) (RepositoryPath, error) {
	return layout.TrashFile(trashID, name)
}

// TrashInfoDirectory holds the trash's info sidecars.
func TrashInfoDirectory() RepositoryPath { return layout.TrashInfoDir() }

// TrashInfoPath is the info sidecar of one trashed file.
func TrashInfoPath(trashID uuid.UUID) (RepositoryPath, error) { return layout.TrashInfo(trashID) }

// MoveNoReplace moves one file between the user tree and Lumilio's private
// tree of the same repository: into the trash or back out of it. It never
// replaces an existing entry and never copies, so the file stays on the
// repository's volume; a move across volumes fails.
//
// The move links the destination and then unlinks the source, which is
// atomic about the destination on every volume with hard links. On a volume
// without them (FAT, exFAT, some network shares) it checks the destination
// and renames; only a file created at the destination inside that window
// could be replaced.
func (r *RepositoryFS) MoveNoReplace(source, destination RepositoryPath) error {
	if !(source.IsMedia() && destination.IsPrivate()) && !(source.IsPrivate() && destination.IsMedia()) {
		return ErrRepositoryPathNamespace
	}
	sourceLocal, err := source.Local()
	if err != nil {
		return err
	}
	destinationLocal, err := destination.Local()
	if err != nil {
		return err
	}
	root, done, err := r.withRoot()
	if err != nil {
		return err
	}
	defer done()

	info, err := root.Lstat(sourceLocal)
	if err != nil {
		return classifyRepositoryEntryError(source.String(), err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%w: %s", ErrRepositoryEntryUnsupported, source.String())
	}
	linkErr := root.Link(sourceLocal, destinationLocal)
	switch {
	case linkErr == nil:
		if err := root.Remove(sourceLocal); err != nil {
			// The source is still in place; take the second name back.
			_ = root.Remove(destinationLocal)
			return classifyRepositoryEntryError(source.String(), err)
		}
	case errors.Is(linkErr, fs.ErrExist):
		return fmt.Errorf("%w: %s", ErrRepositoryDestinationExists, destination.String())
	default:
		if _, err := root.Lstat(destinationLocal); err == nil {
			return fmt.Errorf("%w: %s", ErrRepositoryDestinationExists, destination.String())
		} else if !errors.Is(err, fs.ErrNotExist) {
			return classifyRepositoryEntryError(destination.String(), err)
		}
		if err := root.Rename(sourceLocal, destinationLocal); err != nil {
			return classifyRepositoryEntryError(source.String(), err)
		}
	}
	if err := syncParentDirectory(root, destination); err != nil {
		return err
	}
	return syncParentDirectory(root, source)
}

// syncParentDirectory makes a rename in a path's directory durable; the
// repository root has no repository-relative name of its own.
func syncParentDirectory(root *os.Root, repositoryPath RepositoryPath) error {
	directory := path.Dir(repositoryPath.String())
	if directory != "." {
		return syncRootDirectory(root, directory)
	}
	opened, err := root.Open(".")
	if err != nil {
		return err
	}
	defer opened.Close()
	return syncRepositoryDirectory(opened)
}

// DropDuplicateLink removes drop only when it names the same file as keep. A
// move interrupted between its link and its unlink leaves the file under
// both names; recovery keeps one and never removes a different file.
func (r *RepositoryFS) DropDuplicateLink(keep, drop RepositoryPath) error {
	keepLocal, err := keep.Local()
	if err != nil {
		return err
	}
	dropLocal, err := drop.Local()
	if err != nil {
		return err
	}
	root, done, err := r.withRoot()
	if err != nil {
		return err
	}
	defer done()
	kept, err := root.Lstat(keepLocal)
	if err != nil {
		return err
	}
	dropped, err := root.Lstat(dropLocal)
	if err != nil {
		return err
	}
	if !os.SameFile(kept, dropped) {
		return fmt.Errorf("%w: %s and %s are different files", ErrRepositoryDestinationExists, keep.String(), drop.String())
	}
	if err := root.Remove(dropLocal); err != nil {
		return err
	}
	return syncParentDirectory(root, drop)
}

// SameFile reports whether two repository paths name the same file.
func (r *RepositoryFS) SameFile(left, right RepositoryPath) (bool, error) {
	leftLocal, err := left.Local()
	if err != nil {
		return false, err
	}
	rightLocal, err := right.Local()
	if err != nil {
		return false, err
	}
	root, done, err := r.withRoot()
	if err != nil {
		return false, err
	}
	defer done()
	leftInfo, err := root.Lstat(leftLocal)
	if err != nil {
		return false, err
	}
	rightInfo, err := root.Lstat(rightLocal)
	if err != nil {
		return false, err
	}
	return os.SameFile(leftInfo, rightInfo), nil
}

// Exists reports whether a repository path names any directory entry.
func (r *RepositoryFS) Exists(repositoryPath RepositoryPath) (bool, error) {
	local, err := repositoryPath.Local()
	if err != nil {
		return false, err
	}
	root, done, err := r.withRoot()
	if err != nil {
		return false, err
	}
	defer done()
	if _, err := root.Lstat(local); err == nil {
		return true, nil
	} else if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	} else {
		return false, classifyRepositoryEntryError(repositoryPath.String(), err)
	}
}
