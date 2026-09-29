// Package lifecycle holds the Asset lifecycle's catalog rules. An Asset
// exists if and only if it has at least one entry in repository_entries; its
// lifecycle state (active, missing, trashed) is derived from those entries by
// catalog triggers. PurgeEntriesTx is the only hard delete of an Asset.
package lifecycle

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"server/internal/event"
)

// Lifecycle states of an Asset, derived from its entries.
const (
	StateActive  = "active"
	StateMissing = "missing"
	StateTrashed = "trashed"
)

// PurgeResult reports what one purge removed.
type PurgeResult struct {
	Entries int64
	// Assets are the Assets deleted because no entry was left for them.
	Assets []uuid.UUID
}

// PurgeEntriesTx is the only way an Asset is deleted. It removes the given
// entries and then, in the same transaction, every Asset they referenced that
// has no entry left anywhere, together with its metadata and derived rows.
// Share links and agent pins that selected a purged Asset are removed, logical
// media and presentation stacks that lose their last member are dissolved,
// and the OCR index and Event projection are told. Files and on-disk
// artifacts are never touched here: a caller unlinks trashed files before it
// purges, and the artifact cleaner reclaims artifacts of Assets that no longer
// exist.
//
// Callers keep one purge small enough for the writer's time budget; a
// repository removal purges all of its entries in its own transaction.
func PurgeEntriesTx(ctx context.Context, tx *sql.Tx, entryIDs []uuid.UUID) (PurgeResult, error) {
	var result PurgeResult
	if tx == nil {
		return result, errors.New("purge needs a catalog transaction")
	}
	if len(entryIDs) == 0 {
		return result, nil
	}
	ids := make([]string, 0, len(entryIDs))
	for _, id := range entryIDs {
		ids = append(ids, id.String())
	}
	encoded, err := json.Marshal(ids)
	if err != nil {
		return result, err
	}
	var candidates string
	if err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(json_group_array(DISTINCT asset_id), '[]')
		FROM repository_entries
		WHERE entry_id IN (SELECT value FROM json_each(?)) AND asset_id IS NOT NULL
	`, string(encoded)).Scan(&candidates); err != nil {
		return result, fmt.Errorf("collect purged entries' Assets: %w", err)
	}
	deleted, err := tx.ExecContext(ctx, `
		DELETE FROM repository_entries WHERE entry_id IN (SELECT value FROM json_each(?))
	`, string(encoded))
	if err != nil {
		return result, fmt.Errorf("purge entries: %w", err)
	}
	result.Entries, _ = deleted.RowsAffected()
	assets, err := collectAssetsTx(ctx, tx, candidates)
	if err != nil {
		return result, err
	}
	result.Assets = assets
	return result, nil
}

// PurgeRepositoryEntriesTx purges every entry of one repository, for
// repository removal. Assets with an entry in another repository survive.
func PurgeRepositoryEntriesTx(ctx context.Context, tx *sql.Tx, repositoryID uuid.UUID) (PurgeResult, error) {
	var result PurgeResult
	if tx == nil || repositoryID == uuid.Nil {
		return result, errors.New("repository purge needs a transaction and a repository")
	}
	var candidates string
	if err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(json_group_array(DISTINCT asset_id), '[]')
		FROM repository_entries
		WHERE repository_id = ? AND asset_id IS NOT NULL
	`, repositoryID.String()).Scan(&candidates); err != nil {
		return result, fmt.Errorf("collect repository Assets: %w", err)
	}
	deleted, err := tx.ExecContext(ctx, `DELETE FROM repository_entries WHERE repository_id = ?`, repositoryID.String())
	if err != nil {
		return result, fmt.Errorf("purge repository entries: %w", err)
	}
	result.Entries, _ = deleted.RowsAffected()
	assets, err := collectAssetsTx(ctx, tx, candidates)
	if err != nil {
		return result, err
	}
	result.Assets = assets
	return result, nil
}

