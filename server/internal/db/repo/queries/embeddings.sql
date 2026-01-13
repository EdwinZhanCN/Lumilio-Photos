-- Unified embeddings table queries
-- name: UpsertEmbedding :exec
INSERT INTO embeddings (asset_id, embedding_type, embedding_model, embedding_dimensions, vector, is_primary)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (asset_id, embedding_type, embedding_model)
DO UPDATE SET
    vector = EXCLUDED.vector,
    embedding_dimensions = EXCLUDED.embedding_dimensions,
    is_primary = EXCLUDED.is_primary,
    updated_at = NOW();

-- name: GetEmbedding :one
SELECT id, asset_id, embedding_type, embedding_model, embedding_dimensions, vector, is_primary, created_at, updated_at
FROM embeddings
WHERE asset_id = $1 AND embedding_type = $2 AND embedding_model = $3;

-- name: GetEmbeddingByType :one
SELECT id, asset_id, embedding_type, embedding_model, embedding_dimensions, vector, is_primary, created_at, updated_at
FROM embeddings
WHERE asset_id = $1 AND embedding_type = $2
ORDER BY is_primary DESC, created_at DESC
LIMIT 1;

-- name: GetPrimaryEmbedding :one
SELECT id, asset_id, embedding_type, embedding_model, embedding_dimensions, vector, is_primary, created_at, updated_at
FROM embeddings
WHERE asset_id = $1 AND embedding_type = $2 AND is_primary = true;

-- name: GetAllEmbeddingsForAsset :many
SELECT id, asset_id, embedding_type, embedding_model, embedding_dimensions, is_primary, created_at, updated_at
FROM embeddings
WHERE asset_id = $1
ORDER BY embedding_type, is_primary DESC, created_at DESC;

-- name: SearchEmbeddingsByType :many
SELECT a.asset_id, e.embedding_type, e.embedding_model, (e.vector <-> $1::vector) AS distance
FROM embeddings e
JOIN assets a ON e.asset_id = a.asset_id
WHERE e.embedding_type = $2
  AND e.is_primary = true
  AND a.is_deleted = false
ORDER BY (e.vector <-> $1::vector)
LIMIT $3;

-- name: SearchEmbeddingsByModel :many
SELECT a.asset_id, e.embedding_type, e.embedding_model, (e.vector <-> $1::vector) AS distance
FROM embeddings e
JOIN assets a ON e.asset_id = a.asset_id
WHERE e.embedding_type = $2
  AND e.embedding_model = $3
  AND a.is_deleted = false
ORDER BY (e.vector <-> $1::vector)
LIMIT $4;

-- name: SearchAllEmbeddingsByType :many
SELECT a.asset_id, e.embedding_type, e.embedding_model, e.embedding_dimensions, (e.vector <-> $1::vector) AS distance
FROM embeddings e
JOIN assets a ON e.asset_id = a.asset_id
WHERE e.embedding_type = $2
  AND a.is_deleted = false
ORDER BY (e.vector <-> $1::vector)
LIMIT $3;

-- name: SetPrimaryEmbedding :exec
UPDATE embeddings
SET is_primary = false
WHERE embedding_type = $1 AND asset_id != $2;

-- name: SetPrimaryEmbeddingForAsset :exec
UPDATE embeddings
SET is_primary = CASE
    WHEN embedding_model = $3 THEN true
    ELSE false
END,
updated_at = NOW()
WHERE asset_id = $1 AND embedding_type = $2;

-- name: DeleteEmbedding :exec
DELETE FROM embeddings
WHERE asset_id = $1 AND embedding_type = $2 AND embedding_model = $3;

-- name: DeleteAllEmbeddingsForAsset :exec
DELETE FROM embeddings
WHERE asset_id = $1;

-- name: GetEmbeddingModels :many
SELECT DISTINCT embedding_type, embedding_model, embedding_dimensions
FROM embeddings
WHERE embedding_type = $1
ORDER BY embedding_model;

-- name: ListAssetEmbeddings :many
SELECT asset_id, embedding_type, embedding_model, embedding_dimensions, is_primary, created_at
FROM embeddings
WHERE asset_id IN (SELECT unnest($1::uuid[]))
ORDER BY asset_id, embedding_type, is_primary DESC;

-- name: CountEmbeddingsByType :one
SELECT COUNT(*) as count
FROM embeddings
WHERE embedding_type = $1 AND is_primary = true;