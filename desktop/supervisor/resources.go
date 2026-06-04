package supervisor

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// ResourcesDir resolves the directory that holds bundled runtime assets
// (PostgreSQL, ffmpeg, exiftool). LUMILIO_RESOURCES_DIR overrides it for local
// development, where there is no .app bundle; otherwise it is derived from the
// executable location, which on macOS is <App>.app/Contents/MacOS/<bin> →
// ../Resources.
func ResourcesDir() (string, error) {
	if v := os.Getenv("LUMILIO_RESOURCES_DIR"); v != "" {
		return v, nil
	}
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("locate executable: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return filepath.Clean(filepath.Join(filepath.Dir(exe), "..", "Resources")), nil
}

// pgBinDir returns the directory containing the PostgreSQL binaries for the
// current platform. LUMILIO_PG_BIN_DIR overrides it (e.g. point at a Homebrew
// PostgreSQL during development).
func pgBinDir(resources string) string {
	if v := os.Getenv("LUMILIO_PG_BIN_DIR"); v != "" {
		return v
	}
	platform := runtime.GOOS + "-" + runtime.GOARCH
	return filepath.Join(resources, "postgres", pgMajorVersion, platform, "bin")
}

// resolveToolPath returns candidate if it is an existing regular file, else "".
// An empty result tells the server to resolve the tool via PATH (preserving the
// web/docker default), which is what we want during development.
func resolveToolPath(candidate string) string {
	if candidate == "" {
		return ""
	}
	if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
		return candidate
	}
	return ""
}

func bundledExifTool(resources string) string {
	return resolveToolPath(filepath.Join(resources, "exiftool", "exiftool"))
}

func bundledFFmpeg(resources string) string {
	return resolveToolPath(filepath.Join(resources, "ffmpeg", "ffmpeg"))
}

func bundledFFprobe(resources string) string {
	return resolveToolPath(filepath.Join(resources, "ffmpeg", "ffprobe"))
}

// bundledWebRoot returns the bundled web SPA directory if it contains an
// index.html, else "" (server runs API-only). LUMILIO_WEB_ROOT overrides it for
// development, where the build is at <repo>/web/dist rather than in the bundle.
func bundledWebRoot(resources string) string {
	if v := os.Getenv("LUMILIO_WEB_ROOT"); v != "" {
		return v
	}
	dir := filepath.Join(resources, "web")
	if info, err := os.Stat(filepath.Join(dir, "index.html")); err == nil && !info.IsDir() {
		return dir
	}
	return ""
}
