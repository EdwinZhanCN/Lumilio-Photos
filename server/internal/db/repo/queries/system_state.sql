-- name: GetSystemState :one
SELECT * FROM system_state
WHERE id = 1;

-- name: SetBootstrapPhase :one
UPDATE system_state
SET
    bootstrap_phase = $1,
    updated_at = NOW()
WHERE id = 1
RETURNING *;
