package queue

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/riverqueue/river/riverdriver"
	"github.com/riverqueue/river/rivertype"
)

// continuationWake carries committed short-snooze hints to the local listener.
// A timer per Wait call covers all pending jobs; no detached timer goroutines
// or QueueDB writes are needed. Durable jobs and fallback polling own recovery.
type continuationWake struct {
	mu      sync.Mutex
	due     map[int64]continuationDeadline
	changed chan struct{}
}
type continuationDeadline struct {
	queue string
	at    time.Time
}

func newContinuationWake() *continuationWake {
	return &continuationWake{due: make(map[int64]continuationDeadline), changed: make(chan struct{}, 1)}
}
func (w *continuationWake) add(rows []*rivertype.JobRow) {
	w.mu.Lock()
	for _, row := range rows {
		if row.State == rivertype.JobStateAvailable {
			w.due[row.ID] = continuationDeadline{row.Queue, row.ScheduledAt}
		} else {
			delete(w.due, row.ID)
		}
	}
	w.mu.Unlock()
	select {
	case w.changed <- struct{}{}:
	default:
	}
}
func (w *continuationWake) next(now time.Time) (*riverdriver.Notification, time.Time) {
	w.mu.Lock()
	defer w.mu.Unlock()
	var earliest time.Time
	var queue string
	for _, d := range w.due {
		if !d.at.After(now) {
			queue = d.queue
			break
		}
		if earliest.IsZero() || d.at.Before(earliest) {
			earliest = d.at
		}
	}
	if queue == "" {
		return nil, earliest
	}
	for id, d := range w.due {
		if d.queue == queue && !d.at.After(now) {
			delete(w.due, id)
		}
	}
	payload, _ := json.Marshal(struct {
		Queue string `json:"queue"`
	}{queue})
	return &riverdriver.Notification{Topic: "river_insert", Payload: string(payload)}, time.Time{}
}

type continuationExecutor struct {
	riverdriver.Executor
	wake *continuationWake
}

func (e *continuationExecutor) JobSetStateIfRunningMany(ctx context.Context, params *riverdriver.JobSetStateIfRunningManyParams) ([]*rivertype.JobRow, error) {
	rows, err := e.Executor.JobSetStateIfRunningMany(ctx, params)
	if err == nil {
		e.wake.add(rows)
	} // upstream commits before returning
	return rows, err
}

type continuationListener struct {
	riverdriver.Listener
	wake    *continuationWake
	pending *riverdriver.Notification
}

func (l *continuationListener) WaitForNotification(ctx context.Context) (*riverdriver.Notification, error) {
	if l.pending != nil {
		n := l.pending
		l.pending = nil
		return n, nil
	}
	child, cancel := context.WithCancel(ctx)
	type result struct {
		notification *riverdriver.Notification
		err          error
	}
	received := make(chan result, 1)
	go func() { n, err := l.Listener.WaitForNotification(child); received <- result{n, err} }()
	joined := false
	defer func() {
		cancel()
		if !joined {
			// Preserve an upstream notification racing a local wake.
			l.pending = (<-received).notification
		}
	}()
	timer := time.NewTimer(time.Hour)
	defer timer.Stop()
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		notification, at := l.wake.next(time.Now())
		if notification != nil {
			return notification, nil
		}
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		var due <-chan time.Time
		if !at.IsZero() {
			timer.Reset(time.Until(at))
			due = timer.C
		}
		select {
		case result := <-received:
			joined = true
			return result.notification, result.err
		case <-l.wake.changed:
		case <-due:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}
