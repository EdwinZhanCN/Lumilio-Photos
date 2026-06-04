package config

import (
	"os"
	"strings"
)

// ToolsConfig holds optional absolute paths to the external media tools the
// server shells out to. An empty value means "resolve the bare command name via
// PATH", which is the default behavior for web/docker deployments. The desktop
// build sets these to binaries shipped inside the app bundle, where there is no
// system PATH to rely on.
//
// See docs/agent/exec-plans/active/desktop-wails-v3.md → "Native Dependencies
// Bundling → Track B".
type ToolsConfig struct {
	ExifToolPath string `toml:"exiftool_path"`
	FFmpegPath   string `toml:"ffmpeg_path"`
	FFprobePath  string `toml:"ffprobe_path"`
}

// Bare command names used when no override is configured. Resolved via PATH by
// os/exec, preserving the existing web/docker behavior.
const (
	defaultExifToolCommand = "exiftool"
	defaultFFmpegCommand   = "ffmpeg"
	defaultFFprobeCommand  = "ffprobe"
)

// ExifToolPath returns the exiftool executable to invoke: the EXIFTOOL_PATH
// override when set, otherwise the bare "exiftool" command resolved via PATH.
//
// It reads the resolved environment value rather than an AppConfig field so the
// low-level exec call sites do not need AppConfig threaded through them.
// ApplyRuntimeEnvDefaults bridges the TOML [tools] section into the environment
// at startup, so a value set in server.local.toml is honored here too.
func ExifToolPath() string {
	return toolPath("EXIFTOOL_PATH", defaultExifToolCommand)
}

// FFmpegPath returns the ffmpeg executable to invoke (FFMPEG_PATH override or
// the bare "ffmpeg" command).
func FFmpegPath() string {
	return toolPath("FFMPEG_PATH", defaultFFmpegCommand)
}

// FFprobePath returns the ffprobe executable to invoke (FFPROBE_PATH override or
// the bare "ffprobe" command).
func FFprobePath() string {
	return toolPath("FFPROBE_PATH", defaultFFprobeCommand)
}

func toolPath(envKey, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(envKey)); v != "" {
		return v
	}
	return fallback
}
