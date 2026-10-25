package scan

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/google/uuid"

	"server/internal/db/catalogtx"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/storage"
	"server/internal/storage/pathsemantics"
	fileutil "server/internal/utils/file"
)

// TrashedFile is one file the repository trash took, as its entry records it.
type TrashedFile struct {
	EntryID      uuid.UUID
	RepositoryID uuid.UUID
	AssetID      uuid.UUID
	ContentID    uuid.UUID
	// Path is the file's original repository-relative path.
	Path    string
	Size    int64
	MtimeNs int64
	TrashID uuid.UUID
}

// CommitTrash records files already moved into their repository trash and
// runs also in the same transaction. A scan may have seen a file vanish
// before this commit and marked its entry missing, or dropped the entry as a
// move of an Asset with another copy; the file is in the trash either way, so
// the entry becomes trashed, or a trashed entry is written for it.
func (s *Scanner) CommitTrash(ctx context.Context, files []TrashedFile, trashedAt time.Time, also func(*sql.Tx, *repo.Queries) error) error {
	return s.writer.WithTx(ctx, catalogtx.OperationAssetDelete, func(tx *sql.Tx, queries *repo.Queries) error {
		stamp := dbtypes.NewTimestamp(trashedAt)
		rules := semanticsCache{reader: queries}
		for _, file := range files {
			updated, err := queries.TrashRepositoryEntry(ctx, repo.TrashRepositoryEntryParams{
				TrashID: uuid.NullUUID{UUID: file.TrashID, Valid: true}, TrashedAt: stamp, UpdatedAt: stamp,
				EntryID: file.EntryID, AssetID: uuid.NullUUID{UUID: file.AssetID, Valid: true},
			})
			if err != nil {
				return fmt.Errorf("record trashed file %s: %w", file.Path, err)
			}
			if updated == 1 {
				continue
			}
			semantics, err := rules.get(ctx, file.RepositoryID)
			if err != nil {
				return err
			}
			key, err := pathKey(semantics, file.Path)
			if err != nil {
				return err
			}
			parentKey, err := pathKey(semantics, parentOf(file.Path))
			if err != nil {
				return err
			}
			if err := queries.InsertTrashedRepositoryEntry(ctx, repo.InsertTrashedRepositoryEntryParams{
				EntryID: uuid.New(), RepositoryID: file.RepositoryID, Path: file.Path, PathKey: key,
				ParentKey: parentKey, Size: file.Size, MtimeNs: file.MtimeNs,
				ContentID: uuid.NullUUID{UUID: file.ContentID, Valid: true},
				AssetID:   uuid.NullUUID{UUID: file.AssetID, Valid: true},
				TrashID:   uuid.NullUUID{UUID: file.TrashID, Valid: true}, TrashedAt: stamp, UpdatedAt: stamp,
			}); err != nil {
				return fmt.Errorf("record trashed file %s: %w", file.Path, err)
			}
		}
		if also != nil {
			return also(tx, queries)
		}
		return nil
	})
}

// RestoredFile is one file moved back out of the repository trash.
type RestoredFile struct {
	// EntryID is the file's trashed entry.
	EntryID      uuid.UUID
	RepositoryID uuid.UUID
	// Path is where the file now is: its original path, or a free sibling.
	Path string
	// Observation is a stat of the file at Path after the move.
	Observation storage.FileObservation
}

