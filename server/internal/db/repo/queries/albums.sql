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
