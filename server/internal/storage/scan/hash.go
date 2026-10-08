package scan

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"time"

	"github.com/google/uuid"

	"server/internal/db/catalogtx"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/storage"
	fileutil "server/internal/utils/file"
)

// HashResult reports one bounded hash pass over a repository's pending rows.
type HashResult struct {
	Bound    int
	Deferred int
	Errors   int
	// HasMore is true when pending rows remain after this pass.
	HasMore bool
}

// errStale aborts a hash commit whose row moved on after it was read; the
// next pass sees the row's current state.
var errStale = errors.New("repository entry changed since it was read")

// HashTurn hashes up to limit pending_hash rows of one repository. Reading
// and hashing happen outside any transaction; each commit is a
// compare-and-swap on the row's revision.
func (s *Scanner) HashTurn(ctx context.Context, repositoryID uuid.UUID, limit int) (HashResult, error) {
	var result HashResult
	if limit <= 0 || limit > MaxBatchRows {
		limit = MaxBatchRows
	}
	rows, err := s.reader.ListPendingHashRepositoryEntries(ctx, repo.ListPendingHashRepositoryEntriesParams{
		RepositoryID: repositoryID, Limit: int64(limit),
	})
	if err != nil || len(rows) == 0 {
		return result, err
	}
	result.HasMore = len(rows) == limit
	repository, err := s.reader.GetRepository(ctx, repositoryID)
	if err != nil {
		return result, err
	}
	if repository.DefaultOwnerID == nil || *repository.DefaultOwnerID <= 0 {
		return result, fmt.Errorf("repository %s has no owner for scanned files", repositoryID)
	}
	owner := *repository.DefaultOwnerID
	fsys, err := s.files.OpenContext(ctx, repository)
	if err != nil {
		if isOffline(err) {
			// Pending rows wait for the repository to come back.
			return HashResult{}, nil
		}
		return result, err
	}
	defer fsys.Close()
	for _, row := range rows {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		repositoryPath, err := storage.ParseUserMediaPath(row.Path)
		if err != nil {
			result.Errors++
			continue
		}
		checked := s.config.Now()
		observation, err := fsys.InspectMedia(ctx, repositoryPath, storage.HashQuickAndFull)
		switch {
		case errors.Is(err, storage.ErrRepositoryFileUnstable):
			// Changed while hashing: the row stays pending for the next pass.
			result.Deferred++
			continue
		case errors.Is(err, storage.ErrRepositoryEntryUnsupported):
			if err := s.markUnsupported(ctx, row, checked); err != nil && !errors.Is(err, errStale) {
				return result, err
			}
			continue
		case errors.Is(err, fs.ErrNotExist):
			// Gone before it was hashed; the next walk proves the absence.
			result.Deferred++
			continue
		case err != nil:
			result.Errors++
			continue
		}
		if observation.ContentHash == nil {
			return result, errors.New("full hash did not produce a content hash")
		}
		validation := fileutil.ValidateFile(path.Base(row.Path), "")
		if !validation.Valid {
			if err := s.markUnsupported(ctx, row, checked); err != nil && !errors.Is(err, errStale) {
				return result, err
			}
			continue
		}
		commit := hashCommit{
			row: row, owner: owner, observation: observation, checked: checked,
			assetType: string(validation.AssetType), mimeType: validation.MimeType,
		}
		err = s.writer.WithTx(ctx, catalogtx.OperationRepositoryScanHashCommit, func(tx *sql.Tx, queries *repo.Queries) error {
			_, err := s.commitHashTx(ctx, tx, queries, commit)
			return err
		})
		switch {
		case errors.Is(err, errStale):
			result.Deferred++
		case err != nil:
			return result, err
		default:
			result.Bound++
		}
	}
	return result, nil
}

func (s *Scanner) markUnsupported(ctx context.Context, row repo.RepositoryEntry, checked time.Time) error {
	return s.writer.WithTx(ctx, catalogtx.OperationRepositoryScanHashCommit, func(_ *sql.Tx, queries *repo.Queries) error {
		rows, err := queries.SetRepositoryEntryStateCAS(ctx, repo.SetRepositoryEntryStateCASParams{
			State: StateUnsupported, Size: row.Size, MtimeNs: row.MtimeNs, CtimeNs: row.CtimeNs,
			FileID: row.FileID, StatCheckedNs: checked.UnixNano(), UpdatedAt: dbtypes.NewTimestamp(checked),
			EntryID: row.EntryID, ExpectedRevision: row.Revision,
		})
		if err == nil && rows != 1 {
			return errStale
		}
		return err
	})
}

