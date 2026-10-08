package testutil

import (
	"context"
	"database/sql"
	"encoding/hex"
	"fmt"

	"github.com/google/uuid"
)

// SQLExecutor is implemented by both *sql.DB and *sql.Tx.
type SQLExecutor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

// AssetOccurrenceParams describes one normalized catalog Asset and its one
// file entry. The user and repository rows must already exist.
type AssetOccurrenceParams struct {
	AssetID      uuid.UUID
	RepositoryID uuid.UUID
	OwnerID      int32
	AssetType    string
	Filename     string
	NodeName     string
	MIMEType     string
	FileSize     int64
	UploadTime   int64
	TakenTime    *int64
	// EntryState is the entry's state: present (the default), missing, or
	// trashed. The Asset's lifecycle state is derived from it.
	EntryState string
	Status     string
	ContentID  uuid.UUID
	FullHash   string
}

type AssetOccurrence struct {
	ContentID uuid.UUID
	EntryID   uuid.UUID
}

// InsertAssetOccurrence seeds the owner/content/Asset contract with one file
// entry at the repository root, which is what the active occurrence
// projection and the Asset's lifecycle state read.
func InsertAssetOccurrence(
	ctx context.Context,
	database SQLExecutor,
	params AssetOccurrenceParams,
) (AssetOccurrence, error) {
	if params.AssetID == uuid.Nil || params.RepositoryID == uuid.Nil || params.OwnerID <= 0 {
		return AssetOccurrence{}, fmt.Errorf("asset occurrence requires asset, repository, and owner identities")
	}
	if params.AssetType == "" {
		params.AssetType = "PHOTO"
	}
	if params.Filename == "" {
		params.Filename = params.AssetID.String()
	}
	if params.NodeName == "" {
		params.NodeName = params.AssetID.String() + "-" + params.Filename
	}
	if params.MIMEType == "" {
		params.MIMEType = "application/octet-stream"
	}
	if params.FileSize < 0 {
		return AssetOccurrence{}, fmt.Errorf("asset occurrence file size must be non-negative")
	}
	if params.UploadTime == 0 {
		params.UploadTime = 1
	}
	if params.Status == "" {
		params.Status = `{"state":"completed"}`
	}
	if params.EntryState == "" {
		params.EntryState = "present"
	}
	var missingSince, trashID, trashedAt any
	switch params.EntryState {
	case "present":
	case "missing":
		missingSince = int64(1)
	case "trashed":
		trashID, trashedAt = uuid.NewString(), int64(1)
	default:
		return AssetOccurrence{}, fmt.Errorf("unsupported fixture entry state %q", params.EntryState)
	}
	if params.ContentID == uuid.Nil {
		params.ContentID = uuid.New()
	}
	if params.FullHash == "" {
		encoded := hex.EncodeToString(params.ContentID[:])
		params.FullHash = encoded + encoded
	}

	if _, err := database.ExecContext(ctx, `
		INSERT INTO content_objects (
			content_id, hash_algorithm, full_hash, file_size, created_at
		) VALUES (?, 'blake3-v1', ?, ?, 1)
	`, params.ContentID, params.FullHash, params.FileSize); err != nil {
		return AssetOccurrence{}, fmt.Errorf("insert fixture content object: %w", err)
	}
	if _, err := database.ExecContext(ctx, `
		INSERT INTO assets (
			asset_id, owner_id, content_id, type, original_filename, mime_type,
			upload_time, taken_time, status, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 1)
	`, params.AssetID, params.OwnerID, params.ContentID, params.AssetType,
		params.Filename, params.MIMEType, params.UploadTime, params.TakenTime,
		params.Status); err != nil {
		return AssetOccurrence{}, fmt.Errorf("insert fixture asset: %w", err)
	}

	entryID := uuid.New()
	if _, err := database.ExecContext(ctx, `
		INSERT INTO repository_entries (
			entry_id, repository_id, path, path_key, parent_key, kind, size, mtime_ns,
			stat_checked_ns, state, content_id, asset_id, missing_since, trash_id,
			trashed_at, revision, updated_at
		) VALUES (?, ?, ?, lower(?), '', 'file', ?, 1, 1, ?, ?, ?, ?, ?, ?, 1, 1)
	`, entryID, params.RepositoryID, params.NodeName, params.NodeName, params.FileSize,
		params.EntryState, params.ContentID, params.AssetID, missingSince, trashID, trashedAt); err != nil {
		return AssetOccurrence{}, fmt.Errorf("insert fixture repository entry: %w", err)
	}
	return AssetOccurrence{ContentID: params.ContentID, EntryID: entryID}, nil
}

// EntryStateTrashedIf is the fixture entry state of an Asset that is in the
// repository trash when trashed is set and active otherwise.
func EntryStateTrashedIf(trashed bool) string {
	if trashed {
		return "trashed"
	}
	return "present"
}

// SetAssetEntriesState moves every file entry of an Asset to present,
// missing, or trashed, as a scan or the repository trash would; the Asset's
// lifecycle state follows its entries.
func SetAssetEntriesState(ctx context.Context, database SQLExecutor, assetID uuid.UUID, state string) error {
	switch state {
	case "present", "missing", "trashed":
	default:
		return fmt.Errorf("unsupported fixture entry state %q", state)
	}
	_, err := database.ExecContext(ctx, `
		UPDATE repository_entries
		SET state = ?1,
		    missing_since = CASE WHEN ?1 = 'missing' THEN 1 END,
		    trash_id = CASE WHEN ?1 = 'trashed' THEN ?2 END,
		    trashed_at = CASE WHEN ?1 = 'trashed' THEN 1 END,
		    revision = revision + 1
		WHERE asset_id = ?3 AND kind = 'file'
	`, state, uuid.NewString(), assetID)
	return err
}
