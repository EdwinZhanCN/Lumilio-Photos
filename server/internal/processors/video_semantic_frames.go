package processors

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"server/internal/settings"
	"server/internal/utils/sysproc"
)

// semanticFrame is one extracted JPEG frame tagged with its presentation
// timestamp in milliseconds.
type semanticFrame struct {
	Bytes     []byte
	FrameTsMs int32
}

type frameSamplingStrategy int

const (
	frameStrategyMidpoint frameSamplingStrategy = iota
	frameStrategyScene
	frameStrategyInterval
)

const shortVideoSeconds = 4.0

func chooseFrameSamplingStrategy(durationSec float64, longThresholdSec int) frameSamplingStrategy {
	if durationSec < shortVideoSeconds {
		return frameStrategyMidpoint
	}
	if durationSec >= float64(longThresholdSec) {
		return frameStrategyInterval
	}
	return frameStrategyScene
}

func subsampleTimestamps(timestamps []int32, maxN int) []int32 {
	if maxN <= 0 || len(timestamps) <= maxN {
		return timestamps
	}
	if maxN == 1 {
		return []int32{timestamps[len(timestamps)/2]}
	}
	out := make([]int32, 0, maxN)
	last := len(timestamps) - 1
	for i := 0; i < maxN; i++ {
		idx := int(math.Round(float64(i) * float64(last) / float64(maxN-1)))
		out = append(out, timestamps[idx])
	}
	return dedupeSortedTimestamps(out)
}

func dedupeSortedTimestamps(timestamps []int32) []int32 {
	if len(timestamps) == 0 {
		return timestamps
	}
	sort.Slice(timestamps, func(i, j int) bool { return timestamps[i] < timestamps[j] })
	out := make([]int32, 0, len(timestamps))
	var prev int32
	for i, ts := range timestamps {
		if i == 0 || ts != prev {
			out = append(out, ts)
			prev = ts
		}
	}
	return out
}

func uniformIntervalTimestamps(durationSec float64, maxN int) []int32 {
	if maxN <= 0 {
		return nil
	}
	if durationSec <= 0 {
		return []int32{0}
	}
	if maxN == 1 {
		return []int32{int32(math.Round(durationSec * 500))}
	}
	interval := durationSec / float64(maxN)
	out := make([]int32, 0, maxN)
	for i := 0; i < maxN; i++ {
		sec := (float64(i) + 0.5) * interval
		if sec >= durationSec {
			sec = durationSec - 0.001
		}
		if sec < 0 {
			sec = 0
		}
		out = append(out, int32(math.Round(sec*1000)))
	}
	return dedupeSortedTimestamps(out)
}

func midpointTimestampMs(durationSec float64) int32 {
	if durationSec <= 0 {
		return 0
	}
	return int32(math.Round(durationSec * 500))
}

func webVideoPath(repoPath, contentHash string) string {
	filename := fmt.Sprintf("%s_web.mp4", contentHash)
	return filepath.Join(repoPath, ".lumilio/assets/videos/web", filename)
}

// extractSemanticFrames samples up to N_max frames from a transcoded web.mp4
// using the duration-based strategy from settings.ML. The caller owns cleanup
// of nothing — frames are returned as in-memory JPEG bytes.
func (ap *AssetProcessor) extractSemanticFrames(
	ctx context.Context,
	webPath string,
	durationSec float64,
	cfg settings.ML,
) ([]semanticFrame, error) {
	maxN := cfg.EffectiveVideoMaxFrames()
	longThreshold := cfg.EffectiveVideoLongThresholdSeconds()
	sceneThreshold := cfg.EffectiveVideoSceneThreshold()

	strategy := chooseFrameSamplingStrategy(durationSec, longThreshold)
	switch strategy {
	case frameStrategyMidpoint:
		ts := midpointTimestampMs(durationSec)
		frame, err := ap.extractFrameAt(ctx, webPath, ts)
		if err != nil {
			return nil, err
		}
		return []semanticFrame{frame}, nil
	case frameStrategyInterval:
		timestamps := uniformIntervalTimestamps(durationSec, maxN)
		return ap.extractFramesAtTimestamps(ctx, webPath, timestamps)
	default:
		timestamps, err := ap.detectSceneTimestamps(ctx, webPath, sceneThreshold, durationSec)
		if err != nil || len(timestamps) == 0 {
			// Scene detection can fail on nearly-static clips; fall back to
			// uniform sampling so the embed path still runs.
			timestamps = uniformIntervalTimestamps(durationSec, maxN)
		} else {
			timestamps = subsampleTimestamps(timestamps, maxN)
		}
		return ap.extractFramesAtTimestamps(ctx, webPath, timestamps)
	}
}

