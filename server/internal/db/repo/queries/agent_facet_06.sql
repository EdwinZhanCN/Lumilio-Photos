-- name: AgentFacetTopFocalLengths :many
-- Most-used focal lengths over a ref snapshot, rounded to whole millimetres so
-- 34.9mm and 35mm collapse into one bucket. JSON type checks guard the value.
WITH filter_params AS (
  SELECT CAST(sqlc.narg('asset_ids') AS TEXT) AS asset_ids_json
)
SELECT t.name AS name, t.count AS count FROM (
    SELECT
        (round(json_extract(a.specific_metadata, char(36) || '.focal_length')) || 'mm') AS name,
        COUNT(*) AS count
    FROM assets a
    WHERE a.asset_id IN (SELECT value FROM json_each((SELECT asset_ids_json FROM filter_params)))
      AND a.is_deleted = false
      AND json_type(a.specific_metadata, char(36) || '.focal_length') IN ('integer', 'real')
    GROUP BY 1
) t
WHERE t.name <> '0mm'
ORDER BY t.count DESC
LIMIT sqlc.arg('top_n');
