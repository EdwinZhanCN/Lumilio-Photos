-- Facet aggregates over a ref snapshot (agent describe tool / hydration API).
-- Every query takes the materialized asset id array; results feed
-- ref.FacetSummary. User-content strings (labels, names, camera models) are
-- sanitized in Go before reaching the LLM — never here.

-- name: AgentFacetOverview :one
SELECT
    COUNT(*) AS total,
    MIN(COALESCE(a.taken_time, a.upload_time))::timestamptz AS date_from,
    MAX(COALESCE(a.taken_time, a.upload_time))::timestamptz AS date_to,
    COUNT(*) FILTER (WHERE a.liked = true) AS liked_count
FROM assets a
WHERE a.asset_id = ANY(sqlc.arg('asset_ids')::uuid[])
  AND a.is_deleted = false;

-- name: AgentFacetTimeHistogram :many
-- granularity is 'hour', 'day', 'month' or 'year'.
SELECT
    to_char(
        date_trunc(sqlc.arg('granularity')::text, COALESCE(a.taken_time, a.upload_time)),
        CASE
            WHEN sqlc.arg('granularity')::text = 'hour' THEN 'YYYY-MM-DD HH24:00'
            WHEN sqlc.arg('granularity')::text = 'day' THEN 'YYYY-MM-DD'
            WHEN sqlc.arg('granularity')::text = 'year' THEN 'YYYY'
            ELSE 'YYYY-MM'
        END
    ) AS bucket,
    COUNT(*) AS count
FROM assets a
WHERE a.asset_id = ANY(sqlc.arg('asset_ids')::uuid[])
  AND a.is_deleted = false
GROUP BY 1
ORDER BY 1;

-- name: AgentFacetTypeCounts :many
SELECT a.type, COUNT(*) AS count
FROM assets a
WHERE a.asset_id = ANY(sqlc.arg('asset_ids')::uuid[])
  AND a.is_deleted = false
GROUP BY a.type;

-- name: AgentFacetTopPlaces :many
SELECT
    COALESCE(lc.label, lc.city, lc.region, lc.country)::text AS name,
    COUNT(DISTINCT lca.asset_id) AS count
FROM location_cluster_assets lca
JOIN location_clusters lc ON lc.cluster_id = lca.cluster_id
WHERE lca.asset_id = ANY(sqlc.arg('asset_ids')::uuid[])
  AND COALESCE(lc.label, lc.city, lc.region, lc.country) IS NOT NULL
GROUP BY 1
ORDER BY count DESC
LIMIT sqlc.arg('top_n');

-- name: AgentFacetTopPeople :many
SELECT
    fc.cluster_name::text AS name,
    COUNT(DISTINCT fi.asset_id) AS count
FROM face_items fi
JOIN face_cluster_members fcm ON fcm.face_id = fi.id
JOIN face_clusters fc ON fc.cluster_id = fcm.cluster_id
WHERE fi.asset_id = ANY(sqlc.arg('asset_ids')::uuid[])
  AND fc.cluster_name IS NOT NULL
  AND fc.cluster_name <> ''
GROUP BY 1
ORDER BY count DESC
LIMIT sqlc.arg('top_n');

-- name: AgentFacetCameraCounts :many
SELECT
    (a.specific_metadata ->> 'camera_model')::text AS name,
    COUNT(*) AS count
FROM assets a
WHERE a.asset_id = ANY(sqlc.arg('asset_ids')::uuid[])
  AND a.is_deleted = false
  AND a.specific_metadata ->> 'camera_model' IS NOT NULL
  AND a.specific_metadata ->> 'camera_model' <> ''
GROUP BY 1
ORDER BY count DESC
LIMIT sqlc.arg('top_n');

-- name: AgentFacetRatingDist :many
SELECT COALESCE(a.rating, 0) AS rating, COUNT(*) AS count
FROM assets a
WHERE a.asset_id = ANY(sqlc.arg('asset_ids')::uuid[])
  AND a.is_deleted = false
GROUP BY 1
ORDER BY 1;
