package cloud

import (
	"context"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"server/internal/sourcing"
)

// CloudImportSourceConfig holds the dependencies needed to construct a CloudImportSource.
type CloudImportSourceConfig struct {
	Provider   CloudProvider
	State      SyncStateStore
	StagingDir string // temporary download directory, e.g. /tmp/lumilio-cloud-sync
	RepoID     uuid.UUID
	OwnerID    *int32 // optional; when nil the materializer falls back to repository default
	OnProgress func(delta ImportProgressDelta)
	Logger     *zap.Logger
}

// CloudImportSource implements sourcing.AssetSource for a cloud storage provider.
// It discovers remote files via CloudProvider.List, downloads them to a local
// staging directory, and emits sourcing.IngestSource candidates for the
// SourceMaterializer.
type CloudImportSource struct {
	provider   CloudProvider
	state      SyncStateStore
	stagingDir string
	repoID     uuid.UUID
	ownerID    *int32
	onProgress func(delta ImportProgressDelta)
	logger     *zap.Logger
}

// NewCloudImportSource creates a CloudImportSource.
func NewCloudImportSource(cfg CloudImportSourceConfig) *CloudImportSource {
	if cfg.Logger == nil {
		cfg.Logger = zap.NewNop()
	}
	if cfg.StagingDir == "" {
		cfg.StagingDir = os.TempDir()
	}
	return &CloudImportSource{
		provider:   cfg.Provider,
		state:      cfg.State,
		stagingDir: cfg.StagingDir,
		repoID:     cfg.RepoID,
		ownerID:    cfg.OwnerID,
		onProgress: cfg.OnProgress,
		logger:     cfg.Logger.With(zap.String("component", "cloud_import_source"), zap.String("provider", string(cfg.Provider.Name()))),
	}
}

// Kind returns sourcing.IngestSourceCloud.
func (s *CloudImportSource) Kind() sourcing.IngestSourceKind {
	return sourcing.IngestSourceCloud
}

// Discover lists remote files from the cloud provider, downloads new/changed
// files to a local staging directory, and emits IngestSource candidates.
//
// The returned channel is closed when all pages have been consumed or ctx is
// cancelled.  Callers should range over the channel and call
// SourceMaterializer.Materialize for each candidate.
func (s *CloudImportSource) Discover(ctx context.Context) (<-chan sourcing.IngestSource, error) {
	ch := make(chan sourcing.IngestSource, 10)

	go func() {
		defer close(ch)

		// Resume from last saved cursor
		cursorValue, err := s.state.GetCursor(ctx, s.repoID, s.provider.Name())
		if err != nil {
			s.logger.Error("failed to load sync cursor", zap.Error(err))
			return
		}

		var cursor *Cursor
		if cursorValue != "" {
			cursor = &Cursor{Value: cursorValue}
		}

		for {
			page, err := s.provider.List(ctx, s.repoID, cursor)
			if err != nil {
				s.logger.Error("list remote files failed", zap.Error(err))
				return
			}

			for _, ra := range page.Assets {
				s.progress(ImportProgressDelta{TotalSeen: 1})

				select {
				case <-ctx.Done():
					return
				default:
				}

				// Import-only mode ignores remote tombstones. Lumilio never deletes
				// local media because an upstream cloud provider reports a delete.
				if ra.Deleted {
					s.progress(ImportProgressDelta{Skipped: 1})
					continue
				}

				// Skip already-synced files (etag-based dedup)
				if synced, _ := s.state.IsFileSynced(ctx, s.repoID, s.provider.Name(), ra.RemoteKey, ra.ETag); synced {
					s.progress(ImportProgressDelta{Skipped: 1})
					continue
				}

				// Download to staging (etag-based dedup already checked above)
				stagingPath := filepath.Join(s.stagingDir, uuid.New().String())
				if _, err := s.provider.Download(ctx, s.repoID, ra.RemoteKey, stagingPath); err != nil {
					s.logger.Error("download failed",
						zap.String("remote_key", ra.RemoteKey),
						zap.Error(err),
					)
					s.progress(ImportProgressDelta{Failed: 1})
					continue
				}
				s.progress(ImportProgressDelta{Downloaded: 1})

				select {
				case <-ctx.Done():
					return
				case ch <- sourcing.IngestSource{
					RepositoryID:     s.repoID,
					OwnerID:          s.ownerID,
					Kind:             sourcing.IngestSourceCloud,
					SourcePath:       stagingPath,
					OriginalFilename: ra.Filename,
					Size:             ra.Size,
					ContentHash:      nil, // materializer computes BLAKE3
					Timestamp:        ra.ModifiedAt,
					ContentType:      ra.MIME,
					Metadata: map[string]any{
						"provider":    s.provider.Name(),
						"remote_key":  ra.RemoteKey,
						"remote_etag": ra.ETag,
					},
				}:
				}
			}

			// Persist cursor after a successful page
			if page.Cursor != nil {
				if err := s.state.SaveCursor(ctx, s.repoID, s.provider.Name(), page.Cursor.Value); err != nil {
					s.logger.Warn("failed to save sync cursor", zap.Error(err))
				}
			}

			if !page.HasMore || page.Cursor == nil {
				break
			}
			cursor = page.Cursor
		}
	}()

	return ch, nil
}

func (s *CloudImportSource) progress(delta ImportProgressDelta) {
	if s.onProgress != nil {
		s.onProgress(delta)
	}
}
