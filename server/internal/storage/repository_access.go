package storage

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// RepositoryAccessCoordinator serializes repository lifecycle mutations
// against open rooted filesystem handles. A RepositoryFS holds a read lease
// until Close; relocate/remove/copy identity changes take a write lease.
type RepositoryAccessCoordinator struct {
	mu    sync.Mutex
	locks map[uuid.UUID]*sync.RWMutex
	roots map[uuid.UUID]*sync.RWMutex
}

func NewRepositoryAccessCoordinator() *RepositoryAccessCoordinator {
	return &RepositoryAccessCoordinator{
		locks: make(map[uuid.UUID]*sync.RWMutex),
		roots: make(map[uuid.UUID]*sync.RWMutex),
	}
}

func (c *RepositoryAccessCoordinator) rootLockFor(id uuid.UUID) *sync.RWMutex {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	lock := c.roots[id]
	if lock == nil {
		lock = &sync.RWMutex{}
		c.roots[id] = lock
	}
	return lock
}

func (c *RepositoryAccessCoordinator) lockFor(id uuid.UUID) *sync.RWMutex {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	lock := c.locks[id]
	if lock == nil {
		lock = &sync.RWMutex{}
		c.locks[id] = lock
	}
	return lock
}

func (c *RepositoryAccessCoordinator) acquireRead(id uuid.UUID) func() {
	lock := c.lockFor(id)
	if lock == nil {
		return func() {}
	}
	lock.RLock()
	return lock.RUnlock
}

func (c *RepositoryAccessCoordinator) acquireReadContext(ctx context.Context, id uuid.UUID) (func(), error) {
	lock := c.lockFor(id)
	if lock == nil {
		return func() {}, nil
	}
	if ctx == nil {
		return nil, errors.New("context is required")
	}
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		if lock.TryRLock() {
			return lock.RUnlock, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}

// AcquireMutation blocks until every open RepositoryFS for id has closed. The
// returned release function must be called exactly once.
func (c *RepositoryAccessCoordinator) AcquireMutation(id uuid.UUID) func() {
	lock := c.lockFor(id)
	if lock == nil {
		return func() {}
	}
	lock.Lock()
	return lock.Unlock
}

// AcquireRootRead keeps ordinary child operations behind a concurrent
// Storage Location mutation. Callers acquire the root lease before any child
// repository write lock.
func (c *RepositoryAccessCoordinator) AcquireRootRead(id uuid.UUID) func() {
	lock := c.rootLockFor(id)
	if lock == nil {
		return func() {}
	}
	lock.RLock()
	return lock.RUnlock
}

func (c *RepositoryAccessCoordinator) AcquireRootReadContext(ctx context.Context, id uuid.UUID) (func(), error) {
	lock := c.rootLockFor(id)
	if lock == nil {
		return func() {}, nil
	}
	if ctx == nil {
		return nil, errors.New("context is required")
	}
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		if lock.TryRLock() {
			return lock.RUnlock, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}

// AcquireRootMutationContext takes the root barrier without waiting past the
// caller's deadline. It returns context.Canceled/DeadlineExceeded as
// resource-busy evidence rather than changing paths while leases are active.
func (c *RepositoryAccessCoordinator) AcquireRootMutationContext(ctx context.Context, id uuid.UUID) (func(), error) {
	return acquireWriteLockContext(ctx, c.rootLockFor(id))
}

// AcquireMutationsContext takes child write locks in stable UUID order and
// releases all acquired locks if the context expires.
func (c *RepositoryAccessCoordinator) AcquireMutationsContext(ctx context.Context, ids []uuid.UUID) (func(), error) {
	if c == nil || len(ids) == 0 {
		return func() {}, nil
	}
	ordered := append([]uuid.UUID(nil), ids...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].String() < ordered[j].String() })
	releases := make([]func(), 0, len(ordered))
	for _, id := range ordered {
		release, err := acquireWriteLockContext(ctx, c.lockFor(id))
		if err != nil {
			for i := len(releases) - 1; i >= 0; i-- {
				releases[i]()
			}
			return nil, err
		}
		releases = append(releases, release)
	}
	return func() {
		for i := len(releases) - 1; i >= 0; i-- {
			releases[i]()
		}
	}, nil
}

func acquireWriteLockContext(ctx context.Context, lock *sync.RWMutex) (func(), error) {
	if lock == nil {
		return func() {}, nil
	}
	if ctx == nil {
		return nil, errors.New("context is required")
	}
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		if lock.TryLock() {
			return lock.Unlock, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}
