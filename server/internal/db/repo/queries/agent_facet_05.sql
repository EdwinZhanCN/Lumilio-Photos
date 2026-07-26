-- name: AgentFacetCameraCounts :many
SELECT
    json_extract(a.specific_metadata, char(36) || '.camera_model') AS name,
    COUNT(*) AS count
FROM assets a
WHERE a.asset_id IN (sqlc.slice('asset_ids'))
  AND a.is_deleted = false
  AND json_extract(a.specific_metadata, char(36) || '.camera_model') IS NOT NULL
  AND json_extract(a.specific_metadata, char(36) || '.camera_model') <> ''
GROUP BY 1
ORDER BY count DESC
LIMIT sqlc.arg('top_n');

