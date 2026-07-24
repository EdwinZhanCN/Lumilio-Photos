package ratelimit

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestLimiterLocksAndRecovers(t *testing.T) {
	now := time.Date(2026, time.July, 24, 12, 0, 0, 0, time.UTC)
	limiter, err := newWithClock(Policy{
		Attempts:   3,
		Window:     time.Minute,
		Lockout:    2 * time.Minute,
		MaxEntries: 10,
	}, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}

	for attempt := 1; attempt <= 3; attempt++ {
		if decision := limiter.Allow("subject"); !decision.Allowed {
			t.Fatalf("attempt %d denied: %+v", attempt, decision)
		}
	}
	if decision := limiter.Allow("subject"); decision.Allowed || decision.RetryAfter != 2*time.Minute {
		t.Fatalf("lockout decision = %+v", decision)
	}

	now = now.Add(time.Minute)
	if decision := limiter.Allow("subject"); decision.Allowed || decision.RetryAfter != time.Minute {
		t.Fatalf("mid-lockout decision = %+v", decision)
	}

	now = now.Add(time.Minute)
	if decision := limiter.Allow("subject"); !decision.Allowed {
		t.Fatalf("post-lockout decision = %+v", decision)
	}
}

func TestLimiterResetsAttempts(t *testing.T) {
	limiter, err := New(Policy{
		Attempts:   1,
		Window:     time.Minute,
		Lockout:    time.Minute,
		MaxEntries: 10,
	})
	if err != nil {
		t.Fatal(err)
	}

	if decision := limiter.Allow("subject"); !decision.Allowed {
		t.Fatalf("first decision = %+v", decision)
	}
	limiter.Reset("subject")
	if decision := limiter.Allow("subject"); !decision.Allowed {
		t.Fatalf("decision after reset = %+v", decision)
	}
}

func TestLimiterEvictsUnlockedEntryWhenBounded(t *testing.T) {
	now := time.Date(2026, time.July, 24, 12, 0, 0, 0, time.UTC)
	limiter, err := newWithClock(Policy{
		Attempts:   2,
		Window:     time.Minute,
		Lockout:    time.Minute,
		MaxEntries: 2,
	}, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}

	limiter.Allow("oldest")
	now = now.Add(time.Second)
	limiter.Allow("newer")
	now = now.Add(time.Second)
	if decision := limiter.Allow("incoming"); !decision.Allowed {
		t.Fatalf("incoming decision = %+v", decision)
	}
	if _, exists := limiter.entries["oldest"]; exists {
		t.Fatal("oldest unlocked entry was not evicted")
	}
}

func TestLimiterFailsClosedWhenAllEntriesAreBlocked(t *testing.T) {
	now := time.Date(2026, time.July, 24, 12, 0, 0, 0, time.UTC)
	limiter, err := newWithClock(Policy{
		Attempts:   1,
		Window:     time.Minute,
		Lockout:    time.Minute,
		MaxEntries: 1,
	}, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}

	limiter.Allow("blocked")
	limiter.Allow("blocked")
	if decision := limiter.Allow("new-subject"); decision.Allowed || decision.RetryAfter != time.Minute {
		t.Fatalf("bounded decision = %+v", decision)
	}
}

func TestLimiterAllowsOnlyConfiguredConcurrentAttempts(t *testing.T) {
	const allowedAttempts = 25
	limiter, err := New(Policy{
		Attempts:   allowedAttempts,
		Window:     time.Minute,
		Lockout:    time.Minute,
		MaxEntries: 10,
	})
	if err != nil {
		t.Fatal(err)
	}

	var (
		allowed atomic.Int32
		wg      sync.WaitGroup
	)
	for range 100 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if limiter.Allow("shared").Allowed {
				allowed.Add(1)
			}
		}()
	}
	wg.Wait()

	if got := allowed.Load(); got != allowedAttempts {
		t.Fatalf("allowed attempts = %d, want %d", got, allowedAttempts)
	}
}
