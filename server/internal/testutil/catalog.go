package testutil

import (
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
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
	ContentID  uuid.UUID
	RootNodeID uuid.UUID
	NodeID     uuid.UUID
	LocationID uuid.UUID
}

// InsertAssetOccurrence seeds the post-cutover owner/content/Asset/node/
// Location contract. It intentionally does not create repository observations:
// the active occurrence projection requires the current node and binding, while
// observation history belongs only in tests that exercise reconciliation.
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

	var rootNodeID uuid.UUID
	err := database.QueryRowContext(ctx, `
		SELECT node_id FROM repository_nodes
		WHERE repository_id = ? AND parent_node_id IS NULL AND lifecycle = 'active'
	`, params.RepositoryID).Scan(&rootNodeID)
	if errors.Is(err, sql.ErrNoRows) {
		rootNodeID = uuid.New()
		if _, err := database.ExecContext(ctx, `
			INSERT INTO repository_nodes (
				node_id, repository_id, parent_node_id, name, name_key, kind,
				observation_revision, created_at, updated_at
			) VALUES (?, ?, NULL, '', '', 'directory', 1, 1, 1)
		`, rootNodeID, params.RepositoryID); err != nil {
			return AssetOccurrence{}, fmt.Errorf("insert fixture repository root node: %w", err)
		}
	} else if err != nil {
		return AssetOccurrence{}, fmt.Errorf("load fixture repository root node: %w", err)
	}

	nodeID := uuid.New()
	if _, err := database.ExecContext(ctx, `
		INSERT INTO repository_nodes (
			node_id, repository_id, parent_node_id, name, name_key, kind,
			observation_revision, file_size, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, 'file', 2, ?, 1, 1)
	`, nodeID, params.RepositoryID, rootNodeID, params.NodeName,
		params.NodeName, params.FileSize); err != nil {
		return AssetOccurrence{}, fmt.Errorf("insert fixture repository file node: %w", err)
	}
	locationID := uuid.New()
	if _, err := database.ExecContext(ctx, `
		INSERT INTO asset_locations (
			location_id, node_id, asset_id, bound_observation_revision,
			created_at, updated_at
		) VALUES (?, ?, ?, 2, 1, 1)
	`, locationID, nodeID, params.AssetID); err != nil {
		return AssetOccurrence{}, fmt.Errorf("insert fixture asset Location: %w", err)
	}
	return AssetOccurrence{
		ContentID: params.ContentID, RootNodeID: rootNodeID,
		NodeID: nodeID, LocationID: locationID,
	}, nil
}
