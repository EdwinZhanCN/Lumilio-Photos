package core

import (
	"sync"

	"github.com/cloudwego/eino/adk"
	"github.com/google/uuid"
)

type runRegistryKey struct {
	userID   int32
	threadID string
	runID    uuid.UUID
}

// RunRegistry contains process-local cancel handles only. PostgreSQL remains
// authoritative for run identity and status.
type RunRegistry struct {
	mu      sync.Mutex
	entries map[runRegistryKey]*runRegistryEntry
}

type runRegistryEntry struct {
	cancel          adk.AgentCancelFunc
	cancelRequested bool
}

func NewRunRegistry() *RunRegistry {
	return &RunRegistry{entries: make(map[runRegistryKey]*runRegistryEntry)}
}

func (r *RunRegistry) Register(userID int32, threadID string, runID uuid.UUID, cancel adk.AgentCancelFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries[runRegistryKey{userID: userID, threadID: threadID, runID: runID}] = &runRegistryEntry{cancel: cancel}
}

func (r *RunRegistry) RequestCancel(userID int32, threadID string, runID uuid.UUID) (adk.AgentCancelFunc, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	entry, ok := r.entries[runRegistryKey{userID: userID, threadID: threadID, runID: runID}]
	if !ok {
		return nil, false
	}
	entry.cancelRequested = true
	return entry.cancel, true
}

func (r *RunRegistry) CancelRequested(userID int32, threadID string, runID uuid.UUID) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	entry := r.entries[runRegistryKey{userID: userID, threadID: threadID, runID: runID}]
	return entry != nil && entry.cancelRequested
}

func (r *RunRegistry) Delete(userID int32, threadID string, runID uuid.UUID) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.entries, runRegistryKey{userID: userID, threadID: threadID, runID: runID})
}
