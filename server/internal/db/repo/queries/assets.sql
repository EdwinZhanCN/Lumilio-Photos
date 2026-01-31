-- name: CreateAsset :one
INSERT INTO assets (
    owner_id, type, original_filename, storage_path, mime_type,
    file_size, hash, width, height, duration, taken_time, specific_metadata, rating, liked, repository_id, status
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
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

-- name: UpdateAssetMetadataWithTakenTime :exec
UPDATE assets
SET specific_metadata = sqlc.arg('specific_metadata'),
    taken_time = CASE
        WHEN sqlc.arg('taken_time')::timestamptz IS NOT NULL THEN sqlc.arg('taken_time')::timestamptz
        ELSE COALESCE(taken_time, upload_time)
    END
WHERE asset_id = sqlc.arg('asset_id');

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

-- name: UpdateAssetStatus :one
UPDATE assets
SET status = $2
WHERE asset_id = $1
RETURNING *;

-- name: UpdateAssetStoragePathAndStatus :one
UPDATE assets
SET
    storage_path = $2,
    status = $3
WHERE asset_id = $1
RETURNING *;

-- name: GetAssetsByStatus :many
SELECT * FROM assets
WHERE status->>'state' = $1 AND is_deleted = false
ORDER BY upload_time DESC
LIMIT $2 OFFSET $3;

-- name: GetAssetsWithWarnings :many
SELECT * FROM assets
WHERE status->>'state' = 'warning' AND is_deleted = false
ORDER BY upload_time DESC
LIMIT $1 OFFSET $2;

-- name: GetAssetsWithErrors :many
SELECT * FROM assets
WHERE status->>'state' = 'failed' AND is_deleted = false
ORDER BY upload_time DESC
LIMIT $1 OFFSET $2;

-- name: GetAssetsByStatusAndRepository :many
SELECT * FROM assets
WHERE status->>'state' = $1 AND repository_id = $2 AND is_deleted = false
ORDER BY upload_time DESC
LIMIT $3 OFFSET $4;

-- name: GetAssetsByStatusAndOwner :many
SELECT * FROM assets
WHERE status->>'state' = $1 AND owner_id = $2 AND is_deleted = false
ORDER BY upload_time DESC
LIMIT $3 OFFSET $4;

-- name: CountAssetsByStatus :one
SELECT COUNT(*) as count
FROM assets
WHERE status->>'state' = $1 AND is_deleted = false;

-- name: CountAssetsByStatusAndRepository :one
SELECT COUNT(*) as count
FROM assets
WHERE status->>'state' = $1 AND repository_id = $2 AND is_deleted = false;

-- name: CountAssetsByStatusAndOwner :one
SELECT COUNT(*) as count
FROM assets
WHERE status->>'state' = $1 AND owner_id = $2 AND is_deleted = false;

-- name: ResetAssetStatusForRetry :one
UPDATE assets
SET status = jsonb_set(
    status,
    '{state}',
    '"processing"'
)
WHERE asset_id = $1 AND status->>'state' IN ('warning', 'failed')
RETURNING *;

-- name: UpdateAssetStatusWithErrors :one
UPDATE assets
SET status = $2
WHERE asset_id = $1
RETURNING *;

-- name: BulkUpdateAssetStatus :exec
UPDATE assets
SET status = $2
WHERE asset_id = ANY($1::uuid[])
  AND is_deleted = false;

-- name: GetAssetsByHash :many
SELECT * FROM assets
WHERE hash = $1 AND is_deleted = false;

-- name: GetAssetByHashAndRepository :one
SELECT * FROM assets
WHERE hash = $1 AND repository_id = $2 AND is_deleted = false;

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
LEFT JOIN album_assets aa ON a.asset_id = aa.asset_id
WHERE a.is_deleted = false
  AND (sqlc.narg('asset_type')::text IS NULL OR a.type = sqlc.narg('asset_type'))
  AND (sqlc.narg('owner_id')::integer IS NULL OR a.owner_id = sqlc.narg('owner_id'))
  AND (sqlc.narg('repository_id')::uuid IS NULL OR a.repository_id = sqlc.narg('repository_id'))
  AND (sqlc.narg('album_id')::integer IS NULL OR aa.album_id = sqlc.narg('album_id'))
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
      WHEN sqlc.narg('rating') = 0 THEN a.rating IS NULL
      ELSE a.rating = sqlc.narg('rating')
    END
  )
  AND (sqlc.narg('liked')::boolean IS NULL OR a.liked = sqlc.narg('liked'))
  AND (sqlc.narg('camera_model')::text IS NULL OR a.specific_metadata->>'camera_model' = sqlc.narg('camera_model'))
  AND (sqlc.narg('lens_model')::text IS NULL OR a.specific_metadata->>'lens_model' = sqlc.narg('lens_model'))
