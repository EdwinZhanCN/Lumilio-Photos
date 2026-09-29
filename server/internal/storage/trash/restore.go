package trash

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"server/internal/db/repo"
	"server/internal/storage"
	"server/internal/storage/scan"
)

// RestoreResult is what one Restore moved back.
type RestoreResult struct {
	OperationID string
	Files       int
	// Renamed lists files restored beside their original path because that
	// path was taken.
	Renamed []RenamedFile
}

// RenamedFile is a file restored under a new name.
type RenamedFile struct {
	AssetID      uuid.UUID
	OriginalPath string
	RestoredPath string
}

type restoreMove struct {
	EntryID      uuid.UUID `json:"entry_id"`
	RepositoryID uuid.UUID `json:"repository_id"`
	AssetID      uuid.UUID `json:"asset_id"`
	TrashID      uuid.UUID `json:"trash_id"`
	OriginalPath string    `json:"original_path"`
	RestoredPath string    `json:"restored_path"`
}

type restorePayload struct {
	AssetIDs []uuid.UUID   `json:"asset_ids"`
	Moves    []restoreMove `json:"moves"`
}

func (m restoreMove) source() (storage.RepositoryPath, error) {
	return storage.TrashFilePath(m.TrashID, path.Base(m.OriginalPath))
}
func (m restoreMove) destination() (storage.RepositoryPath, error) {
	return storage.ParseUserMediaPath(m.RestoredPath)
}

// Restore moves every trashed file of the selected Assets back to its
// original path and makes the Assets active again with their metadata. It
// never overwrites: a taken path gets a free sibling name, which the result
// reports. It refuses the whole request, moving nothing, when a repository is
// not reachable or a trashed file is gone.
func (s *Service) Restore(ctx context.Context, request Request) (RestoreResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var result RestoreResult
	assetIDs := uniqueIDs(request.AssetIDs)
	if len(assetIDs) == 0 {
		return result, nil
	}
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

	payload := restorePayload{AssetIDs: assetIDs}
	reserved := map[uuid.UUID]map[string]bool{}
	for _, entry := range entries {
		move := restoreMove{
			EntryID: entry.EntryID, RepositoryID: entry.RepositoryID, AssetID: entry.AssetID.UUID,
			TrashID: entry.TrashID.UUID, OriginalPath: entry.Path,
		}
		files := opened[entry.RepositoryID]
		source, err := move.source()
		if err != nil {
			return result, &Rejection{Reason: ReasonTrashFileMissing, AssetID: move.AssetID, RepositoryID: move.RepositoryID, Path: entry.Path, Cause: err}
		}
		if exists, err := files.Exists(source); err != nil || !exists {
			if err == nil {
				err = errors.New("the trashed file is no longer in the repository trash")
			}
			return result, &Rejection{Reason: ReasonTrashFileMissing, AssetID: move.AssetID, RepositoryID: move.RepositoryID, Path: entry.Path, Cause: err}
		}
		if reserved[entry.RepositoryID] == nil {
			reserved[entry.RepositoryID] = map[string]bool{}
		}
		restored, err := freePath(files, entry.Path, reserved[entry.RepositoryID])
		if err != nil {
			return result, &Rejection{Reason: ReasonMoveFailed, AssetID: move.AssetID, RepositoryID: move.RepositoryID, Path: entry.Path, Cause: err}
		}
		move.RestoredPath = restored
		payload.Moves = append(payload.Moves, move)
	}

	entry, err := s.beginJournal(ctx, storage.LifecycleKindRestoreAssets, request, assetIDs[0], payload)
	if err != nil {
		return result, err
	}
	restored := make([]scan.RestoredFile, 0, len(payload.Moves))
	for moved, move := range payload.Moves {
		file, err := s.moveOutOfTrash(ctx, opened[move.RepositoryID], move)
		if err == nil && s.afterMove != nil {
			err = s.afterMove(moved + 1)
			if errors.Is(err, errSimulatedCrash) {
				return result, err
			}
		}
		if err != nil {
			s.rollbackRestore(opened, payload.Moves[:moved+1])
			s.failJournal(ctx, entry, "filesystem", err)
			return result, &Rejection{Reason: ReasonMoveFailed, AssetID: move.AssetID, RepositoryID: move.RepositoryID, Path: move.OriginalPath, Cause: err}
		}
		restored = append(restored, file)
	}
	if err := s.advanceJournal(ctx, entry, "filesystem_applied"); err != nil {
		return result, err
	}
	if err := s.commitRestore(ctx, entry, restored, assetIDs); err != nil {
		return result, err
	}
	s.finishRestore(ctx, entry, opened, payload.Moves)

	result.OperationID = entry.operationID
	result.Files = len(payload.Moves)
	for _, move := range payload.Moves {
		if move.RestoredPath != move.OriginalPath {
			result.Renamed = append(result.Renamed, RenamedFile{
				AssetID: move.AssetID, OriginalPath: move.OriginalPath, RestoredPath: move.RestoredPath,
			})
		}
	}
	return result, nil
}

