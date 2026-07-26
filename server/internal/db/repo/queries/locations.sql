-- name: DeleteLocationClustersForScope :exec
DELETE FROM location_clusters
WHERE (sqlc.narg('repository_id') IS NULL OR repository_id = sqlc.narg('repository_id'))
  AND (sqlc.narg('owner_id') IS NULL OR owner_id = sqlc.narg('owner_id'));

-- name: InsertLocationClustersForScope :many
INSERT INTO location_clusters (
  owner_id,
  repository_id,
  geohash,
  precision,
  centroid_latitude,
  centroid_longitude,
  photo_count,
  geocode_status
)
SELECT
  a.owner_id,
  a.repository_id,
  a.gps_geohash_7 AS geohash,
  7 AS precision,
  AVG(a.gps_latitude) AS centroid_latitude,
  AVG(a.gps_longitude) AS centroid_longitude,
  COUNT(*) AS photo_count,
  'pending' AS geocode_status
FROM assets a
WHERE a.is_deleted = false
  AND a.type = 'PHOTO'
  AND a.repository_id IS NOT NULL
  AND a.gps_latitude IS NOT NULL
  AND a.gps_longitude IS NOT NULL
  AND a.gps_geohash_7 IS NOT NULL
  AND (sqlc.narg('repository_id') IS NULL OR a.repository_id = sqlc.narg('repository_id'))
  AND (sqlc.narg('owner_id') IS NULL OR a.owner_id = sqlc.narg('owner_id'))
GROUP BY a.owner_id, a.repository_id, a.gps_geohash_7
ON CONFLICT (owner_id, repository_id, geohash) DO UPDATE
SET
  centroid_latitude = EXCLUDED.centroid_latitude,
  centroid_longitude = EXCLUDED.centroid_longitude,
  photo_count = EXCLUDED.photo_count,
  geocode_status = CASE
    WHEN location_clusters.label IS NULL THEN 'pending'
    ELSE location_clusters.geocode_status
  END
RETURNING *;

-- name: InsertLocationClusterAssetsForScope :exec
INSERT INTO location_cluster_assets (cluster_id, asset_id)
SELECT lc.cluster_id, a.asset_id
FROM assets a
JOIN location_clusters lc
  ON lc.owner_id IS a.owner_id
 AND lc.repository_id = a.repository_id
 AND lc.geohash = a.gps_geohash_7
WHERE a.is_deleted = false
  AND a.type = 'PHOTO'
  AND a.repository_id IS NOT NULL
  AND a.gps_latitude IS NOT NULL
  AND a.gps_longitude IS NOT NULL
  AND a.gps_geohash_7 IS NOT NULL
  AND (sqlc.narg('repository_id') IS NULL OR a.repository_id = sqlc.narg('repository_id'))
  AND (sqlc.narg('owner_id') IS NULL OR a.owner_id = sqlc.narg('owner_id'))
ON CONFLICT (cluster_id, asset_id) DO NOTHING;

-- name: ListLocationClusters :many
SELECT *
FROM location_clusters
WHERE (sqlc.narg('repository_id') IS NULL OR repository_id = sqlc.narg('repository_id'))
  AND (sqlc.narg('owner_id') IS NULL OR owner_id = sqlc.narg('owner_id'))
  AND (sqlc.narg('geohash') IS NULL OR geohash = sqlc.narg('geohash'))
ORDER BY photo_count DESC, updated_at DESC, cluster_id DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountLocationClusters :one
SELECT COUNT(*) AS count
FROM location_clusters
WHERE (sqlc.narg('repository_id') IS NULL OR repository_id = sqlc.narg('repository_id'))
  AND (sqlc.narg('owner_id') IS NULL OR owner_id = sqlc.narg('owner_id'))
  AND (sqlc.narg('geohash') IS NULL OR geohash = sqlc.narg('geohash'));

-- name: ListPendingLocationClusters :many
SELECT *
FROM location_clusters
WHERE geocode_status = 'pending'
  AND (sqlc.narg('repository_id') IS NULL OR repository_id = sqlc.narg('repository_id'))
  AND (sqlc.narg('owner_id') IS NULL OR owner_id = sqlc.narg('owner_id'))
ORDER BY photo_count DESC, updated_at DESC
LIMIT sqlc.arg('limit');

-- name: MarkLocationClustersGeocodeDisabled :exec
UPDATE location_clusters
SET geocode_status = 'disabled',
    provider = sqlc.arg('provider'),
    geocoded_at = sqlc.arg('geocoded_at')
WHERE geocode_status = 'pending'
  AND (sqlc.narg('repository_id') IS NULL OR repository_id = sqlc.narg('repository_id'))
  AND (sqlc.narg('owner_id') IS NULL OR owner_id = sqlc.narg('owner_id'));

-- name: GetReverseGeocodeCache :one
SELECT *
FROM reverse_geocode_cache
WHERE cache_key = sqlc.arg('cache_key')
  AND provider = sqlc.arg('provider')
  AND language = sqlc.arg('language')
  AND (expires_at IS NULL OR expires_at > sqlc.arg('now'));

-- name: UpsertReverseGeocodeCache :one
INSERT INTO reverse_geocode_cache (
  cache_key,
  provider,
  language,
  latitude,
  longitude,
  label,
  country,
  region,
  city,
  raw_response,
  queried_at,
  expires_at
) VALUES (
  sqlc.arg('cache_key'),
  sqlc.arg('provider'),
  sqlc.arg('language'),
  sqlc.arg('latitude'),
  sqlc.arg('longitude'),
  sqlc.narg('label'),
  sqlc.narg('country'),
  sqlc.narg('region'),
  sqlc.narg('city'),
  sqlc.narg('raw_response'),
  sqlc.arg('queried_at'),
  sqlc.narg('expires_at')
)
ON CONFLICT (cache_key) DO UPDATE
SET
  provider = EXCLUDED.provider,
  language = EXCLUDED.language,
  latitude = EXCLUDED.latitude,
  longitude = EXCLUDED.longitude,
  label = EXCLUDED.label,
  country = EXCLUDED.country,
  region = EXCLUDED.region,
  city = EXCLUDED.city,
  raw_response = EXCLUDED.raw_response,
  queried_at = EXCLUDED.queried_at,
  expires_at = EXCLUDED.expires_at
RETURNING *;

-- name: UpdateLocationClusterGeocode :exec
UPDATE location_clusters
SET
  label = sqlc.narg('label'),
  country = sqlc.narg('country'),
  region = sqlc.narg('region'),
  city = sqlc.narg('city'),
  provider = sqlc.arg('provider'),
  geocode_status = sqlc.arg('geocode_status'),
  geocoded_at = sqlc.arg('geocoded_at')
WHERE cluster_id = sqlc.arg('cluster_id');
