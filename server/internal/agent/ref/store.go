package ref

import (
	"context"
	"encoding/json"
	"sort"
	"sync"
	"time"

	"server/internal/db/dbtypes"
	"server/internal/db/repo"

	"github.com/google/uuid"
)

const (
	DefaultUserHotBudget   int64 = 64 << 20
	DefaultGlobalHotBudget int64 = 512 << 20
)

// MembershipChecker is implemented by the user-bound AuthorizedLibrary.
type MembershipChecker interface {
	AuthorizeAssetIDs(ctx context.Context, userID int32, ids []uuid.UUID) ([]uuid.UUID, error)
}

type Store interface {
	Create(ctx context.Context, scope Scope, plan Plan, hint, summary string, assetIDs []uuid.UUID, truncated bool) (*Ref, *Error)
	Get(ctx context.Context, scope Scope, id string) (*Ref, *Error)
	List(ctx context.Context, scope Scope) []*Ref
	ReleaseScope(ctx context.Context, scope Scope) error
	DeleteScope(ctx context.Context, scope Scope) error
}

type scopeKey struct {
	userID   int32
	threadID string
}

type scopeRefs struct {
	counter int
	refs    map[string]*Ref
}

// MemoryStore is a bounded hot cache backed by agent_refs when queries is
// configured. Every production ref is persisted before becoming visible; LRU
// eviction therefore spills to SQLite instead of losing checkpoint state.
type MemoryStore struct {
	mu           sync.Mutex
	scopes       map[scopeKey]*scopeRefs
	ttl          time.Duration
	maxPerScope  int
	now          func() time.Time
	queries      *repo.Queries
	checker      MembershipChecker
	userBudget   int64
	globalBudget int64
	userBytes    map[int32]int64
	hotBytes     int64
}

func NewMemoryStore(ttl time.Duration, maxPerScope int) *MemoryStore {
	return newStore(nil, nil, ttl, maxPerScope, 0, 0)
}

func NewPersistentStore(queries *repo.Queries, checker MembershipChecker, ttl time.Duration, maxPerScope int, userBudget, globalBudget int64) *MemoryStore {
	if userBudget <= 0 {
		userBudget = DefaultUserHotBudget
	}
	if globalBudget <= 0 {
		globalBudget = DefaultGlobalHotBudget
	}
	return newStore(queries, checker, ttl, maxPerScope, userBudget, globalBudget)
}

func newStore(queries *repo.Queries, checker MembershipChecker, ttl time.Duration, maxPerScope int, userBudget, globalBudget int64) *MemoryStore {
	if ttl <= 0 {
		ttl = DefaultTTL
	}
	if maxPerScope <= 0 {
		maxPerScope = DefaultMaxRefsPerScope
	}
	return &MemoryStore{
		scopes: make(map[scopeKey]*scopeRefs), ttl: ttl, maxPerScope: maxPerScope,
		now: time.Now, queries: queries, checker: checker,
		userBudget: userBudget, globalBudget: globalBudget, userBytes: make(map[int32]int64),
	}
}

func estimateBytes(r *Ref) int64 { return 256 + int64(len(r.AssetIDs))*16 }

