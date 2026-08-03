package platform

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveMediaToolPathsPrefersBundle(t *testing.T) {
	resources := t.TempDir()
	paths := map[string]string{
		"exiftool/exiftool": "exiftool",
		"ffmpeg/ffmpeg":     "ffmpeg",
		"ffmpeg/ffprobe":    "ffprobe",
	}
	for relative, contents := range paths {
		path := filepath.Join(resources, relative)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("create tool directory: %v", err)
		}
		if err := os.WriteFile(path, []byte(contents), 0o755); err != nil {
			t.Fatalf("write tool fixture: %v", err)
		}
	}

	got := resolveMediaToolPaths(resources, "darwin")
	if got.ExifTool != filepath.Join(resources, "exiftool", "exiftool") ||
		got.FFmpeg != filepath.Join(resources, "ffmpeg", "ffmpeg") ||
		got.FFprobe != filepath.Join(resources, "ffmpeg", "ffprobe") {
		t.Fatalf("bundle tool paths = %+v", got)
	}
}

func TestResolveMediaToolPathsUsesWindowsNames(t *testing.T) {
	resources := t.TempDir()
	for _, relative := range []string{
		"exiftool/exiftool.exe",
		"ffmpeg/ffmpeg.exe",
		"ffmpeg/ffprobe.exe",
	} {
		path := filepath.Join(resources, relative)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("create tool directory: %v", err)
		}
		if err := os.WriteFile(path, []byte("tool"), 0o755); err != nil {
			t.Fatalf("write tool fixture: %v", err)
		}
	}

	got := resolveMediaToolPaths(resources, "windows")
	if got.ExifTool != filepath.Join(resources, "exiftool", "exiftool.exe") ||
		got.FFmpeg != filepath.Join(resources, "ffmpeg", "ffmpeg.exe") ||
		got.FFprobe != filepath.Join(resources, "ffmpeg", "ffprobe.exe") {
		t.Fatalf("Windows bundle tool paths = %+v", got)
	}
}
