-- name: AgentFacetTopPlaces :many
SELECT
    COALESCE(lc.label, lc.city, lc.region, lc.country) AS name,
    COUNT(DISTINCT lca.asset_id) AS count
FROM location_cluster_assets lca
JOIN location_clusters lc ON lc.cluster_id = lca.cluster_id
WHERE lca.asset_id IN (sqlc.slice('asset_ids'))
  AND COALESCE(lc.label, lc.city, lc.region, lc.country) IS NOT NULL
GROUP BY 1
ORDER BY count DESC
LIMIT sqlc.arg('top_n');

