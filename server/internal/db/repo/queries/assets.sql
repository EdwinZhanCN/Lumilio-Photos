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
