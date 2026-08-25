-- name: CountPhotoMapPoints :one
-- Count query matching GetPhotoMapPoints.
SELECT COUNT(*) as count
FROM assets a
WHERE a.is_deleted = false
  AND a.type = 'PHOTO'
  AND (sqlc.narg('repository_id') IS NULL OR EXISTS (
    SELECT 1 FROM active_asset_occurrences occurrence
    WHERE occurrence.asset_id = a.asset_id
      AND occurrence.repository_id = sqlc.narg('repository_id')
  ))
  AND (sqlc.narg('owner_id') IS NULL OR a.owner_id = sqlc.narg('owner_id'))
  AND a.gps_latitude IS NOT NULL
  AND a.gps_longitude IS NOT NULL
  AND (
    sqlc.narg('south') IS NULL
    OR sqlc.narg('north') IS NULL
    OR a.gps_latitude BETWEEN sqlc.narg('south') AND sqlc.narg('north')
  )
  AND (
    sqlc.narg('west') IS NULL
    OR sqlc.narg('east') IS NULL
    OR CASE
      WHEN sqlc.narg('west') <= sqlc.narg('east')
        THEN a.gps_longitude BETWEEN sqlc.narg('west') AND sqlc.narg('east')
      ELSE a.gps_longitude >= sqlc.narg('west') OR a.gps_longitude <= sqlc.narg('east')
    END
  );
