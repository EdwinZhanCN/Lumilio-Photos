-- name: CreateAsset :one
INSERT INTO assets (
    asset_id, owner_id, type, original_filename, storage_path, mime_type,
    file_size, content_hash, quick_fingerprint, quick_fingerprint_version,
    width, height, duration, taken_time, specific_metadata, rating, liked, repository_id, status,
    upload_time, updated_at
) VALUES (
    ?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?9, ?10, ?11, ?12, ?13, ?14, ?15, ?16, ?17, ?18, ?19,
    CAST(unixepoch('subsec') * 1000000 AS INTEGER),
    CAST(unixepoch('subsec') * 1000000 AS INTEGER)
)
RETURNING *;

-- name: CreateMediaItemForAsset :exec
INSERT INTO media_items (
    media_item_id,
    owner_id,
    repository_id,
    media_kind,
    primary_asset_id,
    created_at,
    updated_at
) VALUES (
    sqlc.arg('media_item_id'),
    sqlc.narg('owner_id'),
    sqlc.narg('repository_id'),
    sqlc.arg('media_kind'),
    sqlc.arg('asset_id'),
    sqlc.arg('created_at'),
    sqlc.arg('created_at')
);

-- name: AttachAssetToMediaItem :exec
INSERT INTO media_item_assets (
    asset_id,
    media_item_id,
    relation,
    position,
    created_at
) VALUES (
    sqlc.arg('asset_id'),
    sqlc.arg('media_item_id'),
    sqlc.arg('relation'),
    0,
    sqlc.arg('created_at')
);

-- Reconcile the component relation from metadata-confirmed facts. Relations
-- assigned by stack/live-photo matching are never overwritten, and the update
-- is a no-op when the stored relation already matches, so retries stay
-- idempotent.
-- name: ReconcileMediaItemComponentRelation :exec
UPDATE media_item_assets
SET relation = sqlc.arg('relation')
WHERE asset_id = sqlc.arg('asset_id')
  AND relation NOT IN ('live_photo_still', 'live_photo_video', 'edited_version')
  AND relation <> sqlc.arg('relation');

-- name: GetAssetByID :one
SELECT * FROM assets
WHERE asset_id = ?1 AND is_deleted = false;

-- name: GetAssetByIDAny :one
SELECT * FROM assets
WHERE asset_id = ?1;

-- name: GetAssetsByIDs :many
SELECT * FROM assets
WHERE asset_id IN (sqlc.slice('asset_ids'))
  AND is_deleted = false;

-- name: GetAssetsByIDsForOwner :many
WITH filter_params AS (
  SELECT CAST(sqlc.narg('asset_ids') AS TEXT) AS asset_ids_json
)
SELECT * FROM assets
WHERE asset_id IN (SELECT value FROM json_each((SELECT asset_ids_json FROM filter_params)))
  AND owner_id = sqlc.arg('owner_id')
  AND is_deleted = false;

-- name: GetAuthorizedAssetIDs :many
WITH filter_params AS (
  SELECT CAST(sqlc.narg('asset_ids') AS TEXT) AS asset_ids_json
)
SELECT asset_id FROM assets
WHERE asset_id IN (SELECT value FROM json_each((SELECT asset_ids_json FROM filter_params)))
  AND owner_id = sqlc.arg('owner_id')
  AND is_deleted = false;

-- name: LockAuthorizedAssetIDs :many
WITH filter_params AS (
  SELECT CAST(sqlc.narg('asset_ids') AS TEXT) AS asset_ids_json
)
SELECT asset_id FROM assets
WHERE asset_id IN (SELECT value FROM json_each((SELECT asset_ids_json FROM filter_params)))
  AND owner_id = sqlc.arg('owner_id')
  AND is_deleted = false
;

-- name: GetAssetExifRaw :one
SELECT exif_raw FROM assets
WHERE asset_id = ?1;

-- name: GetAssetsByIDsAny :many
SELECT * FROM assets
WHERE asset_id IN (sqlc.slice('asset_ids'));

-- name: GetAssetsByOwner :many
SELECT * FROM assets
WHERE owner_id = ?1 AND is_deleted = false
ORDER BY upload_time DESC
LIMIT ?2 OFFSET ?3;

-- name: GetAssetsByType :many
SELECT * FROM assets
WHERE type = ?1 AND is_deleted = false
ORDER BY upload_time DESC
LIMIT ?2 OFFSET ?3;

-- name: UpdateAsset :one
UPDATE assets
SET original_filename = ?2, specific_metadata = ?3
WHERE asset_id = ?1
RETURNING *;

