// Package execution owns process-wide admission for fine-grained pipeline work.
package execution

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

type Class int

const (
	ClassMaintenance Class = 1
	ClassBackground  Class = 2
	ClassInteractive Class = 3
)

func (c Class) String() string {
	switch c {
	case ClassInteractive:
		return "interactive"
	case ClassBackground:
		return "background"
	case ClassMaintenance:
		return "maintenance"
	default:
		return "background"
	}
}

func (c Class) Valid() bool {
	return c >= ClassMaintenance && c <= ClassInteractive
}

// ClassFromAdmission maps the serialized pipeline value at the app composition
// boundary without making execution depend on the pipeline package.
func ClassFromAdmission(admission string) (Class, error) {
	switch admission {
	case "interactive":
		return ClassInteractive, nil
	case "background":
		return ClassBackground, nil
	case "maintenance":
		return ClassMaintenance, nil
	default:
		return 0, fmt.Errorf("invalid execution admission class %q", admission)
	}
}

type Resources struct {
	CPU         int64 `json:"cpu"`
	DiskIO      int64 `json:"disk_io"`
	ImageCodec  int64 `json:"image_codec"`
	VideoCodec  int64 `json:"video_codec"`
	Inference   int64 `json:"inference"`
	MemoryBytes int64 `json:"memory_bytes"`
}

func (r Resources) valid() bool {
	return r.CPU >= 0 && r.DiskIO >= 0 && r.ImageCodec >= 0 && r.VideoCodec >= 0 && r.Inference >= 0 && r.MemoryBytes >= 0
}

func (r Resources) fits(cap Resources) bool {
	return r.CPU <= cap.CPU && r.DiskIO <= cap.DiskIO && r.ImageCodec <= cap.ImageCodec && r.VideoCodec <= cap.VideoCodec && r.Inference <= cap.Inference && r.MemoryBytes <= cap.MemoryBytes
}

func add(a, b Resources) Resources {
	return Resources{a.CPU + b.CPU, a.DiskIO + b.DiskIO, a.ImageCodec + b.ImageCodec, a.VideoCodec + b.VideoCodec, a.Inference + b.Inference, a.MemoryBytes + b.MemoryBytes}
}
func sub(a, b Resources) Resources {
	return Resources{a.CPU - b.CPU, a.DiskIO - b.DiskIO, a.ImageCodec - b.ImageCodec, a.VideoCodec - b.VideoCodec, a.Inference - b.Inference, a.MemoryBytes - b.MemoryBytes}
}

var ErrWaitQueueFull = errors.New("execution admission queue is full")

type Snapshot struct {
	Capacity         Resources            `json:"capacity"`
	InUse            Resources            `json:"in_use"`
	QueueCapacity    int                  `json:"queue_capacity"`
	Waiting          int                  `json:"waiting"`
	PeakWaiting      int                  `json:"peak_waiting"`
	WaitingResources Resources            `json:"waiting_resources"`
	Wait             DurationDistribution `json:"wait"`
	WaitByResource   ResourceWaitSnapshot `json:"wait_by_resource"`
	Admitted         uint64               `json:"admitted"`
	Canceled         uint64               `json:"canceled"`
	Rejected         uint64               `json:"rejected"`
}

type request struct {
	class     Class
	resources Resources
	ready     chan struct{}
	enqueued  time.Time
	canceled  bool
}

type Governor struct {
	mu       sync.Mutex
	capacity Resources
	inUse    Resources
	waiters  []*request
	maxWait  int
	admitted uint64
	canceled uint64
	rejected uint64
	peakWait int
	wait     durationHistogram
	waitBy   resourceWaitHistograms
}

func NewGovernor(capacity Resources, maxWaiting int) (*Governor, error) {
	if !capacity.valid() || maxWaiting < 0 {
		return nil, errors.New("resource capacity and wait bound must be non-negative")
	}
	return &Governor{
		capacity: capacity, maxWait: maxWaiting,
		wait: newDurationHistogram(), waitBy: newResourceWaitHistograms(),
	}, nil
}

// Acquire is atomic across all resource classes. Waiters that need a resource
// currently blocking the head remain FIFO; work that is independent of every
// blocked head resource may backfill otherwise idle capacity. A higher class
// may bypass a blocked lower-class head.
func (g *Governor) Acquire(ctx context.Context, class Class, resources Resources) (func(), error) {
	if ctx == nil {
		return nil, errors.New("resource acquisition context is nil")
	}
	if !class.Valid() {
		class = ClassBackground
	}
	if !resources.valid() || !resources.fits(g.capacity) {
		return nil, fmt.Errorf("resource request exceeds capacity: request=%+v capacity=%+v", resources, g.capacity)
	}
	if err := ctx.Err(); err != nil {
		g.mu.Lock()
		g.canceled++
		g.mu.Unlock()
		return nil, err
	}
	g.mu.Lock()
	if len(g.waiters) == 0 && add(g.inUse, resources).fits(g.capacity) {
		g.inUse = add(g.inUse, resources)
		g.admitted++
		g.mu.Unlock()
		return g.releaseOnce(resources), nil
	}
	if len(g.waiters) >= g.maxWait {
		g.rejected++
		g.mu.Unlock()
		return nil, ErrWaitQueueFull
	}
	req := &request{class: class, resources: resources, ready: make(chan struct{}), enqueued: time.Now()}
	g.waiters = append(g.waiters, req)
	if len(g.waiters) > g.peakWait {
		g.peakWait = len(g.waiters)
	}
	g.admitLocked()
	g.mu.Unlock()

	select {
	case <-req.ready:
		return g.releaseOnce(resources), nil
	case <-ctx.Done():
		g.mu.Lock()
		select {
		case <-req.ready:
			g.mu.Unlock()
			g.release(resources)
		default:
			req.canceled = true
			g.canceled++
			g.observeWaitLocked(req.resources, time.Since(req.enqueued))
			g.admitLocked()
			g.mu.Unlock()
		}
		return nil, ctx.Err()
	}
}