ORDER BY a.upload_time DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: SearchAssetsFilename :many
SELECT a.* FROM assets a
LEFT JOIN album_assets aa ON a.asset_id = aa.asset_id
WHERE a.is_deleted = false
  AND a.original_filename ILIKE '%' || sqlc.arg('query') || '%'
  AND (sqlc.narg('asset_type')::text IS NULL OR a.type = sqlc.narg('asset_type'))
  AND (sqlc.narg('owner_id')::integer IS NULL OR a.owner_id = sqlc.narg('owner_id'))
  AND (sqlc.narg('repository_id')::uuid IS NULL OR a.repository_id = sqlc.narg('repository_id'))
  AND (sqlc.narg('album_id')::integer IS NULL OR aa.album_id = sqlc.narg('album_id'))
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
      WHEN sqlc.narg('rating') = 0 THEN a.rating IS NULL
      ELSE a.rating = sqlc.narg('rating')
    END
  )
  AND (sqlc.narg('liked')::boolean IS NULL OR a.liked = sqlc.narg('liked'))
  AND (sqlc.narg('camera_model')::text IS NULL OR a.specific_metadata->>'camera_model' = sqlc.narg('camera_model'))
  AND (sqlc.narg('lens_model')::text IS NULL OR a.specific_metadata->>'lens_model' = sqlc.narg('lens_model'))
ORDER BY a.upload_time DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: SearchAssetsVector :many
SELECT a.*, (e.embedding <-> sqlc.arg('embedding')::vector) AS distance
FROM assets a
JOIN embeddings e ON a.asset_id = e.asset_id
LEFT JOIN album_assets aa ON a.asset_id = aa.asset_id
WHERE a.is_deleted = false
  AND e.embedding_type = sqlc.arg('embedding_type')::text
  AND e.is_primary = true
  AND (sqlc.narg('asset_type')::text IS NULL OR a.type = sqlc.narg('asset_type'))
  AND (sqlc.narg('owner_id')::integer IS NULL OR a.owner_id = sqlc.narg('owner_id'))
  AND (sqlc.narg('repository_id')::uuid IS NULL OR a.repository_id = sqlc.narg('repository_id'))
  AND (sqlc.narg('album_id')::integer IS NULL OR aa.album_id = sqlc.narg('album_id'))
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
      WHEN sqlc.narg('rating') = 0 THEN a.rating IS NULL
      ELSE a.rating = sqlc.narg('rating')
    END
  )
  AND (sqlc.narg('liked')::boolean IS NULL OR a.liked = sqlc.narg('liked'))
  AND (sqlc.narg('camera_model')::text IS NULL OR a.specific_metadata->>'camera_model' = sqlc.narg('camera_model'))
  AND (sqlc.narg('lens_model')::text IS NULL OR a.specific_metadata->>'lens_model' = sqlc.narg('lens_model'))
  AND (e.embedding <-> sqlc.arg('embedding')::vector) <= sqlc.narg('max_distance')::float8
ORDER BY (e.embedding <-> sqlc.arg('embedding')::vector)
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
SET rating = sqlc.arg('rating')::integer
WHERE asset_id = sqlc.arg('asset_id');

-- name: UpdateAssetLike :exec
UPDATE assets
SET liked = sqlc.arg('liked')::boolean
WHERE asset_id = sqlc.arg('asset_id');

-- name: UpdateAssetRatingAndLike :exec
UPDATE assets
SET rating = sqlc.arg('rating')::integer,
    liked = sqlc.arg('liked')::boolean
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
  AND rating = sqlc.arg('rating')::integer
ORDER BY upload_time DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: GetLikedAssets :many
SELECT * FROM assets
WHERE is_deleted = false
  AND liked = true
