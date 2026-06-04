package config

import (
	"os"
	"testing"
)

// unsetForTest removes env vars for the duration of a test and restores their
// original presence/value afterwards. Needed because setEnvDefault treats a
// present-but-empty env var as "already set", which t.Setenv(key, "") would
// produce — not the truly-absent state a fresh process sees.
func unsetForTest(t *testing.T, keys ...string) {
	t.Helper()
	for _, k := range keys {
		orig, had := os.LookupEnv(k)
		if err := os.Unsetenv(k); err != nil {
			t.Fatalf("unset %s: %v", k, err)
		}
		t.Cleanup(func() {
			if had {
				_ = os.Setenv(k, orig)
			} else {
				_ = os.Unsetenv(k)
			}
		})
	}
}

func TestToolPathDefaults(t *testing.T) {
	// With no overrides set, resolvers fall back to the bare command name so
	// os/exec resolves them via PATH (the web/docker default).
	unsetForTest(t, "EXIFTOOL_PATH", "FFMPEG_PATH", "FFPROBE_PATH")

	if got := ExifToolPath(); got != defaultExifToolCommand {
		t.Errorf("ExifToolPath() = %q, want %q", got, defaultExifToolCommand)
	}
	if got := FFmpegPath(); got != defaultFFmpegCommand {
		t.Errorf("FFmpegPath() = %q, want %q", got, defaultFFmpegCommand)
	}
	if got := FFprobePath(); got != defaultFFprobeCommand {
		t.Errorf("FFprobePath() = %q, want %q", got, defaultFFprobeCommand)
	}
}

func TestToolPathEnvOverride(t *testing.T) {
	t.Setenv("EXIFTOOL_PATH", "/bundle/exiftool")
	t.Setenv("FFMPEG_PATH", "/bundle/ffmpeg")
	t.Setenv("FFPROBE_PATH", "/bundle/ffprobe")

	if got := ExifToolPath(); got != "/bundle/exiftool" {
		t.Errorf("ExifToolPath() = %q, want /bundle/exiftool", got)
	}
	if got := FFmpegPath(); got != "/bundle/ffmpeg" {
		t.Errorf("FFmpegPath() = %q, want /bundle/ffmpeg", got)
	}
	if got := FFprobePath(); got != "/bundle/ffprobe" {
		t.Errorf("FFprobePath() = %q, want /bundle/ffprobe", got)
	}
}

// ApplyRuntimeEnvDefaults must bridge the TOML [tools] section into the
// environment so the resolvers (which read env) honor server.local.toml values.
// This is the path the desktop supervisor relies on when it generates the toml.
func TestApplyRuntimeEnvDefaultsBridgesToolPaths(t *testing.T) {
	unsetForTest(t, "EXIFTOOL_PATH", "FFMPEG_PATH", "FFPROBE_PATH")

	cfg := AppConfig{Tools: ToolsConfig{
		ExifToolPath: "/from/toml/exiftool",
		FFmpegPath:   "/from/toml/ffmpeg",
		FFprobePath:  "/from/toml/ffprobe",
	}}
	ApplyRuntimeEnvDefaults(cfg)

	if got := ExifToolPath(); got != "/from/toml/exiftool" {
		t.Errorf("ExifToolPath() after bridge = %q, want /from/toml/exiftool", got)
	}
	if got := FFmpegPath(); got != "/from/toml/ffmpeg" {
		t.Errorf("FFmpegPath() after bridge = %q, want /from/toml/ffmpeg", got)
	}
	if got := FFprobePath(); got != "/from/toml/ffprobe" {
		t.Errorf("FFprobePath() after bridge = %q, want /from/toml/ffprobe", got)
	}
}

// An explicit environment override must win over the TOML value, matching the
// "env applied after TOML" precedence used everywhere else in config.
func TestEnvOverrideBeatsTOMLBridge(t *testing.T) {
	t.Setenv("EXIFTOOL_PATH", "/env/exiftool")

	cfg := AppConfig{Tools: ToolsConfig{ExifToolPath: "/from/toml/exiftool"}}
	ApplyRuntimeEnvDefaults(cfg)

	if got := ExifToolPath(); got != "/env/exiftool" {
		t.Errorf("ExifToolPath() = %q, want /env/exiftool (env beats toml)", got)
	}
}
