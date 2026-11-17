package processors

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// ffmpeg's -threads and -preset come from the tool session, which sizes them
// from the execution budget (FFmpegThreadsArg, FFmpegPresetArg,
// NVENCPresetArg). A processor that spells either flag itself runs outside
// that budget.
func TestProcessorsTakeFFmpegThreadsAndPresetFromTheToolSession(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	var violations []string
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		source, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for number, line := range strings.Split(string(source), "\n") {
			if nakedFFmpegBudgetFlag(line) {
				violations = append(violations, file+":"+strconv.Itoa(number+1)+": "+strings.TrimSpace(line))
			}
		}
	}
	if len(violations) > 0 {
		t.Fatalf("processor spells ffmpeg budget flags itself:\n%s", strings.Join(violations, "\n"))
	}
	for line, want := range map[string]bool{
		`args = append(args, "-threads", "0")`:               true,
		`"-preset", "medium",`:                               true,
		`// "-threads" comes from the session`:               false,
		`args = append(args, session.FFmpegThreadsArg()...)`: false,
	} {
		if got := nakedFFmpegBudgetFlag(line); got != want {
			t.Errorf("nakedFFmpegBudgetFlag(%q) = %t, want %t", line, got, want)
		}
	}
}

func nakedFFmpegBudgetFlag(line string) bool {
	if strings.HasPrefix(strings.TrimSpace(line), "//") {
		return false
	}
	return strings.Contains(line, `"-threads"`) || strings.Contains(line, `"-preset"`)
}