ORDER BY upload_time DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: GetAssetsByOwnerSorted :many
SELECT * FROM assets
WHERE owner_id = $1 AND is_deleted = false
ORDER BY
  CASE WHEN $2 = 'asc' THEN COALESCE(taken_time, upload_time) END ASC,
  CASE WHEN $2 = 'desc' THEN COALESCE(taken_time, upload_time) END DESC
LIMIT $3 OFFSET $4;

-- name: GetAssetsByTypesSorted :many
SELECT * FROM assets
WHERE type = ANY(sqlc.arg('types')::text[]) AND is_deleted = false
ORDER BY
  CASE WHEN sqlc.arg('sort_order') = 'asc' THEN COALESCE(taken_time, upload_time) END ASC,
  CASE WHEN sqlc.arg('sort_order') = 'desc' THEN COALESCE(taken_time, upload_time) END DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: GetAssetsByOwnerAndTypesSorted :many
SELECT * FROM assets
WHERE owner_id = $1 AND type = ANY(sqlc.arg('types')::text[]) AND is_deleted = false
ORDER BY
  CASE WHEN sqlc.arg('sort_order') = 'asc' THEN COALESCE(taken_time, upload_time) END ASC,
  CASE WHEN sqlc.arg('sort_order') = 'desc' THEN COALESCE(taken_time, upload_time) END DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: UpdateAssetDuration :exec
UPDATE assets
SET duration = $2
WHERE asset_id = $1;

-- name: UpdateAssetDimensions :exec
UPDATE assets
SET width = $2, height = $3
WHERE asset_id = $1;

-- name: GetAssetsByRatingRange :many
SELECT * FROM assets
WHERE is_deleted = false
  AND rating IS NOT NULL
  AND rating >= sqlc.arg('min_rating')::integer
  AND rating <= sqlc.arg('max_rating')::integer
  AND (sqlc.narg('owner_id')::integer IS NULL OR owner_id = sqlc.narg('owner_id'))
ORDER BY rating DESC, upload_time DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: GetLikedAssetsByOwner :many
SELECT * FROM assets
WHERE is_deleted = false
  AND liked = true
  AND owner_id = sqlc.arg('owner_id')::integer
ORDER BY upload_time DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: GetTopRatedAssets :many
SELECT * FROM assets
WHERE is_deleted = false
  AND rating IS NOT NULL
  AND rating >= sqlc.arg('min_rating')::integer
  AND (sqlc.narg('owner_id')::integer IS NULL OR owner_id = sqlc.narg('owner_id'))
ORDER BY rating DESC, upload_time DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: GetAssetsByRatingAndType :many
SELECT * FROM assets
WHERE is_deleted = false
  AND rating = sqlc.arg('rating')::integer
  AND type = sqlc.arg('asset_type')::text
  AND (sqlc.narg('owner_id')::integer IS NULL OR owner_id = sqlc.narg('owner_id'))
ORDER BY upload_time DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: GetLikedAssetsByType :many
SELECT * FROM assets
WHERE is_deleted = false
  AND liked = true
  AND type = sqlc.arg('asset_type')::text
  AND (sqlc.narg('owner_id')::integer IS NULL OR owner_id = sqlc.narg('owner_id'))
ORDER BY upload_time DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountAssetsByRating :many
SELECT rating, COUNT(*) as count
FROM assets
WHERE is_deleted = false
  AND rating IS NOT NULL
  AND (sqlc.narg('owner_id')::integer IS NULL OR owner_id = sqlc.narg('owner_id'))
GROUP BY rating
ORDER BY rating DESC;

-- name: CountLikedAssets :one
SELECT COUNT(*) as count
FROM assets
WHERE is_deleted = false
  AND liked = true
  AND (sqlc.narg('owner_id')::integer IS NULL OR owner_id = sqlc.narg('owner_id'));

-- name: GetAssetStatsForOwner :one
SELECT
  COUNT(*) as total_assets,
  COUNT(CASE WHEN liked = true THEN 1 END) as liked_count,
  COUNT(CASE WHEN rating IS NOT NULL THEN 1 END) as rated_count,
  AVG(rating) as avg_rating,
  MAX(rating) as max_rating,
  MIN(rating) as min_rating
FROM assets
WHERE is_deleted = false
  AND owner_id = sqlc.arg('owner_id')::integer;

-- name: BulkUpdateAssetRating :exec
UPDATE assets
SET rating = sqlc.arg('rating')::integer
WHERE asset_id = ANY(sqlc.arg('asset_ids')::uuid[])
  AND is_deleted = false;