func (s *MemoryStore) Create(ctx context.Context, scope Scope, plan Plan, hint, summary string, assetIDs []uuid.UUID, truncated bool) (*Ref, *Error) {
	if s.checker != nil {
		authorized, err := s.checker.AuthorizeAssetIDs(ctx, scope.UserID, assetIDs)
		if err != nil || len(authorized) != len(assetIDs) {
			return nil, NotFound("membership")
		}
		assetIDs = authorized
	}
	normalized, err := plan.Normalize(scope.UserID)
	if err != nil {
		return nil, Internal("ref plan encoding")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	key := scopeKey{scope.UserID, scope.ThreadID}
	sc := s.scopes[key]
	if sc == nil {
		sc = &scopeRefs{refs: make(map[string]*Ref)}
		s.scopes[key] = sc
	}
	if s.queries != nil && sc.counter == 0 && len(sc.refs) == 0 {
		s.loadScopeLocked(ctx, scope, sc)
	}

	sc.counter++
	now := s.now()
	r := &Ref{
		ID: formatID(sc.counter, hint), Scope: scope, Plan: normalized,
		AssetIDs: append([]uuid.UUID(nil), assetIDs...), Truncated: truncated,
		CreatedAt: now, LastAccess: now, Summary: summary, seq: sc.counter,
	}

	if s.queries != nil {
		planJSON, marshalErr := json.Marshal(normalized)
		if marshalErr != nil {
			return nil, Internal("ref persistence")
		}
		if err := s.queries.UpsertAgentRef(ctx, repo.UpsertAgentRefParams{
			UserID: scope.UserID, ThreadID: scope.ThreadID, RefID: r.ID,
			Sequence: int64(r.seq), Plan: dbtypes.JSON(planJSON), AssetIds: dbtypes.UUIDs(assetIDs),
			Summary: summary, Truncated: truncated,
			CreatedAt:      dbtypes.NewTimestamp(now),
			LastAccessedAt: dbtypes.NewTimestamp(now),
			ExpiresAt:      dbtypes.NewTimestamp(now.Add(s.ttl)),
		}); err != nil {
			return nil, ResourceExhausted("ref persistence is unavailable and the hot-memory budget cannot safely accept unspillable state")
		}
		trimmed, err := s.queries.TrimAgentThreadRefs(ctx, repo.TrimAgentThreadRefsParams{
			UserID: scope.UserID, ThreadID: scope.ThreadID, MaxRefs: int64(s.maxPerScope),
		})
		if err != nil {
			return nil, ResourceExhausted("ref persistence is unavailable and the per-thread limit cannot be enforced")
		}
		for _, refID := range trimmed {
			if old := sc.refs[refID]; old != nil {
				s.removeHotLocked(sc, old)
			}
		}
	} else if s.overBudgetLocked(scope.UserID, estimateBytes(r)) {
		return nil, ResourceExhausted("ref hot-memory budget exhausted")
	}

	s.addHotLocked(sc, r)
	s.enforceBudgetsLocked()
	return r, nil
}

func (s *MemoryStore) Get(ctx context.Context, scope Scope, id string) (*Ref, *Error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := scopeKey{scope.UserID, scope.ThreadID}
	sc := s.scopes[key]
	now := s.now()
	if sc != nil {
		if r := sc.refs[id]; r != nil {
			if now.Sub(r.LastAccess) > s.ttl {
				s.removeHotLocked(sc, r)
			} else {
				if s.checker != nil {
					if _, err := s.checker.AuthorizeAssetIDs(ctx, scope.UserID, r.AssetIDs); err != nil {
						s.removeHotLocked(sc, r)
						return nil, NotFound(id)
					}
				}
				if s.queries != nil {
					if _, err := s.queries.GetAgentRef(ctx, repo.GetAgentRefParams{
						Now:       dbtypes.NewTimestamp(now),
						ExpiresAt: dbtypes.NewTimestamp(now.Add(s.ttl)),
						UserID:    scope.UserID,
						ThreadID:  scope.ThreadID,
						RefID:     id,
					}); err != nil {
						s.removeHotLocked(sc, r)
						return nil, NotFound(id)
					}
				}
				r.LastAccess = now
				return r, nil
			}
		}
	}
	if s.queries == nil {
		return nil, NotFound(id)
	}
	row, err := s.queries.GetAgentRef(ctx, repo.GetAgentRefParams{
		Now:       dbtypes.NewTimestamp(now),
		ExpiresAt: dbtypes.NewTimestamp(now.Add(s.ttl)),
		UserID:    scope.UserID, ThreadID: scope.ThreadID, RefID: id,
	})
	if err != nil {
		return nil, NotFound(id)
	}
	r, err := refFromRow(row)
	if err != nil {
		return nil, NotFound(id)
	}
	if s.checker != nil {
		if _, err := s.checker.AuthorizeAssetIDs(ctx, scope.UserID, r.AssetIDs); err != nil {
			return nil, NotFound(id)
		}
	}
	if sc == nil {
		sc = &scopeRefs{refs: make(map[string]*Ref)}
		s.scopes[key] = sc
	}
	sc.counter = max(sc.counter, r.seq)
	s.addHotLocked(sc, r)
	s.enforceBudgetsLocked()
	return r, nil
}

func (s *MemoryStore) List(ctx context.Context, scope Scope) []*Ref {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := scopeKey{scope.UserID, scope.ThreadID}
	sc := s.scopes[key]
	if sc == nil {
		sc = &scopeRefs{refs: make(map[string]*Ref)}
		s.scopes[key] = sc
	}
	if s.queries != nil {
		s.loadScopeLocked(ctx, scope, sc)
	}
	now := s.now()
	out := make([]*Ref, 0, len(sc.refs))
	for _, r := range sc.refs {
		if now.Sub(r.LastAccess) > s.ttl {
			continue
		}
		if s.checker != nil {
			if _, err := s.checker.AuthorizeAssetIDs(ctx, scope.UserID, r.AssetIDs); err != nil {
				s.removeHotLocked(sc, r)
				continue
			}
		}
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].seq < out[j].seq })
	return out
}

// ReleaseScope starts the TTL only after a thread reaches a normal terminal
// state. Active and awaiting-confirmation refs remain durable for recovery.
func (s *MemoryStore) ReleaseScope(ctx context.Context, scope Scope) error {
	s.mu.Lock()
	now := s.now()
	if sc := s.scopes[scopeKey{scope.UserID, scope.ThreadID}]; sc != nil {
		for _, r := range sc.refs {
			r.LastAccess = now
		}
	}
	s.mu.Unlock()
	if s.queries != nil {
		return s.queries.ReleaseAgentThreadRefs(ctx, repo.ReleaseAgentThreadRefsParams{
			Now:       dbtypes.NewTimestamp(now),
			ExpiresAt: dbtypes.NewTimestamp(now.Add(s.ttl)),
			UserID:    scope.UserID,
			ThreadID:  scope.ThreadID,
		})
	}
	return nil
}

