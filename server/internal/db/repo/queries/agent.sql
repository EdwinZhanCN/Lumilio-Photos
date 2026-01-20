-- name: GetCheckpoint :one
SELECT data FROM agent_checkpoints
WHERE id = $1;

-- name: UpsertCheckpoint :exec
INSERT INTO agent_checkpoints (id, data, updated_at)
VALUES ($1, $2, NOW())
ON CONFLICT (id)
DO UPDATE SET
    data = EXCLUDED.data,
    updated_at = NOW();
