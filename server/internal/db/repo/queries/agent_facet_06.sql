-- name: AgentFacetTopFocalLengths :many
-- Most-used focal lengths over a ref snapshot, rounded to whole millimetres so
-- 34.9mm and 35mm collapse into one bucket. JSON type checks guard the value.
SELECT t.name AS name, t.count AS count FROM (
    SELECT
        (round(json_extract(a.specific_metadata, char(36) || '.focal_length')) || 'mm') AS name,
        COUNT(*) AS count
    FROM assets a
    WHERE a.asset_id IN (sqlc.slice('asset_ids'))
      AND a.is_deleted = false
      AND json_type(a.specific_metadata, char(36) || '.focal_length') IN ('integer', 'real')
    GROUP BY 1
) t
WHERE t.name <> '0mm'
ORDER BY t.count DESC
LIMIT sqlc.arg('top_n');

