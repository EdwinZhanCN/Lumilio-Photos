-- Embedding spaces
-- name: UpsertEmbeddingSpace :one
INSERT INTO embedding_spaces (embedding_type, model_id, dimensions, distance_metric)
VALUES ($1, $2, $3, $4)
ON CONFLICT (embedding_type, model_id, dimensions, distance_metric)
DO UPDATE SET updated_at = NOW()
RETURNING *;

-- name: GetEmbeddingSpaceByAttributes :one
SELECT id, embedding_type, model_id, dimensions, distance_metric, search_enabled, is_default_search, created_at, updated_at
FROM embedding_spaces
WHERE embedding_type = $1
  AND model_id = $2
  AND dimensions = $3
  AND distance_metric = $4;

-- name: GetDefaultEmbeddingSpaceByType :one
SELECT id, embedding_type, model_id, dimensions, distance_metric, search_enabled, is_default_search, created_at, updated_at
FROM embedding_spaces
WHERE embedding_type = $1
  AND is_default_search = true
LIMIT 1;

-- name: PromoteEmbeddingSpaceAsDefaultIfNone :one
UPDATE embedding_spaces AS es
SET search_enabled = true,
    is_default_search = true,
    updated_at = NOW()
WHERE es.id = $1
  AND es.embedding_type = $2
  AND NOT EXISTS (
    SELECT 1
    FROM embedding_spaces existing
    WHERE existing.embedding_type = $2
      AND existing.is_default_search = true
  )
RETURNING es.id, es.embedding_type, es.model_id, es.dimensions, es.distance_metric, es.search_enabled, es.is_default_search, es.created_at, es.updated_at;

-- Unified embeddings table queries
-- name: UpsertEmbedding :exec
INSERT INTO embeddings (asset_id, embedding_type, embedding_model, embedding_dimensions, space_id, vector, is_primary)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (asset_id, embedding_type, embedding_model)
DO UPDATE SET
    space_id = EXCLUDED.space_id,
    vector = EXCLUDED.vector,
    embedding_dimensions = EXCLUDED.embedding_dimensions,
    is_primary = EXCLUDED.is_primary,
    updated_at = NOW();

-- name: GetEmbedding :one
SELECT id, asset_id, embedding_type, embedding_model, embedding_dimensions, space_id, vector, is_primary, created_at, updated_at
FROM embeddings
WHERE asset_id = $1 AND embedding_type = $2 AND embedding_model = $3;

-- name: GetEmbeddingByType :one
SELECT id, asset_id, embedding_type, embedding_model, embedding_dimensions, space_id, vector, is_primary, created_at, updated_at
FROM embeddings
WHERE asset_id = $1 AND embedding_type = $2
ORDER BY is_primary DESC, created_at DESC
LIMIT 1;

-- name: GetPrimaryEmbedding :one
SELECT id, asset_id, embedding_type, embedding_model, embedding_dimensions, space_id, vector, is_primary, created_at, updated_at
FROM embeddings
WHERE asset_id = $1 AND embedding_type = $2 AND is_primary = true;

-- name: GetAllEmbeddingsForAsset :many
SELECT id, asset_id, embedding_type, embedding_model, embedding_dimensions, space_id, is_primary, created_at, updated_at
FROM embeddings
WHERE asset_id = $1
ORDER BY embedding_type, is_primary DESC, created_at DESC;

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
SELECT asset_id, embedding_type, embedding_model, embedding_dimensions, space_id, is_primary, created_at
FROM embeddings
WHERE asset_id IN (SELECT unnest($1::uuid[]))
ORDER BY asset_id, embedding_type, is_primary DESC;

-- name: CountEmbeddingsByType :one
SELECT COUNT(*) as count
FROM embeddings
WHERE embedding_type = $1 AND is_primary = true;