func (g *Governor) releaseOnce(resources Resources) func() {
	var once sync.Once
	return func() { once.Do(func() { g.release(resources) }) }
}

func (g *Governor) release(resources Resources) {
	g.mu.Lock()
	g.inUse = sub(g.inUse, resources)
	g.admitLocked()
	g.mu.Unlock()
}

func (g *Governor) admitLocked() {
	for len(g.waiters) > 0 {
		g.removeCanceledLocked()
		if len(g.waiters) == 0 {
			return
		}
		index := g.nextAdmissibleLocked()
		if index < 0 {
			return
		}
		req := g.waiters[index]
		g.waiters = append(g.waiters[:index], g.waiters[index+1:]...)
		g.inUse = add(g.inUse, req.resources)
		g.admitted++
		g.observeWaitLocked(req.resources, time.Since(req.enqueued))
		close(req.ready)
	}
}

func (g *Governor) removeCanceledLocked() {
	kept := g.waiters[:0]
	for _, req := range g.waiters {
		if !req.canceled {
			kept = append(kept, req)
		}
	}
	g.waiters = kept
}

func (g *Governor) nextAdmissibleLocked() int {
	bestIndex := -1
	var bestClass Class

	for index, candidate := range g.waiters {
		if !add(g.inUse, candidate.resources).fits(g.capacity) {
			continue
		}

		canAdmit := true
		for j := 0; j < index; j++ {
			preceding := g.waiters[j]
			if candidate.class > preceding.class {
				// A higher class may bypass a blocked lower-class head.
				continue
			}
			blocked := blockedResources(g.inUse, preceding.resources, g.capacity)
			if overlaps(candidate.resources, blocked) {
				canAdmit = false
				break
			}
		}

		if !canAdmit {
			continue
		}

		if bestIndex == -1 || candidate.class > bestClass {
			bestIndex = index
			bestClass = candidate.class
		}
	}

	return bestIndex
}

func blockedResources(inUse, requested, capacity Resources) Resources {
	var blocked Resources
	if inUse.CPU+requested.CPU > capacity.CPU {
		blocked.CPU = 1
	}
	if inUse.DiskIO+requested.DiskIO > capacity.DiskIO {
		blocked.DiskIO = 1
	}
	if inUse.ImageCodec+requested.ImageCodec > capacity.ImageCodec {
		blocked.ImageCodec = 1
	}
	if inUse.VideoCodec+requested.VideoCodec > capacity.VideoCodec {
		blocked.VideoCodec = 1
	}
	if inUse.Inference+requested.Inference > capacity.Inference {
		blocked.Inference = 1
	}
	if inUse.MemoryBytes+requested.MemoryBytes > capacity.MemoryBytes {
		blocked.MemoryBytes = 1
	}
	return blocked
}

func overlaps(requested, blocked Resources) bool {
	return requested.CPU > 0 && blocked.CPU > 0 ||
		requested.DiskIO > 0 && blocked.DiskIO > 0 ||
		requested.ImageCodec > 0 && blocked.ImageCodec > 0 ||
		requested.VideoCodec > 0 && blocked.VideoCodec > 0 ||
		requested.Inference > 0 && blocked.Inference > 0 ||
		requested.MemoryBytes > 0 && blocked.MemoryBytes > 0
}

func (g *Governor) Snapshot() Snapshot {
	g.mu.Lock()
	defer g.mu.Unlock()
	waiting := 0
	var waitingResources Resources
	for _, req := range g.waiters {
		if !req.canceled {
			waiting++
			waitingResources = add(waitingResources, req.resources)
		}
	}
	return Snapshot{
		Capacity: g.capacity, InUse: g.inUse, QueueCapacity: g.maxWait,
		Waiting: waiting, PeakWaiting: g.peakWait, WaitingResources: waitingResources,
		Wait: g.wait.snapshot(), WaitByResource: g.waitBy.snapshot(),
		Admitted: g.admitted, Canceled: g.canceled, Rejected: g.rejected,
	}
}

func (g *Governor) observeWaitLocked(resources Resources, duration time.Duration) {
	g.wait.record(duration)
	g.waitBy.record(resources, duration)
}

type Engine struct{ governor *Governor }

func NewEngine(governor *Governor) *Engine { return &Engine{governor: governor} }

func (e *Engine) Run(ctx context.Context, class Class, resources Resources, work func(context.Context) error) (err error) {
	if e == nil || e.governor == nil {
		return errors.New("execution engine is not configured")
	}
	if work == nil {
		return errors.New("execution work is nil")
	}
	release, err := e.governor.Acquire(ctx, class, resources)
	if err != nil {
		return err
	}
	defer release()
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("execution panic: %v", recovered)
		}
	}()
	return work(ctx)
}
