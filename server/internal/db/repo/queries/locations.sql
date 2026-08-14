-- name: DeleteLocationClustersForScope :exec
DELETE FROM location_clusters
WHERE (sqlc.narg('repository_id') IS NULL OR repository_id = sqlc.narg('repository_id'))
  AND (sqlc.narg('owner_id') IS NULL OR owner_id = sqlc.narg('owner_id'));

-- name: ListLocationClusterCandidatesForScope :many
SELECT
  a.owner_id,
  a.repository_id,
  a.gps_geohash_7 AS geohash,
  AVG(a.gps_latitude) AS centroid_latitude,
  AVG(a.gps_longitude) AS centroid_longitude,
  COUNT(*) AS photo_count
FROM assets a
WHERE a.is_deleted = false
  AND a.type = 'PHOTO'
  AND a.repository_id IS NOT NULL
  AND a.gps_latitude IS NOT NULL
  AND a.gps_longitude IS NOT NULL
  AND a.gps_geohash_7 IS NOT NULL
  AND (sqlc.narg('repository_id') IS NULL OR a.repository_id = sqlc.narg('repository_id'))
  AND (sqlc.narg('owner_id') IS NULL OR a.owner_id = sqlc.narg('owner_id'))
GROUP BY a.owner_id, a.repository_id, a.gps_geohash_7;

-- name: CreateLocationCluster :one
INSERT INTO location_clusters (
  cluster_id,
  owner_id,
  repository_id,
  geohash,
  precision,
  centroid_latitude,
  centroid_longitude,
  photo_count,
  provider,
  geocode_status,
  created_at,
  updated_at
)
VALUES (
  sqlc.arg('cluster_id'),
  sqlc.narg('owner_id'),
  sqlc.arg('repository_id'),
  sqlc.arg('geohash'),
  sqlc.arg('precision'),
  sqlc.arg('centroid_latitude'),
  sqlc.arg('centroid_longitude'),
  sqlc.arg('photo_count'),
  sqlc.narg('provider'),
  sqlc.arg('geocode_status'),
  sqlc.arg('created_at'),
  sqlc.arg('updated_at')
)
RETURNING *;

-- name: InsertLocationClusterAssetsForScope :exec
INSERT INTO location_cluster_assets (cluster_id, asset_id, created_at)
SELECT
  lc.cluster_id,
  a.asset_id,
  CAST(unixepoch('subsec') * 1000000 AS INTEGER) AS created_at
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
  AND (geocode_next_attempt_at IS NULL OR geocode_next_attempt_at <= sqlc.arg('now'))
  AND (sqlc.narg('repository_id') IS NULL OR repository_id = sqlc.narg('repository_id'))
  AND (sqlc.narg('owner_id') IS NULL OR owner_id = sqlc.narg('owner_id'))
ORDER BY photo_count DESC, updated_at DESC
LIMIT sqlc.arg('limit');

-- name: DisableUnresolvedLocationClusters :exec
UPDATE location_clusters
SET geocode_status = 'disabled',
    provider = 'disabled',
    geocode_attempt_count = 0,
    geocode_next_attempt_at = NULL
WHERE geocode_status IN ('pending', 'failed');

-- name: ResetLocationClustersForGeocodingSource :exec
UPDATE location_clusters
SET label = NULL,
    country = NULL,
    region = NULL,
    city = NULL,
    provider = NULL,
    geocode_status = 'pending',
    geocoded_at = NULL,
    geocode_attempt_count = 0,
    geocode_next_attempt_at = NULL
WHERE 1 = 1;

-- name: ResetLocationClustersForGeocodingUserAgent :exec
UPDATE location_clusters
SET geocode_status = CASE
        WHEN geocode_status IN ('disabled', 'failed') THEN 'pending'
        ELSE geocode_status
    END,
    geocode_attempt_count = 0,
    geocode_next_attempt_at = NULL
WHERE geocode_status IN ('pending', 'disabled', 'failed');

-- name: GetPendingLocationClusterSchedule :one
SELECT COUNT(*) AS pending_count,
       CASE WHEN EXISTS (
           SELECT 1
           FROM location_clusters
           WHERE geocode_status = 'pending'
             AND geocode_next_attempt_at IS NULL
       ) THEN 0 ELSE COALESCE(MIN(geocode_next_attempt_at), 0) END AS next_attempt_at
FROM location_clusters
WHERE geocode_status = 'pending';

-- name: GetReverseGeocodeCache :one
SELECT *
FROM reverse_geocode_cache
WHERE source_key = sqlc.arg('source_key')
  AND geohash = sqlc.arg('geohash')
  AND provider = sqlc.arg('provider')
  AND language = sqlc.arg('language')
  AND (expires_at IS NULL OR expires_at > sqlc.arg('now'));

-- name: UpsertReverseGeocodeCache :one
INSERT INTO reverse_geocode_cache (
  source_key,
  geohash,
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
  sqlc.arg('source_key'),
  sqlc.arg('geohash'),
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
ON CONFLICT (source_key, geohash) DO UPDATE
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

-- name: UpdateLocationClusterGeocodeIfRevision :execrows
UPDATE location_clusters
SET label = sqlc.narg('label'),
    country = sqlc.narg('country'),
    region = sqlc.narg('region'),
    city = sqlc.narg('city'),
    provider = sqlc.narg('provider'),
    geocode_status = sqlc.arg('geocode_status'),
    geocoded_at = sqlc.arg('geocoded_at'),
    geocode_attempt_count = sqlc.arg('geocode_attempt_count'),
    geocode_next_attempt_at = sqlc.narg('geocode_next_attempt_at')
WHERE cluster_id = sqlc.arg('cluster_id')
  AND EXISTS (
      SELECT 1 FROM settings
      WHERE id = 1 AND geocoding_revision = sqlc.arg('geocoding_revision')
  );

-- name: UpdateLocationClusterRetryIfRevision :execrows
UPDATE location_clusters
SET geocode_status = sqlc.arg('geocode_status'),
    geocode_attempt_count = sqlc.arg('geocode_attempt_count'),
    geocode_next_attempt_at = sqlc.narg('geocode_next_attempt_at'),
    geocoded_at = sqlc.arg('geocoded_at')
WHERE cluster_id = sqlc.arg('cluster_id')
  AND EXISTS (
      SELECT 1 FROM settings
      WHERE id = 1 AND geocoding_revision = sqlc.arg('geocoding_revision')
  );