-- name: UpdateAssetMetadata :exec
UPDATE assets
SET specific_metadata = ?2
WHERE asset_id = ?1;

-- name: UpdateAssetMetadataWithTakenTime :exec
UPDATE assets
SET specific_metadata = sqlc.arg('specific_metadata'),
    exif_raw = COALESCE(sqlc.narg('exif_raw'), exif_raw),
    taken_time = CASE
        WHEN sqlc.arg('taken_time') IS NOT NULL THEN sqlc.arg('taken_time')
        ELSE COALESCE(taken_time, upload_time)
    END,
    capture_offset_minutes = COALESCE(
        sqlc.narg('capture_offset_minutes'),
        capture_offset_minutes
    ),
    gps_latitude = sqlc.narg('gps_latitude'),
    gps_longitude = sqlc.narg('gps_longitude'),
    gps_geohash_5 = sqlc.narg('gps_geohash_5'),
    gps_geohash_7 = sqlc.narg('gps_geohash_7')
WHERE asset_id = sqlc.arg('asset_id');

-- name: DeleteAsset :exec
UPDATE assets
SET is_deleted = true, deleted_at = CAST(unixepoch('subsec') * 1000000 AS INTEGER)
WHERE asset_id = ?1;

-- name: RestoreAsset :exec
UPDATE assets
SET is_deleted = false, deleted_at = NULL
WHERE asset_id = ?1;

-- name: SearchAssets :many
SELECT * FROM assets
WHERE is_deleted = false
AND (?1 IS NULL OR original_filename LIKE '%' || ?1 || '%')
AND (?2 IS NULL OR type = ?2)
ORDER BY upload_time DESC
LIMIT ?3 OFFSET ?4;

-- name: UpdateAssetStatus :one
UPDATE assets
SET status = ?2
WHERE asset_id = ?1
RETURNING *;

-- name: UpdateAssetStoragePathAndStatus :one
UPDATE assets
SET
    storage_path = ?2,
    status = ?3
WHERE asset_id = ?1
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
WHERE json_extract(status, char(36) || '.state') = ?1 AND is_deleted = false
ORDER BY upload_time DESC
LIMIT ?2 OFFSET ?3;

-- name: GetAssetsWithWarnings :many
SELECT * FROM assets
WHERE json_extract(status, char(36) || '.state') = 'warning' AND is_deleted = false
ORDER BY upload_time DESC
LIMIT ?1 OFFSET ?2;

-- name: GetAssetsWithErrors :many
SELECT * FROM assets
WHERE json_extract(status, char(36) || '.state') = 'failed' AND is_deleted = false
ORDER BY upload_time DESC
LIMIT ?1 OFFSET ?2;

-- name: GetAssetsByStatusAndRepository :many
SELECT * FROM assets
WHERE json_extract(status, char(36) || '.state') = ?1 AND repository_id = ?2 AND is_deleted = false
ORDER BY upload_time DESC
LIMIT ?3 OFFSET ?4;

-- name: GetAssetsByStatusAndOwner :many
SELECT * FROM assets
WHERE json_extract(status, char(36) || '.state') = ?1 AND owner_id = ?2 AND is_deleted = false
ORDER BY upload_time DESC
LIMIT ?3 OFFSET ?4;

-- name: CountAssetsByStatus :one
SELECT COUNT(*) as count
FROM assets
WHERE json_extract(status, char(36) || '.state') = ?1 AND is_deleted = false;

-- name: CountAssetsByStatusAndRepository :one
SELECT COUNT(*) as count
FROM assets
WHERE json_extract(status, char(36) || '.state') = ?1 AND repository_id = ?2 AND is_deleted = false;

-- name: CountAssetsByStatusAndOwner :one
SELECT COUNT(*) as count
FROM assets
WHERE json_extract(status, char(36) || '.state') = ?1 AND owner_id = ?2 AND is_deleted = false;

-- name: ResetAssetStatusForRetry :one
UPDATE assets
SET status = json_set(
    status,
    char(36) || '.state',
    '"processing"'
)
WHERE asset_id = ?1 AND json_extract(status, char(36) || '.state') IN ('warning', 'failed')
RETURNING *;

-- name: UpdateAssetStatusWithErrors :one
UPDATE assets
SET status = ?2
WHERE asset_id = ?1
RETURNING *;

-- name: BulkUpdateAssetStatus :exec
UPDATE assets
SET status = sqlc.arg('status')
WHERE CAST(sqlc.narg('asset_ids') AS TEXT) LIKE '%"' || asset_id || '"%'
  AND is_deleted = false;

