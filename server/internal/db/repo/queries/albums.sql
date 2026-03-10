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

-- name: CountAlbumsByUserScoped :one
SELECT COUNT(*)
FROM albums al
WHERE al.user_id = sqlc.arg('user_id')
  AND (
    sqlc.narg('repository_id')::uuid IS NULL
    OR EXISTS (
      SELECT 1
      FROM album_assets aa
      JOIN assets a ON a.asset_id = aa.asset_id
      WHERE aa.album_id = al.album_id
        AND a.is_deleted = false
        AND a.repository_id = sqlc.narg('repository_id')
    )
    OR EXISTS (
      SELECT 1
      FROM assets a_cover
      WHERE a_cover.asset_id = al.cover_asset_id
        AND a_cover.is_deleted = false
        AND a_cover.repository_id = sqlc.narg('repository_id')
    )
  );

-- name: GetAlbumsByUserScoped :many
WITH page_albums AS MATERIALIZED (
  SELECT
    al.album_id,
    al.created_at
  FROM albums al
  WHERE al.user_id = sqlc.arg('user_id')
    AND (
      sqlc.narg('repository_id')::uuid IS NULL
      OR EXISTS (
        SELECT 1
        FROM album_assets aa_exists
        JOIN assets a_exists ON a_exists.asset_id = aa_exists.asset_id
        WHERE aa_exists.album_id = al.album_id
          AND a_exists.is_deleted = false
          AND a_exists.repository_id = sqlc.narg('repository_id')
      )
      OR EXISTS (
        SELECT 1
        FROM assets a_cover_exists
        WHERE a_cover_exists.asset_id = al.cover_asset_id
          AND a_cover_exists.is_deleted = false
          AND a_cover_exists.repository_id = sqlc.narg('repository_id')
      )
    )
  ORDER BY al.created_at DESC, al.album_id DESC
  LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset')
)
SELECT
  al.album_id,
  al.user_id,
  al.album_name,
  al.created_at,
  al.updated_at,
  al.description,
  al.cover_asset_id,
  COALESCE(asset_counts.asset_count, 0) AS asset_count,
  COALESCE(cover_asset.cover_asset_id, first_asset.asset_id)::uuid AS display_cover_asset_id
FROM page_albums p
JOIN albums al ON al.album_id = p.album_id
LEFT JOIN LATERAL (
  SELECT COUNT(*) AS asset_count
  FROM album_assets aa_count
  JOIN assets a_count ON a_count.asset_id = aa_count.asset_id
  WHERE aa_count.album_id = al.album_id
    AND a_count.is_deleted = false
    AND (
      sqlc.narg('repository_id')::uuid IS NULL
      OR a_count.repository_id = sqlc.narg('repository_id')
    )
) asset_counts ON true
LEFT JOIN LATERAL (
  SELECT a_cover.asset_id AS cover_asset_id
  FROM assets a_cover
  WHERE a_cover.asset_id = al.cover_asset_id
    AND a_cover.is_deleted = false
    AND (
      sqlc.narg('repository_id')::uuid IS NULL
      OR a_cover.repository_id = sqlc.narg('repository_id')
    )
  LIMIT 1
) cover_asset ON true
LEFT JOIN LATERAL (
  SELECT aa_cover.asset_id
  FROM album_assets aa_cover
  JOIN assets a_scope ON a_scope.asset_id = aa_cover.asset_id
  WHERE aa_cover.album_id = al.album_id
    AND a_scope.is_deleted = false
    AND (
      sqlc.narg('repository_id')::uuid IS NULL
      OR a_scope.repository_id = sqlc.narg('repository_id')
    )
  ORDER BY aa_cover.position ASC NULLS LAST, aa_cover.added_time ASC, aa_cover.asset_id ASC
  LIMIT 1
) first_asset ON true
ORDER BY p.created_at DESC, p.album_id DESC;

-- name: GetAlbumByIDScoped :one
SELECT
  al.album_id,
  al.user_id,
  al.album_name,
  al.created_at,
  al.updated_at,
  al.description,
  al.cover_asset_id,
  COALESCE(asset_counts.asset_count, 0) AS asset_count,
  COALESCE(cover_asset.cover_asset_id, first_asset.asset_id)::uuid AS display_cover_asset_id
FROM albums al
LEFT JOIN LATERAL (
  SELECT COUNT(*) AS asset_count
  FROM album_assets aa_count
  JOIN assets a_count ON a_count.asset_id = aa_count.asset_id
  WHERE aa_count.album_id = al.album_id
    AND a_count.is_deleted = false
    AND (
      sqlc.narg('repository_id')::uuid IS NULL
      OR a_count.repository_id = sqlc.narg('repository_id')
    )
) asset_counts ON true
LEFT JOIN LATERAL (
  SELECT a_cover.asset_id AS cover_asset_id
  FROM assets a_cover
  WHERE a_cover.asset_id = al.cover_asset_id
    AND a_cover.is_deleted = false
    AND (
      sqlc.narg('repository_id')::uuid IS NULL
      OR a_cover.repository_id = sqlc.narg('repository_id')
    )
  LIMIT 1
) cover_asset ON true
LEFT JOIN LATERAL (
  SELECT aa_cover.asset_id
  FROM album_assets aa_cover
  JOIN assets a_scope ON a_scope.asset_id = aa_cover.asset_id
  WHERE aa_cover.album_id = al.album_id
    AND a_scope.is_deleted = false
    AND (
      sqlc.narg('repository_id')::uuid IS NULL
      OR a_scope.repository_id = sqlc.narg('repository_id')
    )
  ORDER BY aa_cover.position ASC NULLS LAST, aa_cover.added_time ASC, aa_cover.asset_id ASC
  LIMIT 1
) first_asset ON true
WHERE al.album_id = sqlc.arg('album_id');

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

-- name: GetAlbumAssetsScoped :many
SELECT a.*, aa.position, aa.added_time
FROM assets a
JOIN album_assets aa ON a.asset_id = aa.asset_id
WHERE aa.album_id = sqlc.arg('album_id')
  AND a.is_deleted = false
  AND (
    sqlc.narg('repository_id')::uuid IS NULL
    OR a.repository_id = sqlc.narg('repository_id')
  )
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

-- name: GetAlbumAssetCountScoped :one
SELECT COUNT(*) as count
FROM album_assets aa
JOIN assets a ON aa.asset_id = a.asset_id
WHERE aa.album_id = sqlc.arg('album_id')
  AND a.is_deleted = false
  AND (
    sqlc.narg('repository_id')::uuid IS NULL
    OR a.repository_id = sqlc.narg('repository_id')
  );
