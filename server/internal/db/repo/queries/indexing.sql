-- name: CountPhotoAssetsForIndexing :one
SELECT COUNT(*) AS count
FROM assets a
WHERE a.type = 'PHOTO'
  AND a.is_deleted = false
  AND (sqlc.narg('repository_id')::uuid IS NULL OR a.repository_id = sqlc.narg('repository_id'));

-- name: CountPhotoAssetsWithEmbeddingType :one
SELECT COUNT(*) AS count
FROM assets a
WHERE a.type = 'PHOTO'
  AND a.is_deleted = false
  AND EXISTS (
    SELECT 1
    FROM embeddings e
    WHERE e.asset_id = a.asset_id
      AND e.embedding_type = sqlc.arg('embedding_type')::text
      AND e.is_primary = true
  )
  AND (sqlc.narg('repository_id')::uuid IS NULL OR a.repository_id = sqlc.narg('repository_id'));

-- name: CountPhotoAssetsWithOCRResults :one
SELECT COUNT(*) AS count
FROM assets a
WHERE a.type = 'PHOTO'
  AND a.is_deleted = false
  AND EXISTS (
    SELECT 1
    FROM ocr_results o
    WHERE o.asset_id = a.asset_id
  )
  AND (sqlc.narg('repository_id')::uuid IS NULL OR a.repository_id = sqlc.narg('repository_id'));

-- name: CountPhotoAssetsWithCaptions :one
SELECT COUNT(*) AS count
FROM assets a
WHERE a.type = 'PHOTO'
  AND a.is_deleted = false
  AND EXISTS (
    SELECT 1
    FROM captions c
    WHERE c.asset_id = a.asset_id
  )
  AND (sqlc.narg('repository_id')::uuid IS NULL OR a.repository_id = sqlc.narg('repository_id'));

-- name: CountPhotoAssetsWithFaceResults :one
SELECT COUNT(*) AS count
FROM assets a
WHERE a.type = 'PHOTO'
  AND a.is_deleted = false
  AND EXISTS (
    SELECT 1
    FROM face_results f
    WHERE f.asset_id = a.asset_id
  )
  AND (sqlc.narg('repository_id')::uuid IS NULL OR a.repository_id = sqlc.narg('repository_id'));

-- name: ListPhotoAssetsForIndexingBatch :many
WITH page_ids AS MATERIALIZED (
  SELECT
    a.asset_id,
    COALESCE(a.taken_time, a.upload_time) AS sort_time
  FROM assets a
  WHERE a.type = 'PHOTO'
    AND a.is_deleted = false
    AND (sqlc.narg('repository_id')::uuid IS NULL OR a.repository_id = sqlc.narg('repository_id'))
  ORDER BY COALESCE(a.taken_time, a.upload_time) DESC, a.asset_id DESC
  LIMIT sqlc.arg('limit')
  OFFSET sqlc.arg('offset')
)
SELECT a.*
FROM page_ids p
JOIN assets a ON a.asset_id = p.asset_id
ORDER BY p.sort_time DESC, p.asset_id DESC;

-- name: ListPhotoAssetsMissingEmbeddingType :many
WITH page_ids AS MATERIALIZED (
  SELECT
    a.asset_id,
    COALESCE(a.taken_time, a.upload_time) AS sort_time
  FROM assets a
  WHERE a.type = 'PHOTO'
    AND a.is_deleted = false
    AND NOT EXISTS (
      SELECT 1
      FROM embeddings e
      WHERE e.asset_id = a.asset_id
        AND e.embedding_type = sqlc.arg('embedding_type')::text
        AND e.is_primary = true
    )
    AND (sqlc.narg('repository_id')::uuid IS NULL OR a.repository_id = sqlc.narg('repository_id'))
  ORDER BY COALESCE(a.taken_time, a.upload_time) DESC, a.asset_id DESC
  LIMIT sqlc.arg('limit')
  OFFSET sqlc.arg('offset')
)
SELECT a.*
FROM page_ids p
JOIN assets a ON a.asset_id = p.asset_id
ORDER BY p.sort_time DESC, p.asset_id DESC;

-- name: ListPhotoAssetsMissingOCRResults :many
WITH page_ids AS MATERIALIZED (
  SELECT
    a.asset_id,
    COALESCE(a.taken_time, a.upload_time) AS sort_time
  FROM assets a
  WHERE a.type = 'PHOTO'
    AND a.is_deleted = false
    AND NOT EXISTS (
      SELECT 1
      FROM ocr_results o
      WHERE o.asset_id = a.asset_id
    )
    AND (sqlc.narg('repository_id')::uuid IS NULL OR a.repository_id = sqlc.narg('repository_id'))
  ORDER BY COALESCE(a.taken_time, a.upload_time) DESC, a.asset_id DESC
  LIMIT sqlc.arg('limit')
  OFFSET sqlc.arg('offset')
)
SELECT a.*
FROM page_ids p
JOIN assets a ON a.asset_id = p.asset_id
ORDER BY p.sort_time DESC, p.asset_id DESC;

-- name: ListPhotoAssetsMissingCaptions :many
WITH page_ids AS MATERIALIZED (
  SELECT
    a.asset_id,
    COALESCE(a.taken_time, a.upload_time) AS sort_time
  FROM assets a
  WHERE a.type = 'PHOTO'
    AND a.is_deleted = false
    AND NOT EXISTS (
      SELECT 1
      FROM captions c
      WHERE c.asset_id = a.asset_id
    )
    AND (sqlc.narg('repository_id')::uuid IS NULL OR a.repository_id = sqlc.narg('repository_id'))
  ORDER BY COALESCE(a.taken_time, a.upload_time) DESC, a.asset_id DESC
  LIMIT sqlc.arg('limit')
  OFFSET sqlc.arg('offset')
)
SELECT a.*
FROM page_ids p
JOIN assets a ON a.asset_id = p.asset_id
ORDER BY p.sort_time DESC, p.asset_id DESC;

-- name: ListPhotoAssetsMissingFaceResults :many
WITH page_ids AS MATERIALIZED (
  SELECT
    a.asset_id,
    COALESCE(a.taken_time, a.upload_time) AS sort_time
  FROM assets a
  WHERE a.type = 'PHOTO'
    AND a.is_deleted = false
    AND NOT EXISTS (
      SELECT 1
      FROM face_results f
      WHERE f.asset_id = a.asset_id
    )
    AND (sqlc.narg('repository_id')::uuid IS NULL OR a.repository_id = sqlc.narg('repository_id'))
  ORDER BY COALESCE(a.taken_time, a.upload_time) DESC, a.asset_id DESC
  LIMIT sqlc.arg('limit')
  OFFSET sqlc.arg('offset')
)
SELECT a.*
FROM page_ids p
JOIN assets a ON a.asset_id = p.asset_id
ORDER BY p.sort_time DESC, p.asset_id DESC;
