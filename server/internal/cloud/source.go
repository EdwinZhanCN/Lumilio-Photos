// Package cloud provides the cloud storage abstraction layer for importing
// remote assets into a local Lumilio repository (Cloud → Local sync).
//
// Layering:
//
//	CloudProvider (S3/WebDAV/iCloud)  ← interface
//	      ↓
//	SyncStateStore                     ← pagination cursor + etag dedup
//	      ↓
//	CloudImportSource                  ← implements sourcing.AssetSource
//	      ↓
//	CloudSyncConsumer                  ← channel → materializer loop
package cloud

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// ProviderKind identifies a cloud storage backend.
type ProviderKind string

const (
	ProviderICloud ProviderKind = "icloud"
	ProviderS3     ProviderKind = "s3"
)

// SyncMode controls how cloud changes are applied to the local repository.
type SyncMode string

const (
	// SyncModeImport downloads new and changed files only. Remote deletes are ignored.
	SyncModeImport SyncMode = "import"

	// SyncModeOneWay downloads new/changed files and soft-deletes local assets
	// when the corresponding remote file has been deleted (tombstone).
	SyncModeOneWay SyncMode = "one_way"
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
// Each concrete provider (S3, iCloud, WebDAV) implements this interface.
type CloudProvider interface {
	// Name returns the provider identifier.
	Name() ProviderKind

	// List returns files changed since the given cursor.
	// Pass nil cursor to start from the beginning (full listing).
	List(ctx context.Context, repoID uuid.UUID, cursor *Cursor) (*Page, error)

	// Download fetches a remote file to the given local path.
	// Returns the number of bytes written.
	Download(ctx context.Context, repoID uuid.UUID, remoteKey string, localPath string) (int64, error)
}
