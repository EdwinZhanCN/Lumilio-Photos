package sqliteuri

import (
	"net/url"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestDSNBuildsLocalFileURI(t *testing.T) {
	path, err := filepath.Abs(filepath.Join(t.TempDir(), "catalog with space.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}

	dsn := DSN(path, url.Values{"mode": {"ro"}, "immutable": {"1"}})
	location, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parse DSN %q: %v", dsn, err)
	}
	if location.Scheme != "file" {
		t.Fatalf("DSN scheme = %q, want file", location.Scheme)
	}
	if got := location.Query().Get("mode"); got != "ro" {
		t.Fatalf("mode = %q, want ro", got)
	}
	if got := location.Query().Get("immutable"); got != "1" {
		t.Fatalf("immutable = %q, want 1", got)
	}
	if runtime.GOOS == "windows" {
		if !strings.HasPrefix(dsn, "file:///") {
			t.Fatalf("Windows DSN = %q, want file:/// drive path", dsn)
		}
		if strings.Contains(strings.ToLower(dsn), "%5c") {
			t.Fatalf("Windows DSN contains encoded backslashes: %q", dsn)
		}
	}
}
