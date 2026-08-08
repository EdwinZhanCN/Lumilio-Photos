package cloud

import (
	"context"
	"errors"
	"fmt"

	"server/internal/db/repo"
	"server/internal/sourcing"
	"server/internal/storage"
)

// ImportStaging is the narrow private-workspace capability cloud downloads
// need before handing a candidate to the materializer.
type ImportStaging interface {
	CreateStagingFile(repository repo.Repository, filename string) (*storage.StagingFile, *storage.RepositoryFile, error)
	MoveStagingToFailed(repository repo.Repository, stagingFile *storage.StagingFile) error
}

// CloudImportSourceConfig holds the dependencies needed to construct a CloudImportSource.
type CloudImportSourceConfig struct {
	Provider   CloudProvider
	State      SyncStateStore
	Repository repo.Repository
	Staging    ImportStaging
	OwnerID    *int32 // optional; when nil the materializer falls back to repository default
	OnProgress func(delta ImportProgressDelta)
}

// CloudImportSource implements sourcing.AssetSource for a cloud storage provider.
// It discovers remote files via CloudProvider.List, downloads them through the
// repository staging capability, and emits sourcing.IngestSource candidates for the
// SourceMaterializer.
type CloudImportSource struct {
	provider   CloudProvider
	state      SyncStateStore
	repository repo.Repository
	staging    ImportStaging
	ownerID    *int32
	onProgress func(delta ImportProgressDelta)
}

// NewCloudImportSource creates a CloudImportSource.
func NewCloudImportSource(cfg CloudImportSourceConfig) *CloudImportSource {
	return &CloudImportSource{
		provider:   cfg.Provider,
		state:      cfg.State,
		repository: cfg.Repository,
		staging:    cfg.Staging,
		ownerID:    cfg.OwnerID,
		onProgress: cfg.OnProgress,
	}
}

// Kind returns sourcing.IngestSourceCloud.
func (s *CloudImportSource) Kind() sourcing.IngestSourceKind {
	return sourcing.IngestSourceCloud
}

// ForEach lists and stages remote files in page order. The durable cursor moves
// only after every candidate in the page has been materialized and recorded.
func (s *CloudImportSource) ForEach(ctx context.Context, consume func(sourcing.IngestSource) error) error {
	if consume == nil {
		return errors.New("cloud import consumer is nil")
	}
	cursorValue, err := s.state.GetCursor(ctx, s.repository.RepoID, s.provider.Name())
	if err != nil {
		return fmt.Errorf("load cloud sync cursor: %w", err)
	}
	var cursor *Cursor
	if cursorValue != "" {
		cursor = &Cursor{Value: cursorValue}
	}

	for {
		page, err := s.provider.List(ctx, s.repository.RepoID, cursor)
		if err != nil {
			return fmt.Errorf("list remote files: %w", err)
		}
		for _, remote := range page.Assets {
			s.progress(ImportProgressDelta{TotalSeen: 1})
			if err := ctx.Err(); err != nil {
				return err
			}
			if remote.Deleted {
				s.progress(ImportProgressDelta{Skipped: 1})
				continue
			}
			synced, err := s.state.IsFileSynced(ctx, s.repository.RepoID, s.provider.Name(), remote.RemoteKey, remote.ETag)
			if err != nil {
				return fmt.Errorf("check cloud sync state for %s: %w", remote.RemoteKey, err)
			}
			if synced {
				s.progress(ImportProgressDelta{Skipped: 1})
				continue
			}

			staged, destination, err := s.staging.CreateStagingFile(s.repository, remote.Filename)
			if err != nil {
				s.progress(ImportProgressDelta{Failed: 1})
				return fmt.Errorf("create cloud staging file for %s: %w", remote.RemoteKey, err)
			}
			downloaded, downloadErr := s.provider.Download(ctx, s.repository.RepoID, remote.RemoteKey, destination)
			if downloadErr == nil && remote.Size > 0 && downloaded != remote.Size {
				downloadErr = errors.New("cloud download size mismatch")
			}
			if downloadErr == nil {
				downloadErr = destination.Sync()
			}
			closeErr := destination.Close()
			if err := errors.Join(downloadErr, closeErr); err != nil {
				_ = s.staging.MoveStagingToFailed(s.repository, staged)
				s.progress(ImportProgressDelta{Failed: 1})
				return fmt.Errorf("download cloud file %s: %w", remote.RemoteKey, err)
			}
			s.progress(ImportProgressDelta{Downloaded: 1})

			if err := consume(sourcing.IngestSource{
				RepositoryID:     s.repository.RepoID,
				OwnerID:          s.ownerID,
				Kind:             sourcing.IngestSourceCloud,
				StagingPath:      staged.PrivatePath,
				OriginalFilename: remote.Filename,
				Size:             remote.Size,
				Timestamp:        remote.ModifiedAt,
				ContentType:      remote.MIME,
				Metadata: map[string]any{
					"provider":    s.provider.Name(),
					"remote_key":  remote.RemoteKey,
					"remote_etag": remote.ETag,
				},
			}); err != nil {
				return err
			}
		}

		if page.Cursor != nil {
			if err := s.state.SaveCursor(ctx, s.repository.RepoID, s.provider.Name(), page.Cursor.Value); err != nil {
				return fmt.Errorf("save cloud sync cursor: %w", err)
			}
		}
		if !page.HasMore || page.Cursor == nil {
			return nil
		}
		cursor = page.Cursor
	}
}

func (s *CloudImportSource) progress(delta ImportProgressDelta) {
	if s.onProgress != nil {
		s.onProgress(delta)
	}
}
