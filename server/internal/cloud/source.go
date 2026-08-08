// Package cloud provides the cloud storage abstraction layer for importing
// remote assets into a local Lumilio repository (Cloud → Local sync).
//
// Layering:
//
//	CloudProvider (iCloud today; other providers may implement the interface)  ← interface
//	      ↓
//	SyncStateStore                     ← pagination cursor + etag dedup
//	      ↓
//	CloudImportSource                  ← implements sourcing.AssetSource
//	      ↓
//	CloudSyncConsumer                  ← acknowledged materializer loop
package cloud

import (
	"context"
	"io"
	"time"

	"github.com/google/uuid"
)

// ProviderKind identifies a cloud storage backend.
type ProviderKind string

const (
	ProviderICloud ProviderKind = "icloud"
)

// ReleaseAsset describes a single file discovered in cloud storage.
type ReleaseAsset struct {
	Provider   ProviderKind
	RemoteKey  string // provider-specific object key
	Filename   string // original filename
	Size       int64
	MIME       string
	ETag       string // for change detection
	ModifiedAt time.Time
	Deleted    bool // true when the provider reports a tombstone
}

// ImportProgressDelta records incremental progress for a cloud import run.
type ImportProgressDelta struct {
	TotalSeen  int64
	Downloaded int64
	Imported   int64
	Skipped    int64
	Failed     int64
}

// Cursor is an opaque pagination marker returned by cloud providers.
// Value is persisted to the DB; Metadata is ephemeral (per-provider extras).
type Cursor struct {
	Value    string
	Metadata map[string]any
}

// Page is a single page of listed remote files.
type Page struct {
	Assets  []ReleaseAsset
	Cursor  *Cursor // nil when no more pages remain
	HasMore bool
}

// CloudProvider abstracts a remote file storage backend.
type CloudProvider interface {
	// Name returns the provider identifier.
	Name() ProviderKind

	// List returns files changed since the given cursor.
	// Pass nil cursor to start from the beginning (full listing).
	List(ctx context.Context, repoID uuid.UUID, cursor *Cursor) (*Page, error)

	// Download streams a remote file to the caller-owned staging handle.
	// Returns the number of bytes written.
	Download(ctx context.Context, repoID uuid.UUID, remoteKey string, destination io.Writer) (int64, error)
}