-- name: BulkUpdateAssetLiked :exec
UPDATE assets
SET liked = sqlc.arg('liked')::boolean
WHERE asset_id = ANY(sqlc.arg('asset_ids')::uuid[])
  AND is_deleted = false;

-- name: BulkToggleAssetLiked :exec
UPDATE assets
SET liked = NOT liked
WHERE asset_id = ANY(sqlc.arg('asset_ids')::uuid[])
  AND is_deleted = false;

-- name: GetAssetsByOwnerWithRatingLiked :many
SELECT * FROM assets
WHERE owner_id = sqlc.arg('owner_id')::integer
  AND is_deleted = false
  AND (sqlc.narg('has_rating')::boolean IS NULL OR
       (sqlc.narg('has_rating') = true AND rating IS NOT NULL) OR
       (sqlc.narg('has_rating') = false AND rating IS NULL))
  AND (sqlc.narg('is_liked')::boolean IS NULL OR liked = sqlc.narg('is_liked'))
ORDER BY
  CASE WHEN sqlc.arg('sort_by') = 'rating' THEN rating END DESC NULLS LAST,
  CASE WHEN sqlc.arg('sort_by') = 'upload_time' THEN upload_time END DESC,
  CASE WHEN sqlc.arg('sort_by') = 'taken_time' THEN COALESCE(taken_time, upload_time) END DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- Repository Asset Statistics (kept for repository management)

-- name: GetRepositoryAssetStats :one
SELECT
  COUNT(*) as total_assets,
  COUNT(CASE WHEN type = 'PHOTO' THEN 1 END) as photo_count,
  COUNT(CASE WHEN type = 'VIDEO' THEN 1 END) as video_count,
  COUNT(CASE WHEN type = 'AUDIO' THEN 1 END) as audio_count,
  COUNT(CASE WHEN liked = true THEN 1 END) as liked_count,
  COUNT(CASE WHEN rating IS NOT NULL THEN 1 END) as rated_count,
  AVG(rating) as avg_rating,
  SUM(file_size) as total_size,
  MIN(upload_time) as oldest_upload,
  MAX(upload_time) as newest_upload
FROM assets
WHERE is_deleted = false
  AND repository_id = sqlc.arg('repository_id')::uuid
  AND (sqlc.narg('owner_id')::integer IS NULL OR owner_id = sqlc.narg('owner_id'));

-- ============================================================================
-- UNIFIED QUERY API
-- These queries consolidate List, Filter, and Search operations with shared WHERE logic
-- ============================================================================

-- name: GetAssetsUnified :many
-- Handles: listing, filename search, and all filtering
-- Use this for most queries unless semantic search is needed
SELECT a.* FROM assets a
LEFT JOIN album_assets aa ON a.asset_id = aa.asset_id
WHERE a.is_deleted = false
  AND (sqlc.narg('query')::text IS NULL OR a.original_filename ILIKE '%' || sqlc.narg('query') || '%')
  AND (sqlc.narg('asset_type')::text IS NULL OR a.type = sqlc.narg('asset_type'))
  AND (sqlc.narg('asset_types')::text[] IS NULL OR a.type = ANY(sqlc.narg('asset_types')::text[]))
  AND (sqlc.narg('owner_id')::integer IS NULL OR a.owner_id = sqlc.narg('owner_id'))
  AND (sqlc.narg('repository_id')::uuid IS NULL OR a.repository_id = sqlc.narg('repository_id'))
  AND (sqlc.narg('album_id')::integer IS NULL OR aa.album_id = sqlc.narg('album_id'))
  AND (sqlc.narg('date_from')::timestamptz IS NULL OR COALESCE(a.taken_time, a.upload_time) >= sqlc.narg('date_from'))
  AND (sqlc.narg('date_to')::timestamptz IS NULL OR COALESCE(a.taken_time, a.upload_time) <= sqlc.narg('date_to'))
  AND (sqlc.narg('is_raw')::boolean IS NULL OR
    CASE
      WHEN sqlc.narg('is_raw') = true THEN (a.specific_metadata->>'is_raw')::boolean = true
      ELSE (a.specific_metadata->>'is_raw')::boolean = false OR a.specific_metadata->>'is_raw' IS NULL
    END
  )
  AND (sqlc.narg('rating')::integer IS NULL OR
    CASE
      WHEN sqlc.narg('rating') = 0 THEN a.rating IS NULL
      ELSE a.rating = sqlc.narg('rating')
    END
  )
  AND (sqlc.narg('liked')::boolean IS NULL OR a.liked = sqlc.narg('liked'))
  AND (sqlc.narg('camera_model')::text IS NULL OR a.specific_metadata->>'camera_model' = sqlc.narg('camera_model'))
  AND (sqlc.narg('lens_model')::text IS NULL OR a.specific_metadata->>'lens_model' = sqlc.narg('lens_model'))
