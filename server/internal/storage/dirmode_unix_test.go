//go:build !windows

package storage

import (
	"os"
	"testing"
)

// requireDirectoryIsPrivate asserts the platform's expression of "only the
// owner may use this directory". On Unix that is the POSIX mode.
func requireDirectoryIsPrivate(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	if perm := info.Mode().Perm(); perm != 0o700 {
		t.Fatalf("%s perm = %o, want 0700", path, perm)
	}
}

// unwritableDir returns a directory that refuses new entries, or "" when the
// platform cannot express that with the mechanism under test.
func unwritableDir(t *testing.T) string {
	t.Helper()
	if os.Getuid() == 0 {
		return ""
	}
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o444); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	return dir
}

// invalidStructurePath is a path the OS cannot turn into a directory. On Unix a
// regular file used as a parent is the classic case.
func invalidStructurePath() string {
	return "/dev/null/invalid"
}
