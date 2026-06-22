package supervisor

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// ResourcesDir resolves the directory that holds bundled runtime assets
// (PostgreSQL, ffmpeg, exiftool). LUMILIO_RESOURCES_DIR overrides it for local
// development, where there is no installed bundle; otherwise it is derived from
// the executable location:
//   - macOS:   <App>.app/Contents/MacOS/<bin> → ../Resources
//   - Windows: <InstallDir>/lumilio-photos.exe → <InstallDir>/resources
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
	if runtime.GOOS == "windows" {
		return filepath.Clean(filepath.Join(filepath.Dir(exe), "resources")), nil
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
	return resolveToolPath(filepath.Join(resources, "exiftool", withExeSuffix("exiftool")))
}

func bundledFFmpeg(resources string) string {
	return resolveToolPath(filepath.Join(resources, "ffmpeg", withExeSuffix("ffmpeg")))
}

func bundledFFprobe(resources string) string {
	return resolveToolPath(filepath.Join(resources, "ffmpeg", withExeSuffix("ffprobe")))
}

// bundledVipsHome returns the bundle-local libvips prefix if dynamic modules
// were staged there. libvips searches $VIPSHOME/lib/vips-modules-<major>.<minor>
// during Startup(), so this must be set before the in-process API server starts.
func bundledVipsHome(resources string) string {
	matches, err := filepath.Glob(filepath.Join(resources, "lib", "vips-modules-*"))
	if err != nil || len(matches) == 0 {
		return ""
	}
	return resources
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
