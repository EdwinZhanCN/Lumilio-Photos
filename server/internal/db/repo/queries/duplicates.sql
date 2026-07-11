-- ============================================================================
-- Duplicate detection candidate queries
-- ============================================================================

-- name: GetStackMembershipForRepository :many
-- Each asset is mapped to its presentation stack when present, otherwise to
-- its logical media item. This skips duplicate edges both within RAW/JPEG or
-- Live Photo components and within intentional burst/manual stacks.
SELECT mia.asset_id, COALESCE(asm.stack_id, mia.media_item_id) AS stack_id
FROM media_item_assets mia
LEFT JOIN asset_stack_members asm ON asm.media_item_id = mia.media_item_id
INNER JOIN assets a ON a.asset_id = mia.asset_id
WHERE a.repository_id = sqlc.arg('repository_id')
  AND a.is_deleted = false;

-- name: GetExactDuplicateCandidates :many
-- Returns assets in a repository that share the exact same (hash, file_size)
-- with at least one other asset of the same owner. Only photos are considered,
-- and only non-deleted assets. Results are ordered so members of the same
-- duplicate set (owner included in the grouping key) are adjacent.
SELECT a.asset_id, a.owner_id, a.hash, a.file_size, a.original_filename, a.taken_time, a.upload_time, a.rating
FROM assets a
WHERE a.is_deleted = false
  AND a.type = 'PHOTO'
  AND a.hash IS NOT NULL
  AND a.repository_id = sqlc.arg('repository_id')
  AND EXISTS (
    SELECT 1 FROM assets b
    WHERE b.is_deleted = false
      AND b.type = 'PHOTO'
      AND b.hash IS NOT NULL
      AND b.repository_id = a.repository_id
      AND b.owner_id IS NOT DISTINCT FROM a.owner_id
      AND b.hash = a.hash
      AND b.file_size = a.file_size
      AND b.asset_id <> a.asset_id
  )
ORDER BY a.owner_id, a.hash, a.file_size, a.asset_id;

-- name: ListPHashEmbeddingsForRepository :many
-- Loads pHash embeddings for every non-deleted photo in a repository so the
-- service layer can build a similarity graph in-memory. owner_id is included
-- because duplicate edges never cross owners.
SELECT a.asset_id, a.owner_id, a.file_size, a.taken_time, a.upload_time, a.rating, e.vector
FROM assets a
JOIN embeddings e ON e.asset_id = a.asset_id
WHERE a.is_deleted = false
  AND a.type = 'PHOTO'
  AND a.repository_id = sqlc.arg('repository_id')
  AND e.embedding_type = 'phash'
  AND e.is_primary = true;

-- name: GetPHashEmbeddingsByAssetIDs :many
-- Ref-scoped variant of ListPHashEmbeddingsForRepository: pHash embeddings for
-- a specific asset set, for the agent dedupe tool's in-memory similarity graph.
SELECT a.asset_id, a.file_size, a.taken_time, a.upload_time, a.rating, e.vector
FROM assets a
JOIN embeddings e ON e.asset_id = a.asset_id
WHERE a.is_deleted = false
  AND a.type = 'PHOTO'
  AND a.asset_id = ANY(sqlc.arg('asset_ids')::uuid[])
  AND e.embedding_type = 'phash'
  AND e.is_primary = true;

-- ============================================================================
-- Duplicate group lifecycle
-- ============================================================================

-- name: DeletePendingDuplicateGroupsByRepository :exec
-- Removes the pending detection state for a repository. Merged and dismissed
-- groups are preserved so the user retains an audit trail of resolved sets.
DELETE FROM duplicate_groups
WHERE repository_id = sqlc.arg('repository_id')
  AND status = 'pending';

-- name: CreateDuplicateGroup :one
INSERT INTO duplicate_groups (
    repository_id, owner_id, method, status, asset_count, total_size,
    recommended_keeper_asset_id, detection_version
) VALUES (
    sqlc.arg('repository_id'),
    sqlc.narg('owner_id'),
    sqlc.arg('method'),
    'pending',
    sqlc.arg('asset_count'),
    sqlc.arg('total_size'),
    sqlc.arg('recommended_keeper_asset_id'),
    sqlc.arg('detection_version')
)
RETURNING group_id;

-- name: InsertDuplicateGroupAsset :exec
INSERT INTO duplicate_group_assets (group_id, asset_id, role, file_size)
VALUES ($1, $2, $3, $4)
ON CONFLICT (group_id, asset_id) DO UPDATE
SET role = EXCLUDED.role,
    file_size = EXCLUDED.file_size;

