package commit

import (
	"time"

	hdrhistogram "github.com/HdrHistogram/hdrhistogram-go"
)

const (
	metricLowestMicroseconds  int64 = 1
	metricHighestMicroseconds int64 = int64((2 * time.Hour) / time.Microsecond)
	metricSignificantFigures        = 3
)

// DurationDistribution is the bounded, JSON-safe latency view published to
// runtime diagnostics. The coordinator retains only fixed-range HDR bucket
// counts; it never retains individual submissions or duration samples.
type DurationDistribution struct {
	Count    int64         `json:"count"`
	Total    time.Duration `json:"total_ns"`
	P50      time.Duration `json:"p50_ns"`
	P95      time.Duration `json:"p95_ns"`
	P99      time.Duration `json:"p99_ns"`
	Max      time.Duration `json:"max_ns"`
	Overflow uint64        `json:"overflow"`
}

// SizeDistribution is the bounded batch-size view published to runtime
// diagnostics. Its histogram range is fixed by Config.MaxBatch.
type SizeDistribution struct {
	Count    int64  `json:"count"`
	P50      int    `json:"p50"`
	P95      int    `json:"p95"`
	P99      int    `json:"p99"`
	Max      int    `json:"max"`
	Overflow uint64 `json:"overflow"`
}

type durationHistogram struct {
	histogram *hdrhistogram.Histogram
	total     time.Duration
	overflow  uint64
}

func newDurationHistogram() durationHistogram {
	return durationHistogram{histogram: hdrhistogram.New(
		metricLowestMicroseconds,
		metricHighestMicroseconds,
		metricSignificantFigures,
	)}
}

func (h *durationHistogram) record(duration time.Duration) {
	h.total += duration
	microseconds := duration.Microseconds()
	if duration > 0 && microseconds == 0 {
		microseconds = metricLowestMicroseconds
	}
	if microseconds < 0 {
		microseconds = 0
	}
	if microseconds > metricHighestMicroseconds {
		microseconds = metricHighestMicroseconds
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

type sizeHistogram struct {
	histogram *hdrhistogram.Histogram
	highest   int64
	overflow  uint64
}

func newSizeHistogram(highest int) sizeHistogram {
	return sizeHistogram{histogram: hdrhistogram.New(1, int64(highest), 1), highest: int64(highest)}
}

func (h *sizeHistogram) record(size int) {
	value := int64(size)
	if value < 1 {
		value = 1
	}
	if value > h.highest {
		value = h.highest
		h.overflow++
	}
	_ = h.histogram.RecordValue(value)
}

func (h *sizeHistogram) snapshot() SizeDistribution {
	count := h.histogram.TotalCount()
	if count == 0 {
		return SizeDistribution{}
	}
	return SizeDistribution{
		Count:    count,
		P50:      int(h.histogram.ValueAtQuantile(50)),
		P95:      int(h.histogram.ValueAtQuantile(95)),
		P99:      int(h.histogram.ValueAtQuantile(99)),
		Max:      int(h.histogram.Max()),
		Overflow: h.overflow,
	}
}
