package trash

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"server/internal/db/repo"
	"server/internal/storage"
	"server/internal/storage/scan"
)

// DeleteResult is what one Delete moved into the repository trash.
type DeleteResult struct {
	OperationID  string
	Assets       int
	Files        int
	Bytes        int64
	Repositories []uuid.UUID
}

// trashMove is one file's move into its repository trash, as the journal
// records it.
type trashMove struct {
	EntryID       uuid.UUID `json:"entry_id"`
	RepositoryID  uuid.UUID `json:"repository_id"`
	AssetID       uuid.UUID `json:"asset_id"`
	ContentID     uuid.UUID `json:"content_id"`
	Path          string    `json:"path"`
	Size          int64     `json:"size"`
	MtimeNs       int64     `json:"mtime_ns"`
	TrashID       uuid.UUID `json:"trash_id"`
	HashAlgorithm string    `json:"hash_algorithm"`
	ContentHash   string    `json:"content_hash"`
}

type trashPayload struct {
	AssetIDs  []uuid.UUID `json:"asset_ids"`
	DeletedAt time.Time   `json:"deleted_at"`
	Moves     []trashMove `json:"moves"`
}

func (m trashMove) source() (storage.RepositoryPath, error) {
	return storage.ParseUserMediaPath(m.Path)
}
func (m trashMove) destination() (storage.RepositoryPath, error) {
	return storage.TrashFilePath(m.TrashID, path.Base(m.Path))
}

// Delete moves every present file of the selected Assets into its
// repository's trash; each Asset becomes trashed with its metadata intact.
// It refuses the whole request, moving nothing, when a repository is not
// reachable, a file's stat tuple differs from the catalog, or an Asset has no
// file to move. A move that fails part-way is rolled back.
func (s *Service) Delete(ctx context.Context, request Request) (DeleteResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var result DeleteResult
	assetIDs := uniqueIDs(request.AssetIDs)
	if len(assetIDs) == 0 {
		return result, nil
	}
	entries, err := s.reader.ListRepositoryEntriesForAssets(ctx, nullIDs(assetIDs))
	if err != nil {
		return result, fmt.Errorf("resolve the selected Assets' files: %w", err)
	}
	byAsset := map[uuid.UUID][]repo.RepositoryEntry{}
	for _, entry := range entries {
		byAsset[entry.AssetID.UUID] = append(byAsset[entry.AssetID.UUID], entry)
	}
	now := s.now()
	payload := trashPayload{DeletedAt: now}
	var repositoryIDs []uuid.UUID
	for _, assetID := range assetIDs {
		present, trashed := 0, 0
		for _, entry := range byAsset[assetID] {
			switch entry.State {
			case scan.StatePendingHash:
				return result, &Rejection{Reason: ReasonFileChanged, AssetID: assetID, RepositoryID: entry.RepositoryID, Path: entry.Path,
					Cause: errors.New("the file changed since the last scan")}
			case scan.StatePresent:
				present++
				payload.Moves = append(payload.Moves, trashMove{
					EntryID: entry.EntryID, RepositoryID: entry.RepositoryID, AssetID: assetID,
					ContentID: entry.ContentID.UUID, Path: entry.Path, Size: entry.Size, MtimeNs: entry.MtimeNs,
					TrashID: uuid.New(),
				})
				repositoryIDs = append(repositoryIDs, entry.RepositoryID)
			case scan.StateTrashed:
				trashed++
			}
		}
		if present > 0 {
			payload.AssetIDs = append(payload.AssetIDs, assetID)
			continue
		}
		if trashed == 0 {
			return result, &Rejection{Reason: ReasonAssetMissing, AssetID: assetID,
				Cause: errors.New("the Asset has no file to move into the trash")}
		}
		// Already in the trash: deleting it again changes nothing.
	}
	if len(payload.Moves) == 0 {
		return result, nil
	}

	opened, err := s.openRepositories(ctx, uniqueIDs(repositoryIDs))
	if err != nil {
		return result, err
	}
	defer opened.close()
	byEntry := map[uuid.UUID]repo.RepositoryEntry{}
	for _, entry := range entries {
		byEntry[entry.EntryID] = entry
	}
	for index, move := range payload.Moves {
		source, err := move.source()
		if err != nil {
			return result, &Rejection{Reason: ReasonMoveFailed, AssetID: move.AssetID, RepositoryID: move.RepositoryID, Path: move.Path, Cause: err}
		}
		if _, err := move.destination(); err != nil {
			return result, &Rejection{Reason: ReasonMoveFailed, AssetID: move.AssetID, RepositoryID: move.RepositoryID, Path: move.Path, Cause: err}
		}
		observation, err := opened[move.RepositoryID].InspectMedia(ctx, source, storage.HashNone)
		if err != nil || observation.EntryKind != storage.EntryKindRegular || !scan.SameTuple(byEntry[move.EntryID], observation) {
			if err == nil {
				err = errors.New("the file's size or modification time differs from the last scan")
			}
			return result, &Rejection{Reason: ReasonFileChanged, AssetID: move.AssetID, RepositoryID: move.RepositoryID, Path: move.Path, Cause: err}
		}
		content, err := s.reader.GetContentObjectByID(ctx, move.ContentID)
		if err != nil {
			return result, fmt.Errorf("read content identity of %s: %w", move.Path, err)
		}
		payload.Moves[index].HashAlgorithm = content.HashAlgorithm
		payload.Moves[index].ContentHash = content.FullHash
	}

	entry, err := s.beginJournal(ctx, storage.LifecycleKindTrashAssets, request, payload.AssetIDs[0], payload)
	if err != nil {
		return result, err
	}
	for moved, move := range payload.Moves {
		err := s.moveToTrash(opened[move.RepositoryID], move, payload.DeletedAt, entry.request.Actor)
		if err == nil && s.afterMove != nil {
			err = s.afterMove(moved + 1)
			if errors.Is(err, errSimulatedCrash) {
				return result, err
			}
		}
		if err != nil {
			kept := s.rollbackTrash(opened, payload.Moves[:moved+1])
			s.failJournal(ctx, entry, "filesystem", err)
			if len(kept) > 0 {
				// A file that could not be put back is in the trash for real.
				if commitErr := s.commitTrash(ctx, entry, kept, now, false); commitErr != nil {
					return result, errors.Join(err, commitErr)
				}
			}
			return result, &Rejection{Reason: ReasonMoveFailed, AssetID: move.AssetID, RepositoryID: move.RepositoryID, Path: move.Path, Cause: err}
		}
	}
	if err := s.advanceJournal(ctx, entry, "filesystem_applied"); err != nil {
		return result, err
	}
	if err := s.commitTrash(ctx, entry, payload.Moves, now, true); err != nil {
		return result, err
	}

	result.OperationID = entry.operationID
	result.Assets = len(payload.AssetIDs)
	result.Files = len(payload.Moves)
	result.Repositories = uniqueIDs(repositoryIDs)
	for _, move := range payload.Moves {
		result.Bytes += move.Size
	}
	return result, nil
}

