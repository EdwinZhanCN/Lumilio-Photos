package processors

import (
	"math"
	"runtime"
	"testing"

	"server/config"
	"server/internal/execution"
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
		got := ResolveHardwareAccel(tt.input)
		if got != tt.expected {
			t.Errorf("resolveHardwareAccel(%q) = %q; want %q", tt.input, got, tt.expected)
		}
	}

	// Test "auto" resolution
	auto := ResolveHardwareAccel("auto")
	if runtime.GOOS == "darwin" && auto != "videotoolbox" {
		t.Errorf("resolveHardwareAccel(\"auto\") on darwin = %q; want \"videotoolbox\"", auto)
	}
}

func TestBitrateForResolution(t *testing.T) {
	tests := []struct {
		width       int
		height      int
		wantMaxrate string
		wantBufsize string
	}{
		{1920, 1080, "6912k", "13824k"},
		{1280, 720, "3072k", "6144k"},
		{640, 480, "2000k", "4000k"}, // floor at 2000k
		{100, 100, "2000k", "4000k"}, // small dimensions floor
	}

	for _, tt := range tests {
		maxrate, bufsize := bitrateForResolution(tt.width, tt.height)
		if maxrate != tt.wantMaxrate || bufsize != tt.wantBufsize {
			t.Errorf("bitrateForResolution(%d, %d) = (%s, %s); want (%s, %s)",
				tt.width, tt.height, maxrate, bufsize, tt.wantMaxrate, tt.wantBufsize)
		}
	}
}

func TestBuildScaleFilter(t *testing.T) {
	// Landscape
	filter := buildScaleFilter(1920, 1080, 1920, 1080)
	if filter != "scale=-2:1080" {
		t.Errorf("expected scale=-2:1080, got %s", filter)
	}

	// Portrait
	filter = buildScaleFilter(1080, 1920, 1080, 1920)
	if filter != "scale=1080:-2" {
		t.Errorf("expected scale=1080:-2, got %s", filter)
	}

	// Square
	filter = buildScaleFilter(1080, 1080, 1080, 1080)
	if filter != "scale=-2:1080" {
		t.Errorf("expected scale=-2:1080, got %s", filter)
	}
}

func TestParseFrameRate(t *testing.T) {
	tests := []struct {
		input    string
		expected float64
	}{
		{"30/1", 30.0},
		{"60/1", 60.0},
		{"24000/1001", 23.976023976023978},
		{"30000/1001", 29.97002997002997},
		{"25/1", 25.0},
		{"invalid", 0.0},
		{"30/0", 0.0},
		{"30", 30.0},
	}

	for _, tt := range tests {
		got := parseFrameRate(tt.input)
		if math.Abs(got-tt.expected) > 0.001 {
			t.Errorf("parseFrameRate(%q) = %f; want %f", tt.input, got, tt.expected)
		}
	}
}

func TestBuildTranscodeArgs(t *testing.T) {
	input := "/path/to/input.mp4"
	output := "/path/to/output.mp4"
	filter := "scale=-2:1080"
	w, h := 1920, 1080
	session := execution.ToolSession{Threads: 2, SoftwarePreset: "medium"}

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
		args := buildTranscodeArgs(input, output, filter, w, h, cfg, session)
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

func TestSoftwareTranscodeThreadsAndPresetContract(t *testing.T) {
	input := "/path/to/input.mp4"
	output := "/path/to/output.mp4"
	filter := "scale=-2:1080"
	w, h := 1920, 1080
	cfg := config.TranscodeConfig{HardwareAccel: "none"}
	session := execution.ToolSession{Threads: 4, SoftwarePreset: "veryfast"}
	args := buildTranscodeArgs(input, output, filter, w, h, cfg, session)

	var threadsVal string
	var presetVal string
	for i, arg := range args {
		if arg == "-threads" && i+1 < len(args) {
			threadsVal = args[i+1]
		}
		if arg == "-preset" && i+1 < len(args) {
			presetVal = args[i+1]
		}
	}

	// ToolSession threads and preset must be honored; naked "-threads 0" is rejected.
	if threadsVal != "4" {
		t.Fatalf("software transcode threads = %q, want \"4\"", threadsVal)
	}
	if presetVal != "veryfast" {
		t.Fatalf("software transcode preset = %q, want \"veryfast\"", presetVal)
	}

	// Verify hardware accel modes never pass "-threads 0".
	for _, hwMode := range []string{"vaapi", "videotoolbox", "nvenc", "qsv"} {
		hwCfg := config.TranscodeConfig{HardwareAccel: hwMode}
		hwArgs := buildTranscodeArgs(input, output, filter, w, h, hwCfg, session)
		for i, arg := range hwArgs {
			if arg == "-threads" && i+1 < len(hwArgs) && hwArgs[i+1] == "0" {
				t.Fatalf("hardware mode %s passed -threads 0", hwMode)
			}
		}
	}
}
