-- name: InsertSpikeRecord :one
INSERT INTO spike_records (
    id,
    optional_parent_id,
    created_at,
    payload,
    embedding
) VALUES (?, ?, ?, ?, ?)
RETURNING *;

-- name: GetSpikeRecord :one
SELECT *
FROM spike_records
WHERE id = ?;
