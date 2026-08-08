package storage

import (
	"sync"

	"github.com/google/uuid"
)

// RepositoryAccessCoordinator serializes repository lifecycle mutations
// against open rooted filesystem handles. A RepositoryFS holds a read lease
// until Close; relocate/remove/copy identity changes take a write lease.
type RepositoryAccessCoordinator struct {
	mu    sync.Mutex
	locks map[uuid.UUID]*sync.RWMutex
}

func NewRepositoryAccessCoordinator() *RepositoryAccessCoordinator {
	return &RepositoryAccessCoordinator{locks: make(map[uuid.UUID]*sync.RWMutex)}
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
