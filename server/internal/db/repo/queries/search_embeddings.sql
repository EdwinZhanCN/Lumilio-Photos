-- Dedicated fixed-dimension semantic search vectors (see migration 000012).
-- Photos have one row (frame_ts_ms IS NULL); videos have one row per frame.

-- name: InsertSearchEmbedding :exec
INSERT INTO search_embeddings (asset_id, space_id, frame_ts_ms, vector, model_id)
VALUES ($1, $2, $3, $4, $5);

-- name: DeleteSearchEmbeddingsByAsset :exec
DELETE FROM search_embeddings
WHERE asset_id = $1;

-- name: DeleteAllSearchEmbeddings :exec
DELETE FROM search_embeddings;

-- name: GetPrimarySearchEmbedding :one
SELECT asset_id, space_id, vector, model_id
FROM search_embeddings
WHERE asset_id = $1 AND frame_ts_ms IS NULL;

-- name: CountAssetsWithSearchEmbedding :one
SELECT COUNT(DISTINCT asset_id) AS count
FROM search_embeddings;
