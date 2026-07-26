-- name: AgentFacetRatingDist :many
SELECT COALESCE(a.rating, 0) AS rating, COUNT(*) AS count
FROM assets a
WHERE a.asset_id IN (sqlc.slice('asset_ids'))
  AND a.is_deleted = false
GROUP BY 1
ORDER BY 1;

