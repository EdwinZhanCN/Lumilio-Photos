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

// AssetOccurrenceParams describes one normalized catalog Asset and one active
// physical Location. The user and repository rows must already exist.
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
	IsDeleted    bool
	Status       string
	ContentID    uuid.UUID
	FullHash     string
}

type AssetOccurrence struct {
	ContentID uuid.UUID
	EntryID   uuid.UUID
}

// InsertAssetOccurrence seeds the owner/content/Asset contract with one present
// file entry at the repository root, which is what the active occurrence
// projection reads.
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
			upload_time, taken_time, is_deleted, status, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1)
	`, params.AssetID, params.OwnerID, params.ContentID, params.AssetType,
		params.Filename, params.MIMEType, params.UploadTime, params.TakenTime,
		params.IsDeleted, params.Status); err != nil {
		return AssetOccurrence{}, fmt.Errorf("insert fixture asset: %w", err)
	}

	entryID := uuid.New()
	if _, err := database.ExecContext(ctx, `
		INSERT INTO repository_entries (
			entry_id, repository_id, path, path_key, parent_key, kind, size, mtime_ns,
			stat_checked_ns, state, content_id, asset_id, revision, updated_at
		) VALUES (?, ?, ?, lower(?), '', 'file', ?, 1, 1, 'present', ?, ?, 1, 1)
	`, entryID, params.RepositoryID, params.NodeName, params.NodeName, params.FileSize,
		params.ContentID, params.AssetID); err != nil {
		return AssetOccurrence{}, fmt.Errorf("insert fixture repository entry: %w", err)
	}
	return AssetOccurrence{ContentID: params.ContentID, EntryID: entryID}, nil
}