-- name: GetAssetsByContentHash :many
SELECT * FROM assets
WHERE content_hash = ?1 AND is_deleted = false;

-- name: GetAssetByContentHashAndRepository :one
SELECT * FROM assets
WHERE content_hash = ?1 AND repository_id = ?2 AND is_deleted = false;

-- name: GetAssetsByContentHashesAndRepository :many
WITH filter_params AS (
  SELECT CAST(sqlc.narg('content_hashes') AS TEXT) AS content_hashes_json
)
SELECT asset_id, content_hash, file_size, original_filename, storage_path
FROM assets
WHERE content_hash IN (
    SELECT CAST(value AS TEXT)
    FROM json_each((SELECT content_hashes_json FROM filter_params))
  )
  AND repository_id = sqlc.arg('repository_id')
  AND is_deleted = false;

-- name: GetAssetsByQuickFingerprintsAndRepository :many
WITH filter_params AS (
  SELECT CAST(sqlc.narg('quick_fingerprints') AS TEXT) AS quick_fingerprints_json
)
SELECT asset_id, quick_fingerprint, quick_fingerprint_version, file_size, original_filename
FROM assets
WHERE quick_fingerprint IN (
    SELECT CAST(value AS TEXT)
    FROM json_each((SELECT quick_fingerprints_json FROM filter_params))
  )
  AND repository_id = sqlc.arg('repository_id')
  AND is_deleted = false;

-- name: GetAssetByRepositoryAndStoragePathAny :one
SELECT * FROM assets
WHERE repository_id = ?1 AND storage_path = ?2
LIMIT 1;

-- name: ListAssetsByRepositoryAny :many
SELECT * FROM assets
WHERE repository_id = ?1
  AND storage_path IS NOT NULL
ORDER BY storage_path ASC;

-- name: SoftDeleteAssetByRepositoryAndStoragePath :execrows
UPDATE assets
SET is_deleted = true, deleted_at = CAST(unixepoch('subsec') * 1000000 AS INTEGER)
WHERE repository_id = ?1
  AND storage_path = ?2
  AND is_deleted = false;

-- name: UpdateDiscoveredAssetByID :one
UPDATE assets
SET original_filename = ?2,
    mime_type = ?3,
    file_size = ?4,
    content_hash = ?5,
    quick_fingerprint = ?6,
    quick_fingerprint_version = ?7,
    taken_time = ?8,
    status = ?9,
    is_deleted = false,
    deleted_at = NULL
WHERE asset_id = ?1
RETURNING *;

-- name: CreateThumbnail :one
INSERT INTO thumbnails (asset_id, size, storage_path, mime_type, created_at)
VALUES (?1, ?2, ?3, ?4, CAST(unixepoch('subsec') * 1000000 AS INTEGER))
ON CONFLICT (asset_id, size) DO UPDATE
SET storage_path = EXCLUDED.storage_path,
    mime_type = EXCLUDED.mime_type,
    created_at = CAST(unixepoch('subsec') * 1000000 AS INTEGER)
RETURNING *;

-- name: GetThumbnailByID :one
SELECT * FROM thumbnails WHERE thumbnail_id = ?1;

-- name: GetThumbnailByAssetAndSize :one
SELECT * FROM thumbnails
WHERE asset_id = ?1 AND size = ?2;

-- name: GetThumbnailsByAsset :many
SELECT * FROM thumbnails
WHERE asset_id = ?1
ORDER BY CASE size
    WHEN 'small' THEN 1
    WHEN 'medium' THEN 2
    WHEN 'large' THEN 3
END, thumbnail_id;

-- name: AddAssetToAlbum :exec
INSERT INTO album_assets (asset_id, album_id, position, added_time)
VALUES (?1, ?2, ?3, CAST(unixepoch('subsec') * 1000000 AS INTEGER))
ON CONFLICT (asset_id, album_id) DO NOTHING;

-- name: RemoveAssetFromAlbum :exec
DELETE FROM album_assets
WHERE asset_id = ?1 AND album_id = ?2;

-- name: AddTagToAsset :exec
INSERT INTO asset_tags (asset_id, tag_id, confidence, source)
VALUES (?1, ?2, ?3, ?4)
ON CONFLICT (asset_id, tag_id) DO UPDATE
SET confidence = ?3, source = ?4;

-- name: RemoveTagFromAsset :exec
DELETE FROM asset_tags
WHERE asset_id = ?1 AND tag_id = ?2;

