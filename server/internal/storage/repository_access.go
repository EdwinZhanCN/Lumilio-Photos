package storage

import (
	"context"
	"errors"
	"sort"
	"sync"

	"github.com/google/uuid"
)

// RepositoryAccessCoordinator serializes repository lifecycle mutations
// against open rooted filesystem handles. A RepositoryFS holds a read lease
// until Close; relocate/remove/copy identity changes take a write lease.
type RepositoryAccessCoordinator struct {
	mu    sync.Mutex
	locks map[uuid.UUID]*repositoryAccessLock
	roots map[uuid.UUID]*repositoryAccessLock
}

func NewRepositoryAccessCoordinator() *RepositoryAccessCoordinator {
	return &RepositoryAccessCoordinator{
		locks: make(map[uuid.UUID]*repositoryAccessLock),
		roots: make(map[uuid.UUID]*repositoryAccessLock),
	}
}

func (c *RepositoryAccessCoordinator) rootLockFor(id uuid.UUID) *repositoryAccessLock {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	lock := c.roots[id]
	if lock == nil {
		lock = &repositoryAccessLock{}
		c.roots[id] = lock
	}
	return lock
}

func (c *RepositoryAccessCoordinator) lockFor(id uuid.UUID) *repositoryAccessLock {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	lock := c.locks[id]
	if lock == nil {
		lock = &repositoryAccessLock{}
		c.locks[id] = lock
	}
	return lock
}

func (c *RepositoryAccessCoordinator) acquireRead(id uuid.UUID) func() {
	lock := c.lockFor(id)
	if lock == nil {
		return func() {}
	}
	release, _ := lock.acquire(context.Background(), repositoryAccessRead)
	return release
}

func (c *RepositoryAccessCoordinator) acquireReadContext(ctx context.Context, id uuid.UUID) (func(), error) {
	lock := c.lockFor(id)
	if lock == nil {
		return func() {}, nil
	}
	if ctx == nil {
		return nil, errors.New("context is required")
	}
	return lock.acquire(ctx, repositoryAccessRead)
}

// AcquireMutation blocks until every open RepositoryFS for id has closed. The
// returned release function must be called exactly once.
func (c *RepositoryAccessCoordinator) AcquireMutation(id uuid.UUID) func() {
	lock := c.lockFor(id)
	if lock == nil {
		return func() {}
	}
	release, _ := lock.acquire(context.Background(), repositoryAccessWrite)
	return release
}

// AcquireRootRead keeps ordinary child operations behind a concurrent
// Storage Location mutation. Callers acquire the root lease before any child
// repository write lock.
func (c *RepositoryAccessCoordinator) AcquireRootRead(id uuid.UUID) func() {
	lock := c.rootLockFor(id)
	if lock == nil {
		return func() {}
	}
	release, _ := lock.acquire(context.Background(), repositoryAccessRead)
	return release
}

func (c *RepositoryAccessCoordinator) AcquireRootReadContext(ctx context.Context, id uuid.UUID) (func(), error) {
	lock := c.rootLockFor(id)
	if lock == nil {
		return func() {}, nil
	}
	if ctx == nil {
		return nil, errors.New("context is required")
	}
	return lock.acquire(ctx, repositoryAccessRead)
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

func acquireWriteLockContext(ctx context.Context, lock *repositoryAccessLock) (func(), error) {
	if lock == nil {
		return func() {}, nil
	}
	return lock.acquire(ctx, repositoryAccessWrite)
}

type repositoryAccessMode uint8

const (
	repositoryAccessRead repositoryAccessMode = iota
	repositoryAccessWrite
)

type repositoryAccessWaiter struct {
	mode    repositoryAccessMode
	ready   chan struct{}
	granted bool
}

// repositoryAccessLock is a context-aware FIFO read/write gate. In contrast
// to polling sync.RWMutex.TryLock, queueing a writer immediately prevents later
// readers from bypassing it. That writer preference is required for bounded
// lifecycle and queue-enqueue latency while scans and media reads are active.
type repositoryAccessLock struct {
	mu      sync.Mutex
	readers int
	writer  bool
	waiters []*repositoryAccessWaiter
}

func (lock *repositoryAccessLock) acquire(ctx context.Context, mode repositoryAccessMode) (func(), error) {
	if ctx == nil {
		return nil, errors.New("context is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	waiter := &repositoryAccessWaiter{mode: mode, ready: make(chan struct{})}
	lock.mu.Lock()
	if len(lock.waiters) == 0 && lock.canAcquireLocked(mode) {
		lock.takeLocked(mode)
		lock.mu.Unlock()
		return lock.releaseFunc(mode), nil
	}
	lock.waiters = append(lock.waiters, waiter)
	lock.grantLocked()
	lock.mu.Unlock()

	select {
	case <-waiter.ready:
		return lock.releaseFunc(mode), nil
	case <-ctx.Done():
		lock.mu.Lock()
		if waiter.granted {
			lock.putLocked(mode)
		} else {
			lock.removeWaiterLocked(waiter)
		}
		lock.grantLocked()
		lock.mu.Unlock()
		return nil, ctx.Err()
	}
}

func (lock *repositoryAccessLock) canAcquireLocked(mode repositoryAccessMode) bool {
	if lock.writer {
		return false
	}
	return mode == repositoryAccessRead || lock.readers == 0
}

func (lock *repositoryAccessLock) takeLocked(mode repositoryAccessMode) {
	if mode == repositoryAccessRead {
		lock.readers++
		return
	}
	lock.writer = true
}

func (lock *repositoryAccessLock) putLocked(mode repositoryAccessMode) {
	if mode == repositoryAccessRead {
		lock.readers--
		return
	}
	lock.writer = false
}

func (lock *repositoryAccessLock) releaseFunc(mode repositoryAccessMode) func() {
	released := false
	return func() {
		lock.mu.Lock()
		if released {
			lock.mu.Unlock()
			return
		}
		released = true
		lock.putLocked(mode)
		lock.grantLocked()
		lock.mu.Unlock()
	}
}

func (lock *repositoryAccessLock) removeWaiterLocked(target *repositoryAccessWaiter) {
	for index, waiter := range lock.waiters {
		if waiter != target {
			continue
		}
		copy(lock.waiters[index:], lock.waiters[index+1:])
		lock.waiters[len(lock.waiters)-1] = nil
		lock.waiters = lock.waiters[:len(lock.waiters)-1]
		return
	}
}

func (lock *repositoryAccessLock) grantLocked() {
	if lock.writer || len(lock.waiters) == 0 {
		return
	}
	if lock.waiters[0].mode == repositoryAccessWrite {
		if lock.readers != 0 {
			return
		}
		waiter := lock.popWaiterLocked()
		lock.writer = true
		waiter.granted = true
		close(waiter.ready)
		return
	}
	for len(lock.waiters) > 0 && lock.waiters[0].mode == repositoryAccessRead {
		waiter := lock.popWaiterLocked()
		lock.readers++
		waiter.granted = true
		close(waiter.ready)
	}
}

func (lock *repositoryAccessLock) popWaiterLocked() *repositoryAccessWaiter {
	waiter := lock.waiters[0]
	copy(lock.waiters, lock.waiters[1:])
	lock.waiters[len(lock.waiters)-1] = nil
	lock.waiters = lock.waiters[:len(lock.waiters)-1]
	return waiter
}
