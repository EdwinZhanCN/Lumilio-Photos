package storage

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// CanonicalizeRepositoryPath returns the canonical form of a repository path:
// absolute, cleaned, with symlinks resolved and the real on-disk casing applied.
// It is the single normalization used by every path read and write in storage,
// so that a symlinked path, a case-differing path on APFS/HFS+, and a plain
// path all resolve to one `repositories.path` row.
//
// Canonicalization is deliberately best-effort on the parts of the path that do
// not exist. An offline repository — an unplugged external drive is the ordinary
// case — has no readable path at all, and a canonicalizer that failed there
// would turn "this repository is offline" into "this path is invalid" for every
// lookup and for reconcile itself. Only the deepest existing ancestor is
// resolved; the missing tail is appended verbatim.
func CanonicalizeRepositoryPath(path string) (string, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "", errors.New("path is empty")
	}
	abs, err := filepath.Abs(filepath.Clean(trimmed))
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}
	return canonicalizeExisting(abs), nil
}

// IsRootedPath reports whether path is anchored to a filesystem root or volume
// and therefore must not be accepted where a repository-relative path is
// required.
//
// filepath.IsAbs alone is the wrong test. On Windows it answers false for a
// rooted-but-driveless path like "/etc/passwd" or "\Windows", because Go
// requires a volume for a path to be absolute there. Every containment check
// written as `if filepath.IsAbs(p) { reject }` therefore lets those through on
// Windows, which is exactly the class of path such a check exists to stop.
//
// The rules are applied uniformly rather than switching on runtime.GOOS, so the
// behaviour is identical everywhere and can be tested on any platform. The cost
// is that a relative file literally named `\foo` is rejected on Unix, where that
// is a legal name. Failing closed on a pathological filename is the right bias
// for a containment check.
func IsRootedPath(path string) bool {
	if path == "" {
		return false
	}
	if filepath.IsAbs(path) {
		return true
	}
	// "/foo" and "\foo": rooted on Windows, which IsAbs does not report. "\\server\share"
	// (UNC) is covered by the same leading separator.
	if path[0] == '/' || path[0] == '\\' {
		return true
	}
	// "C:\foo" and "C:foo" (drive-relative). Detected explicitly rather than via
	// filepath.VolumeName, which returns "" off Windows and would make this
	// answer differ by platform — and so be untestable on the Linux and macOS
	// runners that execute these tests.
	return len(path) >= 2 && path[1] == ':' && isASCIILetter(path[0])
}

func isASCIILetter(c byte) bool {
	return ('a' <= c && c <= 'z') || ('A' <= c && c <= 'Z')
}

// canonicalizeExisting resolves the longest existing prefix of abs and rejoins
// the components that do not exist yet.
func canonicalizeExisting(abs string) string {
	var missing []string
	current := abs
	for {
		if _, err := os.Lstat(current); err == nil {
			break
		}
		parent := filepath.Dir(current)
		if parent == current {
			// Nothing along the path exists; there is nothing to resolve.
			return abs
		}
		missing = append(missing, filepath.Base(current))
		current = parent
	}

	resolved, err := filepath.EvalSymlinks(current)
	if err != nil {
		resolved = current
	}
	resolved = realCasePath(resolved)

	for i := len(missing) - 1; i >= 0; i-- {
		resolved = filepath.Join(resolved, missing[i])
	}
	return resolved
}

// realCasePath rewrites each component of an existing absolute path to the
// casing the filesystem actually stores. On a case-sensitive filesystem this is
// a no-op because the exact match always wins.
func realCasePath(path string) string {
	volume := canonicalVolumeName(filepath.VolumeName(path))
	rest := strings.TrimPrefix(path[len(filepath.VolumeName(path)):], string(filepath.Separator))
	if rest == "" {
		return volume + string(filepath.Separator)
	}

	current := volume + string(filepath.Separator)
	for _, component := range strings.Split(rest, string(filepath.Separator)) {
		if component == "" {
			continue
		}
		current = filepath.Join(current, realCaseComponent(current, component))
	}
	return current
}

// canonicalVolumeName uppercases a Windows drive letter. The per-component
// casing walk below cannot reach the volume — there is no parent directory to
// list it from — so without this `c:\photos` and `C:\photos` stay two distinct
// strings and produce two `repositories.path` rows for one directory. Empty on
// Unix, and UNC volumes are left alone.
func canonicalVolumeName(volume string) string {
	if len(volume) == 2 && volume[1] == ':' {
		return strings.ToUpper(volume)
	}
	return volume
}

func realCaseComponent(dir, name string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return name
	}
	for _, entry := range entries {
		if entry.Name() == name {
			return name
		}
	}
	for _, entry := range entries {
		if strings.EqualFold(entry.Name(), name) {
			return entry.Name()
		}
	}
	return name
}
