package config

// ToolsConfig holds optional absolute paths to the external media tools the
// server shells out to. An empty value means "resolve the bare command name via
// PATH", which is the default behavior for web/docker deployments. The desktop
// build sets these to binaries shipped inside the app bundle, where there is no
// system PATH to rely on.
//
// See site/docs/internal/agent/exec-plans/active/desktop-wails-v3.md → "Native Dependencies
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

func (c ToolsConfig) ExifToolCommand() string {
	if c.ExifToolPath != "" {
		return c.ExifToolPath
	}
	return defaultExifToolCommand
}

func (c ToolsConfig) FFmpegCommand() string {
	if c.FFmpegPath != "" {
		return c.FFmpegPath
	}
	return defaultFFmpegCommand
}

func (c ToolsConfig) FFprobeCommand() string {
	if c.FFprobePath != "" {
		return c.FFprobePath
	}
	return defaultFFprobeCommand
}