// CommitRestore records files already moved back out of their repository
// trash and runs also in the same transaction. Each trashed entry becomes
// present at the restored path, under a live directory entry, so its Asset
// is active again with all of its metadata. A scan that indexed a restored
// file first owns it: the trashed entry is dropped and the scan's entry
// binds to the same Asset by content.
func (s *Scanner) CommitRestore(ctx context.Context, files []RestoredFile, also func(*sql.Tx, *repo.Queries) error) error {
	return s.writer.WithTx(ctx, catalogtx.OperationAssetRestore, func(tx *sql.Tx, queries *repo.Queries) error {
		now := s.config.Now()
		rules := semanticsCache{reader: queries}
		for _, file := range files {
			semantics, err := rules.get(ctx, file.RepositoryID)
			if err != nil {
				return err
			}
			if err := s.ensureDirectoriesTx(ctx, queries, semantics, file.RepositoryID, parentOf(file.Path)); err != nil {
				return err
			}
			key, err := pathKey(semantics, file.Path)
			if err != nil {
				return err
			}
			if _, err := queries.GetLiveRepositoryEntryByKey(ctx, repo.GetLiveRepositoryEntryByKeyParams{
				RepositoryID: file.RepositoryID, PathKey: key,
			}); err == nil {
				if _, err := queries.DeleteTrashedRepositoryEntry(ctx, file.EntryID); err != nil {
					return err
				}
				continue
			} else if !errors.Is(err, sql.ErrNoRows) {
				return err
			}
			parentKey, err := pathKey(semantics, parentOf(file.Path))
			if err != nil {
				return err
			}
			// A restore keeps the file's mtime, so the tuple is racily clean
			// only if the file was written in the last moments before it was
			// trashed; recording now as the check time is Git's rule.
			restored, err := queries.RestoreRepositoryEntry(ctx, repo.RestoreRepositoryEntryParams{
				Path: file.Path, PathKey: key, ParentKey: parentKey,
				Size: file.Observation.Size, MtimeNs: file.Observation.ModTimeNS,
				CtimeNs: file.Observation.ChangeTimeNS, FileID: file.Observation.FileIdentity,
				StatCheckedNs: now.UnixNano(), UpdatedAt: dbtypes.NewTimestamp(now), EntryID: file.EntryID,
			})
			if err != nil {
				return fmt.Errorf("record restored file %s: %w", file.Path, err)
			}
			if restored == 0 || s.config.Activate == nil {
				continue
			}
			row, err := queries.GetRepositoryEntry(ctx, file.EntryID)
			if err != nil {
				return err
			}
			if err := s.config.Activate(ctx, tx, queries, row.RepositoryID, row.EntryID, row.AssetID.UUID, row.ContentID.UUID); err != nil {
				return err
			}
		}
		if also != nil {
			return also(tx, queries)
		}
		return nil
	})
}

// SameTuple reports whether a file still has the stat tuple its entry
// recorded, under the rule the walk uses to skip unchanged files.
func SameTuple(row repo.RepositoryEntry, observation storage.FileObservation) bool {
	return sameTuple(row, observation)
}

// TrashRecord is a trashed file described by its info sidecar, for
// rebuilding the Trash of a catalog that does not know it: after the catalog
// was rebuilt from disk, or a repository was opened into a new catalog.
type TrashRecord struct {
	TrashID       uuid.UUID
	OriginalPath  string
	HashAlgorithm string
	ContentHash   string
	Size          int64
	MtimeNs       int64
	DeletedAt     time.Time
	// Held keeps the adopted file out of automatic expiry until the user
	// decides: a sidecar this catalog never wrote is inherited Trash.
	Held bool
}

// TrashHoldInherited marks a trashed entry the Trash rebuild adopted from a
// sidecar this catalog did not write.
const TrashHoldInherited = "inherited"

