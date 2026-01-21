-- name: CreateAlbum :one
INSERT INTO albums (user_id, album_name, description, cover_asset_id)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetAlbumByID :one
SELECT * FROM albums WHERE album_id = $1;

-- name: GetAlbumsByUser :many
SELECT * FROM albums
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: UpdateAlbum :one
UPDATE albums
SET album_name = $2, description = $3, cover_asset_id = $4, updated_at = CURRENT_TIMESTAMP
WHERE album_id = $1
RETURNING *;

-- name: DeleteAlbum :exec
DELETE FROM albums WHERE album_id = $1;

-- name: GetAlbumAssets :many
SELECT a.*, aa.position, aa.added_time
FROM assets a
JOIN album_assets aa ON a.asset_id = aa.asset_id
WHERE aa.album_id = $1 AND a.is_deleted = false
ORDER BY aa.position ASC, aa.added_time ASC;

-- name: GetAssetAlbums :many
SELECT al.*, aa.position, aa.added_time
FROM albums al
JOIN album_assets aa ON al.album_id = aa.album_id
WHERE aa.asset_id = $1
ORDER BY al.album_name ASC;

-- name: UpdateAssetPositionInAlbum :exec
UPDATE album_assets
SET position = $3
WHERE album_id = $1 AND asset_id = $2;

-- name: GetAlbumAssetCount :one
SELECT COUNT(*) as count
FROM album_assets aa
JOIN assets a ON aa.asset_id = a.asset_id
WHERE aa.album_id = $1 AND a.is_deleted = false;

-- name: FilterAlbumAssets :many
SELECT a.* FROM assets a
JOIN album_assets aa ON a.asset_id = aa.asset_id
WHERE aa.album_id = sqlc.arg('album_id')
  AND a.is_deleted = false
  AND (sqlc.narg('asset_type')::text IS NULL OR a.type = sqlc.narg('asset_type'))
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
ORDER BY aa.position ASC, aa.added_time DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');
