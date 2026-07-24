package processors

import (
	"runtime"
	"testing"

	"server/config"
)

func TestResolveHardwareAccel(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"none", "none"},
		{"vaapi", "vaapi"},
		{"nvenc", "nvenc"},
		{"qsv", "qsv"},
		{"videotoolbox", "videotoolbox"},
	}

	for _, tt := range tests {
		got := resolveHardwareAccel(tt.input)
		if got != tt.expected {
			t.Errorf("resolveHardwareAccel(%q) = %q; want %q", tt.input, got, tt.expected)
		}
	}

	// Test "auto" resolution
	autoGot := resolveHardwareAccel("auto")
	if runtime.GOOS == "darwin" {
		if autoGot != "videotoolbox" {
			t.Errorf("resolveHardwareAccel(\"auto\") on macOS = %q; want \"videotoolbox\"", autoGot)
		}
	} else {
		if autoGot != "vaapi" && autoGot != "none" {
			t.Errorf("resolveHardwareAccel(\"auto\") on %s = %q; expected \"vaapi\" or \"none\"", runtime.GOOS, autoGot)
		}
	}
}

func TestBuildTranscodeArgs(t *testing.T) {
	input := "/path/to/input.mp4"
	output := "/path/to/output.mp4"
	filter := "scale=-2:1080"
	w, h := 1920, 1080

	cases := []struct {
		mode          string
		expectedCodec string
	}{
		{"none", "libx264"},
		{"nvenc", "h264_nvenc"},
		{"vaapi", "h264_vaapi"},
		{"qsv", "h264_qsv"},
		{"videotoolbox", "h264_videotoolbox"},
	}

	for _, tc := range cases {
		cfg := config.TranscodeConfig{HardwareAccel: tc.mode}
		args := buildTranscodeArgs(input, output, filter, w, h, cfg)
		foundCodec := false
		for i, arg := range args {
			if arg == "-c:v" && i+1 < len(args) {
				if args[i+1] == tc.expectedCodec {
					foundCodec = true
					break
				}
			}
		}
		if !foundCodec {
			t.Errorf("buildTranscodeArgs(%q) did not contain expected codec %q, args: %v", tc.mode, tc.expectedCodec, args)
		}
	}
}
