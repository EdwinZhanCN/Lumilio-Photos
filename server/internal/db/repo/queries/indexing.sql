-- name: CountPhotoAssetsForIndexing :one
SELECT COUNT(*) AS count
FROM assets a
WHERE a.type = 'PHOTO'
  AND a.is_deleted = false
  AND (sqlc.narg('repository_id')::uuid IS NULL OR a.repository_id = sqlc.narg('repository_id'));

-- name: CountPhotoAssetsWithEmbeddingType :one
SELECT COUNT(DISTINCT a.asset_id) AS count
FROM assets a
JOIN embeddings e ON a.asset_id = e.asset_id
WHERE a.type = 'PHOTO'
  AND a.is_deleted = false
  AND e.embedding_type = sqlc.arg('embedding_type')::text
  AND e.is_primary = true
  AND (sqlc.narg('repository_id')::uuid IS NULL OR a.repository_id = sqlc.narg('repository_id'));

-- name: CountPhotoAssetsWithOCRResults :one
SELECT COUNT(DISTINCT a.asset_id) AS count
FROM assets a
JOIN ocr_results o ON a.asset_id = o.asset_id
WHERE a.type = 'PHOTO'
  AND a.is_deleted = false
  AND (sqlc.narg('repository_id')::uuid IS NULL OR a.repository_id = sqlc.narg('repository_id'));

-- name: CountPhotoAssetsWithCaptions :one
SELECT COUNT(DISTINCT a.asset_id) AS count
FROM assets a
JOIN captions c ON a.asset_id = c.asset_id
WHERE a.type = 'PHOTO'
  AND a.is_deleted = false
  AND (sqlc.narg('repository_id')::uuid IS NULL OR a.repository_id = sqlc.narg('repository_id'));

-- name: CountPhotoAssetsWithFaceResults :one
SELECT COUNT(DISTINCT a.asset_id) AS count
FROM assets a
JOIN face_results f ON a.asset_id = f.asset_id
WHERE a.type = 'PHOTO'
  AND a.is_deleted = false
  AND (sqlc.narg('repository_id')::uuid IS NULL OR a.repository_id = sqlc.narg('repository_id'));

-- name: ListPhotoAssetsForIndexingBatch :many
SELECT a.*
FROM assets a
WHERE a.type = 'PHOTO'
  AND a.is_deleted = false
  AND (sqlc.narg('repository_id')::uuid IS NULL OR a.repository_id = sqlc.narg('repository_id'))
ORDER BY COALESCE(a.taken_time, a.upload_time) DESC
LIMIT sqlc.arg('limit')
OFFSET sqlc.arg('offset');

-- name: ListPhotoAssetsMissingEmbeddingType :many
SELECT a.*
FROM assets a
LEFT JOIN embeddings e
  ON a.asset_id = e.asset_id
 AND e.embedding_type = sqlc.arg('embedding_type')::text
 AND e.is_primary = true
WHERE a.type = 'PHOTO'
  AND a.is_deleted = false
  AND e.asset_id IS NULL
  AND (sqlc.narg('repository_id')::uuid IS NULL OR a.repository_id = sqlc.narg('repository_id'))
ORDER BY COALESCE(a.taken_time, a.upload_time) DESC
LIMIT sqlc.arg('limit')
OFFSET sqlc.arg('offset');

-- name: ListPhotoAssetsMissingOCRResults :many
SELECT a.*
FROM assets a
LEFT JOIN ocr_results o ON a.asset_id = o.asset_id
WHERE a.type = 'PHOTO'
  AND a.is_deleted = false
  AND o.asset_id IS NULL
  AND (sqlc.narg('repository_id')::uuid IS NULL OR a.repository_id = sqlc.narg('repository_id'))
ORDER BY COALESCE(a.taken_time, a.upload_time) DESC
LIMIT sqlc.arg('limit')
OFFSET sqlc.arg('offset');

-- name: ListPhotoAssetsMissingCaptions :many
SELECT a.*
FROM assets a
LEFT JOIN captions c ON a.asset_id = c.asset_id
WHERE a.type = 'PHOTO'
  AND a.is_deleted = false
  AND c.asset_id IS NULL
  AND (sqlc.narg('repository_id')::uuid IS NULL OR a.repository_id = sqlc.narg('repository_id'))
ORDER BY COALESCE(a.taken_time, a.upload_time) DESC
LIMIT sqlc.arg('limit')
OFFSET sqlc.arg('offset');

-- name: ListPhotoAssetsMissingFaceResults :many
SELECT a.*
FROM assets a
LEFT JOIN face_results f ON a.asset_id = f.asset_id
WHERE a.type = 'PHOTO'
  AND a.is_deleted = false
  AND f.asset_id IS NULL
  AND (sqlc.narg('repository_id')::uuid IS NULL OR a.repository_id = sqlc.narg('repository_id'))
ORDER BY COALESCE(a.taken_time, a.upload_time) DESC
LIMIT sqlc.arg('limit')
OFFSET sqlc.arg('offset');
