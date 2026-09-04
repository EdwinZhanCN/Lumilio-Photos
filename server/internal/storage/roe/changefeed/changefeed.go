// Package changefeed defines the narrow native-change contract consumed by
// the Repository Observation Engine controller. Adapters report opaque
// cursors and dirty path hints; they never mutate catalog state or decide
// identity, binding, or absence.
package changefeed

import (
	"bytes"
	"context"
	"errors"
	"strings"

	"server/internal/db/repo"

	"github.com/google/uuid"
)

type Health string

const (
	HealthHealthy     Health = "healthy"
	HealthGap         Health = "gap"
	HealthOverflow    Health = "overflow"
	HealthUnavailable Health = "unavailable"
)

type EventKind string

const (
	EventCreate EventKind = "create"
	EventModify EventKind = "modify"
	EventRemove EventKind = "remove"
	EventRename EventKind = "rename"
)

// Checkpoint is an adapter-owned position. Cursor bytes are opaque outside the
// adapter. Identity fields make a cursor fail closed when a volume, journal,
// or live-capture instance changes.
type Checkpoint struct {
	AdapterKind     string
	Cursor          []byte
	VolumeIdentity  string
	VolumeKind      string
	JournalIdentity string
	Health          Health
}

func (checkpoint Checkpoint) Valid() bool {
	return checkpoint.Health == HealthHealthy && strings.TrimSpace(checkpoint.AdapterKind) != "" &&
		len(checkpoint.Cursor) > 0 && strings.TrimSpace(checkpoint.VolumeIdentity) != ""
}

func (checkpoint Checkpoint) SameIdentity(other Checkpoint) bool {
	return checkpoint.AdapterKind == other.AdapterKind &&
		checkpoint.VolumeIdentity == other.VolumeIdentity &&
		checkpoint.JournalIdentity == other.JournalIdentity
}

func (checkpoint Checkpoint) SamePosition(other Checkpoint) bool {
	return checkpoint.SameIdentity(other) && bytes.Equal(checkpoint.Cursor, other.Cursor)
}

// Event is an advisory dirty-path fact. Paths are slash-separated and
// repository-relative. Recursive asks the controller to verify the named
// subtree, as required for coalesced native events.
type Event struct {
	Key       string
	Kind      EventKind
	Path      string
	OldPath   string
	Recursive bool
	Cursor    []byte
}

// Batch covers (After, Through] and advances Next only over the returned,
// durably applicable event prefix. Done means the requested Through boundary
// has been exhausted.
type Batch struct {
	Events []Event
	Next   Checkpoint
	Done   bool
}

// Feed is safe for repeated calls after crashes. Snapshot must begin or
// confirm capture before returning its checkpoint. Read never interprets a
// cursor produced by a different adapter identity.
type Feed interface {
	Snapshot(context.Context, repo.Repository) (Checkpoint, error)
	Read(context.Context, repo.Repository, Checkpoint, Checkpoint, int) (Batch, error)
}

// DirectoryWatcher is implemented by live feeds that need watches installed
// directory-by-directory (notably inotify). The controller calls it before
// enumerating that directory, so capture always precedes authoritative reads
// without a separate unbounded watcher-registration walk.
type DirectoryWatcher interface {
	WatchDirectory(context.Context, repo.Repository, string) error
}

// NotificationSource wakes the controller when a live adapter has new hints.
// Notifications are deliberately lossy: Request coalescing and the persisted
// cursor make one wakeup sufficient for any number of queued native events.
type NotificationSource interface {
	Notifications() <-chan uuid.UUID
}

type Closer interface {
	Close() error
}

var ErrCursorInvalid = errors.New("repository change cursor is invalid")

// Periodic is the explicit unsupported-volume fallback. It provides positive
// full-verification observations but never authorizes absence finalization.
type Periodic struct{}

func (Periodic) Snapshot(context.Context, repo.Repository) (Checkpoint, error) {
	return Checkpoint{AdapterKind: "periodic", Health: HealthUnavailable}, nil
}

func (Periodic) Read(_ context.Context, _ repo.Repository, _, through Checkpoint, _ int) (Batch, error) {
	return Batch{Next: through, Done: true}, nil
}
