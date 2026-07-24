package ratelimit

import (
	"errors"
	"sync"
	"time"
)

// Policy controls a fixed-window limiter with a lockout after the configured
// number of attempts. MaxEntries bounds memory use.
type Policy struct {
	Attempts   int
	Window     time.Duration
	Lockout    time.Duration
	MaxEntries int
}

// Decision reports whether a request may proceed and, when denied, how long
// the caller should wait before retrying.
type Decision struct {
	Allowed    bool
	RetryAfter time.Duration
}

type entry struct {
	attempts       int
	windowStarted  time.Time
	blockedUntil   time.Time
	lastAccessedAt time.Time
}

// Limiter is a concurrency-safe, bounded, in-memory fixed-window limiter.
type Limiter struct {
	mu      sync.Mutex
	policy  Policy
	entries map[string]entry
	now     func() time.Time
}

// New constructs a limiter for the supplied policy.
func New(policy Policy) (*Limiter, error) {
	return newWithClock(policy, time.Now)
}

func newWithClock(policy Policy, now func() time.Time) (*Limiter, error) {
	switch {
	case policy.Attempts <= 0:
		return nil, errors.New("rate limit attempts must be positive")
	case policy.Window <= 0:
		return nil, errors.New("rate limit window must be positive")
	case policy.Lockout <= 0:
		return nil, errors.New("rate limit lockout must be positive")
	case policy.MaxEntries <= 0:
		return nil, errors.New("rate limit max entries must be positive")
	case now == nil:
		return nil, errors.New("rate limit clock is required")
	}

	return &Limiter{
		policy:  policy,
		entries: make(map[string]entry, policy.MaxEntries),
		now:     now,
	}, nil
}

// Allow consumes one attempt for key. The first Policy.Attempts calls in a
// window are allowed; the next call starts the lockout.
func (l *Limiter) Allow(key string) Decision {
	if key == "" {
		return Decision{RetryAfter: l.policy.Lockout}
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	current, found := l.entries[key]
	if !found {
		l.pruneExpired(now)
		if len(l.entries) >= l.policy.MaxEntries && !l.evictOldestUnlocked(now) {
			return Decision{RetryAfter: l.shortestBlockedRetry(now)}
		}
		current = entry{windowStarted: now}
	}

	current.lastAccessedAt = now
	if now.Before(current.blockedUntil) {
		l.entries[key] = current
		return Decision{RetryAfter: current.blockedUntil.Sub(now)}
	}

	if !current.blockedUntil.IsZero() {
		current.attempts = 0
		current.windowStarted = now
		current.blockedUntil = time.Time{}
	}
	if current.windowStarted.IsZero() || !now.Before(current.windowStarted.Add(l.policy.Window)) {
		current.attempts = 0
		current.windowStarted = now
	}

	if current.attempts >= l.policy.Attempts {
		current.blockedUntil = now.Add(l.policy.Lockout)
		l.entries[key] = current
		return Decision{RetryAfter: l.policy.Lockout}
	}

	current.attempts++
	l.entries[key] = current
	return Decision{Allowed: true}
}

// Reset forgets all attempts for key.
func (l *Limiter) Reset(key string) {
	if key == "" {
		return
	}
	l.mu.Lock()
	delete(l.entries, key)
	l.mu.Unlock()
}

func (l *Limiter) pruneExpired(now time.Time) {
	retention := max(l.policy.Window, l.policy.Lockout) * 2
	for key, candidate := range l.entries {
		if now.Before(candidate.blockedUntil) {
			continue
		}
		if !now.Before(candidate.lastAccessedAt.Add(retention)) {
			delete(l.entries, key)
		}
	}
}

func (l *Limiter) evictOldestUnlocked(now time.Time) bool {
	var (
		oldestKey  string
		oldestTime time.Time
	)
	for key, candidate := range l.entries {
		if now.Before(candidate.blockedUntil) {
			continue
		}
		if oldestKey == "" || candidate.lastAccessedAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = candidate.lastAccessedAt
		}
	}
	if oldestKey == "" {
		return false
	}
	delete(l.entries, oldestKey)
	return true
}

func (l *Limiter) shortestBlockedRetry(now time.Time) time.Duration {
	retryAfter := l.policy.Lockout
	for _, candidate := range l.entries {
		if remaining := candidate.blockedUntil.Sub(now); remaining > 0 && remaining < retryAfter {
			retryAfter = remaining
		}
	}
	return retryAfter
}
