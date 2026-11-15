-- name: GetStackKindsByIDs :many
SELECT stack_id, stack_kind, cover_media_item_id
FROM asset_stacks
WHERE stack_id IN (sqlc.slice('stack_ids'));
