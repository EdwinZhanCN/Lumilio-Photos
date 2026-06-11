package ref

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Store is the session-scoped handle store. The memory implementation is the
// eager-first decision on record: snapshots live in process memory, survive
// for TTL since last access, and die on restart (checkpoints in PostgreSQL
// outlive them — tools must answer RefNotFound with a recovery hint).
type Store interface {
	// Create materializes an ordered snapshot under the scope and returns the
	// stored ref. The input slice is copied; hint is mnemonic only; summary
	// is the receipt one-liner kept for the ref ledger.
	Create(scope Scope, plan Plan, hint, summary string, assetIDs []uuid.UUID, truncated bool) *Ref

	// Get resolves a ref id within scope, refreshing its TTL. Missing,
	// expired and cross-scope ids all return CodeRefNotFound (INV-4).
	Get(scope Scope, id string) (*Ref, *Error)

	// List returns the scope's active refs in creation order (ref ledger).
	List(scope Scope) []*Ref
}

type scopeKey struct {
	userID   int32
	threadID string
}

type scopeRefs struct {
	counter int
	refs    map[string]*Ref
}

// MemoryStore is the in-memory Store. It is safe for concurrent use; the
// eino ToolsNode may execute tool calls in parallel.
type MemoryStore struct {
	mu          sync.Mutex
	scopes      map[scopeKey]*scopeRefs
	ttl         time.Duration
	maxPerScope int
	now         func() time.Time
}

// NewMemoryStore creates a store with the given TTL and per-scope cap;
// non-positive values fall back to the package defaults.
func NewMemoryStore(ttl time.Duration, maxPerScope int) *MemoryStore {
	if ttl <= 0 {
		ttl = DefaultTTL
	}
	if maxPerScope <= 0 {
		maxPerScope = DefaultMaxRefsPerScope
	}
	return &MemoryStore{
		scopes:      make(map[scopeKey]*scopeRefs),
		ttl:         ttl,
		maxPerScope: maxPerScope,
		now:         time.Now,
	}
}

func (s *MemoryStore) Create(scope Scope, plan Plan, hint, summary string, assetIDs []uuid.UUID, truncated bool) *Ref {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := scopeKey{scope.UserID, scope.ThreadID}
	sc := s.scopes[key]
	if sc == nil {
		sc = &scopeRefs{refs: make(map[string]*Ref)}
		s.scopes[key] = sc
	}

	sc.counter++
	now := s.now()
	r := &Ref{
		ID:         formatID(sc.counter, hint),
		Scope:      scope,
		Plan:       plan,
		AssetIDs:   append([]uuid.UUID(nil), assetIDs...),
		Truncated:  truncated,
		CreatedAt:  now,
		LastAccess: now,
		Summary:    summary,
		seq:        sc.counter,
	}
	sc.refs[r.ID] = r

	if len(sc.refs) > s.maxPerScope {
		s.evictLRULocked(sc)
	}
	return r
}

func (s *MemoryStore) Get(scope Scope, id string) (*Ref, *Error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sc := s.scopes[scopeKey{scope.UserID, scope.ThreadID}]
	if sc == nil {
		return nil, NotFound(id)
	}
	r, ok := sc.refs[id]
	if !ok {
		return nil, NotFound(id)
	}
	now := s.now()
	if now.Sub(r.LastAccess) > s.ttl {
		delete(sc.refs, id)
		return nil, NotFound(id)
	}
	r.LastAccess = now
	return r, nil
}

func (s *MemoryStore) List(scope Scope) []*Ref {
	s.mu.Lock()
	defer s.mu.Unlock()

	sc := s.scopes[scopeKey{scope.UserID, scope.ThreadID}]
	if sc == nil {
		return nil
	}
	now := s.now()
	out := make([]*Ref, 0, len(sc.refs))
	for id, r := range sc.refs {
		if now.Sub(r.LastAccess) > s.ttl {
			delete(sc.refs, id)
			continue
		}
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].seq < out[j].seq })
	return out
}

// RunJanitor sweeps expired refs and empty scopes until ctx is done. Expiry
// is also enforced lazily on Get/List; the janitor only bounds memory for
// abandoned sessions.
func (s *MemoryStore) RunJanitor(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.sweep()
		}
	}
}

func (s *MemoryStore) sweep() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	for key, sc := range s.scopes {
		for id, r := range sc.refs {
			if now.Sub(r.LastAccess) > s.ttl {
				delete(sc.refs, id)
			}
		}
		if len(sc.refs) == 0 {
			delete(s.scopes, key)
		}
	}
}

func (s *MemoryStore) evictLRULocked(sc *scopeRefs) {
	for len(sc.refs) > s.maxPerScope {
		var oldest *Ref
		for _, r := range sc.refs {
			if oldest == nil || r.LastAccess.Before(oldest.LastAccess) {
				oldest = r
			}
		}
		delete(sc.refs, oldest.ID)
	}
}
