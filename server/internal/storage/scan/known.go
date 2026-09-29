package scan

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"server/internal/db/catalogtx"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/storage"
)

// KnownContent is a file Lumilio committed into a repository itself, from an
// upload or a cloud import, with its content already hashed from the same
// handle it committed. Binding it needs no rehash.
type KnownContent struct {
	RepositoryID uuid.UUID
	OwnerID      int32
	RelativePath string
	AssetType    string
	MimeType     string
	FullHash     string
	// Observation is a stat of the committed file taken after the commit.
	Observation storage.FileObservation
}

// KnownBinding reports the entry and Asset a known file was bound to.
type KnownBinding struct {
	EntryID   uuid.UUID
	AssetID   uuid.UUID
	ContentID uuid.UUID
}

// BindKnownContent indexes a file Lumilio just committed, in one catalog
// transaction: its directories and entry are upserted, and the entry is
// bound to its Asset under the same rules as a scan's hash commit,
// including the in-place carry-over rule and reactivation of an Asset whose
// content was already known. It is idempotent for a repeated commit.
func (s *Scanner) BindKnownContent(ctx context.Context, known KnownContent) (KnownBinding, error) {
	var binding KnownBinding
	if known.RepositoryID == uuid.Nil || known.OwnerID <= 0 {
		return binding, errors.New("known content needs a repository and an owner")
	}
	if len(known.FullHash) != 64 || known.Observation.Size < 0 || known.AssetType == "" || known.MimeType == "" {
		return binding, errors.New("known content needs a stable identity and media type")
	}
	repositoryPath, err := storage.ParseUserMediaPath(known.RelativePath)
	if err != nil {
		return binding, err
	}
	relative := repositoryPath.String()
	fullHash := strings.ToLower(known.FullHash)
	observation := known.Observation
	observation.ContentHash = &fullHash
	now := s.config.Now()
	err = s.writer.WithTx(ctx, catalogtx.OperationRepositoryScanHashCommit, func(tx *sql.Tx, queries *repo.Queries) error {
		if err := s.ensureDirectoriesTx(ctx, queries, known.RepositoryID, parentOf(relative)); err != nil {
			return err
		}
		key, err := s.pathKey(relative)
		if err != nil {
			return err
		}
		row, err := queries.GetLiveRepositoryEntryByKey(ctx, repo.GetLiveRepositoryEntryByKeyParams{RepositoryID: known.RepositoryID, PathKey: key})
		switch {
		case errors.Is(err, sql.ErrNoRows):
			parentKey, keyErr := s.pathKey(parentOf(relative))
			if keyErr != nil {
				return keyErr
			}
			row, err = queries.InsertRepositoryEntry(ctx, repo.InsertRepositoryEntryParams{
				EntryID: uuid.New(), RepositoryID: known.RepositoryID, Path: relative, PathKey: key,
				ParentKey: parentKey, Kind: KindFile, Size: observation.Size, MtimeNs: observation.ModTimeNS,
				CtimeNs: observation.ChangeTimeNS, FileID: observation.FileIdentity, StatCheckedNs: now.UnixNano(),
				State: StatePendingHash, UpdatedAt: dbtypes.NewTimestamp(now),
			})
			if err != nil {
				return err
			}
		case err != nil:
			return err
		default:
			if row.Kind != KindFile {
				return fmt.Errorf("known content path %s is a directory in the catalog", relative)
			}
			if _, err := queries.UpdateRepositoryEntryObservedCAS(ctx, repo.UpdateRepositoryEntryObservedCASParams{
				Path: relative, Size: observation.Size, MtimeNs: observation.ModTimeNS,
				CtimeNs: observation.ChangeTimeNS, FileID: observation.FileIdentity,
				StatCheckedNs: now.UnixNano(), State: StatePendingHash, UpdatedAt: dbtypes.NewTimestamp(now),
				EntryID: row.EntryID, ExpectedRevision: row.Revision,
			}); err != nil {
				return err
			}
			if row, err = queries.GetRepositoryEntry(ctx, row.EntryID); err != nil {
				return err
			}
		}
		target, err := s.commitHashTx(ctx, tx, queries, hashCommit{
			row: row, owner: known.OwnerID, observation: observation, checked: now,
			assetType: known.AssetType, mimeType: known.MimeType, trusted: true,
		})
		if err != nil {
			return err
		}
		asset, err := queries.GetAssetByIDAny(ctx, target)
		if err != nil {
			return err
		}
		binding = KnownBinding{EntryID: row.EntryID, AssetID: target, ContentID: asset.ContentID}
		return nil
	})
	return binding, err
}

// ensureDirectoriesTx gives every ancestor of a committed file a live
// directory entry, so the walk reaches it. The rows carry no stat tuple yet;
// the next walk records one.
func (s *Scanner) ensureDirectoriesTx(ctx context.Context, queries *repo.Queries, repositoryID uuid.UUID, directory string) error {
	if directory == "" {
		return nil
	}
	if err := s.ensureDirectoriesTx(ctx, queries, repositoryID, parentOf(directory)); err != nil {
		return err
	}
	key, err := s.pathKey(directory)
	if err != nil {
		return err
	}
	if _, err := queries.GetLiveRepositoryEntryByKey(ctx, repo.GetLiveRepositoryEntryByKeyParams{RepositoryID: repositoryID, PathKey: key}); err == nil {
		return nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	parentKey, err := s.pathKey(parentOf(directory))
	if err != nil {
		return err
	}
	now := s.config.Now()
	_, err = queries.InsertRepositoryEntry(ctx, repo.InsertRepositoryEntryParams{
		EntryID: uuid.New(), RepositoryID: repositoryID, Path: directory, PathKey: key, ParentKey: parentKey,
		Kind: KindDirectory, StatCheckedNs: now.UnixNano(), State: StatePresent, UpdatedAt: dbtypes.NewTimestamp(now),
	})
	return err
}
