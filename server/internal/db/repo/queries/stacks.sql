-- Logical media items -------------------------------------------------------

-- name: GetMediaItemByAssetID :one
SELECT mi.*
FROM media_items mi
JOIN media_item_assets mia ON mia.media_item_id = mi.media_item_id
WHERE mia.asset_id = ?1;

-- name: GetMediaItemComponents :many
SELECT mia.asset_id, mia.media_item_id, mia.relation, mia.position, mia.created_at
FROM media_item_assets mia
JOIN assets a ON a.asset_id = mia.asset_id
WHERE mia.media_item_id = sqlc.arg('media_item_id')
  AND (sqlc.narg('owner_id') IS NULL OR a.owner_id = sqlc.narg('owner_id'))
ORDER BY mia.position ASC, mia.created_at ASC;

-- name: GetMediaItemsByAssetIDs :many
SELECT mia.asset_id, mia.media_item_id, mi.primary_asset_id
FROM media_item_assets mia
JOIN media_items mi ON mi.media_item_id = mia.media_item_id
WHERE mia.asset_id IN (sqlc.slice('asset_ids'));

-- name: MoveMediaItemComponent :exec
UPDATE media_item_assets
SET media_item_id = sqlc.arg('target_media_item_id'),
    relation = sqlc.arg('relation'),
    position = sqlc.arg('position')
WHERE asset_id = sqlc.arg('asset_id');

-- name: MoveAllMediaItemComponents :exec
UPDATE media_item_assets
SET media_item_id = sqlc.arg('target_media_item_id')
WHERE media_item_id = sqlc.arg('source_media_item_id');

-- name: DeleteMediaItem :exec
DELETE FROM media_items WHERE media_item_id = ?1;

-- Presentation stacks ------------------------------------------------------

-- name: DeleteStack :exec
DELETE FROM asset_stacks WHERE stack_id = ?1;

-- name: AddStackMember :exec
INSERT INTO asset_stack_members (media_item_id, stack_id, position, created_at)
VALUES (
  sqlc.arg('media_item_id'),
  sqlc.arg('stack_id'),
  sqlc.arg('position'),
  sqlc.arg('created_at')
)
ON CONFLICT (media_item_id) DO NOTHING;

-- name: RemoveStackMemberByAssetID :exec
DELETE FROM asset_stack_members
WHERE media_item_id IN (
  SELECT media_item_id
  FROM media_item_assets
  WHERE asset_id = ?1
);

-- name: GetStackMembers :many
SELECT asm.media_item_id, mi.primary_asset_id AS asset_id, asm.position, asm.created_at
FROM asset_stack_members asm
JOIN media_items mi ON mi.media_item_id = asm.media_item_id
JOIN assets a ON a.asset_id = mi.primary_asset_id
WHERE asm.stack_id = sqlc.arg('stack_id')
  AND a.is_deleted = false
  AND (sqlc.narg('owner_id') IS NULL OR mi.owner_id = sqlc.narg('owner_id'))
ORDER BY asm.position ASC, asm.created_at ASC;

-- name: GetStackMembersAny :many
SELECT asm.media_item_id, mi.primary_asset_id AS asset_id, asm.position, asm.created_at
FROM asset_stack_members asm
JOIN media_items mi ON mi.media_item_id = asm.media_item_id
WHERE asm.stack_id = sqlc.arg('stack_id')
  AND (sqlc.narg('owner_id') IS NULL OR mi.owner_id = sqlc.narg('owner_id'))
ORDER BY asm.position ASC, asm.created_at ASC;

-- name: GetStackByAssetID :one
SELECT asm.stack_id, asm.media_item_id, asm.position
FROM media_item_assets mia
JOIN asset_stack_members asm ON asm.media_item_id = mia.media_item_id
WHERE mia.asset_id = ?1;

-- name: GetStacksByAssetIDs :many
SELECT mia.asset_id, asm.media_item_id, asm.stack_id, asm.position
FROM media_item_assets mia
JOIN asset_stack_members asm ON asm.media_item_id = mia.media_item_id
WHERE mia.asset_id IN (sqlc.slice('asset_ids'));

-- name: GetStackMemberCount :one
SELECT COUNT(*) AS count
FROM asset_stack_members asm
JOIN media_items mi ON mi.media_item_id = asm.media_item_id
JOIN assets a ON a.asset_id = mi.primary_asset_id
WHERE asm.stack_id = sqlc.arg('stack_id')
  AND a.is_deleted = false
  AND (sqlc.narg('owner_id') IS NULL OR mi.owner_id = sqlc.narg('owner_id'));

-- name: GetStackMemberCountAny :one
SELECT COUNT(*) AS count
FROM asset_stack_members asm
JOIN media_items mi ON mi.media_item_id = asm.media_item_id
WHERE asm.stack_id = sqlc.arg('stack_id')
  AND (sqlc.narg('owner_id') IS NULL OR mi.owner_id = sqlc.narg('owner_id'));

-- Structural and burst detection ------------------------------------------

