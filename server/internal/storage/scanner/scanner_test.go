package scanner

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestShouldScanPathFiltersWorkspace(t *testing.T) {
	tests := map[string]bool{
		"album/photo.jpg":           true,
		"album/clip.mp4":            true,
		".lumilio/assets/photo.jpg": false,
		"inbox/photo.jpg":           false,
		"../escape/photo.jpg":       false,
		"/absolute/photo.jpg":       false,
		"album/document.txt":        false,
		"album/sub/../photo.jpg":    true,
		".lumilio":                  false,
		"inbox":                     false,
		"album/.hidden/photo.jpg":   true,
	}

	for path, want := range tests {
		t.Run(path, func(t *testing.T) {
			_, got := ShouldScanPath(path)
			if got != want {
				t.Fatalf("ShouldScanPath(%q) = %v, want %v", path, got, want)
			}
		})
	}
}

func TestWalkRepositorySkipsExcludedAndUnsettledFiles(t *testing.T) {
	root := t.TempDir()
	writeFile := func(rel string, modTime time.Time) {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(path, []byte("data"), 0644); err != nil {
			t.Fatalf("write file: %v", err)
		}
		if err := os.Chtimes(path, modTime, modTime); err != nil {
			t.Fatalf("chtimes: %v", err)
		}
	}

	old := time.Now().Add(-10 * time.Minute)
	writeFile("album/photo.jpg", old)
	writeFile(".lumilio/assets/thumb.jpg", old)
	writeFile("inbox/upload.jpg", old)
	writeFile("album/recent.jpg", time.Now())
	writeFile("album/readme.txt", old)

	entries, skipped, err := walkRepository(root, 5*time.Second)
	if err != nil {
		t.Fatalf("walk repository: %v", err)
	}
	if _, ok := entries["album/photo.jpg"]; !ok {
		t.Fatalf("expected album/photo.jpg to be scanned, got %#v", entries)
	}
	if len(entries) != 1 {
		t.Fatalf("expected only one scanned entry, got %#v", entries)
	}
	if skipped == 0 {
		t.Fatalf("expected skipped files to be counted")
	}
}