// moveToTrash writes the file's info sidecar and then moves the file into a
// fresh trash directory of its own. A move that fails leaves no sidecar.
func (s *Service) moveToTrash(files *storage.RepositoryFS, move trashMove, deletedAt time.Time, actor string) error {
	source, err := move.source()
	if err != nil {
		return err
	}
	destination, err := move.destination()
	if err != nil {
		return err
	}
	directory, err := storage.TrashFileDirectory(move.TrashID)
	if err != nil {
		return err
	}
	info, err := storage.TrashInfoPath(move.TrashID)
	if err != nil {
		return err
	}
	sidecar, err := Sidecar{
		TrashID: move.TrashID.String(), RepositoryID: move.RepositoryID.String(), OriginalPath: move.Path,
		AssetID: move.AssetID.String(), HashAlgorithm: move.HashAlgorithm, ContentHash: move.ContentHash,
		Size: move.Size, MtimeNs: move.MtimeNs, DeletedAt: deletedAt, Actor: actor,
	}.Marshal()
	if err != nil {
		return err
	}
	if err := files.MkdirAllPrivate(storage.TrashInfoDirectory(), 0o700); err != nil {
		return err
	}
	if err := files.MkdirAllPrivate(directory, 0o700); err != nil {
		return err
	}
	if _, err := files.WritePrivateFileAtomic(info, bytes.NewReader(sidecar), 0o600); err != nil {
		_ = files.RemovePrivate(directory)
		return err
	}
	if err := files.MoveNoReplace(source, destination); err != nil {
		_ = files.RemovePrivate(info)
		_ = files.RemovePrivate(directory)
		return err
	}
	return nil
}

// rollbackTrash puts moved files back, newest first, and returns the ones
// that could not go back because their original path is taken again.
func (s *Service) rollbackTrash(opened repositories, moves []trashMove) []trashMove {
	var kept []trashMove
	for index := len(moves) - 1; index >= 0; index-- {
		move := moves[index]
		files := opened[move.RepositoryID]
		source, _ := move.source()
		destination, err := move.destination()
		if err != nil {
			continue
		}
		inTrash, err := files.Exists(destination)
		if err != nil || !inTrash {
			continue
		}
		if err := files.MoveNoReplace(destination, source); err != nil {
			s.logger.Warn("trashed file could not be put back; it stays in the trash",
				zap.String("path", move.Path), zap.Error(err))
			kept = append(kept, move)
			continue
		}
		removeTrashRecord(files, move.TrashID)
	}
	return kept
}

// removeTrashRecord removes a trash directory and its sidecar once the file
// is out of it.
func removeTrashRecord(files *storage.RepositoryFS, trashID uuid.UUID) {
	if info, err := storage.TrashInfoPath(trashID); err == nil {
		_ = files.RemovePrivate(info)
	}
	if directory, err := storage.TrashFileDirectory(trashID); err == nil {
		_ = files.RemovePrivate(directory)
	}
}

// commitTrash records moved files as trashed and closes the journal in the
// same transaction.
func (s *Service) commitTrash(ctx context.Context, entry journal, moves []trashMove, trashedAt time.Time, complete bool) error {
	files := make([]scan.TrashedFile, 0, len(moves))
	assetIDs := make([]string, 0, len(moves))
	var bytes int64
	for _, move := range moves {
		files = append(files, scan.TrashedFile{
			EntryID: move.EntryID, RepositoryID: move.RepositoryID, AssetID: move.AssetID, ContentID: move.ContentID,
			Path: move.Path, Size: move.Size, MtimeNs: move.MtimeNs, TrashID: move.TrashID,
		})
		assetIDs = append(assetIDs, move.AssetID.String())
		bytes += move.Size
	}
	return s.scanner.CommitTrash(ctx, files, trashedAt, func(tx *sql.Tx, queries *repo.Queries) error {
		if err := markEventsChangedTx(ctx, tx, assetIDs, "asset_trashed"); err != nil {
			return err
		}
		if !complete {
			return nil
		}
		if err := advanceJournalTx(ctx, queries, entry.operationID, "catalog_committed", s.now()); err != nil {
			return err
		}
		return completeJournalTx(ctx, queries, entry, storage.AuditResultSucceeded,
			map[string]any{"files": len(moves), "bytes": bytes}, s.now())
	})
}