-- name: RemoveAssetTagsBySources :exec
DELETE FROM asset_tags
WHERE asset_id = ?1
  AND CAST(sqlc.narg('sources') AS TEXT) LIKE '%"' || source || '"%';

-- name: GetDistinctCameraModels :many
SELECT DISTINCT json_extract(a.specific_metadata, char(36) || '.camera_model') as camera_model
FROM assets a
WHERE a.is_deleted = false
  AND json_extract(a.specific_metadata, char(36) || '.camera_model') IS NOT NULL
  AND json_extract(a.specific_metadata, char(36) || '.camera_model') != ''
ORDER BY camera_model;

-- name: GetDistinctLenses :many
SELECT DISTINCT json_extract(a.specific_metadata, char(36) || '.lens_model') as lens_model
FROM assets a
WHERE a.is_deleted = false
  AND json_extract(a.specific_metadata, char(36) || '.lens_model') IS NOT NULL
  AND json_extract(a.specific_metadata, char(36) || '.lens_model') != ''
ORDER BY lens_model;

-- name: UpdateAssetRating :exec
UPDATE assets
SET rating = sqlc.arg('rating')
WHERE asset_id = sqlc.arg('asset_id');

-- name: UpdateAssetLike :exec
UPDATE assets
SET liked = sqlc.arg('liked')
WHERE asset_id = sqlc.arg('asset_id');

-- name: UpdateAssetRatingAndLike :exec
UPDATE assets
SET rating = sqlc.arg('rating'),
    liked = sqlc.arg('liked')
WHERE asset_id = sqlc.arg('asset_id');

-- name: UpdateAssetDescription :exec
UPDATE assets
SET specific_metadata = json_set(
    COALESCE(specific_metadata, '{}'),
    char(36) || '.description',
    sqlc.arg('description')
)
WHERE asset_id = sqlc.arg('asset_id');

-- name: GetAssetsByRating :many
SELECT * FROM assets
WHERE is_deleted = false
  AND rating = sqlc.arg('rating')
  AND (sqlc.narg('owner_id') IS NULL OR owner_id = sqlc.narg('owner_id'))
ORDER BY upload_time DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: GetLikedAssets :many
SELECT * FROM assets
WHERE is_deleted = false
  AND liked = true
  AND (sqlc.narg('owner_id') IS NULL OR owner_id = sqlc.narg('owner_id'))
ORDER BY upload_time DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: GetAssetsByOwnerSorted :many
WITH sort_params AS (SELECT CAST(sqlc.arg('sort_order') AS TEXT) AS sort_order)
SELECT a.* FROM assets a
CROSS JOIN sort_params
WHERE a.owner_id = sqlc.arg('owner_id') AND a.is_deleted = false
ORDER BY
  CASE WHEN sort_params.sort_order = 'asc' THEN COALESCE(a.taken_time, a.upload_time) END ASC,
  CASE WHEN sort_params.sort_order = 'desc' THEN COALESCE(a.taken_time, a.upload_time) END DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: GetAssetsByTypesSorted :many
WITH filter_params AS (
  SELECT
    CAST(sqlc.narg('types') AS TEXT) AS types_json,
    CAST(sqlc.arg('sort_order') AS TEXT) AS sort_order
)
SELECT a.* FROM assets a
CROSS JOIN filter_params
WHERE a.type IN (SELECT value FROM json_each(filter_params.types_json))
  AND a.is_deleted = false
ORDER BY
  CASE WHEN filter_params.sort_order = 'asc' THEN COALESCE(a.taken_time, a.upload_time) END ASC,
  CASE WHEN filter_params.sort_order = 'desc' THEN COALESCE(a.taken_time, a.upload_time) END DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: GetAssetsByOwnerAndTypesSorted :many
WITH filter_params AS (
  SELECT
    CAST(sqlc.narg('types') AS TEXT) AS types_json,
    CAST(sqlc.arg('sort_order') AS TEXT) AS sort_order
)
SELECT a.* FROM assets a
CROSS JOIN filter_params
WHERE a.owner_id = sqlc.arg('owner_id')
  AND a.type IN (SELECT value FROM json_each(filter_params.types_json))
  AND a.is_deleted = false
ORDER BY
  CASE WHEN filter_params.sort_order = 'asc' THEN COALESCE(a.taken_time, a.upload_time) END ASC,
  CASE WHEN filter_params.sort_order = 'desc' THEN COALESCE(a.taken_time, a.upload_time) END DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: UpdateAssetDuration :exec
