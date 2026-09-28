package scan

import (
	"context"
	"errors"
	"math/rand"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/syncthing/notify"
	"go.uber.org/zap"
)

type fakeEvent string

func (e fakeEvent) Event() notify.Event { return notify.Write }
func (e fakeEvent) Path() string        { return string(e) }
func (e fakeEvent) Sys() any            { return nil }

type recordedRequest struct {
	trigger string
	scope   string
}

// watchHarness runs one repository watch against a fake event source and
// records the scan requests it makes.
type watchHarness struct {
	root     string
	events   chan<- notify.EventInfo
	started  chan struct{}
	mu       sync.Mutex
	requests []recordedRequest
	watcher  *Watcher
}

func newWatchHarness(t *testing.T, config WatchConfig, startErrors int) *watchHarness {
	t.Helper()
	h := &watchHarness{root: t.TempDir(), started: make(chan struct{}, 8)}
	attempts := 0
	config.watch = func(path string, events chan<- notify.EventInfo) error {
		attempts++
		if attempts <= startErrors {
			return errors.New("watch limit reached")
		}
		h.mu.Lock()
		h.events = events
		h.mu.Unlock()
		h.started <- struct{}{}
		return nil
	}
	config.stop = func(chan<- notify.EventInfo) {}
	h.watcher = &Watcher{config: config.withDefaults(), watches: map[uuid.UUID]*repositoryWatch{}}
	h.watcher.logger = nopLogger()
	h.watcher.request = func(_ context.Context, _ uuid.UUID, trigger, scope string) error {
		h.mu.Lock()
		h.requests = append(h.requests, recordedRequest{trigger: trigger, scope: scope})
		h.mu.Unlock()
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		h.watcher.watchRepository(ctx, uuid.New(), h.root)
	}()
	t.Cleanup(func() { cancel(); <-done })
	return h
}

func (h *watchHarness) send(relative ...string) {
	h.mu.Lock()
	events := h.events
	h.mu.Unlock()
	for _, path := range relative {
		events <- fakeEvent(filepath.Join(h.root, filepath.FromSlash(path)))
	}
}

func (h *watchHarness) waitRequests(t *testing.T, want int) []recordedRequest {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		h.mu.Lock()
		got := append([]recordedRequest(nil), h.requests...)
		h.mu.Unlock()
		if len(got) >= want {
			return got
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("watcher made %d scan requests, want %d", len(h.requests), want)
	return nil
}

func TestWatcherBatchesEventsIntoTheSmallestCoveringScan(t *testing.T) {
	h := newWatchHarness(t, WatchConfig{Delay: 50 * time.Millisecond}, 0)
	<-h.started
	h.send("trips/2026/a.jpg", "trips/2026/b.jpg", "trips/2025/c.jpg")
	requests := h.waitRequests(t, 1)
	if len(requests) != 1 || requests[0] != (recordedRequest{trigger: TriggerWatcher, scope: "trips"}) {
		t.Fatalf("requests = %+v, want one watcher scan of trips", requests)
	}
}

func TestWatcherTurnsAnEventFloodIntoAFullScan(t *testing.T) {
	h := newWatchHarness(t, WatchConfig{Delay: 50 * time.Millisecond, FullScanEvents: 4}, 0)
	<-h.started
	h.send("a/1.jpg", "a/2.jpg", "b/3.jpg", "b/4.jpg", "c/5.jpg")
	requests := h.waitRequests(t, 1)
	if requests[0].scope != "" {
		t.Fatalf("flood requested scope %q, want the whole repository", requests[0].scope)
	}
}

func TestWatcherIgnoresLumilioPrivateFiles(t *testing.T) {
	h := newWatchHarness(t, WatchConfig{Delay: 20 * time.Millisecond}, 0)
	<-h.started
	h.send(".lumilio/staging/upload.tmp", ".lumiliorepo")
	time.Sleep(100 * time.Millisecond)
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.requests) != 0 {
		t.Fatalf("private writes requested scans: %+v", h.requests)
	}
}

func TestWatcherFailureSchedulesAFullScanAndRetries(t *testing.T) {
	h := newWatchHarness(t, WatchConfig{Delay: 20 * time.Millisecond, RetryMin: 10 * time.Millisecond}, 2)
	requests := h.waitRequests(t, 2)
	for _, request := range requests[:2] {
		if request.scope != "" {
			t.Fatalf("watch failure requested scope %q, want a full scan", request.scope)
		}
	}
	select {
	case <-h.started:
	case <-time.After(5 * time.Second):
		t.Fatal("watch was not retried after failing")
	}
}

func TestPeriodicDelayStaysWithinJitterBounds(t *testing.T) {
	random := rand.New(rand.NewSource(1))
	interval := time.Hour
	for range 1000 {
		delay := PeriodicDelay(interval, random)
		if delay < interval*3/4 || delay > interval*5/4 {
			t.Fatalf("periodic delay %s outside [45m, 75m]", delay)
		}
	}
}

func nopLogger() *zap.Logger { return zap.NewNop() }
