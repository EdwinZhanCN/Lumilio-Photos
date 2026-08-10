package cloud

import (
	"context"
	"errors"
	"fmt"
	"io"

	"server/internal/db/repo"
	"server/internal/sourcing"
	"server/internal/storage"
)

const capacityResampleBytes int64 = 16 << 20

type capacitySamplingWriter struct {
	ctx          context.Context
	repositoryID string
	guard        RepositoryCapacityGuard
	destination  io.Writer
	sinceSample  int64
}

func (w *capacitySamplingWriter) Write(data []byte) (int, error) {
	total := 0
	for len(data) > 0 {
		if w.guard != nil && w.sinceSample >= capacityResampleBytes {
			if _, err := w.guard.CheckRepositoryWriteCapacity(w.ctx, w.repositoryID, 0); err != nil {
				return total, fmt.Errorf("continuous capacity check: %w", err)
			}
			w.sinceSample = 0
		}
		chunk := data
		remaining := capacityResampleBytes - w.sinceSample
		if remaining > 0 && int64(len(chunk)) > remaining {
			chunk = chunk[:remaining]
		}
		written, err := w.destination.Write(chunk)
		total += written
		w.sinceSample += int64(written)
		data = data[written:]
		if err != nil {
			return total, err
		}
		if written != len(chunk) {
			return total, io.ErrShortWrite
		}
	}
	return total, nil
}

// ImportStaging is the narrow private-workspace capability cloud downloads
// need before handing a candidate to the materializer.
type ImportStaging interface {
	CreateStagingFile(repository repo.Repository, filename string) (*storage.StagingFile, *storage.RepositoryFile, error)
	RemoveStagingFile(repository repo.Repository, stagingFile *storage.StagingFile) error
	MoveStagingToFailed(repository repo.Repository, stagingFile *storage.StagingFile) error
}

type RepositoryCapacityGuard interface {
	CheckRepositoryWriteCapacity(context.Context, string, uint64) (storage.CapacityDecision, error)
}

// CloudImportSourceConfig holds the dependencies needed to construct a CloudImportSource.
type CloudImportSourceConfig struct {
	Provider      CloudProvider
	State         SyncStateStore
	Repository    repo.Repository
	Staging       ImportStaging
	OwnerID       *int32 // optional; when nil the materializer falls back to repository default
	OnProgress    func(delta ImportProgressDelta)
	RemoteScope   map[string]string
	CapacityGuard RepositoryCapacityGuard
}

// CloudImportSource implements sourcing.AssetSource for a cloud storage provider.
// It discovers remote files via CloudProvider.List, downloads them through the
// repository staging capability, and emits sourcing.IngestSource candidates for the
// SourceMaterializer.
type CloudImportSource struct {
	provider      CloudProvider
	state         SyncStateStore
	repository    repo.Repository
	staging       ImportStaging
	ownerID       *int32
	onProgress    func(delta ImportProgressDelta)
	remoteScope   map[string]string
	capacityGuard RepositoryCapacityGuard
}

// NewCloudImportSource creates a CloudImportSource.
func NewCloudImportSource(cfg CloudImportSourceConfig) *CloudImportSource {
	return &CloudImportSource{
		provider:      cfg.Provider,
		state:         cfg.State,
		repository:    cfg.Repository,
		staging:       cfg.Staging,
		ownerID:       cfg.OwnerID,
		onProgress:    cfg.OnProgress,
		remoteScope:   cfg.RemoteScope,
		capacityGuard: cfg.CapacityGuard,
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
		page, err := s.provider.List(ctx, s.repository.RepoID, cursor, s.remoteScope)
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
			if s.capacityGuard != nil {
				expectedBytes := uint64(0)
				if remote.Size > 0 {
					expectedBytes = uint64(remote.Size)
				}
				if _, err := s.capacityGuard.CheckRepositoryWriteCapacity(ctx, s.repository.RepoID.String(), expectedBytes); err != nil {
					return fmt.Errorf("cloud import capacity preflight for %s: %w", remote.RemoteKey, err)
				}
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
			downloadWriter := io.Writer(destination)
			if s.capacityGuard != nil {
				downloadWriter = &capacitySamplingWriter{
					ctx: ctx, repositoryID: s.repository.RepoID.String(), guard: s.capacityGuard, destination: destination,
				}
			}
			downloaded, downloadErr := s.provider.Download(ctx, s.repository.RepoID, remote.RemoteKey, downloadWriter)
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
				if !sourcing.StagingIsPrepared(err) {
					if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
						if cleanupErr := s.staging.RemoveStagingFile(s.repository, staged); cleanupErr != nil {
							return errors.Join(err, fmt.Errorf("remove unclaimed cloud staging: %w", cleanupErr))
						}
					} else if cleanupErr := s.staging.MoveStagingToFailed(s.repository, staged); cleanupErr != nil {
						return errors.Join(err, fmt.Errorf("quarantine unclaimed cloud staging: %w", cleanupErr))
					}
				}
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
