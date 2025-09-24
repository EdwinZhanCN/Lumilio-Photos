-- name: CreateAsset :one
INSERT INTO assets (
    owner_id, type, original_filename, storage_path, mime_type,
    file_size, hash, width, height, duration, specific_metadata
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: GetAssetByID :one
SELECT * FROM assets
WHERE asset_id = $1 AND is_deleted = false;

-- name: GetAssetsByOwner :many
SELECT * FROM assets
WHERE owner_id = $1 AND is_deleted = false
ORDER BY upload_time DESC
LIMIT $2 OFFSET $3;

-- name: GetAssetsByType :many
SELECT * FROM assets
WHERE type = $1 AND is_deleted = false
ORDER BY upload_time DESC
LIMIT $2 OFFSET $3;

-- name: UpdateAsset :one
UPDATE assets
SET original_filename = $2, specific_metadata = $3, updated_at = CURRENT_TIMESTAMP
WHERE asset_id = $1
RETURNING *;

-- name: UpdateAssetMetadata :exec
UPDATE assets
SET specific_metadata = $2
WHERE asset_id = $1;

-- name: DeleteAsset :exec
UPDATE assets
SET is_deleted = true, deleted_at = CURRENT_TIMESTAMP
WHERE asset_id = $1;

-- name: SearchAssets :many
SELECT * FROM assets
WHERE is_deleted = false
AND ($1::text IS NULL OR original_filename ILIKE '%' || $1 || '%')
AND ($2::text IS NULL OR type = $2)
ORDER BY upload_time DESC
LIMIT $3 OFFSET $4;

-- name: GetAssetsByHash :many
SELECT * FROM assets
WHERE hash = $1 AND is_deleted = false;

-- name: CreateThumbnail :one
INSERT INTO thumbnails (asset_id, size, storage_path, mime_type)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetThumbnailByID :one
SELECT * FROM thumbnails WHERE thumbnail_id = $1;

-- name: GetThumbnailByAssetAndSize :one
SELECT * FROM thumbnails
WHERE asset_id = $1 AND size = $2;

-- name: GetThumbnailsByAsset :many
SELECT * FROM thumbnails
WHERE asset_id = $1
ORDER BY CASE size
    WHEN 'small' THEN 1
    WHEN 'medium' THEN 2
    WHEN 'large' THEN 3
END, thumbnail_id;

-- name: AddAssetToAlbum :exec
INSERT INTO album_assets (asset_id, album_id, position)
VALUES ($1, $2, $3)
ON CONFLICT (asset_id, album_id) DO NOTHING;

-- name: RemoveAssetFromAlbum :exec
DELETE FROM album_assets
WHERE asset_id = $1 AND album_id = $2;

-- name: AddTagToAsset :exec
INSERT INTO asset_tags (asset_id, tag_id, confidence, source)
VALUES ($1, $2, $3, $4)
ON CONFLICT (asset_id, tag_id) DO UPDATE
SET confidence = $3, source = $4;

-- name: RemoveTagFromAsset :exec
DELETE FROM asset_tags
WHERE asset_id = $1 AND tag_id = $2;

-- name: FilterAssets :many
SELECT a.* FROM assets a
WHERE a.is_deleted = false
  AND (sqlc.narg('asset_type')::text IS NULL OR a.type = sqlc.narg('asset_type'))
  AND (sqlc.narg('owner_id')::integer IS NULL OR a.owner_id = sqlc.narg('owner_id'))
  AND (sqlc.narg('filename_val')::text IS NULL OR
    CASE sqlc.narg('filename_mode')::text
      WHEN 'contains' THEN a.original_filename ILIKE '%' || sqlc.narg('filename_val') || '%'
      WHEN 'matches' THEN a.original_filename ILIKE sqlc.narg('filename_val')
      WHEN 'startswith' THEN a.original_filename ILIKE sqlc.narg('filename_val') || '%'
      WHEN 'endswith' THEN a.original_filename ILIKE '%' || sqlc.narg('filename_val')
      ELSE true
    END
  )
  AND (sqlc.narg('date_from')::timestamptz IS NULL OR a.upload_time >= sqlc.narg('date_from'))
  AND (sqlc.narg('date_to')::timestamptz IS NULL OR a.upload_time <= sqlc.narg('date_to'))
  AND (sqlc.narg('is_raw')::boolean IS NULL OR
    CASE
      WHEN sqlc.narg('is_raw') = true THEN (a.specific_metadata->>'is_raw')::boolean = true
      WHEN sqlc.narg('is_raw') = false THEN (a.specific_metadata->>'is_raw')::boolean = false OR a.specific_metadata->>'is_raw' IS NULL
      ELSE true
    END
  )
  AND (sqlc.narg('rating')::integer IS NULL OR
    CASE
      WHEN sqlc.narg('rating') = 0 THEN a.specific_metadata->>'rating' IS NULL
      ELSE (a.specific_metadata->>'rating')::integer = sqlc.narg('rating')
    END
  )
  AND (sqlc.narg('liked')::boolean IS NULL OR (a.specific_metadata->>'liked')::boolean = sqlc.narg('liked'))
  AND (sqlc.narg('camera_model')::text IS NULL OR a.specific_metadata->>'camera_model' = sqlc.narg('camera_model'))
  AND (sqlc.narg('lens_model')::text IS NULL OR a.specific_metadata->>'lens_model' = sqlc.narg('lens_model'))
ORDER BY a.upload_time DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: SearchAssetsFilename :many
SELECT a.* FROM assets a
WHERE a.is_deleted = false
  AND a.original_filename ILIKE '%' || sqlc.arg('query') || '%'
  AND (sqlc.narg('asset_type')::text IS NULL OR a.type = sqlc.narg('asset_type'))
  AND (sqlc.narg('owner_id')::integer IS NULL OR a.owner_id = sqlc.narg('owner_id'))
  AND (sqlc.narg('filename_val')::text IS NULL OR
    CASE sqlc.narg('filename_mode')::text
      WHEN 'contains' THEN a.original_filename ILIKE '%' || sqlc.narg('filename_val') || '%'
      WHEN 'matches' THEN a.original_filename ILIKE sqlc.narg('filename_val')
      WHEN 'startswith' THEN a.original_filename ILIKE sqlc.narg('filename_val') || '%'
      WHEN 'endswith' THEN a.original_filename ILIKE '%' || sqlc.narg('filename_val')
      ELSE true
    END
  )
  AND (sqlc.narg('date_from')::timestamptz IS NULL OR a.upload_time >= sqlc.narg('date_from'))
  AND (sqlc.narg('date_to')::timestamptz IS NULL OR a.upload_time <= sqlc.narg('date_to'))
  AND (sqlc.narg('is_raw')::boolean IS NULL OR
    CASE
      WHEN sqlc.narg('is_raw') = true THEN (a.specific_metadata->>'is_raw')::boolean = true
      WHEN sqlc.narg('is_raw') = false THEN (a.specific_metadata->>'is_raw')::boolean = false OR a.specific_metadata->>'is_raw' IS NULL
      ELSE true
    END
  )
  AND (sqlc.narg('rating')::integer IS NULL OR
    CASE
      WHEN sqlc.narg('rating') = 0 THEN a.specific_metadata->>'rating' IS NULL
      ELSE (a.specific_metadata->>'rating')::integer = sqlc.narg('rating')
    END
  )
  AND (sqlc.narg('liked')::boolean IS NULL OR (a.specific_metadata->>'liked')::boolean = sqlc.narg('liked'))
  AND (sqlc.narg('camera_model')::text IS NULL OR a.specific_metadata->>'camera_model' = sqlc.narg('camera_model'))
  AND (sqlc.narg('lens_model')::text IS NULL OR a.specific_metadata->>'lens_model' = sqlc.narg('lens_model'))
ORDER BY a.upload_time DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: SearchAssetsVector :many
SELECT a.*, (a.embedding <-> sqlc.arg('embedding')::vector) AS distance FROM assets a
WHERE a.is_deleted = false
  AND a.embedding IS NOT NULL
  AND (sqlc.narg('asset_type')::text IS NULL OR a.type = sqlc.narg('asset_type'))
  AND (sqlc.narg('owner_id')::integer IS NULL OR a.owner_id = sqlc.narg('owner_id'))
  AND (sqlc.narg('filename_val')::text IS NULL OR
    CASE sqlc.narg('filename_mode')::text
      WHEN 'contains' THEN a.original_filename ILIKE '%' || sqlc.narg('filename_val') || '%'
      WHEN 'matches' THEN a.original_filename ILIKE sqlc.narg('filename_val')
      WHEN 'startswith' THEN a.original_filename ILIKE sqlc.narg('filename_val') || '%'
      WHEN 'endswith' THEN a.original_filename ILIKE '%' || sqlc.narg('filename_val')
      ELSE true
    END
  )
  AND (sqlc.narg('date_from')::timestamptz IS NULL OR a.upload_time >= sqlc.narg('date_from'))
  AND (sqlc.narg('date_to')::timestamptz IS NULL OR a.upload_time <= sqlc.narg('date_to'))
  AND (sqlc.narg('is_raw')::boolean IS NULL OR
    CASE
      WHEN sqlc.narg('is_raw') = true THEN (a.specific_metadata->>'is_raw')::boolean = true
      WHEN sqlc.narg('is_raw') = false THEN (a.specific_metadata->>'is_raw')::boolean = false OR a.specific_metadata->>'is_raw' IS NULL
      ELSE true
    END
  )
  AND (sqlc.narg('rating')::integer IS NULL OR
    CASE
      WHEN sqlc.narg('rating') = 0 THEN a.specific_metadata->>'rating' IS NULL
      ELSE (a.specific_metadata->>'rating')::integer = sqlc.narg('rating')
    END
  )
  AND (sqlc.narg('liked')::boolean IS NULL OR (a.specific_metadata->>'liked')::boolean = sqlc.narg('liked'))
  AND (sqlc.narg('camera_model')::text IS NULL OR a.specific_metadata->>'camera_model' = sqlc.narg('camera_model'))
  AND (sqlc.narg('lens_model')::text IS NULL OR a.specific_metadata->>'lens_model' = sqlc.narg('lens_model'))
ORDER BY (a.embedding <-> sqlc.arg('embedding')::vector)
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: GetDistinctCameraMakes :many
SELECT DISTINCT a.specific_metadata->>'camera_model' as camera_model
FROM assets a
WHERE a.is_deleted = false
  AND a.specific_metadata->>'camera_model' IS NOT NULL
  AND a.specific_metadata->>'camera_model' != ''
ORDER BY camera_model;

-- name: GetDistinctLenses :many
SELECT DISTINCT a.specific_metadata->>'lens_model' as lens_model
FROM assets a
WHERE a.is_deleted = false
  AND a.specific_metadata->>'lens_model' IS NOT NULL
  AND a.specific_metadata->>'lens_model' != ''
ORDER BY lens_model;

-- name: UpdateAssetRating :exec
UPDATE assets
SET specific_metadata = jsonb_set(
    COALESCE(specific_metadata, '{}'::jsonb),
    '{rating}',
    to_jsonb(sqlc.arg('rating')::integer)
)
WHERE asset_id = sqlc.arg('asset_id');

-- name: UpdateAssetLike :exec
UPDATE assets
SET specific_metadata = jsonb_set(
    COALESCE(specific_metadata, '{}'::jsonb),
    '{liked}',
    to_jsonb(sqlc.arg('liked')::boolean)
)
WHERE asset_id = sqlc.arg('asset_id');

-- name: UpdateAssetRatingAndLike :exec
UPDATE assets
SET specific_metadata = jsonb_set(
    jsonb_set(
        COALESCE(specific_metadata, '{}'::jsonb),
        '{rating}',
        to_jsonb(sqlc.arg('rating')::integer)
    ),
    '{liked}',
    to_jsonb(sqlc.arg('liked')::boolean)
)
WHERE asset_id = sqlc.arg('asset_id');

-- name: UpdateAssetDescription :exec
UPDATE assets
SET specific_metadata = jsonb_set(
    COALESCE(specific_metadata, '{}'::jsonb),
    '{description}',
    to_jsonb(sqlc.arg('description')::text)
)
WHERE asset_id = sqlc.arg('asset_id');

-- name: GetAssetsByRating :many
SELECT * FROM assets
WHERE is_deleted = false
  AND (specific_metadata->>'rating')::integer = sqlc.arg('rating')::integer
ORDER BY upload_time DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: GetLikedAssets :many
SELECT * FROM assets
WHERE is_deleted = false
  AND (specific_metadata->>'liked')::boolean = true
ORDER BY upload_time DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');
