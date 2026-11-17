-- name: GetSystemState :one
SELECT * FROM system_state
WHERE id = 1;

-- name: SetBootstrapPhase :one
UPDATE system_state
SET
    bootstrap_phase = ?1,
    updated_at = CAST(unixepoch('subsec') * 1000000 AS INTEGER)
WHERE id = 1
RETURNING *;
