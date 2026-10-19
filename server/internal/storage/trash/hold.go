package trash

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"server/internal/db/catalogtx"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/storage"
)

// HeldAssetIDs lists the Assets with at least one held trashed file, in one
// repository (or all) and one owner scope (or all). The Trash view marks
// them and offers the explicit decisions.
func (s *Service) HeldAssetIDs(ctx context.Context, repositoryID uuid.NullUUID, ownerID *int32) ([]uuid.UUID, error) {
	rows, err := s.reader.ListHeldTrashedAssetIDs(ctx, repo.ListHeldTrashedAssetIDsParams{RepositoryID: repositoryID, OwnerID: ownerID})
	if err != nil {
		return nil, fmt.Errorf("list held trashed Assets: %w", err)
	}
	ids := make([]uuid.UUID, 0, len(rows))
	for _, row := range rows {
		if row.Valid {
			ids = append(ids, row.UUID)
		}
	}
	return ids, nil
}

// AcceptRetentionResult is what one acceptance released.
type AcceptRetentionResult struct {
	Entries int64
}

// AcceptRetention lifts the hold on inherited trashed files: the selected
// Assets' files, or every held file of one repository in the caller's owner
// scope. The files then follow the configured retention counted from this
// acceptance; nothing is unlinked here. Restore and DeletePermanently are the
// other ways out of a hold.
func (s *Service) AcceptRetention(ctx context.Context, request Request, repositoryID uuid.NullUUID, ownerID *int32) (AcceptRetentionResult, error) {
	var result AcceptRetentionResult
	assetIDs := uniqueIDs(request.AssetIDs)
	if len(assetIDs) == 0 && !repositoryID.Valid {
		return result, errors.New("retention acceptance needs a selection or a repository")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := dbtypes.NewTimestamp(s.now())
	err := s.writer.WithTx(ctx, catalogtx.OperationRepositoryRecovery, func(_ *sql.Tx, queries *repo.Queries) error {
		var released int64
		var err error
		if len(assetIDs) > 0 {
			entries, err := queries.ListHeldTrashedRepositoryEntriesForAssets(ctx, nullIDs(assetIDs))
			if err != nil {
				return err
			}
			for _, entry := range entries {
				count, err := queries.ReleaseTrashHoldForEntry(ctx, repo.ReleaseTrashHoldForEntryParams{
					TrashedAt: now, UpdatedAt: now, EntryID: entry.EntryID,
				})
				if err != nil {
					return err
				}
				released += count
			}
		} else {
			released, err = queries.ReleaseTrashHoldForRepository(ctx, repo.ReleaseTrashHoldForRepositoryParams{
				TrashedAt: now, UpdatedAt: now, RepositoryID: repositoryID.UUID, OwnerID: ownerID,
			})
		}
		if err != nil {
			return err
		}
		result.Entries = released
		if released == 0 {
			return nil
		}
		actor := request.Actor
		if actor == "" {
			actor = "server"
		}
		details := map[string]any{"entries": released}
		targetType, targetID := "asset", ""
		if len(assetIDs) > 0 {
			targetID = assetIDs[0].String()
			details["assets"] = len(assetIDs)
		} else {
			targetType, targetID = "repository", repositoryID.UUID.String()
		}
		_, err = storage.RecordLifecycleAuditTx(ctx, queries, storage.LifecycleAuditInput{
			Actor: actor, ActorUserID: request.ActorUserID, RequestID: request.RequestID,
			Action: "trash_hold_released", TargetType: targetType, TargetID: targetID,
			ConfirmationType: request.ConfirmationType, Result: storage.AuditResultSucceeded, Details: details,
		})
		return err
	})
	return result, err
}
