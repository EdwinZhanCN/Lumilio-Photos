-- name: AgentFacetTypeCounts :many
SELECT a.type, COUNT(*) AS count
FROM assets a
WHERE a.asset_id IN (sqlc.slice('asset_ids'))
  AND a.is_deleted = false
GROUP BY a.type;

