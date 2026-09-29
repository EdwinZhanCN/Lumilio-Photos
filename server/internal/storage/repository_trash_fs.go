package storage

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"

	"github.com/google/uuid"
)

// RepositoryTrashDirectory is the per-repository trash. Deleting an Asset
// moves its files into files/<trash_id>/<name> and writes a self-describing
// sidecar to info/<trash_id>.json, so the Trash view can be rebuilt from the
// repository alone.
const RepositoryTrashDirectory = ".lumilio/trash"

// ErrRepositoryDestinationExists reports a move whose destination is taken;
// a lifecycle move never replaces a file.
var ErrRepositoryDestinationExists = errors.New("repository move destination already exists")

// TrashFileDirectory is the directory that holds one trashed file.
func TrashFileDirectory(trashID uuid.UUID) (RepositoryPath, error) {
	if trashID == uuid.Nil {
		return RepositoryPath{}, fmt.Errorf("%w: zero trash ID", ErrRepositoryPathInvalid)
	}
	return ParsePrivateRepositoryPath(path.Join(RepositoryTrashDirectory, "files", trashID.String()))
}

// TrashFilePath is where a trashed file lives, under its original name.
func TrashFilePath(trashID uuid.UUID, name string) (RepositoryPath, error) {
	directory, err := TrashFileDirectory(trashID)
	if err != nil {
		return RepositoryPath{}, err
	}
	if name == "" || name == "." || name == ".." || path.Base(name) != name {
		return RepositoryPath{}, fmt.Errorf("%w: trashed file name %q", ErrRepositoryPathInvalid, name)
	}
	return ParsePrivateRepositoryPath(path.Join(directory.String(), name))
}

// TrashInfoDirectory holds the trash's info sidecars.
func TrashInfoDirectory() RepositoryPath {
	return RepositoryPath{value: path.Join(RepositoryTrashDirectory, "info"), namespace: namespacePrivate}
}

// TrashInfoPath is the info sidecar of one trashed file.
func TrashInfoPath(trashID uuid.UUID) (RepositoryPath, error) {
	if trashID == uuid.Nil {
		return RepositoryPath{}, fmt.Errorf("%w: zero trash ID", ErrRepositoryPathInvalid)
	}
	return ParsePrivateRepositoryPath(path.Join(TrashInfoDirectory().String(), trashID.String()+".json"))
}

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
	if !(source.isUserMedia() && destination.isPrivate()) && !(source.isPrivate() && destination.isUserMedia()) {
		return ErrRepositoryPathNamespace
	}
	sourceLocal, err := source.local()
	if err != nil {
		return err
	}
	destinationLocal, err := destination.local()
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
	keepLocal, err := keep.local()
	if err != nil {
		return err
	}
	dropLocal, err := drop.local()
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
	leftLocal, err := left.local()
	if err != nil {
		return false, err
	}
	rightLocal, err := right.local()
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
	local, err := repositoryPath.local()
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