func (s *MemoryStore) DeleteScope(ctx context.Context, scope Scope) error {
	s.mu.Lock()
	key := scopeKey{scope.UserID, scope.ThreadID}
	if sc := s.scopes[key]; sc != nil {
		for _, r := range sc.refs {
			s.hotBytes -= estimateBytes(r)
			s.userBytes[scope.UserID] -= estimateBytes(r)
		}
		delete(s.scopes, key)
	}
	s.mu.Unlock()
	if s.queries != nil {
		return s.queries.DeleteAgentThreadRefs(ctx, repo.DeleteAgentThreadRefsParams{
			UserID: scope.UserID, ThreadID: scope.ThreadID,
		})
	}
	return nil
}

func (s *MemoryStore) loadScopeLocked(ctx context.Context, scope Scope, sc *scopeRefs) {
	rows, err := s.queries.ListAgentRefs(ctx, repo.ListAgentRefsParams{
		UserID: scope.UserID, ThreadID: scope.ThreadID, Now: dbtypes.NewTimestamp(s.now()),
	})
	if err != nil {
		return
	}
	for _, row := range rows {
		r, err := refFromRow(row)
		if err != nil {
			continue
		}
		sc.counter = max(sc.counter, r.seq)
		if sc.refs[r.ID] == nil {
			s.addHotLocked(sc, r)
		}
	}
	s.enforceBudgetsLocked()
}

func refFromRow(row repo.AgentRef) (*Ref, error) {
	var plan Plan
	if err := json.Unmarshal(row.Plan, &plan); err != nil {
		return nil, err
	}
	ids := append([]uuid.UUID(nil), row.AssetIds...)
	return &Ref{
		ID: row.RefID, Scope: Scope{UserID: row.UserID, ThreadID: row.ThreadID},
		Plan: plan, AssetIDs: ids, Summary: row.Summary, Truncated: row.Truncated,
		CreatedAt: row.CreatedAt.Time, LastAccess: row.LastAccessedAt.Time, seq: int(row.Sequence),
	}, nil
}

func (s *MemoryStore) addHotLocked(sc *scopeRefs, r *Ref) {
	if existing := sc.refs[r.ID]; existing != nil {
		s.removeHotLocked(sc, existing)
	}
	sc.refs[r.ID] = r
	bytes := estimateBytes(r)
	s.hotBytes += bytes
	s.userBytes[r.Scope.UserID] += bytes
	if len(sc.refs) > s.maxPerScope {
		s.evictScopeLRULocked(sc)
	}
}

func (s *MemoryStore) removeHotLocked(sc *scopeRefs, r *Ref) {
	delete(sc.refs, r.ID)
	bytes := estimateBytes(r)
	s.hotBytes -= bytes
	s.userBytes[r.Scope.UserID] -= bytes
}

func (s *MemoryStore) overBudgetLocked(userID int32, additional int64) bool {
	return (s.userBudget > 0 && s.userBytes[userID]+additional > s.userBudget) ||
		(s.globalBudget > 0 && s.hotBytes+additional > s.globalBudget)
}

func (s *MemoryStore) enforceBudgetsLocked() {
	for (s.globalBudget > 0 && s.hotBytes > s.globalBudget) || s.anyUserOverBudgetLocked() {
		var oldest *Ref
		var oldestScope *scopeRefs
		for _, sc := range s.scopes {
			for _, r := range sc.refs {
				if oldest == nil || r.LastAccess.Before(oldest.LastAccess) {
					oldest, oldestScope = r, sc
				}
			}
		}
		if oldest == nil {
			return
		}
		s.removeHotLocked(oldestScope, oldest)
	}
}

func (s *MemoryStore) anyUserOverBudgetLocked() bool {
	if s.userBudget <= 0 {
		return false
	}
	for _, size := range s.userBytes {
		if size > s.userBudget {
			return true
		}
	}
	return false
}

func (s *MemoryStore) evictScopeLRULocked(sc *scopeRefs) {
	for len(sc.refs) > s.maxPerScope {
		var oldest *Ref
		for _, r := range sc.refs {
			if oldest == nil || r.LastAccess.Before(oldest.LastAccess) {
				oldest = r
			}
		}
		s.removeHotLocked(sc, oldest)
	}
}

func (s *MemoryStore) RunJanitor(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.sweep(ctx)
		}
	}
}

func (s *MemoryStore) sweep(ctx context.Context) {
	s.mu.Lock()
	now := s.now()
	for key, sc := range s.scopes {
		for _, r := range sc.refs {
			if now.Sub(r.LastAccess) > s.ttl {
				s.removeHotLocked(sc, r)
			}
		}
		if len(sc.refs) == 0 {
			delete(s.scopes, key)
		}
	}
	s.mu.Unlock()
	if s.queries != nil {
		_ = s.queries.DeleteExpiredAgentRefs(ctx, dbtypes.NewTimestamp(now))
	}
}