UPDATE assets
SET duration = ?2
WHERE asset_id = ?1;

-- name: UpdateAssetDimensions :exec
UPDATE assets
SET width = ?2, height = ?3
WHERE asset_id = ?1;

-- name: GetAssetsByRatingRange :many
SELECT * FROM assets
WHERE is_deleted = false
  AND rating IS NOT NULL
  AND rating >= sqlc.arg('min_rating')
  AND rating <= sqlc.arg('max_rating')
  AND (sqlc.narg('owner_id') IS NULL OR owner_id = sqlc.narg('owner_id'))
ORDER BY rating DESC, upload_time DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: GetLikedAssetsByOwner :many
SELECT * FROM assets
WHERE is_deleted = false
  AND liked = true
  AND owner_id = sqlc.arg('owner_id')
ORDER BY upload_time DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: GetTopRatedAssets :many
SELECT * FROM assets
WHERE is_deleted = false
  AND rating IS NOT NULL
  AND rating >= sqlc.arg('min_rating')
  AND (sqlc.narg('owner_id') IS NULL OR owner_id = sqlc.narg('owner_id'))
ORDER BY rating DESC, upload_time DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: GetAssetsByRatingAndType :many
SELECT * FROM assets
WHERE is_deleted = false
  AND rating = sqlc.arg('rating')
  AND type = sqlc.arg('asset_type')
  AND (sqlc.narg('owner_id') IS NULL OR owner_id = sqlc.narg('owner_id'))
ORDER BY upload_time DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: GetLikedAssetsByType :many
SELECT * FROM assets
WHERE is_deleted = false
  AND liked = true
  AND type = sqlc.arg('asset_type')
  AND (sqlc.narg('owner_id') IS NULL OR owner_id = sqlc.narg('owner_id'))
ORDER BY upload_time DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountAssetsByRating :many
SELECT rating, COUNT(*) as count
FROM assets
WHERE is_deleted = false
  AND rating IS NOT NULL
  AND (sqlc.narg('owner_id') IS NULL OR owner_id = sqlc.narg('owner_id'))
GROUP BY rating
ORDER BY rating DESC;

-- name: CountLikedAssets :one
SELECT COUNT(*) as count
FROM assets
WHERE is_deleted = false
  AND liked = true
  AND (sqlc.narg('owner_id') IS NULL OR owner_id = sqlc.narg('owner_id'));

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
  AND owner_id = sqlc.arg('owner_id');

-- name: BulkUpdateAssetRating :exec
UPDATE assets
SET rating = sqlc.arg('rating')
WHERE CAST(sqlc.narg('asset_ids') AS TEXT) LIKE '%"' || asset_id || '"%'
  AND is_deleted = false;

-- name: BulkUpdateAssetLiked :exec
UPDATE assets
SET liked = sqlc.arg('liked')
WHERE CAST(sqlc.narg('asset_ids') AS TEXT) LIKE '%"' || asset_id || '"%'
  AND is_deleted = false;

-- name: BulkToggleAssetLiked :exec
UPDATE assets
SET liked = NOT liked
WHERE asset_id IN (sqlc.slice('asset_ids'))
  AND is_deleted = false;

-- name: GetAssetsByOwnerWithRatingLiked :many
WITH sort_params AS (SELECT CAST(sqlc.arg('sort_by') AS TEXT) AS sort_by)
SELECT a.* FROM assets a
CROSS JOIN sort_params
WHERE a.owner_id = sqlc.arg('owner_id')
  AND a.is_deleted = false
  AND (sqlc.narg('has_rating') IS NULL OR
       (sqlc.narg('has_rating') = true AND a.rating IS NOT NULL) OR
       (sqlc.narg('has_rating') = false AND a.rating IS NULL))
  AND (sqlc.narg('is_liked') IS NULL OR a.liked = sqlc.narg('is_liked'))
ORDER BY
  CASE WHEN sort_params.sort_by = 'rating' THEN a.rating END DESC NULLS LAST,
  CASE WHEN sort_params.sort_by = 'upload_time' THEN a.upload_time END DESC,
  CASE WHEN sort_params.sort_by = 'taken_time' THEN COALESCE(a.taken_time, a.upload_time) END DESC
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
  AND repository_id = sqlc.arg('repository_id')
  AND (sqlc.narg('owner_id') IS NULL OR owner_id = sqlc.narg('owner_id'));

-- ============================================================================
-- UNIFIED QUERY API
-- These queries consolidate List, Filter, and Search operations with shared WHERE logic
-- ============================================================================