type hashCommit struct {
	row         repo.RepositoryEntry
	owner       int32
	observation storage.FileObservation
	checked     time.Time
	assetType   string
	mimeType    string
	// trusted marks a file Lumilio wrote itself (upload or cloud import):
	// no other writer can race its mtime, so it is recorded as checked past
	// the racy window and the next scan does not rehash it.
	trusted bool
}

func (commit hashCommit) checkedNs() int64 {
	checked := commit.checked.UnixNano()
	if commit.trusted {
		return max(checked, commit.observation.ModTimeNS+RacyGranularity.Nanoseconds()+1)
	}
	return checked
}

// commitHashTx binds a hashed entry to its Asset. When the path was bound to
// Asset A and now holds new content C2, it applies #223's in-place
// carry-over rule:
//  1. no Asset for (owner, C2) and A has no other present entry: A is
//     re-pointed to C2 and keeps its ID and metadata;
//  2. no Asset for (owner, C2) but A has another present entry: a new Asset
//     for C2 inherits a copy of A's user metadata;
//  3. an Asset X for (owner, C2) exists: the entry binds to X with no merge,
//     and A keeps the old binding as a missing entry if it has no other
//     present entry.
//
// In every branch the scan leaves no Asset without an entry.
func (s *Scanner) commitHashTx(ctx context.Context, tx *sql.Tx, queries *repo.Queries, commit hashCommit) (uuid.UUID, error) {
	row := commit.row
	current, err := queries.GetRepositoryEntry(ctx, row.EntryID)
	if errors.Is(err, sql.ErrNoRows) {
		return uuid.Nil, errStale
	}
	if err != nil {
		return uuid.Nil, err
	}
	if current.Revision != row.Revision || current.State != StatePendingHash {
		return uuid.Nil, errStale
	}
	now := dbtypes.NewTimestamp(commit.checked)
	content, err := queries.InsertContentObject(ctx, repo.InsertContentObjectParams{
		ContentID: uuid.New(), HashAlgorithm: "blake3-v1", FullHash: *commit.observation.ContentHash,
		FileSize: commit.observation.Size, CreatedAt: now,
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("insert content identity: %w", err)
	}
	existing, err := queries.GetOwnerContentAsset(ctx, repo.GetOwnerContentAssetParams{OwnerID: &commit.owner, ContentID: content.ContentID})
	hasExisting := err == nil
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return uuid.Nil, err
	}
	newAsset := func() (uuid.UUID, error) {
		asset, err := queries.InsertOwnerContentAsset(ctx, repo.InsertOwnerContentAssetParams{
			AssetID: uuid.New(), OwnerID: &commit.owner, ContentID: content.ContentID,
			Type: commit.assetType, OriginalFilename: path.Base(row.Path), MimeType: commit.mimeType,
			UploadTime: now, TakenTime: dbtypes.NewTimestamp(time.Unix(0, commit.observation.ModTimeNS)),
			Rating: new(int64), Status: dbtypes.JSON(`{"state":"processing","message":"Pending processing"}`),
			UpdatedAt: now,
		})
		return asset.AssetID, err
	}

	var target uuid.UUID
	var previous repo.Asset
	bound := row.AssetID.Valid
	if bound {
		previous, err = queries.GetAssetByIDAny(ctx, row.AssetID.UUID)
		if err != nil {
			return uuid.Nil, fmt.Errorf("load bound asset: %w", err)
		}
	}
	switch {
	case bound && previous.ContentID == content.ContentID:
		target = previous.AssetID
	case hasExisting:
		target = existing.AssetID
		if bound {
			if err := keepAsMissingTx(ctx, queries, row, previous, now); err != nil {
				return uuid.Nil, err
			}
		}
	case bound:
		elsewhere, err := queries.HasOtherPresentRepositoryEntry(ctx, repo.HasOtherPresentRepositoryEntryParams{
			AssetID: row.AssetID, EntryID: row.EntryID,
		})
		if err != nil {
			return uuid.Nil, err
		}
		if elsewhere == 0 {
			rows, err := queries.RepointAssetContent(ctx, repo.RepointAssetContentParams{
				ContentID: content.ContentID, UpdatedAt: now,
				AssetID: previous.AssetID, ExpectedContentID: previous.ContentID,
			})
			if err != nil {
				return uuid.Nil, fmt.Errorf("re-point asset content: %w", err)
			}
			if rows != 1 {
				return uuid.Nil, errStale
			}
			target = previous.AssetID
			break
		}
		if target, err = newAsset(); err != nil {
			return uuid.Nil, err
		}
		if err := copyUserMetadataTx(ctx, queries, previous.AssetID, target); err != nil {
			return uuid.Nil, err
		}
	default:
		if target, err = newAsset(); err != nil {
			return uuid.Nil, err
		}
	}

	rows, err := queries.BindRepositoryEntryCAS(ctx, repo.BindRepositoryEntryCASParams{
		ContentID: uuid.NullUUID{UUID: content.ContentID, Valid: true},
		AssetID:   uuid.NullUUID{UUID: target, Valid: true},
		Size:      commit.observation.Size, MtimeNs: commit.observation.ModTimeNS,
		CtimeNs: commit.observation.ChangeTimeNS, FileID: commit.observation.FileIdentity,
		QuickFingerprint: commit.observation.QuickFingerprint, QuickFingerprintVersion: commit.observation.QuickFingerprintVer,
		StatCheckedNs: commit.checkedNs(), UpdatedAt: now,
		EntryID: row.EntryID, ExpectedRevision: row.Revision,
	})
	if err != nil {
		return uuid.Nil, err
	}
	if rows != 1 {
		return uuid.Nil, errStale
	}
	moved, err := queries.DeleteMissingRepositoryEntriesForAsset(ctx, uuid.NullUUID{UUID: target, Valid: true})
	if err != nil {
		return uuid.Nil, err
	}
	if s.config.Activate != nil {
		if err := s.config.Activate(ctx, tx, queries, row.RepositoryID, row.EntryID, target, content.ContentID); err != nil {
			return uuid.Nil, err
		}
	}
	scanID, err := queries.GetLatestStartedRepositoryScanID(ctx, row.RepositoryID)
	if errors.Is(err, sql.ErrNoRows) {
		return target, nil
	}
	if err != nil {
		return uuid.Nil, err
	}
	return target, queries.AddRepositoryScanHashProgress(ctx, repo.AddRepositoryScanHashProgressParams{
		HashedBytes: commit.observation.Size, MovedEntries: moved, UpdatedAt: now, ScanID: scanID,
	})
}

