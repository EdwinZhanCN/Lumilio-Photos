-- name: UpsertStorageLocation :one
INSERT INTO storage_locations (
    storage_location_id,
    name,
    path,
    kind,
    status,
    mount_fingerprint,
    created_at,
    updated_at
) VALUES (
    ?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8
)
ON CONFLICT (storage_location_id) DO UPDATE SET
    name = EXCLUDED.name,
    path = EXCLUDED.path,
    kind = EXCLUDED.kind,
    status = EXCLUDED.status,
    mount_fingerprint = CASE
        WHEN storage_locations.mount_fingerprint = '' THEN EXCLUDED.mount_fingerprint
        ELSE storage_locations.mount_fingerprint
    END,
    updated_at = EXCLUDED.updated_at
RETURNING *;

-- name: UpdateStorageLocationMountFingerprint :one
UPDATE storage_locations
SET mount_fingerprint = ?2, updated_at = ?3
WHERE storage_location_id = ?1
RETURNING *;

-- name: GetStorageLocation :one
SELECT * FROM storage_locations
WHERE storage_location_id = ?1;

-- name: GetStorageLocationByPath :one
SELECT * FROM storage_locations
WHERE path = ?1;

-- name: GetDefaultStorageLocation :one
SELECT * FROM storage_locations
WHERE kind = 'default';

-- name: ListStorageLocations :many
SELECT * FROM storage_locations
ORDER BY kind ASC, created_at ASC;

-- name: UpdateStorageLocationFromDisk :one
UPDATE storage_locations
SET
    name = ?2,
    status = ?3,
    updated_at = ?4
WHERE storage_location_id = ?1
RETURNING *;

-- name: DeleteExternalStorageLocation :execrows
DELETE FROM storage_locations
WHERE storage_locations.storage_location_id = ?1
  AND storage_locations.kind = 'external'
  AND NOT EXISTS (
      SELECT 1 FROM repositories WHERE repositories.storage_location_id = storage_locations.storage_location_id
  );
