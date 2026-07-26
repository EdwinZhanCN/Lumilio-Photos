-- name: GetStackKindsByIDs :many
SELECT stack_id, stack_kind
FROM asset_stacks
WHERE stack_id IN (sqlc.slice('stack_ids'));
