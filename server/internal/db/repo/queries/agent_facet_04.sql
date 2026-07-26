-- name: AgentFacetTopPeople :many
WITH filter_params AS (
  SELECT CAST(sqlc.narg('asset_ids') AS TEXT) AS asset_ids_json
)
SELECT
    fc.cluster_name AS name,
    COUNT(DISTINCT fi.asset_id) AS count
FROM face_items fi
JOIN face_cluster_members fcm ON fcm.face_id = fi.id
JOIN face_clusters fc ON fc.cluster_id = fcm.cluster_id
WHERE fi.asset_id IN (SELECT value FROM json_each((SELECT asset_ids_json FROM filter_params)))
  AND fc.cluster_name IS NOT NULL
  AND fc.cluster_name <> ''
GROUP BY 1
ORDER BY count DESC
LIMIT sqlc.arg('top_n');
