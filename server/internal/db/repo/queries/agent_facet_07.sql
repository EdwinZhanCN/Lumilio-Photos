-- name: AgentFacetTopLenses :many
WITH filter_params AS (
  SELECT CAST(sqlc.narg('asset_ids') AS TEXT) AS asset_ids_json
)
SELECT
    json_extract(a.specific_metadata, char(36) || '.lens_model') AS name,
    COUNT(*) AS count
FROM assets a
WHERE a.asset_id IN (SELECT value FROM json_each((SELECT asset_ids_json FROM filter_params)))
  AND a.is_deleted = false
  AND json_extract(a.specific_metadata, char(36) || '.lens_model') IS NOT NULL
  AND json_extract(a.specific_metadata, char(36) || '.lens_model') <> ''
GROUP BY 1
ORDER BY count DESC
LIMIT sqlc.arg('top_n');
