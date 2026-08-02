// Package state owns the immutable DesktopSnapshot and its latest-only
// notification stream. Producers commit reducers; consumers always read the
// complete snapshot after receiving a notice.
package state

import (
	"crypto/rand"
	"encoding/hex"
	"reflect"
	"sync"

	"desktop/internal/control/dto"
)

type Store struct {
	mu        sync.RWMutex
	snapshot  dto.DesktopSnapshot
	nextSubID uint64
	subs      map[uint64]chan dto.SnapshotChanged
	closed    bool
}

func New() *Store {
	return NewWithInstanceID(newInstanceID())
}

func NewWithInstanceID(instanceID string) *Store {
	if instanceID == "" {
		instanceID = newInstanceID()
	}
	return &Store{
		snapshot: dto.InitialSnapshot(instanceID),
		subs:     make(map[uint64]chan dto.SnapshotChanged),
	}
}

func NewWithSnapshot(snapshot dto.DesktopSnapshot) *Store {
	if snapshot.InstanceID == "" {
		snapshot.InstanceID = newInstanceID()
	}
	if snapshot.Revision == 0 {
		snapshot.Revision = 1
	}
	return &Store{
		snapshot: snapshot.Clone(),
		subs:     make(map[uint64]chan dto.SnapshotChanged),
	}
}

func (s *Store) Get() dto.DesktopSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.snapshot.Clone()
}

// Commit applies a reducer to a private copy. A revision is emitted only when
// the reducer changes a value, which keeps repeated lifecycle callbacks
// idempotent and makes event deduplication safe.
func (s *Store) Commit(reducer func(*dto.DesktopSnapshot)) (dto.DesktopSnapshot, bool) {
	if reducer == nil {
		return s.Get(), false
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return s.snapshot.Clone(), false
	}

	candidate := s.snapshot.Clone()
	reducer(&candidate)
	if reflect.DeepEqual(candidate, s.snapshot) {
		return s.snapshot.Clone(), false
	}
	candidate.InstanceID = s.snapshot.InstanceID
	candidate.Revision = s.snapshot.Revision + 1
	s.snapshot = candidate.Clone()
	notice := dto.SnapshotChanged{InstanceID: candidate.InstanceID, Revision: candidate.Revision}
	for _, ch := range s.subs {
		publishLatest(ch, notice)
	}
	return candidate.Clone(), true
}

// Subscribe returns a latest-only notice channel and an idempotent cancel
// function. A slow consumer never blocks a producer or causes state loss: it
// only loses intermediate notices and must read Store.Get for the latest value.
func (s *Store) Subscribe(buffer int) (<-chan dto.SnapshotChanged, func()) {
	if buffer < 1 {
		buffer = 1
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		ch := make(chan dto.SnapshotChanged)
		close(ch)
		return ch, func() {}
	}
	s.nextSubID++
	id := s.nextSubID
	ch := make(chan dto.SnapshotChanged, buffer)
	s.subs[id] = ch
	var once sync.Once
	cancel := func() {
		once.Do(func() {
			s.mu.Lock()
			defer s.mu.Unlock()
			if current, ok := s.subs[id]; ok {
				delete(s.subs, id)
				close(current)
			}
		})
	}
	return ch, cancel
}

func (s *Store) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.closed = true
	for id, ch := range s.subs {
		delete(s.subs, id)
		close(ch)
	}
}

func publishLatest(ch chan dto.SnapshotChanged, notice dto.SnapshotChanged) {
	select {
	case ch <- notice:
		return
	default:
	}
	select {
	case <-ch:
	default:
	}
	select {
	case ch <- notice:
	default:
	}
}

func newInstanceID() string {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		// crypto/rand failure is not expected on supported OSes. A stable
		// process-local fallback is still preferable to an empty identity.
		return "desktop-instance-unknown"
	}
	return hex.EncodeToString(raw[:])
}