ORDER BY
  -- When sorting by type, use mime_type for actual file format grouping
  -- This ensures image/jpeg, image/png, image/heic etc. are grouped together
  CASE WHEN sqlc.narg('sort_by')::text = 'type' THEN a.mime_type END ASC,
  COALESCE(a.taken_time, a.upload_time) DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: SearchAssetsVectorUnified :many
-- Handles: semantic vector search with all filtering
-- Same WHERE clause as GetAssetsUnified for consistency
SELECT a.*, (e.embedding <-> sqlc.arg('embedding')::vector) AS distance
FROM assets a
JOIN embeddings e ON a.asset_id = e.asset_id
LEFT JOIN album_assets aa ON a.asset_id = aa.asset_id
WHERE a.is_deleted = false
  AND e.embedding_type = sqlc.arg('embedding_type')::text
  AND e.is_primary = true
  AND (sqlc.narg('asset_type')::text IS NULL OR a.type = sqlc.narg('asset_type'))
  AND (sqlc.narg('asset_types')::text[] IS NULL OR a.type = ANY(sqlc.narg('asset_types')::text[]))
  AND (sqlc.narg('owner_id')::integer IS NULL OR a.owner_id = sqlc.narg('owner_id'))
  AND (sqlc.narg('repository_id')::uuid IS NULL OR a.repository_id = sqlc.narg('repository_id'))
  AND (sqlc.narg('album_id')::integer IS NULL OR aa.album_id = sqlc.narg('album_id'))
  AND (sqlc.narg('date_from')::timestamptz IS NULL OR COALESCE(a.taken_time, a.upload_time) >= sqlc.narg('date_from'))
  AND (sqlc.narg('date_to')::timestamptz IS NULL OR COALESCE(a.taken_time, a.upload_time) <= sqlc.narg('date_to'))
  AND (sqlc.narg('is_raw')::boolean IS NULL OR
    CASE
      WHEN sqlc.narg('is_raw') = true THEN (a.specific_metadata->>'is_raw')::boolean = true
      ELSE (a.specific_metadata->>'is_raw')::boolean = false OR a.specific_metadata->>'is_raw' IS NULL
    END
  )
  AND (sqlc.narg('rating')::integer IS NULL OR
    CASE
      WHEN sqlc.narg('rating') = 0 THEN a.rating IS NULL
      ELSE a.rating = sqlc.narg('rating')
    END
  )
  AND (sqlc.narg('liked')::boolean IS NULL OR a.liked = sqlc.narg('liked'))
  AND (sqlc.narg('camera_model')::text IS NULL OR a.specific_metadata->>'camera_model' = sqlc.narg('camera_model'))
  AND (sqlc.narg('lens_model')::text IS NULL OR a.specific_metadata->>'lens_model' = sqlc.narg('lens_model'))
  AND (e.embedding <-> sqlc.arg('embedding')::vector) <= sqlc.narg('max_distance')::float8