-- name: InsertDuplicateGroupEdge :exec
-- Stores pair-level evidence. Callers must order endpoints so asset_id_a < asset_id_b.
INSERT INTO duplicate_group_edges (group_id, asset_id_a, asset_id_b, method, distance, confidence)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (group_id, asset_id_a, asset_id_b, method) DO UPDATE
SET distance = EXCLUDED.distance,
    confidence = EXCLUDED.confidence;

-- name: GetDuplicateGroupByID :one
SELECT
    g.group_id,
    g.repository_id,
    g.owner_id,
    g.method,
    g.status,
    g.asset_count,
    g.total_size,
    g.recommended_keeper_asset_id,
    g.keeper_asset_id,
    g.detection_version,
    g.detected_at,
    g.resolved_at,
    g.created_at,
    g.updated_at
FROM duplicate_groups g
WHERE g.group_id = sqlc.arg('group_id');

-- name: ListDuplicateGroups :many
-- Paginated list of duplicate groups for the given repository, owner, and
-- status. owner_id NULL means no owner scope (admin); non-admin callers pass
-- their own ID and never see NULL-owner or foreign groups.
-- Pending groups are returned newest-first; resolved groups by resolution time.
SELECT
    g.group_id,
    g.repository_id,
    g.owner_id,
    g.method,
    g.status,
    g.asset_count,
    g.total_size,
    g.recommended_keeper_asset_id,
    g.keeper_asset_id,
    g.detection_version,
    g.detected_at,
    g.resolved_at,
    g.created_at,
    g.updated_at
FROM duplicate_groups g
WHERE (sqlc.narg('repository_id')::uuid IS NULL OR g.repository_id = sqlc.narg('repository_id'))
  AND (sqlc.narg('owner_id')::integer IS NULL OR g.owner_id = sqlc.narg('owner_id'))
  AND (sqlc.narg('status')::text IS NULL OR g.status = sqlc.narg('status'))
ORDER BY
    CASE WHEN g.status = 'pending' THEN g.detected_at ELSE g.resolved_at END DESC NULLS LAST,
    g.group_id DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountDuplicateGroups :one
SELECT COUNT(*) AS count
FROM duplicate_groups g
WHERE (sqlc.narg('repository_id')::uuid IS NULL OR g.repository_id = sqlc.narg('repository_id'))
  AND (sqlc.narg('owner_id')::integer IS NULL OR g.owner_id = sqlc.narg('owner_id'))
  AND (sqlc.narg('status')::text IS NULL OR g.status = sqlc.narg('status'));

-- name: GetDuplicateGroupAssets :many
SELECT
    dga.group_id,
    dga.asset_id,
    dga.role,
    dga.file_size
FROM duplicate_group_assets dga
WHERE dga.group_id = sqlc.arg('group_id')
ORDER BY dga.file_size DESC, dga.asset_id ASC;

-- name: GetDuplicateGroupAssetsBatch :many
-- Bulk variant used for the list endpoint to enrich many groups in one query.
SELECT
    dga.group_id,
    dga.asset_id,
    dga.role,
    dga.file_size
FROM duplicate_group_assets dga
WHERE dga.group_id = ANY(sqlc.arg('group_ids')::uuid[])
ORDER BY dga.group_id, dga.file_size DESC, dga.asset_id ASC;

-- name: GetDuplicateGroupEdges :many
SELECT
    dge.group_id,
    dge.asset_id_a,
    dge.asset_id_b,
    dge.method,
    dge.distance,
    dge.confidence
FROM duplicate_group_edges dge
WHERE dge.group_id = sqlc.arg('group_id')
ORDER BY dge.method, dge.asset_id_a, dge.asset_id_b;

-- name: UpdateDuplicateGroupKeeperRole :exec
-- Resets all asset roles in a group, then flags the chosen keeper.
UPDATE duplicate_group_assets
SET role = CASE
    WHEN asset_id = sqlc.arg('keeper_asset_id') THEN 'keeper'
    ELSE 'duplicate'
END
WHERE group_id = sqlc.arg('group_id');

-- name: MarkDuplicateGroupMerged :exec
UPDATE duplicate_groups
SET status = 'merged',
    keeper_asset_id = sqlc.arg('keeper_asset_id'),
    resolved_at = CURRENT_TIMESTAMP,
    updated_at = CURRENT_TIMESTAMP
WHERE group_id = sqlc.arg('group_id');

-- name: MarkDuplicateGroupDismissed :exec
UPDATE duplicate_groups
SET status = 'dismissed',
    resolved_at = CURRENT_TIMESTAMP,
    updated_at = CURRENT_TIMESTAMP
WHERE group_id = sqlc.arg('group_id');

