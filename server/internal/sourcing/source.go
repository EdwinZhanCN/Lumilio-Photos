// Package sourcing defines the unified asset source abstraction that decouples
// how assets are discovered (upload, scanner, cloud sync, import) from how they
// are materialized into the repository and fed into the ingest pipeline.
package sourcing

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// IngestSourceKind identifies the origin of an asset being ingested.
type IngestSourceKind string

const (
	IngestSourceUpload IngestSourceKind = "upload" // HTTP upload
	IngestSourceScan   IngestSourceKind = "scan"   // filesystem scanner
	IngestSourceCloud  IngestSourceKind = "cloud"  // cloud sync (S3, iCloud, GDrive, etc.)
)

// IngestSource is a unified asset candidate produced by any AssetSource.
// The SourceMaterializer consumes these to validate, materialize, and
// enqueue into the asset ingest pipeline.
type IngestSource struct {
	RepositoryID uuid.UUID
	OwnerID      *int32 // nullable; when nil the materializer falls back to repository default
	Kind         IngestSourceKind
	// SkipCommit skips staging→inbox commit. When true, SourcePath is treated
	// as the final repo-relative storage path (files already in-place, like
	// cloud provider downloads landing directly in cloud/).
	SkipCommit              bool
	SourcePath              string // staging path (upload/cloud) or repo-relative path (scan)
	OriginalFilename        string
	Size                    int64   // optional hint; the materializer always stats the file for the authoritative size
	ContentHash             *string // authoritative full hash for trusted in-place sources
	QuickFingerprint        *string // non-authoritative large-file precheck hint
	QuickFingerprintVersion *string
	Timestamp               time.Time
	ContentType             string
	Metadata                map[string]any // source-specific metadata (e.g. cloud object key, upload session ID)
}

// AssetSource produces IngestSource candidates from a specific origin.
// Each source type (upload handler, scanner, cloud sync provider, bulk importer)
// implements this interface so the materializer can consume them uniformly.
type AssetSource interface {
	// Kind returns the source kind identifier.
	Kind() IngestSourceKind

	// Discover sends discovered asset candidates to the returned channel.
	// The source must close the channel when discovery is complete.
	// The caller is responsible for cancelling ctx to stop discovery early.
	Discover(ctx context.Context) (<-chan IngestSource, error)
}
