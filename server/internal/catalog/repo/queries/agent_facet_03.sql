-- name: AgentFacetTopPlaces :many
WITH filter_params AS (
  SELECT CAST(sqlc.narg('asset_ids') AS TEXT) AS asset_ids_json
)
SELECT
    COALESCE(lc.label, lc.city, lc.region, lc.country) AS name,
    COUNT(DISTINCT lca.asset_id) AS count
FROM location_cluster_assets lca
JOIN location_clusters lc ON lc.cluster_id = lca.cluster_id
WHERE lca.asset_id IN (SELECT value FROM json_each((SELECT asset_ids_json FROM filter_params)))
  AND COALESCE(lc.label, lc.city, lc.region, lc.country) IS NOT NULL
GROUP BY 1
ORDER BY count DESC
LIMIT sqlc.arg('top_n');
