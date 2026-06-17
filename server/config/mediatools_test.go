package config

import "testing"

func TestToolCommandsDefaultToPathLookupNames(t *testing.T) {
	cfg := ToolsConfig{}

	if got := cfg.ExifToolCommand(); got != defaultExifToolCommand {
		t.Errorf("ExifToolCommand() = %q, want %q", got, defaultExifToolCommand)
	}
	if got := cfg.FFmpegCommand(); got != defaultFFmpegCommand {
		t.Errorf("FFmpegCommand() = %q, want %q", got, defaultFFmpegCommand)
	}
	if got := cfg.FFprobeCommand(); got != defaultFFprobeCommand {
		t.Errorf("FFprobeCommand() = %q, want %q", got, defaultFFprobeCommand)
	}
}

func TestToolCommandsUseTypedConfigPaths(t *testing.T) {
	cfg := ToolsConfig{
		ExifToolPath: "/bundle/exiftool",
		FFmpegPath:   "/bundle/ffmpeg",
		FFprobePath:  "/bundle/ffprobe",
	}

	if got := cfg.ExifToolCommand(); got != "/bundle/exiftool" {
		t.Errorf("ExifToolCommand() = %q, want /bundle/exiftool", got)
	}
	if got := cfg.FFmpegCommand(); got != "/bundle/ffmpeg" {
		t.Errorf("FFmpegCommand() = %q, want /bundle/ffmpeg", got)
	}
	if got := cfg.FFprobeCommand(); got != "/bundle/ffprobe" {
		t.Errorf("FFprobeCommand() = %q, want /bundle/ffprobe", got)
	}
}
