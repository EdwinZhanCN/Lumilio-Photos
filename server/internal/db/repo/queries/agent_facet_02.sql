-- name: AgentFacetTypeCounts :many
SELECT a.type, COUNT(*) AS count
FROM assets a
WHERE a.asset_id IN (sqlc.slice('asset_ids'))
  AND a.lifecycle_state = 'active'
GROUP BY a.type;