// keepAsMissingTx preserves an Asset whose only present file now holds other
// content: the old binding survives as a missing entry, so the Asset shows as
// missing with its metadata until the user removes it.
func keepAsMissingTx(ctx context.Context, queries *repo.Queries, row repo.RepositoryEntry, previous repo.Asset, now dbtypes.Timestamp) error {
	elsewhere, err := queries.HasOtherPresentRepositoryEntry(ctx, repo.HasOtherPresentRepositoryEntryParams{
		AssetID: row.AssetID, EntryID: row.EntryID,
	})
	if err != nil || elsewhere == 1 {
		return err
	}
	_, err = queries.InsertRepositoryEntry(ctx, repo.InsertRepositoryEntryParams{
		EntryID: uuid.New(), RepositoryID: row.RepositoryID, Path: row.Path, PathKey: row.PathKey,
		ParentKey: row.ParentKey, Kind: KindFile, Size: row.Size, MtimeNs: row.MtimeNs,
		CtimeNs: row.CtimeNs, FileID: row.FileID, StatCheckedNs: row.StatCheckedNs, State: StateMissing,
		ContentID:    uuid.NullUUID{UUID: previous.ContentID, Valid: true},
		AssetID:      uuid.NullUUID{UUID: previous.AssetID, Valid: true},
		MissingSince: now, UpdatedAt: now,
	})
	return err
}

func copyUserMetadataTx(ctx context.Context, queries *repo.Queries, source, target uuid.UUID) error {
	if err := queries.CopyAssetUserMetadata(ctx, repo.CopyAssetUserMetadataParams{SourceAssetID: source, TargetAssetID: target}); err != nil {
		return fmt.Errorf("copy user metadata: %w", err)
	}
	if err := queries.CopyAssetAlbumMemberships(ctx, repo.CopyAssetAlbumMembershipsParams{SourceAssetID: source, TargetAssetID: target}); err != nil {
		return fmt.Errorf("copy album memberships: %w", err)
	}
	if err := queries.CopyAssetUserTags(ctx, repo.CopyAssetUserTagsParams{SourceAssetID: source, TargetAssetID: target}); err != nil {
		return fmt.Errorf("copy user tags: %w", err)
	}
	return nil
}
