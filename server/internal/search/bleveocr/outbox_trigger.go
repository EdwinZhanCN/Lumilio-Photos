package bleveocr

import (
	"sync"
	"sync/atomic"
	"time"
)

const (
	DefaultOutboxWakeInterval     = time.Second
	DefaultOutboxRecoveryInterval = time.Minute
)

// OutboxTrigger turns best-effort mutation notifications into coalesced drain
// requests. The SQLite outbox remains authoritative: an asynchronous reader
// probe calls Notify when a notification is missed or the process restarts.
type OutboxTrigger struct {
	pending atomic.Bool

	mu            sync.Mutex
	lastScheduled time.Time
}

func NewOutboxTrigger() *OutboxTrigger {
	return &OutboxTrigger{}
}

// Notify records that a committed OCR mutation may need to be drained. Many
// concurrent notifications collapse into one pending wakeup.
func (t *OutboxTrigger) Notify() {
	if t != nil {
		t.pending.Store(true)
	}
}

// ConsumePending is the non-blocking scheduler boundary. Durable recovery
// probes run outside River's PeriodicJobConstructor and call Notify only after
// observing authoritative outbox work; the constructor only consumes this
// process-local hint.
func (t *OutboxTrigger) ConsumePending() bool {
	return t != nil && t.pending.Swap(false)
}

// ShouldSchedule consumes one pending wakeup or requests a recovery drain when
// the fallback interval has elapsed. Calls are safe from concurrent periodic
// scheduler callbacks, though River currently invokes a constructor serially.
func (t *OutboxTrigger) ShouldSchedule(now time.Time, recoveryInterval time.Duration) bool {
	if t == nil {
		return false
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	woken := t.ConsumePending()
	recoveryDue := t.lastScheduled.IsZero() || recoveryInterval <= 0 || !now.Before(t.lastScheduled.Add(recoveryInterval))
	if !woken && !recoveryDue {
		return false
	}

	t.lastScheduled = now
	return true
}
