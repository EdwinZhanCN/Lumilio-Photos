package storage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCanonicalizeRepositoryPathRejectsEmpty(t *testing.T) {
	if _, err := CanonicalizeRepositoryPath("   "); err == nil {
		t.Fatal("CanonicalizeRepositoryPath(\"   \") returned no error")
	}
}

func TestCanonicalizeRepositoryPathResolvesSymlink(t *testing.T) {
	base := t.TempDir()
	target := filepath.Join(base, "target")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatalf("mkdir target: %v", err)
	}
	link := filepath.Join(base, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	viaLink, err := CanonicalizeRepositoryPath(link)
	if err != nil {
		t.Fatalf("CanonicalizeRepositoryPath(link) returned error: %v", err)
	}
	viaTarget, err := CanonicalizeRepositoryPath(target)
	if err != nil {
		t.Fatalf("CanonicalizeRepositoryPath(target) returned error: %v", err)
	}

	if viaLink != viaTarget {
		t.Fatalf("symlinked path canonicalized to %q, want %q", viaLink, viaTarget)
	}
}

func TestCanonicalizeRepositoryPathNormalizesCaseOnInsensitiveFilesystem(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "Photos")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	lowered := filepath.Join(base, "photos")
	if _, err := os.Stat(lowered); err != nil {
		t.Skip("filesystem is case-sensitive")
	}

	got, err := CanonicalizeRepositoryPath(lowered)
	if err != nil {
		t.Fatalf("CanonicalizeRepositoryPath returned error: %v", err)
	}
	want, err := CanonicalizeRepositoryPath(dir)
	if err != nil {
		t.Fatalf("CanonicalizeRepositoryPath returned error: %v", err)
	}
	if got != want {
		t.Fatalf("case-differing path canonicalized to %q, want %q", got, want)
	}
}

// An offline repository — the unplugged-external-drive case — must still
// canonicalize to a stable path so lookups and reconcile can compare it.
func TestCanonicalizeRepositoryPathSurvivesMissingPath(t *testing.T) {
	base := t.TempDir()
	missing := filepath.Join(base, "unplugged", "volume", "library")

	got, err := CanonicalizeRepositoryPath(missing)
	if err != nil {
		t.Fatalf("CanonicalizeRepositoryPath returned error: %v", err)
	}

	canonicalBase, err := CanonicalizeRepositoryPath(base)
	if err != nil {
		t.Fatalf("CanonicalizeRepositoryPath(base) returned error: %v", err)
	}
	want := filepath.Join(canonicalBase, "unplugged", "volume", "library")
	if got != want {
		t.Fatalf("missing path canonicalized to %q, want %q", got, want)
	}
}

// The existing prefix of a partially missing path is still resolved, so a
// repository that reappears under a symlinked mount matches its stored row.
func TestCanonicalizeRepositoryPathResolvesExistingPrefixOfMissingPath(t *testing.T) {
	base := t.TempDir()
	target := filepath.Join(base, "target")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatalf("mkdir target: %v", err)
	}
	link := filepath.Join(base, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	got, err := CanonicalizeRepositoryPath(filepath.Join(link, "library"))
	if err != nil {
		t.Fatalf("CanonicalizeRepositoryPath returned error: %v", err)
	}
	if strings.Contains(got, string(filepath.Separator)+"link"+string(filepath.Separator)) {
		t.Fatalf("existing prefix was not resolved: %q", got)
	}
	if filepath.Base(got) != "library" {
		t.Fatalf("missing tail was not preserved: %q", got)
	}
}

func TestCanonicalizeRepositoryPathIsIdempotent(t *testing.T) {
	base := t.TempDir()

	once, err := CanonicalizeRepositoryPath(base)
	if err != nil {
		t.Fatalf("CanonicalizeRepositoryPath returned error: %v", err)
	}
	twice, err := CanonicalizeRepositoryPath(once)
	if err != nil {
		t.Fatalf("CanonicalizeRepositoryPath returned error: %v", err)
	}
	if once != twice {
		t.Fatalf("canonicalization is not idempotent: %q then %q", once, twice)
	}
}

// The per-component casing walk cannot reach the volume, so a Windows drive
// letter has to be normalized on its own or `c:\photos` and `C:\photos` become
// two repositories.path rows for one directory.
func TestCanonicalVolumeNameUppercasesDriveLetter(t *testing.T) {
	cases := map[string]string{
		"c:":             "C:",
		"C:":             "C:",
		"":               "",
		`\\server\share`: `\\server\share`,
		`\\SERVER\Share`: `\\SERVER\Share`,
	}

	for input, want := range cases {
		if got := canonicalVolumeName(input); got != want {
			t.Fatalf("canonicalVolumeName(%q) = %q, want %q", input, got, want)
		}
	}
}

// canonicalTempDir returns a temp directory in the same canonical form the
// repository manager stores. On macOS t.TempDir() sits under /var, a symlink to
// /private/var, so raw temp paths never equal what the manager persists.
func canonicalTempDir(t *testing.T) string {
	t.Helper()
	dir, err := CanonicalizeRepositoryPath(t.TempDir())
	if err != nil {
		t.Fatalf("canonicalize temp dir: %v", err)
	}
	return dir
}
