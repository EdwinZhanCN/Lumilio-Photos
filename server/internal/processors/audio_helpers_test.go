package processors

import (
	"slices"
	"testing"

	"server/internal/execution"
)

func TestBuildAudioTranscodeArgsKeepsMonoChannelAndFormatFlagsDistinct(t *testing.T) {
	t.Parallel()

	args := buildAudioTranscodeArgs("source.wav", "output.mp3", "128k", 1, execution.ToolSession{Threads: 2})
	wantTail := []string{"-ar", "44100", "-ac", "1", "-threads", "2", "-f", "mp3", "-y", "output.mp3"}
	if !slices.Equal(args[len(args)-len(wantTail):], wantTail) {
		t.Fatalf("audio transcode tail = %v, want %v", args[len(args)-len(wantTail):], wantTail)
	}
}