func (ap *AssetProcessor) extractFramesAtTimestamps(ctx context.Context, webPath string, timestamps []int32) ([]semanticFrame, error) {
	frames := make([]semanticFrame, 0, len(timestamps))
	for _, ts := range timestamps {
		frame, err := ap.extractFrameAt(ctx, webPath, ts)
		if err != nil {
			return nil, fmt.Errorf("extract frame at %dms: %w", ts, err)
		}
		frames = append(frames, frame)
	}
	return frames, nil
}

func (ap *AssetProcessor) extractFrameAt(ctx context.Context, webPath string, tsMs int32) (semanticFrame, error) {
	outputPath := filepath.Join(os.TempDir(), fmt.Sprintf("semantic_frame_%d_%d.jpg", os.Getpid(), tsMs))
	defer os.Remove(outputPath)

	ss := formatFFmpegTimestamp(tsMs)
	args := []string{
		"-ss", ss,
		"-i", webPath,
		"-frames:v", "1",
		"-q:v", "2",
		"-f", "image2",
		"-y",
		outputPath,
	}
	cmd := exec.CommandContext(ctx, ap.toolsConfig.FFmpegCommand(), args...)
	sysproc.HideConsole(cmd)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return semanticFrame{}, fmt.Errorf("ffmpeg extract frame: %w\nstderr: %s", err, stderr.String())
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		return semanticFrame{}, fmt.Errorf("read extracted frame: %w", err)
	}
	if len(data) == 0 {
		return semanticFrame{}, fmt.Errorf("extracted frame is empty at %dms", tsMs)
	}
	return semanticFrame{Bytes: data, FrameTsMs: tsMs}, nil
}

func formatFFmpegTimestamp(tsMs int32) string {
	if tsMs < 0 {
		tsMs = 0
	}
	totalMs := int(tsMs)
	hours := totalMs / 3_600_000
	totalMs %= 3_600_000
	minutes := totalMs / 60_000
	totalMs %= 60_000
	seconds := totalMs / 1000
	millis := totalMs % 1000
	return fmt.Sprintf("%02d:%02d:%02d.%03d", hours, minutes, seconds, millis)
}

// detectSceneTimestamps runs ffmpeg scene detection and returns cut timestamps
// in milliseconds. Uses showinfo on selected frames.
func (ap *AssetProcessor) detectSceneTimestamps(
	ctx context.Context,
	webPath string,
	sceneThreshold float64,
	durationSec float64,
) ([]int32, error) {
	filter := fmt.Sprintf(`select='gt(scene,%g)',showinfo`, sceneThreshold)
	args := []string{
		"-i", webPath,
		"-vf", filter,
		"-vsync", "vfr",
		"-f", "null",
		"-",
	}
	cmd := exec.CommandContext(ctx, ap.toolsConfig.FFmpegCommand(), args...)
	sysproc.HideConsole(cmd)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	// ffmpeg writes showinfo to stderr; a non-zero exit can still leave usable output.
	_ = cmd.Run()

	timestamps := parseShowinfoTimestamps(stderr.String())
	if len(timestamps) == 0 {
		return nil, fmt.Errorf("no scene cuts detected")
	}

	// Always include midpoint coverage for very sparse cut lists by keeping
	// cuts within duration; drop anything past EOF.
	maxMs := int32(math.Max(0, math.Round(durationSec*1000)))
	filtered := make([]int32, 0, len(timestamps))
	for _, ts := range timestamps {
		if maxMs > 0 && ts > maxMs {
			continue
		}
		filtered = append(filtered, ts)
	}
	return dedupeSortedTimestamps(filtered), nil
}

func parseShowinfoTimestamps(stderr string) []int32 {
	var out []int32
	for _, line := range strings.Split(stderr, "\n") {
		if !strings.Contains(line, "pts_time:") {
			continue
		}
		idx := strings.Index(line, "pts_time:")
		if idx < 0 {
			continue
		}
		rest := line[idx+len("pts_time:"):]
		rest = strings.TrimLeft(rest, " \t")
		end := 0
		for end < len(rest) {
			c := rest[end]
			if (c < '0' || c > '9') && c != '.' && c != '-' && c != 'e' && c != 'E' && c != '+' {
				break
			}
			end++
		}
		if end == 0 {
			continue
		}
		sec, err := strconv.ParseFloat(rest[:end], 64)
		if err != nil || sec < 0 {
			continue
		}
		out = append(out, int32(math.Round(sec*1000)))
	}
	return out
}
