-- name: CreateAsset :one
INSERT INTO assets (
    owner_id, type, original_filename, storage_path, mime_type,
    file_size, hash, width, height, duration, taken_time, specific_metadata, rating, liked, repository_id, status
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
RETURNING *;

-- name: GetAssetByID :one
SELECT * FROM assets
WHERE asset_id = $1 AND is_deleted = false;

-- name: GetAssetByIDAny :one
SELECT * FROM assets
WHERE asset_id = $1;

-- name: GetAssetsByIDs :many
SELECT * FROM assets
WHERE asset_id = ANY(sqlc.arg('asset_ids')::uuid[])
  AND is_deleted = false;

-- name: GetAssetExifRaw :one
SELECT exif_raw FROM assets
WHERE asset_id = $1;

-- name: GetAssetsByIDsAny :many
SELECT * FROM assets
WHERE asset_id = ANY(sqlc.arg('asset_ids')::uuid[]);

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
SET original_filename = $2, specific_metadata = $3
WHERE asset_id = $1
RETURNING *;

-- name: UpdateAssetMetadata :exec
UPDATE assets
SET specific_metadata = $2
WHERE asset_id = $1;

-- name: UpdateAssetMetadataWithTakenTime :exec
UPDATE assets
SET specific_metadata = sqlc.arg('specific_metadata'),
    exif_raw = COALESCE(sqlc.narg('exif_raw')::jsonb, exif_raw),
    taken_time = CASE
        WHEN sqlc.arg('taken_time')::timestamptz IS NOT NULL THEN sqlc.arg('taken_time')::timestamptz
        ELSE COALESCE(taken_time, upload_time)
    END,
    capture_offset_minutes = COALESCE(
        sqlc.narg('capture_offset_minutes')::smallint,
        capture_offset_minutes
    ),
    gps_latitude = CASE
        WHEN sqlc.narg('gps_latitude')::float8 BETWEEN -90 AND 90
         AND sqlc.narg('gps_longitude')::float8 BETWEEN -180 AND 180
        THEN sqlc.narg('gps_latitude')::float8
        ELSE NULL
    END,
    gps_longitude = CASE
        WHEN sqlc.narg('gps_latitude')::float8 BETWEEN -90 AND 90
         AND sqlc.narg('gps_longitude')::float8 BETWEEN -180 AND 180
        THEN sqlc.narg('gps_longitude')::float8
        ELSE NULL
    END,
    gps_geohash_5 = CASE
        WHEN sqlc.narg('gps_latitude')::float8 BETWEEN -90 AND 90
         AND sqlc.narg('gps_longitude')::float8 BETWEEN -180 AND 180
        THEN sqlc.narg('gps_geohash_5')::text
        ELSE NULL
    END,
    gps_geohash_7 = CASE
        WHEN sqlc.narg('gps_latitude')::float8 BETWEEN -90 AND 90
         AND sqlc.narg('gps_longitude')::float8 BETWEEN -180 AND 180
        THEN sqlc.narg('gps_geohash_7')::text
        ELSE NULL
    END
WHERE asset_id = sqlc.arg('asset_id');

-- name: DeleteAsset :exec
UPDATE assets
SET is_deleted = true, deleted_at = CURRENT_TIMESTAMP
WHERE asset_id = $1;

-- name: RestoreAsset :exec
UPDATE assets
SET is_deleted = false, deleted_at = NULL
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

-- name: MoveAssetWithinRepository :one
UPDATE assets
SET
    storage_path = sqlc.arg('storage_path'),
    original_filename = sqlc.arg('original_filename'),
    is_deleted = false,
    deleted_at = NULL
WHERE asset_id = sqlc.arg('asset_id')
  AND repository_id = sqlc.arg('repository_id')
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

-- name: GetAssetByRepositoryAndStoragePathAny :one
SELECT * FROM assets
WHERE repository_id = $1 AND storage_path = $2
LIMIT 1;

-- name: ListAssetsByRepositoryAny :many
SELECT * FROM assets
WHERE repository_id = $1
  AND storage_path IS NOT NULL
ORDER BY storage_path ASC;

-- name: SoftDeleteAssetByRepositoryAndStoragePath :execrows
UPDATE assets
SET is_deleted = true, deleted_at = CURRENT_TIMESTAMP
WHERE repository_id = $1
  AND storage_path = $2
  AND is_deleted = false;

-- name: UpdateDiscoveredAssetByID :one
UPDATE assets
SET original_filename = $2,
    mime_type = $3,
    file_size = $4,
    hash = $5,
    taken_time = $6,
    status = $7,
    is_deleted = false,
    deleted_at = NULL
WHERE asset_id = $1
RETURNING *;

-- name: CreateThumbnail :one
INSERT INTO thumbnails (asset_id, size, storage_path, mime_type)
VALUES ($1, $2, $3, $4)
ON CONFLICT (asset_id, size) DO UPDATE
SET storage_path = EXCLUDED.storage_path,
    mime_type = EXCLUDED.mime_type,
    created_at = CURRENT_TIMESTAMP
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

-- name: RemoveAssetTagsBySources :exec
DELETE FROM asset_tags
WHERE asset_id = $1
  AND source = ANY(sqlc.arg('sources')::text[]);

-- name: GetDistinctCameraModels :many
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
  AND (sqlc.narg('owner_id')::integer IS NULL OR owner_id = sqlc.narg('owner_id'))
ORDER BY upload_time DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: GetLikedAssets :many
SELECT * FROM assets
WHERE is_deleted = false
  AND liked = true
  AND (sqlc.narg('owner_id')::integer IS NULL OR owner_id = sqlc.narg('owner_id'))
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

-- name: GetAssetIDsUnified :many
-- Agent ref materialization: same filter semantics as GetAssetsUnified but
-- returns ordered asset ids only (capture time desc). The limit is the ref
-- snapshot cap; callers detect truncation by requesting cap+1.
SELECT a.asset_id
FROM assets a
WHERE a.is_deleted = COALESCE(sqlc.narg('is_deleted')::boolean, false)
  AND (sqlc.narg('query')::text IS NULL OR a.original_filename ILIKE '%' || sqlc.narg('query') || '%')
  AND (sqlc.narg('asset_type')::text IS NULL OR a.type = sqlc.narg('asset_type'))
  AND (sqlc.narg('asset_types')::text[] IS NULL OR a.type = ANY(sqlc.narg('asset_types')::text[]))
  AND (sqlc.narg('owner_id')::integer IS NULL OR a.owner_id = sqlc.narg('owner_id'))
  AND (sqlc.narg('repository_id')::uuid IS NULL OR a.repository_id = sqlc.narg('repository_id'))
  AND (
    sqlc.narg('tag_name')::text IS NULL
    OR EXISTS (
      SELECT 1
      FROM asset_tags at
      JOIN tags t ON t.tag_id = at.tag_id
      WHERE at.asset_id = a.asset_id
        AND t.tag_name = sqlc.narg('tag_name')
        AND (sqlc.narg('tag_source')::text IS NULL OR at.source = sqlc.narg('tag_source'))
    )
  )
  AND (
    sqlc.narg('person_id')::integer IS NULL
    OR EXISTS (
      SELECT 1
      FROM face_cluster_members fcm
      JOIN face_items fi_person ON fi_person.id = fcm.face_id
      WHERE fcm.cluster_id = sqlc.narg('person_id')
        AND fi_person.asset_id = a.asset_id
    )
  )
  AND (
    sqlc.narg('album_id')::integer IS NULL
    OR EXISTS (
      SELECT 1
      FROM album_assets aa
      WHERE aa.asset_id = a.asset_id
        AND aa.album_id = sqlc.narg('album_id')
    )
  )
  AND (sqlc.narg('filename_val')::text IS NULL OR
    CASE COALESCE(sqlc.narg('filename_operator')::text, 'contains')
      WHEN 'matches' THEN a.original_filename ILIKE sqlc.narg('filename_val')
      WHEN 'starts_with' THEN a.original_filename ILIKE sqlc.narg('filename_val') || '%'
      WHEN 'ends_with' THEN a.original_filename ILIKE '%' || sqlc.narg('filename_val')
      ELSE a.original_filename ILIKE '%' || sqlc.narg('filename_val') || '%'
    END
  )
  AND (sqlc.narg('date_from')::timestamptz IS NULL OR COALESCE(a.taken_time, a.upload_time) >= sqlc.narg('date_from'))
  AND (sqlc.narg('date_to')::timestamptz IS NULL OR COALESCE(a.taken_time, a.upload_time) <= sqlc.narg('date_to'))
  AND (sqlc.narg('is_raw')::boolean IS NULL OR
    CASE
      WHEN sqlc.narg('is_raw') = true THEN a.specific_metadata->>'is_raw' = 'true'
      ELSE a.specific_metadata->>'is_raw' = 'false' OR a.specific_metadata->>'is_raw' IS NULL
    END
  )
  AND (sqlc.narg('rating')::integer IS NULL OR
    CASE
      WHEN sqlc.narg('rating') = 0 THEN a.rating IS NULL OR a.rating = 0
      ELSE a.rating = sqlc.narg('rating')
    END
  )
  AND (sqlc.narg('liked')::boolean IS NULL OR
    CASE
      WHEN sqlc.narg('liked') = false THEN a.liked IS NULL OR a.liked = false
      ELSE a.liked = true
    END
  )
  AND (sqlc.narg('camera_model')::text IS NULL OR a.specific_metadata->>'camera_model' = sqlc.narg('camera_model'))
  AND (sqlc.narg('lens_model')::text IS NULL OR a.specific_metadata->>'lens_model' = sqlc.narg('lens_model'))
  AND (
    sqlc.narg('place')::text IS NULL
    OR EXISTS (
      SELECT 1
      FROM location_cluster_assets lca
      JOIN location_clusters lc ON lc.cluster_id = lca.cluster_id
      WHERE lca.asset_id = a.asset_id
        AND lc.search_vector @@ plainto_tsquery('simple', sqlc.narg('place'))
    )
  )
ORDER BY COALESCE(a.taken_time, a.upload_time) DESC, a.asset_id DESC
LIMIT sqlc.arg('limit');

-- name: GetAssetsUnified :many
-- Handles: listing, filename search, and all filtering
-- Use this for most queries unless semantic search is needed
WITH page_ids AS MATERIALIZED (
  SELECT
    a.asset_id,
    CASE
      WHEN sqlc.narg('sort_by')::text = 'recently_added' THEN a.upload_time
      ELSE COALESCE(a.taken_time, a.upload_time)
    END AS sort_time
  FROM assets a
  WHERE a.is_deleted = COALESCE(sqlc.narg('is_deleted')::boolean, false)
    AND (sqlc.narg('query')::text IS NULL OR a.original_filename ILIKE '%' || sqlc.narg('query') || '%')
    AND (sqlc.narg('asset_type')::text IS NULL OR a.type = sqlc.narg('asset_type'))
    AND (sqlc.narg('asset_types')::text[] IS NULL OR a.type = ANY(sqlc.narg('asset_types')::text[]))
    AND (sqlc.narg('owner_id')::integer IS NULL OR a.owner_id = sqlc.narg('owner_id'))
    AND (sqlc.narg('repository_id')::uuid IS NULL OR a.repository_id = sqlc.narg('repository_id'))
    AND (
      sqlc.narg('person_id')::integer IS NULL
      OR EXISTS (
        SELECT 1
        FROM face_cluster_members fcm
        JOIN face_items fi_person ON fi_person.id = fcm.face_id
        WHERE fcm.cluster_id = sqlc.narg('person_id')
          AND fi_person.asset_id = a.asset_id
      )
    )
    AND (
      sqlc.narg('album_id')::integer IS NULL
      OR EXISTS (
        SELECT 1
        FROM album_assets aa
        WHERE aa.asset_id = a.asset_id
          AND aa.album_id = sqlc.narg('album_id')
      )
    )
    AND (
      sqlc.narg('tag_name')::text IS NULL
      OR EXISTS (
        SELECT 1
        FROM asset_tags at
        JOIN tags t ON t.tag_id = at.tag_id
        WHERE at.asset_id = a.asset_id
          AND t.tag_name = sqlc.narg('tag_name')
          AND (sqlc.narg('tag_source')::text IS NULL OR at.source = sqlc.narg('tag_source'))
      )
    )
    AND (sqlc.narg('filename_val')::text IS NULL OR
      CASE COALESCE(sqlc.narg('filename_operator')::text, 'contains')
        WHEN 'matches' THEN a.original_filename ILIKE sqlc.narg('filename_val')
        WHEN 'starts_with' THEN a.original_filename ILIKE sqlc.narg('filename_val') || '%'
        WHEN 'ends_with' THEN a.original_filename ILIKE '%' || sqlc.narg('filename_val')
        ELSE a.original_filename ILIKE '%' || sqlc.narg('filename_val') || '%'
      END
    )
    AND (sqlc.narg('date_from')::timestamptz IS NULL OR COALESCE(a.taken_time, a.upload_time) >= sqlc.narg('date_from'))
    AND (sqlc.narg('date_to')::timestamptz IS NULL OR COALESCE(a.taken_time, a.upload_time) <= sqlc.narg('date_to'))
    AND (sqlc.narg('is_raw')::boolean IS NULL OR
      CASE
        WHEN sqlc.narg('is_raw') = true THEN a.specific_metadata->>'is_raw' = 'true'
        ELSE a.specific_metadata->>'is_raw' = 'false' OR a.specific_metadata->>'is_raw' IS NULL
      END
    )
    AND (sqlc.narg('rating')::integer IS NULL OR
      CASE
        WHEN sqlc.narg('rating') = 0 THEN a.rating IS NULL OR a.rating = 0
        ELSE a.rating = sqlc.narg('rating')
      END
    )
    AND (sqlc.narg('liked')::boolean IS NULL OR
      CASE
        WHEN sqlc.narg('liked') = false THEN a.liked IS NULL OR a.liked = false
        ELSE a.liked = true
      END
    )
    AND (sqlc.narg('camera_model')::text IS NULL OR a.specific_metadata->>'camera_model' = sqlc.narg('camera_model'))
    AND (sqlc.narg('lens_model')::text IS NULL OR a.specific_metadata->>'lens_model' = sqlc.narg('lens_model'))
    AND (
      sqlc.narg('location_north')::float8 IS NULL
      OR sqlc.narg('location_south')::float8 IS NULL
      OR sqlc.narg('location_east')::float8 IS NULL
      OR sqlc.narg('location_west')::float8 IS NULL
      OR (
        a.gps_latitude IS NOT NULL
        AND a.gps_longitude IS NOT NULL
        AND a.gps_latitude
          BETWEEN LEAST(sqlc.narg('location_south')::float8, sqlc.narg('location_north')::float8)
          AND GREATEST(sqlc.narg('location_south')::float8, sqlc.narg('location_north')::float8)
        AND (
          CASE
            WHEN sqlc.narg('location_west')::float8 <= sqlc.narg('location_east')::float8 THEN
              a.gps_longitude BETWEEN sqlc.narg('location_west')::float8 AND sqlc.narg('location_east')::float8
            ELSE
              a.gps_longitude >= sqlc.narg('location_west')::float8
              OR a.gps_longitude <= sqlc.narg('location_east')::float8
          END
        )
      )
    )
  ORDER BY
    sort_time DESC,
    a.asset_id DESC
  LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset')
)
SELECT a.*
FROM page_ids p
JOIN assets a ON a.asset_id = p.asset_id
ORDER BY p.sort_time DESC, p.asset_id DESC;

-- name: CountAssetsUnified :one
-- Count query matching GetAssetsUnified WHERE clause
-- Returns total count of assets matching the filters (for pagination)
SELECT COUNT(*) as count
FROM assets a
WHERE a.is_deleted = COALESCE(sqlc.narg('is_deleted')::boolean, false)
  AND (sqlc.narg('query')::text IS NULL OR a.original_filename ILIKE '%' || sqlc.narg('query') || '%')
  AND (sqlc.narg('asset_type')::text IS NULL OR a.type = sqlc.narg('asset_type'))
  AND (sqlc.narg('asset_types')::text[] IS NULL OR a.type = ANY(sqlc.narg('asset_types')::text[]))
  AND (sqlc.narg('owner_id')::integer IS NULL OR a.owner_id = sqlc.narg('owner_id'))
  AND (sqlc.narg('repository_id')::uuid IS NULL OR a.repository_id = sqlc.narg('repository_id'))
  AND (
    sqlc.narg('person_id')::integer IS NULL
    OR EXISTS (
      SELECT 1
      FROM face_cluster_members fcm
      JOIN face_items fi_person ON fi_person.id = fcm.face_id
      WHERE fcm.cluster_id = sqlc.narg('person_id')
        AND fi_person.asset_id = a.asset_id
    )
  )
  AND (
    sqlc.narg('album_id')::integer IS NULL
    OR EXISTS (
      SELECT 1
      FROM album_assets aa
      WHERE aa.asset_id = a.asset_id
        AND aa.album_id = sqlc.narg('album_id')
    )
  )
  AND (
    sqlc.narg('tag_name')::text IS NULL
    OR EXISTS (
      SELECT 1
      FROM asset_tags at
      JOIN tags t ON t.tag_id = at.tag_id
      WHERE at.asset_id = a.asset_id
        AND t.tag_name = sqlc.narg('tag_name')
        AND (sqlc.narg('tag_source')::text IS NULL OR at.source = sqlc.narg('tag_source'))
    )
  )
  AND (sqlc.narg('filename_val')::text IS NULL OR
    CASE COALESCE(sqlc.narg('filename_operator')::text, 'contains')
      WHEN 'matches' THEN a.original_filename ILIKE sqlc.narg('filename_val')
      WHEN 'starts_with' THEN a.original_filename ILIKE sqlc.narg('filename_val') || '%'
      WHEN 'ends_with' THEN a.original_filename ILIKE '%' || sqlc.narg('filename_val')
      ELSE a.original_filename ILIKE '%' || sqlc.narg('filename_val') || '%'
    END
  )
  AND (sqlc.narg('date_from')::timestamptz IS NULL OR COALESCE(a.taken_time, a.upload_time) >= sqlc.narg('date_from'))
  AND (sqlc.narg('date_to')::timestamptz IS NULL OR COALESCE(a.taken_time, a.upload_time) <= sqlc.narg('date_to'))
  AND (sqlc.narg('is_raw')::boolean IS NULL OR
    CASE
      WHEN sqlc.narg('is_raw') = true THEN a.specific_metadata->>'is_raw' = 'true'
      ELSE a.specific_metadata->>'is_raw' = 'false' OR a.specific_metadata->>'is_raw' IS NULL
    END
  )
  AND (sqlc.narg('rating')::integer IS NULL OR
    CASE
      WHEN sqlc.narg('rating') = 0 THEN a.rating IS NULL OR a.rating = 0
      ELSE a.rating = sqlc.narg('rating')
    END
  )
  AND (sqlc.narg('liked')::boolean IS NULL OR
    CASE
      WHEN sqlc.narg('liked') = false THEN a.liked IS NULL OR a.liked = false
      ELSE a.liked = true
    END
  )
  AND (sqlc.narg('camera_model')::text IS NULL OR a.specific_metadata->>'camera_model' = sqlc.narg('camera_model'))
  AND (sqlc.narg('lens_model')::text IS NULL OR a.specific_metadata->>'lens_model' = sqlc.narg('lens_model'))
  AND (
    sqlc.narg('location_north')::float8 IS NULL
    OR sqlc.narg('location_south')::float8 IS NULL
    OR sqlc.narg('location_east')::float8 IS NULL
    OR sqlc.narg('location_west')::float8 IS NULL
    OR (
    a.gps_latitude IS NOT NULL
    AND a.gps_longitude IS NOT NULL
    AND a.gps_latitude
      BETWEEN LEAST(sqlc.narg('location_south')::float8, sqlc.narg('location_north')::float8)
      AND GREATEST(sqlc.narg('location_south')::float8, sqlc.narg('location_north')::float8)
    AND (
      CASE
        WHEN sqlc.narg('location_west')::float8 <= sqlc.narg('location_east')::float8 THEN
          a.gps_longitude BETWEEN sqlc.narg('location_west')::float8 AND sqlc.narg('location_east')::float8
        ELSE
          a.gps_longitude >= sqlc.narg('location_west')::float8
          OR a.gps_longitude <= sqlc.narg('location_east')::float8
      END
    )
    )
  );

-- name: GetCollapsedBrowseItemsUnified :many
WITH filtered AS MATERIALIZED (
  SELECT
    a.asset_id,
    a.upload_time,
    COALESCE(a.taken_time, a.upload_time) AS captured_time,
    asm.stack_id,
    asm.position
  FROM assets a
  LEFT JOIN asset_stack_members asm ON asm.asset_id = a.asset_id
  WHERE a.is_deleted = COALESCE(sqlc.narg('is_deleted')::boolean, false)
    AND (sqlc.narg('query')::text IS NULL OR a.original_filename ILIKE '%' || sqlc.narg('query') || '%')
    AND (sqlc.narg('asset_type')::text IS NULL OR a.type = sqlc.narg('asset_type'))
    AND (sqlc.narg('asset_types')::text[] IS NULL OR a.type = ANY(sqlc.narg('asset_types')::text[]))
    AND (sqlc.narg('owner_id')::integer IS NULL OR a.owner_id = sqlc.narg('owner_id'))
    AND (sqlc.narg('repository_id')::uuid IS NULL OR a.repository_id = sqlc.narg('repository_id'))
    AND (
      sqlc.narg('person_id')::integer IS NULL
      OR EXISTS (
        SELECT 1
        FROM face_cluster_members fcm
        JOIN face_items fi_person ON fi_person.id = fcm.face_id
        WHERE fcm.cluster_id = sqlc.narg('person_id')
          AND fi_person.asset_id = a.asset_id
      )
    )
    AND (
      sqlc.narg('album_id')::integer IS NULL
      OR EXISTS (
        SELECT 1
        FROM album_assets aa
        WHERE aa.asset_id = a.asset_id
          AND aa.album_id = sqlc.narg('album_id')
      )
    )
    AND (
      sqlc.narg('tag_name')::text IS NULL
      OR EXISTS (
        SELECT 1
        FROM asset_tags at
        JOIN tags t ON t.tag_id = at.tag_id
        WHERE at.asset_id = a.asset_id
          AND t.tag_name = sqlc.narg('tag_name')
          AND (sqlc.narg('tag_source')::text IS NULL OR at.source = sqlc.narg('tag_source'))
      )
    )
    AND (sqlc.narg('filename_val')::text IS NULL OR
      CASE COALESCE(sqlc.narg('filename_operator')::text, 'contains')
        WHEN 'matches' THEN a.original_filename ILIKE sqlc.narg('filename_val')
        WHEN 'starts_with' THEN a.original_filename ILIKE sqlc.narg('filename_val') || '%'
        WHEN 'ends_with' THEN a.original_filename ILIKE '%' || sqlc.narg('filename_val')
        ELSE a.original_filename ILIKE '%' || sqlc.narg('filename_val') || '%'
      END
    )
    AND (sqlc.narg('date_from')::timestamptz IS NULL OR COALESCE(a.taken_time, a.upload_time) >= sqlc.narg('date_from'))
    AND (sqlc.narg('date_to')::timestamptz IS NULL OR COALESCE(a.taken_time, a.upload_time) <= sqlc.narg('date_to'))
    AND (sqlc.narg('is_raw')::boolean IS NULL OR
      CASE
        WHEN sqlc.narg('is_raw') = true THEN a.specific_metadata->>'is_raw' = 'true'
        ELSE a.specific_metadata->>'is_raw' = 'false' OR a.specific_metadata->>'is_raw' IS NULL
      END
    )
    AND (sqlc.narg('rating')::integer IS NULL OR
      CASE
        WHEN sqlc.narg('rating') = 0 THEN a.rating IS NULL OR a.rating = 0
        ELSE a.rating = sqlc.narg('rating')
      END
    )
    AND (sqlc.narg('liked')::boolean IS NULL OR
      CASE
        WHEN sqlc.narg('liked') = false THEN a.liked IS NULL OR a.liked = false
        ELSE a.liked = true
      END
    )
    AND (sqlc.narg('camera_model')::text IS NULL OR a.specific_metadata->>'camera_model' = sqlc.narg('camera_model'))
    AND (sqlc.narg('lens_model')::text IS NULL OR a.specific_metadata->>'lens_model' = sqlc.narg('lens_model'))
    AND (
      sqlc.narg('location_north')::float8 IS NULL
      OR sqlc.narg('location_south')::float8 IS NULL
      OR sqlc.narg('location_east')::float8 IS NULL
      OR sqlc.narg('location_west')::float8 IS NULL
      OR (
        a.gps_latitude IS NOT NULL
        AND a.gps_longitude IS NOT NULL
        AND a.gps_latitude
          BETWEEN LEAST(sqlc.narg('location_south')::float8, sqlc.narg('location_north')::float8)
          AND GREATEST(sqlc.narg('location_south')::float8, sqlc.narg('location_north')::float8)
        AND (
          CASE
            WHEN sqlc.narg('location_west')::float8 <= sqlc.narg('location_east')::float8 THEN
              a.gps_longitude BETWEEN sqlc.narg('location_west')::float8 AND sqlc.narg('location_east')::float8
            ELSE
              a.gps_longitude >= sqlc.narg('location_west')::float8
              OR a.gps_longitude <= sqlc.narg('location_east')::float8
          END
        )
      )
    )
),
stack_covers AS MATERIALIZED (
  SELECT DISTINCT ON (asm.stack_id)
    asm.stack_id,
    asm.asset_id AS cover_asset_id
  FROM asset_stack_members asm
  JOIN assets a ON a.asset_id = asm.asset_id
  WHERE a.is_deleted = COALESCE(sqlc.narg('is_deleted')::boolean, false)
  ORDER BY asm.stack_id, asm.position ASC NULLS LAST, asm.asset_id ASC
),
stack_members_all AS MATERIALIZED (
  SELECT
    asm.stack_id,
    ARRAY_AGG(asm.asset_id ORDER BY asm.position ASC NULLS LAST, asm.asset_id ASC)::uuid[] AS member_asset_ids
  FROM asset_stack_members asm
  JOIN assets a ON a.asset_id = asm.asset_id
  WHERE a.is_deleted = COALESCE(sqlc.narg('is_deleted')::boolean, false)
  GROUP BY asm.stack_id
),
browse_items AS MATERIALIZED (
  SELECT
    CASE WHEN f.stack_id IS NULL THEN 'asset'::text ELSE 'stack'::text END AS item_type,
    f.stack_id,
    CASE WHEN f.stack_id IS NULL THEN f.asset_id ELSE sc.cover_asset_id END AS cover_asset_id,
    sma.member_asset_ids,
    ARRAY_AGG(f.asset_id ORDER BY f.position ASC NULLS LAST, f.asset_id ASC)::uuid[] AS matched_asset_ids
  FROM filtered f
  LEFT JOIN stack_covers sc ON sc.stack_id = f.stack_id
  LEFT JOIN stack_members_all sma ON sma.stack_id = f.stack_id
  GROUP BY
    CASE WHEN f.stack_id IS NULL THEN 'asset'::text ELSE 'stack'::text END,
    f.stack_id,
    CASE WHEN f.stack_id IS NULL THEN f.asset_id ELSE sc.cover_asset_id END,
    sma.member_asset_ids
),
paged AS (
  SELECT
    bi.item_type,
    bi.stack_id,
    bi.cover_asset_id,
    bi.member_asset_ids,
    bi.matched_asset_ids,
    CASE
      WHEN sqlc.narg('sort_by')::text = 'recently_added' THEN cover.upload_time
      ELSE COALESCE(cover.taken_time, cover.upload_time)
    END AS sort_time
  FROM browse_items bi
  JOIN assets cover ON cover.asset_id = bi.cover_asset_id
  ORDER BY sort_time DESC, cover.asset_id DESC
  LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset')
)
SELECT
  p.item_type,
  p.stack_id,
  p.cover_asset_id,
  p.member_asset_ids,
  p.matched_asset_ids,
  sqlc.embed(cover)
FROM paged p
JOIN assets cover ON cover.asset_id = p.cover_asset_id
ORDER BY p.sort_time DESC, p.cover_asset_id DESC;

-- name: CountCollapsedBrowseItemsUnified :one
WITH filtered AS MATERIALIZED (
  SELECT
    a.asset_id,
    asm.stack_id
  FROM assets a
  LEFT JOIN asset_stack_members asm ON asm.asset_id = a.asset_id
  WHERE a.is_deleted = COALESCE(sqlc.narg('is_deleted')::boolean, false)
    AND (sqlc.narg('query')::text IS NULL OR a.original_filename ILIKE '%' || sqlc.narg('query') || '%')
    AND (sqlc.narg('asset_type')::text IS NULL OR a.type = sqlc.narg('asset_type'))
    AND (sqlc.narg('asset_types')::text[] IS NULL OR a.type = ANY(sqlc.narg('asset_types')::text[]))
    AND (sqlc.narg('owner_id')::integer IS NULL OR a.owner_id = sqlc.narg('owner_id'))
    AND (sqlc.narg('repository_id')::uuid IS NULL OR a.repository_id = sqlc.narg('repository_id'))
    AND (
      sqlc.narg('person_id')::integer IS NULL
      OR EXISTS (
        SELECT 1
        FROM face_cluster_members fcm
        JOIN face_items fi_person ON fi_person.id = fcm.face_id
        WHERE fcm.cluster_id = sqlc.narg('person_id')
          AND fi_person.asset_id = a.asset_id
      )
    )
    AND (
      sqlc.narg('album_id')::integer IS NULL
      OR EXISTS (
        SELECT 1
        FROM album_assets aa
        WHERE aa.asset_id = a.asset_id
          AND aa.album_id = sqlc.narg('album_id')
      )
    )
    AND (
      sqlc.narg('tag_name')::text IS NULL
      OR EXISTS (
        SELECT 1
        FROM asset_tags at
        JOIN tags t ON t.tag_id = at.tag_id
        WHERE at.asset_id = a.asset_id
          AND t.tag_name = sqlc.narg('tag_name')
          AND (sqlc.narg('tag_source')::text IS NULL OR at.source = sqlc.narg('tag_source'))
      )
    )
    AND (sqlc.narg('filename_val')::text IS NULL OR
      CASE COALESCE(sqlc.narg('filename_operator')::text, 'contains')
        WHEN 'matches' THEN a.original_filename ILIKE sqlc.narg('filename_val')
        WHEN 'starts_with' THEN a.original_filename ILIKE sqlc.narg('filename_val') || '%'
        WHEN 'ends_with' THEN a.original_filename ILIKE '%' || sqlc.narg('filename_val')
        ELSE a.original_filename ILIKE '%' || sqlc.narg('filename_val') || '%'
      END
    )
    AND (sqlc.narg('date_from')::timestamptz IS NULL OR COALESCE(a.taken_time, a.upload_time) >= sqlc.narg('date_from'))
    AND (sqlc.narg('date_to')::timestamptz IS NULL OR COALESCE(a.taken_time, a.upload_time) <= sqlc.narg('date_to'))
    AND (sqlc.narg('is_raw')::boolean IS NULL OR
      CASE
        WHEN sqlc.narg('is_raw') = true THEN a.specific_metadata->>'is_raw' = 'true'
        ELSE a.specific_metadata->>'is_raw' = 'false' OR a.specific_metadata->>'is_raw' IS NULL
      END
    )
    AND (sqlc.narg('rating')::integer IS NULL OR
      CASE
        WHEN sqlc.narg('rating') = 0 THEN a.rating IS NULL OR a.rating = 0
        ELSE a.rating = sqlc.narg('rating')
      END
    )
    AND (sqlc.narg('liked')::boolean IS NULL OR
      CASE
        WHEN sqlc.narg('liked') = false THEN a.liked IS NULL OR a.liked = false
        ELSE a.liked = true
      END
    )
    AND (sqlc.narg('camera_model')::text IS NULL OR a.specific_metadata->>'camera_model' = sqlc.narg('camera_model'))
    AND (sqlc.narg('lens_model')::text IS NULL OR a.specific_metadata->>'lens_model' = sqlc.narg('lens_model'))
    AND (
      sqlc.narg('location_north')::float8 IS NULL
      OR sqlc.narg('location_south')::float8 IS NULL
      OR sqlc.narg('location_east')::float8 IS NULL
      OR sqlc.narg('location_west')::float8 IS NULL
      OR (
        a.gps_latitude IS NOT NULL
        AND a.gps_longitude IS NOT NULL
        AND a.gps_latitude
          BETWEEN LEAST(sqlc.narg('location_south')::float8, sqlc.narg('location_north')::float8)
          AND GREATEST(sqlc.narg('location_south')::float8, sqlc.narg('location_north')::float8)
        AND (
          CASE
            WHEN sqlc.narg('location_west')::float8 <= sqlc.narg('location_east')::float8 THEN
              a.gps_longitude BETWEEN sqlc.narg('location_west')::float8 AND sqlc.narg('location_east')::float8
            ELSE
              a.gps_longitude >= sqlc.narg('location_west')::float8
              OR a.gps_longitude <= sqlc.narg('location_east')::float8
          END
        )
      )
    )
)
SELECT COUNT(*)::bigint
FROM (
  SELECT CASE WHEN stack_id IS NULL THEN asset_id::text ELSE stack_id::text END AS browse_id
  FROM filtered
  GROUP BY 1
) browse_items;

-- name: GetPhotoMapPoints :many
-- Lightweight photo locations for map clustering/rendering.
SELECT
  a.asset_id,
  a.original_filename,
  a.upload_time,
  a.taken_time,
  a.gps_latitude AS gps_latitude,
  a.gps_longitude AS gps_longitude
FROM assets a
WHERE a.is_deleted = false
  AND a.type = 'PHOTO'
  AND (sqlc.narg('repository_id')::uuid IS NULL OR a.repository_id = sqlc.narg('repository_id'))
  AND (sqlc.narg('owner_id')::integer IS NULL OR a.owner_id = sqlc.narg('owner_id'))
  AND a.gps_latitude IS NOT NULL
  AND a.gps_longitude IS NOT NULL
ORDER BY COALESCE(a.taken_time, a.upload_time) DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountPhotoMapPoints :one
-- Count query matching GetPhotoMapPoints.
SELECT COUNT(*) as count
FROM assets a
WHERE a.is_deleted = false
  AND a.type = 'PHOTO'
  AND (sqlc.narg('repository_id')::uuid IS NULL OR a.repository_id = sqlc.narg('repository_id'))
  AND (sqlc.narg('owner_id')::integer IS NULL OR a.owner_id = sqlc.narg('owner_id'))
  AND a.gps_latitude IS NOT NULL
  AND a.gps_longitude IS NOT NULL;
