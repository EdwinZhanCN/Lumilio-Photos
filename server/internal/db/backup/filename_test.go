package backup

import (
	"path/filepath"
	"testing"
	"time"
)

func TestSnapshotFilenameRoundTrip(t *testing.T) {
	createdAt := time.Date(2026, 7, 25, 18, 42, 13, 123456000, time.FixedZone("test", -4*60*60))
	name := FileName(createdAt)
	if name != "20260725T224213.123456Z-library.sqlite3" {
		t.Fatalf("FileName() = %q", name)
	}
	info, ok := ParseName(name)
	if !ok {
		t.Fatalf("ParseName(%q) rejected generated name", name)
	}
	if !info.CreatedAt.Equal(createdAt) {
		t.Fatalf("created_at = %s, want %s", info.CreatedAt, createdAt)
	}
	if got := ManifestName(name); got != "20260725T224213.123456Z-library-manifest.json" {
		t.Fatalf("ManifestName() = %q", got)
	}
	if got := ManifestPath(filepath.Join("/backups", name)); got != filepath.Join("/backups", ManifestName(name)) {
		t.Fatalf("ManifestPath() = %q", got)
	}
}

func TestSnapshotFilenameRejectsUnsafeNames(t *testing.T) {
	for _, name := range []string{
		"",
		"../20260725T224213.123456Z-library.sqlite3",
		"20260725T224213Z-library.sqlite3",
		"restore-point-" + FileName(time.Now()),
		FileName(time.Now()) + TmpSuffix,
	} {
		if _, ok := ParseName(name); ok {
			t.Errorf("ParseName(%q) unexpectedly succeeded", name)
		}
	}
}
