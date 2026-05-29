package cloud

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

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

	// MarkFileSynced records that a remote file was successfully ingested and
	// maps it to a local asset for later tombstone reconciliation.
	MarkFileSynced(ctx context.Context, repositoryID uuid.UUID, provider ProviderKind, remoteKey, etag string, assetID uuid.UUID) error

	// GetAssetIDByRemoteKey looks up the local asset ID mapped to a remote file.
	// Returns uuid.Nil when no mapping exists.
	GetAssetIDByRemoteKey(ctx context.Context, repositoryID uuid.UUID, provider ProviderKind, remoteKey string) (uuid.UUID, error)
}

// pgSyncStateStore is the PostgreSQL-backed implementation of SyncStateStore.
type pgSyncStateStore struct {
	queries *repo.Queries
}

// NewPGSyncStateStore creates a SyncStateStore backed by PostgreSQL via sqlc-generated queries.
func NewPGSyncStateStore(queries *repo.Queries) SyncStateStore {
	return &pgSyncStateStore{queries: queries}
}

func (s *pgSyncStateStore) GetCursor(ctx context.Context, repositoryID uuid.UUID, provider ProviderKind) (string, error) {
	val, err := s.queries.GetCloudSyncCursor(ctx, repo.GetCloudSyncCursorParams{
		RepositoryID: toPGUUID(repositoryID),
		Provider:     string(provider),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	return val, nil
}

func (s *pgSyncStateStore) SaveCursor(ctx context.Context, repositoryID uuid.UUID, provider ProviderKind, cursor string) error {
	return s.queries.UpsertCloudSyncCursor(ctx, repo.UpsertCloudSyncCursorParams{
		RepositoryID: toPGUUID(repositoryID),
		Provider:     string(provider),
		CursorValue:  cursor,
	})
}

func (s *pgSyncStateStore) IsFileSynced(ctx context.Context, repositoryID uuid.UUID, provider ProviderKind, remoteKey, etag string) (bool, error) {
	row, err := s.queries.GetCloudSyncFile(ctx, repo.GetCloudSyncFileParams{
		RepositoryID: toPGUUID(repositoryID),
		Provider:     string(provider),
		RemoteKey:    remoteKey,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return row.Etag == etag, nil
}

func (s *pgSyncStateStore) MarkFileSynced(ctx context.Context, repositoryID uuid.UUID, provider ProviderKind, remoteKey, etag string, assetID uuid.UUID) error {
	var pgAssetID pgtype.UUID
	if assetID != uuid.Nil {
		pgAssetID = pgtype.UUID{Bytes: assetID, Valid: true}
	}
	return s.queries.MarkCloudSyncFile(ctx, repo.MarkCloudSyncFileParams{
		RepositoryID: toPGUUID(repositoryID),
		Provider:     string(provider),
		RemoteKey:    remoteKey,
		Etag:         etag,
		LocalHash:    "", // filled lazily; materializer computes BLAKE3
		AssetID:      pgAssetID,
	})
}

func (s *pgSyncStateStore) GetAssetIDByRemoteKey(ctx context.Context, repositoryID uuid.UUID, provider ProviderKind, remoteKey string) (uuid.UUID, error) {
	pgID, err := s.queries.GetAssetIDByCloudFile(ctx, repo.GetAssetIDByCloudFileParams{
		RepositoryID: toPGUUID(repositoryID),
		Provider:     string(provider),
		RemoteKey:    remoteKey,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, nil
		}
		return uuid.Nil, err
	}
	if !pgID.Valid {
		return uuid.Nil, nil
	}
	return uuid.UUID(pgID.Bytes), nil
}

func toPGUUID(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}
