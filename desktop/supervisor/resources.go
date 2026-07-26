package supervisor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// ResourcesDir resolves the directory that holds bundled runtime assets
// (ffmpeg, exiftool, and the SPA). LUMILIO_RESOURCES_DIR overrides it for local
// development, where there is no bundle; otherwise it is derived from the
// executable location: on macOS <App>.app/Contents/MacOS/<bin> → ../Resources,
// on Windows a flat portable layout <dir>\lumilio-photos.exe → <dir>\resources.
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
		return filepath.Join(filepath.Dir(exe), "resources"), nil
	}
	return filepath.Clean(filepath.Join(filepath.Dir(exe), "..", "Resources")), nil
}

// toolExe appends the Windows executable suffix where needed.
func toolExe(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

// resolveToolPath returns the bundled candidate when present, otherwise it
// resolves the named development tool from PATH. The resolved absolute path is
// compiled into the immutable runtime manifest; the server never performs an
// implicit fallback.
func resolveToolPath(candidate, developmentTool string) string {
	if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
		return candidate
	}
	if path, err := exec.LookPath(developmentTool); err == nil {
		if absolute, err := filepath.Abs(path); err == nil {
			return absolute
		}
		return path
	}
	return ""
}

func bundledExifTool(resources string) string {
	tool := toolExe("exiftool")
	return resolveToolPath(filepath.Join(resources, "exiftool", tool), tool)
}

func bundledFFmpeg(resources string) string {
	tool := toolExe("ffmpeg")
	return resolveToolPath(filepath.Join(resources, "ffmpeg", tool), tool)
}

func bundledFFprobe(resources string) string {
	tool := toolExe("ffprobe")
	return resolveToolPath(filepath.Join(resources, "ffmpeg", tool), tool)
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
