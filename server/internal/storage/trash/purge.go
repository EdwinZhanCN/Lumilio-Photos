package trash

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"server/internal/db/catalogtx"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/lifecycle"
	"server/internal/storage"
)

// purgeBatch bounds one purge transaction, like every other lifecycle write.
const purgeBatch = 256

// PurgeResult is what an irreversible step removed.
type PurgeResult struct {
	Entries int64
	Files   int
	Bytes   int64
	// Assets are the Assets deleted because no entry was left for them.
	Assets int
}

func (r *PurgeResult) add(other lifecycle.PurgeResult) {
	r.Entries += other.Entries
	r.Assets += len(other.Assets)
}

// Expire deletes permanently every trashed file older than the retention:
// each file is unlinked from its repository trash, then its entry is purged,
// taking its Asset with it when no other entry is left. A repository that is
// offline keeps its trash until it is back. It is idempotent: a file already
// gone is purged all the same.
func (s *Service) Expire(ctx context.Context, retention time.Duration) (PurgeResult, error) {
	var result PurgeResult
	if retention <= 0 {
		return result, errors.New("trash retention must be positive")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	cutoff := dbtypes.NewTimestamp(s.now().Add(-retention))
	repositories, err := s.reader.ListRepositories(ctx)
	if err != nil {
		return result, fmt.Errorf("list repositories for trash expiry: %w", err)
	}
	var failures []error
	for _, repository := range repositories {
		if repository.Reachability != dbtypes.RepositoryReachabilityActive {
			continue
		}
		opened, err := s.openRepositories(ctx, []uuid.UUID{repository.RepoID})
		if err != nil {
			s.logger.Info("trash expiry waits for an unavailable repository",
				zap.String("repository_id", repository.RepoID.String()), zap.Error(err))
			continue
		}
		for {
			entries, err := s.reader.ListExpiredTrashedRepositoryEntries(ctx, repo.ListExpiredTrashedRepositoryEntriesParams{
				RepositoryID: repository.RepoID, Cutoff: cutoff, RowLimit: purgeBatch,
			})
			if err != nil {
				failures = append(failures, err)
				break
			}
			if len(entries) == 0 {
				break
			}
			batch, err := s.unlinkAndPurge(ctx, opened, entries, "trash_expired", Request{Actor: "server"})
			result.Files += batch.Files
			result.Bytes += batch.Bytes
			result.Entries += batch.Entries
			result.Assets += batch.Assets
			if err != nil {
				failures = append(failures, err)
				break
			}
		}
		opened.close()
	}
	return result, errors.Join(failures...)
}

// DeletePermanently unlinks the trashed files of the selected Assets and
// purges them. It is the Trash view's explicit, confirmed action; it refuses
// the request when a repository holding one of the files is offline.
func (s *Service) DeletePermanently(ctx context.Context, request Request) (PurgeResult, error) {
	var result PurgeResult
	assetIDs := uniqueIDs(request.AssetIDs)
	if len(assetIDs) == 0 {
		return result, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := s.reader.ListTrashedRepositoryEntriesForAssets(ctx, nullIDs(assetIDs))
	if err != nil {
		return result, fmt.Errorf("resolve the selected Assets' trashed files: %w", err)
	}
	trashed := map[uuid.UUID]bool{}
	var repositoryIDs []uuid.UUID
	for _, entry := range entries {
		trashed[entry.AssetID.UUID] = true
		repositoryIDs = append(repositoryIDs, entry.RepositoryID)
	}
	for _, assetID := range assetIDs {
		if !trashed[assetID] {
			return result, &Rejection{Reason: ReasonNotTrashed, AssetID: assetID,
				Cause: errors.New("the Asset has no file in the trash")}
		}
	}
	opened, err := s.openRepositories(ctx, uniqueIDs(repositoryIDs))
	if err != nil {
		return result, err
	}
	defer opened.close()
	for start := 0; start < len(entries); start += purgeBatch {
		end := min(start+purgeBatch, len(entries))
		batch, err := s.unlinkAndPurge(ctx, opened, entries[start:end], "delete_permanently", request)
		result.Files += batch.Files
		result.Bytes += batch.Bytes
		result.Entries += batch.Entries
		result.Assets += batch.Assets
		if err != nil {
			return result, err
		}
	}
	return result, nil
}

// RemoveMissingRequest selects missing files to forget: those of the listed
// Assets, or every missing file of one repository.
type RemoveMissingRequest struct {
	Request
	RepositoryID uuid.UUID
}

// RemoveMissing purges missing entries. The files are already gone, so no
// file is touched; an Asset left without an entry is deleted with its
// metadata. A file that comes back later is imported as new.
func (s *Service) RemoveMissing(ctx context.Context, request RemoveMissingRequest) (PurgeResult, error) {
	var result PurgeResult
	s.mu.Lock()
	defer s.mu.Unlock()
	if request.RepositoryID != uuid.Nil {
		for {
			entries, err := s.reader.ListMissingRepositoryEntries(ctx, repo.ListMissingRepositoryEntriesParams{
				RepositoryID: request.RepositoryID, RowLimit: purgeBatch,
			})
			if err != nil {
				return result, err
			}
			if len(entries) == 0 {
				return result, nil
			}
			if err := s.purgeTx(ctx, entries, "remove_missing", request.Request, 0, 0, &result); err != nil {
				return result, err
			}
		}
	}
	assetIDs := uniqueIDs(request.AssetIDs)
	if len(assetIDs) == 0 {
		return result, nil
	}
	entries, err := s.reader.ListMissingRepositoryEntriesForAssets(ctx, nullIDs(assetIDs))
	if err != nil {
		return result, err
	}
	for start := 0; start < len(entries); start += purgeBatch {
		end := min(start+purgeBatch, len(entries))
		if err := s.purgeTx(ctx, entries[start:end], "remove_missing", request.Request, 0, 0, &result); err != nil {
			return result, err
		}
	}
	return result, nil
}

// unlinkAndPurge removes trashed files from disk, then purges their entries.
// A crash between the two leaves trashed entries whose files are gone; the
// next expiry or delete purges them, since a file already gone counts as
// unlinked.
func (s *Service) unlinkAndPurge(ctx context.Context, opened repositories, entries []repo.RepositoryEntry, action string, request Request) (PurgeResult, error) {
	var result PurgeResult
	var files int
	var bytes int64
	for _, entry := range entries {
		repositoryFS := opened[entry.RepositoryID]
		if repositoryFS == nil {
			return result, &Rejection{Reason: ReasonRepositoryOffline, RepositoryID: entry.RepositoryID}
		}
		if err := unlinkTrashed(repositoryFS, entry); err != nil {
			return result, fmt.Errorf("unlink trashed %s: %w", entry.Path, err)
		}
		files++
		bytes += entry.Size
	}
	err := s.purgeTx(ctx, entries, action, request, files, bytes, &result)
	result.Files, result.Bytes = files, bytes
	return result, err
}

// unlinkTrashed removes one trashed file, its trash directory, and its info
// sidecar. Anything already gone is fine.
func unlinkTrashed(files *storage.RepositoryFS, entry repo.RepositoryEntry) error {
	if !entry.TrashID.Valid {
		return errors.New("entry is not in the trash")
	}
	trashed, err := storage.TrashFilePath(entry.TrashID.UUID, path.Base(entry.Path))
	if err != nil {
		return err
	}
	directory, err := storage.TrashFileDirectory(entry.TrashID.UUID)
	if err != nil {
		return err
	}
	info, err := storage.TrashInfoPath(entry.TrashID.UUID)
	if err != nil {
		return err
	}
	for _, target := range []storage.RepositoryPath{trashed, directory, info} {
		if err := files.RemovePrivate(target); err != nil && !isNotExist(err) {
			return err
		}
	}
	return nil
}

// purgeTx purges one batch of entries and audits it in the same transaction.
func (s *Service) purgeTx(ctx context.Context, entries []repo.RepositoryEntry, action string, request Request, files int, bytes int64, result *PurgeResult) error {
	ids := make([]uuid.UUID, 0, len(entries))
	for _, entry := range entries {
		ids = append(ids, entry.EntryID)
	}
	return s.writer.WithTx(ctx, catalogtx.OperationRepositoryRemove, func(tx *sql.Tx, queries *repo.Queries) error {
		purged, err := lifecycle.PurgeEntriesTx(ctx, tx, ids)
		if err != nil {
			return err
		}
		result.add(purged)
		actor := request.Actor
		if actor == "" {
			actor = "server"
		}
		_, err = storage.RecordLifecycleAuditTx(ctx, queries, storage.LifecycleAuditInput{
			Actor: actor, ActorUserID: request.ActorUserID, RequestID: request.RequestID,
			Action: action, TargetType: "asset", ConfirmationType: request.ConfirmationType, Result: storage.AuditResultSucceeded,
			Details: map[string]any{
				"entries": purged.Entries, "assets": len(purged.Assets), "files_unlinked": files, "bytes": bytes,
			},
		})
		return err
	})
}

// Empty removes only trashed entries in the authorized scope, preserving copies
// in other Repositories. The caller supplies owner scope for ordinary users.
func (s *Service) Empty(ctx context.Context, request Request, repositoryID uuid.NullUUID, ownerID *int32) (PurgeResult, error) {
	var result PurgeResult
	s.mu.Lock()
	defer s.mu.Unlock()
	// Open every affected Repository before unlinking the first file.
	repositories, err := s.reader.ListRepositories(ctx)
	if err != nil {
		return result, err
	}
	var ids []uuid.UUID
	for _, repository := range repositories {
		if repositoryID.Valid && repositoryID.UUID != repository.RepoID {
			continue
		}
		entries, err := s.reader.ListScopedTrashedRepositoryEntries(ctx, repo.ListScopedTrashedRepositoryEntriesParams{RepositoryID: uuid.NullUUID{UUID: repository.RepoID, Valid: true}, OwnerID: ownerID, RowLimit: 1})
		if err != nil {
			return result, err
		}
		if len(entries) > 0 {
			ids = append(ids, repository.RepoID)
		}
	}
	opened, err := s.openRepositories(ctx, ids)
	if err != nil {
		return result, err
	}
	defer opened.close()
	for {
		entries, err := s.reader.ListScopedTrashedRepositoryEntries(ctx, repo.ListScopedTrashedRepositoryEntriesParams{RepositoryID: repositoryID, OwnerID: ownerID, RowLimit: purgeBatch})
		if err != nil {
			return result, err
		}
		if len(entries) == 0 {
			return result, nil
		}
		batch, err := s.unlinkAndPurge(ctx, opened, entries, "empty_trash", request)
		result.Files += batch.Files
		result.Bytes += batch.Bytes
		result.Entries += batch.Entries
		result.Assets += batch.Assets
		if err != nil {
			return result, err
		}
	}
}
