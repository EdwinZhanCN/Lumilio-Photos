package execution

import (
	"time"

	hdrhistogram "github.com/HdrHistogram/hdrhistogram-go"
)

const (
	waitLowestMicroseconds  int64 = 1
	waitHighestMicroseconds int64 = int64((2 * time.Hour) / time.Microsecond)
	waitSignificantFigures        = 3
)

// DurationDistribution is the bounded, JSON-safe resource-wait view published
// to runtime diagnostics. The governor retains fixed HDR buckets rather than
// individual acquisition samples.
type DurationDistribution struct {
	Count    int64         `json:"count"`
	Total    time.Duration `json:"total_ns"`
	P50      time.Duration `json:"p50_ns"`
	P95      time.Duration `json:"p95_ns"`
	P99      time.Duration `json:"p99_ns"`
	Max      time.Duration `json:"max_ns"`
	Overflow uint64        `json:"overflow"`
}

type ResourceWaitSnapshot struct {
	CPU        DurationDistribution `json:"cpu"`
	DiskIO     DurationDistribution `json:"disk_io"`
	ImageCodec DurationDistribution `json:"image_codec"`
	VideoCodec DurationDistribution `json:"video_codec"`
	Inference  DurationDistribution `json:"inference"`
	Memory     DurationDistribution `json:"memory"`
}

type durationHistogram struct {
	histogram *hdrhistogram.Histogram
	total     time.Duration
	overflow  uint64
}

func newDurationHistogram() durationHistogram {
	return durationHistogram{histogram: hdrhistogram.New(
		waitLowestMicroseconds,
		waitHighestMicroseconds,
		waitSignificantFigures,
	)}
}

func (h *durationHistogram) record(duration time.Duration) {
	h.total += duration
	microseconds := duration.Microseconds()
	if duration > 0 && microseconds == 0 {
		microseconds = waitLowestMicroseconds
	}
	if microseconds < 0 {
		microseconds = 0
	}
	if microseconds > waitHighestMicroseconds {
		microseconds = waitHighestMicroseconds
		h.overflow++
	}
	_ = h.histogram.RecordValue(microseconds)
}

func (h *durationHistogram) snapshot() DurationDistribution {
	count := h.histogram.TotalCount()
	if count == 0 {
		return DurationDistribution{}
	}
	return DurationDistribution{
		Count:    count,
		Total:    h.total,
		P50:      time.Duration(h.histogram.ValueAtQuantile(50)) * time.Microsecond,
		P95:      time.Duration(h.histogram.ValueAtQuantile(95)) * time.Microsecond,
		P99:      time.Duration(h.histogram.ValueAtQuantile(99)) * time.Microsecond,
		Max:      time.Duration(h.histogram.Max()) * time.Microsecond,
		Overflow: h.overflow,
	}
}

type resourceWaitHistograms struct {
	cpu, diskIO, imageCodec, videoCodec, inference, memory durationHistogram
}

func newResourceWaitHistograms() resourceWaitHistograms {
	return resourceWaitHistograms{
		cpu: newDurationHistogram(), diskIO: newDurationHistogram(),
		imageCodec: newDurationHistogram(), videoCodec: newDurationHistogram(),
		inference: newDurationHistogram(), memory: newDurationHistogram(),
	}
}

func (h *resourceWaitHistograms) record(resources Resources, duration time.Duration) {
	if resources.CPU > 0 {
		h.cpu.record(duration)
	}
	if resources.DiskIO > 0 {
		h.diskIO.record(duration)
	}
	if resources.ImageCodec > 0 {
		h.imageCodec.record(duration)
	}
	if resources.VideoCodec > 0 {
		h.videoCodec.record(duration)
	}
	if resources.Inference > 0 {
		h.inference.record(duration)
	}
	if resources.MemoryBytes > 0 {
		h.memory.record(duration)
	}
}

func (h *resourceWaitHistograms) snapshot() ResourceWaitSnapshot {
	return ResourceWaitSnapshot{
		CPU: h.cpu.snapshot(), DiskIO: h.diskIO.snapshot(),
		ImageCodec: h.imageCodec.snapshot(), VideoCodec: h.videoCodec.snapshot(),
		Inference: h.inference.snapshot(), Memory: h.memory.snapshot(),
	}
}
