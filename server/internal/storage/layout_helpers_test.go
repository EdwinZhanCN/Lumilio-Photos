package storage

import (
	"os"
	"path/filepath"
	"testing"

	"server/internal/storage/layout"
)

// repoConfigFile is the portable repository marker.
const repoConfigFile = ".lumiliorepo"

// onDisk joins a layout path, and any names below it, onto a repository
// root.
func onDisk(t testing.TB, repositoryPath string, p layout.Path, names ...string) string {
	t.Helper()
	local, err := p.Local()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(append([]string{repositoryPath, local}, names...)...)
}

// mustPath unwraps a constructor result; a test path is always valid.
func mustPath(p layout.Path, err error) layout.Path {
	if err != nil {
		panic(err)
	}
	return p
}

// assertNoPrivateTemporaries fails when an atomic write left a temporary
// behind.
func assertNoPrivateTemporaries(t testing.TB, repositoryPath string) {
	t.Helper()
	entries, err := os.ReadDir(onDisk(t, repositoryPath, layout.TempDir()))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("atomic write left %d temporaries", len(entries))
	}
}
