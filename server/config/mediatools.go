package config

// ToolsConfig holds explicit media-tool commands or paths. Bare commands use
// PATH lookup; empty values are rejected by the manifest loader.
type ToolsConfig struct {
	ExifToolPath string
	FFmpegPath   string
	FFprobePath  string
}

func (c ToolsConfig) ExifToolCommand() string { return c.ExifToolPath }
func (c ToolsConfig) FFmpegCommand() string   { return c.FFmpegPath }
func (c ToolsConfig) FFprobeCommand() string  { return c.FFprobePath }
