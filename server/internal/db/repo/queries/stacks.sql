-- Logical media items -------------------------------------------------------

-- name: GetMediaItemByAssetID :one
SELECT mi.*
FROM media_items mi
JOIN media_item_assets mia ON mia.media_item_id = mi.media_item_id
WHERE mia.asset_id = $1;

-- name: GetMediaItemComponents :many
SELECT mia.asset_id, mia.media_item_id, mia.relation, mia.position, mia.created_at
FROM media_item_assets mia
JOIN assets a ON a.asset_id = mia.asset_id
WHERE mia.media_item_id = sqlc.arg('media_item_id')
  AND (sqlc.narg('owner_id')::integer IS NULL OR a.owner_id = sqlc.narg('owner_id'))
ORDER BY mia.position ASC, mia.created_at ASC;

-- name: GetMediaItemsByAssetIDs :many
SELECT mia.asset_id, mia.media_item_id, mi.primary_asset_id
FROM media_item_assets mia
JOIN media_items mi ON mi.media_item_id = mia.media_item_id
WHERE mia.asset_id = ANY($1::uuid[]);

-- name: MoveMediaItemComponent :exec
UPDATE media_item_assets
SET media_item_id = sqlc.arg('target_media_item_id'),
    relation = sqlc.arg('relation'),
    position = sqlc.arg('position')
WHERE asset_id = sqlc.arg('asset_id');

-- name: DeleteMediaItem :exec
DELETE FROM media_items WHERE media_item_id = $1;

-- Presentation stacks ------------------------------------------------------

-- name: DeleteStack :exec
DELETE FROM asset_stacks WHERE stack_id = $1;

-- name: AddStackMember :exec
INSERT INTO asset_stack_members (media_item_id, stack_id, position)
VALUES ($1, $2, $3);

-- name: RemoveStackMemberByAssetID :exec
DELETE FROM asset_stack_members asm
USING media_item_assets mia
WHERE mia.asset_id = $1
  AND asm.media_item_id = mia.media_item_id;

-- name: GetStackMembers :many
SELECT asm.media_item_id, mi.primary_asset_id AS asset_id, asm.position, asm.created_at
FROM asset_stack_members asm
JOIN media_items mi ON mi.media_item_id = asm.media_item_id
JOIN assets a ON a.asset_id = mi.primary_asset_id
WHERE asm.stack_id = sqlc.arg('stack_id')
  AND a.is_deleted = false
  AND (sqlc.narg('owner_id')::integer IS NULL OR mi.owner_id = sqlc.narg('owner_id'))
ORDER BY asm.position ASC, asm.created_at ASC;

-- name: GetStackMembersAny :many
SELECT asm.media_item_id, mi.primary_asset_id AS asset_id, asm.position, asm.created_at
FROM asset_stack_members asm
JOIN media_items mi ON mi.media_item_id = asm.media_item_id
WHERE asm.stack_id = sqlc.arg('stack_id')
  AND (sqlc.narg('owner_id')::integer IS NULL OR mi.owner_id = sqlc.narg('owner_id'))
ORDER BY asm.position ASC, asm.created_at ASC;

-- name: GetStackByAssetID :one
SELECT asm.stack_id, asm.media_item_id, asm.position
FROM media_item_assets mia
JOIN asset_stack_members asm ON asm.media_item_id = mia.media_item_id
WHERE mia.asset_id = $1;

-- name: GetStacksByAssetIDs :many
SELECT mia.asset_id, asm.media_item_id, asm.stack_id, asm.position
FROM media_item_assets mia
JOIN asset_stack_members asm ON asm.media_item_id = mia.media_item_id
WHERE mia.asset_id = ANY($1::uuid[]);

-- name: GetStackMemberCount :one
SELECT COUNT(*) AS count
FROM asset_stack_members asm
JOIN media_items mi ON mi.media_item_id = asm.media_item_id
JOIN assets a ON a.asset_id = mi.primary_asset_id
WHERE asm.stack_id = sqlc.arg('stack_id')
  AND a.is_deleted = false
  AND (sqlc.narg('owner_id')::integer IS NULL OR mi.owner_id = sqlc.narg('owner_id'));

-- name: GetStackMemberCountAny :one
SELECT COUNT(*) AS count
FROM asset_stack_members asm
JOIN media_items mi ON mi.media_item_id = asm.media_item_id
WHERE asm.stack_id = sqlc.arg('stack_id')
  AND (sqlc.narg('owner_id')::integer IS NULL OR mi.owner_id = sqlc.narg('owner_id'));

-- Structural and burst detection ------------------------------------------

-- name: FindCandidatesForStackingByName :many
SELECT a.asset_id,
       mia.media_item_id,
       a.owner_id,
       a.original_filename,
       a.mime_type,
       a.specific_metadata->>'is_raw' AS is_raw,
       COALESCE(a.specific_metadata->>'camera_model', '')::text AS camera_model,
       COALESCE(
           NULLIF(a.exif_raw->>'BurstUUID', ''),
           NULLIF(a.exif_raw->>'BurstID', ''),
           NULLIF(a.exif_raw->>'BurstGroupID', ''),
           ''
       )::text AS burst_id,
       a.taken_time,
       a.upload_time,
       regexp_replace(a.original_filename, '\.[^.]+$', '') AS base_name
FROM assets a
JOIN media_item_assets mia ON mia.asset_id = a.asset_id
WHERE a.repository_id = $1
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
       COALESCE(primary_asset.specific_metadata->>'camera_model', '')::text AS camera_model,
       COALESCE(
           MAX(NULLIF(component.exif_raw->>'BurstUUID', '')),
           MAX(NULLIF(component.exif_raw->>'BurstID', '')),
           MAX(NULLIF(component.exif_raw->>'BurstGroupID', '')),
           ''
       )::text AS burst_id
FROM media_items mi
JOIN assets primary_asset ON primary_asset.asset_id = mi.primary_asset_id
JOIN media_item_assets mia ON mia.media_item_id = mi.media_item_id
JOIN assets component ON component.asset_id = mia.asset_id
LEFT JOIN asset_stack_members asm ON asm.media_item_id = mi.media_item_id
WHERE mi.repository_id = $1
  AND mi.media_kind = 'photo'
  AND primary_asset.is_deleted = false
  AND asm.media_item_id IS NULL
GROUP BY mi.media_item_id, primary_asset.asset_id
ORDER BY COALESCE(primary_asset.taken_time, primary_asset.upload_time), mi.media_item_id;
