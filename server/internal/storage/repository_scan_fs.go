package storage

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"sort"
	"strings"
	"syscall"

	fileutil "server/internal/utils/file"
)

// DirectoryListing is one complete user-media directory as the scan index
// sees it: positive observations sorted by name, plus the children that exist
// but could not be observed. A child that appears in neither exists only if a
// later positive probe says so.
type DirectoryListing struct {
	Directories []FileObservation
	Files       []FileObservation
	Issues      []WalkIssue
}

// ListUserMediaDirectory reads one whole directory of the user tree in a
// single pass, without descending. It applies the walk policy of
// ReadUserMediaDirectory: private and marker names are skipped, nested
// repositories are reported and not listed, files with unsupported
// extensions are ignored, and symlinks are never followed into directories.
// An error means the directory itself could not be read; the caller must
// treat its catalog rows as unknown, not absent.
func (r *RepositoryFS) ListUserMediaDirectory(ctx context.Context, directory string) (DirectoryListing, error) {
	var listing DirectoryListing
	if err := ctx.Err(); err != nil {
		return listing, err
	}
	local := "."
	if directory != "" {
		parsed, err := ParseUserMediaPath(directory)
		if err != nil {
			return listing, err
		}
		local, err = parsed.local()
		if err != nil {
			return listing, err
		}
	}
	root, done, err := r.withRoot()
	if err != nil {
		return listing, err
	}
	defer done()
	opened, err := root.Open(local)
	if err != nil {
		return listing, classifyRepositoryEntryError(directory, err)
	}
	defer opened.Close()
	info, err := opened.Stat()
	if err != nil {
		return listing, err
	}
	if !info.IsDir() {
		return listing, fmt.Errorf("%w: %s is not a directory", ErrRepositoryEntryUnsupported, directory)
	}
	entries, err := opened.ReadDir(-1)
	if err != nil {
		return listing, classifyRepositoryEntryError(directory, err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return listing, err
		}
		child := entry.Name()
		if directory != "" {
			child = directory + "/" + child
		}
		if child == ".lumilio" || child == ".lumiliorepo" || child == ".lumilioroot" {
			continue
		}
		if entry.IsDir() {
			if _, markerErr := root.Stat(path.Join(child, ".lumiliorepo")); markerErr == nil {
				listing.Issues = append(listing.Issues, WalkIssue{Path: child, Reason: "nested_repository", Err: ErrNestedRepository})
				continue
			} else if !errors.Is(markerErr, fs.ErrNotExist) {
				listing.Issues = append(listing.Issues, WalkIssue{Path: child, Reason: "nested_repository_check", Err: markerErr})
				continue
			}
		}
		repositoryPath, parseErr := ParseUserMediaPath(child)
		if parseErr != nil {
			listing.Issues = append(listing.Issues, WalkIssue{Path: child, Reason: "invalid_path", Err: parseErr})
			continue
		}
		if !entry.IsDir() && !fileutil.IsSupportedExtension(path.Ext(repositoryPath.String())) {
			continue
		}
		observation, observeErr := r.observeNodeWithHeldRoot(ctx, root, repositoryPath, entry.IsDir())
		if observeErr != nil {
			reason := "inspect_error"
			if errors.Is(observeErr, ErrRepositoryEntryUnsupported) {
				reason = "unsupported_entry"
			}
			listing.Issues = append(listing.Issues, WalkIssue{Path: child, Reason: reason, Err: observeErr})
			continue
		}
		if observation.EntryKind == EntryKindDirectory {
			listing.Directories = append(listing.Directories, observation)
		} else {
			listing.Files = append(listing.Files, observation)
		}
	}
	return listing, nil
}

// ProbeUserMediaAbsence reports whether a catalog path is positively gone,
// the only evidence on which the scan index marks an entry missing. It is
// true when the path or one of its ancestors does not exist, when an
// ancestor is no longer a real directory (a file, or a symlink the walk
// would not follow), or when the path now holds the other kind of entry.
// Any other failure, such as a permission error or an unreadable volume, is
// returned as an error and proves nothing.
func (r *RepositoryFS) ProbeUserMediaAbsence(ctx context.Context, repositoryPath RepositoryPath, kind EntryKind) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if !repositoryPath.isUserMedia() {
		return false, ErrRepositoryPathNamespace
	}
	local, err := repositoryPath.local()
	if err != nil {
		return false, err
	}
	root, done, err := r.withRoot()
	if err != nil {
		return false, err
	}
	defer done()
	components := strings.Split(repositoryPath.String(), "/")
	for depth := 1; depth < len(components); depth++ {
		ancestor := strings.Join(components[:depth], string(os.PathSeparator))
		info, statErr := root.Lstat(ancestor)
		if statErr != nil {
			return absentFromError(statErr)
		}
		if !info.IsDir() {
			return true, nil
		}
	}
	info, err := root.Lstat(local)
	if err != nil {
		return absentFromError(err)
	}
	if kind == EntryKindDirectory {
		return !info.IsDir(), nil
	}
	return info.IsDir(), nil
}

func absentFromError(err error) (bool, error) {
	if errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR) {
		return true, nil
	}
	return false, err
}

// ListUserMediaSubdirectories returns the sorted names of the directories a
// walk would descend into below directory, without observing any file. A
// resumed walk uses it to rebuild its position cheaply.
func (r *RepositoryFS) ListUserMediaSubdirectories(ctx context.Context, directory string) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	local := "."
	if directory != "" {
		parsed, err := ParseUserMediaPath(directory)
		if err != nil {
			return nil, err
		}
		local, err = parsed.local()
		if err != nil {
			return nil, err
		}
	}
	root, done, err := r.withRoot()
	if err != nil {
		return nil, err
	}
	defer done()
	opened, err := root.Open(local)
	if err != nil {
		return nil, classifyRepositoryEntryError(directory, err)
	}
	defer opened.Close()
	entries, err := opened.ReadDir(-1)
	if err != nil {
		return nil, classifyRepositoryEntryError(directory, err)
	}
	names := make([]string, 0)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		child := entry.Name()
		if directory != "" {
			child = directory + "/" + child
		}
		if child == ".lumilio" {
			continue
		}
		if _, markerErr := root.Stat(path.Join(child, ".lumiliorepo")); !errors.Is(markerErr, fs.ErrNotExist) {
			continue
		}
		if _, parseErr := ParseUserMediaPath(child); parseErr != nil {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	return names, nil
}
