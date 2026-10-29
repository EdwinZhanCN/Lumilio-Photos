-- Content identity and the Assets bound to it. Exact byte identity is
-- content_objects; one owner and one content have one Asset.

-- name: GetContentObjectByID :one
SELECT * FROM content_objects WHERE content_id = ?1;

-- name: InsertContentObject :one
INSERT INTO content_objects (
    content_id, hash_algorithm, full_hash, file_size, created_at
) VALUES (?1, ?2, ?3, ?4, ?5)
ON CONFLICT (hash_algorithm, full_hash, file_size) DO UPDATE SET
    created_at = content_objects.created_at
RETURNING *;

-- name: InsertOwnerContentAsset :one
INSERT INTO assets (
    asset_id, owner_id, content_id, type, original_filename, mime_type,
    upload_time, taken_time, rating, status, updated_at
) VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?9, ?10, ?11)
ON CONFLICT (owner_id, content_id) DO UPDATE SET
    updated_at = assets.updated_at
RETURNING *;

-- name: GetOwnerContentAsset :one
SELECT * FROM assets
WHERE owner_id = ?1 AND content_id = ?2;

-- name: GetPreferredActiveAssetOccurrence :one
SELECT *
FROM active_asset_occurrences
WHERE asset_id = ?1
ORDER BY repository_id, entry_id
LIMIT 1;

-- name: ListAssetFullHashPrecheckMatches :many
WITH filter_params AS (
  SELECT CAST(sqlc.arg('full_hashes') AS TEXT) AS full_hashes_json
)
SELECT DISTINCT
    asset.asset_id,
    asset.original_filename,
    content.full_hash,
    content.file_size
FROM assets asset
JOIN content_objects content ON content.content_id = asset.content_id
JOIN active_asset_occurrences occurrence ON occurrence.asset_id = asset.asset_id
CROSS JOIN filter_params
WHERE occurrence.repository_id = sqlc.arg('repository_id')
  AND content.full_hash IN (
    SELECT value FROM json_each(filter_params.full_hashes_json)
  )
ORDER BY asset.asset_id;

-- name: ListAssetQuickFingerprintPrecheckMatches :many
WITH filter_params AS (
  SELECT CAST(sqlc.arg('quick_fingerprints') AS TEXT) AS quick_fingerprints_json
)
SELECT DISTINCT
    asset.asset_id,
    asset.original_filename,
    occurrence.quick_fingerprint,
    occurrence.file_size
FROM assets asset
JOIN active_asset_occurrences occurrence ON occurrence.asset_id = asset.asset_id
CROSS JOIN filter_params
WHERE occurrence.repository_id = sqlc.arg('repository_id')
  AND occurrence.quick_fingerprint IN (
    SELECT value FROM json_each(filter_params.quick_fingerprints_json)
  )
ORDER BY asset.asset_id;
