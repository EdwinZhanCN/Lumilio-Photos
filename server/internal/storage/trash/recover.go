package trash

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"server/internal/db/repo"
	"server/internal/storage"
	"server/internal/storage/scan"
)

// errSimulatedCrash lets tests stop a Delete or Restore between a move and
// its catalog commit, as a crash would.
var errSimulatedCrash = errors.New("simulated crash")

// Recover reconciles every Delete and Restore a crash interrupted. The disk
// decides: a file found at its destination was moved and is committed as
// such; a file still at its source was not, and its trash record is removed.
// A file found under both names is one file whose move stopped between link
// and unlink, and keeps the destination name. A file is never lost and never
// left under two names. An operation whose repository is offline stays
// journaled for the next startup.
func (s *Service) Recover(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	operations, err := s.reader.ListIncompleteLifecycleOperations(ctx)
	if err != nil {
		return fmt.Errorf("list interrupted trash operations: %w", err)
	}
	var failures []error
	for _, operation := range operations {
		var err error
		switch operation.Kind {
		case storage.LifecycleKindTrashAssets:
			err = s.recoverTrash(ctx, operation)
		case storage.LifecycleKindRestoreAssets:
			err = s.recoverRestore(ctx, operation)
		default:
			continue
		}
		if err != nil {
			s.logger.Warn("trash operation left for the next recovery",
				zap.String("operation_id", operation.OperationID), zap.Error(err))
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}

func recoveredJournal(operation repo.LifecycleOperation) journal {
	request := Request{Actor: operation.Actor, RequestID: operation.RequestID}
	if operation.ActorUserID != nil {
		value := int32(*operation.ActorUserID)
		request.ActorUserID = &value
	}
	target := ""
	if operation.TargetID != nil {
		target = *operation.TargetID
	}
	return journal{operationID: operation.OperationID, requestID: operation.RequestID, kind: operation.Kind, targetID: target, request: request}
}

// openForRecovery opens the repositories an operation touched. A repository
// that no longer exists has no files to reconcile; one that is offline makes
// the operation wait.
func (s *Service) openForRecovery(ctx context.Context, ids []uuid.UUID) (repositories, error) {
	opened := repositories{}
	for _, id := range uniqueIDs(ids) {
		if _, err := s.reader.GetRepository(ctx, id); errors.Is(err, sql.ErrNoRows) {
			continue
		}
		files, err := s.openRepositories(ctx, []uuid.UUID{id})
		if err != nil {
			opened.close()
			return nil, err
		}
		opened[id] = files[id]
	}
	return opened, nil
}

// locate reports whether a move from source to destination happened.
func locate(files *storage.RepositoryFS, source, destination storage.RepositoryPath) (bool, error) {
	atDestination, err := files.Exists(destination)
	if err != nil {
		return false, err
	}
	if !atDestination {
		return false, nil
	}
	atSource, err := files.Exists(source)
	if err != nil {
		return false, err
	}
	if atSource {
		same, err := files.SameFile(destination, source)
		if err != nil {
			return false, err
		}
		if same {
			if err := files.DropDuplicateLink(destination, source); err != nil {
				return false, err
			}
		}
		// A different file at the source arrived after the move.
	}
	return true, nil
}

func (s *Service) recoverTrash(ctx context.Context, operation repo.LifecycleOperation) error {
	var payload trashPayload
	if err := json.Unmarshal([]byte(operation.Payload), &payload); err != nil {
		return fmt.Errorf("read trash journal: %w", err)
	}
	ids := make([]uuid.UUID, 0, len(payload.Moves))
	for _, move := range payload.Moves {
		ids = append(ids, move.RepositoryID)
	}
	opened, err := s.openForRecovery(ctx, ids)
	if err != nil {
		return err
	}
	defer opened.close()
	var moved []trashMove
	for _, move := range payload.Moves {
		files := opened[move.RepositoryID]
		if files == nil {
			continue
		}
		source, err := move.source()
		if err != nil {
			continue
		}
		destination, err := move.destination()
		if err != nil {
			continue
		}
		done, err := locate(files, source, destination)
		if err != nil {
			return err
		}
		if done {
			moved = append(moved, move)
		} else {
			removeTrashRecord(files, move.TrashID)
		}
	}
	entry := recoveredJournal(operation)
	files := make([]scan.TrashedFile, 0, len(moved))
	assetIDs := make([]string, 0, len(moved))
	for _, move := range moved {
		files = append(files, scan.TrashedFile{
			EntryID: move.EntryID, RepositoryID: move.RepositoryID, AssetID: move.AssetID, ContentID: move.ContentID,
			Path: move.Path, Size: move.Size, MtimeNs: move.MtimeNs, TrashID: move.TrashID,
		})
		assetIDs = append(assetIDs, move.AssetID.String())
	}
	return s.scanner.CommitTrash(ctx, files, payload.DeletedAt, func(tx *sql.Tx, queries *repo.Queries) error {
		if err := markEventsChangedTx(ctx, tx, assetIDs, "asset_trashed"); err != nil {
			return err
		}
		return completeJournalTx(ctx, queries, entry, storage.AuditResultRecovered,
			map[string]any{"files": len(moved), "not_moved": len(payload.Moves) - len(moved)}, s.now())
	})
}

func (s *Service) recoverRestore(ctx context.Context, operation repo.LifecycleOperation) error {
	var payload restorePayload
	if err := json.Unmarshal([]byte(operation.Payload), &payload); err != nil {
		return fmt.Errorf("read restore journal: %w", err)
	}
	ids := make([]uuid.UUID, 0, len(payload.Moves))
	for _, move := range payload.Moves {
		ids = append(ids, move.RepositoryID)
	}
	opened, err := s.openForRecovery(ctx, ids)
	if err != nil {
		return err
	}
	defer opened.close()
	entry := recoveredJournal(operation)
	if operation.Phase == "catalog_committed" {
		s.finishRestore(ctx, entry, opened, payload.Moves)
		return nil
	}
	var restored []scan.RestoredFile
	var moved []restoreMove
	for _, move := range payload.Moves {
		files := opened[move.RepositoryID]
		if files == nil {
			continue
		}
		source, err := move.source()
		if err != nil {
			continue
		}
		destination, err := move.destination()
		if err != nil {
			continue
		}
		done, err := locate(files, source, destination)
		if err != nil {
			return err
		}
		if !done {
			continue
		}
		observation, err := files.InspectMedia(ctx, destination, storage.HashNone)
		if err != nil {
			return err
		}
		restored = append(restored, scan.RestoredFile{
			EntryID: move.EntryID, RepositoryID: move.RepositoryID, Path: move.RestoredPath, Observation: observation,
		})
		moved = append(moved, move)
	}
	if err := s.commitRestore(ctx, entry, restored, payload.AssetIDs); err != nil {
		return err
	}
	for _, move := range moved {
		removeTrashRecord(opened[move.RepositoryID], move.TrashID)
	}
	return s.writer.WithTx(ctx, operationFor(entry.kind), func(_ *sql.Tx, queries *repo.Queries) error {
		return completeJournalTx(ctx, queries, entry, storage.AuditResultRecovered,
			map[string]any{"files": len(moved), "not_moved": len(payload.Moves) - len(moved)}, s.now())
	})
}
