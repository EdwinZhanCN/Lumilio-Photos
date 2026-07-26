-- name: AgentFacetTopPeople :many
SELECT
    fc.cluster_name AS name,
    COUNT(DISTINCT fi.asset_id) AS count
FROM face_items fi
JOIN face_cluster_members fcm ON fcm.face_id = fi.id
JOIN face_clusters fc ON fc.cluster_id = fcm.cluster_id
WHERE fi.asset_id IN (sqlc.slice('asset_ids'))
  AND fc.cluster_name IS NOT NULL
  AND fc.cluster_name <> ''
GROUP BY 1
ORDER BY count DESC
LIMIT sqlc.arg('top_n');

