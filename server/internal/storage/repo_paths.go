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
