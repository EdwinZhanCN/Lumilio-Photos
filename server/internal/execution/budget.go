package execution

import (
	"errors"
	"fmt"
	"strconv"
)

// ToolSession captures process execution parameters for external media tools
// (ffmpeg, ffprobe, libvips).
type ToolSession struct {
	Threads        int
	SoftwarePreset string
	HardwareAccel  string
}

// FFmpegThreadsArg returns the argv flag pair for video transcoding thread count.
func (s ToolSession) FFmpegThreadsArg() []string {
	threads := s.Threads
	if threads <= 0 {
		threads = 1
	}
	return []string{"-threads", strconv.Itoa(threads)}
}

// FFmpegPresetArg returns the argv flag pair for software transcode preset.
func (s ToolSession) FFmpegPresetArg() []string {
	preset := s.SoftwarePreset
	if preset == "" {
		preset = "veryfast"
	}
	return []string{"-preset", preset}
}

// NVENCPresetArg returns the argv flag pair for NVENC transcode preset.
func (s ToolSession) NVENCPresetArg() []string {
	return []string{"-preset", "p4"}
}

// SingleThreadArg returns the argv flag pair to constrain auxiliary tool tasks to 1 thread.
func (s ToolSession) SingleThreadArg() []string {
	return []string{"-threads", "1"}
}

// Budget represents the host capacity granted to execution, River macro workers,
// and tool session settings.
type Budget struct {
	CPU          int64
	DiskIO       int64
	ImageCodec   int64
	VideoCodec   int64
	Inference    int64
	MemoryBytes  int64
	MacroWorkers int
	MaxWaiting   int
	ToolSession  ToolSession
}

// Validate validates that budget capacities and tool settings are internally consistent.
func (b Budget) Validate() error {
	if b.CPU < 0 || b.DiskIO < 0 || b.ImageCodec < 0 || b.VideoCodec < 0 || b.Inference < 0 {
		return errors.New("execution budget resource capacities must be non-negative")
	}
	if b.CPU == 0 && b.ImageCodec == 0 && b.VideoCodec == 0 && b.Inference == 0 {
		return errors.New("execution budget requires at least one compute slot (cpu, image_codec, video_codec, inference)")
	}
	if b.MemoryBytes < (64 << 20) {
		return fmt.Errorf("execution budget memory_bytes %d must be at least 64 MiB", b.MemoryBytes)
	}
	if b.CPU >= 1 && b.ToolSession.Threads > int(b.CPU) {
		return fmt.Errorf("ffmpeg_threads (%d) cannot exceed execution cpu budget (%d)", b.ToolSession.Threads, b.CPU)
	}
	return nil
}

// DeriveMacroWorkers computes the River macro admission lane width from host capacities.
// Production formula: min(32, 2 * (CPU + ImageCodec + VideoCodec + Inference)), bounded to [2, 32].
func DeriveMacroWorkers(cpu, imageCodec, videoCodec, inference int64) int {
	w := int(2 * (cpu + imageCodec + videoCodec + inference))
	if w > 32 {
		w = 32
	}
	if w < 2 {
		w = 2
	}
	return w
}

// DerivedMacroWorkers returns MacroWorkers if explicitly set (>= 2), or the derived capacity formula.
func (b Budget) DerivedMacroWorkers() int {
	if b.MacroWorkers >= 2 {
		return b.MacroWorkers
	}
	return DeriveMacroWorkers(b.CPU, b.ImageCodec, b.VideoCodec, b.Inference)
}

// Governor constructs the production governor bounded by this budget.
func (b Budget) Governor() (*Governor, error) {
	if err := b.Validate(); err != nil {
		return nil, fmt.Errorf("invalid execution budget: %w", err)
	}
	capacity := Resources{
		CPU:         b.CPU,
		DiskIO:      b.DiskIO,
		ImageCodec:  b.ImageCodec,
		VideoCodec:  b.VideoCodec,
		Inference:   b.Inference,
		MemoryBytes: b.MemoryBytes,
	}
	maxWait := b.MaxWaiting
	if maxWait <= 0 {
		workers := b.DerivedMacroWorkers()
		maxWait = workers * 8
		if maxWait < 256 {
			maxWait = 256
		}
	}
	return NewGovernor(capacity, maxWait)
}

// DemandCatalog returns the sole resource-vector catalog for this immutable
// budget and its resolved tool session.
func (b Budget) DemandCatalog() DemandCatalog { return NewDemandCatalog(b.ToolSession) }
