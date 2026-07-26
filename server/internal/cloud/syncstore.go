package cloud

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"

	"server/internal/db/repo"
)

// SyncStateStore persists cloud sync pagination cursors and per-file etag
// tracking so that subsequent sync cycles can resume incrementally.
type SyncStateStore interface {
	// GetCursor returns the last saved pagination cursor for a repository+provider pair.
	// Returns empty string when no cursor exists.
	GetCursor(ctx context.Context, repositoryID uuid.UUID, provider ProviderKind) (string, error)

	// SaveCursor persists the latest pagination cursor after a successful page.
	SaveCursor(ctx context.Context, repositoryID uuid.UUID, provider ProviderKind, cursor string) error

	// IsFileSynced checks whether a remote file (key + etag) has already been ingested.
	IsFileSynced(ctx context.Context, repositoryID uuid.UUID, provider ProviderKind, remoteKey, etag string) (bool, error)

	// MarkFileSynced records that a remote file was successfully ingested. A
	// nil assetID is stored when the file was deduped (already present), which
	// still records the etag so later runs skip it.
	MarkFileSynced(ctx context.Context, repositoryID uuid.UUID, provider ProviderKind, remoteKey, etag string, assetID uuid.UUID) error
}

// sqliteSyncStateStore is the SQLite-backed implementation of SyncStateStore.
type sqliteSyncStateStore struct {
	queries      *repo.Queries
	credentialID uuid.UUID
}

// NewSQLiteSyncStateStore creates a SyncStateStore backed by SQLite via sqlc-generated queries.
func NewSQLiteSyncStateStore(queries *repo.Queries, credentialID uuid.UUID) SyncStateStore {
	return &sqliteSyncStateStore{queries: queries, credentialID: credentialID}
}

func (s *sqliteSyncStateStore) GetCursor(ctx context.Context, repositoryID uuid.UUID, provider ProviderKind) (string, error) {
	val, err := s.queries.GetCloudSyncCursor(ctx, repo.GetCloudSyncCursorParams{
		RepositoryID: repositoryID,
		CredentialID: s.credentialID,
		Provider:     string(provider),
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	return val, nil
}

func (s *sqliteSyncStateStore) SaveCursor(ctx context.Context, repositoryID uuid.UUID, provider ProviderKind, cursor string) error {
	return s.queries.UpsertCloudSyncCursor(ctx, repo.UpsertCloudSyncCursorParams{
		RepositoryID: repositoryID,
		CredentialID: s.credentialID,
		Provider:     string(provider),
		CursorValue:  cursor,
	})
}

func (s *sqliteSyncStateStore) IsFileSynced(ctx context.Context, repositoryID uuid.UUID, provider ProviderKind, remoteKey, etag string) (bool, error) {
	row, err := s.queries.GetCloudSyncFile(ctx, repo.GetCloudSyncFileParams{
		RepositoryID: repositoryID,
		CredentialID: s.credentialID,
		Provider:     string(provider),
		RemoteKey:    remoteKey,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return row.Etag == etag, nil
}

func (s *sqliteSyncStateStore) MarkFileSynced(ctx context.Context, repositoryID uuid.UUID, provider ProviderKind, remoteKey, etag string, assetID uuid.UUID) error {
	nullableAssetID := uuid.NullUUID{UUID: assetID, Valid: assetID != uuid.Nil}
	return s.queries.MarkCloudSyncFile(ctx, repo.MarkCloudSyncFileParams{
		RepositoryID: repositoryID,
		CredentialID: s.credentialID,
		Provider:     string(provider),
		RemoteKey:    remoteKey,
		Etag:         etag,
		LocalHash:    "", // filled lazily; materializer computes BLAKE3
		AssetID:      nullableAssetID,
	})
}