// freePath is the original path when it is free, or else the first free
// "name (restored).ext", "name (restored 2).ext", ... beside it.
func freePath(files *storage.RepositoryFS, original string, reserved map[string]bool) (string, error) {
	directory, name := path.Split(original)
	extension := path.Ext(name)
	stem := strings.TrimSuffix(name, extension)
	for attempt := 0; attempt < 1000; attempt++ {
		candidate := original
		switch attempt {
		case 0:
		case 1:
			candidate = directory + stem + " (restored)" + extension
		default:
			candidate = fmt.Sprintf("%s%s (restored %d)%s", directory, stem, attempt, extension)
		}
		parsed, err := storage.ParseUserMediaPath(candidate)
		if err != nil {
			return "", err
		}
		if reserved[strings.ToLower(parsed.String())] {
			continue
		}
		exists, err := files.Exists(parsed)
		if err != nil {
			return "", err
		}
		if !exists {
			reserved[strings.ToLower(parsed.String())] = true
			return parsed.String(), nil
		}
	}
	return "", fmt.Errorf("no free name beside %s", original)
}

func (s *Service) moveOutOfTrash(ctx context.Context, files *storage.RepositoryFS, move restoreMove) (scan.RestoredFile, error) {
	source, err := move.source()
	if err != nil {
		return scan.RestoredFile{}, err
	}
	destination, err := move.destination()
	if err != nil {
		return scan.RestoredFile{}, err
	}
	if parent := path.Dir(move.RestoredPath); parent != "." {
		directory, err := storage.ParseUserMediaPath(parent)
		if err != nil {
			return scan.RestoredFile{}, err
		}
		if err := files.MkdirAllMedia(directory, 0o755); err != nil {
			return scan.RestoredFile{}, err
		}
	}
	if err := files.MoveNoReplace(source, destination); err != nil {
		return scan.RestoredFile{}, err
	}
	observation, err := files.InspectMedia(ctx, destination, storage.HashNone)
	if err != nil {
		return scan.RestoredFile{}, err
	}
	return scan.RestoredFile{EntryID: move.EntryID, RepositoryID: move.RepositoryID, Path: move.RestoredPath, Observation: observation}, nil
}

// rollbackRestore puts restored files back into the trash, newest first.
func (s *Service) rollbackRestore(opened repositories, moves []restoreMove) {
	for index := len(moves) - 1; index >= 0; index-- {
		move := moves[index]
		files := opened[move.RepositoryID]
		source, _ := move.source()
		destination, err := move.destination()
		if err != nil {
			continue
		}
		if inTrash, err := files.Exists(source); err != nil || inTrash {
			continue
		}
		if err := files.MoveNoReplace(destination, source); err != nil {
			s.logger.Error("restored file could not be put back into the trash",
				zap.String("path", move.RestoredPath), zap.Error(err))
		}
	}
}

func (s *Service) commitRestore(ctx context.Context, entry journal, files []scan.RestoredFile, assetIDs []uuid.UUID) error {
	ids := make([]string, 0, len(assetIDs))
	for _, id := range assetIDs {
		ids = append(ids, id.String())
	}
	return s.scanner.CommitRestore(ctx, files, func(tx *sql.Tx, queries *repo.Queries) error {
		if err := markEventsChangedTx(ctx, tx, ids, "asset_restored"); err != nil {
			return err
		}
		return advanceJournalTx(ctx, queries, entry.operationID, "catalog_committed", s.now())
	})
}

// finishRestore removes the emptied trash directories and sidecars, then
// closes the journal. Recovery repeats it after a crash.
func (s *Service) finishRestore(ctx context.Context, entry journal, opened repositories, moves []restoreMove) {
	for _, move := range moves {
		if files := opened[move.RepositoryID]; files != nil {
			removeTrashRecord(files, move.TrashID)
		}
	}
	err := s.writer.WithTx(ctx, operationFor(entry.kind), func(_ *sql.Tx, queries *repo.Queries) error {
		return completeJournalTx(ctx, queries, entry, storage.AuditResultSucceeded,
			map[string]any{"files": len(moves)}, s.now())
	})
	if err != nil {
		s.logger.Error("close restore journal", zap.String("operation_id", entry.operationID), zap.Error(err))
	}
}
