package processors

import (
	"slices"
	"testing"
)

func TestBuildAudioTranscodeArgsKeepsMonoChannelAndFormatFlagsDistinct(t *testing.T) {
	t.Parallel()

	args := buildAudioTranscodeArgs("source.wav", "output.mp3", "128k", 1)
	wantTail := []string{"-ar", "44100", "-ac", "1", "-f", "mp3", "-y", "output.mp3"}
	if !slices.Equal(args[len(args)-len(wantTail):], wantTail) {
		t.Fatalf("audio transcode tail = %v, want %v", args[len(args)-len(wantTail):], wantTail)
	}
}
