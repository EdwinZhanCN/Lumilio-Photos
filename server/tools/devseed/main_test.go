package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRequireLoopbackAPI(t *testing.T) {
	t.Parallel()
	for _, rawURL := range []string{
		"http://localhost:6680",
		"http://127.0.0.1:6680",
		"https://[::1]:6680",
	} {
		if err := requireLoopbackAPI(rawURL); err != nil {
			t.Errorf("requireLoopbackAPI(%q) returned %v", rawURL, err)
		}
	}
	for _, rawURL := range []string{
		"https://photos.example.com",
		"http://localhost:6680/api",
		"http://localhost:not-a-port",
		"file:///tmp/lumilio",
	} {
		if err := requireLoopbackAPI(rawURL); err == nil {
			t.Errorf("requireLoopbackAPI(%q) unexpectedly succeeded", rawURL)
		}
	}
}

func TestSafeJoin(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	joined, ok := safeJoin(root, filepath.Join("旅行档案", "photo.jpg"))
	if !ok {
		t.Fatal("safeJoin rejected a repository-relative path")
	}
	if want := filepath.Join(root, "旅行档案", "photo.jpg"); joined != want {
		t.Fatalf("safeJoin returned %q, want %q", joined, want)
	}
	for _, rel := range []string{"", "../photo.jpg", filepath.Join("..", "outside", "photo.jpg")} {
		if _, ok := safeJoin(root, rel); ok {
			t.Errorf("safeJoin unexpectedly accepted %q", rel)
		}
	}
}

func TestCopyFileExclusiveAndMatch(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	source := filepath.Join(root, "source.jpg")
	target := filepath.Join(root, "nested", "target.jpg")
	content := []byte("seed-photo-bytes")
	if err := os.WriteFile(source, content, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := copyFileExclusive(source, target); err != nil {
		t.Fatal(err)
	}
	match, exists, err := filesMatch(source, target)
	if err != nil {
		t.Fatal(err)
	}
	if !exists || !match {
		t.Fatalf("filesMatch returned match=%v exists=%v", match, exists)
	}
	if err := copyFileExclusive(source, target); err == nil {
		t.Fatal("copyFileExclusive unexpectedly overwrote an existing target")
	}
}