-- name: FindCandidatesForStackingByName :many
SELECT a.asset_id,
       mia.media_item_id,
       a.owner_id,
       a.original_filename,
       a.mime_type,
       CAST(COALESCE(json_extract(a.specific_metadata, '$.is_raw'), 0) AS INTEGER) AS is_raw,
       CAST(COALESCE(json_extract(a.specific_metadata, '$.camera_model'), '') AS TEXT) AS camera_model,
       CAST(COALESCE(
           NULLIF(json_extract(a.exif_raw, '$.BurstUUID'), ''),
           NULLIF(json_extract(a.exif_raw, '$.BurstID'), ''),
           NULLIF(json_extract(a.exif_raw, '$.BurstGroupID'), ''),
           ''
       ) AS TEXT) AS burst_id,
       a.taken_time,
       a.upload_time,
       CAST(lower(a.original_filename) AS TEXT) AS base_name
FROM assets a
JOIN media_item_assets mia ON mia.asset_id = a.asset_id
WHERE a.repository_id = ?1
  AND a.is_deleted = false
  AND a.type = 'PHOTO'
ORDER BY base_name, a.original_filename;

-- name: FindMediaItemsForBurstDetection :many
SELECT mi.media_item_id,
       mi.owner_id,
       mi.repository_id,
       mi.primary_asset_id,
       primary_asset.original_filename,
       primary_asset.taken_time,
       primary_asset.upload_time,
       CAST(COALESCE(json_extract(primary_asset.specific_metadata, '$.camera_model'), '') AS TEXT) AS camera_model,
       CAST(COALESCE(
           MAX(NULLIF(json_extract(component.exif_raw, '$.BurstUUID'), '')),
           MAX(NULLIF(json_extract(component.exif_raw, '$.BurstID'), '')),
           MAX(NULLIF(json_extract(component.exif_raw, '$.BurstGroupID'), '')),
           ''
       ) AS TEXT) AS burst_id
FROM media_items mi
JOIN assets primary_asset ON primary_asset.asset_id = mi.primary_asset_id
JOIN media_item_assets mia ON mia.media_item_id = mi.media_item_id
JOIN assets component ON component.asset_id = mia.asset_id
LEFT JOIN asset_stack_members asm ON asm.media_item_id = mi.media_item_id
WHERE mi.repository_id = ?1
  AND mi.media_kind = 'photo'
  AND primary_asset.is_deleted = false
  AND asm.media_item_id IS NULL
GROUP BY mi.media_item_id, primary_asset.asset_id
ORDER BY COALESCE(primary_asset.taken_time, primary_asset.upload_time), mi.media_item_id;

-- name: GetStackMembershipsByMediaItemIDs :many
SELECT stack_id, CAST(MIN(position) AS INTEGER) AS position
FROM asset_stack_members
WHERE media_item_id IN (sqlc.slice('media_item_ids'))
GROUP BY stack_id;

-- name: RemoveStackMembershipsByMediaItemIDs :exec
DELETE FROM asset_stack_members
WHERE media_item_id IN (sqlc.slice('media_item_ids'));

-- name: UpdateMediaItemAfterStructuralMerge :exec
UPDATE media_items
SET primary_asset_id = sqlc.arg('primary_asset_id'),
    media_kind = CASE
      WHEN EXISTS (
        SELECT 1
        FROM media_item_assets
        WHERE media_item_assets.media_item_id = sqlc.arg('target_media_item_id')
          AND relation = 'live_photo_video'
      ) THEN 'live_photo'
      ELSE 'photo'
    END,
    group_key = sqlc.arg('group_key'),
    updated_at = sqlc.arg('updated_at')
WHERE media_items.media_item_id = sqlc.arg('target_media_item_id');

-- name: GetBurstStackByGroupKey :one
SELECT *
FROM asset_stacks
WHERE stack_kind = 'burst' AND group_key = sqlc.arg('group_key');

-- name: GetNextStackPosition :one
SELECT CAST(COALESCE(MAX(position) + 1, 0) AS INTEGER)
FROM asset_stack_members
WHERE stack_id = sqlc.arg('stack_id');

-- name: CreateAssetStack :one
INSERT INTO asset_stacks (
  stack_id, owner_id, repository_id, stack_kind, cover_media_item_id,
  group_key, created_at, updated_at
) VALUES (
  sqlc.arg('stack_id'),
  sqlc.narg('owner_id'),
  sqlc.narg('repository_id'),
  sqlc.arg('stack_kind'),
  sqlc.narg('cover_media_item_id'),
  sqlc.narg('group_key'),
  sqlc.arg('created_at'),
  sqlc.arg('updated_at')
)
RETURNING *;

-- name: FindLivePhotoPair :many
SELECT asset_id, type
FROM assets
WHERE owner_id = sqlc.arg('owner_id')
  AND is_deleted = false
  AND type IN ('PHOTO', 'VIDEO')
  AND json_extract(specific_metadata, '$.content_identifier')
      = CAST(sqlc.arg('content_identifier') AS TEXT)
ORDER BY type, asset_id;

-- name: CountStackedMediaItems :one
SELECT COUNT(DISTINCT media_item_id)
FROM asset_stack_members
WHERE media_item_id IN (sqlc.slice('media_item_ids'));

-- name: UpdateMediaItemAsLivePhoto :exec
UPDATE media_items
SET media_kind = 'live_photo',
    primary_asset_id = sqlc.arg('primary_asset_id'),
    group_key = sqlc.arg('group_key'),
    updated_at = sqlc.arg('updated_at')
WHERE media_item_id = sqlc.arg('media_item_id');
