package platform

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// MediaToolPaths contains the absolute commands used by the embedded Server.
// Packaged Desktop runs without a package-manager PATH, so the host resolves
// bundled tools before the Server consumes the runtime manifest.
type MediaToolPaths struct {
	ExifTool string
	FFmpeg   string
	FFprobe  string
}

// ResolveBundledResourcesDir resolves the runtime resource root next to the
// Desktop executable. LUMILIO_RESOURCES_DIR is only a host-side development
// override; standalone Server configuration never searches for this directory.
func ResolveBundledResourcesDir() (string, error) {
	if override := os.Getenv("LUMILIO_RESOURCES_DIR"); override != "" {
		return filepath.Clean(override), nil
	}

	executable, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve Desktop executable: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(executable); err == nil {
		executable = resolved
	}
	return bundledResourcesDir(executable, runtime.GOOS), nil
}

func bundledResourcesDir(executable, goos string) string {
	if executable == "" {
		return ""
	}
	if goos == "windows" {
		return filepath.Join(filepath.Dir(executable), "resources")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(executable), "..", "Resources"))
}

// ResolveMediaToolPaths prefers the tools shipped in the app bundle and falls
// back to absolute development-tool paths. An empty field means neither source
// was available; the strict Server manifest loader will report that clearly.
func ResolveMediaToolPaths() (MediaToolPaths, error) {
	resources, err := ResolveBundledResourcesDir()
	if err != nil {
		return MediaToolPaths{}, err
	}
	return resolveMediaToolPaths(resources, runtime.GOOS), nil
}

func resolveMediaToolPaths(resources, goos string) MediaToolPaths {
	suffix := ""
	if goos == "windows" {
		suffix = ".exe"
	}
	return MediaToolPaths{
		ExifTool: resolveToolPath(filepath.Join(resources, "exiftool", "exiftool"+suffix), "exiftool"+suffix),
		FFmpeg:   resolveToolPath(filepath.Join(resources, "ffmpeg", "ffmpeg"+suffix), "ffmpeg"+suffix),
		FFprobe:  resolveToolPath(filepath.Join(resources, "ffmpeg", "ffprobe"+suffix), "ffprobe"+suffix),
	}
}

func resolveToolPath(candidate, developmentTool string) string {
	if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
		return candidate
	}
	path, err := exec.LookPath(developmentTool)
	if err != nil {
		return ""
	}
	absolute, err := filepath.Abs(path)
	if err == nil {
		return absolute
	}
	return path
}

// ClearBundledResourcesQuarantine removes macOS's quarantine marker from
// shipped command-line tools after the user has opened the trusted Desktop
// app. It is a non-fatal cleanup: local builds and already-approved bundles
// commonly have no such attribute.
func ClearBundledResourcesQuarantine() error {
	if runtime.GOOS != "darwin" {
		return nil
	}
	resources, err := ResolveBundledResourcesDir()
	if err != nil {
		return err
	}
	return exec.Command("xattr", "-dr", "com.apple.quarantine", resources).Run()
}
