-- name: AgentFacetRatingDist :many
SELECT COALESCE(a.rating, 0) AS rating, COUNT(*) AS count
FROM assets a
WHERE a.asset_id IN (sqlc.slice('asset_ids'))
  AND a.lifecycle_state = 'active'
GROUP BY 1
ORDER BY 1;

