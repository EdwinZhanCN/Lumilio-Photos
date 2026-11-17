package layout

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"server/internal/foundation/fsprivacy"
)

// Inbox is the public directory managed uploads land in. It is the only
// directory Lumilio creates outside .lumilio/.
const Inbox = "inbox"

// CacheDirTag marks cache/ as rebuildable for backup tools that honour the
// Cache Directory Tagging Specification (https://bford.info/cachedir/).
const CacheDirTag = "Signature: 8a477f597d28d172789f06886806bc55\n" +
	"# This directory holds Lumilio derivatives (thumbnails, transcodes,\n" +
	"# waveforms, face crops). Lumilio rebuilds them; it is safe to delete\n" +
	"# and to exclude from backups.\n"

// entry is one directory or file the tree holds from init, root-relative.
type entry struct {
	path string
	perm fs.FileMode
	file string // file content; empty for a directory or an empty file
	dir  bool
}

// initEntries is the tree Init creates, parents first.
func initEntries() []entry {
	entries := []entry{
		{path: Inbox, perm: 0o755, dir: true},
		{path: PrivateRoot, perm: 0o755, dir: true},
	}
	for _, policy := range Policies {
		if policy.Created == CreatedAtInit {
			entries = append(entries, entry{path: path.Join(PrivateRoot, string(policy.Class)), perm: policy.Perm, dir: true})
		}
	}
	for _, dir := range []string{studioDir, assetSidecarDir, trashFilesDir, trashInfoDir, artifactsDir, uploadsDir, tempDir} {
		class, _, _ := strings.Cut(dir, "/")
		entries = append(entries, entry{path: path.Join(PrivateRoot, dir), perm: PolicyOf(Class(class)).Perm, dir: true})
	}
	return append(entries,
		entry{path: path.Join(PrivateRoot, cacheDirTag), perm: 0o644, file: CacheDirTag},
		entry{path: path.Join(PrivateRoot, string(Logs), "operations.log"), perm: 0o644},
		entry{path: path.Join(PrivateRoot, string(Logs), "error.log"), perm: 0o644},
	)
}

// Init creates the Repository tree under repositoryPath: inbox/, every
// class created at init with its permission, CACHEDIR.TAG, and empty log
// files. It is idempotent: existing directories keep their contents and an
// existing file is never truncated, so Init also repairs a tree that lost
// a directory.
func Init(repositoryPath string) error {
	root, err := filepath.Abs(filepath.Clean(repositoryPath))
	if err != nil {
		return fmt.Errorf("invalid repository path: %w", err)
	}
	for _, item := range initEntries() {
		target := filepath.Join(root, filepath.FromSlash(item.path))
		if item.dir {
			if err := os.MkdirAll(target, item.perm); err != nil {
				return fmt.Errorf("create %s: %w", item.path, err)
			}
			if err := fsprivacy.ApplyDirectoryMode(target, item.perm); err != nil {
				return fmt.Errorf("set permissions of %s: %w", item.path, err)
			}
			continue
		}
		file, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, item.perm)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		if err != nil {
			return fmt.Errorf("create %s: %w", item.path, err)
		}
		_, writeErr := file.WriteString(item.file)
		if closeErr := file.Close(); writeErr == nil {
			writeErr = closeErr
		}
		if writeErr != nil {
			return fmt.Errorf("write %s: %w", item.path, writeErr)
		}
	}
	return nil
}

// Report is what Validate found, each path root-relative.
type Report struct {
	// Missing lists init entries that do not exist; Init repairs them.
	Missing []string
	// WrongKind lists a file where a directory belongs, or the reverse.
	WrongKind []string
	// Inaccessible lists entries that could not be inspected.
	Inaccessible []string
	// Unknown lists names under .lumilio/ that are not a class. They are
	// preserved unchanged and disclosed, never removed.
	Unknown []string
}

// Repairable reports whether Init can bring the tree to the layout: only
// missing entries, if anything, are wrong.
func (r Report) Repairable() bool { return len(r.WrongKind) == 0 && len(r.Inaccessible) == 0 }

// Validate compares the tree under repositoryPath with the layout. An
// absent or non-directory repository root is an error, not a report.
func Validate(repositoryPath string) (Report, error) {
	root, err := filepath.Abs(filepath.Clean(repositoryPath))
	if err != nil {
		return Report{}, fmt.Errorf("invalid repository path: %w", err)
	}
	if info, err := os.Stat(root); err != nil {
		return Report{}, fmt.Errorf("repository root: %w", err)
	} else if !info.IsDir() {
		return Report{}, fmt.Errorf("repository root %s is not a directory", root)
	}
	var report Report
	for _, item := range initEntries() {
		info, err := os.Lstat(filepath.Join(root, filepath.FromSlash(item.path)))
		switch {
		case errors.Is(err, fs.ErrNotExist):
			report.Missing = append(report.Missing, item.path)
		case err != nil:
			report.Inaccessible = append(report.Inaccessible, item.path)
		case item.dir != info.IsDir() || (!item.dir && !info.Mode().IsRegular()):
			report.WrongKind = append(report.WrongKind, item.path)
		}
	}
	children, err := os.ReadDir(filepath.Join(root, PrivateRoot))
	if err == nil {
		for _, child := range children {
			if PolicyOf(Class(child.Name())).Class == Unknown {
				report.Unknown = append(report.Unknown, path.Join(PrivateRoot, child.Name()))
			}
		}
		sort.Strings(report.Unknown)
	}
	return report, nil
}
