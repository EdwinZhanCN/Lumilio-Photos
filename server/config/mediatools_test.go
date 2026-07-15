package config

import "testing"

func TestToolCommandsReturnManifestValues(t *testing.T) {
	cfg := ToolsConfig{ExifToolPath: "exiftool", FFmpegPath: "/bundle/ffmpeg", FFprobePath: "/bundle/ffprobe"}
	if cfg.ExifToolCommand() != "exiftool" || cfg.FFmpegCommand() != "/bundle/ffmpeg" || cfg.FFprobeCommand() != "/bundle/ffprobe" {
		t.Fatalf("unexpected tool commands: %+v", cfg)
	}
}
