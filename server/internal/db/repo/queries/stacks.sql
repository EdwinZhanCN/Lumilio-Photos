-- name: CreateStack :one
INSERT INTO asset_stacks DEFAULT VALUES
RETURNING *;

-- name: GetStackByID :one
SELECT * FROM asset_stacks
WHERE stack_id = $1;

-- name: DeleteStack :exec
DELETE FROM asset_stacks
WHERE stack_id = $1;

-- name: AddStackMember :exec
INSERT INTO asset_stack_members (asset_id, stack_id, relation, position)
VALUES ($1, $2, $3, $4)
ON CONFLICT (asset_id) DO UPDATE
SET stack_id = EXCLUDED.stack_id,
    relation = EXCLUDED.relation,
    position = EXCLUDED.position;

-- name: RemoveStackMember :exec
DELETE FROM asset_stack_members
WHERE asset_id = $1;

-- name: GetStackMembers :many
SELECT asm.asset_id, asm.stack_id, asm.relation, asm.position, asm.created_at
FROM asset_stack_members asm
JOIN assets a ON a.asset_id = asm.asset_id
WHERE asm.stack_id = $1 AND a.is_deleted = false
ORDER BY asm.position ASC, asm.created_at ASC;

-- name: GetStackMembersAny :many
SELECT asm.asset_id, asm.stack_id, asm.relation, asm.position, asm.created_at
FROM asset_stack_members asm
JOIN assets a ON a.asset_id = asm.asset_id
WHERE asm.stack_id = $1
ORDER BY asm.position ASC, asm.created_at ASC;

-- name: GetStackByAssetID :one
SELECT asm.stack_id, asm.relation, asm.position
FROM asset_stack_members asm
WHERE asm.asset_id = $1;

-- name: GetStacksByAssetIDs :many
SELECT asm.asset_id, asm.stack_id, asm.relation, asm.position
FROM asset_stack_members asm
WHERE asm.asset_id = ANY($1::uuid[]);

-- name: GetStackMemberCount :one
SELECT COUNT(*) as count
FROM asset_stack_members asm
JOIN assets a ON a.asset_id = asm.asset_id
WHERE asm.stack_id = $1 AND a.is_deleted = false;

-- name: GetStackMemberCountAny :one
SELECT COUNT(*) as count
FROM asset_stack_members asm
JOIN assets a ON a.asset_id = asm.asset_id
WHERE asm.stack_id = $1;

-- name: FindCandidatesForStacking :many
-- Find assets in the same repository that share a base filename pattern
-- This is used for auto-detecting RAW+JPEG stacks
SELECT a.asset_id, a.original_filename, a.mime_type,
       a.specific_metadata->>'is_raw' as is_raw,
       regexp_replace(a.original_filename, '\.[^.]+$', '') as base_name
FROM assets a
WHERE a.repository_id = $1
  AND a.is_deleted = false
  AND a.type = 'PHOTO'
  AND a.asset_id NOT IN (
      SELECT asm.asset_id FROM asset_stack_members asm
  )
ORDER BY base_name, a.original_filename;

-- name: FindAssetsByBaseName :many
-- Find all assets sharing the same base filename (without extension and without iteration suffix)
SELECT a.asset_id, a.original_filename, a.mime_type,
       a.specific_metadata->>'is_raw' as is_raw
FROM assets a
WHERE a.repository_id = $1
  AND a.is_deleted = false
  AND a.type = 'PHOTO'
  AND (
    -- Match exact base name (without extension)
    a.original_filename ILIKE $2 || '.%'
    -- Also match iteration suffixes like ABC001-1.JPG, ABC001-2.JPG
    OR a.original_filename ILIKE $2 || '-%.%'
  )
ORDER BY a.original_filename;

-- name: FindCandidatesForStackingByName :many
-- Find assets that share base names but are not yet in any stack.
-- Includes taken_time and upload_time for time-based clustering.
SELECT a.asset_id, a.original_filename, a.mime_type,
       a.specific_metadata->>'is_raw' as is_raw,
       a.taken_time, a.upload_time,
       regexp_replace(a.original_filename, '\.[^.]+$', '') as base_name
FROM assets a
LEFT JOIN asset_stack_members asm ON asm.asset_id = a.asset_id
WHERE a.repository_id = $1
  AND a.is_deleted = false
  AND a.type = 'PHOTO'
  AND asm.asset_id IS NULL
ORDER BY base_name, a.original_filename;