-- name: GetDuplicateSummary :one
-- Top-level metrics for the Utilities rail card.
SELECT
    COUNT(*) FILTER (WHERE g.status = 'pending')   AS pending_groups,
    COUNT(*) FILTER (WHERE g.status = 'merged')    AS merged_groups,
    COUNT(*) FILTER (WHERE g.status = 'dismissed') AS dismissed_groups,
    COALESCE(SUM(g.asset_count) FILTER (WHERE g.status = 'pending'), 0)::bigint AS pending_assets,
    COALESCE(SUM(GREATEST(g.asset_count - 1, 0)) FILTER (WHERE g.status = 'pending'), 0)::bigint AS recoverable_assets,
    COALESCE(
        SUM(
            CASE
                WHEN g.status = 'pending'
                THEN GREATEST(g.total_size - COALESCE((
                    SELECT MAX(dga.file_size)
                    FROM duplicate_group_assets dga
                    WHERE dga.group_id = g.group_id
                ), 0), 0)
                ELSE 0
            END
        ),
        0
    )::bigint AS recoverable_bytes,
    MAX(g.detected_at)::timestamptz AS last_detected_at
FROM duplicate_groups g
WHERE (sqlc.narg('repository_id')::uuid IS NULL OR g.repository_id = sqlc.narg('repository_id'))
  AND (sqlc.narg('owner_id')::integer IS NULL OR g.owner_id = sqlc.narg('owner_id'));

-- ============================================================================
-- Metadata merge helpers (used inside merge transactions)
-- ============================================================================

-- name: MergeAlbumAssetsForDuplicate :exec
-- Copies a duplicate asset's album memberships onto the keeper asset.
-- Existing keeper memberships are preserved (the conflict clause is a no-op).
INSERT INTO album_assets (album_id, asset_id, position, added_time)
SELECT aa.album_id, sqlc.arg('keeper_asset_id'), aa.position, aa.added_time
FROM album_assets aa
WHERE aa.asset_id = sqlc.arg('duplicate_asset_id')
ON CONFLICT (album_id, asset_id) DO NOTHING;

-- name: MergeAssetTagsForDuplicate :exec
-- Copies duplicate tags onto the keeper, choosing the higher confidence and
-- preferring user-provided tags over AI-generated ones on conflict.
INSERT INTO asset_tags (asset_id, tag_id, confidence, source)
SELECT
    sqlc.arg('keeper_asset_id'),
    t.tag_id,
    t.confidence,
    t.source
FROM asset_tags t
WHERE t.asset_id = sqlc.arg('duplicate_asset_id')
ON CONFLICT (asset_id, tag_id) DO UPDATE
SET confidence = GREATEST(asset_tags.confidence, EXCLUDED.confidence),
    source = CASE
        WHEN EXCLUDED.source = 'user' THEN 'user'
        WHEN asset_tags.source = 'user' THEN asset_tags.source
        ELSE EXCLUDED.source
    END;

-- name: MergeFaceClustersForDuplicate :exec
-- Re-parents the duplicate asset's face_items onto the keeper so cluster
-- memberships (and thus person assignments) follow the keeper after merge.
-- Used only for exact duplicates where bounding boxes match by construction.
UPDATE face_items
SET asset_id = sqlc.arg('keeper_asset_id')
WHERE asset_id = sqlc.arg('duplicate_asset_id');

-- name: ApplyMergedKeeperPreferences :exec
-- Applies merged rating/liked/description on top of the existing keeper values.
-- Rating uses MAX, liked is OR'd, description is set only when keeper currently
-- has no description (or the field is empty).
UPDATE assets
SET
    rating = CASE
        WHEN sqlc.narg('merged_rating')::integer IS NULL THEN rating
        WHEN rating IS NULL THEN sqlc.narg('merged_rating')::integer
        ELSE GREATEST(rating, sqlc.narg('merged_rating')::integer)
    END,
    liked = CASE
        WHEN sqlc.narg('merged_liked')::boolean IS NULL THEN liked
        WHEN liked IS NULL THEN sqlc.narg('merged_liked')::boolean
        ELSE liked OR sqlc.narg('merged_liked')::boolean
    END,
    specific_metadata = CASE
        WHEN sqlc.narg('merged_description')::text IS NULL THEN specific_metadata
        WHEN COALESCE(specific_metadata->>'description', '') <> '' THEN specific_metadata
        ELSE jsonb_set(
            COALESCE(specific_metadata, '{}'::jsonb),
            '{description}',
            to_jsonb(sqlc.narg('merged_description')::text)
        )
    END
WHERE asset_id = sqlc.arg('keeper_asset_id');
