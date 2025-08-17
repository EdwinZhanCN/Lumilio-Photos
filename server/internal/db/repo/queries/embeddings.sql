-- name: UpsertEmbedding :exec
UPDATE assets
SET embedding = $2
WHERE asset_id = $1;

-- name: SearchNearestAssets :many
SELECT asset_id, (embedding <-> $1::vector) AS distance
FROM assets
WHERE embedding IS NOT NULL
  AND is_deleted = false
ORDER BY (embedding <-> $1::vector)
LIMIT $2;

-- name: GetAssetEmbedding :one
SELECT asset_id, embedding
FROM assets
WHERE asset_id = $1 AND embedding IS NOT NULL;

-- name: GetAssetsWithEmbeddings :many
SELECT asset_id, embedding
FROM assets
WHERE embedding IS NOT NULL
  AND is_deleted = false
ORDER BY upload_time DESC
LIMIT $1 OFFSET $2;