// AdoptTrash records trashed files the catalog does not know, in one
// transaction. Each binds to the owner's Asset for its content, or to a new
// trashed Asset, so it is listed in the Trash and can be restored or expire.
// A missing row of that Asset at the same path is superseded by the trashed
// row. Records whose trash ID is already known are skipped. It returns how
// many records it adopted.
func (s *Scanner) AdoptTrash(ctx context.Context, repositoryID uuid.UUID, owner int32, records []TrashRecord) (int, error) {
	adopted := 0
	err := s.writer.WithTx(ctx, catalogtx.OperationAssetDelete, func(_ *sql.Tx, queries *repo.Queries) error {
		adopted = 0
		semantics, err := repositorySemantics(ctx, queries, repositoryID)
		if err != nil {
			return err
		}
		for _, record := range records {
			known, err := queries.CountRepositoryEntriesWithTrashID(ctx, uuid.NullUUID{UUID: record.TrashID, Valid: true})
			if err != nil {
				return err
			}
			if known > 0 {
				continue
			}
			validation := fileutil.ValidateFile(path.Base(record.OriginalPath), "")
			if !validation.Valid {
				continue
			}
			stamp := dbtypes.NewTimestamp(record.DeletedAt)
			content, err := queries.InsertContentObject(ctx, repo.InsertContentObjectParams{
				ContentID: uuid.New(), HashAlgorithm: record.HashAlgorithm, FullHash: record.ContentHash,
				FileSize: record.Size, CreatedAt: stamp,
			})
			if err != nil {
				return fmt.Errorf("record trashed content: %w", err)
			}
			assetID := uuid.Nil
			existing, err := queries.GetOwnerContentAsset(ctx, repo.GetOwnerContentAssetParams{OwnerID: &owner, ContentID: content.ContentID})
			switch {
			case err == nil:
				assetID = existing.AssetID
			case errors.Is(err, sql.ErrNoRows):
				created, err := queries.InsertOwnerContentAsset(ctx, repo.InsertOwnerContentAssetParams{
					AssetID: uuid.New(), OwnerID: &owner, ContentID: content.ContentID,
					Type: string(validation.AssetType), OriginalFilename: path.Base(record.OriginalPath),
					MimeType: validation.MimeType, UploadTime: stamp,
					TakenTime: dbtypes.NewTimestamp(time.Unix(0, record.MtimeNs)),
					Rating:    new(int64), Status: dbtypes.JSON(`{"state":"processing","message":"Pending processing"}`),
					UpdatedAt: stamp,
				})
				if err != nil {
					return fmt.Errorf("record trashed Asset: %w", err)
				}
				assetID = created.AssetID
				// The library and the Trash view list logical media items,
				// which activation creates for an Asset bound to a present
				// file. A trashed Asset the catalog never saw present needs
				// its media item here, or the Trash view would not show it.
				mediaItemID := uuid.New()
				if err := queries.CreateMediaItemForAsset(ctx, repo.CreateMediaItemForAssetParams{
					MediaItemID: mediaItemID, OwnerID: &owner,
					RepositoryID: uuid.NullUUID{UUID: repositoryID, Valid: true},
					MediaKind:    strings.ToLower(created.Type),
					AssetID:      uuid.NullUUID{UUID: assetID, Valid: true}, CreatedAt: stamp,
				}); err != nil {
					return fmt.Errorf("record trashed media item: %w", err)
				}
				if err := queries.AttachAssetToMediaItem(ctx, repo.AttachAssetToMediaItemParams{
					AssetID: assetID, MediaItemID: mediaItemID, Relation: "original", CreatedAt: stamp,
				}); err != nil {
					return fmt.Errorf("attach trashed Asset to its media item: %w", err)
				}
			default:
				return err
			}
			key, err := pathKey(semantics, record.OriginalPath)
			if err != nil {
				return err
			}
			parentKey, err := pathKey(semantics, parentOf(record.OriginalPath))
			if err != nil {
				return err
			}
			var hold *string
			if record.Held {
				held := TrashHoldInherited
				hold = &held
			}
			if err := queries.InsertTrashedRepositoryEntry(ctx, repo.InsertTrashedRepositoryEntryParams{
				EntryID: uuid.New(), RepositoryID: repositoryID, Path: record.OriginalPath, PathKey: key,
				ParentKey: parentKey, Size: record.Size, MtimeNs: record.MtimeNs,
				ContentID: uuid.NullUUID{UUID: content.ContentID, Valid: true},
				AssetID:   uuid.NullUUID{UUID: assetID, Valid: true},
				TrashID:   uuid.NullUUID{UUID: record.TrashID, Valid: true}, TrashedAt: stamp, TrashHold: hold, UpdatedAt: stamp,
			}); err != nil {
				return fmt.Errorf("record trashed file %s: %w", record.OriginalPath, err)
			}
			// A scan that ran before this rebuild saw the file gone and
			// marked its row missing (a catalog restored from a backup
			// taken before the Delete); the trashed row supersedes it.
			if _, err := queries.DeleteMissingRepositoryEntriesAtTrashedPath(ctx, repo.DeleteMissingRepositoryEntriesAtTrashedPathParams{
				AssetID: uuid.NullUUID{UUID: assetID, Valid: true}, RepositoryID: repositoryID, PathKey: key,
			}); err != nil {
				return err
			}
			adopted++
		}
		return nil
	})
	return adopted, err
}

// semanticsCache reads each repository's volume rule once per transaction
// body, for commits that span repositories.
type semanticsCache struct {
	reader repositoryReader
	rules  map[uuid.UUID]pathsemantics.Semantics
}

func (c *semanticsCache) get(ctx context.Context, repositoryID uuid.UUID) (pathsemantics.Semantics, error) {
	if rule, ok := c.rules[repositoryID]; ok {
		return rule, nil
	}
	rule, err := repositorySemantics(ctx, c.reader, repositoryID)
	if err != nil {
		return pathsemantics.Semantics{}, err
	}
	if c.rules == nil {
		c.rules = map[uuid.UUID]pathsemantics.Semantics{}
	}
	c.rules[repositoryID] = rule
	return rule, nil
}