// collectAssetsTx deletes the candidate Assets that have no entry left.
func collectAssetsTx(ctx context.Context, tx *sql.Tx, candidates string) ([]uuid.UUID, error) {
	var orphans string
	if err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(json_group_array(candidate.value), '[]')
		FROM json_each(?) candidate
		WHERE NOT EXISTS (
			SELECT 1 FROM repository_entries entry WHERE entry.asset_id = candidate.value
		)
	`, candidates).Scan(&orphans); err != nil {
		return nil, fmt.Errorf("find Assets without entries: %w", err)
	}
	var ids []string
	if err := json.Unmarshal([]byte(orphans), &ids); err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}
	var owners []int32
	rows, err := tx.QueryContext(ctx, `
		SELECT DISTINCT owner_id FROM assets
		WHERE asset_id IN (SELECT value FROM json_each(?)) AND owner_id IS NOT NULL
	`, orphans)
	if err != nil {
		return nil, fmt.Errorf("read purged Asset owners: %w", err)
	}
	for rows.Next() {
		var owner int32
		if err := rows.Scan(&owner); err != nil {
			rows.Close()
			return nil, err
		}
		owners = append(owners, owner)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}

	var media, stacks string
	if err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(json_group_array(DISTINCT media_item_id), '[]')
		FROM media_item_assets
		WHERE asset_id IN (SELECT value FROM json_each(?))
	`, orphans).Scan(&media); err != nil {
		return nil, fmt.Errorf("collect purged media items: %w", err)
	}
	if err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(json_group_array(DISTINCT stack_id), '[]')
		FROM asset_stack_members
		WHERE media_item_id IN (SELECT value FROM json_each(?))
	`, media).Scan(&stacks); err != nil {
		return nil, fmt.Errorf("collect purged stacks: %w", err)
	}

	statements := []struct {
		what, sql string
		arg       string
	}{
		// Links and pins select Assets by ID in JSON; a selection that named a
		// purged Asset no longer means what its owner shared or pinned.
		{"remove share links", `
			DELETE FROM share_links
			WHERE EXISTS (
				SELECT 1 FROM json_each(share_links.asset_ids) selected
				WHERE selected.value IN (SELECT value FROM json_each(?))
			)`, orphans},
		{"remove agent pins", `
			DELETE FROM agent_pins
			WHERE EXISTS (
				SELECT 1 FROM json_each(agent_pins.asset_ids) selected
				WHERE selected.value IN (SELECT value FROM json_each(?))
			)`, orphans},
		// A published OCR document is withdrawn: the outbox writer deletes the
		// document of an Asset that no longer exists.
		{"withdraw OCR documents", `
			UPDATE ocr_index_metadata
			SET revision = revision + 1,
			    updated_at = CAST(unixepoch('subsec') * 1000000 AS INTEGER)
			WHERE asset_id IN (SELECT value FROM json_each(?))`, orphans},
		{"queue OCR withdrawals", `
			INSERT INTO ocr_index_outbox (asset_id, revision, updated_at)
			SELECT asset_id, revision, updated_at FROM ocr_index_metadata
			WHERE asset_id IN (SELECT value FROM json_each(?))
			ON CONFLICT (asset_id) DO UPDATE SET
				revision = excluded.revision,
				updated_at = excluded.updated_at`, orphans},
		{"delete Assets", `
			DELETE FROM assets WHERE asset_id IN (SELECT value FROM json_each(?))`, orphans},
		// Logical media and stacks are projections of Assets: one with no
		// member left is dissolved, and a media item whose primary was purged
		// serves from its best remaining component.
		{"dissolve empty media items", `
			DELETE FROM media_items
			WHERE media_item_id IN (SELECT value FROM json_each(?))
			  AND NOT EXISTS (
				SELECT 1 FROM media_item_assets member
				WHERE member.media_item_id = media_items.media_item_id
			  )`, media},
		{"re-pick media item primaries", `
			UPDATE media_items
			SET primary_asset_id = (
					SELECT member.asset_id
					FROM media_item_assets member
					JOIN assets component ON component.asset_id = member.asset_id
					WHERE member.media_item_id = media_items.media_item_id
					ORDER BY
						component.lifecycle_state <> 'active',
						CASE member.relation
							WHEN 'jpeg_original' THEN 0
							WHEN 'live_photo_still' THEN 1
							WHEN 'edited_version' THEN 2
							WHEN 'raw_original' THEN 3
							ELSE 4
						END,
						member.position ASC,
						member.created_at ASC
					LIMIT 1
				),
				updated_at = CAST(unixepoch('subsec') * 1000000 AS INTEGER)
			WHERE media_item_id IN (SELECT value FROM json_each(?))
			  AND primary_asset_id IS NULL`, media},
		{"dissolve degenerate stacks", `
			DELETE FROM asset_stacks
			WHERE stack_id IN (SELECT value FROM json_each(?))
			  AND (
				SELECT count(*) FROM asset_stack_members member
				WHERE member.stack_id = asset_stacks.stack_id
			  ) < 2`, stacks},
	}
	for _, statement := range statements {
		if _, err := tx.ExecContext(ctx, statement.sql, statement.arg); err != nil {
			return nil, fmt.Errorf("%s: %w", statement.what, err)
		}
	}
	for _, owner := range owners {
		if err := event.MarkEventFactsChangedTx(ctx, tx, owner, "asset_purged"); err != nil {
			return nil, err
		}
	}
	purged := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		parsed, err := uuid.Parse(id)
		if err != nil {
			return nil, err
		}
		purged = append(purged, parsed)
	}
	return purged, nil
}