ORDER BY (e.embedding <-> sqlc.arg('embedding')::vector)
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountAssetsUnified :one
-- Count query matching GetAssetsUnified WHERE clause
-- Returns total count of assets matching the filters (for pagination)
SELECT COUNT(DISTINCT a.asset_id) as count
FROM assets a
LEFT JOIN album_assets aa ON a.asset_id = aa.asset_id
WHERE a.is_deleted = false
  AND (sqlc.narg('query')::text IS NULL OR a.original_filename ILIKE '%' || sqlc.narg('query') || '%')
  AND (sqlc.narg('asset_type')::text IS NULL OR a.type = sqlc.narg('asset_type'))
  AND (sqlc.narg('asset_types')::text[] IS NULL OR a.type = ANY(sqlc.narg('asset_types')::text[]))
  AND (sqlc.narg('owner_id')::integer IS NULL OR a.owner_id = sqlc.narg('owner_id'))
  AND (sqlc.narg('repository_id')::uuid IS NULL OR a.repository_id = sqlc.narg('repository_id'))
  AND (sqlc.narg('album_id')::integer IS NULL OR aa.album_id = sqlc.narg('album_id'))
  AND (sqlc.narg('date_from')::timestamptz IS NULL OR COALESCE(a.taken_time, a.upload_time) >= sqlc.narg('date_from'))
  AND (sqlc.narg('date_to')::timestamptz IS NULL OR COALESCE(a.taken_time, a.upload_time) <= sqlc.narg('date_to'))
  AND (sqlc.narg('is_raw')::boolean IS NULL OR
    CASE
      WHEN sqlc.narg('is_raw') = true THEN (a.specific_metadata->>'is_raw')::boolean = true
      ELSE (a.specific_metadata->>'is_raw')::boolean = false OR a.specific_metadata->>'is_raw' IS NULL
    END
  )
  AND (sqlc.narg('rating')::integer IS NULL OR
    CASE
      WHEN sqlc.narg('rating') = 0 THEN a.rating IS NULL
      ELSE a.rating = sqlc.narg('rating')
    END
  )
  AND (sqlc.narg('liked')::boolean IS NULL OR a.liked = sqlc.narg('liked'))
  AND (sqlc.narg('camera_model')::text IS NULL OR a.specific_metadata->>'camera_model' = sqlc.narg('camera_model'))
  AND (sqlc.narg('lens_model')::text IS NULL OR a.specific_metadata->>'lens_model' = sqlc.narg('lens_model'));

-- name: CountAssetsVectorUnified :one
-- Count query matching SearchAssetsVectorUnified WHERE clause
-- Returns total count of assets matching semantic search (for pagination)
SELECT COUNT(DISTINCT a.asset_id) as count
FROM assets a
JOIN embeddings e ON a.asset_id = e.asset_id
LEFT JOIN album_assets aa ON a.asset_id = aa.asset_id
WHERE a.is_deleted = false
  AND e.embedding_type = sqlc.arg('embedding_type')::text
  AND e.is_primary = true
  AND (sqlc.narg('asset_type')::text IS NULL OR a.type = sqlc.narg('asset_type'))
  AND (sqlc.narg('asset_types')::text[] IS NULL OR a.type = ANY(sqlc.narg('asset_types')::text[]))
  AND (sqlc.narg('owner_id')::integer IS NULL OR a.owner_id = sqlc.narg('owner_id'))
  AND (sqlc.narg('repository_id')::uuid IS NULL OR a.repository_id = sqlc.narg('repository_id'))
  AND (sqlc.narg('album_id')::integer IS NULL OR aa.album_id = sqlc.narg('album_id'))
  AND (sqlc.narg('date_from')::timestamptz IS NULL OR COALESCE(a.taken_time, a.upload_time) >= sqlc.narg('date_from'))
  AND (sqlc.narg('date_to')::timestamptz IS NULL OR COALESCE(a.taken_time, a.upload_time) <= sqlc.narg('date_to'))
  AND (sqlc.narg('is_raw')::boolean IS NULL OR
    CASE
      WHEN sqlc.narg('is_raw') = true THEN (a.specific_metadata->>'is_raw')::boolean = true
      ELSE (a.specific_metadata->>'is_raw')::boolean = false OR a.specific_metadata->>'is_raw' IS NULL
    END
  )
  AND (sqlc.narg('rating')::integer IS NULL OR
    CASE
      WHEN sqlc.narg('rating') = 0 THEN a.rating IS NULL
      ELSE a.rating = sqlc.narg('rating')
    END
  )
  AND (sqlc.narg('liked')::boolean IS NULL OR a.liked = sqlc.narg('liked'))
  AND (sqlc.narg('camera_model')::text IS NULL OR a.specific_metadata->>'camera_model' = sqlc.narg('camera_model'))
  AND (sqlc.narg('lens_model')::text IS NULL OR a.specific_metadata->>'lens_model' = sqlc.narg('lens_model'))
  AND (e.embedding <-> sqlc.arg('embedding')::vector) <= sqlc.narg('max_distance')::float8;

